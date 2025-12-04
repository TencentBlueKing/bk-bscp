#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Main entry point for Mako template rendering
Supports reading context from stdin or file and rendering templates
"""

import sys
import json
import argparse
from pathlib import Path

# Add current directory to Python path
sys.path.insert(0, str(Path(__file__).parent))

from types import SimpleNamespace
from lxml import etree
from mako_render import mako_render
from mako_render.patch import patch, default_black_list

# HELP_TEMPLATE 用于生成帮助文档，显示所有可用的变量和对象
# 参考 Python 代码：config_version.py 第 37-141 行
HELP_TEMPLATE = """
<%
import datetime
%>
***********************************
* NOW: ${datetime.datetime.now()} *
***********************************

********************
* Global Variables *
********************

% for k, v in global_variables.items():
    % if k == "global_variables":
        <% continue %>
    % endif
    % if k == "cc_xml":
        <% continue %>
    % endif
<%text>${</%text> ${k} <%text>}</%text> = ${v}
% endfor

*****************
* 'this' object *
*****************

===========
this.attrib
===========

% if len(this.attrib):
    % for k, v in this.attrib.items():
<%text>${</%text> this.attrib["${k}"] <%text>}</%text> = ${v}
    % endfor
% else:
<empty>
% endif

===========
this.cc_set
===========

% for k, v in this.cc_set.attrib.items():
<%text>${</%text> this.cc_set.attrib["${k}"] <%text>}</%text> = ${v}
% endfor

==============
this.cc_module
==============

% for k, v in this.cc_module.attrib.items():
<%text>${</%text> this.cc_module.attrib["${k}"] <%text>}</%text> = ${v}
% endfor

============
this.cc_host
============

% for k, v in this.cc_host.attrib.items():
<%text>${</%text> this.cc_host.attrib["${k}"] <%text>}</%text> = ${v}
% endfor


***************
* 'cc' object *
***************

=====================
find all host element
=====================

<%text>
% for host in cc.findall('.//Host'):
    ${host.attrib['InnerIP'] }
% endfor
</%text>

% for host in cc.findall('.//Host'):
    ${host.attrib['InnerIP'] }
% endfor

==========================================
find all host element of module "gamesvr"
==========================================

<%text>
% for host in cc.findall('.//Module[@ModuleName="gamesvr"]/Host'):
    ${host.attrib['InnerIP'] }
% endfor
</%text>
% for host in cc.findall('.//Module[@ModuleName="gamesvr"]/Host'):
    ${host.attrib['InnerIP'] }
% endfor

==================================
find all host element of set "qq"
==================================

<%text>
% for host in cc.findall('.//Set[@SetName="qq"]//Host'):
    ${host.attrib['InnerIP'] }
% endfor
</%text>

% for host in cc.findall('.//Set[@SetName="qq"]//Host'):
    ${host.attrib['InnerIP'] }
% endfor

************
* end help *
************
"""


def main():
    """
    Main function to handle template rendering
    
    Expected input formats:
    1. Via stdin: JSON containing 'template' and 'context' keys
    2. Via file: --template-file and --context-file arguments
    3. Via inline: --template and --context arguments
    
    Security:
    - 在启动时应用运行时补丁（patch），拦截危险函数调用
    - 使用 MakoSandbox 上下文管理器跟踪用户代码执行
    """
    # 应用运行时补丁，拦截黑名单中的危险函数调用
    # 参考原项目：bk-process-config-manager/apps/utils/mako_utils/patch.py
    patch(default_black_list)
    parser = argparse.ArgumentParser(
        description='Render Mako templates with given context'
    )
    parser.add_argument(
        '--template',
        help='Template content string (inline)'
    )
    parser.add_argument(
        '--template-file',
        help='Path to template file'
    )
    parser.add_argument(
        '--context',
        help='Context JSON string (inline)'
    )
    parser.add_argument(
        '--context-file',
        help='Path to context JSON file'
    )
    parser.add_argument(
        '--stdin',
        action='store_true',
        help='Read JSON input from stdin (expected format: {"template": "...", "context": {...}})'
    )
    
    args = parser.parse_args()
    
    try:
        # Read input data
        if args.stdin or (not args.template and not args.template_file):
            # Read from stdin
            input_data = json.load(sys.stdin)
            template_content = input_data.get('template', '')
            context = input_data.get('context', {})
        else:
            # Read template
            if args.template_file:
                with open(args.template_file, 'r', encoding='utf-8') as f:
                    template_content = f.read()
            elif args.template:
                template_content = args.template
            else:
                print("Error: No template provided", file=sys.stderr)
                sys.exit(1)
            
            # Read context
            if args.context_file:
                with open(args.context_file, 'r', encoding='utf-8') as f:
                    context = json.load(f)
            elif args.context:
                context = json.loads(args.context)
            else:
                context = {}
        
        # Build cc and this objects from context
        def build_cc_context(ctx: dict) -> dict:
            # 1. Handle cc_xml if present
            cc_xml = ctx.get('cc_xml')
            if cc_xml:
                # Parse cc_xml into lxml Element
                try:
                    if isinstance(cc_xml, str):
                        cc = etree.fromstring(cc_xml.encode('utf-8'))
                    else:
                        cc = etree.fromstring(cc_xml)
                    ctx['cc'] = cc
                except (etree.XMLSyntaxError, etree.ParseError) as e:
                    # Provide descriptive error message indicating which field failed
                    xml_preview = (cc_xml[:200] + '...') if isinstance(cc_xml, str) and len(cc_xml) > 200 else str(cc_xml)[:200]
                    raise ValueError(
                        f"Failed to parse 'cc_xml' field as XML. "
                        f"XML syntax error: {str(e)}. "
                        f"XML content preview: {xml_preview}"
                    ) from e

            # 2. Build 'this' object if not already provided
            # If Go already passed a 'this' dict, convert it to object for attribute access
            if 'this' in ctx:
                if isinstance(ctx['this'], dict):
                    # Convert dict to object for this.attr access in Mako
                    this_obj = SimpleNamespace(**ctx['this'])
                    ctx['this'] = this_obj
                # else: already an object, keep as-is
                return ctx
            
            # 3. Auto-build 'this' from cc_xml + identifiers (backward compatibility)
            if cc_xml:
                bk_set_name = ctx.get('bk_set_name')
                bk_module_name = ctx.get('bk_module_name')
                bk_host_innerip = ctx.get('bk_host_innerip')
                bk_cloud_id = ctx.get('bk_cloud_id')

                this_obj = SimpleNamespace()
                cc = ctx.get('cc')

                # cc_set
                if bk_set_name and cc is not None:
                    this_obj.cc_set = cc.find(f'.//Set[@SetName="{bk_set_name}"]')
                # cc_module
                if bk_set_name and bk_module_name and cc is not None:
                    this_obj.cc_module = cc.find(
                        f'.//Set[@SetName="{bk_set_name}"]/Module[@ModuleName="{bk_module_name}"]'
                    )
                # cc_host
                if bk_set_name and bk_module_name and bk_host_innerip is not None and bk_cloud_id is not None and cc is not None:
                    xpath = (
                        f'.//Set[@SetName="{bk_set_name}"]'
                        f'/Module[@ModuleName="{bk_module_name}"]'
                        f'/Host[@InnerIP="{bk_host_innerip}"][@bk_cloud_id="{bk_cloud_id}"]'
                    )
                    this_obj.cc_host = cc.find(xpath)

                # attach attrib container to mimic original API (empty by default)
                if not hasattr(this_obj, 'attrib'):
                    this_obj.attrib = {}

                ctx['this'] = this_obj

            # 4. 补充内置字段（从 biz_global_variables 中提取属性值）
            # 完全按照 Python 代码逻辑实现（config_version.py 第 350-356 行）
            # for bk_obj_id, bk_obj_variables in biz_global_variables.items():
            #     for variable in bk_obj_variables:
            #         if bk_obj_id == CMDBHandler.BK_GLOBAL_OBJ_ID:
            #             continue
            #         bk_property_id = variable["bk_property_id"]
            #         context[bk_property_id] = getattr(this_context, f"cc_{bk_obj_id}").attrib.get(bk_property_id)
            biz_global_variables = ctx.get('biz_global_variables')
            if biz_global_variables and isinstance(biz_global_variables, dict) and 'this' in ctx:
                this_context = ctx.get('this')
                # CMDBHandler.BK_GLOBAL_OBJ_ID = "global" (参考 cmdb.py 第 68 行)
                BK_GLOBAL_OBJ_ID = "global"
                
                for bk_obj_id, bk_obj_variables in biz_global_variables.items():
                    # 跳过全局对象ID（与 Python 代码一致）
                    if bk_obj_id == BK_GLOBAL_OBJ_ID:
                        continue
                    
                    # 跳过 topo_variables（这是字段列表，不是对象类型，Go 实现中添加的辅助字段）
                    if bk_obj_id == "topo_variables":
                        continue
                    
                    # 验证 bk_obj_variables 是否为列表或元组
                    if not isinstance(bk_obj_variables, (list, tuple)):
                        continue
                    
                    # 获取对应的 cc 对象（this.cc_set, this.cc_module, this.cc_host）
                    cc_obj_attr = f"cc_{bk_obj_id}"
                    cc_obj = getattr(this_context, cc_obj_attr, None) if hasattr(this_context, cc_obj_attr) else None
                    
                    if cc_obj is None:
                        continue
                    
                    # 从 cc_obj 的 attrib 中提取属性值
                    # 完全按照 Python 代码逻辑：context[bk_property_id] = getattr(this_context, f"cc_{bk_obj_id}").attrib.get(bk_property_id)
                    if hasattr(cc_obj, 'attrib'):
                        for variable in bk_obj_variables:
                            if isinstance(variable, dict):
                                bk_property_id = variable.get("bk_property_id")
                                if bk_property_id:
                                    # 从 XML 元素的属性中获取值
                                    # lxml Element 的 attrib 支持 .get() 方法，如果属性不存在返回 None
                                    attr_value = cc_obj.attrib.get(bk_property_id)
                                    # Python 代码中即使 attr_value 为 None 也会设置，但这里只设置非 None 值
                                    # 因为 Python 代码中 .get() 可能返回 None，但实际 XML 属性通常不会为 None
                                    if attr_value is not None:
                                        ctx[bk_property_id] = attr_value
            
            return ctx

        context = build_cc_context(context)

        # Python 代码中最后会设置：context["global_variables"] = context
        # 这允许模板中通过 global_variables 访问所有变量
        # 注意：在 Python 端设置可以避免 JSON 编码时的循环引用问题
        context["global_variables"] = context

        # 生成 HELP（如果请求）
        # 参考原项目：bk-process-config-manager/apps/gsekit/configfile/config_version.py
        # mako_render 内部已经使用 MakoSandbox 上下文管理器，提供安全保护
        with_help = context.get('_with_help', False) or "${HELP}" in template_content
        if with_help:
            try:
                # mako_render 内部会使用 MakoSandbox 上下文管理器
                # 配合 patch.py 中的运行时拦截，提供双重安全保护
                context["HELP"] = mako_render(HELP_TEMPLATE, context)
            except Exception as e:
                # 如果生成 HELP 失败，记录错误但不中断渲染
                context["HELP"] = f"Error generating help: {str(e)}"

        # Render template
        # mako_render 内部已经使用 MakoSandbox 上下文管理器，提供安全保护
        # 安全机制包括：
        # 1. 编译时检查：通过 AST 访问器检查模板语法树（checker.py + visitor.py）
        # 2. 运行时拦截：通过 monkey patch 拦截危险函数调用（patch.py）
        # 3. 上下文跟踪：通过 MakoSandbox 跟踪用户代码执行（context.py）
        rendered_output = mako_render(template_content, context)
        
        # Output result to stdout without trailing newline
        # Use sys.stdout.write() instead of print() to avoid adding newline
        sys.stdout.write(rendered_output)
        sys.exit(0)
        
    except FileNotFoundError as e:
        print(f"Error: File not found - {e}", file=sys.stderr)
        sys.exit(1)
    except json.JSONDecodeError as e:
        print(f"Error: Invalid JSON format - {e}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"Error: {str(e)}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()
