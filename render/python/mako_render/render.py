# -*- coding: utf-8 -*-
"""
Mako template rendering core logic
参考原项目：bk-process-config-manager/apps/utils/mako_utils/render.py
"""

import hashlib
import os
import sys
from collections import OrderedDict
from collections.abc import Mapping
from types import SimpleNamespace
from typing import Dict, Any

from lxml import etree
from mako.template import Template
from mako.exceptions import MakoException, RichTraceback

from .checker import clean_mako_content, check_mako_template_safety
from .context import MakoSandbox
from .exceptions import ForbiddenMakoTemplateException
from .visitor import MakoNodeVisitor

SAFE_CONTEXT_SCALAR_TYPES = (str, bytes, int, float, bool, type(None))
SAFE_CONTEXT_SEQUENCE_TYPES = (list, tuple, set, frozenset)

# 默认缓存预算(MB)，按模板原文字节计；编译后的实际驻留内存约为其 2.7 倍
DEFAULT_TEMPLATE_CACHE_MB = 32
TEMPLATE_CACHE_MB_ENV = "BSCP_RENDER_TEMPLATE_CACHE_MB"


def _template_cache_max_bytes() -> int:
    try:
        mb = int(os.environ.get(TEMPLATE_CACHE_MB_ENV, ""))
    except ValueError:
        mb = 0
    if mb <= 0:
        mb = DEFAULT_TEMPLATE_CACHE_MB
    return mb * 1024 * 1024


# 模板缓存，避免重复编译。worker 进程常驻，缓存会随模板种类持续累积，
# 故按模板原文字节数做 LRU 上限。淘汰只能阻止继续增长，已分配的内存不会归还 OS，
# RSS 回落依赖 Go 侧按常驻内存重建 worker 进程。
TEMPLATE_CACHE_MAX_BYTES = _template_cache_max_bytes()
# key 为模板内容摘要，value 为 (template, 模板原文字节数)
TEMPLATE_CACHE = OrderedDict()
_TEMPLATE_CACHE_BYTES = 0


def _cache_put(key: bytes, template: Template, size: int):
    """写入缓存并按字节预算淘汰最久未使用的模板"""
    global _TEMPLATE_CACHE_BYTES

    if size > TEMPLATE_CACHE_MAX_BYTES:
        # 单个模板就超过预算，缓存它会挤空整个缓存，直接不缓存
        return

    TEMPLATE_CACHE[key] = (template, size)
    _TEMPLATE_CACHE_BYTES += size
    while _TEMPLATE_CACHE_BYTES > TEMPLATE_CACHE_MAX_BYTES:
        _, (_, evicted_size) = TEMPLATE_CACHE.popitem(last=False)
        _TEMPLATE_CACHE_BYTES -= evicted_size


def _validate_context_value(value: Any, path: str, seen: set):
    if isinstance(value, SAFE_CONTEXT_SCALAR_TYPES):
        return
    if callable(value):
        raise ForbiddenMakoTemplateException("发现非法上下文可调用对象:[{}]，请修改".format(path))

    value_id = id(value)
    if value_id in seen:
        return
    seen.add(value_id)

    if isinstance(value, Mapping):
        for key, item in value.items():
            _validate_context_value(item, "{}[{}]".format(path, repr(key)), seen)
        return

    if isinstance(value, SAFE_CONTEXT_SEQUENCE_TYPES):
        for index, item in enumerate(value):
            _validate_context_value(item, "{}[{}]".format(path, index), seen)
        return

    if isinstance(value, SimpleNamespace):
        for key, item in vars(value).items():
            _validate_context_value(item, "{}.{}".format(path, key), seen)
        return

    if isinstance(value, etree._Element):
        return

    raise ForbiddenMakoTemplateException("发现非法上下文对象:[{}]，请修改".format(path))


def validate_context_safety(context: Dict[str, Any]):
    _validate_context_value(context, "context", set())


def get_cache_template(content: str, enable_safety_check: bool = True) -> Template:
    """
    Get or create a cached Mako template
    参考原项目：bk-process-config-manager/apps/utils/mako_utils/render.py
    
    Args:
        content: Template content string
        enable_safety_check: 是否启用编译时安全检查（默认 True，启用安全检查）
        
    Returns:
        Compiled Mako Template object
        
    Raises:
        ForbiddenMakoTemplateException: 如果启用了安全检查且模板不安全
    """
    # 清理模板内容（替换制表符为空格）
    content = clean_mako_content(content)
    encoded = content.encode("utf-8")
    # 用摘要作为 key，避免缓存里再存一份模板全文
    key = hashlib.sha256(encoded).digest()

    # 缓存 template，避免重复构造耗时
    cached = TEMPLATE_CACHE.get(key)
    if cached is not None:
        TEMPLATE_CACHE.move_to_end(key)
        return cached[0]

    # 编译时安全检查（默认启用）
    # 通过 AST 访问器检查模板语法树，提前发现危险操作
    if enable_safety_check:
        # 安全检查失败时直接抛出异常，阻止模板编译
        # 这样可以提前发现危险代码，避免运行时拦截
        check_mako_template_safety(content, MakoNodeVisitor())

    template = Template(content)
    _cache_put(key, template, len(encoded))
    return template


def mako_render(content: str, context: Dict[str, Any], enable_safety_check: bool = True) -> str:
    """
    Render Mako template with given context
    参考原项目：bk-process-config-manager/apps/utils/mako_utils/render.py
    
    Args:
        content: Template content string
        context: Dictionary containing template variables
        enable_safety_check: 是否启用编译时安全检查（默认 True，启用安全检查）
        
    Returns:
        Rendered template string
        
    Raises:
        Exception: If template rendering fails
    """
    validate_context_safety(context)
    template = get_cache_template(content, enable_safety_check=enable_safety_check)
    try:
        # 使用 MakoSandbox 上下文管理器来跟踪用户代码执行
        # 配合 patch.py 中的运行时拦截机制，提供双重安全保护
        with MakoSandbox():
            return template.render(**context)
    except MakoException as error:
        # Print detailed error traceback
        traceback = RichTraceback()
        for (filename, lineno, function, line) in traceback.traceback:
            print(f"File {filename}, line {lineno}, in {function}", file=sys.stderr)
            print(f"  {line}", file=sys.stderr)
        print(f"{traceback.error.__class__.__name__}: {traceback.error}", file=sys.stderr)
        raise Exception(f"Mako render failed: {str(error)}")
    except Exception as error:
        # Print detailed error traceback for non-Mako exceptions
        # RichTraceback() 只适用于 Mako 异常，对于非 Mako 异常需要特殊处理
        error_message = str(error)
        try:
            traceback = RichTraceback()
            error_message = traceback.message
            for _traceback in traceback.traceback:
                __, lineno, function, line = _traceback
                if function == "render_body":
                    error_message = f"第{lineno}行：{line}，错误：{traceback.message}"
        except (AttributeError, TypeError, Exception):
            # 如果 RichTraceback() 不可用或失败，使用标准异常信息
            # 这通常发生在非 Mako 异常的情况下
            error_message = f"{error.__class__.__name__}: {str(error)}"
        print(f"Error: {error_message}", file=sys.stderr)
        raise Exception(f"Mako render failed: {error_message}")
