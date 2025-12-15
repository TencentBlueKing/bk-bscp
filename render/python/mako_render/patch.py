# -*- coding: utf-8 -*-
"""
Runtime patching for dangerous functions
参考原项目：bk-process-config-manager/apps/utils/mako_utils/patch.py

通过 monkey patch 在运行时拦截危险函数调用
"""

import functools
from importlib import import_module

from .context import get_thread_id, is_thread_id_in_list
from .exceptions import ForbiddenMakoTemplateException


def patch(black_list):
    """
    对黑名单中的模块和函数进行 monkey patch，在用户代码执行时拦截调用
    
    Args:
        black_list: 黑名单字典，格式为 {module_name: [call_name1, call_name2, ...]}
    """
    for module_name, call_names in black_list.items():
        try:
            module = import_module(module_name)
        except ImportError:
            # 如果模块不存在，跳过
            continue
        
        for call_name in call_names:
            # 使用默认参数来正确捕获循环变量，避免闭包问题
            # 参考原项目：bk-process-config-manager/apps/utils/mako_utils/patch.py
            def create_patched_call(module=module, call_name=call_name):
                """创建补丁函数"""
                try:
                    call = getattr(module, call_name)
                except AttributeError:
                    # 如果函数不存在，跳过
                    return None
                
                @functools.wraps(call)
                def patched_call(*args, **kwargs):
                    """补丁函数：检查是否在用户代码线程中执行"""
                    thread_id = get_thread_id()
                    # 只有当线程 ID 存在且在用户代码线程列表中时才拦截
                    # 这样可以避免拦截 Mako 内部或其他系统代码的调用
                    # 注意：如果 thread_id 为 None，说明不在跟踪范围内，允许执行
                    if thread_id is not None and is_thread_id_in_list(thread_id):
                        raise ForbiddenMakoTemplateException("I am watching you!")
                    else:
                        return call(*args, **kwargs)
                
                return patched_call
            
            try:
                new_call = create_patched_call()
                if new_call is not None:
                    setattr(module, call_name, new_call)
            except AttributeError:
                # 如果函数不存在，跳过（与原项目保持一致）
                continue
            except Exception:
                # 如果设置失败，跳过
                continue


# 默认黑名单：禁止在模板中使用的模块和函数
# 参考原项目：bk-process-config-manager/apps/utils/mako_utils/patch.py
default_black_list = {
    "os": [
        "chdir",
        "chmod",
        "kill",
        "link",
        "listdir",
        "mkdir",
        "putenv",
        "remove",
        "rename",
        "rmdir",
        "scandir",
        "symlink",
        "system",
        "truncate",
        "utime",
        "popen",
        "execl",
        "execle",
        "execv",
        "execlp",
        "execlpe",
        "execvp",
        "execvpe",
        "spawnl",
        "spawnlpe",
        "spawnv",
        "spawnlp",
        "spawnve",
        "getenv",
        "fdopen",
        "spawnvpe",
    ],
    "subprocess": ["Popen", "call", "getstatusoutput", "getoutput", "check_output", "check_call", "run"],
    "ctypes": [
        "addressof",
        "create_string_buffer",
        "create_unicode_buffer",
        "string_at",
        "wstring_at",
        "CDLL",
        "PyDLL",
        "LibraryLoader",
    ],
    "fcntl": ["fcntl", "flock", "ioctl", "lockf"],
    "glob": ["glob"],
    "imaplib": ["socket"],
    "pdb": ["Pdb"],
    "pty": ["spawn"],
    "shutil": [
        "copy",
        "copy2",
        "chown",
        "which",
        "disk_usage",
        "copyfile",
        "copymode",
        "copytree",
        "copystat",
        "copytree",
        "make_archive",
        "move",
        "rmtree",
        "unpack_archive",
    ],
    "signal": [
        "pthread_kill",
        "pause",
        "pthread_sigmask",
        "set_wakeup_fd",
        "setitimer",
        "siginterrupt",
        "sigpending",
        "sigwait",
    ],
    "socket": [
        "socket",
        "create_connection",
        "getaddrinfo",
        "gethostbyaddr",
        "gethostbyname",
        "gethostname",
        "getnameinfo",
        "getservbyname",
        "getservbyport",
        "sethostname",
    ],
    "sys": [
        "callstats",
        "call_tracing",
        "getprofile",
        "setcheckinterval",
        "setdlopenflags",
        "setrecursionlimit",
        "setswitchinterval",
        "setprofile",
        "set_asyncgen_hooks",
        "set_coroutine_origin_tracking_depth",
        "set_coroutine_wrapper",
        "settrace",
        "exit",
        "__loader__",
    ],
    "tempfile": ["mkdtemp", "mkstemp"],
    "webbrowser": ["open"],
}
