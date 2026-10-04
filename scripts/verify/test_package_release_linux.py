"""The Linux release contains only the intended, verified Cloud payload."""

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
        for name in ("server", "discovery-scheduler", "discovery-worker", "migrate", "config-check"):
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
        local_config = self.root / "config"
        (local_config / "database").mkdir(parents=True)
        (local_config / "app.toml").write_text("name = 'local-only'\n", encoding="utf-8")
        (local_config / "database" / "primary.toml").write_text(
            "password = 'local-only-secret'\n", encoding="utf-8"
        )
        online_config = self.root / "config_online"
        (online_config / "database").mkdir(parents=True)
        (online_config / "app.toml").write_text("name = 'wt-media-cloud'\n", encoding="utf-8")
        (online_config / "database" / "primary.toml.tpl").write_text(
            "password = {{WT_PRIMARY_DB_PASSWORD}}\n", encoding="utf-8"
        )
        (self.root / "migrations").mkdir()
        (self.root / "migrations" / "001_identity.sql").write_text(
            "CREATE TABLE users (id INT PRIMARY KEY);\n", encoding="utf-8"
        )
        deploy = self.root / "deploy"
        deploy.mkdir(parents=True)
        (deploy / "DEPLOYMENT.md").write_text("# Deployment\n", encoding="utf-8")
        shell_names = (
            "init-config.sh",
            "install.sh",
            "activate.sh",
            "rollback.sh",
            "migrate.sh",
            "verify-package.sh",
            "verify-database.sh",
            "verify-runtime.sh",
        )
        for name in shell_names:
            script = deploy / name
            script.write_text("#!/usr/bin/env bash\nset -euo pipefail\n", encoding="utf-8")
            script.chmod(0o755)
        (deploy / "prepare-database.sql.example").write_text("-- prepare\n", encoding="utf-8")
        render_script = deploy / "render-config.py"
        render_script.write_text(
            "#!/usr/bin/env python3\n", encoding="utf-8"
        )
        render_script.chmod(0o755)
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

    def test_package_contains_binaries_web_and_provenance_without_credentials(self) -> None:
        result = self.package()
        self.assertEqual(result.returncode, 0, result.stderr)
        archive = Path(self.tmp.name) / "out" / "wt-media-cloud_v0.1.0-rc.1_linux-amd64.tar.gz"
        with tarfile.open(archive, "r:gz") as tar:
            names = set(tar.getnames())
            for name in ("server", "discovery-scheduler", "discovery-worker", "migrate", "config-check", "ffmpeg", "ffprobe"):
                self.assertIn(f"wt-media-cloud_v0.1.0-rc.1_linux-amd64/bin/{name}", names)
            self.assertIn("wt-media-cloud_v0.1.0-rc.1_linux-amd64/web/index.cloud.html", names)
            self.assertIn("wt-media-cloud_v0.1.0-rc.1_linux-amd64/ffmpeg-source.json", names)
            self.assertIn("wt-media-cloud_v0.1.0-rc.1_linux-amd64/migrations/001_identity.sql", names)
            self.assertIn("wt-media-cloud_v0.1.0-rc.1_linux-amd64/deploy/DEPLOYMENT.md", names)
            self.assertIn("wt-media-cloud_v0.1.0-rc.1_linux-amd64/config/app.toml", names)
            self.assertIn(
                "wt-media-cloud_v0.1.0-rc.1_linux-amd64/config/database/primary.toml.tpl", names
            )
            self.assertIn("wt-media-cloud_v0.1.0-rc.1_linux-amd64/deploy/render-config.py", names)
            self.assertFalse(any("config_online" in name for name in names))
            self.assertFalse(any(name.endswith("/deploy/systemd/") for name in names))
            self.assertFalse(any(name.endswith("nginx-site-locations.conf.example") for name in names))
            self.assertFalse(any(name.endswith("/deploy/config-template/") for name in names))
            self.assertFalse(any("config_test" in name for name in names))
            template = tar.extractfile(
                "wt-media-cloud_v0.1.0-rc.1_linux-amd64/config/database/primary.toml.tpl"
            ).read().decode("utf-8")  # type: ignore[union-attr]
            self.assertIn("{{WT_PRIMARY_DB_PASSWORD}}", template)
            self.assertNotIn("local-only-secret", template)
            release_info = json.loads(
                tar.extractfile(
                    "wt-media-cloud_v0.1.0-rc.1_linux-amd64/release-info.json"
                ).read().decode("utf-8")  # type: ignore[union-attr]
            )
            self.assertEqual(release_info["source_commit"], "a" * 40)
            self.assertEqual(release_info["product_tag"], "v0.1.0-rc.1")
            self.assertEqual(release_info["database_migration"], "migrations")
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
