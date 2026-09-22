/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package render

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

var (
	// defaultRenderer is a singleton Renderer instance reused across multiple calls
	defaultRenderer     *Renderer
	defaultRendererOnce sync.Once
	defaultRendererErr  error
)

const (
	// defaultPoolSizeCap 自动推导池大小时的上限，避免在高核数机器上启动过多常驻进程
	defaultPoolSizeCap = 16
	// poolSizeEnv 覆盖渲染进程池大小的环境变量
	poolSizeEnv = "BSCP_RENDER_POOL_SIZE"
	// workerRSSLimitEnv 覆盖 worker 常驻内存上限(MB)的环境变量
	workerRSSLimitEnv = "BSCP_RENDER_WORKER_RSS_MB"
	// defaultWorkerRSSLimitMB worker 常驻内存超过该值时重建进程
	defaultWorkerRSSLimitMB = 250
	// workerMaxUsesEnv 覆盖 worker 最大渲染次数的环境变量
	workerMaxUsesEnv = "BSCP_RENDER_WORKER_MAX_USES"
	// defaultWorkerMaxUses 读不到进程常驻内存时，按渲染次数兜底重建
	defaultWorkerMaxUses = 500
)

// GetDefaultRenderer returns a singleton Renderer instance
// It initializes the renderer on first call and reuses it for subsequent calls
func GetDefaultRenderer() (*Renderer, error) {
	defaultRendererOnce.Do(func() {
		defaultRenderer, defaultRendererErr = NewRenderer()
		if defaultRendererErr != nil {
			logs.Errorf("failed to initialize default renderer: %+v", defaultRendererErr)
		}
	})
	return defaultRenderer, defaultRendererErr
}

// Renderer handles Mako template rendering by reusing long-lived Python worker processes.
// 渲染请求通过常驻 worker 进程池处理，避免每次渲染都 fork uv/python。
type Renderer struct {
	// uvPath is the path to uv executable
	uvPath string
	// scriptPath is the path to the Python main.py script
	scriptPath string
	// timeout is the maximum duration for a single rendering operation
	timeout time.Duration
	// poolSize 是常驻 worker 进程数；<=0 表示自动推导
	poolSize int
	// workerRSSLimitMB 是 worker 常驻内存上限，超过则在归还时重建进程
	workerRSSLimitMB int
	// workerMaxUses 是读不到常驻内存时，触发重建的渲染次数阈值
	workerMaxUses int

	// mu 保护 workers 的延迟初始化与 Close
	mu sync.Mutex
	// workers 是空闲 worker 的调度通道；每个 worker 同一时刻只被一个 goroutine 持有
	workers chan *renderWorker
}

// renderWorker 表示池中的一个常驻渲染进程槽位。
// 进程采用懒启动：首次取用时才 spawn；崩溃后标记 dead，下次取用时重启。
type renderWorker struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	dead   bool
	// uses 是当前进程自启动以来完成的渲染次数，重建后归零
	uses int
}

// renderResponse 是常驻进程返回的一帧响应。
type renderResponse struct {
	Result string `json:"result"`
	Error  string `json:"error"`
}

// RendererOption is a function that configures a Renderer
type RendererOption func(*Renderer)

const gsekitRenderTimezone = "Asia/Shanghai"

// WithUvPath sets the path to uv executable
func WithUvPath(path string) RendererOption {
	return func(r *Renderer) {
		r.uvPath = path
	}
}

// WithScriptPath sets the path to Python script
func WithScriptPath(path string) RendererOption {
	return func(r *Renderer) {
		r.scriptPath = path
	}
}

// WithTimeout sets the timeout for rendering operations
func WithTimeout(timeout time.Duration) RendererOption {
	return func(r *Renderer) {
		r.timeout = timeout
	}
}

// WithPoolSize 设置常驻 worker 进程数；<=0 表示自动推导
func WithPoolSize(size int) RendererOption {
	return func(r *Renderer) {
		r.poolSize = size
	}
}

// WithWorkerRSSLimitMB 设置 worker 常驻内存上限(MB)，超过则在归还时重建进程
func WithWorkerRSSLimitMB(limit int) RendererOption {
	return func(r *Renderer) {
		r.workerRSSLimitMB = limit
	}
}

// WithWorkerMaxUses 设置读不到常驻内存时，触发重建的渲染次数阈值
func WithWorkerMaxUses(maxUses int) RendererOption {
	return func(r *Renderer) {
		r.workerMaxUses = maxUses
	}
}

// NewRenderer creates a new Renderer instance
func NewRenderer(opts ...RendererOption) (*Renderer, error) {
	// Get default script path from environment variable or use default
	defaultScriptPath := "render/python/main.py"
	if envPath := os.Getenv("BSCP_PYTHON_RENDER_PATH"); envPath != "" {
		// If environment variable is set, use it as the directory path and append main.py
		defaultScriptPath = filepath.Join(envPath, "main.py")
	}

	r := &Renderer{
		uvPath:           "uv", // default to uv in PATH
		scriptPath:       defaultScriptPath,
		timeout:          60 * time.Second,
		poolSize:         poolSizeFromEnv(),
		workerRSSLimitMB: intFromEnv(workerRSSLimitEnv, defaultWorkerRSSLimitMB),
		workerMaxUses:    intFromEnv(workerMaxUsesEnv, defaultWorkerMaxUses),
	}

	// Apply options
	for _, opt := range opts {
		opt(r)
	}

	// Validate uv executable
	if _, err := exec.LookPath(r.uvPath); err != nil {
		return nil, &RenderError{
			Op:  "NewRenderer",
			Err: ErrPythonNotFound,
		}
	}

	// Validate script path
	if _, err := os.Stat(r.scriptPath); err != nil {
		return nil, &RenderError{
			Op:  "NewRenderer",
			Err: fmt.Errorf("%w: %s", ErrScriptNotFound, r.scriptPath),
		}
	}

	return r, nil
}

// poolSizeFromEnv 从环境变量解析池大小，非法或缺省时返回 0（交由自动推导）
func poolSizeFromEnv() int {
	return intFromEnv(poolSizeEnv, 0)
}

// intFromEnv 解析正整数环境变量，缺省或非法时返回 def
func intFromEnv(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		logs.Warnf("invalid %s=%q, fallback to %d", key, v, def)
		return def
	}
	return n
}

// resolvePoolSize 在 size<=0 时按 CPU 数自动推导，并限制在 [1, defaultPoolSizeCap]
func resolvePoolSize(size int) int {
	if size <= 0 {
		size = runtime.NumCPU()
	}
	if size < 1 {
		size = 1
	}
	if size > defaultPoolSizeCap {
		size = defaultPoolSizeCap
	}
	return size
}

func renderCommandEnv() []string {
	return append(os.Environ(), "TZ="+gsekitRenderTimezone)
}

// ensurePool 延迟初始化进程池：填充固定数量的 worker 槽位（进程懒启动）。
func (r *Renderer) ensurePool() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.workers != nil {
		return
	}
	size := resolvePoolSize(r.poolSize)
	r.workers = make(chan *renderWorker, size)
	for i := 0; i < size; i++ {
		r.workers <- &renderWorker{}
	}
}

// spawn 启动一个常驻渲染进程并填充 worker 字段。
// 仅在全部管道建立、进程启动成功后才修改 w，失败时保持 w 原状以便后续重试。
func (r *Renderer) spawn(w *renderWorker) error {
	projectPath := filepath.Dir(r.scriptPath)
	// 启动常驻渲染进程。--no-sync 跳过依赖同步（构建期已 uv sync --frozen 预装），
	// 避免离线环境下访问 PyPI 失败。
	cmd := exec.Command(r.uvPath, "run", "--no-sync", "--project", projectPath,
		"python3", r.scriptPath, "--server")
	cmd.Env = renderCommandEnv()
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return &RenderError{Op: "Render", Err: fmt.Errorf("%w: open stdin: %v", ErrRenderFailed, err)}
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return &RenderError{Op: "Render", Err: fmt.Errorf("%w: open stdout: %v", ErrRenderFailed, err)}
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return &RenderError{Op: "Render", Err: fmt.Errorf("%w: start worker: %v", ErrRenderFailed, err)}
	}

	w.cmd = cmd
	w.stdin = stdin
	w.stdout = bufio.NewReader(stdout)
	w.dead = false
	w.uses = 0
	return nil
}

// terminate 终止进程并关闭其 stdin（不调用 Wait）。
// 用于超时打断阻塞中的读取；reap 必须在唯一的读取方退出后再调用。
func (w *renderWorker) terminate() {
	if w.cmd != nil && w.cmd.Process != nil {
		_ = w.cmd.Process.Kill()
	}
	if w.stdin != nil {
		_ = w.stdin.Close()
	}
}

// reap 回收已终止进程，避免僵尸。调用前必须保证没有 goroutine 仍在读取 stdout。
func (w *renderWorker) reap() {
	if w.cmd != nil {
		_ = w.cmd.Wait()
	}
}

// recycleIfNeeded 在归还 worker 前判断是否重建进程。
// Python 侧模板缓存淘汰后内存不会归还 OS，只有进程退出才能让 RSS 回落，
// 故以常驻内存为主判据，读不到时退化为渲染次数。
// 重建发生在本次渲染完成、结果已返回之后，不影响任何进行中的请求。
func (r *Renderer) recycleIfNeeded(w *renderWorker) {
	if w.cmd == nil || w.cmd.Process == nil || w.dead {
		return
	}
	w.uses++

	pid := w.cmd.Process.Pid
	var reason string
	if rssMB, ok := workerRSSMB(pid); ok {
		if rssMB >= r.workerRSSLimitMB {
			reason = fmt.Sprintf("rss %dMB >= %dMB", rssMB, r.workerRSSLimitMB)
		}
	} else if w.uses >= r.workerMaxUses {
		reason = fmt.Sprintf("uses %d >= %d", w.uses, r.workerMaxUses)
	}
	if reason == "" {
		return
	}

	logs.Infof("recycle render worker, pid: %d, reason: %s", pid, reason)
	w.terminate()
	w.reap()
	w.dead = true
}

// workerRSSMB 读取进程常驻内存(MB)；非 Linux 或读取失败时返回 ok=false。
func workerRSSMB(pid int) (int, bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/statm", pid))
	if err != nil {
		return 0, false
	}
	// statm 第二列为常驻页数
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0, false
	}
	pages, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return int(pages * int64(os.Getpagesize()) / (1024 * 1024)), true
}

// exchange 在当前 goroutine 内同步完成一次「写请求-读响应」。
// 返回 fatal=true 表示进程 IO 异常（需重启）；fatal=false 且 err!=nil 表示渲染级错误（进程仍可用）。
func (w *renderWorker) exchange(req []byte) (result string, fatal bool, err error) {
	if werr := writeFrame(w.stdin, req); werr != nil {
		return "", true, &RenderError{Op: "Render", Err: fmt.Errorf("%w: write request: %v", ErrRenderFailed, werr)}
	}
	payload, rerr := readFrame(w.stdout)
	if rerr != nil {
		return "", true, &RenderError{Op: "Render", Err: fmt.Errorf("%w: read response: %v", ErrRenderFailed, rerr)}
	}
	var resp renderResponse
	if uerr := json.Unmarshal(payload, &resp); uerr != nil {
		return "", true, &RenderError{Op: "Render", Err: fmt.Errorf("%w: %v", ErrDecodeJSON, uerr)}
	}
	if resp.Error != "" {
		return "", false, &RenderError{Op: "Render", Err: ErrRenderFailed, Stderr: resp.Error}
	}
	return resp.Result, false, nil
}

// writeFrame 写出一帧：4 字节大端长度前缀 + 负载。
func writeFrame(w io.Writer, payload []byte) error {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	return nil
}

// readFrame 读取一帧：4 字节大端长度前缀 + 负载。
func readFrame(r *bufio.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// isLiteralTemplate 保守判断模板是否为纯静态文本（不含任何 Mako 语法）。
// 命中则可直接原样返回，跳过渲染进程；任何不确定情况都返回 false 走正常渲染。
func isLiteralTemplate(s string) bool {
	if strings.Contains(s, "${") || strings.Contains(s, "<%") ||
		strings.Contains(s, "%>") || strings.Contains(s, "##") {
		return false
	}
	// 行首（忽略前导空白）出现 % 即为 Mako 控制行
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '\n' {
			line := s[start:i]
			j := 0
			for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
				j++
			}
			if j < len(line) && line[j] == '%' {
				return false
			}
			start = i + 1
		}
	}
	return true
}

// Render renders a Mako template with given context
func (r *Renderer) Render(template string, ctx map[string]interface{}) (string, error) {
	return r.RenderWithContext(context.Background(), template, ctx)
}

// RenderWithContext renders a Mako template with given context and Go context
func (r *Renderer) RenderWithContext(ctx context.Context, template string, contextData map[string]interface{}) (string, error) {
	if template == "" {
		return "", &RenderError{
			Op:  "Render",
			Err: fmt.Errorf("%w: template is empty", ErrInvalidInput),
		}
	}

	// 快路径：纯静态模板直接原样返回，不进入渲染进程池
	if isLiteralTemplate(template) {
		return template, nil
	}

	inputJSON, err := json.Marshal(RenderInput{Template: template, Context: contextData})
	if err != nil {
		return "", &RenderError{
			Op:  "Render",
			Err: fmt.Errorf("%w: %v", ErrEncodeJSON, err),
		}
	}

	// Create context with timeout
	if r.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	r.ensurePool()

	w := <-r.workers
	defer func() {
		r.recycleIfNeeded(w)
		r.workers <- w
	}()

	if w.cmd == nil || w.dead {
		if err := r.spawn(w); err != nil {
			return "", err
		}
	}

	// 看门狗：ctx 取消/超时时终止进程以打断阻塞中的读取。
	var wg sync.WaitGroup
	done := make(chan struct{})
	killed := false
	if ctx.Done() != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				killed = true
				w.terminate()
			case <-done:
			}
		}()
	}

	out, fatal, exErr := w.exchange(inputJSON)
	close(done)
	wg.Wait()

	if killed {
		w.dead = true
		w.reap()
		return "", &RenderError{Op: "Render", Err: fmt.Errorf("%w: %v", ErrRenderFailed, ctx.Err())}
	}
	if fatal {
		w.dead = true
		w.reap()
		return "", exErr
	}
	return out, exErr
}

// RenderWithFile renders a template from file with given context
func (r *Renderer) RenderWithFile(templatePath string, ctx map[string]interface{}) (string, error) {
	return r.RenderWithFileContext(context.Background(), templatePath, ctx)
}

// RenderWithFileContext renders a template from file with given context and Go context
func (r *Renderer) RenderWithFileContext(ctx context.Context, templatePath string, contextData map[string]interface{}) (string, error) {
	// Read template file
	template, err := os.ReadFile(templatePath)
	if err != nil {
		return "", &RenderError{
			Op:  "RenderWithFile",
			Err: fmt.Errorf("failed to read template file: %v", err),
		}
	}

	return r.RenderWithContext(ctx, string(template), contextData)
}

// RenderWithTempFile 历史上用于通过临时文件传递超大 context；
// 常驻进程采用长度前缀分帧，已能承载任意大小负载，故直接委托 RenderWithContext。
func (r *Renderer) RenderWithTempFile(template string, ctx map[string]interface{}) (string, error) {
	return r.RenderWithContext(context.Background(), template, ctx)
}

// RenderWithTempFileContext 见 RenderWithTempFile 说明，直接委托 RenderWithContext。
func (r *Renderer) RenderWithTempFileContext(ctx context.Context, template string, contextData map[string]interface{}) (string, error) {
	return r.RenderWithContext(ctx, template, contextData)
}

// Close 关闭进程池中的全部 worker，回收常驻进程。供测试清理与进程退出使用。
func (r *Renderer) Close() {
	r.mu.Lock()
	ch := r.workers
	r.workers = nil
	r.mu.Unlock()
	if ch == nil {
		return
	}
	for i := 0; i < cap(ch); i++ {
		w := <-ch
		w.terminate()
		w.reap()
	}
}

// GetScriptPath returns the absolute path to the Python script
func (r *Renderer) GetScriptPath() (string, error) {
	return filepath.Abs(r.scriptPath)
}
