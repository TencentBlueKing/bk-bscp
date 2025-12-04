# -*- coding: utf-8 -*-
"""
Exceptions for Mako template rendering
参考原项目：bk-process-config-manager/apps/utils/mako_utils/exceptions.py
"""


class ForbiddenMakoTemplateException(Exception):
    """禁止的 Mako 模板操作异常"""
    pass

