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
        shutil.copytree(REPO_ROOT / "config_online", source / "config")
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
            "config-check",
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

    def write_variables(self, name: str, *, database: str, prefix: str) -> Path:
        path = self.root / name
        path.write_text(json.dumps({
            "WT_PRIMARY_DB_HOST": "127.0.0.1",
            "WT_PRIMARY_DB_PORT": 3306,
            "WT_PRIMARY_DB_NAME": database,
            "WT_PRIMARY_DB_USERNAME": "wt_media_cloud",
            "WT_PRIMARY_DB_PASSWORD": "db-'pass",
            "WT_AGENT_AUTH_TOKEN": "agent-token",
            "WT_DOUYIN_API_KEY": "douyin-key",
            "WT_DOUYIN_COOKIE": "douyin-cookie",
            "WT_OBJECT_STORAGE_PREFIX": prefix,
            "WT_OBJECT_STORAGE_ACCESS_KEY": "access-key",
            "WT_OBJECT_STORAGE_SECRET_KEY": "secret-key",
        }, ensure_ascii=False), encoding="utf-8")
        return path

    def test_shell_entries_are_executable_and_syntax_valid(self) -> None:
        scripts = sorted(DEPLOY.glob("*.sh"))
        self.assertGreater(len(scripts), 5)
        for script in scripts:
            self.assertTrue(script.is_file())
            self.assertGreater(script.stat().st_mode & 0o111, 0, f"{script} must be executable")
            result = self.run_script(["bash", "-n", str(script)])
            self.assertEqual(result.returncode, 0, result.stderr)

    def test_init_config_renders_environment_and_validates_configuration(self) -> None:
        release = self.make_release("v1")
        destination = release / "config"
        variables = self.write_variables("online.json", database="wt_media_online", prefix="online/")
        result = self.run_script(
            [
                str(release / "deploy" / "init-config.sh"),
                "--config", str(destination),
                "--environment", "online",
                "--variables-file", str(variables),
                "--owner", os.environ["USER"],
            ]
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        database = (destination / "database" / "primary.toml").read_text(encoding="utf-8")
        self.assertEqual(tomllib.loads(database)["database"], "wt_media_online")
        self.assertEqual(tomllib.loads(database)["password"], "db-'pass")
        app = (destination / "app.toml").read_text(encoding="utf-8")
        self.assertEqual(tomllib.loads(app)["initial_admin"]["username"], "admin")
        self.assertEqual(tomllib.loads(app)["initial_admin"]["password"], "admin123")
        self.assertEqual(
            tomllib.loads((destination / "storage" / "object_storage.toml").read_text(encoding="utf-8"))["prefix"],
            "online/",
        )
        self.assertTrue((destination / ".render-info.json").is_file())
        self.assertFalse(list(destination.rglob("*.tpl")))
        for relative in (
            "credentials/agent.toml",
            "credentials/douyin.toml",
            "credentials/object_storage.toml",
            "storage/object_storage.toml",
            "scheduler/scheduler.toml",
        ):
            self.assertTrue((destination / relative).is_file(), relative)

    def test_init_config_rejects_missing_and_unknown_variables_without_mutating_templates(self) -> None:
        release = self.make_release("v1")
        destination = release / "config"
        variables = self.write_variables("broken.json", database="wt_media_online", prefix="online/")
        values = json.loads(variables.read_text(encoding="utf-8"))
        del values["WT_PRIMARY_DB_PASSWORD"]
        values["WT_UNUSED"] = "unexpected"
        variables.write_text(json.dumps(values), encoding="utf-8")
        result = self.run_script(
            [
                str(release / "deploy" / "init-config.sh"),
                "--config", str(destination),
                "--environment", "online",
                "--variables-file", str(variables),
                "--owner", os.environ["USER"],
            ]
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("missing variables", result.stderr)
        self.assertTrue((destination / "database" / "primary.toml.tpl").is_file())
        self.assertFalse((destination / "database" / "primary.toml").is_file())

    def test_init_config_keeps_templates_when_cloud_validation_fails(self) -> None:
        release = self.make_release("v1")
        validator = release / "bin" / "config-check"
        validator.write_text("#!/bin/sh\necho 'configuration rejected' >&2\nexit 1\n", encoding="utf-8")
        validator.chmod(0o755)
        variables = self.write_variables("online.json", database="wt_media_online", prefix="online/")
        result = self.run_script([
            str(release / "deploy" / "init-config.sh"),
            "--config", str(release / "config"),
            "--environment", "online",
            "--variables-file", str(variables),
            "--owner", os.environ["USER"],
        ])
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("configuration rejected", result.stderr)
        self.assertTrue((release / "config" / "database" / "primary.toml.tpl").is_file())
        self.assertFalse((release / "config" / "database" / "primary.toml").is_file())

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
        self.assertTrue((install_root / "releases" / "v1" / "config").is_dir())
        self.assertFalse((install_root / "releases" / "v1" / "config").is_symlink())
        self.assertTrue((install_root / "releases" / "v1" / "logs").is_dir())
        self.assertTrue((install_root / "releases" / "v1" / "data" / "tmp").is_dir())
        self.assertFalse((install_root / "shared").exists())
        self.assertFalse(current.exists(), "install must not activate before migration")

        variables = self.write_variables("online.json", database="wt_media_online", prefix="online/")
        initialized = self.run_script(
            [
                str(install_root / "releases" / "v1" / "deploy" / "init-config.sh"),
                "--config", str(install_root / "releases" / "v1" / "config"),
                "--environment", "online",
                "--variables-file", str(variables),
                "--owner", os.environ["USER"],
            ]
        )
        self.assertEqual(initialized.returncode, 0, initialized.stderr)
        self.assertTrue((install_root / "releases" / "v1" / "config" / "app.toml").is_file())

        configured_database = install_root / "releases" / "v1" / "config" / "database" / "primary.toml"
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
                "--service-user", os.environ["USER"],
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
        pre_variables = self.write_variables(
            "pre.json", database="wt_media_pre", prefix="pre/"
        )
        second_initialized = self.run_script([
            str(install_root / "releases" / "v2" / "deploy" / "init-config.sh"),
            "--config", str(install_root / "releases" / "v2" / "config"),
            "--environment", "pre",
            "--variables-file", str(pre_variables),
            "--owner", os.environ["USER"],
        ])
        self.assertEqual(second_initialized.returncode, 0, second_initialized.stderr)
        second_database = tomllib.loads(
            (install_root / "releases" / "v2" / "config" / "database" / "primary.toml").read_text(
                encoding="utf-8"
            )
        )
        self.assertEqual(second_database["database"], "wt_media_pre")
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
        self.assertIn("admin / admin123", manual)
        self.assertIn("wt-media/vars/cloud/online.json", manual)
        self.assertIn("wt-media/vars/cloud/pre.json", manual)
        self.assertIn("--environment online", manual)
        self.assertIn("bin/config-check", manual)
        self.assertIn("回退", manual)
        self.assertNotIn("shared/config", manual)
        self.assertNotIn("systemd", manual)
        self.assertNotIn("nginx-site-locations.conf.example", manual)

    def test_baota_projects_serve_and_manage_cloud_processes(self) -> None:
        manual = (DEPLOY / "DEPLOYMENT.md").read_text(encoding="utf-8")
        self.assertIn("### Server：宝塔 Go 项目", manual)
        self.assertIn("/www/wt-media-cloud/current/bin/server", manual)
        self.assertIn("### Scheduler：宝塔进程管理器", manual)
        self.assertIn("/www/wt-media-cloud/current/bin/discovery-scheduler", manual)
        self.assertIn("### Worker：宝塔进程管理器", manual)
        self.assertIn("/www/wt-media-cloud/current/bin/discovery-worker", manual)
        self.assertIn("工作目录都是 `/www/wt-media-cloud/current`", manual)
        self.assertIn("不配置监听端口", manual)
        self.assertIn("Cloud Server 已提供 Cloud Web", manual)
        self.assertIn("`/login` 返回 Cloud Web", manual)
        self.assertIn("未知 `/api/...` 保持 API 404", manual)
        self.assertFalse((DEPLOY / "systemd").exists())
        self.assertFalse((DEPLOY / "nginx-site-locations.conf.example").exists())
        self.assertFalse((DEPLOY / "config-template").exists())


if __name__ == "__main__":
    unittest.main()
