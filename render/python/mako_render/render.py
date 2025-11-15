# -*- coding: utf-8 -*-
"""
Mako template rendering core logic
"""

import sys
from typing import Dict, Any
from mako.template import Template
from mako.exceptions import MakoException, RichTraceback


# Template cache to avoid repeated compilation
TEMPLATE_CACHE = {}


def get_cache_template(content: str) -> Template:
    """
    Get or create a cached Mako template
    
    Args:
        content: Template content string
        
    Returns:
        Compiled Mako Template object
    """
    template = TEMPLATE_CACHE.get(content)
    if not template:
        template = Template(content)
        TEMPLATE_CACHE[content] = template
    return template


def mako_render(content: str, context: Dict[str, Any]) -> str:
    """
    Render Mako template with given context
    
    Args:
        content: Template content string
        context: Dictionary containing template variables
        
    Returns:
        Rendered template string
        
    Raises:
        MakoException: If template rendering fails
    """
    template = get_cache_template(content)
    
    try:
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
        print(f"Unexpected error during rendering: {str(error)}", file=sys.stderr)
        raise
