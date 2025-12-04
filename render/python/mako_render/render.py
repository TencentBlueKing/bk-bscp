# -*- coding: utf-8 -*-
"""
Mako template rendering core logic
参考原项目：bk-process-config-manager/apps/utils/mako_utils/render.py
"""

import sys
from typing import Dict, Any
from mako.template import Template
from mako.exceptions import MakoException, RichTraceback

from .checker import clean_mako_content, check_mako_template_safety
from .context import MakoSandbox
from .visitor import MakoNodeVisitor

# Template cache to avoid repeated compilation
TEMPLATE_CACHE = {}


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
    
    # 缓存 template，避免重复构造耗时
    template = TEMPLATE_CACHE.get(content)
    if not template:
        # 编译时安全检查（默认启用）
        # 通过 AST 访问器检查模板语法树，提前发现危险操作
        if enable_safety_check:
            # 安全检查失败时直接抛出异常，阻止模板编译
            # 这样可以提前发现危险代码，避免运行时拦截
            check_mako_template_safety(content, MakoNodeVisitor())
        
        template = Template(content)
        TEMPLATE_CACHE[content] = template
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
