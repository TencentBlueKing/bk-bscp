# -*- coding: utf-8 -*-
"""
AST visitor for Mako template safety checking
参考原项目：bk-process-config-manager/apps/utils/mako_utils/visitor.py

通过遍历抽象语法树（AST）来检查模板中是否使用了危险的操作
"""

import ast

from .exceptions import ForbiddenMakoTemplateException


def _optional_ast_node_types(*names):
    return tuple(getattr(ast, name) for name in names if hasattr(ast, name))


class MakoNodeVisitor(ast.NodeVisitor):
    """
    遍历语法树节点，只放行业务模板需要的语法和调用

    参考原项目：bk-process-config-manager/apps/utils/mako_utils/visitor.py
    """

    # 允许导入的模块。实际可访问成员由 WHITE_LIST_MODULE_CALLS/WHITE_LIST_MODULE_ATTRS 继续收窄。
    WHITE_LIST_MODULES = {
        "datetime",
        "re",
        "random",
        "json",
        "math",
    }

    WHITE_LIST_MODULE_CALLS = {
        "datetime": {
            ("date",),
            ("date", "today"),
            ("datetime",),
            ("datetime", "now"),
            ("datetime", "utcnow"),
            ("timedelta",),
        },
        "json": {
            ("dumps",),
            ("loads",),
        },
        "math": {
            ("acos",),
            ("asin",),
            ("atan",),
            ("atan2",),
            ("ceil",),
            ("cos",),
            ("degrees",),
            ("exp",),
            ("fabs",),
            ("floor",),
            ("fmod",),
            ("fsum",),
            ("hypot",),
            ("log",),
            ("log10",),
            ("pow",),
            ("radians",),
            ("sin",),
            ("sqrt",),
            ("tan",),
            ("trunc",),
        },
        "random": {
            ("choice",),
            ("choices",),
            ("randint",),
            ("random",),
            ("randrange",),
            ("sample",),
            ("uniform",),
        },
        "re": {
            ("compile",),
            ("fullmatch",),
            ("match",),
            ("search",),
            ("split",),
            ("sub",),
        },
    }

    WHITE_LIST_MODULE_ATTRS = {
        "datetime": {
            ("date",),
            ("datetime",),
            ("timedelta",),
        },
        "math": {
            ("e",),
            ("inf",),
            ("nan",),
            ("pi",),
            ("tau",),
        },
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
        ast.NamedExpr,
        ast.Raise,
        ast.SetComp,
        ast.Try,
        ast.With,
        ast.Yield,
        ast.YieldFrom,
        *_optional_ast_node_types("Match", "TryStar", "TypeAlias"),
    )

    def __init__(self, white_list_modules=None):
        """
        初始化节点访问器
        
        Args:
            white_list_modules: 自定义白名单模块列表（默认使用类属性）
        """
        self.white_list_modules = set(white_list_modules or self.WHITE_LIST_MODULES)
        self.allowed_module_bindings = {}
        self.allowed_import_bindings = {}

    def _reject(self, message):
        raise ForbiddenMakoTemplateException(message)

    def _is_dunder(self, name):
        return name.startswith("__") and name.endswith("__")

    def _attribute_parts(self, node):
        parts = []
        while isinstance(node, ast.Attribute):
            parts.append(node.attr)
            node = node.value
        if not isinstance(node, ast.Name):
            return "", ()
        parts.reverse()
        return node.id, tuple(parts)

    def _module_member_path(self, node):
        root, parts = self._attribute_parts(node)
        if root in self.allowed_module_bindings:
            return self.allowed_module_bindings[root], parts
        if root in self.allowed_import_bindings:
            module_name, base_parts = self.allowed_import_bindings[root]
            return module_name, base_parts + parts
        return "", ()

    def _validate_module_member_path(self, module_name, member_path):
        if not module_name or not member_path:
            return
        for part in member_path:
            if part.startswith("_"):
                self._reject("发现非法属性使用:[{}]，请修改".format(part))

    def _module_allowed_paths(self, module_name):
        return (
            self.WHITE_LIST_MODULE_CALLS.get(module_name, set())
            | self.WHITE_LIST_MODULE_ATTRS.get(module_name, set())
        )

    def _is_allowed_module_attr_path(self, module_name, member_path):
        self._validate_module_member_path(module_name, member_path)
        allowed_paths = self._module_allowed_paths(module_name)
        return any(path[: len(member_path)] == member_path for path in allowed_paths)

    def _is_allowed_module_call_path(self, module_name, member_path):
        self._validate_module_member_path(module_name, member_path)
        return member_path in self.WHITE_LIST_MODULE_CALLS.get(module_name, set())

    def _validate_binding_name(self, name):
        if self._is_dunder(name) or name in self.FORBIDDEN_NAMES:
            self._reject("发现非法名称使用:[{}]，请修改".format(name))

    def _unbind_target(self, target):
        if isinstance(target, ast.Name):
            self._validate_binding_name(target.id)
            self.allowed_module_bindings.pop(target.id, None)
            self.allowed_import_bindings.pop(target.id, None)
            return

        if isinstance(target, (ast.Tuple, ast.List)):
            for element in target.elts:
                self._unbind_target(element)
            return

        if isinstance(target, ast.Starred):
            self._unbind_target(target.value)
            return

        self._reject("发现非法赋值目标:[{}]，请修改".format(target.__class__.__name__))

    def generic_visit(self, node):
        if isinstance(node, self.FORBIDDEN_NODE_TYPES):
            self._reject("发现非法语法使用:[{}]，请修改".format(node.__class__.__name__))
        super().generic_visit(node)

    def visit_Attribute(self, node):
        """访问属性节点"""
        if self._is_dunder(node.attr):
            raise ForbiddenMakoTemplateException("发现非法属性使用:[{}]，请修改".format(node.attr))

        module_name, member_path = self._module_member_path(node)
        if module_name:
            if self._is_allowed_module_attr_path(module_name, member_path):
                return
            self._reject("发现非法属性使用:[{}]，请修改".format(".".join(member_path)))

        if isinstance(node.value, ast.Name) and node.value.id == "this":
            return

        if node.attr in self.WHITE_LIST_ATTRS or node.attr in self.WHITE_LIST_METHODS:
            return

        self._reject("发现非法属性使用:[{}]，请修改".format(node.attr))

    def visit_Call(self, node):
        """访问函数调用节点"""
        func = node.func
        if isinstance(func, ast.Name):
            if func.id in self.allowed_import_bindings:
                module_name, member_path = self.allowed_import_bindings[func.id]
                if not self._is_allowed_module_call_path(module_name, member_path):
                    self._reject("发现非法函数调用:[{}]，请修改".format(func.id))
            elif func.id not in self.WHITE_LIST_FUNCTIONS:
                self._reject("发现非法函数调用:[{}]，请修改".format(func.id))
        elif isinstance(func, ast.Attribute):
            if self._is_dunder(func.attr):
                self._reject("发现非法函数调用:[{}]，请修改".format(func.attr))
            module_name, member_path = self._module_member_path(func)
            if module_name:
                if not self._is_allowed_module_call_path(module_name, member_path):
                    self._reject("发现非法函数调用:[{}]，请修改".format(".".join(member_path)))
            elif func.attr not in self.WHITE_LIST_METHODS:
                self._reject("发现非法函数调用:[{}]，请修改".format(func.attr))
        else:
            self._reject("发现非法函数调用:[{}]，请修改".format(func.__class__.__name__))
        self.generic_visit(node)

    def visit_Assign(self, node):
        """访问赋值节点"""
        self.visit(node.value)
        for target in node.targets:
            self._unbind_target(target)

    def visit_AnnAssign(self, node):
        """访问带类型标注的赋值节点"""
        if node.annotation is not None:
            self.visit(node.annotation)
        if node.value is not None:
            self.visit(node.value)
        self._unbind_target(node.target)

    def visit_AugAssign(self, node):
        """访问复合赋值节点"""
        self._unbind_target(node.target)
        self.visit(node.value)

    def visit_For(self, node):
        """访问循环节点"""
        self._unbind_target(node.target)
        self.visit(node.iter)
        for child in node.body:
            self.visit(child)
        for child in node.orelse:
            self.visit(child)

    def visit_Name(self, node):
        """访问名称节点"""
        self._validate_binding_name(node.id)

    def visit_Import(self, node):
        """访问导入节点"""
        for name in node.names:
            module_name = name.name.split(".", 1)[0]
            if name.name != module_name or module_name not in self.white_list_modules:
                self._reject("发现非法导入:[{}]，请修改".format(name.name))
            binding_name = name.asname or module_name
            self._validate_binding_name(binding_name)
            self.allowed_module_bindings[binding_name] = module_name

    def visit_ImportFrom(self, node):
        """访问从模块导入节点"""
        module_name = node.module or ""
        if node.level != 0 or module_name not in self.white_list_modules:
            self._reject("发现非法导入:[{}]，请修改".format(node.module or ""))
        for name in node.names:
            if name.name == "*" or name.name.startswith("_"):
                self._reject("发现非法导入:[{}]，请修改".format(name.name))
            binding_name = name.asname or name.name
            member_path = (name.name,)
            self._validate_binding_name(binding_name)
            if not self._is_allowed_module_attr_path(module_name, member_path):
                self._reject("发现非法导入:[{}]，请修改".format(name.name))
            self.allowed_import_bindings[binding_name] = (module_name, member_path)
