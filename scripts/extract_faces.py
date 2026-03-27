"""
Extract face embeddings from an image using InsightFace.

Usage: python extract_faces.py <image_path>
Output: JSON to stdout with detected faces, bounding boxes and 512-dim embeddings.
"""

import sys
import json

import cv2
import numpy as np
from insightface.app import FaceAnalysis

_app = None


def get_app():
    global _app
    if _app is None:
        _app = FaceAnalysis(
            name="buffalo_l",
            providers=["CPUExecutionProvider"],
        )
        _app.prepare(ctx_id=-1, det_size=(640, 640))
    return _app


def extract_faces(image_path: str) -> dict:
    img = cv2.imread(image_path)
    if img is None:
        return {"error": f"cannot read image: {image_path}", "faces": []}

    app = get_app()
    faces = app.get(img)

    results = []
    for face in faces:
        results.append({
            "bbox": face.bbox.tolist(),
            "embedding": face.normed_embedding.tolist(),
            "det_score": float(face.det_score),
        })

    return {"faces": results}


def main():
    if len(sys.argv) < 2:
        json.dump({"error": "usage: extract_faces.py <image_path>", "faces": []}, sys.stdout)
        sys.exit(1)

    image_path = sys.argv[1]
    result = extract_faces(image_path)
    json.dump(result, sys.stdout)


if __name__ == "__main__":
    main()
