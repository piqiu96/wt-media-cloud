#!/usr/bin/env python3
"""Assemble the Linux Cloud and Desktop Web artifacts from verified build output."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import shutil
import tarfile
import tempfile
import urllib.request
from pathlib import Path


BINARIES = ("server", "discovery-scheduler", "discovery-worker", "migrate")
TAG = re.compile(r"^v\d+\.\d+\.\d+(?:-rc\.[1-9]\d*)?$")


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def require_elf(path: Path) -> None:
    if not path.is_file() or path.stat().st_size < 5:
        raise ValueError(f"expected Linux ELF executable: {path}")
    with path.open("rb") as source:
        if source.read(4) != b"\x7fELF":
            raise ValueError(f"expected Linux ELF executable: {path}")


def copy_ffmpeg(lock_path: Path, destination: Path, temporary: Path) -> dict[str, str]:
    lock = json.loads(lock_path.read_text(encoding="utf-8"))
    source_url = lock["source_url"]
    expected = lock["archive_sha256"]
    if not re.fullmatch(r"[0-9a-f]{64}", expected):
        raise ValueError("FFmpeg archive SHA-256 must be 64 lowercase hex digits")
    archive = temporary / "ffmpeg.tar.xz"
    with urllib.request.urlopen(source_url, timeout=120) as response, archive.open("wb") as output:
        shutil.copyfileobj(response, output)
    actual = sha256(archive)
    if actual != expected:
        raise ValueError(f"FFmpeg archive SHA-256 mismatch: expected {expected}, got {actual}")

    with tarfile.open(archive, "r:xz") as source:
        for name in ("ffmpeg", "ffprobe"):
            members = [entry for entry in source if entry.isfile() and entry.name.endswith(f"/bin/{name}")]
            if len(members) != 1:
                raise ValueError(f"FFmpeg archive must contain exactly one bin/{name}")
            stream = source.extractfile(members[0])
            if stream is None:
                raise ValueError(f"cannot read bin/{name} from FFmpeg archive")
            target = destination / name
            with target.open("wb") as output:
                shutil.copyfileobj(stream, output)
            target.chmod(0o755)
            require_elf(target)
    result = {"source_url": source_url, "archive_sha256": expected, "license": lock["license"]}
    result["ffmpeg_sha256"] = sha256(destination / "ffmpeg")
    result["ffprobe_sha256"] = sha256(destination / "ffprobe")
    return result


def package(root: Path, tag: str, source_commit: str, lock: Path, output: Path) -> tuple[Path, Path]:
    if not TAG.fullmatch(tag):
        raise ValueError(f"invalid product Tag: {tag}")
    if not re.fullmatch(r"[0-9a-f]{40}", source_commit):
        raise ValueError("source commit must be a full Git SHA-1 value")
    required_deployment_paths = (
        root / "deploy" / "DEPLOYMENT.md",
        root / "deploy" / "config-template" / "app.toml",
        root / "deploy" / "config-template" / "database" / "primary.toml",
        root / "deploy" / "migrate.sh",
        root / "deploy" / "nginx-site-locations.conf.example",
    )
    if any(not path.is_file() for path in required_deployment_paths):
        raise ValueError("deployment scripts/templates are required")
    if not any((root / "migrations").glob("*.sql")):
        raise ValueError("SQL migrations are required")
    for name in BINARIES:
        require_elf(root / "bin" / name)
    cloud_web = root / "web" / "dist-cloud"
    desktop_web = root / "web" / "dist-desktop"
    if not (cloud_web / "index.cloud.html").is_file() or not all(
        (desktop_web / name).is_file() for name in ("index.desktop.html", "index.html", "frontend-build.json")
    ):
        raise ValueError("Cloud and Desktop Web builds are both required")
    output.mkdir(parents=True, exist_ok=True)
    package_name = f"wt-media-cloud_{tag}_linux-amd64"
    cloud_archive = output / f"{package_name}.tar.gz"
    desktop_archive = output / f"desktop-web_{tag}.tar.gz"
    if cloud_archive.exists() or desktop_archive.exists():
        raise FileExistsError("release output already exists; use a new output directory")
    with tempfile.TemporaryDirectory(prefix="wt-media-cloud-release-") as temp_name:
        temp = Path(temp_name)
        staged = temp / package_name
        bin_dir = staged / "bin"
        bin_dir.mkdir(parents=True)
        for name in BINARIES:
            shutil.copy2(root / "bin" / name, bin_dir / name)
        provenance = copy_ffmpeg(lock, bin_dir, temp)
        (staged / "ffmpeg-source.json").write_text(json.dumps(provenance, indent=2) + "\n", encoding="utf-8")
        shutil.copytree(cloud_web, staged / "web")
        shutil.copytree(root / "migrations", staged / "migrations")
        shutil.copytree(root / "deploy", staged / "deploy")
        release_info = {
            "schema_version": 1,
            "product_tag": tag,
            "source_commit": source_commit,
            "processes": ("server", "discovery-scheduler", "discovery-worker"),
            "web_targets": ("cloud", "desktop"),
            "database_migration": "migrations",
        }
        (staged / "release-info.json").write_text(
            json.dumps(release_info, indent=2, ensure_ascii=False) + "\n", encoding="utf-8"
        )
        (staged / "DEPLOYMENT.txt").write_text(
            "Read deploy/DEPLOYMENT.md. Inject this environment's private config/ on the server "
            "before migration or startup. No live database, object-storage, or platform "
            "credentials are shipped.\n",
            encoding="utf-8",
        )
        with tarfile.open(cloud_archive, "w:gz") as archive:
            archive.add(staged, arcname=package_name)
        web_staged = temp / f"desktop-web_{tag}"
        shutil.copytree(desktop_web, web_staged)
        with tarfile.open(desktop_archive, "w:gz") as archive:
            archive.add(web_staged, arcname=web_staged.name)
    (output / "SHA256SUMS").write_text(
        "".join(f"{sha256(path)}  {path.name}\n" for path in (cloud_archive, desktop_archive)),
        encoding="utf-8",
    )
    return cloud_archive, desktop_archive


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
    parser.add_argument("--tag", required=True)
    parser.add_argument("--source-commit", required=True, help="full Cloud commit SHA")
    parser.add_argument("--ffmpeg-lock", type=Path, required=True)
    parser.add_argument("--output-dir", type=Path, required=True)
    args = parser.parse_args()
    try:
        for path in package(args.root, args.tag, args.source_commit, args.ffmpeg_lock, args.output_dir):
            print(path)
    except (OSError, ValueError, KeyError, tarfile.TarError) as exc:
        parser.exit(1, f"release package failed: {exc}\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
