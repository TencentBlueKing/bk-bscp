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

    def test_rejects_unsafe_expression_filters(self):
        cases = [
            '${"open(\\"/etc/hosts\\").read()" | eval}',
            '${"__import__(\\"os\\").system(\\"id\\")" | eval}',
            '${"/etc/hosts" | open}',
            '<%text filter="eval">1+1</%text>',
        ]

        for template in cases:
            with self.subTest(template=template):
                self.assert_unsafe(template)

    def test_allows_safe_expression_filters(self):
        result = mako_render('${"  <bscp>  " | h,trim}', {})

        self.assertEqual("&lt;bscp&gt;", result)

    def test_preserves_legacy_line_leading_double_percent_text(self):
        template = """%% 重要：修改之后，要同步给运维
  %% expend配置。格式必须是{K,V}
%%% 装饰注释
{cluster_id, "${bk_set_name}"}.
"""

        result = mako_render(template, {"bk_set_name": "20001"})

        self.assertEqual(
            """%% 重要：修改之后，要同步给运维
  % expend配置。格式必须是{K,V}
%% 装饰注释
{cluster_id, "20001"}.
""",
            result,
        )

    def test_preserves_legacy_first_line_triple_percent_text(self):
        template = """%%%中间件ws接口
{global, _Z_PORT_MIDDLE_WS, 8081}."""

        result = mako_render(template, {})

        self.assertEqual(
            """%%%中间件ws接口
{global, _Z_PORT_MIDDLE_WS, 8081}.""",
            result,
        )

    def test_legacy_percent_compat_does_not_touch_control_or_inline_percent(self):
        template = """% for item in ["a"]:
{rate, "100%%"}.
% endfor"""

        result = mako_render(template, {})

        self.assertEqual('{rate, "100%%"}.\n', result)

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

    def test_host_global_variable_overrides_same_set_variable(self):
        cc_xml = (
            '<?xml version="1.0" encoding="UTF-8"?>'
            '<Application><Set SetName="9701" game_vip="">'
            '<Module ModuleName="game">'
            '<Host InnerIP="30.49.244.164" bk_cloud_id="0" game_vip="117.62.240.28" />'
            '</Module></Set></Application>'
        )
        process = subprocess.run(
            [sys.executable, str(PYTHON_ROOT / "main.py"), "--stdin"],
            input=json.dumps(
                {
                    "template": "${game_vip}",
                    "context": {
                        "cc_xml": cc_xml,
                        "bk_set_name": "9701",
                        "bk_module_name": "game",
                        "bk_host_innerip": "30.49.244.164",
                        "bk_cloud_id": "0",
                        "biz_global_variables": {
                            "host": [{"bk_property_id": "game_vip"}],
                            "set": [{"bk_property_id": "game_vip"}],
                        },
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
        self.assertEqual("117.62.240.28", process.stdout)

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
            """<%
import random
random.shuffle(this.cc_set.attrib)
%>""",
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

    def test_allows_random_shuffle_on_local_list(self):
        template = """<%
import random
values = ["11.147.75.165", "11.147.75.28", "11.147.75.1"]
random.shuffle(values)
result = ",".join(sorted(values))
%>${result}"""

        self.assertTrue(check_mako_template_safety(template))
        result = mako_render(template, {})
        self.assertIn("11.147.75.1,11.147.75.165,11.147.75.28", result)

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

    def test_allows_template_helper_functions(self):
        template = """<%
def getAppId():
    if this.cc_set.attrib['bk_set_name'] == "prod":
        return ""
    return ""

def getMongoInfo():
    if this.cc_set.attrib['bk_set_name'] == "prod":
        return "mongodb://g"
    return "mongodb://gameus"
%>
${getAppId()}|${getMongoInfo()}"""

        self.assertTrue(check_mako_template_safety(template))

        this = SimpleNamespace(
            cc_set=SimpleNamespace(attrib={"bk_set_name": "prod"}),
        )
        result = mako_render(template, {"this": this})

        self.assertIn("|mongodb://g", result)

    def test_rejects_nested_template_function_def(self):
        template = """<%
def outer():
    def inner():
        return 1
%>"""

        self.assert_unsafe(template)

    def test_rejects_decorated_template_function_def(self):
        template = """<%
def helper():
    return 1

@decorator
def wrapped():
    return 2
%>"""

        self.assert_unsafe(template)

    def test_rejects_unsafe_code_inside_template_function(self):
        template = """<%
def evil():
    return open("/etc/passwd").read()
%>"""

        self.assert_unsafe(template)

    def test_function_local_assignment_does_not_clear_outer_module_binding(self):
        template = """<%
import json

def helper():
    json = {}
    return "ok"
%>
${json.dumps({"name": "bscp"})}"""

        result = mako_render(template, {})

        self.assertIn('"name": "bscp"', result)

    def test_function_local_import_does_not_create_outer_module_binding(self):
        template = """<%
def helper():
    import json
    return json.dumps({"name": "bscp"})
%>
${json.dumps({"name": "bscp"})}"""

        self.assert_unsafe(template)

    def test_rejects_module_call_after_helper_rebinds_module_name(self):
        template = """<%
import json

def json():
    return "helper"
%>
${json.dumps({"name": "bscp"})}"""

        self.assert_unsafe(template)

    def test_allows_helper_to_call_later_helper_in_same_code_block(self):
        template = """<%
def first():
    return second()

def second():
    return "helper"
%>
${first()}"""

        result = mako_render(template, {})

        self.assertIn("helper", result)

    def test_allows_helper_to_use_later_module_import_in_same_code_block(self):
        template = """<%
def helper():
    return json.dumps({"name": "bscp"})

import json
%>
${helper()}"""

        result = mako_render(template, {})

        self.assertIn('"name": "bscp"', result)

    def test_allows_helper_to_use_later_from_import_in_same_code_block(self):
        template = """<%
def helper():
    return to_json({"name": "bscp"})

from json import dumps as to_json
%>
${helper()}"""

        result = mako_render(template, {})

        self.assertIn('"name": "bscp"', result)

    def test_rejects_helper_later_module_import_rebound_in_same_code_block(self):
        template = """<%
def helper():
    return json.dumps({"name": "bscp"})

import json
json = attacker
%>
${helper()}"""

        self.assert_unsafe(template)

    def test_rejects_helper_import_inside_python_control_block(self):
        template = """<%
def helper():
    return json.dumps({"name": "bscp"})

if flag:
    import json
%>
${helper()}"""

        self.assert_unsafe(template)

    def test_rejects_helper_call_before_later_import_in_same_code_block(self):
        template = """<%
def helper():
    return json.dumps({"name": "bscp"})

value = helper()
import json
%>
${value}"""

        self.assert_unsafe(template)

    def test_allows_helper_call_after_later_import_in_same_code_block(self):
        template = """<%
def helper():
    return json.dumps({"name": "bscp"})

import json
value = helper()
%>
${value}"""

        result = mako_render(template, {})

        self.assertIn('"name": "bscp"', result)

    def test_rejects_helper_call_to_rebound_later_helper_name(self):
        template = """<%
def first():
    return second()

def second():
    return "helper"

second = attacker
%>
${first()}"""

        self.assert_unsafe(template)

    def test_rejects_helper_call_before_later_helper_in_same_code_block(self):
        template = """<%
def first():
    return second()

value = first()

def second():
    return "helper"
%>
${value}"""

        self.assert_unsafe(template)

    def test_rejects_call_after_later_import_rebinds_helper_name(self):
        template = """<%
def json():
    return "helper"

import json
%>
${json()}"""

        self.assert_unsafe(template)

    def test_rejects_module_attr_after_from_import_rebinds_module_name(self):
        template = """<%
import json
from math import sqrt as json
%>
${json.dumps({"name": "bscp"})}"""

        self.assert_unsafe(template)

    def test_rejects_helper_defined_inside_mako_if_block(self):
        template = """% if flag:
<%
def helper():
    return "helper"
%>
% endif
${helper()}"""

        self.assert_unsafe(template)

    def test_allows_helper_defined_inside_mako_if_block_for_inner_nodes(self):
        template = """% if flag:
<%
def helper():
    return "helper"
%>
${helper()}
% endif"""

        result = mako_render(template, {"flag": True})

        self.assertIn("helper", result)

    def test_allows_ginclude_helper_style_inside_mako_control_block(self):
        from lxml import etree

        template = '''% if flag:
#Ginclude "常量配置"
<%
  def get_DSN(db_ip, set_id, db_pwd):
      set_id = set_id.lstrip("s")
      if SET_MSG[bk_set_name]['version'] == "live":
        return "DSN(%s) = \\"DRIVER={SQL Server};SERVER=%s;DATABASE=Lin2World_%s;UID=syncmaster;PWD=%s\\"" %(set_id, db_ip, set_id, db_pwd)
      return "DSN(%s) = \\"DRIVER={SQL Server Native Client 10.0};SERVER=%s;DATABASE=Lin2World_%s;UID=syncmaster;PWD=%s\\"" %(set_id, db_ip, set_id, db_pwd)

  def get_ip(set_id, module_name):
      for host in cc.findall('Set[@SetName="%s"]/Module[@ModuleName="%s"]/Host' % (set_id, module_name)):
          server_ip = host.attrib['bk_host_innerip']
          return server_ip
%>
${get_DSN("127.0.0.1", "s1001", "pwd")}
% endif'''

        result = mako_render(
            template,
            {
                "flag": True,
                "SET_MSG": {"991": {"version": "live"}},
                "bk_set_name": "991",
                "cc": etree.Element("Application"),
            },
        )

        self.assertIn("SQL Server", result)

    def test_rejects_helper_defined_inside_mako_for_block(self):
        template = """% for item in items:
<%
def helper():
    return "helper"
%>
% endfor
${helper()}"""

        self.assert_unsafe(template)

    def test_allows_import_defined_inside_mako_if_block_for_inner_nodes(self):
        template = """% if flag:
<%
import json
%>
${json.dumps({"name": "bscp"})}
% endif"""

        result = mako_render(template, {"flag": True})

        self.assertIn('"name": "bscp"', result)

    def test_rejects_import_defined_inside_mako_if_block_leaking_out(self):
        template = """% if flag:
<%
import json
%>
% endif
${json.dumps({"name": "bscp"})}"""

        self.assert_unsafe(template)

    def test_rejects_mako_control_rebind_of_outer_module_binding(self):
        template = """<%
import json
%>
% if flag:
<%
json = attacker
%>
% endif
${json.dumps({"name": "bscp"})}"""

        self.assert_unsafe(template)

    def test_rejects_helper_defined_inside_python_control_block(self):
        template = """<%
if flag:
    def helper():
        return "helper"
%>
${helper()}"""

        self.assert_unsafe(template)

    def test_rejects_unsafe_template_function_annotations(self):
        cases = [
            """<%
def helper(value: open("/etc/passwd").read()):
    return "ok"
%>""",
            """<%
def helper() -> open("/etc/passwd").read():
    return "ok"
%>""",
        ]

        for template in cases:
            with self.subTest(template=template):
                self.assert_unsafe(template)

    def test_rejects_call_before_template_function_def(self):
        template = """${getAppId()}
<%
def getAppId():
    return ""
%>"""

        self.assert_unsafe(template)

    def test_rejects_rebound_template_function_name(self):
        template = """<%
def getAppId():
    return "safe"
getAppId = attacker
%>
${getAppId()}"""

        self.assert_unsafe(template)

    def test_allows_extended_whitelist_calls(self):
        cases = [
            ('${"a,b,c".split(",")}', {}, "a"),
            ('${"prod".startswith("p")}', {}, "True"),
            ('${"9".isdigit()}', {}, "True"),
            ('${"  x  ".strip()}', {}, "x"),
            ('${"{}{}{}".format("a", "-", "b")}', {}, "a-b"),
            ('${",".join(["a", "b"])}', {}, "a,b"),
            ('${"  x".lstrip()}', {}, "x"),
            ('${list(zip([1, 2], [3, 4]))}', {}, "1"),
            ('${"abc".index("b")}', {}, "1"),
            ('${sorted([3, 1, 2])}', {}, "1"),
            ('${len(set([1, 2, 1]))}', {}, "2"),
            ('<% items = set(); items.add("a") %>${",".join(sorted(items))}', {}, "a"),
            ('${",".join(map(str, [1, 2]))}', {}, "1,2"),
        ]

        for template, context, expected in cases:
            with self.subTest(template=template):
                self.assertTrue(check_mako_template_safety(template))
                result = mako_render(template, context)
                self.assertIn(expected, result)

    def test_allows_mako_loop_index_attribute(self):
        template = """% for item in ["a", "b"]:
${loop.index}:${item}
% endfor"""
        self.assertTrue(check_mako_template_safety(template))

    def test_allows_zonelist_helper_whitelist_calls(self):
        template = r"""<%
def get_boolean(key, set_name, default=None):
    value = cc_set.attrib.get(key)
    if value.lower() == 'true':
        result = True
    elif value.lower() == 'false':
        result = False
    else:
        result = default
    return result

d = {}
hutongfu_list = [1803]
hutongfu_list.extend(list(range(1803, 1805)))
d.setdefault("1001", {}).update({"id": 1001, "name": "srv"})
import json
data = {"servers": sorted(d.values(), key=lambda i: i["id"])}
%>${json.dumps(data, indent=4, ensure_ascii=False)}"""

        self.assertTrue(check_mako_template_safety(template))

    def test_allows_nested_subscript_assignment_on_local_containers(self):
        template = """<%
config = {}
config["l2"] = {}
config["l2"]["port"] = "1001"
items = [0, 1]
items[0] = 2
%>${config["l2"]["port"]}:${items[0]}"""

        self.assertTrue(check_mako_template_safety(template))
        result = mako_render(template, {})
        self.assertIn("1001:2", result)

    def test_allows_subscript_assignment_on_local_xml_element_attrib(self):
        from lxml import etree

        cc = etree.fromstring("<Application><Set><Module><Host InnerIP='127.0.0.1'/></Module></Set></Application>")
        template = """<%
game_ins = cc.findall(".//Host")[0]
game_ins.attrib["bk_set_name"] = world_name
%>${game_ins.attrib["InnerIP"]}:${game_ins.attrib["bk_set_name"]}"""

        self.assertTrue(check_mako_template_safety(template))
        result = mako_render(template, {"cc": cc, "world_name": "world-1"})
        self.assertIn("127.0.0.1:world-1", result)

    def test_allows_render_diff_tail_whitelist_calls(self):
        from lxml import etree

        cases = [
            (
                """<%
import re
m = re.search(r"(\\d+)", "svr12")
result = m.group(1) if m else ""
%>${result}""",
                {},
                "12",
            ),
            (
                """<%
from datetime import datetime
result = datetime(2020, 1, 1).strftime("%Y-%m-%d")
%>${result}""",
                {},
                "2020-01-01",
            ),
            (
                """<%
import datetime
now = datetime.datetime.now
result = now().strftime("ok")
%>${result}""",
                {},
                "ok",
            ),
            (
                """<%
from datetime import datetime
now = datetime.now
result = now().strftime("ok")
%>${result}""",
                {},
                "ok",
            ),
            (
                """% if flag:
<%
from datetime import datetime
%>
% endif
${datetime.now().strftime("ok")}""",
                {"flag": True},
                "ok",
            ),
            (
                """<%
from datetime import datetime
dt = datetime(2020, 1, 1)
result = int(dt.timestamp())
%>${result}""",
                {},
                "1577808000",
            ),
            (
                """<%
items = [{"name": "B"}, {"name": "a"}]
items.sort(key=lambda x: x["name"].lower())
result = ",".join([item["name"] for item in items])
%>${result}""",
                {},
                "a,B",
            ),
            (
                """<%
result = ",".join([item.attrib["SetName"] for item in sorted(cc.xpath("Set"), key=lambda x: int(x.attrib["SetName"]))])
%>${result}""",
                {"cc": etree.fromstring('<Application><Set SetName="10"/><Set SetName="2"/></Application>')},
                "2,10",
            ),
            (
                """<%
format_num = lambda x: "0" + str(x) if x < 10 else str(x)
result = ",".join([format_num(3), format_num(12)])
%>${result}""",
                {},
                "03,12",
            ),
            (
                """<%
rs_host_tmpl0 = "game{}.%s%02d.lzjd.db:27017"
result = ",".join(map(lambda x: rs_host_tmpl0.format(x), ("P", "S")))
%>${result}""",
                {},
                "gameP.%s%02d.lzjd.db:27017,gameS.%s%02d.lzjd.db:27017",
            ),
            (
                """<%
items = [3, 1, 2]
items.sort()
result = ",".join([str(x) for x in items])
%>${result}""",
                {},
                "1,2,3",
            ),
        ]
        for template, context, expected in cases:
            with self.subTest(template=template):
                self.assertTrue(check_mako_template_safety(template))
                result = mako_render(template, context)
                self.assertIn(expected, result)

        self.assertTrue(check_mako_template_safety('<% x = {"a": 1}.items() %>'))

    def test_allows_append_and_xpath_in_template_helper(self):
        template = """<%
def build_items():
    items = []
    items.append("a")
    items.append("b")
    return items[0] + "," + items[1]
%>
${build_items()}"""

        self.assertTrue(check_mako_template_safety(template))
        result = mako_render(template, {})
        self.assertIn("a,b", result)

        from lxml import etree

        xpath_template = '${this.cc_host.xpath("string(@InnerIP)")}'
        self.assertTrue(check_mako_template_safety(xpath_template))
        this = SimpleNamespace(cc_host=etree.Element("Host", InnerIP="127.0.0.1"))
        result = mako_render(xpath_template, {"this": this})
        self.assertIn("127.0.0.1", result)

    def test_rejects_unsafe_calls_after_whitelist_extension(self):
        cases = [
            '${open("/etc/passwd").read()}',
            '${"a".endswith("a")}',
        ]

        for template in cases:
            with self.subTest(template=template):
                self.assert_unsafe(template)

    def test_allows_legacy_template_control_flow(self):
        template = """<%
def get_prefix(set_name):
    return '{}-{}'.format("a", "b")

try:
    value = "ok"
except:
    value = "fallback"

items = [x for x in ["a", "b"]]
config = {}
config['key'] = value
%>
${get_prefix("1001")}|${",".join(items)}|${config['key']}"""

        self.assertTrue(check_mako_template_safety(template))
        result = mako_render(template, {})
        self.assertIn("a-b", result)
        self.assertIn("a,b", result)
        self.assertIn("ok", result)

    def test_allows_datetime_strptime_in_template_helper(self):
        template = """<%
from datetime import datetime

def get_opentime(value):
    return datetime.strptime(value, "%Y-%m-%d %H:%M:%S")

result = "ok"
try:
    result = get_opentime("2020-01-01 00:00:00")
except ValueError:
    result = "bad"
%>
${result}"""

        self.assertTrue(check_mako_template_safety(template))
        result = mako_render(template, {})
        self.assertIn("2020-01-01", result)

    def test_allows_raise_exception_in_safety_check_only(self):
        template = """<%
raise Exception("need config")
%>"""
        self.assertTrue(check_mako_template_safety(template))

    def test_allows_raise_type_error_in_safety_check(self):
        template = """<%
flag = False
if flag:
    raise TypeError("set_name type error")
%>ok"""

        self.assertTrue(check_mako_template_safety(template))
        self.assertEqual("ok", mako_render(template, {}))

    def test_allows_legacy_raise_exception_percent_format(self):
        template = """<%
flag = False
set_name = "1001"
sc_count = 3
if flag:
    raise Exception("invalid sc count SetName:%s count:%s") % (set_name, sc_count)
%>ok"""

        self.assertTrue(check_mako_template_safety(template))
        self.assertEqual("ok", mako_render(template, {}))

    def test_allows_lambda_body_with_common_template_syntax(self):
        """lambda 体沿用模板其它位置的白名单，兼容 GSEKit 历史模板写法。"""
        cases = [
            (
                """<%
zones = [{"zonenum": 3}, {"zonenum": 1}]
zones.sort(key=lambda k: (k.get("zonenum")))
result = ",".join([str(z["zonenum"]) for z in zones])
%>${result}""",
                "1,3",
            ),
            (
                """<%
zones = [{"id": 1}, {"id": 3}]
result = ",".join([str(z["id"]) for z in sorted(zones, key=lambda z: -z["id"])])
%>${result}""",
                "3,1",
            ),
            (
                """<%
names = ["bbb", "a"]
result = ",".join(sorted(names, key=lambda n: len(n)))
%>${result}""",
                "a,bbb",
            ),
            (
                """<%
items = [{"w": 2}, {"w": 0}]
result = ",".join([str(i["w"]) for i in sorted(items, key=lambda i: i.get("w") or 0)])
%>${result}""",
                "0,2",
            ),
            (
                """<%
names = [" b ", "a"]
result = ",".join(sorted(names, key=lambda n: n.strip()))
%>${result}""",
                "a, b ",
            ),
            (
                """<%
field = "id"
rows = [{"id": 2}, {"id": 1}]
result = ",".join([str(r["id"]) for r in sorted(rows, key=lambda r: r[field])])
%>${result}""",
                "1,2",
            ),
            (
                """<%
result = ",".join(map(lambda i: f"n{i}", [1, 2]))
%>${result}""",
                "n1,n2",
            ),
            (
                """<%
result = ",".join(map(lambda n: n.replace("a", "b"), ["aa"]))
%>${result}""",
                "bb",
            ),
        ]

        for template, expected in cases:
            with self.subTest(template=template):
                self.assertTrue(check_mako_template_safety(template))
                self.assertIn(expected, mako_render(template, {}))

    def test_rejects_unsafe_lambda_and_dict_methods_abuse(self):
        cases = [
            '<% x = sorted([1], key=lambda: open("/etc/passwd")) %>',
            '<% x = sorted([1], key=lambda a, b: a) %>',
            '<% x = sorted([1], key=lambda i: __import__("os")) %>',
            '<% x = sorted([1], key=lambda i: open("/etc/passwd") if i < 2 else str(i)) %>',
            '<% x = sorted([1], key=lambda i: i.__class__) %>',
            '<% x = sorted([1], key=lambda i: eval("1")) %>',
            '<% x = sorted([1], key=lambda i: i.system("ls")) %>',
            '<% x = sorted([1], key=lambda i: globals()) %>',
            """<%
import datetime
x = sorted([1], key=lambda datetime: datetime.datetime.now())
%>""",
            '<% this.cc_set.attrib.update({"k": "v"}) %>',
            '<% this.cc_set.attrib["k"] = "v" %>',
            '<% cc[0]["k"] = "v" %>',
            """<%
import datetime
datetime["k"] = "v"
%>""",
            """<%
import datetime
now = datetime.datetime.now
now = dangerous
%>${now()}""",
        ]

        for template in cases:
            with self.subTest(template=template):
                self.assert_unsafe(template)

    def test_rejects_unsafe_try_and_raise_after_control_flow_extension(self):
        cases = [
            """<%
try:
    open("/etc/passwd").read()
except:
    pass
%>""",
            """<%
raise OSError("deny")
%>""",
            """<%
nested = [x for x in [y for y in range(2)]]
%>""",
            """<%
this.cc_set.attrib['k'] = "v"
%>""",
        ]

        for template in cases:
            with self.subTest(template=template):
                self.assert_unsafe(template)

    def test_allows_help_template_safety_check(self):
        from main import HELP_TEMPLATE

        self.assertTrue(check_mako_template_safety(HELP_TEMPLATE))

    def test_rejects_exec_only_expressions_that_still_run_inside_mako(self):
        """赋值、分号、import 不是合法 eval，但 Mako 仍会把原文放进 str() 执行。"""
        cases = [
            '${x = open("/etc/passwd")}',
            '${\nx = open("/etc/passwd")\n}',
            '${1; open("/etc/passwd")}',
            "${\nimport posix\n}",
        ]

        for template in cases:
            with self.subTest(template=template):
                self.assert_unsafe(template)

    def test_allows_parenthesized_multiline_expression(self):
        template = """${
",".join([
    "a",
    "b",
])
}"""

        self.assertTrue(check_mako_template_safety(template))
        self.assertEqual("a,b", mako_render(template, {}))

    def test_allows_existing_control_lines(self):
        template = """% for idx, item in enumerate(items):
${idx}:${item}
% endfor
% if flag:
ok
% endif"""

        result = mako_render(template, {"items": ["x"], "flag": True})

        self.assertIn("0:x", result)
        self.assertIn("ok", result)


if __name__ == "__main__":
    unittest.main()
