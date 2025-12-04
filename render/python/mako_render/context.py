# -*- coding: utf-8 -*-
"""
MakoSandbox context manager for secure template rendering
参考原项目：bk-process-config-manager/apps/utils/mako_utils/context.py
"""

import threading
import uuid
from contextlib import ContextDecorator

# 存储正在执行用户代码的线程 ID 列表
in_user_code_thread_ids = []
_thread_local = threading.local()


def set_thread_id(thread_id=None):
    """
    设置当前线程的 thread_id
    """
    if not thread_id:
        thread_id = str(uuid.uuid4())
    _thread_local.thread_id = thread_id
    return thread_id


def get_thread_id():
    """获取当前线程的 thread_id"""
    return getattr(_thread_local, "thread_id", None)


class MakoSandbox(ContextDecorator):
    """
    MakoSandbox 上下文管理器
    用于跟踪用户代码的执行，配合 patch.py 中的运行时拦截机制使用
    
    参考原项目：bk-process-config-manager/apps/utils/mako_utils/context.py
    """
    
    def __init__(self, *args, **kwargs):
        self.thread_id = set_thread_id()
    
    def __enter__(self, *args, **kwargs):
        """进入上下文时，将当前线程 ID 添加到用户代码线程列表"""
        in_user_code_thread_ids.append(self.thread_id)
        return self
    
    def __exit__(self, exc_type, exc_value, traceback):
        """退出上下文时，从用户代码线程列表中移除当前线程 ID"""
        if self.thread_id in in_user_code_thread_ids:
            in_user_code_thread_ids.remove(self.thread_id)

