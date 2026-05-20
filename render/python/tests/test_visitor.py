# -*- coding: utf-8 -*-

import sys
import unittest
from pathlib import Path

PYTHON_ROOT = Path(__file__).resolve().parents[1]
if str(PYTHON_ROOT) not in sys.path:
    sys.path.insert(0, str(PYTHON_ROOT))

from mako_render import mako_render
from mako_render.checker import check_mako_template_safety
from mako_render.exceptions import ForbiddenMakoTemplateException


class MakoSafetyTest(unittest.TestCase):
    def assert_unsafe(self, template):
        with self.assertRaises(ForbiddenMakoTemplateException):
            check_mako_template_safety(template)

    def test_rejects_unsafe_template_features(self):
        cases = [
            '${__import__("os").system("id")}',
            '${().__class__.__mro__[1].__subclasses__()}',
            '${sorted([2, 1])}',
            '${getattr(this, "cc_host", None)}',
            '${open("/etc/passwd").read()}',
        ]

        for template in cases:
            with self.subTest(template=template):
                self.assert_unsafe(template)

    def test_allows_business_template_features(self):
        template = """Hello ${name}
${data.get("role", "none")}
${text.replace("a", "b")}
% for idx, item in enumerate(items):
${idx}:${item}
% endfor"""

        result = mako_render(
            template,
            {
                "name": "BSCP",
                "data": {"role": "server"},
                "text": "a-a",
                "items": ["x", "y"],
            },
        )

        self.assertIn("Hello BSCP", result)
        self.assertIn("server", result)
        self.assertIn("b-b", result)
        self.assertIn("0:x", result)
        self.assertIn("1:y", result)

    def test_allows_help_template_safety_check(self):
        from main import HELP_TEMPLATE

        self.assertTrue(check_mako_template_safety(HELP_TEMPLATE))


if __name__ == "__main__":
    unittest.main()
