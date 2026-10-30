# MakoSandbox 安全配置说明

## 概述

本实现参考原项目 `bk-process-config-manager/apps/utils/mako_utils`，提供了三层安全机制来保护 Mako 模板渲染：

1. **编译时检查**：通过 AST 访问器检查模板语法树（`checker.py` + `visitor.py`）
2. **运行时拦截**：通过 monkey patch 拦截危险函数调用（`patch.py`）
3. **上下文跟踪**：通过 `MakoSandbox` 上下文管理器跟踪用户代码执行（`context.py`）

## 安全机制详解

### 1. MakoSandbox 上下文管理器

`MakoSandbox` 是一个上下文管理器（`ContextDecorator`），通过线程 ID 来跟踪用户代码的执行。

**实现位置**：`mako_render/context.py`

**工作原理**：
- 进入上下文时，将当前线程 ID 添加到 `in_user_code_thread_ids` 列表
- 退出上下文时，从列表中移除线程 ID
- 配合 `patch.py` 中的运行时拦截机制，当检测到用户代码线程时，拦截危险函数调用

**使用方式**：
```python
from mako_render.context import MakoSandbox

with MakoSandbox():
    # 模板渲染代码
    result = template.render(**context)
```

### 2. 运行时补丁（Runtime Patching）

通过 monkey patch 在运行时拦截危险函数调用。

**实现位置**：`mako_render/patch.py`

**工作原理**：
- 在程序启动时（`main.py` 的 `main()` 函数），调用 `patch(default_black_list)` 应用补丁
- 对黑名单中的模块和函数进行 monkey patch
- 当函数被调用时，检查当前线程是否在 `in_user_code_thread_ids` 中
- 如果是用户代码线程，抛出 `ForbiddenMakoTemplateException` 异常

**黑名单配置（`default_black_list`）**：
- `os` 与 `posix` 使用同一份进程调用名单：`system`, `chdir`, `chmod`, `kill`, `popen`, `open`, `exec*`, `spawn*` 等。两者是不同模块属性，只拦截 `os` 挡不住 `posix`
- `builtins`、`io`：只拦截 `open`
- `subprocess` 模块：`Popen`, `call`, `run` 等
- `ctypes` 模块：`CDLL`, `PyDLL`, `LibraryLoader` 等
- `socket` 模块：`socket`, `create_connection` 等
- `sys` 模块：`exit`, `settrace`, `setprofile` 等
- 其他危险模块：`shutil`, `signal`, `fcntl`, `tempfile`, `webbrowser` 等

**注意**：
- 运行时只拦截 `builtins.open` 和 `io.open`，且仅在 `MakoSandbox` 线程内生效
- **不拦截** `builtins.compile`、`builtins.exec`、`builtins.__import__`：Mako 编译模板，以及模板中的 `import json` 等白名单导入，都依赖这些入口
- `eval`、`exec`、`__import__` 等其余危险内置名称仍由编译时检查（`visitor.py`）拒绝

### 3. AST 语法树检查（编译时检查）

通过遍历抽象语法树（AST）来检查模板中是否使用了危险的操作。

**实现位置**：`mako_render/checker.py` + `mako_render/visitor.py`

**工作原理**：
- 使用 `MakoNodeVisitor` 遍历模板的 AST
- 检查 `Import`、`ImportFrom`、`Attribute`、`Name` 节点
- 如果发现黑名单中的模块或方法，抛出 `ForbiddenMakoTemplateException` 异常
- **检查失败时直接抛出异常，阻止模板编译**

**黑名单配置（`BLACK_LIST_MODULE_METHODS`）**：
- 使用 `dir(__import__("module"))` 动态获取整个模块的所有方法（更严格）
- 禁止导入的模块：`os`, `subprocess`, `ctypes`, `socket`, `sys`, `shutil`, `signal` 等
- 禁止使用的方法：`open`, `eval`, `exec`, `compile`, `__import__`, `exit`, `input`, `help` 等
- 包括 `builtins` 模块的危险函数：`open`, `eval`, `exec`, `compile`, `__import__`, `help` 等

**白名单**：
- 允许导入的模块：`datetime`, `re`, `random`, `json`, `math`, `time` 等
- 允许使用的属性：`get`, `replace` 等
- 允许使用的变量名：`HELP`（上下文变量）

**表达式解析**：
- `${expression}` 和 `<% code %>` 都按 Mako `PythonCode` 的 `"exec"` 模式解析 AST
- 解析出现 `SyntaxError` 时直接拒绝渲染，不再跳过检查
- 控制行（`% for`、`% if` 等）先按 Mako `PythonFragment` 补成完整语句，再做同样的检查
- 因此 `${open}`，以及赋值、分号、`import` 这类不能单独作为 `eval` 表达式、但会被 Mako 嵌进 `str(...)` 执行的写法，都会在编译前被拦截

## 两个黑名单的区别

### `default_black_list` (在 `patch.py` 中)

- **用途**：运行时拦截（Runtime Interception）
- **时机**：模板执行时
- **机制**：Monkey Patch，拦截函数调用
- **覆盖范围**：手动列出需要拦截的函数（部分函数）
- **使用场景**：当模板代码在 `with MakoSandbox():` 中执行时，如果调用了黑名单中的函数，会被拦截
- **示例**：
  ```python
  "os": ["system", "chdir", "popen", ...]  # 只列出部分危险函数
  ```

### `BLACK_LIST_MODULE_METHODS` (在 `visitor.py` 中)

- **用途**：编译时检查（Compile-time Check）
- **时机**：模板编译前
- **机制**：AST 访问器，检查模板语法树
- **覆盖范围**：使用 `dir(__import__("module"))` 动态获取整个模块的所有方法（更严格）
- **使用场景**：在模板编译前，检查模板代码中是否使用了黑名单中的模块或方法
- **示例**：
  ```python
  "os": dir(__import__("os"))  # 拦截 os 模块的所有方法和属性
  ```

### 为什么需要两个？

1. **双重保护**：
   - 编译时检查：提前发现危险代码
   - 运行时拦截：防止绕过编译时检查

2. **互补**：
   - `BLACK_LIST_MODULE_METHODS` 更严格，拦截整个模块
   - `default_black_list` 更精确，只拦截明确列出的函数

3. **性能**：
   - 编译时检查：一次检查，避免运行时开销
   - 运行时拦截：作为最后一道防线

## 使用方式

### 在代码中使用

```python
from mako_render import mako_render

# mako_render 内部已经使用 MakoSandbox 上下文管理器
# 配合 patch.py 中的运行时拦截，提供双重安全保护
# 编译时检查默认启用，会在模板编译前检查安全性
rendered = mako_render(template_content, context)
```

### 当前实现

在 `render/python/main.py` 中：

1. **启动时应用补丁**：
```python
# 应用运行时补丁，拦截黑名单中的危险函数调用
patch(default_black_list)
```

2. **渲染时使用安全机制**：
```python
# mako_render 内部已经使用 MakoSandbox 上下文管理器
# 编译时检查默认启用，会在模板编译前检查安全性
rendered_output = mako_render(template_content, context)
```

## 安全机制对比

### 原项目实现（bk-process-config-manager）

- ✅ 使用 `MakoSandbox` 上下文管理器
- ✅ 运行时补丁拦截危险函数（不拦截 `builtins` 模块）
- ✅ AST 语法树检查（编译时）
- ✅ 模板内容清理（替换制表符）

### 当前实现（bk-bscp）

- ✅ 使用 `MakoSandbox` 上下文管理器（完全一致）
- ✅ 运行时补丁拦截危险函数，并额外拦截 `posix` 以及 `builtins.open` / `io.open`（仍不拦截 `compile`、`exec`、`__import__`）
- ✅ AST 语法树检查（包括 `builtins` 中的危险名称）
- ✅ 模板内容清理（完全一致）
- ✅ `${expression}` 与代码块统一按 `"exec"` 解析；`SyntaxError` 拒绝渲染
- ✅ 编译时检查失败时抛出异常（阻止模板编译）

## 拦截示例

### 编译时拦截

以下模板会在编译时被拦截：

```python
# 模板内容：${open}
# 结果：ForbiddenMakoTemplateException("发现非法名称使用:[open]，请修改")

# 模板内容：${help()}
# 结果：ForbiddenMakoTemplateException("发现非法名称使用:[help]，请修改")

# 模板内容：<% import os %>
# 结果：ForbiddenMakoTemplateException("发现非法导入:[os]，请修改")
```

### 运行时拦截

以下模板代码会在运行时被拦截：

```python
# 模板内容：<% os.system("ls") %>
# 结果：ForbiddenMakoTemplateException("I am watching you!")

# 模板内容：<% subprocess.call(["ls"]) %>
# 结果：ForbiddenMakoTemplateException("I am watching you!")
```

### 允许的操作

以下操作是允许的：

```python
# 模板内容：${HELP}
# 结果：正常显示 HELP 内容（HELP 在白名单中）

# 模板内容：${datetime.datetime.now()}
# 结果：正常显示当前时间（datetime 在白名单中）

# 模板内容：${len([1, 2, 3])}
# 结果：正常显示 3（len 是安全的内置函数）
```

## 安全建议

1. **始终使用安全机制**：所有模板渲染都应该通过 `mako_render` 函数，它会自动应用安全保护
2. **限制上下文变量**：只传入必要的变量，避免传入敏感信息
3. **验证模板内容**：编译时检查会自动验证模板内容的合法性
4. **监控异常**：记录所有渲染异常，及时发现安全问题
5. **定期审查**：定期审查模板内容，确保没有安全风险

## 已知限制

1. **性能影响**：运行时补丁和上下文跟踪可能带来轻微的性能开销
2. **功能限制**：某些高级功能可能无法在安全模式下使用
3. **错误信息**：安全机制拦截的错误信息可能与普通模板错误不同
4. **builtins 其余入口**：运行时只拦截 `open`。`compile`、`exec`、`__import__` 仍依赖编译时检查，不能在运行时整体打补丁

## 参考文档

- [原项目实现](https://github.com/TencentBlueKing/bk-process-config-manager/tree/master/apps/utils/mako_utils)
- [Mako 官方文档](https://docs.makotemplates.org/)
- [Python AST 模块文档](https://docs.python.org/3/library/ast.html)
