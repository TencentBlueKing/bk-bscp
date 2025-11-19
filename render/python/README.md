# BK-BSCP Mako Template Rendering Module

这是 bk-bscp 项目中用于处理 Mako 模板渲染的 Python 模块。

## 功能特性

- **Mako 模板渲染**：支持 Mako 模板语法，高效渲染配置模板
- **模板缓存**：内置模板缓存机制，避免重复编译，提升性能
- **灵活的输入方式**：支持通过 stdin、文件或命令行参数传递模板和上下文
- **lxml 支持**：集成 lxml 库，方便进行 XML 处理

## 目录结构

```
python/
├── mako_render/          # 核心渲染模块
│   ├── __init__.py
│   └── render.py         # Mako 渲染核心逻辑
├── templates/            # 模板文件存放目录
├── main.py               # 主入口脚本
├── requirements.txt      # Python 依赖
├── pyproject.toml        # 项目配置
└── README.md             # 本文档
```

## 安装依赖

### 使用 pip

```bash
cd python
pip install -r requirements.txt
```

### 使用 uv (推荐)

```bash
cd python
uv pip install -r requirements.txt
```

## 使用方法

### 1. 通过 stdin 传递 JSON 数据

```bash
echo '{"template": "Hello ${name}!", "context": {"name": "World"}}' | python3 main.py --stdin
```

### 2. 通过文件传递模板和上下文

```bash
# 准备模板文件
echo 'Server: ${server_name}\nPort: ${port}' > template.mako

# 准备上下文文件
echo '{"server_name": "bk-bscp", "port": 8080}' > context.json

# 执行渲染
python3 main.py --template-file template.mako --context-file context.json
```

### 3. 通过命令行参数传递

```bash
python3 main.py --template 'Hello ${name}!' --context '{"name": "BSCP"}'
```

### 4. 在 Go 中调用

#### 方式一：通过 stdin 传递数据

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "os/exec"
)

type RenderInput struct {
    Template string                 `json:"template"`
    Context  map[string]interface{} `json:"context"`
}

func renderTemplate(template string, context map[string]interface{}) (string, error) {
    input := RenderInput{
        Template: template,
        Context:  context,
    }
    
    inputJSON, err := json.Marshal(input)
    if err != nil {
        return "", err
    }
    
    cmd := exec.Command("python3", "python/main.py", "--stdin")
    cmd.Stdin = bytes.NewReader(inputJSON)
    
    output, err := cmd.Output()
    if err != nil {
        return "", err
    }
    
    return string(output), nil
}

func main() {
    context := map[string]interface{}{
        "server_name": "bk-bscp",
        "port":        8080,
    }
    
    template := "Server: ${server_name}\nPort: ${port}"
    
    result, err := renderTemplate(template, context)
    if err != nil {
        panic(err)
    }
    
    fmt.Println(result)
}
```

#### 方式二：通过临时文件传递大数据

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
)

func renderTemplateWithFile(template string, context map[string]interface{}) (string, error) {
    // 写入临时文件
    tmpFile, err := os.CreateTemp("", "context-*.json")
    if err != nil {
        return "", err
    }
    defer os.Remove(tmpFile.Name())
    
    if err := json.NewEncoder(tmpFile).Encode(context); err != nil {
        return "", err
    }
    tmpFile.Close()
    
    // 执行渲染
    cmd := exec.Command("python3", "python/main.py", 
        "--template", template,
        "--context-file", tmpFile.Name())
    
    output, err := cmd.Output()
    if err != nil {
        return "", err
    }
    
    return string(output), nil
}
```

## API 说明

### render.py 模块

#### `get_cache_template(content: str) -> Template`

获取或创建缓存的 Mako 模板对象。

**参数：**
- `content`: 模板内容字符串

**返回：**
- 编译后的 Mako Template 对象

#### `mako_render(content: str, context: Dict[str, Any]) -> str`

使用给定的上下文渲染 Mako 模板。

**参数：**
- `content`: 模板内容字符串
- `context`: 包含模板变量的字典

**返回：**
- 渲染后的字符串

**异常：**
- `MakoException`: 模板渲染失败时抛出

### main.py 入口脚本

支持以下命令行参数：

- `--template`: 内联模板内容字符串
- `--template-file`: 模板文件路径
- `--context`: 内联上下文 JSON 字符串
- `--context-file`: 上下文 JSON 文件路径
- `--stdin`: 从 stdin 读取 JSON 输入（格式：`{"template": "...", "context": {...}}`）

## Context 数据结构说明

根据原 bk-process-config-manager 项目的 `get_process_context` 逻辑，Context 应该包含以下字段：

```json
{
  "Scope": "SetName.ModuleName.ServiceInstanceName.ProcessName.ProcessID",
  "FuncID": "process_name",
  "ModuleInstSeq": 1,
  "InstID0": 0,
  "HostInstSeq": 1,
  "LocalInstID0": 0,
  "bk_set_name": "set_name",
  "bk_module_name": "module_name",
  "bk_host_innerip": "127.0.0.1",
  "bk_cloud_id": 0,
  "bk_process_id": 123,
  "bk_process_name": "process_name",
  "FuncName": "func_name",
  "ProcName": "process_name",
  "WorkPath": "/data/work",
  "global_variables": {...}
}
```

这些变量需要由 Go 服务负责组装后传递给 Python 模块。

## 与原 Python 项目的兼容性

本模块保持了与 bk-process-config-manager 项目中 `apps/utils/mako_utils/render.py` 相同的核心逻辑：

- 模板缓存机制
- Mako 渲染逻辑
- 异常处理和错误追踪

主要区别在于：
- 去除了 Django 依赖
- 简化了 MakoSandbox 逻辑（可根据需要补充）
- 增加了命令行工具支持

## 注意事项

1. **大数据传递**：对于大型 context 数据，建议使用临时文件方式传递，避免命令行参数长度限制
2. **错误处理**：渲染失败时会输出详细的 Mako 错误追踪信息到 stderr
3. **模板缓存**：相同内容的模板会被缓存，避免重复编译
4. **Python 版本**：要求 Python >= 3.8

## TODO

- [ ] 补充 MakoSandbox 安全沙箱逻辑
- [ ] 添加 clean_mako_content 模板清理功能
- [ ] 增加单元测试
- [ ] 支持模板依赖（Ginclude）处理
- [ ] 添加更多 lxml 处理示例

## 许可证

本项目遵循与 bk-bscp 主项目相同的许可证。
