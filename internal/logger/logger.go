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

// Package logger 提供日志功能
package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
)

// Init 初始化 slog
func Init() {
	textHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelInfo,
		ReplaceAttr: ReplaceSourceAttr,
	})

	logger := slog.New(textHandler)
	slog.SetDefault(logger)
}

// ReplaceSourceAttr source 格式化为 dir/file:line 格式
func ReplaceSourceAttr(groups []string, a slog.Attr) slog.Attr {
	if a.Key != slog.SourceKey {
		return a
	}

	src, ok := a.Value.Any().(*slog.Source)
	if !ok {
		return a
	}

	a.Value = slog.StringValue(filepath.Base(src.File) + ":" + strconv.Itoa(src.Line))
	return a
}
