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

// Package space provides bscp space manager.
package space

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/bluele/gcache"
	"github.com/samber/lo"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	esbcli "github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

// Type 空间类型
type Type struct {
	ID     string
	Name   string
	EnName string
}

var (
	// BCS 项目类型
	BCS = Type{ID: "bcs", Name: "容器项目", EnName: "Container Project"}
	// BK_CMDB cmdb 业务类型
	BK_CMDB = Type{ID: "bkcmdb", Name: "业务", EnName: "Business"}
)

// Status 空间状态, 预留
type Status string

const (
	// SpaceNormal 正常状态
	SpaceNormal Status = "normal"
)

// Space 空间
type Space struct {
	SpaceId       string
	SpaceName     string
	SpaceTypeID   string
	SpaceTypeName string
	SpaceUid      string
	SpaceEnName   string
}

// Manager Space定时拉取
type Manager struct {
	ctx            context.Context
	requestedCache gcache.Cache // 用于检查cmdb空间是否请求过，避免短时间内高频刷新缓存
	spaceCache     gcache.Cache
}

// NewSpaceMgr Space按租户被动拉取, 注: 每个实例一个 cache
func NewSpaceMgr(ctx context.Context, client esbcli.Client) (*Manager, error) {
	mgr := &Manager{
		ctx:            ctx,
		requestedCache: gcache.New(1000).Expiration(time.Second * 30).EvictType(gcache.TYPE_LRU).Build(),
		spaceCache:     gcache.New(1000).Expiration(time.Minute * 10).EvictType(gcache.TYPE_LRU).Build(),
	}

	return mgr, nil
}

// AllSpaces 返回全量业务
func (s *Manager) AllSpaces(ctx context.Context) []*Space {
	kit := kit.MustGetKit(ctx)
	if cacheResult, err := s.spaceCache.Get(kit.TenantID); err == nil {
		return cacheResult.([]*Space)
	}

	spaceList, err := s.fetchAllSpace(ctx)
	if err != nil {
		slog.Error("fetch all space failed", "err", err)
		return []*Space{}
	}

	return spaceList
}

// GetSpaceByUID 按id查询业务
func (s *Manager) GetSpaceByUID(ctx context.Context, uid string) (*Space, error) {
	for _, v := range s.AllSpaces(ctx) {
		if v.SpaceId == uid {
			return v, nil
		}
	}
	return nil, fmt.Errorf("space %s not found", uid)
}

// QuerySpace 按uid批量查询业务
func (s *Manager) QuerySpace(ctx context.Context, spaceUidList []string) ([]*Space, error) {
	spaceList := []*Space{}
	spaceUidMap := map[string]struct{}{}

	for _, uid := range spaceUidList {
		spaceUidMap[uid] = struct{}{}
	}
	for _, v := range s.AllSpaces(ctx) {
		if _, ok := spaceUidMap[v.SpaceId]; ok {
			spaceList = append(spaceList, v)
		}
	}
	return spaceList, nil
}

// fetchAllSpace 获取全量业务列表
func (s *Manager) fetchAllSpace(ctx context.Context) ([]*Space, error) {
	bizList, err := bkcmdb.ListAllBusiness(ctx)
	if err != nil {
		return nil, err
	}

	if len(bizList) == 0 {
		return nil, fmt.Errorf("biz list is empty")
	}

	spaceList := make([]*Space, 0, len(bizList))
	for _, biz := range bizList {
		bizID := strconv.FormatInt(biz.BizID, 10)
		s := &Space{
			SpaceId:       bizID,
			SpaceName:     biz.BizName,
			SpaceTypeID:   BK_CMDB.ID,
			SpaceTypeName: BK_CMDB.Name,
			SpaceEnName:   BK_CMDB.EnName,
			SpaceUid:      BuildSpaceUid(BK_CMDB, strconv.FormatInt(biz.BizID, 10)),
		}
		spaceList = append(spaceList, s)
	}

	kit := kit.MustGetKit(ctx)
	if err = s.spaceCache.Set(kit.TenantID, spaceList); err != nil {
		slog.Error("set space cache failed", "tenant_id", kit.TenantID, "err", err)
	}

	slog.Info("fetch all space done", "tenant_id", kit.TenantID, "biz_count", len(spaceList))
	return spaceList, nil
}

// buildSpaceMap 分解
func buildSpaceMap(spaceUidList []string) (map[string][]string, error) {
	s := map[string][]string{}
	for _, uid := range spaceUidList {
		patterns := strings.Split(uid, "__")
		if len(patterns) != 2 {
			return nil, fmt.Errorf("space_uid not valid, %s", uid)
		}
		s[patterns[0]] = append(s[patterns[0]], patterns[1])
	}
	return s, nil
}

// BuildSpaceUid 组装 space_uid
func BuildSpaceUid(t Type, id string) string {
	return fmt.Sprintf("%s__%s", t.ID, id)
}

// HasCMDBSpace checks if cmdb space exists
func (s *Manager) HasCMDBSpace(ctx context.Context, spaceId string) bool {
	kit := kit.MustGetKit(ctx)
	key := fmt.Sprintf("%s/%s", kit.TenantID, spaceId)

	// 设置请求缓存
	defer s.requestedCache.Set(key, struct{}{}) // nolint:errcheck

	spaceList := s.AllSpaces(ctx)

	_, ok := lo.Find(spaceList, func(space *Space) bool {
		return space.SpaceId == spaceId
	})
	if ok {
		return true
	}

	// 最近较短时间内没有请求过该cmdb命名空间，则尝试重新拉取并刷新缓存
	if !s.requestedCache.Has(key) {
		ctx, cancel := context.WithTimeout(s.ctx, time.Second*10)
		defer cancel()

		spaceList, err := s.fetchAllSpace(ctx)
		if err != nil {
			slog.Error("fetch all space failed", "err", err)
			return false
		}

		_, ok := lo.Find(spaceList, func(space *Space) bool {
			return space.SpaceId == spaceId
		})
		if ok {
			return true
		}
	}

	return false
}
