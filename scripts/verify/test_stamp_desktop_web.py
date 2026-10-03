from __future__ import annotations

import hashlib
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).resolve().parents[1] / "dev" / "stamp_desktop_web.py"


class StampDesktopWebTest(unittest.TestCase):
    def test_marker_identifies_exact_web_tree_and_source(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            web = Path(temporary)
            (web / "package.json").write_text('{"version":"0.1.0"}', encoding="utf-8")
            dist = web / "dist-desktop"
            dist.mkdir()
            (dist / "index.desktop.html").write_text("desktop", encoding="utf-8")
            (dist / "index.html").write_text("desktop", encoding="utf-8")
            result = subprocess.run([
                sys.executable, str(SCRIPT), "--web-dir", str(web),
                "--source-commit", "abcdef123456",
            ], text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            marker = json.loads((dist / "frontend-build.json").read_text(encoding="utf-8"))
            self.assertEqual(marker["source_commit"], "abcdef123456")
            self.assertEqual(marker["package_version"], "0.1.0")
            lines = "".join(
                f"{name} {hashlib.sha256((dist / name).read_bytes()).hexdigest()}\n"
                for name in ("index.desktop.html", "index.html")
            )
            self.assertEqual(marker["digest"], hashlib.sha256(lines.encode()).hexdigest())
            self.assertEqual(marker["files"], 2)


if __name__ == "__main__":
    unittest.main()
