#!/usr/bin/env python3
"""Stamp the exact Desktop Web tree consumed by the Desktop release gate."""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def stamp(web_dir: Path, source_commit: str) -> Path:
    dist = web_dir / "dist-desktop"
    if not (dist / "index.html").is_file() or not (dist / "index.desktop.html").is_file():
        raise ValueError("Desktop Web must contain both index.html and index.desktop.html")
    marker = dist / "frontend-build.json"
    if marker.exists():
        raise FileExistsError(f"frontend marker already exists: {marker}")
    package_version = json.loads((web_dir / "package.json").read_text(encoding="utf-8"))["version"]
    files = sorted((path for path in dist.rglob("*") if path.is_file()), key=lambda path: path.relative_to(dist).as_posix())
    manifest = {path.relative_to(dist).as_posix(): sha256(path) for path in files}
    lines = "".join(f"{name} {digest}\n" for name, digest in manifest.items())
    marker.write_text(json.dumps({
        "source": "wt-media-cloud/web",
        "package_version": package_version,
        "source_commit": source_commit,
        "source_dirty": False,
        "files": len(files),
        "digest": hashlib.sha256(lines.encode("utf-8")).hexdigest(),
        "manifest": manifest,
    }, indent=2) + "\n", encoding="utf-8")
    return marker


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--web-dir", type=Path, default=Path(__file__).resolve().parents[2] / "web")
    parser.add_argument("--source-commit", required=True)
    args = parser.parse_args()
    print(stamp(args.web_dir, args.source_commit))


if __name__ == "__main__":
    main()
