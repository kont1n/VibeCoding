"""
Extract face embeddings from images using InsightFace.

Single mode: python extract_faces.py <image_path> [--gpu] [--thumb-dir DIR]
Batch mode:  python extract_faces.py --batch [--gpu] [--thumb-dir DIR]
             Reads image paths from stdin (one per line), outputs JSONL to stdout.

Embedding format: base64-encoded float32 array (2.7 KB vs ~8 KB JSON per face).
"""

import sys
import os
import json
import base64
import argparse
import threading
from concurrent.futures import ThreadPoolExecutor
from queue import Queue

import cv2
import numpy as np
from insightface.app import FaceAnalysis
from insightface.utils.face_align import norm_crop

_app = None
_max_dim = 0

PREFETCH_WORKERS = 8
PREFETCH_DEPTH = 16
REC_BATCH_SIZE = 32


def get_app(use_gpu=False):
    global _app
    if _app is None:
        providers = (
            ["CUDAExecutionProvider", "CPUExecutionProvider"]
            if use_gpu
            else ["CPUExecutionProvider"]
        )
        old_stdout = sys.stdout
        sys.stdout = sys.stderr
        try:
            _app = FaceAnalysis(name="buffalo_l", providers=providers)
            _app.prepare(ctx_id=0 if use_gpu else -1, det_size=(640, 640))
        finally:
            sys.stdout = old_stdout
        active = set()
        for model in _app.models.values():
            if hasattr(model, 'session'):
                active.update(model.session.get_providers())
        print(f"ONNX active providers: {sorted(active)}", file=sys.stderr)
    return _app


def encode_embedding(emb):
    return base64.b64encode(emb.astype(np.float32).tobytes()).decode("ascii")


def load_image(path):
    img = cv2.imread(path)
    if img is not None and _max_dim > 0:
        h, w = img.shape[:2]
        if max(h, w) > _max_dim:
            scale = _max_dim / max(h, w)
            img = cv2.resize(img, (int(w * scale), int(h * scale)), interpolation=cv2.INTER_AREA)
    return path, img


def save_thumbnail(img, face_bbox, image_path, face_idx, thumb_dir):
    h, w = img.shape[:2]
    x1, y1, x2, y2 = (int(v) for v in face_bbox)
    pad_x = int((x2 - x1) * 0.25)
    pad_y = int((y2 - y1) * 0.25)
    cx1, cy1 = max(0, x1 - pad_x), max(0, y1 - pad_y)
    cx2, cy2 = min(w, x2 + pad_x), min(h, y2 + pad_y)
    crop = img[cy1:cy2, cx1:cx2]
    if crop.size == 0:
        return ""
    crop = cv2.resize(crop, (160, 160))
    base = os.path.splitext(os.path.basename(image_path))[0]
    thumb_name = f"{base}_face_{face_idx}.jpg"
    thumb_path = os.path.join(thumb_dir, thumb_name)
    cv2.imwrite(thumb_path, crop, [cv2.IMWRITE_JPEG_QUALITY, 90])
    return thumb_path


def extract_faces(image_path, img, app, thumb_dir=None, thumb_pool=None):
    if img is None:
        return {"file": image_path, "error": f"cannot read image: {image_path}", "faces": []}

    faces = app.get(img)
    results = []

    thumb_futures = []
    for i, face in enumerate(faces):
        if thumb_dir and thumb_pool:
            fut = thumb_pool.submit(save_thumbnail, img, face.bbox, image_path, i, thumb_dir)
            thumb_futures.append((i, fut))
        elif thumb_dir:
            thumb_path = save_thumbnail(img, face.bbox, image_path, i, thumb_dir)
            results.append({
                "bbox": face.bbox.tolist(),
                "embedding": encode_embedding(face.normed_embedding),
                "det_score": float(face.det_score),
                "thumbnail": thumb_path,
            })
            continue

        results.append({
            "bbox": face.bbox.tolist(),
            "embedding": encode_embedding(face.normed_embedding),
            "det_score": float(face.det_score),
            "thumbnail": "",
        })

    for i, fut in thumb_futures:
        results[i]["thumbnail"] = fut.result()

    return {"file": image_path, "faces": results}


def _get_rec_model(app):
    for taskname, model in app.models.items():
        if taskname == 'recognition':
            return model
    return None


def batch_process(app, thumb_dir):
    """
    Pipelined batch: prefetch -> detect (GPU) -> align (CPU threads) -> batch recognize (GPU).
    Recognition runs on batches of aligned faces for higher GPU utilization.
    """
    rec_model = _get_rec_model(app)
    if rec_model is None:
        _batch_process_sequential(app, thumb_dir)
        return

    rec_size = rec_model.input_size[0]
    det_size = tuple(app.det_size)

    image_queue = Queue(maxsize=PREFETCH_DEPTH)
    read_pool = ThreadPoolExecutor(max_workers=PREFETCH_WORKERS)
    align_pool = ThreadPoolExecutor(max_workers=PREFETCH_WORKERS)
    thumb_pool = ThreadPoolExecutor(max_workers=PREFETCH_WORKERS) if thumb_dir else None

    def reader():
        futures = []
        for line in sys.stdin:
            path = line.strip()
            if not path:
                continue
            futures.append(read_pool.submit(load_image, path))
            while len(futures) >= PREFETCH_DEPTH:
                image_queue.put(futures.pop(0).result())
        for f in futures:
            image_queue.put(f.result())
        image_queue.put(None)

    threading.Thread(target=reader, daemon=True).start()

    pending = []
    pending_face_count = 0

    def flush_pending():
        nonlocal pending, pending_face_count
        if not pending:
            return

        all_aligned = []
        for _, _, _, _, aligned in pending:
            all_aligned.extend(aligned)

        all_embeddings = None
        if all_aligned:
            feat = rec_model.get_feat(all_aligned)
            norms = np.linalg.norm(feat, axis=1, keepdims=True)
            np.clip(norms, 1e-10, None, out=norms)
            all_embeddings = feat / norms

        emb_offset = 0
        for path, img, bboxes, det_scores, aligned in pending:
            n = len(aligned)
            faces_result = []
            for i in range(n):
                thumb = ""
                if thumb_dir:
                    thumb = save_thumbnail(img, bboxes[i], path, i, thumb_dir)
                faces_result.append({
                    "bbox": bboxes[i].tolist(),
                    "embedding": encode_embedding(all_embeddings[emb_offset + i]),
                    "det_score": det_scores[i],
                    "thumbnail": thumb,
                })
            emb_offset += n
            _emit({"file": path, "faces": faces_result})

        pending = []
        pending_face_count = 0

    while True:
        item = image_queue.get()
        if item is None:
            flush_pending()
            break

        path, img = item
        if img is None:
            _emit({"file": path, "error": f"cannot read image: {path}", "faces": []})
            continue

        bboxes, kpss = app.det_model.detect(img, input_size=det_size)

        if len(bboxes) == 0:
            _emit({"file": path, "faces": []})
            continue

        futs = []
        bbox_list = []
        det_scores = []
        for i in range(bboxes.shape[0]):
            kps = kpss[i] if kpss is not None else None
            futs.append(align_pool.submit(norm_crop, img, kps, rec_size))
            bbox_list.append(bboxes[i, 0:4])
            det_scores.append(float(bboxes[i, 4]))

        aligned = [f.result() for f in futs]
        pending.append((path, img, bbox_list, det_scores, aligned))
        pending_face_count += len(aligned)

        if pending_face_count >= REC_BATCH_SIZE:
            flush_pending()

    if thumb_pool:
        thumb_pool.shutdown(wait=True)
    align_pool.shutdown(wait=False)
    read_pool.shutdown(wait=False)


def _emit(result):
    sys.stdout.write(json.dumps(result) + "\n")
    sys.stdout.flush()


def _batch_process_sequential(app, thumb_dir):
    """Fallback: original sequential batch processing."""
    image_queue = Queue(maxsize=PREFETCH_DEPTH)
    read_pool = ThreadPoolExecutor(max_workers=PREFETCH_WORKERS)
    thumb_pool = ThreadPoolExecutor(max_workers=PREFETCH_WORKERS) if thumb_dir else None

    def reader():
        futures = []
        for line in sys.stdin:
            path = line.strip()
            if not path:
                continue
            futures.append(read_pool.submit(load_image, path))
            while len(futures) >= PREFETCH_DEPTH:
                image_queue.put(futures.pop(0).result())
        for f in futures:
            image_queue.put(f.result())
        image_queue.put(None)

    threading.Thread(target=reader, daemon=True).start()

    while True:
        item = image_queue.get()
        if item is None:
            break
        path, img = item
        result = extract_faces(path, img, app, thumb_dir, thumb_pool)
        _emit(result)

    if thumb_pool:
        thumb_pool.shutdown(wait=True)
    read_pool.shutdown(wait=False)


def main():
    parser = argparse.ArgumentParser(description="InsightFace embedding extractor")
    parser.add_argument("image", nargs="?", help="Single image path")
    parser.add_argument("--batch", action="store_true", help="Batch mode: read paths from stdin")
    parser.add_argument("--gpu", action="store_true", help="Use CUDA GPU")
    parser.add_argument("--thumb-dir", default="", help="Save face thumbnails to this directory")
    parser.add_argument("--max-dim", type=int, default=0,
                        help="Downscale images so longest side <= this value (0 = no resize)")
    args = parser.parse_args()

    global _max_dim
    _max_dim = args.max_dim

    if args.thumb_dir:
        os.makedirs(args.thumb_dir, exist_ok=True)

    app = get_app(use_gpu=args.gpu)

    if args.batch:
        batch_process(app, args.thumb_dir)
    elif args.image:
        _, img = load_image(args.image)
        result = extract_faces(args.image, img, app, args.thumb_dir)
        json.dump(result, sys.stdout)
    else:
        json.dump({"error": "usage: extract_faces.py <image> or --batch", "faces": []}, sys.stdout)
        sys.exit(1)


if __name__ == "__main__":
    main()
