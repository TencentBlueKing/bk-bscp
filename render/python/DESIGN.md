# Python模块说明文档

## 目录结构

```
python/
├── mako_render/              # 核心渲染模块包
│   ├── __init__.py          # 模块初始化文件
│   └── render.py            # Mako渲染核心逻辑
├── templates/                # 示例模板目录
│   ├── example.mako         # 示例Mako模板
│   └── example_context.json # 示例上下文数据
├── main.py                   # 主入口脚本（命令行工具）
├── requirements.txt          # Python依赖列表
├── pyproject.toml           # Python项目配置
└── README.md                 # 详细使用文档
```

## 核心文件说明

### 1. `mako_render/render.py`

核心渲染逻辑，包含两个主要函数：

- **`get_cache_template(content)`**: 获取或创建缓存的模板对象
- **`mako_render(content, context)`**: 执行模板渲染

参考原项目：`bk-process-config-manager/apps/utils/mako_utils/render.py`

### 2. `main.py`

命令行入口脚本，支持多种调用方式：
- 通过 stdin 读取 JSON 数据
- 通过文件读取模板和上下文
- 通过命令行参数传递

### 3. `requirements.txt`

Python 依赖：
- `mako>=1.3.0` - 模板引擎
- `lxml>=4.9.0` - XML处理库

## Go 服务调用方式

### 方式一：通过 stdin 传递数据（推荐用于小数据）

```go
cmd := exec.Command("python3", "python/main.py", "--stdin")
input := map[string]interface{}{
    "template": "Hello ${name}!",
    "context": map[string]interface{}{"name": "World"},
}
inputJSON, _ := json.Marshal(input)
cmd.Stdin = bytes.NewReader(inputJSON)
output, err := cmd.Output()
```

### 方式二：通过临时文件传递（推荐用于大数据）

```go
// 1. 将 context 写入临时文件
tmpFile, _ := os.CreateTemp("", "context-*.json")
json.NewEncoder(tmpFile).Encode(context)
tmpFile.Close()

// 2. 调用 Python 脚本
cmd := exec.Command("python3", "python/main.py", 
    "--template", templateContent,
    "--context-file", tmpFile.Name())
output, err := cmd.Output()

// 3. 清理临时文件
os.Remove(tmpFile.Name())
```

## Context 数据结构

根据原 `get_process_context` 函数，Go 需要组装的 context 结构：

```go
type ProcessContext struct {
    Scope           string                 `json:"Scope"`
    FuncID          string                 `json:"FuncID"`
    ModuleInstSeq   int                    `json:"ModuleInstSeq"`
    InstID0         int                    `json:"InstID0"`
    HostInstSeq     int                    `json:"HostInstSeq"`
    LocalInstID0    int                    `json:"LocalInstID0"`
    BkSetName       string                 `json:"bk_set_name"`
    BkModuleName    string                 `json:"bk_module_name"`
    BkHostInnerIP   string                 `json:"bk_host_innerip"`
    BkCloudID       int                    `json:"bk_cloud_id"`
    BkProcessID     int                    `json:"bk_process_id"`
    BkProcessName   string                 `json:"bk_process_name"`
    FuncName        string                 `json:"FuncName"`
    ProcName        string                 `json:"ProcName"`
    WorkPath        string                 `json:"WorkPath"`
    GlobalVariables map[string]interface{} `json:"global_variables"`
    // ... 其他字段
}
```

## 后续 Go 封装建议

可以在 Go 项目中创建一个 `pkg/mako` 或 `internal/mako` 包，提供统一的渲染接口：

```go
package mako

type Renderer struct {
    pythonPath string
    scriptPath string
}

func NewRenderer(pythonPath, scriptPath string) *Renderer {
    return &Renderer{
        pythonPath: pythonPath,
        scriptPath: scriptPath,
    }
}

func (r *Renderer) Render(template string, context map[string]interface{}) (string, error) {
    // 实现调用逻辑
}

func (r *Renderer) RenderWithFile(templateFile string, context map[string]interface{}) (string, error) {
    // 实现文件方式调用
}
```

## 安装和测试

### 安装依赖

```bash
cd python
pip install -r requirements.txt
# 或使用 uv
uv pip install -r requirements.txt
```

### 测试运行

```bash
# 测试示例模板
cd python
python3 main.py \
  --template-file templates/example.mako \
  --context-file templates/example_context.json
```

预期输出：
```
Hello BSCP!

Your server is: bk-bscp-server
Port: 8080

Environment: production
```

## 注意事项

1. **Python 版本要求**：>=3.8
2. **大数据处理**：context 超过 1MB 建议用文件方式传递
3. **错误处理**：Python 脚本失败会返回非0退出码，Go 侧需检查
4. **性能优化**：考虑保持 Python 进程常驻，避免频繁启动开销
5. **安全性**：生产环境建议增加 MakoSandbox 沙箱限制

## 下一步工作

- [ ] Go 侧封装 Renderer 包
- [ ] 实现进程池管理（可选）
- [ ] 增加单元测试和集成测试
- [ ] 补充错误处理和日志记录
- [ ] 性能基准测试
