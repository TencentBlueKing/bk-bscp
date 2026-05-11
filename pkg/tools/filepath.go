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

package tools

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

var (
	ErrInvalidFileName = errors.New("invalid file name")
	ErrPathTraversal   = errors.New("path traversal detected")
)

// ValidateFileName 校验单文件名是否合法
// 仅允许:
//
//	test.txt
//	a.yaml
//
// 禁止:
//
//	../a
//	a/b
//	/tmp/a
//	..\a
func ValidateFileName(kt *kit.Kit, name string) error {
	if name == "" || name == "." {
		return errors.New(i18n.T(kt, "invalid file name"))
	}

	// 禁止目录
	if name != filepath.Base(name) {
		return errors.New(i18n.T(kt, "invalid file name"))
	}

	// Windows 路径穿越
	if strings.Contains(name, `\`) {
		return errors.New(i18n.T(kt, "invalid file name"))
	}

	return nil
}

// SecureJoin 安全拼接路径，防止路径穿越
//
// 允许:
//
//	a/b/c.txt
//
// 禁止:
//
//	../a
//	/etc/passwd
//	..\a
func SecureJoin(kt *kit.Kit, baseDir, unsafeName string) (string, error) {
	if baseDir == "" {
		return "", errors.New(i18n.T(kt, "invalid base directory"))
	}

	unsafeName = filepath.FromSlash(unsafeName)

	cleanName := filepath.Clean(unsafeName)

	dstPath := filepath.Join(baseDir, cleanName)

	// 转绝对路径
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", errors.New(i18n.T(kt, "get absolute base path failed, baseDir: %s, err: %v", baseDir, err))
	}

	dstAbs, err := filepath.Abs(dstPath)
	if err != nil {
		return "", errors.New(i18n.T(kt, "get absolute destination path failed, path: %s, err: %v", dstPath, err))
	}

	// 防止路径逃逸
	rel, err := filepath.Rel(baseAbs, dstAbs)
	if err != nil {
		return "", errors.New(i18n.T(kt, "calculate relative path failed, base: %s, dst: %s, err: %v", baseAbs, dstAbs, err))
	}

	if strings.HasPrefix(rel, "..") {
		return "", errors.New(i18n.T(kt, "invalid file path"))
	}

	return dstAbs, nil
}

// SecureCreateFile 安全创建文件
func SecureCreateFile(kt *kit.Kit, baseDir, unsafeName string) (*os.File, string, error) {
	dstPath, err := SecureJoin(kt, baseDir, unsafeName)
	if err != nil {
		return nil, "", err
	}

	parentDir := filepath.Dir(dstPath)

	// 创建父目录
	if err = os.MkdirAll(parentDir, 0755); err != nil {
		return nil, "", errors.New(i18n.T(kt, "create parent directory failed, dir: %s, err: %v", parentDir, err))
	}

	f, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, "", errors.New(i18n.T(kt, "create file failed, path: %s, err: %v", dstPath, err))
	}

	return f, dstPath, nil
}

// SecureSaveFile 安全保存文件
func SecureSaveFile(
	kt *kit.Kit,
	reader io.Reader,
	baseDir,
	unsafeName string,
) error {

	f, dstPath, err := SecureCreateFile(kt, baseDir, unsafeName)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err = io.Copy(f, reader); err != nil {
		return errors.New(i18n.T(kt, "write file failed, path: %s, err: %v", dstPath, err))
	}

	return nil
}
