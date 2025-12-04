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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// Renderer handles Mako template rendering by calling Python scripts
type Renderer struct {
	// uvPath is the path to uv executable
	uvPath string
	// scriptPath is the path to the Python main.py script
	scriptPath string
	// timeout is the maximum duration for rendering operation
	timeout time.Duration
}

// RendererOption is a function that configures a Renderer
type RendererOption func(*Renderer)

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

// NewRenderer creates a new Renderer instance
func NewRenderer(opts ...RendererOption) (*Renderer, error) {
	// Get default script path from environment variable or use default
	defaultScriptPath := "render/python/main.py"
	if envPath := os.Getenv("BSCP_PYTHON_RENDER_PATH"); envPath != "" {
		// If environment variable is set, use it as the directory path and append main.py
		defaultScriptPath = filepath.Join(envPath, "main.py")
	}

	r := &Renderer{
		uvPath:     "uv", // default to uv in PATH
		scriptPath: defaultScriptPath,
		timeout:    30 * time.Second, // default 30s timeout
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

// Render renders a Mako template with given context
// It uses stdin to pass JSON data to Python script
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

	// Prepare input data
	input := RenderInput{
		Template: template,
		Context:  contextData,
	}

	inputJSON, err := json.Marshal(input)
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

	// Execute Python script with uv
	cmd := exec.CommandContext(ctx, r.uvPath, "run", "--with", "mako", "--with", "lxml", "python3", r.scriptPath, "--stdin")
	cmd.Stdin = bytes.NewReader(inputJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	if err := cmd.Run(); err != nil {
		return "", &RenderError{
			Op:     "Render",
			Err:    fmt.Errorf("%w: %v", ErrRenderFailed, err),
			Stderr: stderr.String(),
		}
	}

	return stdout.String(), nil
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

// RenderWithTempFile renders a template using temporary file for large context data
// This is useful when context data is too large to pass via stdin
func (r *Renderer) RenderWithTempFile(template string, ctx map[string]interface{}) (string, error) {
	return r.RenderWithTempFileContext(context.Background(), template, ctx)
}

// RenderWithTempFileContext renders a template using temporary file with Go context
func (r *Renderer) RenderWithTempFileContext(ctx context.Context, template string, contextData map[string]interface{}) (string, error) {
	if template == "" {
		return "", &RenderError{
			Op:  "RenderWithTempFile",
			Err: fmt.Errorf("%w: template is empty", ErrInvalidInput),
		}
	}

	// Create temporary file for context
	tmpFile, err := os.CreateTemp("", "bk-bscp-context-*.json")
	if err != nil {
		return "", &RenderError{
			Op:  "RenderWithTempFile",
			Err: fmt.Errorf("failed to create temp file: %v", err),
		}
	}
	defer os.Remove(tmpFile.Name())

	// Write context to temp file
	if err := json.NewEncoder(tmpFile).Encode(contextData); err != nil {
		tmpFile.Close()
		return "", &RenderError{
			Op:  "RenderWithTempFile",
			Err: fmt.Errorf("%w: %v", ErrEncodeJSON, err),
		}
	}
	tmpFile.Close()

	// Create context with timeout
	if r.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	// Execute Python script with uv
	cmd := exec.CommandContext(ctx, r.uvPath, "run", "--with", "mako", "--with", "lxml", "python3", r.scriptPath,
		"--template", template,
		"--context-file", tmpFile.Name())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	if err := cmd.Run(); err != nil {
		return "", &RenderError{
			Op:     "RenderWithTempFile",
			Err:    fmt.Errorf("%w: %v", ErrRenderFailed, err),
			Stderr: stderr.String(),
		}
	}

	return stdout.String(), nil
}

// GetScriptPath returns the absolute path to the Python script
func (r *Renderer) GetScriptPath() (string, error) {
	return filepath.Abs(r.scriptPath)
}
