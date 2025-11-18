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


def main():
    """
    Main function to handle template rendering
    
    Expected input formats:
    1. Via stdin: JSON containing 'template' and 'context' keys
    2. Via file: --template-file and --context-file arguments
    3. Via inline: --template and --context arguments
    """
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
            
            return ctx

        context = build_cc_context(context)

        # Render template
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
