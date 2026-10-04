"""The Linux release contains the wtmctl deployment payload and no Python runtime."""

from __future__ import annotations

import hashlib
import io
import json
import subprocess
import sys
import tarfile
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parents[1] / "dev" / "package_release_linux.py"


class PackageReleaseLinuxTest(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name) / "cloud"
        for name in ("wt-media-cloud", "discovery-scheduler", "discovery-worker", "migrate", "config-check", "wtmctl"):
            path = self.root / "bin" / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(b"\x7fELF" + name.encode())
            path.chmod(0o755)
        for name in ("dist-cloud", "dist-desktop"):
            path = self.root / "web" / name / f"index.{name.removeprefix('dist-')}.html"
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("<html>ready</html>", encoding="utf-8")
        (self.root / "web" / "dist-desktop" / "index.html").write_text("<html>ready</html>", encoding="utf-8")
        (self.root / "web" / "dist-desktop" / "frontend-build.json").write_text("{}", encoding="utf-8")

        local = self.root / "config"
        (local / "database").mkdir(parents=True)
        (local / "clients" / "http").mkdir(parents=True)
        (local / "storage").mkdir(parents=True)
        (local / "credentials").mkdir(parents=True)
        (local / "app.toml").write_text("name = 'local-only'\n", encoding="utf-8")
        (local / "clients" / "http" / "agent.toml").write_text("host = '127.0.0.1'\n", encoding="utf-8")
        (local / "clients" / "http" / "douyin.toml").write_text("host = 'api.itfaba.com'\n", encoding="utf-8")
        (local / "database" / "primary.toml").write_text("password = 'local-only-secret'\n", encoding="utf-8")
        (local / "storage" / "object_storage.toml").write_text("prefix = 'local/'\n", encoding="utf-8")
        (local / "credentials" / "agent.toml").write_text("auth_token = ''\n", encoding="utf-8")
        (local / "credentials" / "douyin.toml").write_text("api_key = ''\n", encoding="utf-8")
        (local / "credentials" / "object_storage.toml").write_text("access_key = ''\n", encoding="utf-8")

        online = self.root / "config_online"
        (online / "database").mkdir(parents=True)
        (online / "clients" / "http").mkdir(parents=True)
        (online / "storage").mkdir(parents=True)
        (online / "credentials").mkdir(parents=True)
        (online / "app.toml").write_text("name = 'wt-media-cloud'\n", encoding="utf-8")
        (online / "clients" / "http" / "agent.toml").write_text("host = '127.0.0.1'\n", encoding="utf-8")
        (online / "clients" / "http" / "douyin.toml").write_text("host = 'api.itfaba.com'\n", encoding="utf-8")
        (online / "database" / "primary.toml.tpl").write_text("password = {{WT_PRIMARY_DB_PASSWORD}}\n", encoding="utf-8")
        (online / "storage" / "object_storage.toml.tpl").write_text("prefix = {{WT_OBJECT_STORAGE_PREFIX}}\n", encoding="utf-8")
        (online / "credentials" / "agent.toml.tpl").write_text("auth_token = {{WT_AGENT_AUTH_TOKEN}}\n", encoding="utf-8")
        (online / "credentials" / "douyin.toml.tpl").write_text("api_key = {{WT_DOUYIN_API_KEY}}\n", encoding="utf-8")
        (online / "credentials" / "object_storage.toml.tpl").write_text("access_key = {{WT_OBJECT_STORAGE_ACCESS_KEY}}\n", encoding="utf-8")

        (self.root / "migrations").mkdir()
        (self.root / "migrations" / "001_identity.sql").write_text("CREATE TABLE users (id INT);\n", encoding="utf-8")
        deploy = self.root / "deploy"
        (deploy / "examples").mkdir(parents=True)
        (deploy / "DEPLOYMENT.md").write_text("# Deployment\n", encoding="utf-8")
        (deploy / "config-variable-schema.toml").write_text("schema_version = 1\n", encoding="utf-8")
        (deploy / "prepare-database.sql.example").write_text("-- prepare\n", encoding="utf-8")
        (deploy / "examples" / "online.toml.example").write_text("# online\n", encoding="utf-8")
        (deploy / "examples" / "online-deploy.toml.example").write_text("# profile\n", encoding="utf-8")
        (self.root / "release-info.json").write_text(json.dumps({
            "schema_version": 1,
            "product_tag": "v0.1.0-rc.1",
            "source_commit": "a" * 40,
            "configuration": "template-state config/ rendered by wtmctl",
        }), encoding="utf-8")

        self.archive = Path(self.tmp.name) / "ffmpeg.tar.xz"
        with tarfile.open(self.archive, "w:xz") as tar:
            for name in ("ffmpeg", "ffprobe"):
                data = b"\x7fELF" + name.encode()
                entry = tarfile.TarInfo(f"ffmpeg-test/bin/{name}")
                entry.mode = 0o755
                entry.size = len(data)
                tar.addfile(entry, io.BytesIO(data))
        self.digest = hashlib.sha256(self.archive.read_bytes()).hexdigest()
        self.lock = Path(self.tmp.name) / "ffmpeg-lock.json"
        self.lock.write_text(json.dumps({
            "source_url": self.archive.as_uri(),
            "archive_sha256": self.digest,
            "license": "GPL-3.0-or-later",
        }), encoding="utf-8")

    def package(self, digest: str | None = None) -> subprocess.CompletedProcess[str]:
        if digest is not None:
            data = json.loads(self.lock.read_text(encoding="utf-8"))
            data["archive_sha256"] = digest
            self.lock.write_text(json.dumps(data), encoding="utf-8")
        return subprocess.run([
            sys.executable, str(SCRIPT), "--root", str(self.root),
            "--tag", "v0.1.0-rc.1", "--source-commit", "a" * 40,
            "--ffmpeg-lock", str(self.lock),
            "--output-dir", str(Path(self.tmp.name) / "out"),
        ], text=True, capture_output=True)

    def test_package_contains_wtmctl_schema_and_no_python(self) -> None:
        result = self.package()
        self.assertEqual(result.returncode, 0, result.stderr)
        archive = Path(self.tmp.name) / "out" / "wt-media-cloud_v0.1.0-rc.1_linux-amd64.tar.gz"
        with tarfile.open(archive, "r:gz") as tar:
            names = set(tar.getnames())
            root = "wt-media-cloud_v0.1.0-rc.1_linux-amd64/"
            for name in ("wt-media-cloud", "discovery-scheduler", "discovery-worker", "migrate", "config-check", "wtmctl", "ffmpeg", "ffprobe"):
                self.assertIn(root + "bin/" + name, names)
            self.assertIn(root + "deploy/config-variable-schema.toml", names)
            self.assertIn(root + "deploy/examples/online.toml.example", names)
            self.assertIn(root + "config/database/primary.toml.tpl", names)
            self.assertFalse(any(name.endswith(".py") for name in names))
            self.assertFalse(any(name.endswith(".sh") for name in names))
            self.assertFalse(any("config_online" in name for name in names))
            self.assertFalse(any("local-only-secret" in (tar.extractfile(member).read().decode("utf-8") if tar.extractfile(member) else "") for member in tar.getmembers() if member.isfile()))
        self.assertTrue((Path(self.tmp.name) / "out" / "desktop-web_v0.1.0-rc.1.tar.gz").is_file())

    def test_rejects_unexpected_ffmpeg_archive_digest(self) -> None:
        result = self.package("0" * 64)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("SHA-256", result.stderr)
        self.assertFalse((Path(self.tmp.name) / "out" / "wt-media-cloud_v0.1.0-rc.1_linux-amd64.tar.gz").exists())

    def test_rejects_config_and_config_online_layout_mismatch(self) -> None:
        (self.root / "config_online" / "database" / "primary.toml.tpl").unlink()
        result = self.package()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("config layout mismatch", result.stderr)


if __name__ == "__main__":
    unittest.main()
