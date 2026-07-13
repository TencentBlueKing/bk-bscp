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

import "errors"

// 与 gsekit expression_utils/exceptions.py 对齐的表达式错误分类。
var (
	// ErrExpressionSyntax 表达式语法错误（如枚举缺少 `]`、范围非法）。
	ErrExpressionSyntax = errors.New("expression syntax error")
	// ErrExpressionParse 表达式解析异常（解析过程中的非预期错误）。
	ErrExpressionParse = errors.New("expression parse error")
	// ErrExpressionSlice 表达式切片解析异常。
	ErrExpressionSlice = errors.New("expression slice error")
)
