# -*- coding: utf-8 -*-

import builtins
import posix
import sys
import unittest
from importlib import import_module
from pathlib import Path

PYTHON_ROOT = Path(__file__).resolve().parents[1]
if str(PYTHON_ROOT) not in sys.path:
    sys.path.insert(0, str(PYTHON_ROOT))

from mako_render import mako_render
from mako_render.context import MakoSandbox
from mako_render.exceptions import ForbiddenMakoTemplateException
from mako_render.patch import default_black_list, patch


def _snapshot_blacklist():
    saved = {}
    for module_name, names in default_black_list.items():
        try:
            module = import_module(module_name)
        except ImportError:
            continue
        for name in names:
            if hasattr(module, name):
                saved[(module, name)] = getattr(module, name)
    return saved


def _restore_blacklist(saved):
    for (module, name), func in saved.items():
        setattr(module, name, func)


class RuntimePatchTest(unittest.TestCase):
    def test_blacklist_covers_posix_and_builtin_open(self):
        for name in ("system", "popen", "execl", "execv", "open"):
            self.assertIn(name, default_black_list["posix"])
        self.assertIn("open", default_black_list["builtins"])
        self.assertIn("open", default_black_list["io"])

    def test_sandbox_blocks_posix_system_and_builtin_open(self):
        called = {"posix": 0, "open": 0}
        saved = _snapshot_blacklist()

        def posix_sentinel(*_args, **_kwargs):
            called["posix"] += 1
            return 0

        def open_sentinel(*_args, **_kwargs):
            called["open"] += 1
            return None

        try:
            posix.system = posix_sentinel
            builtins.open = open_sentinel
            patch(default_black_list)
            with MakoSandbox():
                with self.assertRaises(ForbiddenMakoTemplateException):
                    posix.system("true")
                with self.assertRaises(ForbiddenMakoTemplateException):
                    builtins.open("/no/such/file")
            self.assertEqual({"posix": 0, "open": 0}, called)
            # 沙箱外仍放行，避免渲染进程自己的文件访问被误伤。
            posix.system("true")
            builtins.open("/no/such/file")
            self.assertEqual({"posix": 1, "open": 1}, called)
        finally:
            _restore_blacklist(saved)

    def test_runtime_patch_still_renders_whitelisted_templates(self):
        saved = _snapshot_blacklist()
        try:
            patch(default_black_list)
            template = """<%
import datetime
import json
import math
%>
${json.dumps({"n": math.sqrt(4)})}
${datetime.datetime.now()}
"""
            result = mako_render(template, {})
            self.assertIn('"n": 2.0', result)
            self.assertIn("20", result)
        finally:
            _restore_blacklist(saved)


if __name__ == "__main__":
    unittest.main()
