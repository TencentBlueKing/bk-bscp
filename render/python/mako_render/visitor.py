# -*- coding: utf-8 -*-
"""
AST visitor for Mako template safety checking
参考原项目：bk-process-config-manager/apps/utils/mako_utils/visitor.py

通过遍历抽象语法树（AST）来检查模板中是否使用了危险的操作
"""

import ast

from .exceptions import ForbiddenMakoTemplateException


class MakoNodeVisitor(ast.NodeVisitor):
    """
    遍历语法树节点，只放行业务模板需要的语法和调用

    参考原项目：bk-process-config-manager/apps/utils/mako_utils/visitor.py
    """

    # 允许导入的模块。导入别名会在 visit_Import/visit_ImportFrom 中加入 allowed_module_names。
    WHITE_LIST_MODULES = {
        "datetime",
        "re",
        "random",
        "json",
        "math",
    }

    # 业务模板常用的基础函数。未列出的 builtin 即使 Python 可用，也不能在模板中调用。
    WHITE_LIST_FUNCTIONS = {
        "abs",
        "bool",
        "dict",
        "enumerate",
        "float",
        "int",
        "len",
        "list",
        "max",
        "min",
        "range",
        "round",
        "str",
        "sum",
        "tuple",
    }

    # 业务模板允许调用的方法。
    WHITE_LIST_METHODS = {
        "find",
        "findall",
        "get",
        "items",
        "keys",
        "replace",
        "values",
    }

    # 业务模板允许访问的数据属性。
    WHITE_LIST_ATTRS = {
        "attrib",
        "cc_host",
        "cc_module",
        "cc_set",
    }

    # 明确禁止的名称。普通上下文变量默认允许，但这些名称不能作为变量或函数出现。
    FORBIDDEN_NAMES = {
        "__import__",
        "breakpoint",
        "capture",
        "compile",
        "context",
        "delattr",
        "dir",
        "eval",
        "exec",
        "execfile",
        "exit",
        "getattr",
        "globals",
        "hasattr",
        "help",
        "input",
        "iter",
        "locals",
        "memoryview",
        "next",
        "octal",
        "open",
        "print",
        "quit",
        "self",
        "setattr",
        "super",
        "vars",
    }

    FORBIDDEN_NODE_TYPES = (
        ast.AsyncFunctionDef,
        ast.AsyncWith,
        ast.Await,
        ast.ClassDef,
        ast.Delete,
        ast.DictComp,
        ast.FunctionDef,
        ast.GeneratorExp,
        ast.Global,
        ast.Lambda,
        ast.ListComp,
        ast.Nonlocal,
        ast.Raise,
        ast.SetComp,
        ast.Try,
        ast.With,
        ast.Yield,
        ast.YieldFrom,
    )

    def __init__(self, white_list_modules=None):
        """
        初始化节点访问器
        
        Args:
            white_list_modules: 自定义白名单模块列表（默认使用类属性）
        """
        self.white_list_modules = set(white_list_modules or self.WHITE_LIST_MODULES)
        self.allowed_module_names = set(self.white_list_modules)

    def _reject(self, message):
        raise ForbiddenMakoTemplateException(message)

    def _is_dunder(self, name):
        return name.startswith("__") and name.endswith("__")

    def _root_name(self, node):
        while isinstance(node, ast.Attribute):
            node = node.value
        if isinstance(node, ast.Name):
            return node.id
        return ""

    def _is_allowed_module_attr(self, node):
        return self._root_name(node) in self.allowed_module_names

    def generic_visit(self, node):
        if isinstance(node, self.FORBIDDEN_NODE_TYPES):
            self._reject("发现非法语法使用:[{}]，请修改".format(node.__class__.__name__))
        super().generic_visit(node)

    def visit_Attribute(self, node):
        """访问属性节点"""
        if self._is_dunder(node.attr):
            raise ForbiddenMakoTemplateException("发现非法属性使用:[{}]，请修改".format(node.attr))

        if self._is_allowed_module_attr(node):
            return

        if isinstance(node.value, ast.Name) and node.value.id == "this":
            return

        if node.attr in self.WHITE_LIST_ATTRS or node.attr in self.WHITE_LIST_METHODS:
            return

        self._reject("发现非法属性使用:[{}]，请修改".format(node.attr))

    def visit_Call(self, node):
        """访问函数调用节点"""
        func = node.func
        if isinstance(func, ast.Name):
            if func.id not in self.WHITE_LIST_FUNCTIONS:
                self._reject("发现非法函数调用:[{}]，请修改".format(func.id))
        elif isinstance(func, ast.Attribute):
            if self._is_dunder(func.attr):
                self._reject("发现非法函数调用:[{}]，请修改".format(func.attr))
            if not self._is_allowed_module_attr(func) and func.attr not in self.WHITE_LIST_METHODS:
                self._reject("发现非法函数调用:[{}]，请修改".format(func.attr))
        else:
            self._reject("发现非法函数调用:[{}]，请修改".format(func.__class__.__name__))
        self.generic_visit(node)

    def visit_Name(self, node):
        """访问名称节点"""
        if self._is_dunder(node.id) or node.id in self.FORBIDDEN_NAMES:
            raise ForbiddenMakoTemplateException("发现非法名称使用:[{}]，请修改".format(node.id))

    def visit_Import(self, node):
        """访问导入节点"""
        for name in node.names:
            module_name = name.name.split(".", 1)[0]
            if module_name not in self.white_list_modules:
                self._reject("发现非法导入:[{}]，请修改".format(name.name))
            self.allowed_module_names.add(name.asname or module_name)

    def visit_ImportFrom(self, node):
        """访问从模块导入节点"""
        module_name = (node.module or "").split(".", 1)[0]
        if node.level != 0 or module_name not in self.white_list_modules:
            self._reject("发现非法导入:[{}]，请修改".format(node.module or ""))
        for name in node.names:
            if name.name.startswith("_"):
                self._reject("发现非法导入:[{}]，请修改".format(name.name))
            self.allowed_module_names.add(name.asname or name.name)
