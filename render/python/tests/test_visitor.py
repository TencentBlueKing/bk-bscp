# -*- coding: utf-8 -*-

import json
import subprocess
import sys
import unittest
from pathlib import Path
from types import SimpleNamespace

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
            '${open("/etc/passwd").read().replace("a", "b")}',
            '${__import__("os").system("id").replace("x", "y")}',
            '${json.system("id")}',
        ]

        for template in cases:
            with self.subTest(template=template):
                self.assert_unsafe(template)

    def test_rejects_calls_on_rebound_allowed_module_names(self):
        cases = [
            """<%
import json
json = attacker
%>
${json.system("id")}""",
            """<%
import json as safe_json
safe_json = attacker
%>
${safe_json.system("id")}""",
        ]

        for template in cases:
            with self.subTest(template=template):
                self.assert_unsafe(template)

    def test_allows_assignment_rhs_to_use_current_module_binding(self):
        template = """<%
import json
json = json.dumps({"name": "bscp"})
%>
${json.replace("bscp", "BSCP")}"""

        result = mako_render(template, {})

        self.assertIn('"name": "BSCP"', result)

    def test_rejects_module_calls_after_assignment_rebinds_name(self):
        template = """<%
import json
json = json.dumps({"name": "bscp"})
%>
${json.dumps({"name": "bscp"})}"""

        self.assert_unsafe(template)

    def test_allows_for_iter_to_use_current_module_binding(self):
        template = """<%
import json
%>
% for json in [json.dumps({"name": "bscp"})]:
${json.replace("bscp", "BSCP")}
% endfor"""

        result = mako_render(template, {})

        self.assertIn('"name": "BSCP"', result)

    def test_rejects_module_calls_after_for_target_rebinds_name(self):
        template = """<%
import json
%>
% for json in [json.dumps({"name": "bscp"})]:
${json.dumps({"name": "bscp"})}
% endfor"""

        self.assert_unsafe(template)

    @unittest.skipIf(sys.version_info < (3, 10), "pattern matching requires Python 3.10+")
    def test_rejects_pattern_matching_rebound_allowed_module_names(self):
        template = """<%
import json
match attacker:
    case _ as json:
        pass
%>
${json.dumps("id")}"""

        self.assert_unsafe(template)

    def test_rejects_callable_context_values(self):
        context = {
            "obj": SimpleNamespace(get=lambda _: "unsafe"),
        }

        with self.assertRaises(ForbiddenMakoTemplateException):
            mako_render('${obj.get("id")}', context)

    def test_rejects_cc_xml_with_doctype(self):
        cc_xml = '<!DOCTYPE root [<!ENTITY xxe "INTERNAL">]><root>&xxe;</root>'
        process = subprocess.run(
            [sys.executable, str(PYTHON_ROOT / "main.py"), "--stdin"],
            input=json.dumps({"template": "ok", "context": {"cc_xml": cc_xml}}),
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            check=False,
        )

        self.assertNotEqual(0, process.returncode)
        self.assertIn("DOCTYPE", process.stderr)

    def test_allows_normal_cc_xml_context(self):
        cc_xml = (
            '<?xml version="1.0" encoding="UTF-8"?>'
            '<Application><Set SetName="qq"><Module ModuleName="gamesvr">'
            '<Host InnerIP="127.0.0.1" bk_cloud_id="0" />'
            '</Module></Set></Application>'
        )
        process = subprocess.run(
            [sys.executable, str(PYTHON_ROOT / "main.py"), "--stdin"],
            input=json.dumps(
                {
                    "template": '${this.cc_host.attrib.get("InnerIP")}',
                    "context": {
                        "cc_xml": cc_xml,
                        "bk_set_name": "qq",
                        "bk_module_name": "gamesvr",
                        "bk_host_innerip": "127.0.0.1",
                        "bk_cloud_id": "0",
                    },
                }
            ),
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            check=False,
        )

        self.assertEqual("", process.stderr)
        self.assertEqual(0, process.returncode)
        self.assertEqual("127.0.0.1", process.stdout)

    def test_rejects_unapproved_allowed_module_members(self):
        cases = [
            """<%
import random
%>
${random._os.open("/etc/hosts", 0)}""",
            """<%
import random
%>
${random._os.read(0, 10)}""",
            """<%
import json
%>
${json.tool}""",
            """<%
from random import _os
%>
${_os.open("/etc/hosts", 0)}""",
            """<%
from json import tool
%>
${tool}""",
            """<%
import json.tool as json_tool
%>
${json_tool.dumps({"name": "bscp"})}""",
            """<%
from json.tool import dumps
%>
${dumps({"name": "bscp"})}""",
            """<%
import math
%>
${math.__dict__}""",
        ]

        for template in cases:
            with self.subTest(template=template):
                self.assert_unsafe(template)

    def test_allows_explicit_safe_module_members(self):
        template = """<%
import datetime
import json
import math
import random
import re
%>
${datetime.datetime.now()}
${json.dumps({"name": "bscp"})}
${math.sqrt(4)}
${random.randint(1, 1)}
${re.compile("bs")}
"""

        result = mako_render(template, {})

        self.assertIn('"name": "bscp"', result)
        self.assertIn("2.0", result)
        self.assertIn("1", result)
        self.assertIn("bs", result)

    def test_allows_safe_members_imported_from_allowed_modules(self):
        template = """<%
from math import sqrt
from json import dumps as to_json
%>
${sqrt(9)}
${to_json({"name": "bscp"})}
"""

        result = mako_render(template, {})

        self.assertIn("3.0", result)
        self.assertIn('"name": "bscp"', result)

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
