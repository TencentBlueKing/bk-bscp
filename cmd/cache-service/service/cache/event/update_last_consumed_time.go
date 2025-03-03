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

// Package event set app last consumed time
package event

import (
	"encoding/json"
	"fmt"

	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	lastConsumedTimePattern = `*bscp:app-last-consumed-time:*`
)

func (cm *ClientMetric) consumeAppLastConsumedTime(kt *kit.Kit) {
	// 先获取符合规则的key
	keys, err := cm.matchAppLastConsumedTimeKeys()
	if err != nil {
		logs.Errorf("the KEY is not matched, err: %s, rid: %s", err.Error(), kt.Rid)
		return
	}
	if len(keys) == 0 {
		logs.V(2).Infof("there is no matching KEY, rid: %s", kt.Rid)
		return
	}
	for _, key := range keys {
		lLen, err := cm.bds.LLen(kt.Ctx, key)
		if err != nil {
			logs.Errorf("get key: %s list length failed, err: %s", key, err.Error())
			continue
		}
		if lLen != 0 {
			cm.getLastConsumedTimeList(kt, key, lLen)
		}
	}
}

func (cm *ClientMetric) getLastConsumedTimeList(kt *kit.Kit, key string, listLen int64) {
	batchSize := 1000
	for i := 0; i < int(listLen); i += batchSize {
		startIndex := int64(i)
		endIndex := int64(i + batchSize - 1)
		if endIndex >= listLen {
			endIndex = listLen - 1
		}
		list, err := cm.bds.LRange(kt.Ctx, key, startIndex, endIndex)
		if err != nil {
			logs.Errorf("get key: %s  %v to %v client metric data failed, rid: %s, err: %s ", key,
				startIndex, endIndex, kt.Rid, err.Error())
			continue
		}
		appIDs := parseAndFilterJSON(list)
		if errB := cm.op.BatchUpdateLastConsumedTime(kt, appIDs); errB != nil {
			logs.Errorf("batch upsert client metrics failed, rid: %s, err: %s", kt.Rid, errB.Error())
			continue
		}

		_, err = cm.bds.LTrim(kt.Ctx, key, endIndex+1, -1)
		if err != nil {
			logs.Errorf("delete the Specify keys values data failed, key: %s, rid: %s, err: %s", key, kt.Rid, err.Error())
			continue
		}
	}
}

func parseAndFilterJSON(list []string) []uint32 {
	unique := make(map[uint32]struct{})
	var result []uint32

	for _, item := range list {
		var numbers []uint32
		// 使用 JSON 解析
		err := json.Unmarshal([]byte(item), &numbers)
		if err != nil {
			fmt.Printf("Failed to parse JSON: %s, error: %v\n", item, err)
			continue
		}

		// 去重逻辑
		for _, num := range numbers {
			if _, exists := unique[num]; !exists {
				unique[num] = struct{}{}
				result = append(result, num)
			}
		}
	}

	return result
}

func (cm *ClientMetric) matchAppLastConsumedTimeKeys() ([]string, error) {
	kt := kit.New()
	keys, err := cm.bds.Keys(kt.Ctx, lastConsumedTimePattern)
	if err != nil {
		return nil, err
	}

	return keys, nil
}
