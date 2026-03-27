"""
Extract face embeddings from images using InsightFace.

Single mode: python extract_faces.py <image_path> [--gpu] [--thumb-dir DIR]
Batch mode:  python extract_faces.py --batch [--gpu] [--thumb-dir DIR]
             Reads image paths from stdin (one per line), outputs JSONL to stdout.
"""

import sys
import os
import json
import argparse

import cv2
import numpy as np
from insightface.app import FaceAnalysis

_app = None


def get_app(use_gpu=False):
    global _app
    if _app is None:
        providers = (
            ["CUDAExecutionProvider", "CPUExecutionProvider"]
            if use_gpu
            else ["CPUExecutionProvider"]
        )
        _app = FaceAnalysis(name="buffalo_l", providers=providers)
        _app.prepare(ctx_id=0 if use_gpu else -1, det_size=(640, 640))
    return _app


def extract_faces(image_path, app, thumb_dir=None):
    img = cv2.imread(image_path)
    if img is None:
        return {"file": image_path, "error": f"cannot read image: {image_path}", "faces": []}

    faces = app.get(img)
    results = []
    h, w = img.shape[:2]

    for i, face in enumerate(faces):
        thumb_path = ""
        if thumb_dir:
            x1, y1, x2, y2 = (int(v) for v in face.bbox)
            pad_x = int((x2 - x1) * 0.25)
            pad_y = int((y2 - y1) * 0.25)
            cx1, cy1 = max(0, x1 - pad_x), max(0, y1 - pad_y)
            cx2, cy2 = min(w, x2 + pad_x), min(h, y2 + pad_y)
            crop = img[cy1:cy2, cx1:cx2]
            if crop.size > 0:
                crop = cv2.resize(crop, (160, 160))
                base = os.path.splitext(os.path.basename(image_path))[0]
                thumb_name = f"{base}_face_{i}.jpg"
                thumb_path = os.path.join(thumb_dir, thumb_name)
                cv2.imwrite(thumb_path, crop, [cv2.IMWRITE_JPEG_QUALITY, 90])

        results.append({
            "bbox": face.bbox.tolist(),
            "embedding": face.normed_embedding.tolist(),
            "det_score": float(face.det_score),
            "thumbnail": thumb_path,
        })

    return {"file": image_path, "faces": results}


def main():
    parser = argparse.ArgumentParser(description="InsightFace embedding extractor")
    parser.add_argument("image", nargs="?", help="Single image path")
    parser.add_argument("--batch", action="store_true", help="Batch mode: read paths from stdin")
    parser.add_argument("--gpu", action="store_true", help="Use CUDA GPU")
    parser.add_argument("--thumb-dir", default="", help="Save face thumbnails to this directory")
    args = parser.parse_args()

    if args.thumb_dir:
        os.makedirs(args.thumb_dir, exist_ok=True)

    app = get_app(use_gpu=args.gpu)

    if args.batch:
        for line in sys.stdin:
            path = line.strip()
            if not path:
                continue
            result = extract_faces(path, app, args.thumb_dir)
            sys.stdout.write(json.dumps(result) + "\n")
            sys.stdout.flush()
    elif args.image:
        result = extract_faces(args.image, app, args.thumb_dir)
        json.dump(result, sys.stdout)
    else:
        json.dump({"error": "usage: extract_faces.py <image> or --batch", "faces": []}, sys.stdout)
        sys.exit(1)


if __name__ == "__main__":
    main()
