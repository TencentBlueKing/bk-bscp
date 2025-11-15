# Render Package

Go 包，用于调用 Python Mako 模板渲染模块。

## 功能特性

- **通过 stdin 传递数据**：使用 JSON 格式通过标准输入传递模板和上下文
- **使用 uv 启动**：通过 uv 命令启动 Python 脚本，确保依赖隔离
- **支持超时控制**：可配置渲染操作的超时时间
- **临时文件支持**：对于大型上下文数据，支持通过临时文件传递
- **完善的错误处理**：详细的错误信息和 stderr 输出

## 安装

确保已安装 uv：

```bash
# macOS/Linux
curl -LsSf https://astral.sh/uv/install.sh | sh

# 或使用 pip
pip install uv
```

## 使用示例

### 基本用法

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/TencentBlueKing/bk-bscp/render"
)

func main() {
    // 创建渲染器
    renderer, err := render.NewRenderer()
    if err != nil {
        log.Fatal(err)
    }
    
    // 渲染模板
    template := "Hello ${name}!"
    context := map[string]interface{}{
        "name": "World",
    }
    
    result, err := renderer.Render(template, context)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println(result) // 输出: Hello World!
}
```

### 自定义配置

```go
// 自定义 uv 路径、脚本路径和超时时间
renderer, err := render.NewRenderer(
    render.WithUvPath("/usr/local/bin/uv"),
    render.WithScriptPath("custom/path/to/main.py"),
    render.WithTimeout(60 * time.Second),
)
```

### 使用上下文控制

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

result, err := renderer.RenderWithContext(ctx, template, context)
```

### 大数据场景（使用临时文件）

```go
// 对于大型 context，使用临时文件传递
result, err := renderer.RenderWithTempFile(template, largeContext)
```

### 从文件读取模板

```go
result, err := renderer.RenderWithFile("path/to/template.mako", context)
```

## API 文档

### NewRenderer

```go
func NewRenderer(opts ...RendererOption) (*Renderer, error)
```

创建新的渲染器实例。

**选项：**
- `WithUvPath(path string)`: 设置 uv 可执行文件路径
- `WithScriptPath(path string)`: 设置 Python 脚本路径
- `WithTimeout(timeout time.Duration)`: 设置渲染超时时间

### Render

```go
func (r *Renderer) Render(template string, context map[string]interface{}) (string, error)
```

渲染 Mako 模板，通过 stdin 传递数据。

### RenderWithContext

```go
func (r *Renderer) RenderWithContext(ctx context.Context, template string, context map[string]interface{}) (string, error)
```

使用 Go context 控制渲染过程（支持超时和取消）。

### RenderWithTempFile

```go
func (r *Renderer) RenderWithTempFile(template string, context map[string]interface{}) (string, error)
```

使用临时文件传递 context，适用于大型数据。

### RenderWithFile

```go
func (r *Renderer) RenderWithFile(templatePath string, context map[string]interface{}) (string, error)
```

从文件读取模板内容进行渲染。

## 错误处理

包提供了以下预定义错误：

- `ErrPythonNotFound`: Python 或 uv 可执行文件未找到
- `ErrScriptNotFound`: Python 脚本未找到
- `ErrRenderFailed`: 模板渲染失败
- `ErrInvalidInput`: 输入数据无效
- `ErrEncodeJSON`: JSON 编码失败
- `ErrDecodeJSON`: JSON 解码失败

使用 `RenderError` 类型获取详细错误信息，包括 stderr 输出：

```go
if err != nil {
    if renderErr, ok := err.(*render.RenderError); ok {
        fmt.Printf("Operation: %s\n", renderErr.Op)
        fmt.Printf("Error: %v\n", renderErr.Err)
        fmt.Printf("Stderr: %s\n", renderErr.Stderr)
    }
}
```

## 运行示例

```bash
# 运行示例代码
cd render/example
go run main.go
```

## 运行测试

```bash
cd render
go test -v
```

## 目录结构

```
render/
├── renderer.go          # 核心渲染器实现
├── types.go            # 类型定义
├── errors.go           # 错误定义
├── renderer_test.go    # 单元测试
├── README.md           # 本文档
├── example/
│   └── main.go         # 使用示例
└── python/             # Python 渲染模块
    ├── main.py
    ├── mako_render/
    └── ...
```

## 性能建议

1. **复用 Renderer 实例**：避免频繁创建 Renderer 对象
2. **选择合适的传递方式**：
   - 小数据（< 1MB）：使用 `Render()` 通过 stdin
   - 大数据（> 1MB）：使用 `RenderWithTempFile()`
3. **设置合理的超时**：根据模板复杂度调整超时时间
4. **使用 Context**：在 HTTP 请求等场景中使用 `RenderWithContext`

## 注意事项

1. 确保 Python 脚本路径正确（默认为 `render/python/main.py`）
2. uv 会自动管理 Python 依赖，首次运行可能需要安装依赖
3. 渲染失败时检查 stderr 输出以获取详细错误信息
4. Python 进程每次调用都会重新启动，如需高性能场景可考虑实现进程池

## 故障排查

### uv 未找到

```bash
# 检查 uv 是否安装
which uv

# 安装 uv
curl -LsSf https://astral.sh/uv/install.sh | sh
```

### Python 依赖问题

```bash
# 手动安装 Python 依赖
cd render/python
uv pip install -r requirements.txt
```

### 脚本路径问题

```go
// 使用绝对路径或相对于工作目录的路径
renderer, err := render.NewRenderer(
    render.WithScriptPath("/absolute/path/to/main.py"),
)
```
