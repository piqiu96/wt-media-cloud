"""Deployment payload scripts are executable and generate server-private inputs."""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import tempfile
import tomllib
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
DEPLOY = REPO_ROOT / "deploy"


class DeploymentPackageTest(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)

    def run_script(self, command: list[str], env: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
        merged = os.environ.copy()
        if env:
            merged.update(env)
        return subprocess.run(command, text=True, capture_output=True, env=merged)

    def make_release(self, name: str) -> Path:
        source = self.root / name
        shutil.copytree(DEPLOY, source / "deploy")
        (source / "migrations").mkdir()
        (source / "migrations" / "001_identity.sql").write_text(
            "CREATE TABLE users (id INT PRIMARY KEY);\n", encoding="utf-8"
        )
        (source / "web").mkdir()
        (source / "web" / "index.cloud.html").write_text("<html></html>\n", encoding="utf-8")
        for binary in (
            "server",
            "discovery-scheduler",
            "discovery-worker",
            "migrate",
            "ffmpeg",
            "ffprobe",
        ):
            path = source / "bin" / binary
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
            path.chmod(0o755)
        (source / "release-info.json").write_text(
            json.dumps({"product_tag": name, "source_commit": "a" * 40}) + "\n",
            encoding="utf-8",
        )
        return source

    def test_shell_entries_are_executable_and_syntax_valid(self) -> None:
        scripts = sorted(DEPLOY.glob("*.sh"))
        self.assertGreater(len(scripts), 5)
        for script in scripts:
            self.assertTrue(script.is_file())
            self.assertGreater(script.stat().st_mode & 0o111, 0, f"{script} must be executable")
            result = self.run_script(["bash", "-n", str(script)])
            self.assertEqual(result.returncode, 0, result.stderr)

    def test_init_config_generates_private_configuration(self) -> None:
        destination = self.root / "shared-config"
        result = self.run_script(
            [
                str(DEPLOY / "init-config.sh"),
                "--config", str(destination),
                "--db-name", "wt_media_cloud",
                "--db-user", "wt_media_cloud",
                "--db-password", "db-'pass",
                "--admin-username", "admin",
                "--admin-password", "admin123",
            ]
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        database = (destination / "database" / "primary.toml").read_text(encoding="utf-8")
        self.assertEqual(tomllib.loads(database)["database"], "wt_media_cloud")
        self.assertEqual(tomllib.loads(database)["password"], "db-'pass")
        app = (destination / "app.toml").read_text(encoding="utf-8")
        self.assertEqual(tomllib.loads(app)["initial_admin"]["username"], "admin")
        self.assertEqual(tomllib.loads(app)["initial_admin"]["password"], "admin123")
        for relative in (
            "credentials/agent.toml",
            "credentials/douyin.toml",
            "credentials/object_storage.toml.example",
            "storage/object_storage.toml",
            "scheduler/scheduler.toml",
        ):
            self.assertTrue((destination / relative).is_file(), relative)

    def test_deploy_scripts_reject_short_bootstrap_password(self) -> None:
        result = self.run_script(
            [
                str(DEPLOY / "init-config.sh"),
                "--config", str(self.root / "rejected"),
                "--db-password", "secret",
                "--admin-password", "short",
            ]
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("at least 6 characters", result.stderr)

    def test_install_verify_migrate_and_rollback_paths(self) -> None:
        install_root = self.root / "wt-media-cloud"
        first = self.make_release("v1")
        verify = self.run_script([str(first / "deploy" / "verify-package.sh")])
        self.assertEqual(verify.returncode, 0, verify.stderr)

        installed = self.run_script(
            [
                str(first / "deploy" / "install.sh"),
                "--install-root", str(install_root),
                "--release", "v1",
                "--service-user", os.environ["USER"],
            ]
        )
        self.assertEqual(installed.returncode, 0, installed.stderr)
        current = install_root / "current"
        self.assertFalse(current.is_symlink())
        self.assertTrue((install_root / "releases" / "v1" / "config").is_symlink())
        self.assertTrue((install_root / "releases" / "v1" / "logs").is_symlink())
        self.assertTrue((install_root / "shared" / "data" / "tmp").is_dir())
        self.assertFalse(current.exists(), "install must not activate before migration")

        initialized = self.run_script(
            [str(first / "deploy" / "init-config.sh"),
             "--config", str(install_root / "shared" / "config"),
             "--db-password", "secret", "--admin-password", "admin123"]
        )
        self.assertEqual(initialized.returncode, 0, initialized.stderr)
        self.assertTrue((install_root / "shared" / "config" / "app.toml").is_file())

        configured_database = install_root / "shared" / "config" / "database" / "primary.toml"
        self.assertTrue(configured_database.is_file())
        migrate = self.run_script([str(install_root / "releases" / "v1" / "deploy" / "migrate.sh"), "--dry-run"])
        self.assertEqual(migrate.returncode, 0, migrate.stderr)
        self.assertIn("--create-database=false", migrate.stdout)

        activated = self.run_script([str(first / "deploy" / "activate.sh"),
                                     "--install-root", str(install_root), "--to", "v1"])
        self.assertEqual(activated.returncode, 0, activated.stderr)
        self.assertEqual(current.resolve(), (install_root / "releases" / "v1").resolve())

        duplicate = self.run_script(
            [
                str(first / "deploy" / "install.sh"),
                "--install-root", str(install_root),
                "--release", "v1",
            ]
        )
        self.assertNotEqual(duplicate.returncode, 0)
        self.assertIn("already exists", duplicate.stderr)

        second = self.make_release("v2")
        second_install = self.run_script(
            [
                str(second / "deploy" / "install.sh"),
                "--install-root", str(install_root),
                "--release", "v2",
                "--service-user", os.environ["USER"],
            ]
        )
        self.assertEqual(second_install.returncode, 0, second_install.stderr)
        self.assertEqual(current.resolve(), (install_root / "releases" / "v1").resolve())
        rollback = self.run_script(
            [
                str(first / "deploy" / "rollback.sh"),
                "--install-root", str(install_root),
                "--to", "v1",
            ]
        )
        self.assertEqual(rollback.returncode, 0, rollback.stderr)
        self.assertEqual(current.resolve(), (install_root / "releases" / "v1").resolve())

    def test_manual_documents_explicit_database_and_bootstrap(self) -> None:
        manual = (DEPLOY / "DEPLOYMENT.md").read_text(encoding="utf-8")
        self.assertIn("--create-database=false", manual)
        self.assertIn("--admin-username admin", manual)
        self.assertIn("wt-media-cloud-server.service", manual)
        self.assertIn("wt-media-cloud-scheduler.service", manual)
        self.assertIn("wt-media-cloud-worker.service", manual)
        self.assertIn("回退", manual)


if __name__ == "__main__":
    unittest.main()
