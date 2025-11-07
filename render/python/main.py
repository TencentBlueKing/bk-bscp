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
        
        # Render template
        rendered_output = mako_render(template_content, context)
        
        # Output result to stdout
        print(rendered_output)
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
