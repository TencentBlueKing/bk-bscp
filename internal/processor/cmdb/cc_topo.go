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

package cmdb

import (
	"context"
	"encoding/xml"
	"fmt"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// CCTopoXMLService 用于获取和构建 CC 拓扑 XML 的服务
// 参考 Python 代码中的 CMDBHandler.cache_topo_tree_attr 方法
type CCTopoXMLService struct {
	bizID int
	svc   bkcmdb.Service
	// 缓存字段列表，避免重复查询
	setFieldsCache    []string
	moduleFieldsCache []string
	hostFieldsCache   []string
}

const (
	// BK_SET_OBJ_ID Set 对象ID
	BK_SET_OBJ_ID = "set"
	// BK_MODULE_OBJ_ID Module 对象ID
	BK_MODULE_OBJ_ID = "module"
	// BK_HOST_OBJ_ID Host 对象ID
	BK_HOST_OBJ_ID = "host"
)

// NewCCTopoXMLService 创建 CC 拓扑 XML 服务
func NewCCTopoXMLService(bizID int, svc bkcmdb.Service) *CCTopoXMLService {
	return &CCTopoXMLService{
		bizID: bizID,
		svc:   svc,
	}
}

// GetTopoTreeXML 获取业务拓扑树的 XML 格式数据
// 参考 Python 代码中的 CMDBHandler.cache_topo_tree_attr(bk_set_env) 方法
// 参数 setEnv: 环境类型过滤（1-测试, 2-体验, 3-正式），如果为空则不过滤
// 返回包含所有 Set、Module、Host 及其属性的完整 XML 字符串
// 结构：Application -> Set -> Module -> Host
func (s *CCTopoXMLService) GetTopoTreeXML(ctx context.Context, setEnv string) (string, error) {
	// 1. 获取拓扑结构（使用 FindTopoBrief 接口，SearchBizInstTopo 已废弃）
	topoBrief, err := s.svc.FindTopoBrief(ctx, s.bizID)
	if err != nil {
		return "", fmt.Errorf("find topo brief failed: %w", err)
	}

	// 2. 从拓扑树中提取 Set ID 和 Module ID，并建立层级关系
	// 注意：setEnv 过滤在 buildSetsXML 阶段进行，因为那里已经有了 Set 的详细信息（包括 BkSetEnv）
	setModuleMap, setIDs, moduleIDs := s.extractTopoInfo(topoBrief)

	if len(setIDs) == 0 {
		return "", fmt.Errorf("no sets found for biz %d", s.bizID)
	}

	// 3. 批量获取 Set 完整属性
	setInfoMap, err := s.fetchSetDetails(ctx, setIDs)
	if err != nil {
		return "", fmt.Errorf("fetch set details failed: %w", err)
	}

	// 4. 批量获取 Module 完整属性
	moduleInfoMap, err := s.fetchModuleDetails(ctx, moduleIDs)
	if err != nil {
		return "", fmt.Errorf("fetch module details failed: %w", err)
	}

	// 5. 获取所有 Host 属性
	hostInfoMap, err := s.fetchHostDetails(ctx)
	if err != nil {
		return "", fmt.Errorf("fetch host details failed: %w", err)
	}

	// 6. 获取所有 Host 与 Module 的关系
	// 使用所有 Host ID 来获取关系，而不是只查询指定 Module 的关系
	// 这样可以获取所有 Host 的绑定关系，包括未绑定到当前 Module 的 Host
	allHostIDs := make([]int, 0, len(hostInfoMap))
	for hostID := range hostInfoMap {
		allHostIDs = append(allHostIDs, hostID)
	}

	hostModuleMap, err := s.fetchHostModuleRelationsByHostIDs(ctx, allHostIDs)
	if err != nil {
		return "", fmt.Errorf("fetch host module relations failed: %w", err)
	}

	// 7. 获取字段列表（用于补充缺失字段）
	setFields, err := s.getAllSetFields(ctx)
	if err != nil {
		return "", fmt.Errorf("get all set fields failed: %w", err)
	}
	moduleFields, err := s.getAllModuleFields(ctx)
	if err != nil {
		return "", fmt.Errorf("get all module fields failed: %w", err)
	}
	hostFields, err := s.getAllHostFields(ctx)
	if err != nil {
		return "", fmt.Errorf("get all host fields failed: %w", err)
	}

	// 8. 构建 XML 结构
	setsXML := s.buildSetsXML(setInfoMap, moduleInfoMap, hostInfoMap, setModuleMap, hostModuleMap, setEnv, setFields, moduleFields, hostFields)

	// 9. 生成 XML
	// 注意：Python 代码中使用的是 Application 作为根节点，不是 Business
	application := &ApplicationXML{
		Sets: setsXML,
	}

	// xml.MarshalIndent 会自动转义 XML 特殊字符（<, >, &, ", '）在属性值中
	// 这确保了即使 CMDB 数据包含这些特殊字符，生成的 XML 也是有效且安全的
	// 参考：https://pkg.go.dev/encoding/xml#Marshal
	xmlData, err := xml.MarshalIndent(application, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal XML failed: %w", err)
	}

	// 添加 XML 声明
	xmlStr := xml.Header + string(xmlData)
	return xmlStr, nil
}

// extractTopoInfo 从拓扑树中提取 Set 和 Module 信息
// 返回: setModuleMap(setID -> []moduleID), setIDs, moduleIDs
// 注意：setEnv 过滤不在这个阶段进行，因为此时还没有 Set 的详细信息（包括 BkSetEnv）
// setEnv 过滤在 buildSetsXML 阶段进行，那里已经有了完整的 Set 信息
func (s *CCTopoXMLService) extractTopoInfo(
	topoBrief *bkcmdb.TopoBriefResp,
) (map[int][]int, []int, []int) {
	setModuleMap := make(map[int][]int)
	setIDs := make([]int, 0)
	moduleIDs := make([]int, 0)
	setIDMap := make(map[int]bool)
	moduleIDMap := make(map[int]bool)

	// 递归遍历 TopoBriefNode 树
	var traverse func(nodes []*bkcmdb.TopoBriefNode, parentSetID int)
	traverse = func(nodes []*bkcmdb.TopoBriefNode, parentSetID int) {
		for _, node := range nodes {
			switch node.Obj {
			case "set":
				setID := node.ID
				// 收集所有 Set ID，后续在 buildSetsXML 阶段根据 setEnv 进行过滤
				if !setIDMap[setID] {
					setIDs = append(setIDs, setID)
					setIDMap[setID] = true
					setModuleMap[setID] = make([]int, 0)
				}
				// 递归处理子节点（Module）
				if len(node.Nodes) > 0 {
					traverse(node.Nodes, setID)
				}
			case "module":
				moduleID := node.ID
				if !moduleIDMap[moduleID] {
					moduleIDs = append(moduleIDs, moduleID)
					moduleIDMap[moduleID] = true
				}
				// 将 Module 关联到对应的 Set
				if parentSetID > 0 {
					setModuleMap[parentSetID] = append(setModuleMap[parentSetID], moduleID)
				}
			}
		}
	}

	// 遍历业务节点（Nodes）和空闲机池（Idle）
	// 注意：Idle 是空闲机池，通常也需要包含在拓扑中
	if len(topoBrief.Nodes) > 0 {
		traverse(topoBrief.Nodes, 0)
	}
	// 不处理空闲机池
	// if len(topoBrief.Idle) > 0 {
	// 	traverse(topoBrief.Idle, 0)
	// }

	return setModuleMap, setIDs, moduleIDs
}

// fetchSetDetails 批量获取 Set 的完整属性
// 参考 sync_cmdb.go 中的 fetchAllSets 实现方式
func (s *CCTopoXMLService) fetchSetDetails(ctx context.Context, setIDs []int) (map[int]*bkcmdb.SetInfo, error) {
	setInfoMap := make(map[int]*bkcmdb.SetInfo)

	if len(setIDs) == 0 {
		return setInfoMap, nil
	}

	// 构建 setID 映射用于快速查找
	setIDMap := make(map[int]bool, len(setIDs))
	for _, setID := range setIDs {
		setIDMap[setID] = true
	}

	// 获取所有字段（动态获取，与 Python 的 biz_global_variables 一致）
	fields, err := s.getAllSetFields(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all set fields failed: %w", err)
	}

	// 使用 PageFetcher 获取所有 Set，然后在内存中过滤
	allSets, err := PageFetcher(func(page *bkcmdb.PageParam) ([]bkcmdb.SetInfo, int, error) {
		resp, searchErr := s.svc.SearchSet(ctx, bkcmdb.SearchSetReq{
			BkSupplierAccount: "0",
			BkBizID:           s.bizID,
			Fields:            fields,
			Page:              page,
		})
		if searchErr != nil {
			return nil, 0, searchErr
		}

		return resp.Info, resp.Count, nil
	})
	if err != nil {
		return nil, fmt.Errorf("search set failed: %w", err)
	}

	// 过滤出需要的 Set
	for i := range allSets {
		setInfo := allSets[i]
		if setIDMap[setInfo.BkSetID] {
			setInfoMap[setInfo.BkSetID] = &setInfo
		}
	}

	return setInfoMap, nil
}

// fetchModuleDetails 批量获取 Module 的完整属性
func (s *CCTopoXMLService) fetchModuleDetails(ctx context.Context, moduleIDs []int) (map[int]*bkcmdb.ModuleInfo, error) {
	moduleInfoMap := make(map[int]*bkcmdb.ModuleInfo)

	// 分批查询（每批最多500个）
	batchSize := 500
	for i := 0; i < len(moduleIDs); i += batchSize {
		end := i + batchSize
		if end > len(moduleIDs) {
			end = len(moduleIDs)
		}
		batch := moduleIDs[i:end]

		// 获取所有字段（动态获取，与 Python 的 biz_global_variables 一致）
		fields, err := s.getAllModuleFields(ctx)
		if err != nil {
			return nil, fmt.Errorf("get all module fields failed: %w", err)
		}
		moduleInfos, err := s.svc.FindModuleBatch(ctx, &bkcmdb.ModuleReq{
			BkBizID: s.bizID,
			BkIDs:   batch,
			Fields:  fields,
		})
		if err != nil {
			return nil, fmt.Errorf("find module batch failed: %w", err)
		}

		for _, moduleInfo := range moduleInfos {
			moduleInfoMap[moduleInfo.BkModuleID] = moduleInfo
		}
	}

	return moduleInfoMap, nil
}

// fetchHostDetails 获取业务下所有 Host 的完整属性
func (s *CCTopoXMLService) fetchHostDetails(ctx context.Context) (map[int]*bkcmdb.HostInfo, error) {
	hostInfoMap := make(map[int]*bkcmdb.HostInfo)

	// 获取所有字段（动态获取，与 Python 的 biz_global_variables 一致）
	fields, err := s.getAllHostFields(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all host fields failed: %w", err)
	}

	// 分页获取所有主机
	page := &bkcmdb.PageParam{
		Start: 0,
		Limit: 500,
	}

	for {
		hosts, err := s.svc.ListBizHosts(ctx, &bkcmdb.ListBizHostsRequest{
			BkBizID: s.bizID,
			Page:    *page,
			Fields:  fields,
		})
		if err != nil {
			return nil, fmt.Errorf("list biz hosts failed: %w", err)
		}

		for i := range hosts.Info {
			hostInfo := hosts.Info[i]
			hostInfoMap[hostInfo.BkHostID] = &hostInfo
		}

		// 检查是否还有更多数据
		if page.Start+page.Limit >= hosts.Count {
			break
		}
		page.Start += page.Limit
	}

	return hostInfoMap, nil
}

// fetchHostModuleRelationsByHostIDs 通过 Host ID 获取 Host 与 Module 的关系
// 这样可以获取所有 Host 的绑定关系，包括未绑定到当前 Module 的 Host
func (s *CCTopoXMLService) fetchHostModuleRelationsByHostIDs(
	ctx context.Context,
	hostIDs []int,
) (map[int][]int, error) {
	// moduleID -> []hostID 的映射（一个 Host 可能属于多个 Module）
	moduleHostMap := make(map[int][]int)

	if len(hostIDs) == 0 {
		return moduleHostMap, nil
	}

	// 分批查询（每批最多500个 Host）
	batchSize := 500
	for i := 0; i < len(hostIDs); i += batchSize {
		end := i + batchSize
		if end > len(hostIDs) {
			end = len(hostIDs)
		}
		batch := hostIDs[i:end]

		// 使用 FindHostBizRelations 获取 Host 与业务的关系（包含 Module 信息）
		relations, err := s.svc.FindHostBizRelations(ctx, &bkcmdb.FindHostBizRelationsRequest{
			BkBizID:  s.bizID,
			BkHostID: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("find host biz relations failed: %w", err)
		}

		for _, rel := range relations {
			// 只处理有 Module 绑定的 Host（BkModuleID > 0）
			if rel.BkModuleID > 0 {
				moduleHostMap[rel.BkModuleID] = append(moduleHostMap[rel.BkModuleID], rel.BkHostID)
			}
		}
	}

	return moduleHostMap, nil
}

// buildSetsXML 构建 Set XML 结构
func (s *CCTopoXMLService) buildSetsXML(
	setInfoMap map[int]*bkcmdb.SetInfo,
	moduleInfoMap map[int]*bkcmdb.ModuleInfo,
	hostInfoMap map[int]*bkcmdb.HostInfo,
	setModuleMap map[int][]int,
	moduleHostMap map[int][]int,
	setEnv string,
	setFields []string,
	moduleFields []string,
	hostFields []string,
) []SetXML {
	var setsXML []SetXML

	for setID, moduleIDs := range setModuleMap {
		setInfo, exists := setInfoMap[setID]
		if !exists {
			logs.Warnf("set info not found for setID: %d", setID)
			continue
		}

		// 如果设置了 setEnv 过滤，只处理匹配的 Set
		if setEnv != "" && setInfo.BkSetEnv != setEnv {
			continue
		}

		setXML := convertSetInfoToXML(setInfo, setFields)

		// 构建 Module
		for _, moduleID := range moduleIDs {
			moduleInfo, exists := moduleInfoMap[moduleID]
			if !exists {
				logs.Warnf("module info not found for moduleID: %d", moduleID)
				continue
			}

			moduleXML := convertModuleInfoToXML(moduleInfo, moduleFields)

			// 构建 Host（通过 moduleHostMap 找到属于该 Module 的 Host）
			hostIDs := moduleHostMap[moduleID]
			for _, hostID := range hostIDs {
				hostInfo, exists := hostInfoMap[hostID]
				if !exists {
					logs.Warnf("host info not found for hostID: %d", hostID)
					continue
				}

				hostXML := convertHostInfoToXML(hostInfo, hostFields)
				moduleXML.Hosts = append(moduleXML.Hosts, hostXML)
			}

			setXML.Modules = append(setXML.Modules, moduleXML)
		}

		setsXML = append(setsXML, setXML)
	}

	return setsXML
}

// getSystemCommonAttributes 获取系统常用属性（对应 Python 的 BK_SYSTEM_COMMON_ATTRIBUTE）
func getSystemCommonAttributes(objID string) []string {
	switch objID {
	case BK_SET_OBJ_ID:
		return []string{
			"bk_set_name",
			"bk_set_env",
			"bk_service_status",
			"bk_world_id",
			"bk_platform",
			"bk_system",
			"bk_chn_name",
			"bk_category",
		}
	case BK_MODULE_OBJ_ID:
		return []string{
			"bk_module_name",
			"bk_module_type",
		}
	case BK_HOST_OBJ_ID:
		return []string{
			"bk_host_innerip",
			"bk_host_name",
			"operator",
			"bk_cloud_id",
		}
	default:
		return nil
	}
}

// getAllSetFields 获取所有 Set 字段列表（动态获取，与 Python 的 biz_global_variables 一致）
func (s *CCTopoXMLService) getAllSetFields(ctx context.Context) ([]string, error) {
	// 如果已缓存，直接返回
	if len(s.setFieldsCache) > 0 {
		return s.setFieldsCache, nil
	}

	// 使用 SearchObjectAttr 动态获取所有属性
	attrs, err := s.svc.SearchObjectAttr(ctx, bkcmdb.SearchObjectAttrReq{
		BkObjID: BK_SET_OBJ_ID,
		BkBizID: s.bizID,
	})
	if err != nil {
		return nil, fmt.Errorf("search set object attr failed: %w", err)
	}

	// 获取系统常用属性
	systemAttrs := getSystemCommonAttributes(BK_SET_OBJ_ID)
	systemAttrMap := make(map[string]bool, len(systemAttrs))
	for _, attr := range systemAttrs {
		systemAttrMap[attr] = true
	}

	// 提取字段列表（业务自定义属性 bk_biz_id != 0，或系统常用属性）
	// 参考 Python 代码的筛选逻辑
	fields := make([]string, 0, len(attrs))
	fieldMap := make(map[string]bool)

	for _, attr := range attrs {
		// 筛选：业务自定义属性（bk_biz_id != 0）或系统常用属性
		if attr.BkBizID != 0 || systemAttrMap[attr.BkPropertyID] {
			if !fieldMap[attr.BkPropertyID] {
				fields = append(fields, attr.BkPropertyID)
				fieldMap[attr.BkPropertyID] = true
			}
		}
	}

	// 补充基础字段（这些字段可能不在 SearchObjectAttr 返回的列表中，但需要包含）
	// 这些字段在 CMDB API 中通常会自动返回
	baseFields := []string{
		"bk_set_id", "bk_biz_id", "bk_capacity", "bk_set_desc",
		"set_template_id", "bk_supplier_account",
		"create_time", "last_time", "description",
		"bk_created_at", "bk_updated_at", "default", "bk_parent_id",
	}
	for _, field := range baseFields {
		if !fieldMap[field] {
			fields = append(fields, field)
			fieldMap[field] = true
		}
	}

	// 缓存结果
	s.setFieldsCache = fields
	return fields, nil
}

// getAllModuleFields 获取所有 Module 字段列表（动态获取，与 Python 的 biz_global_variables 一致）
func (s *CCTopoXMLService) getAllModuleFields(ctx context.Context) ([]string, error) {
	// 如果已缓存，直接返回
	if len(s.moduleFieldsCache) > 0 {
		return s.moduleFieldsCache, nil
	}

	// 使用 SearchObjectAttr 动态获取所有属性
	attrs, err := s.svc.SearchObjectAttr(ctx, bkcmdb.SearchObjectAttrReq{
		BkObjID: BK_MODULE_OBJ_ID,
		BkBizID: s.bizID,
	})
	if err != nil {
		return nil, fmt.Errorf("search module object attr failed: %w", err)
	}

	// 获取系统常用属性
	systemAttrs := getSystemCommonAttributes(BK_MODULE_OBJ_ID)
	systemAttrMap := make(map[string]bool, len(systemAttrs))
	for _, attr := range systemAttrs {
		systemAttrMap[attr] = true
	}

	// 提取字段列表（业务自定义属性 bk_biz_id != 0，或系统常用属性）
	// 参考 Python 代码的筛选逻辑
	fields := make([]string, 0, len(attrs))
	fieldMap := make(map[string]bool)

	for _, attr := range attrs {
		// 筛选：业务自定义属性（bk_biz_id != 0）或系统常用属性
		if attr.BkBizID != 0 || systemAttrMap[attr.BkPropertyID] {
			if !fieldMap[attr.BkPropertyID] {
				fields = append(fields, attr.BkPropertyID)
				fieldMap[attr.BkPropertyID] = true
			}
		}
	}

	// 补充基础字段（这些字段可能不在 SearchObjectAttr 返回的列表中，但需要包含）
	baseFields := []string{
		"bk_module_id", "bk_set_id", "bk_biz_id",
		"service_template_id", "operator", "bk_bak_operator",
		"service_category_id", "set_template_id",
		"host_apply_enabled", "bk_supplier_account",
		"create_time", "last_time",
		"bk_created_at", "bk_updated_at", "bk_created_by",
		"default", "bk_parent_id",
	}
	for _, field := range baseFields {
		if !fieldMap[field] {
			fields = append(fields, field)
			fieldMap[field] = true
		}
	}

	// 缓存结果
	s.moduleFieldsCache = fields
	return fields, nil
}

// getAllHostFields 获取所有 Host 字段列表（动态获取，与 Python 的 biz_global_variables 一致）
func (s *CCTopoXMLService) getAllHostFields(ctx context.Context) ([]string, error) {
	// 如果已缓存，直接返回
	if len(s.hostFieldsCache) > 0 {
		return s.hostFieldsCache, nil
	}

	// 使用 SearchObjectAttr 动态获取所有属性
	attrs, err := s.svc.SearchObjectAttr(ctx, bkcmdb.SearchObjectAttrReq{
		BkObjID: BK_HOST_OBJ_ID,
		BkBizID: s.bizID,
	})
	if err != nil {
		return nil, fmt.Errorf("search host object attr failed: %w", err)
	}

	// 获取系统常用属性
	systemAttrs := getSystemCommonAttributes(BK_HOST_OBJ_ID)
	systemAttrMap := make(map[string]bool, len(systemAttrs))
	for _, attr := range systemAttrs {
		systemAttrMap[attr] = true
	}

	// 提取字段列表（业务自定义属性 bk_biz_id != 0，或系统常用属性）
	// 参考 Python 代码的筛选逻辑
	fields := make([]string, 0, len(attrs))
	fieldMap := make(map[string]bool)

	for _, attr := range attrs {
		// 筛选：业务自定义属性（bk_biz_id != 0）或系统常用属性
		if attr.BkBizID != 0 || systemAttrMap[attr.BkPropertyID] {
			if !fieldMap[attr.BkPropertyID] {
				fields = append(fields, attr.BkPropertyID)
				fieldMap[attr.BkPropertyID] = true
			}
		}
	}

	// 补充基础字段（这些字段可能不在 SearchObjectAttr 返回的列表中，但需要包含）
	baseFields := []string{
		"bk_host_id", "bk_agent_id", "bk_cpu", "bk_mem", "bk_disk",
		"bk_os_name", "bk_os_type", "bk_os_version", "bk_os_bit",
		"bk_host_outerip", "bk_mac", "bk_outer_mac",
		"bk_comment", "bk_bak_operator",
		"bk_sla", "bk_sn", "bk_state",
		"import_from", "bk_asset_id", "bk_cloud_inst_id",
		"bk_cloud_vendor", "bk_cloud_host_status",
		"bk_cpu_architecture", "bk_cpu_module",
		"bk_host_innerip_v6", "bk_host_outerip_v6",
		"bk_isp_name", "bk_province_name", "bk_service_term",
		"bk_state_name",
	}
	for _, field := range baseFields {
		if !fieldMap[field] {
			fields = append(fields, field)
			fieldMap[field] = true
		}
	}

	// 缓存结果
	s.hostFieldsCache = fields
	return fields, nil
}

// BizGlobalVariables 业务全局变量结构
// 参考 Python 代码中的 biz_global_variables
type BizGlobalVariables struct {
	// TopoVariables 拓扑变量字段列表
	// 包含 Set、Module、Host 的所有字段列表，用于补充 XML 属性
	// 参考 Python 代码：topo_variables 用于 fillMissingFields
	TopoVariables struct {
		SetFields    []string `json:"set_fields"`    // Set 字段列表
		ModuleFields []string `json:"module_fields"` // Module 字段列表
		HostFields   []string `json:"host_fields"`   // Host 字段列表
	} `json:"topo_variables"`
	// 其他业务级全局变量可以在这里扩展
}

// GetBizGlobalVariables 获取业务全局变量
// 参考 Python 代码中的 biz_global_variables 获取逻辑
// 返回包含 topo_variables 等全局变量的结构
func (s *CCTopoXMLService) GetBizGlobalVariables(ctx context.Context) (*BizGlobalVariables, error) {
	// 获取 Set 字段列表
	setFields, err := s.getAllSetFields(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all set fields failed: %w", err)
	}

	// 获取 Module 字段列表
	moduleFields, err := s.getAllModuleFields(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all module fields failed: %w", err)
	}

	// 获取 Host 字段列表
	hostFields, err := s.getAllHostFields(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all host fields failed: %w", err)
	}

	return &BizGlobalVariables{
		TopoVariables: struct {
			SetFields    []string `json:"set_fields"`
			ModuleFields []string `json:"module_fields"`
			HostFields   []string `json:"host_fields"`
		}{
			SetFields:    setFields,
			ModuleFields: moduleFields,
			HostFields:   hostFields,
		},
	}, nil
}

// GetBizGlobalVariablesMap 获取业务全局变量（Map 格式，用于模板渲染）
// 返回 map[string]interface{} 格式，可以直接用于模板渲染上下文
// 参考 Python 代码：biz_global_variables 在模板渲染时会被合并到 context 中
// Python 代码逻辑：
//   - biz_global_variables 的结构：{ "set": [...], "module": [...], "host": [...] }
//   - 每个对象类型对应一个变量列表，每个变量包含 bk_property_id
//   - 在模板渲染时，会从 this.cc_set.attrib、this.cc_module.attrib、this.cc_host.attrib 中提取属性值
//   - 补充内置字段：for bk_obj_id, bk_obj_variables in biz_global_variables.items():
//     for variable in bk_obj_variables:
//     bk_property_id = variable["bk_property_id"]
//     context[bk_property_id] = getattr(this_context, f"cc_{bk_obj_id}").attrib.get(bk_property_id)
func (s *CCTopoXMLService) GetBizGlobalVariablesMap(ctx context.Context) (map[string]interface{}, error) {
	globalVars, err := s.GetBizGlobalVariables(ctx)
	if err != nil {
		return nil, err
	}

	// 转换为 map 格式，与 Python 代码的 biz_global_variables 结构一致
	// Python 代码结构：{ "set": [{"bk_property_id": "bk_set_name", ...}, ...], "module": [...], "host": [...] }
	result := make(map[string]interface{})

	// 构建按对象类型分组的变量列表
	// Python 代码中 biz_global_variables 按 bk_obj_id 分组（"set", "module", "host"）
	setVariables := make([]map[string]interface{}, 0, len(globalVars.TopoVariables.SetFields))
	for _, field := range globalVars.TopoVariables.SetFields {
		setVariables = append(setVariables, map[string]interface{}{
			"bk_property_id": field,
		})
	}

	moduleVariables := make([]map[string]interface{}, 0, len(globalVars.TopoVariables.ModuleFields))
	for _, field := range globalVars.TopoVariables.ModuleFields {
		moduleVariables = append(moduleVariables, map[string]interface{}{
			"bk_property_id": field,
		})
	}

	hostVariables := make([]map[string]interface{}, 0, len(globalVars.TopoVariables.HostFields))
	for _, field := range globalVars.TopoVariables.HostFields {
		hostVariables = append(hostVariables, map[string]interface{}{
			"bk_property_id": field,
		})
	}

	// 参考 Python 代码：biz_global_variables 按对象类型分组
	// 结构：{ "set": [...], "module": [...], "host": [...] }
	result["set"] = setVariables
	result["module"] = moduleVariables
	result["host"] = hostVariables

	// 同时保留 topo_variables 作为字段名列表（用于 fillMissingFields）
	// 合并所有字段（Set、Module、Host），因为 fillMissingFields 需要完整的字段列表
	allTopoFields := make([]string, 0)
	fieldMap := make(map[string]bool)

	for _, field := range globalVars.TopoVariables.SetFields {
		if !fieldMap[field] {
			allTopoFields = append(allTopoFields, field)
			fieldMap[field] = true
		}
	}
	for _, field := range globalVars.TopoVariables.ModuleFields {
		if !fieldMap[field] {
			allTopoFields = append(allTopoFields, field)
			fieldMap[field] = true
		}
	}
	for _, field := range globalVars.TopoVariables.HostFields {
		if !fieldMap[field] {
			allTopoFields = append(allTopoFields, field)
			fieldMap[field] = true
		}
	}
	result["topo_variables"] = allTopoFields

	return result, nil
}
