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
	"errors"
	"fmt"
)

var (
	// ErrPythonNotFound is returned when Python or uv executable is not found
	ErrPythonNotFound = errors.New("python or uv executable not found")

	// ErrScriptNotFound is returned when the Python script is not found
	ErrScriptNotFound = errors.New("python script not found")

	// ErrRenderFailed is returned when template rendering fails
	ErrRenderFailed = errors.New("template rendering failed")

	// ErrInvalidInput is returned when input data is invalid
	ErrInvalidInput = errors.New("invalid input data")

	// ErrEncodeJSON is returned when JSON encoding fails
	ErrEncodeJSON = errors.New("failed to encode JSON")

	// ErrDecodeJSON is returned when JSON decoding fails
	ErrDecodeJSON = errors.New("failed to decode JSON")
)

// RenderError represents a rendering error with details
// nolint:revive
type RenderError struct {
	Op     string // operation that failed
	Err    error  // underlying error
	Stderr string // stderr output from Python script
}

func (e *RenderError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("%s: %v\nStderr: %s", e.Op, e.Err, e.Stderr)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *RenderError) Unwrap() error {
	return e.Err
}
