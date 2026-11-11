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

package apigw

import (
	"errors"
	"fmt"
	"regexp"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/TencentBlueKing/bk-bscp/docs"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

const (
	// mcpServersFile MCP Server 声明文件
	mcpServersFile = "swagger/bkapigw/mcp_servers.yaml"
	// mcpToolNameMaxLen 网关要求 MCP Server 全名与工具名直接拼接后不超过 64 字符
	mcpToolNameMaxLen = 64
	// mcpCategoryOfficial 官方资源分类
	mcpCategoryOfficial = "Official"
)

// ErrMcpSync MCP Server 同步或授权失败
var ErrMcpSync = errors.New("sync mcp servers failed")

var mcpServerNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type mcpServersDefinition struct {
	McpServers []McpServer `yaml:"mcp_servers"`
}

// loadMcpServers 解析 MCP Server 声明
func loadMcpServers(data []byte) ([]McpServer, error) {
	def := new(mcpServersDefinition)
	if err := yaml.Unmarshal(data, def); err != nil {
		return nil, fmt.Errorf("unmarshal mcp servers failed, err: %s", err.Error())
	}
	return def.McpServers, nil
}

// validateMcpServers 校验 MCP Server 声明，gwName、stage 用于计算工具名长度
func validateMcpServers(servers []McpServer, gwName, stage string) error {
	if len(servers) == 0 {
		return errors.New("no mcp server declared")
	}

	names := make(map[string]struct{}, len(servers))
	for _, s := range servers {
		if !mcpServerNameRe.MatchString(s.Name) {
			return fmt.Errorf("mcp server name %q must match %s", s.Name, mcpServerNameRe.String())
		}
		if _, ok := names[s.Name]; ok {
			return fmt.Errorf("mcp server name %q is duplicated", s.Name)
		}
		names[s.Name] = struct{}{}

		if s.Description == "" {
			return fmt.Errorf("mcp server %s: description is required", s.Name)
		}
		if s.Status == nil {
			return fmt.Errorf("mcp server %s: status is required", s.Name)
		}
		if s.IsPublic == nil {
			return fmt.Errorf("mcp server %s: is_public is required", s.Name)
		}
		if s.ProtocolType == "" {
			return fmt.Errorf("mcp server %s: protocol_type is required", s.Name)
		}
		if !slices.Contains(s.CategoryNames, mcpCategoryOfficial) {
			return fmt.Errorf("mcp server %s: category_names must contain %s", s.Name, mcpCategoryOfficial)
		}
		if len(s.ResourceNames) == 0 {
			return fmt.Errorf("mcp server %s: resource_names is required", s.Name)
		}

		fullName := fmt.Sprintf("%s-%s-%s", gwName, stage, s.Name)
		tools := make(map[string]struct{}, len(s.ResourceNames))
		for _, r := range s.ResourceNames {
			if _, ok := tools[r]; ok {
				return fmt.Errorf("mcp server %s: resource %s is duplicated", s.Name, r)
			}
			tools[r] = struct{}{}

			if len(fullName)+len(r) > mcpToolNameMaxLen {
				return fmt.Errorf("mcp server %s: %s + %s exceeds %d characters",
					s.Name, fullName, r, mcpToolNameMaxLen)
			}
		}
	}
	return nil
}

// mcpTargetAppCodes 授权应用：bscp 自身 app_code 与额外配置的应用，去重并保持顺序
func mcpTargetAppCodes(esbAppCode string, grantAppCodes []string) []string {
	return uniqueStrings(append([]string{esbAppCode}, grantAppCodes...))
}

// mcpResourceNames 所有 MCP Server 所用资源的并集，去重并保持顺序
func mcpResourceNames(servers []McpServer) []string {
	var names []string
	for _, s := range servers {
		names = append(names, s.ResourceNames...)
	}
	return uniqueStrings(names)
}

// syncMcpServers 同步网关环境下的 MCP Server，并按资源维度为授权应用授予 MCP 所用资源的权限。
// MCP 授权与资源授权相互独立，只授予 MCP 权限时调用方经 MCP 调用接口仍会被资源权限拦截。
func syncMcpServers(gw ApiGw, gwName, stage string, esbOpt cc.Esb, mcpOpt cc.ApiGatewayMcp) error {
	if !mcpOpt.Enabled {
		fmt.Println("mcp servers sync disabled, skip")
		return nil
	}

	data, err := docs.Assets.ReadFile(mcpServersFile)
	if err != nil {
		return fmt.Errorf("read %s failed, err: %s", mcpServersFile, err.Error())
	}
	servers, err := loadMcpServers(data)
	if err != nil {
		return err
	}
	if err = validateMcpServers(servers, gwName, stage); err != nil {
		return err
	}

	appCodes := mcpTargetAppCodes(esbOpt.AppCode, mcpOpt.GrantAppCodes)
	for i := range servers {
		servers[i].TargetAppCodes = appCodes
	}

	syncResp, err := gw.SyncStageMcpServers(gwName, stage, &SyncStageMcpServersReq{
		Name:       stage,
		McpServers: servers,
	})
	if err != nil {
		return err
	}
	for _, s := range syncResp.Data {
		fmt.Printf("mcp server synced, id: %d, name: %s, action: %s\n", s.ID, s.Name, s.Action)
	}

	resourceNames := mcpResourceNames(servers)
	for _, appCode := range appCodes {
		if _, err = gw.GrantPermissions(gwName, &GrantPermissionsReq{
			TargetAppCode:  appCode,
			GrantDimension: grantDimensionResource,
			ResourceNames:  resourceNames,
		}); err != nil {
			return fmt.Errorf("grant %s failed, err: %s", appCode, err.Error())
		}
		fmt.Printf("granted %d mcp resources to app %s\n", len(resourceNames), appCode)
	}

	return nil
}

func uniqueStrings(list []string) []string {
	result := make([]string, 0, len(list))
	seen := make(map[string]struct{}, len(list))
	for _, v := range list {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	return result
}
