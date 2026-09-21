# -*- coding: utf-8 -*-

import sys
import unittest
from pathlib import Path

PYTHON_ROOT = Path(__file__).resolve().parents[1]
if str(PYTHON_ROOT) not in sys.path:
    sys.path.insert(0, str(PYTHON_ROOT))

from mako_render import render as render_mod
from mako_render.render import get_cache_template


def make_template(index: int, size: int) -> str:
    """构造指定字节数、内容互不相同的模板"""
    head = "${name}" + str(index) + "-"
    return head + "x" * (size - len(head))


class TemplateCacheTest(unittest.TestCase):
    def setUp(self):
        self._origin_max_bytes = render_mod.TEMPLATE_CACHE_MAX_BYTES
        self.reset_cache()

    def tearDown(self):
        render_mod.TEMPLATE_CACHE_MAX_BYTES = self._origin_max_bytes
        self.reset_cache()

    def reset_cache(self):
        render_mod.TEMPLATE_CACHE.clear()
        render_mod._TEMPLATE_CACHE_BYTES = 0

    def test_reuses_compiled_template(self):
        content = make_template(1, 100)

        self.assertIs(get_cache_template(content), get_cache_template(content))
        self.assertEqual(len(render_mod.TEMPLATE_CACHE), 1)

    def test_evicts_until_within_budget(self):
        render_mod.TEMPLATE_CACHE_MAX_BYTES = 300

        for i in range(10):
            get_cache_template(make_template(i, 100))

        self.assertEqual(len(render_mod.TEMPLATE_CACHE), 3)
        self.assertLessEqual(render_mod._TEMPLATE_CACHE_BYTES, 300)

    def test_evicts_least_recently_used(self):
        render_mod.TEMPLATE_CACHE_MAX_BYTES = 300
        first, second, third = (make_template(i, 100) for i in range(3))
        for content in (first, second, third):
            get_cache_template(content)

        # 命中 first 使其成为最近使用，再写入新模板应淘汰 second
        cached_first = get_cache_template(first)
        get_cache_template(make_template(3, 100))

        self.assertIs(get_cache_template(first), cached_first)
        self.assertEqual(len(render_mod.TEMPLATE_CACHE), 3)

    def test_skips_template_larger_than_budget(self):
        render_mod.TEMPLATE_CACHE_MAX_BYTES = 300
        get_cache_template(make_template(0, 100))

        get_cache_template(make_template(1, 500))

        self.assertEqual(len(render_mod.TEMPLATE_CACHE), 1)
        self.assertEqual(render_mod._TEMPLATE_CACHE_BYTES, 100)


if __name__ == "__main__":
    unittest.main()
