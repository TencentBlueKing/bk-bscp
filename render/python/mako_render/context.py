# -*- coding: utf-8 -*-
"""
MakoSandbox context manager for secure template rendering
参考原项目：bk-process-config-manager/apps/utils/mako_utils/context.py
"""

import threading
import uuid
from contextlib import ContextDecorator

# 存储正在执行用户代码的线程 ID 列表（线程安全）
_in_user_code_thread_ids = []
_thread_ids_lock = threading.Lock()
_thread_local = threading.local()


def add_thread_id(thread_id):
    """
    线程安全地添加线程 ID 到用户代码线程列表
    
    Args:
        thread_id: 要添加的线程 ID
    """
    with _thread_ids_lock:
        if thread_id not in _in_user_code_thread_ids:
            _in_user_code_thread_ids.append(thread_id)


def remove_thread_id(thread_id):
    """
    线程安全地从用户代码线程列表中移除线程 ID
    
    Args:
        thread_id: 要移除的线程 ID
    """
    with _thread_ids_lock:
        if thread_id in _in_user_code_thread_ids:
            _in_user_code_thread_ids.remove(thread_id)


def is_thread_id_in_list(thread_id):
    """
    线程安全地检查线程 ID 是否在用户代码线程列表中
    
    Args:
        thread_id: 要检查的线程 ID
        
    Returns:
        bool: 如果线程 ID 在列表中返回 True，否则返回 False
    """
    with _thread_ids_lock:
        return thread_id in _in_user_code_thread_ids


def get_in_user_code_thread_ids():
    """
    获取用户代码线程 ID 列表的副本（线程安全）
    
    Returns:
        list: 用户代码线程 ID 列表的副本
    """
    with _thread_ids_lock:
        return list(_in_user_code_thread_ids)


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
        add_thread_id(self.thread_id)
        return self
    
    def __exit__(self, exc_type, exc_value, traceback):
        """退出上下文时，从用户代码线程列表中移除当前线程 ID"""
        remove_thread_id(self.thread_id)

