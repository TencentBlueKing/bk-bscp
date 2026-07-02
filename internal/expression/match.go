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

package expression

import (
	"regexp"
	"strings"
)

// ExpressionSplitter 与 gsekit constants.EXPRESSION_SPLITTER 对齐，用于内存拼接五段表达式，
// 作为字段之间的字面锚点。仅在内存匹配时使用，不落库。
const ExpressionSplitter = "<-GSEKIT->"

// fnmatchTranslate 将 fnmatch 风格模式翻译为 Go regexp（对齐 CPython fnmatch.translate 语义）：
// `*`->`.*`，`?`->`.`，`[!seq]`->`[^seq]`，整体以 `\A(?s:...)\z` 做全串锚定匹配。
func fnmatchTranslate(pat string) string {
	var sb strings.Builder
	sb.WriteString(`\A(?s:`)

	i, n := 0, len(pat)
	for i < n {
		c := pat[i]
		i++
		switch c {
		case '*':
			sb.WriteString(".*")
		case '?':
			sb.WriteString(".")
		case '[':
			j := i
			if j < n && pat[j] == '!' {
				j++
			}
			if j < n && pat[j] == ']' {
				j++
			}
			for j < n && pat[j] != ']' {
				j++
			}
			if j >= n {
				// 无闭合 `]`，按字面量 `[` 处理
				sb.WriteString(`\[`)
			} else {
				sb.WriteString(translateCharClass(pat[i:j]))
				i = j + 1
			}
		default:
			// 按原始字节输出，仅转义 ASCII 正则元字符；
			// 不能用 string(byte) 转换（会把 >=0x80 的 UTF-8 字节当码点重新编码，破坏多字节字符）。
			if isRegexMeta(c) {
				sb.WriteByte('\\')
			}
			sb.WriteByte(c)
		}
	}

	sb.WriteString(`)\z`)
	return sb.String()
}

// isRegexMeta 判断是否为 Go regexp 需要转义的 ASCII 元字符。
func isRegexMeta(c byte) bool {
	switch c {
	case '\\', '.', '+', '*', '?', '(', ')', '|', '[', ']', '{', '}', '^', '$':
		return true
	default:
		return false
	}
}

// translateCharClass 将 fnmatch 字符集内容转为 Go regexp 字符集（对齐 fnmatch.translate）。
func translateCharClass(stuff string) string {
	var cls strings.Builder
	cls.WriteByte('[')

	start := 0
	if len(stuff) > 0 && stuff[0] == '!' {
		cls.WriteByte('^')
		start = 1
	} else if len(stuff) > 0 && (stuff[0] == '^' || stuff[0] == '[') {
		cls.WriteByte('\\')
	}
	for k := start; k < len(stuff); k++ {
		if stuff[k] == '\\' {
			cls.WriteString(`\\`)
		} else {
			cls.WriteByte(stuff[k])
		}
	}

	cls.WriteByte(']')
	return cls.String()
}

// Match 判断 name 是否匹配 expression（两层解析：`[...]` 预处理 + fnmatch 兜底）。
func Match(name, expression string) (bool, error) {
	candidates, err := ParseExp2UnixShellStyle(expression)
	if err != nil {
		return false, err
	}
	for _, cand := range candidates {
		re, err := regexp.Compile(fnmatchTranslate(cand))
		if err != nil {
			return false, err
		}
		if re.MatchString(name) {
			return true, nil
		}
	}
	return false, nil
}

// ListMatch 返回 names 中匹配 expression 的子集，保持相对 names 的顺序（对齐 list_match）。
func ListMatch(names []string, expression string) ([]string, error) {
	candidates, err := ParseExp2UnixShellStyle(expression)
	if err != nil {
		return nil, err
	}

	regexps := make([]*regexp.Regexp, 0, len(candidates))
	for _, cand := range candidates {
		re, err := regexp.Compile(fnmatchTranslate(cand))
		if err != nil {
			return nil, err
		}
		regexps = append(regexps, re)
	}

	matched := make(map[string]struct{}, len(names))
	for _, name := range names {
		for _, re := range regexps {
			if re.MatchString(name) {
				matched[name] = struct{}{}
				break
			}
		}
	}

	result := make([]string, 0, len(matched))
	for _, name := range names {
		if _, ok := matched[name]; ok {
			result = append(result, name)
		}
	}
	return result, nil
}
