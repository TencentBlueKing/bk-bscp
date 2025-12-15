# -*- coding: utf-8 -*-
"""
AST visitor for Mako template safety checking
参考原项目：bk-process-config-manager/apps/utils/mako_utils/visitor.py

通过遍历抽象语法树（AST）来检查模板中是否使用了危险的操作
"""

import ast
import _ast

from .exceptions import ForbiddenMakoTemplateException


class MakoNodeVisitor(ast.NodeVisitor):
    """
    遍历语法树节点，遇到黑名单中的模块或方法时，抛出异常
    
    参考原项目：bk-process-config-manager/apps/utils/mako_utils/visitor.py
    """
    
    # 黑名单：禁止使用的模块和方法
    # 参考原项目：使用 dir(__import__("module")) 动态获取所有方法，拦截整个模块
    BLACK_LIST_MODULE_METHODS = {
        "os": dir(__import__("os")),
        "subprocess": dir(__import__("subprocess")),
        "shutil": dir(__import__("shutil")),
        "ctypes": dir(__import__("ctypes")),
        "codecs": dir(__import__("codecs")),
        "sys": dir(__import__("sys")),
        "socket": dir(__import__("socket")),
        "webbrowser": dir(__import__("webbrowser")),
        "threading": dir(__import__("threading")),
        "sqlite3": dir(__import__("sqlite3")),
        "signal": dir(__import__("signal")),
        "imaplib": dir(__import__("imaplib")),
        "fcntl": dir(__import__("fcntl")),
        "pdb": dir(__import__("pdb")),
        "pty": dir(__import__("pty")),
        "glob": dir(__import__("glob")),
        "tempfile": dir(__import__("tempfile")),
        # types 模块需要特殊处理
        "types": dir(__import__("types").CodeType) + dir(__import__("types").FrameType),
        "builtins": [
            "getattr",
            "hasattr",
            "breakpoint",
            "compile",
            "delattr",
            "open",
            "eval",
            "exec",
            "execfile",
            "exit",
            "dir",
            "globals",
            "locals",
            "input",
            "iter",
            "next",
            "quit",
            "setattr",
            "vars",
            "memoryview",
            "super",
            "print",
            "__import__",
            "help",  # 添加 help，原项目没有但应该拦截
        ],
        "mako_built": ["context", "self", "octal", "capture"],
    }
    
    # 构建黑名单方法集合
    BLACK_LIST_METHODS = set()
    for module_name, methods in BLACK_LIST_MODULE_METHODS.items():
        BLACK_LIST_METHODS.add(module_name)
        BLACK_LIST_METHODS.update(methods)
    
    # 白名单：允许使用的模块
    WHITE_LIST_MODULES = [
        "datetime",
        "re",
        "random",
        "json",
        "math",
        "test",
        "path",
        "enumerate",
        "name",
        "time",
        "replace",
    ]
    
    # 白名单：允许使用的属性
    WHITE_LIST_ATTR = ["get", "replace"]
    
    # 白名单：允许使用的变量名（上下文变量）
    WHITE_LIST_NAMES = [
        "HELP",  # HELP 是生成的上下文变量，应该允许使用
    ]
    
    def __init__(self, black_list_methods=None, white_list_modules=None):
        """
        初始化节点访问器
        
        Args:
            black_list_methods: 自定义黑名单方法集合（默认使用类属性）
            white_list_modules: 自定义白名单模块列表（默认使用类属性）
        """
        self.black_list_methods = black_list_methods or self.BLACK_LIST_METHODS
        self.white_list_modules = white_list_modules or self.WHITE_LIST_MODULES
    
    def is_white_list_ast_obj(self, ast_obj: _ast.AST) -> bool:
        """
        判断是否白名单对象，特殊豁免
        
        Args:
            ast_obj: 抽象语法树节点
            
        Returns:
            bool: 如果是白名单对象返回 True
        """
        # re 正则表达式允许使用 compile
        if isinstance(ast_obj, _ast.Attribute):
            if ast_obj.attr in self.WHITE_LIST_ATTR:
                return True
            if isinstance(ast_obj.value, _ast.Name):
                if ast_obj.value.id == "re" and ast_obj.attr in ["compile"]:
                    return True
                if ast_obj.value.id in self.WHITE_LIST_MODULES:
                    return True
        if isinstance(ast_obj, _ast.Name):
            if ast_obj.id in self.WHITE_LIST_MODULES:
                return True
            # 允许使用白名单中的变量名（如 HELP）
            if ast_obj.id in self.WHITE_LIST_NAMES:
                return True
        return False
    
    def visit_Attribute(self, node):
        """访问属性节点"""
        if self.is_white_list_ast_obj(node):
            return
        if node.attr in self.black_list_methods:
            raise ForbiddenMakoTemplateException("发现非法属性使用:[{}]，请修改".format(node.attr))
    
    def visit_Name(self, node):
        """访问名称节点"""
        if self.is_white_list_ast_obj(node):
            return
        if node.id in self.black_list_methods:
            raise ForbiddenMakoTemplateException("发现非法名称使用:[{}]，请修改".format(node.id))
    
    def visit_Import(self, node):
        """访问导入节点"""
        for name in node.names:
            if name.name not in self.white_list_modules:
                raise ForbiddenMakoTemplateException("发现非法导入:[{}]，请修改".format(name.name))
    
    def visit_ImportFrom(self, node):
        """访问从模块导入节点"""
        self.visit_Import(node)

