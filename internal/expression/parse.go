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
	"fmt"
	"strconv"
	"strings"
)

// matchType 对齐 gsekit parse.MatchType，标识 `[...]` 块内内容的类型。
type matchType int

const (
	matchWord           matchType = iota // 普通词字面量
	matchRange                           // 数字范围 / 单字符字母范围
	matchWordList                        // 逗号分隔词列表
	matchBuildInEnum                     // 以 `[` 开头 `]` 结尾的枚举
	matchBuildInExclude                  // 以 `!` 开头的排除
)

const (
	comma        = ","
	hyphen       = "-"
	leftBracket  = "["
	rightBracket = "]"
	exclamation  = "!"
)

// getMatchType 判定表达式类型（对齐 gsekit parse.get_match_type，判定优先级一致）。
func getMatchType(expression string) matchType {
	switch {
	case strings.HasPrefix(expression, leftBracket) && strings.HasSuffix(expression, rightBracket):
		return matchBuildInEnum
	case strings.HasPrefix(expression, exclamation):
		return matchBuildInExclude
	case strings.Contains(expression, comma):
		return matchWordList
	case strings.Contains(expression, hyphen):
		if isRangeFormat(expression) && (isSingleAlphaRange(expression) || isNumberRange(expression)) {
			return matchRange
		}
		return matchWord
	default:
		return matchWord
	}
}

// getRangeScope 拆分 `a-b` 为 (a, b)（假定为合法二元范围）。
func getRangeScope(rangeExpression string) (string, string) {
	parts := strings.Split(rangeExpression, hyphen)
	return parts[0], parts[1]
}

// isRangeFormat 判断是否恰好由一个 `-` 分成两段。
func isRangeFormat(expression string) bool {
	return len(strings.Split(expression, hyphen)) == 2
}

// isSingleAlphaRange 判断是否为单字符字母范围（同大小写、单字符、ascii 升序）。
func isSingleAlphaRange(rangeExpression string) bool {
	if !isRangeFormat(rangeExpression) {
		return false
	}
	begin, end := getRangeScope(rangeExpression)
	if len(begin) != 1 || len(end) != 1 {
		return false
	}
	if !isAlpha(begin[0]) || !isAlpha(end[0]) {
		return false
	}
	if isLower(begin[0]) != isLower(end[0]) {
		return false
	}
	return begin[0] < end[0]
}

// isNumberRange 判断是否为数字范围（begin/end 均十进制且 begin < end）。
func isNumberRange(rangeExpression string) bool {
	if !isRangeFormat(rangeExpression) {
		return false
	}
	begin, end := getRangeScope(rangeExpression)
	b, err1 := strconv.Atoi(begin)
	e, err2 := strconv.Atoi(end)
	if err1 != nil || err2 != nil {
		return false
	}
	// Python isdecimal 只接受非负十进制，负号会被判否
	if strings.HasPrefix(begin, "-") || strings.HasPrefix(end, "-") {
		return false
	}
	return b < e
}

func isAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isLower(c byte) bool { return c >= 'a' && c <= 'z' }

// parseWordListExpression 按逗号切分并去除各元素首尾空白（对齐 parse_word_list_expression）。
func parseWordListExpression(wordListExpression string) []string {
	parts := strings.Split(wordListExpression, comma)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// parseRangeExpression 将范围表达式展开为 fnmatch 字符集片段（对齐 parse_range_expression）。
func parseRangeExpression(rangeExpression string) ([]string, error) {
	if isSingleAlphaRange(rangeExpression) {
		return []string{"[" + rangeExpression + "]"}, nil
	}
	if isNumberRange(rangeExpression) {
		begin, end := getRangeScope(rangeExpression)
		b, _ := strconv.Atoi(begin)
		e, _ := strconv.Atoi(end)
		return Range2Re(b, e), nil
	}
	return nil, fmt.Errorf("%w: 范围表达式解析错误: %s", ErrExpressionSyntax, rangeExpression)
}

// parseEnumExpression 将 `[...]` 块内内容展开为一维候选值列表
// （对齐 gsekit parse_enum_expression + expand_list_element 的组合效果）。
func parseEnumExpression(enumExpression string) ([]string, error) {
	switch getMatchType(enumExpression) {
	case matchWord, matchBuildInEnum:
		return []string{enumExpression}, nil
	case matchBuildInExclude:
		return []string{"[" + enumExpression + "]"}, nil
	case matchWordList:
		var out []string
		for _, w := range parseWordListExpression(enumExpression) {
			sub, err := parseEnumExpression(w)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
		}
		return out, nil
	case matchRange:
		frags, err := parseRangeExpression(enumExpression)
		if err != nil {
			return nil, err
		}
		var out []string
		for _, f := range frags {
			sub, err := parseEnumExpression(f)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
		}
		return out, nil
	}
	return []string{enumExpression}, nil
}

// parseExp2UnixShellStyleMain 从左到右扫描 `[...]` 块并展开成多条 Unix shell 风格候选串
// （对齐 parse_exp2unix_shell_style_main：块外文本作前缀，块内展开，块间笛卡尔积）。
func parseExp2UnixShellStyleMain(expression string) ([]string, error) {
	parsed := []string{""}
	lastEnumEnd := -1
	enumBegin := strings.Index(expression, leftBracket)

	for enumBegin != -1 {
		enumEnd := strings.Index(expression[enumBegin:], rightBracket)
		if enumEnd == -1 {
			return nil, fmt.Errorf("%w: 枚举表达式缺少`]`: %s", ErrExpressionSyntax, expression[enumBegin:])
		}
		enumEnd += enumBegin

		enumContent := expression[enumBegin+1 : enumEnd]
		enumValues, err := parseEnumExpression(enumContent)
		if err != nil {
			return nil, err
		}

		prefix := expression[lastEnumEnd+1 : enumBegin]
		subParsed := make([]string, 0, len(enumValues))
		for _, v := range enumValues {
			subParsed = append(subParsed, prefix+v)
		}

		next := make([]string, 0, len(parsed)*len(subParsed))
		for _, p := range parsed {
			for _, s := range subParsed {
				next = append(next, p+s)
			}
		}
		parsed = next

		lastEnumEnd = enumEnd
		enumBegin = strings.Index(expression[enumEnd+1:], leftBracket)
		if enumBegin != -1 {
			enumBegin += enumEnd + 1
		}
	}

	tail := expression[lastEnumEnd+1:]
	for i := range parsed {
		parsed[i] += tail
	}
	return parsed, nil
}

// ParseExp2UnixShellStyle 解析表达式为去重后的 Unix shell 风格候选串
// （对齐 parse_exp2unix_shell_style）。
func ParseExp2UnixShellStyle(expression string) ([]string, error) {
	parsed, err := parseExp2UnixShellStyleMain(expression)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(parsed))
	out := make([]string, 0, len(parsed))
	for _, p := range parsed {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out, nil
}
