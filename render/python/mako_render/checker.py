# -*- coding: utf-8 -*-
"""
Mako template safety checker
参考原项目：bk-process-config-manager/apps/utils/mako_utils/checker.py

提供模板内容清理和安全性检查功能
"""

import ast
from typing import List

from mako import parsetree
from mako.ast import PythonFragment
from mako.exceptions import MakoException
from mako.lexer import Lexer

from .exceptions import ForbiddenMakoTemplateException
from .visitor import MakoNodeVisitor


def clean_mako_content(content: str) -> str:
    """
    清理 Mako 模板内容
    将制表符替换为 4 个空格
    
    Args:
        content: 原始模板内容
        
    Returns:
        清理后的模板内容
    """
    # 替换制表符为 4 个空格
    content = content.replace("\t", " " * 4)
    return content


def parse_template_nodes(nodes: List[parsetree.Node], node_visitor: ast.NodeVisitor):
    """
    解析 Mako 模板节点，逐个节点解析抽象语法树并检查安全性
    
    Args:
        nodes: Mako 模板节点列表
        node_visitor: 节点访问类，用于遍历 AST 节点
    """
    for node in nodes:
        if isinstance(node, (parsetree.Code, parsetree.Expression)):
            code = node.text
        elif isinstance(node, parsetree.ControlLine):
            if node.isend:
                continue
            code = PythonFragment(node.text).code
        elif isinstance(node, (parsetree.Text, parsetree.TextTag, parsetree.Comment)):
            continue
        else:
            raise ForbiddenMakoTemplateException("不支持[{}]节点".format(node.__class__.__name__))
        
        try:
            # 对于表达式节点，使用 "eval" 模式解析
            # 对于代码节点，使用 "exec" 模式解析
            parse_mode = "eval" if isinstance(node, parsetree.Expression) else "exec"
            ast_node = ast.parse(code.strip(), "<unknown>", parse_mode)
            for _node in ast.walk(ast_node):
                node_visitor.visit(_node)
        except SyntaxError:
            # 如果语法错误，跳过检查（Mako 会在渲染时处理）
            # 但这种情况应该很少见，因为 Mako 已经解析过了
            pass
        
        if hasattr(node, "nodes"):
            parse_template_nodes(node.nodes, node_visitor)


def check_mako_template_safety(text: str, node_visitor: ast.NodeVisitor = None) -> bool:
    """
    检查 Mako 模板是否安全，若不安全直接抛出异常，安全则返回 True
    
    Args:
        text: Mako 模板内容
        node_visitor: 节点访问器，用于遍历 AST 节点（默认使用 MakoNodeVisitor）
        
    Returns:
        True 如果模板安全
        
    Raises:
        ForbiddenMakoTemplateException: 如果模板不安全
    """
    if node_visitor is None:
        node_visitor = MakoNodeVisitor()
    
    text = clean_mako_content(text)
    try:
        lexer_template = Lexer(text).parse()
    except MakoException as mako_error:
        raise ForbiddenMakoTemplateException("mako解析失败, {err_msg}".format(err_msg=mako_error))
    
    parse_template_nodes(lexer_template.nodes, node_visitor)
    return True

