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

package release

import (
	"context"
	"fmt"
	"hash/crc32"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/TencentBlueKing/bk-bscp/cmd/feed-server/bll/types"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/pkg/runtime/selector"
	ptypes "github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// GetMatchedRelease get the app instance's matched release id.
func (rs *ReleasedService) GetMatchedRelease(kt *kit.Kit, meta *types.AppInstanceMeta) (uint32, error) {

	ctx, cancel := context.WithTimeout(context.TODO(), rs.matchReleaseWaitTime)
	defer cancel()

	if err := rs.limiter.Wait(ctx); err != nil {
		return 0, err
	}

	am, err := rs.cache.App.GetMeta(kt, meta.BizID, meta.AppID)
	if err != nil {
		return 0, err
	}

	switch am.ConfigType {
	case table.File:
	case table.KV:
	default:
		return 0, errf.New(errf.InvalidParameter, "only supports File and KV configuration types.")
	}

	groups, err := rs.listReleasedGroups(kt, meta)
	if err != nil {
		return 0, err
	}

	matched, err := rs.matchReleasedGroupWithLabels(kt, groups, meta)
	if err != nil {
		return 0, err
	}

	return matched.ReleaseID, nil
}

// listReleasedGroups list released groups
func (rs *ReleasedService) listReleasedGroups(kt *kit.Kit, meta *types.AppInstanceMeta) (
	[]*ptypes.ReleasedGroupCache, error) {
	list, err := rs.cache.ReleasedGroup.Get(kt, meta.BizID, meta.AppID)
	if err != nil {
		return nil, fmt.Errorf("get current published strategy failed, err: %v", err)
	}

	return list, nil
}

type matchedMeta struct {
	StrategyID  uint32
	ReleaseID   uint32
	GroupID     uint32
	GrayPercent float64 // 灰度比例，用于选择最大比例的分组
}

// matchOneStrategyWithLabels match at most only one strategy with app instance labels.
func (rs *ReleasedService) matchReleasedGroupWithLabels(
	_ *kit.Kit,
	groups []*ptypes.ReleasedGroupCache,
	meta *types.AppInstanceMeta) (*matchedMeta, error) {
	// 1. sort released groups by update time
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].UpdatedAt.After(groups[j].UpdatedAt)
	})
	// 2. match groups with labels
	matchedList := []*matchedMeta{}
	var def *matchedMeta
	for _, group := range groups {
		switch group.Mode {
		case table.GroupModeDebug:
			if group.UID == meta.Uid {
				matchedList = append(matchedList, &matchedMeta{
					ReleaseID:  group.ReleaseID,
					GroupID:    group.GroupID,
					StrategyID: group.StrategyID,
				})
			}
		case table.GroupModeCustom:
			matched, grayPercent, err := rs.matchCustomGroupWithGrayStrategy(group, meta)
			if err != nil {
				return nil, err
			}

			if matched {
				matchedList = append(matchedList, &matchedMeta{
					ReleaseID:   group.ReleaseID,
					GroupID:     group.GroupID,
					StrategyID:  group.StrategyID,
					GrayPercent: grayPercent,
				})
			}
		case table.GroupModeDefault:
			def = &matchedMeta{
				ReleaseID:  group.ReleaseID,
				GroupID:    group.GroupID,
				StrategyID: group.StrategyID,
			}
		}
	}

	if len(matchedList) == 0 {
		if def == nil {
			return nil, errf.ErrAppInstanceNotMatchedRelease
		}
		return def, nil
	}

	// 在多个匹配的分组中选择最大灰度比例的分组（递进式灰度策略）
	selected := matchedList[0]
	for _, match := range matchedList {
		if match.GrayPercent > selected.GrayPercent {
			selected = match
			logs.Infof("Selected higher gray percent group: %d with %.2f%%",
				match.GroupID, match.GrayPercent*100)
		}
	}

	return selected, nil
}

// matchCustomGroupWithGrayStrategy 处理自定义分组的灰度匹配逻辑
func (rs *ReleasedService) matchCustomGroupWithGrayStrategy(
	group *ptypes.ReleasedGroupCache,
	meta *types.AppInstanceMeta,
) (matched bool, grayPercent float64, err error) {
	if group.Selector == nil {
		return false, 0, errf.New(errf.InvalidParameter, "custom group must have selector")
	}

	hasGrayPercent := false
	// 检查是否有灰度标签并解析比例
	for _, v := range group.Selector.LabelsAnd {
		if v.Key == table.GrayPercentKey {
			hasGrayPercent = true
			grayPercent = rs.parseGrayPercent(v.Value)
			break
		}
	}

	if hasGrayPercent {
		nonGrayMatched := false
		// 灰度策略：先匹配其他标签，再进行灰度匹配
		logs.Infof("Gray strategy detected, uid: %s, grayPercent: %.2f%%, selector: %v",
			meta.Uid, grayPercent*100, group.Selector)

		// 1. 先匹配除了gray_percent之外的其他标签
		nonGrayMatched, err = rs.matchNonGrayLabels(group.Selector, meta.Labels)
		if err != nil {
			return false, 0, err
		}

		if nonGrayMatched {
			// 2. 其他标签匹配成功，再进行灰度匹配
			matched, err = rs.matchReleasedGrayClients(group, meta)
			if err != nil {
				return false, 0, err
			}
		}
	} else {
		// 普通标签匹配
		matched, err = group.Selector.MatchLabels(meta.Labels)
		if err != nil {
			return false, 0, err
		}
	}

	return matched, grayPercent, nil
}

// matchReleasedGrayClients 匹配灰度客户端
// nolint: unparam
func (rs *ReleasedService) matchReleasedGrayClients(
	group *ptypes.ReleasedGroupCache,
	meta *types.AppInstanceMeta,
) (matched bool, err error) {
	// 1. 解析灰度百分比
	var grayPercent float64
	for _, label := range group.Selector.LabelsAnd {
		if label.Key == table.GrayPercentKey {
			// 处理不同类型的 value
			var percentStr string
			switch v := label.Value.(type) {
			case string:
				percentStr = v
			case float64:
				percentStr = fmt.Sprintf("%.1f%%", v)
			case int:
				percentStr = fmt.Sprintf("%d%%", v)
			default:
				logs.Warnf("unsupported gray_percent value type: %T", label.Value)
				continue
			}

			// 移除百分号并解析
			percentStr = strings.TrimSuffix(percentStr, "%")
			if percent, err := strconv.ParseFloat(percentStr, 64); err == nil {
				grayPercent = percent / 100.0
			}
			break
		}
	}

	if grayPercent <= 0 || grayPercent > 1 {
		logs.Warnf("invalid gray percent: %f for group %d", grayPercent, group.GroupID)
		return false, nil
	}

	// 2. 生成稳定的客户端哈希值
	// 关键设计：使用 UID + ReleaseID 确保同一版本下不同灰度分组的一致性
	hashSeed := fmt.Sprintf("%s:%d", meta.Uid, group.ReleaseID)
	hash := crc32.ChecksumIEEE([]byte(hashSeed))

	// 3. 映射到 [0, 1) 区间，使用高精度计算
	// 使用 math.MaxUint32 确保精度和一致性
	hashPercent := float64(hash) / float64(math.MaxUint32)

	// 4. 判断是否在灰度范围内
	// 这样设计确保：如果在20%时选中，在50%时一定还会选中
	matched = hashPercent < grayPercent

	// nolint:lll
	logs.Infof("Gray client matching - UID: %s, ReleaseID: %d, Hash: %d, HashPercent: %.6f%%, GrayPercent: %.2f%%, Matched: %v",
		meta.Uid, group.ReleaseID, hash, hashPercent*100, grayPercent*100, matched)

	return matched, nil
}

// parseGrayPercent 解析灰度百分比
func (rs *ReleasedService) parseGrayPercent(value interface{}) float64 {
	var percentStr string
	switch v := value.(type) {
	case string:
		percentStr = v
	case float64:
		percentStr = fmt.Sprintf("%.1f%%", v)
	case int:
		percentStr = fmt.Sprintf("%d%%", v)
	default:
		logs.Warnf("unsupported gray_percent value type: %T", value)
		return 0
	}

	// 移除百分号并解析
	percentStr = strings.TrimSuffix(percentStr, "%")
	if percent, err := strconv.ParseFloat(percentStr, 64); err == nil {
		return percent / 100.0
	}
	return 0
}

// matchNonGrayLabels 匹配除了gray_percent之外的其他标签
func (rs *ReleasedService) matchNonGrayLabels(sel *selector.Selector, labels map[string]string) (bool, error) {
	// 过滤掉gray_percent标签，只匹配其他标签
	filteredLabelsAnd := make([]selector.Element, 0)
	for _, element := range sel.LabelsAnd {
		if element.Key != table.GrayPercentKey {
			filteredLabelsAnd = append(filteredLabelsAnd, element)
		}
	}

	// 如果没有其他标签，直接返回true
	if len(filteredLabelsAnd) == 0 {
		return true, nil
	}

	// 创建临时selector进行匹配
	tempSelector := &selector.Selector{
		MatchAll:  sel.MatchAll,
		LabelsOr:  sel.LabelsOr,
		LabelsAnd: filteredLabelsAnd,
	}

	return tempSelector.MatchLabels(labels)
}
