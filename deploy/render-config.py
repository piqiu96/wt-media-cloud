#!/usr/bin/env python3
"""Render a packaged config template tree and validate it before replacement."""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import urllib.request
from pathlib import Path
from typing import Any


PLACEHOLDER = re.compile(r"\{\{([A-Z][A-Z0-9_]*)\}\}")


def load_variables(path: Path | None, url: str | None) -> tuple[dict[str, Any], str]:
    if (path is None) == (url is None):
        raise ValueError("exactly one of --variables-file or --variables-url is required")
    if path is not None:
        raw = path.read_bytes()
    else:
        assert url is not None
        if not url.startswith("https://"):
            raise ValueError("--variables-url must use https://")
        request = urllib.request.Request(url)
        token = os.environ.get("WT_CONFIG_BEARER_TOKEN", "")
        if token:
            request.add_header("Authorization", f"Bearer {token}")
        with urllib.request.urlopen(request, timeout=30) as response:
            raw = response.read()
    try:
        parsed = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise ValueError(f"variables file is not valid JSON: {exc}") from exc
    if not isinstance(parsed, dict) or not all(isinstance(key, str) for key in parsed):
        raise ValueError("variables file must be a JSON object with string keys")
    for key, value in parsed.items():
        if not re.fullmatch(r"[A-Z][A-Z0-9_]*", key):
            raise ValueError(f"invalid variable name: {key}")
        if value is None or isinstance(value, (dict, list)):
            raise ValueError(f"variable {key} must be a string, number, or boolean")
    return parsed, hashlib.sha256(raw).hexdigest()


def toml_literal(value: Any) -> str:
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, str):
        return json.dumps(value, ensure_ascii=False)
    if isinstance(value, (int, float)) and not isinstance(value, bool):
        return str(value)
    raise ValueError(f"unsupported variable value type: {type(value).__name__}")


def render_tree(source: Path, destination: Path, variables: dict[str, Any]) -> None:
    used: set[str] = set()
    template_count = 0
    for item in sorted(source.rglob("*")):
        relative = item.relative_to(source)
        if item.is_symlink():
            raise ValueError(f"config templates may not contain symlinks: {relative}")
        if item.is_dir():
            continue
        if item.name.endswith(".toml.tpl"):
            template_count += 1
            output_relative = Path(str(relative)[: -len(".tpl")])
            text = item.read_text(encoding="utf-8")
            names = set(PLACEHOLDER.findall(text))
            missing = sorted(names - variables.keys())
            if missing:
                raise ValueError("missing variables: " + ", ".join(missing))
            used.update(names)
            rendered = PLACEHOLDER.sub(lambda match: toml_literal(variables[match.group(1)]), text)
            if "{{" in rendered or "}}" in rendered:
                raise ValueError(f"unresolved or invalid placeholder in {relative}")
            target = destination / output_relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(rendered, encoding="utf-8")
        elif item.suffix == ".toml":
            target = destination / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(item, target)
    if template_count == 0:
        raise ValueError("config directory contains no .toml.tpl templates")
    unknown = sorted(variables.keys() - used)
    if unknown:
        raise ValueError("unknown variables: " + ", ".join(unknown))


def render_atomic(config_dir: Path, environment: str, variables: dict[str, Any], digest: str, validator: Path) -> None:
    if environment not in {"pre", "online"}:
        raise ValueError("--environment must be pre or online")
    if not config_dir.is_dir():
        raise ValueError(f"config template directory does not exist: {config_dir}")
    if not validator.is_file() or not os.access(validator, os.X_OK):
        raise ValueError(f"config validator is missing or not executable: {validator}")

    parent = config_dir.parent
    temporary = Path(tempfile.mkdtemp(prefix=f".{config_dir.name}.render-", dir=parent))
    backup = parent / f".{config_dir.name}.templates-{os.getpid()}"
    try:
        render_tree(config_dir, temporary, variables)
        subprocess.run([str(validator), "--config-dir", str(temporary)], check=True)
        (temporary / ".render-info.json").write_text(
            json.dumps(
                {
                    "schema_version": 1,
                    "environment": environment,
                    "variables_sha256": digest,
                    "rendered_at": dt.datetime.now(dt.timezone.utc).isoformat(),
                },
                ensure_ascii=False,
                indent=2,
            )
            + "\n",
            encoding="utf-8",
        )
        config_dir.rename(backup)
        try:
            temporary.rename(config_dir)
        except BaseException:
            backup.rename(config_dir)
            raise
        shutil.rmtree(backup)
    finally:
        if temporary.exists():
            shutil.rmtree(temporary)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True, type=Path)
    parser.add_argument("--environment", required=True)
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--variables-file", type=Path)
    source.add_argument("--variables-url")
    parser.add_argument("--validator", required=True, type=Path)
    args = parser.parse_args()
    try:
        variables, digest = load_variables(args.variables_file, args.variables_url)
        render_atomic(args.config, args.environment, variables, digest, args.validator)
    except (OSError, ValueError, subprocess.CalledProcessError) as exc:
        print(f"config rendering failed: {exc}", file=sys.stderr)
        return 1
    print(f"configuration rendered: environment={args.environment} config={args.config}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
