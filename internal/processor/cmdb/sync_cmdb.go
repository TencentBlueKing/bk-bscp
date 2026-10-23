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

// Package cmdb provides cmdb service.
package cmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

// SyncCMDBService 同步cmdb
type syncCMDBService struct {
	tenantID string
	bizID    int
	svc      bkcmdb.Service
	gseSvc   *gse.Service
	dao      dao.Set
	// redLock 同一 (tenant, biz) 全量同步的分布式租约；未注入时退化为进程内互斥
	redLock syncLocker
	// lockWait 单次抢锁的等待窗口；<=0 时取 processSyncLockWait（单测可缩短）
	lockWait time.Duration
}

// syncLocker 全量同步所需的互斥能力：生产实现为 *lock.TokenLock（带持有者 token 与续期），
// 单测注入桩实现
type syncLocker interface {
	// TryAcquire 尝试获取锁，不等待；获取失败返回 nil
	TryAcquire(res string) lock.Handle
}

// NewSyncCMDBService 初始化同步cmdb（多租户：tenantID 为空时按单租户 "default" 处理）
func NewSyncCMDBService(tenantID string, bizID int, svc bkcmdb.Service,
	gseSvc *gse.Service, dao dao.Set, redLock *lock.TokenLock) *syncCMDBService {
	s := &syncCMDBService{
		tenantID: tenantID,
		bizID:    bizID,
		svc:      svc,
		gseSvc:   gseSvc,
		dao:      dao,
	}
	// 显式判空后赋值：typed-nil 指针赋给接口会变成非 nil 接口，
	// 导致 s.redLock == nil 的降级判断失效并对 nil 锁调用 TryAcquire
	if redLock != nil {
		s.redLock = redLock
	}
	return s
}

const (
	// processSyncLockWait 等待同一业务全量同步锁的最长时间。
	// 超时说明其他实例正在做同样的全量对账，本次并未执行同步（返回 ErrSyncSkipped）。
	// 取值不宜过大：watcher 的事件页处理是同步阻塞的，等待锁会一并延迟事件消费。
	processSyncLockWait = 10 * time.Second
	// processSyncLockRetryInterval 抢锁重试间隔
	processSyncLockRetryInterval = time.Second
	// processSyncLockRenewInterval 续期窗口的兜底值：仅在锁实现未给出建议间隔时使用，
	// 正常取锁 TTL 的一半（600s TTL → 300s），留出一次续期失败的余量
	processSyncLockRenewInterval = time.Minute
)

// bizSyncLocks 进程内互斥锁，仅在未注入分布式锁时使用（单测或未接 Redis 的场景）。
// 引用计数包含持有者和等待者，最后一个调用退出后删除条目，避免业务 ID 变化时无界增长。
var bizSyncLocks = struct {
	sync.Mutex
	entries map[string]*localSyncLock
}{entries: make(map[string]*localSyncLock)}

type localSyncLock struct {
	mu   sync.Mutex
	refs int
}

// lockBizSyncLocal 获取 (tenant, biz) 维度的进程内互斥锁，已有同步在跑时阻塞等待。
func lockBizSyncLocal(tenantID string, bizID int) func() {
	key := fmt.Sprintf("%s-%d", tenantID, bizID)
	bizSyncLocks.Lock()
	entry := bizSyncLocks.entries[key]
	if entry == nil {
		entry = &localSyncLock{}
		bizSyncLocks.entries[key] = entry
	}
	entry.refs++
	bizSyncLocks.Unlock()
	if !entry.mu.TryLock() {
		logs.Infof("[SyncBizProcesses] wait for running sync, bizID=%d, tenant=%s", bizID, tenantID)
		entry.mu.Lock()
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			entry.mu.Unlock()
			bizSyncLocks.Lock()
			entry.refs--
			if entry.refs == 0 {
				delete(bizSyncLocks.entries, key)
			}
			bizSyncLocks.Unlock()
		})
	}
}

// processSyncLockResource 构造全量同步锁的资源名（锁实现内部会统一加 "Lock:" 前缀）。
// 以 tenant + biz 为维度：不同业务可并行，同一业务跨实例串行。
func processSyncLockResource(tenantID string, bizID int) string {
	return fmt.Sprintf("cmdb:process:sync:%s:%d", tenantID, bizID)
}

// ErrSyncSkipped 本次全量同步未执行：等待窗口内没抢到同一业务的全量同步锁。
//
// 调用方必须把它与「已执行且无变更」区分开：跳过时 CMDB 快照没有被重新拉取，
// 遗留的变更要靠后续的定时全量同步或一键同步覆盖，不能当成「已同步」上报：
//   - 事件驱动（watcher）路径：其他入口正在同步，本轮跳过即可，游标照常推进；
//   - 任务驱动（一键同步）路径：应如实上报「未执行」，不能当作成功。
var ErrSyncSkipped = errors.New("process full sync skipped: lock is held by another instance")

// IsSyncSkipped 判断错误是否表示「本轮全量同步未执行」（未抢到锁），而非执行失败
func IsSyncSkipped(err error) bool {
	return errors.Is(err, ErrSyncSkipped)
}

// lockProcessSync 获取 (tenant, biz) 维度的全量同步租约，返回受租约控制的 context 与解锁函数。
//
// watcher 事件、定时任务、一键同步等多个入口可能同时触发同一业务的全量同步，
// 并发执行会让两次全量 diff 互相覆盖（重复实例、重复进程行），大事务互锁还会引发回滚。
// 优先使用 Redis 分布式租约协调多实例；redLock 未注入时退化为进程内互斥。
//
// 锁在拉取全量数据之前获取，覆盖分页拉取、GSE 查询、事务内 diff 与写库，
// 大业务耗时可能超过锁 TTL，因此持锁期间按 TTL 的一半续期（见 keepProcessSyncLock）；
// 释放只作用于本次写入的 token，锁过期被接管后不会误删新持有者的锁。
//
// 错误语义：
//   - ErrSyncSkipped：等待超过 lockWait 仍抢不到锁，本次同步未执行，调用方需按「未执行」处理；
//   - 其他错误（如 ctx 取消）：调用方应直接返回，避免 shutdown 期间继续排队。
func (s *syncCMDBService) lockProcessSync(ctx context.Context, tenantID string) (context.Context, func(), error) {
	if s.redLock == nil {
		return ctx, lockBizSyncLocal(tenantID, s.bizID), nil
	}

	wait := s.lockWait
	if wait <= 0 {
		wait = processSyncLockWait
	}
	res := processSyncLockResource(tenantID, s.bizID)
	deadline := time.Now().Add(wait)
	for {
		if handle := s.redLock.TryAcquire(res); handle != nil {
			return s.keepProcessSyncLock(ctx, handle)
		}

		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if time.Now().After(deadline) {
			return nil, nil, ErrSyncSkipped
		}

		// 重试间隔内响应 ctx 取消，避免 shutdown 时最长空转；间隔不超过整个等待窗口，
		// 避免窗口小于重试间隔时空转到窗口外才判定超时
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(min(processSyncLockRetryInterval, wait)):
		}
	}
}

// keepProcessSyncLock 在持锁期间按 TTL 的一半续期；续期失败或父 context 取消会取消返回的工作 context。
//
// 全量同步的耗时随业务规模增长（分页拉取 + GSE 状态查询 + 上万行的事务内 diff 与写库），
// 不续期时一旦超过锁 TTL，其他实例会按「过期」接管，两个全量 diff 同时写同一业务。
// 返回的解锁函数先停掉续期 goroutine 再释放锁，保证释放发生时不再有并发续期。
func (s *syncCMDBService) keepProcessSyncLock(ctx context.Context, handle lock.Handle) (context.Context, func(), error) {
	interval := handle.RenewInterval()
	if interval <= 0 {
		interval = processSyncLockRenewInterval
	}

	lockCtx, cancel := context.WithCancel(ctx)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-lockCtx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				if err := handle.Renew(lockCtx); err != nil {
					logs.Errorf("[SyncBizProcesses] renew process sync lock failed, bizID=%d, tenant=%s, err: %v",
						s.bizID, s.getTenantID(), err)
					cancel()
					return
				}
			}
		}
	}()

	var once sync.Once
	unlock := func() {
		once.Do(func() {
			close(stop)
			<-done
			cancel()
			handle.Release()
		})
	}
	return lockCtx, unlock, nil
}

// SyncBizProcesses 全量同步业务进程：拉取业务全量进程后与库内数据做全量 diff，
// 返回同步结果供调用方派发进程状态同步任务。
// 注意：diff 的删除语义基于「新列表即业务全量」（SyncProcessData 会删除新列表外的存量进程），
// 禁止在此按进程 ID 做增量拉取——子集新列表会导致子集外的存量进程被全部误删。
//
// 返回的 error 为 ErrSyncSkipped 时表示本轮未执行（锁被其他实例占用），
// 调用方不能把它当成「已同步且无变更」处理。
func (s *syncCMDBService) SyncBizProcesses(ctx context.Context) (*SyncProcessResult, error) {
	kt := kit.MustGetKit(ctx)
	tenantID := s.getTenantID()

	// 同一 (tenant, biz) 跨实例串行执行，避免并发全量 diff 互相覆盖；
	// 锁被占用超时则本轮不执行，由调用方按「未执行」处理（watcher 跳过、任务如实上报）
	ctx, unlock, err := s.lockProcessSync(ctx, tenantID)
	if err != nil {
		if errors.Is(err, ErrSyncSkipped) {
			logs.Warnf("[SyncBizProcesses] another instance is syncing, this round is NOT executed, "+
				"bizID=%d, tenant=%s", s.bizID, tenantID)
			return nil, fmt.Errorf("[SyncBizProcesses] bizID=%d tenant=%s: %w", s.bizID, tenantID, ErrSyncSkipped)
		}
		return nil, fmt.Errorf("[SyncBizProcesses] acquire sync lock failed: %w", err)
	}
	defer unlock()
	kt.Ctx = ctx

	logs.Infof("[SyncBizProcesses] start, bizID=%d, tenant=%s", s.bizID, tenantID)

	// 1. 获取业务的进程相关信息（全量）
	processData, err := s.fetchAllProcessRelatedInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("[SyncBizProcesses] failed to fetch process related info: %w", err)
	}
	if errC := ctx.Err(); errC != nil {
		return nil, fmt.Errorf("[SyncBizProcesses] sync lock lost or context canceled: %w", errC)
	}
	if errV := validateProcessRelatedInfo(processData, s.bizID); errV != nil {
		return nil, fmt.Errorf("[SyncBizProcesses] invalid process related info: %w", errV)
	}

	// 2. 构建进程表数据
	processes := s.buildProcessEntities(kt, processData, tenantID)
	if len(processes) == 0 {
		if errB := checkEmptyProcessBuild(s.bizID, len(processData), len(processes)); errB != nil {
			return nil, errB
		}
		// 成功的空列表即「该业务在 CMDB 已无进程」：空列表不能提前返回，必须交给 diff
		// 按删除语义收敛库内存活行，否则最后一个进程被删除（或整业务进程被清空）后，
		// 本地行会一直停留在 synced，定时全量同步也反复走到这里
		logs.Warnf("[SyncBizProcesses] CMDB returned no process, bizID=%d, tenant=%s, "+
			"stored processes of this biz will be marked deleted", s.bizID, tenantID)
	}

	// 3. 回填服务模板 ID
	if errF := s.fillServiceTemplateID(ctx, processes); errF != nil {
		return nil, fmt.Errorf("[SyncBizProcesses] fill template id failed: %w", errF)
	}
	if errC := ctx.Err(); errC != nil {
		return nil, fmt.Errorf("[SyncBizProcesses] sync lock lost or context canceled: %w", errC)
	}

	// 4. diff 落库
	res, err := s.syncProcessesWithTx(kt, processes)
	if err != nil {
		return nil, err
	}

	logs.Infof("[SyncBizProcesses] success, bizID=%d, count=%d", s.bizID, len(processes))
	return res, nil
}

// checkEmptyProcessBuild 判断「构建后的进程列表为空」是否为异常，作为独立于「CC 成功空页」的信号：
//   - fetched == 0：CC 成功返回空列表，代表该业务在 CMDB 已无进程，返回 nil，
//     由调用方把空列表交给 diff 按删除语义收敛库内存活行；
//   - fetched > 0 但 built == 0：拉到了关联信息却一条进程都没构建出来（如 source_data
//     序列化失败逐条跳过），属构建阶段整体失败，必须返回错误放弃本轮，不能按删除处理。
//
// CC 权限、限流、接口异常不在此判断：它们由 fetchAllProcessRelatedInfo 的错误返回拦截。
func checkEmptyProcessBuild(bizID, fetched, built int) error {
	if built > 0 || fetched <= 0 {
		return nil
	}
	return fmt.Errorf("[SyncBizProcesses] fetched %d process related info but built none, bizID=%d",
		fetched, bizID)
}

// syncProcessesWithTx 在单个事务内完成「旧进程盘点 → diff → 落库 → 冲突检查」：
//  1. ListProcByBizIDWithTx 拉取库内该业务全部进程行（含软删除行，由 diffProcesses 内部区分）；
//     多租户模式下由全局租户钩子 + 下方显式过滤双重限定为当前租户，只对比本租户的进程；
//  2. SyncProcessData 做全量 diff 并写库（删除/新增/更新/实例扩缩容）；
//  3. CheckAndMarkHostAliasConflicts 做同主机同别名冲突检查与标记。
//
// 任一步骤失败整体回滚；调用方需保证 newProcesses 为业务全量（否则子集外进程会被误删）。
//
// legacy 数据（tenant_id 为空串）仅允许 default 租户参与 diff，与 DAO 租户钩子一致，
// 避免同号 bk_biz_id 的多个租户争抢同一批历史行。
func (s *syncCMDBService) syncProcessesWithTx(kt *kit.Kit, newProcesses []*table.Process) (*SyncProcessResult, error) {
	tx := s.dao.GenQuery().Begin()
	committed := false
	defer func() {
		if !committed {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kt.Rid)
			}
		}
	}()

	oldProcesses, err := s.dao.Process().ListProcByBizIDWithTx(kt, tx, uint32(s.bizID))
	if err != nil {
		return nil, fmt.Errorf("[syncProcesses] list old failed: %w", err)
	}

	// 只处理当前租户的进程，default 同时兼容历史空租户行。
	if s.tenantID != "" {
		filtered := make([]*table.Process, 0, len(oldProcesses))
		for _, p := range oldProcesses {
			if p != nil && p.Attachment != nil && sameOrLegacyTenant(p.Attachment.TenantID, s.getTenantID()) {
				filtered = append(filtered, p)
			}
		}
		oldProcesses = filtered
	}

	res, err := SyncProcessData(kt, s.dao, tx, uint32(s.bizID), oldProcesses, newProcesses)
	if err != nil {
		return nil, fmt.Errorf("[syncProcesses] sync failed: %w", err)
	}

	// 无变更时跳过冲突检查：冲突检查会对本业务进程做两次批量 UPDATE，
	// 而事件驱动的全量同步绝大多数是无变更场景，无谓写会放大数据库压力
	if res.Changed {
		if err = CheckAndMarkHostAliasConflicts(kt, s.dao, tx, uint32(s.bizID)); err != nil {
			return nil, fmt.Errorf("[syncProcesses] conflict check failed: %w", err)
		}
	}
	if err = kt.Ctx.Err(); err != nil {
		return nil, fmt.Errorf("[syncProcesses] sync lock lost or context canceled before commit: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("[syncProcesses] commit failed: %w", err)
	}

	committed = true

	return res, nil
}

// buildProcessEntities 将 CC 的进程关联信息列表转换为 BSCP 进程表实体。
//
// 输入 data 为 list_process_related_info 的原始条目，每个条目是一条
// (set, module, host, service_instance, process_template, process) 关联记录，
// 即「某进程在某服务实例/主机上的一个部署点」；同一 bk_process_id 绑定多个服务实例时会产出多条。
//
// 处理细节：
//   - agent 状态：先收集全部 bk_agent_id 批量查 GSE ListAgentState（失败不阻断主流程，
//     默认按 Abnormal 处理），再逐条回填；agent_id 为空的进程强制 Abnormal（见 resolveAgentStatus）；
//   - source_data：仅聚合进程操作相关字段（工作路径/启停命令/超时等，见 buildSourceData），
//     作为后续 diff 的配置一致性对比基准；
//   - os_type：CC 该接口不返回，置空，由 BuildProcessChanges 的 resolveOsType 沿用库内旧值；
//   - 单条构建失败（如 source_data 序列化异常）只跳过该条并告警，不影响其余条目。
//
//nolint:unparam // tenantID 在多租户场景下由 getTenantID() 动态返回，单租户部署时恒为 "default" 属预期行为
func (s *syncCMDBService) buildProcessEntities(kt *kit.Kit, data []*bkcmdb.ProcessRelatedInfoItem, tenantID string) []*table.Process {

	now := time.Now()
	result := make([]*table.Process, 0, len(data))

	// 处理 agent 状态：同一主机上的进程会重复命中同一 agent_id，先去重再分批查询，
	// 避免单次请求携带上万条重复 ID 触发 GSE 参数上限
	agentIDs := uniqueAgentIDs(data)
	statusMap := make(map[string]int, len(agentIDs))
	for start := 0; start < len(agentIDs); start += agentStateBatchLimit {
		end := min(start+agentStateBatchLimit, len(agentIDs))
		agentStatus, err := s.gseSvc.ListAgentState(kt.Ctx, &gse.ListAgentStateReq{
			AgentIDList: agentIDs[start:end],
		})
		if err != nil {
			logs.Errorf("fetch agent status failed, err: %v, agentIDs size: %d", err, end-start)
			// agent 状态查询失败不影响主流程，该批 agent 默认 AgentStatus 为 Abnormal
			continue
		}
		for _, st := range agentStatus {
			if st.BkAgentID == "" {
				continue
			}
			statusMap[st.BkAgentID] = st.StatusCode
		}
	}

	for _, item := range data {
		agentState := table.AgentStatusAbnormal
		if code, ok := statusMap[item.Host.BkAgentID]; ok && code == gseAgentStatusRunning {
			agentState = table.AgentStatusNormal
		}
		agentState = resolveAgentStatus(item.Host.BkAgentID, agentState)

		sourceData, err := s.buildSourceData(item)
		if err != nil {
			logs.Errorf("[buildProcessEntities] source data failed: %v", err)
			continue
		}

		// 非服务模板创建的进程，CC 可能不返回 process_template，此时模板 ID 落 0
		processTemplateID := uint32(0)
		if item.ProcessTemplate != nil {
			processTemplateID = uint32(item.ProcessTemplate.ID)
		}

		proc := &table.Process{
			Attachment: &table.ProcessAttachment{
				TenantID:          tenantID,
				BizID:             uint32(item.Process.BkBizID),
				CcProcessID:       uint32(item.Process.BkProcessID),
				SetID:             uint32(item.Set.BkSetID),
				ModuleID:          uint32(item.Module.BkModuleID),
				ServiceInstanceID: uint32(item.ServiceInstance.ID),
				HostID:            uint32(item.Host.BkHostID),
				CloudID:           uint32(item.Host.BkCloudID),
				AgentID:           item.Host.BkAgentID,
				ProcessTemplateID: processTemplateID,
			},
			Spec: &table.ProcessSpec{
				SetName:      item.Set.BkSetName,
				ModuleName:   item.Module.BkModuleName,
				ServiceName:  item.ServiceInstance.Name,
				Environment:  item.Set.BkSetEnv,
				Alias:        item.Process.BkProcessName,
				InnerIP:      item.Host.BkHostInnerIP,
				InnerIPV6:    item.Host.BkHostInnerIPV6,
				SourceData:   sourceData,
				PrevData:     "{}",
				ProcNum:      normalizeProcNum(item.Process.ProcNum),
				FuncName:     item.Process.BkFuncName,
				OsType:       "",
				CcSyncStatus: table.Synced,
				AgentStatus:  agentState,
				Priority:     item.Process.Priority,
			},
			Revision: &table.Revision{
				CreatedAt: now,
				UpdatedAt: now,
			},
		}

		result = append(result, proc)
	}

	return result
}

// validateProcessRelatedInfo prevents incomplete CMDB responses from becoming a partial
// full-sync input, which would make the diff mark valid stored processes as deleted.
//
// 必填关联：process / set / module / host / service_instance，且各 ID 必须在 (0, MaxUint32] 内。
// process_template 不强制要求：CC 对非服务模板创建的进程可能不返回该对象（或返回 0），
// 此时进程落库的 ProcessTemplateID 为 0；仅当取到非 0 值时校验其取值范围。
func validateProcessRelatedInfo(data []*bkcmdb.ProcessRelatedInfoItem, bizID int) error {
	for i, item := range data {
		if item == nil || item.Set == nil || item.Module == nil || item.Host == nil ||
			item.ServiceInstance == nil || item.Process == nil {
			return fmt.Errorf("item %d has missing process association", i)
		}
		ids := []struct {
			name string
			id   int64
		}{
			{"biz", item.Process.BkBizID},
			{"process", item.Process.BkProcessID},
			{"set", item.Set.BkSetID},
			{"module", item.Module.BkModuleID},
			{"host", item.Host.BkHostID},
			{"service instance", item.ServiceInstance.ID},
		}
		for _, id := range ids {
			if id.id <= 0 || uint64(id.id) > math.MaxUint32 {
				return fmt.Errorf("item %d has invalid %s ID %d", i, id.name, id.id)
			}
		}
		// 进程模板非必需，仅校验非 0 值的范围
		if item.ProcessTemplate != nil {
			if tplID := item.ProcessTemplate.ID; tplID != 0 && (tplID < 0 || uint64(tplID) > math.MaxUint32) {
				return fmt.Errorf("item %d has invalid process template ID %d", i, tplID)
			}
		}
		if item.Process.BkBizID != int64(bizID) {
			return fmt.Errorf("item %d belongs to biz %d, expected %d", i, item.Process.BkBizID, bizID)
		}
	}
	return nil
}

func (s *syncCMDBService) buildSourceData(item *bkcmdb.ProcessRelatedInfoItem) (string, error) {
	info := table.ProcessInfo{
		WorkPath:       item.Process.WorkPath,
		PidFile:        item.Process.PidFile,
		User:           item.Process.User,
		ReloadCmd:      item.Process.ReloadCmd,
		RestartCmd:     item.Process.RestartCmd,
		StartCmd:       item.Process.StartCmd,
		StopCmd:        item.Process.StopCmd,
		FaceStopCmd:    item.Process.FaceStopCmd,
		Timeout:        item.Process.Timeout,
		StartCheckSecs: item.Process.BkStartCheckSecs,
	}

	return info.Value()
}

// resolveOsType 决定进程最终 os_type：新值非空时采用新值，否则沿用旧值。
// 用于进程重建场景，避免同步取不到 os_type 时把已有类型覆盖为空（R-002）。
func resolveOsType(newOsType, oldOsType string) string {
	if newOsType != "" {
		return newOsType
	}
	return oldOsType
}

// resolveAgentStatus agent_id 为空时状态一律为 abnormal，否则会被判定为可同步、用空 agent_id 调用 GSE
func resolveAgentStatus(agentID string, status table.AgentStatus) table.AgentStatus {
	if agentID == "" {
		return table.AgentStatusAbnormal
	}
	return status
}

// normalizeProcNum 归一化进程数量：CMDB 约定 proc_num 为 0 时按 1 处理
func normalizeProcNum(n int) uint {
	if n <= 0 {
		return 1
	}
	return uint(n)
}

const (
	// gseAgentStatusRunning GSE agent 运行中状态码
	gseAgentStatusRunning = 2
	// agentStateBatchLimit 单次 ListAgentState 请求携带的 agent 数量上限
	agentStateBatchLimit = 500
)

// uniqueAgentIDs 收集去重后的非空 agent id，保持首次出现顺序
func uniqueAgentIDs(data []*bkcmdb.ProcessRelatedInfoItem) []string {
	ids := make([]string, 0, len(data))
	seen := make(map[string]struct{}, len(data))
	for _, item := range data {
		if item == nil || item.Host == nil || item.Host.BkAgentID == "" {
			continue
		}
		if _, ok := seen[item.Host.BkAgentID]; ok {
			continue
		}
		seen[item.Host.BkAgentID] = struct{}{}
		ids = append(ids, item.Host.BkAgentID)
	}
	return ids
}

const findModuleBatchLimit = 500

// uniquePositiveInts 去重且丢弃 <=0 的 ID，保持首次出现顺序
func uniquePositiveInts(ids []int) []int {
	out := make([]int, 0, len(ids))
	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// fillServiceTemplateID 根据去重后的 module_id 批量查询并填充 service_template_id
func (s *syncCMDBService) fillServiceTemplateID(ctx context.Context, processes []*table.Process) error {
	rawIDs := make([]int, 0, len(processes))
	for _, p := range processes {
		if p == nil || p.Attachment == nil {
			continue
		}
		rawIDs = append(rawIDs, int(p.Attachment.ModuleID))
	}
	moduleIDs := uniquePositiveInts(rawIDs)
	if len(moduleIDs) == 0 {
		return nil
	}

	mapping := make(map[int]int, len(moduleIDs))
	for _, batch := range chunkIntIDs(moduleIDs, findModuleBatchLimit) {
		resp, err := s.svc.FindModuleBatch(ctx, &bkcmdb.ModuleReq{
			BkBizID: s.bizID,
			BkIDs:   batch,
			Fields:  []string{"service_template_id", "bk_module_id"},
		})
		if err != nil {
			return err
		}
		for _, m := range resp {
			mapping[m.BkModuleID] = m.ServiceTemplateID
		}
	}

	for _, p := range processes {
		if p == nil || p.Attachment == nil {
			continue
		}
		if id, ok := mapping[int(p.Attachment.ModuleID)]; ok {
			p.Attachment.ServiceTemplateID = uint32(id)
		}
	}

	return nil
}

func (s *syncCMDBService) getTenantID() string {
	if s.tenantID == "" {
		return constant.DefaultTenantID
	}
	return s.tenantID
}

// syncTenantID 归一化本轮同步的生效租户，与写入侧 getTenantID 保持一致：
// 单租户模式下 kit 租户为空，写入落在 default（兼容 legacy 空串），
// 删除扫描按同一归一化结果比对，避免把本租户行误判为其他租户
func syncTenantID(kt *kit.Kit) string {
	if kt == nil || kt.TenantID == "" {
		return constant.DefaultTenantID
	}
	return kt.TenantID
}

func chunkIntIDs(src []int, size int) [][]int {
	if size <= 0 || len(src) == 0 {
		return nil
	}
	var res [][]int
	for i := 0; i < len(src); i += size {
		end := i + size
		if end > len(src) {
			end = len(src)
		}
		res = append(res, src[i:end])
	}
	return res
}

const (
	// pageFetcherLimit 每页请求条数；CC 可能对页大小设上限，实际以返回条数为准
	pageFetcherLimit = 500
	// maxPageFetcherPages 单次聚合拉取的翻页上限（约 10 万条），防止 CC 分页异常导致无限翻页
	maxPageFetcherPages = 200
)

// PageFetcher 封装 CC 分页接口的聚合拉取：从 start=0 起按页请求，
// 总数可信时取满 total 为止，总数不可信（count <= 0）时翻到空页为止。
//
// 推进策略：以「实际返回条数 len(data)」推进 start，而不是请求的 limit——CC 部分接口对页大小
// 有上限（实际返回 < limit），按 limit 推进会跳页漏数据；对全量 diff 场景，漏段会被误判为
// 「CMDB 已删除」而触发批量删除，必须避免。len(data)==0 时退出，防止 CC 对越界 start 重复
// 返回尾页导致死循环。
//
// 失败语义：无法确认已取满全量时必须返回错误，禁止返回部分数据。调用方的删除语义基于
// 「返回列表即业务全量」，部分数据会把漏掉的进程误判为已删除并批量软删。
//
// 泛型适配同构的 CC 分页响应（count + info），已被进程、集群、服务模板等拉取复用。
func PageFetcher[T any](fetch func(page *bkcmdb.PageParam) ([]T, int, error)) ([]T, error) {
	var (
		start = 0
		all   []T
		total = -1 // 首页返回的总数；<= 0 视为 CC 未提供可靠总数
	)

	for pages := 1; ; pages++ {
		data, count, err := fetch(&bkcmdb.PageParam{Start: start, Limit: pageFetcherLimit})
		if err != nil {
			return nil, err
		}

		all = append(all, data...)
		if pages == 1 {
			total = count
			// count 恰好等于首页条数且首页已取满 limit 时，无法区分「总数」与「本页条数」：
			// 保守地视为总数不可信，翻到空页为止（多一次请求，换取不漏页导致误删）
			if total > 0 && total == len(data) && len(data) >= pageFetcherLimit {
				total = -1
			}
		}

		if len(data) == 0 {
			// 空页但总数未取满：CC 分页在拉取过程中返回了空页（抖动/限流/越界），
			// 无法确认已取满全量，返回错误而不是部分数据
			if total > 0 && len(all) < total {
				return nil, fmt.Errorf("page fetcher got empty page before total reached: total=%d, fetched=%d",
					total, len(all))
			}
			break
		}

		// 总数可靠时以总数为准；总数不可靠（CC 返回 0/负数）时继续翻页直到空页，
		// 不能以「短页」判断结束：CC 可能把页大小截断到 limit 以下，提前结束会漏段，
		// 而全量 diff 会把漏段的进程误判为「CMDB 已删除」并批量删除
		if total > 0 && len(all) >= total {
			break
		}

		// 以实际返回条数推进游标：CC 返回条数可能小于请求的 limit，
		// 按 limit 推进会跳页漏数据；len(data) == 0 已在上面退出，不会死循环
		if pages >= maxPageFetcherPages {
			// 达到翻页上限仍无法确认取满：返回错误，由调用方放弃本轮同步，
			// 绝不能把部分列表当作全量交给 diff（会批量误删）
			return nil, fmt.Errorf("page fetcher reached max pages %d before total reached: total=%d, fetched=%d",
				maxPageFetcherPages, total, len(all))
		}
		start += len(data)
	}

	return all, nil
}

// diffProcesses 对比数据库中的进程列表与 CC 同步过来的进程列表，计算差异
//
// 职责：
//  1. 识别新增 / 更新 / 删除的进程
//  2. 计算实例的新增 / 删除（扩缩容）
//  3. 不做任何 DB 写操作，仅负责 diff 计算
//
// 返回值：
//   - ProcessDiff：包含进程与实例的所有变更集合
//   - error：构建 diff 过程中任一阶段失败
//
// nolint:funlen,gocyclo // diff 规则（新增/更新/删除/软删除恢复/重复行收敛）集中在此，拆分反而降低可读性
func diffProcesses(ctx *SyncContext, dbProcesses []*table.Process,
	newProcesses []*table.Process) (*ProcessDiff, error) {

	diff := &ProcessDiff{
		ModulesToReorder: make(map[ModuleAliasKey]struct{}),
	}

	// 0. 过滤残缺记录（Spec / Attachment 为空）：既无法参与 diff，后续字段访问还会 panic
	validProcesses := make([]*table.Process, 0, len(dbProcesses))
	for i, p := range dbProcesses {
		if p == nil || p.Spec == nil || p.Attachment == nil {
			logs.Errorf("[ProcessDiff] skip invalid db process record, index=%d", i)
			continue
		}
		// 索引前限定租户，避免同号 CC 进程优先匹配到其他租户的行。
		if !sameOrLegacyTenant(p.Attachment.TenantID, syncTenantID(ctx.Kit)) {
			continue
		}
		validProcesses = append(validProcesses, p)
	}
	dbProcesses = validProcesses

	// 1. 构建进程索引（以 CcProcessID 为主键）
	// 存活进程索引：cc_process_id -> 进程列表。唯一键为
	// (tenant_id, biz_id, cc_process_id, cc_sync_status, alias)，同 cc_process_id 可能存在多条
	// 不同别名的存活行（别名重建残留），因此用切片保留全部候选，由 pickProcessMatch 做确定性选择
	dbProcessesByCCID := make(map[uint32][]*table.Process, len(dbProcesses))
	// 软删除记录单独索引：CMDB 进程命中软删除记录时走「原地恢复」而不是新增，
	// 避免同 cc_process_id 出现重复存活行（此前误删的进程靠它恢复）
	deletedProcessByCCID := make(map[uint32]*table.Process)
	for _, p := range dbProcesses {
		if p.Spec != nil && p.Spec.CcSyncStatus == table.Deleted {
			deletedProcessByCCID[p.Attachment.CcProcessID] = p
			continue
		}
		dbProcessesByCCID[p.Attachment.CcProcessID] = append(
			dbProcessesByCCID[p.Attachment.CcProcessID], p)
	}

	// newProcesses 索引：cc_process_id -> process
	newProcessByCCID := make(map[uint32]*table.Process, len(newProcesses))
	for _, p := range newProcesses {
		newProcessByCCID[p.Attachment.CcProcessID] = p
	}

	// 2. 遍历 newProcesses，计算新增 / 更新 / 实例变更
	for _, newP := range newProcesses {
		matches := dbProcessesByCCID[newP.Attachment.CcProcessID]
		oldP, exists := pickProcessMatch(matches, newP.Spec.Alias)

		// 2.0 同 cc_process_id 的多条存活行（唯一键含 alias，别名重建会留下多条 synced 行）：
		// 保留与 CMDB 别名一致的基准行，其余在「实例全部停止且未托管」时收敛为 deleted，
		// 否则保留并告警（存在运行中实例时不可贸然删除）
		if exists {
			for _, dup := range matches {
				if dup.ID == oldP.ID {
					continue
				}
				// 租户一致性保护：单租户模式下列表未按租户过滤，禁止把其他租户的行当重复行删掉
				if dup.Attachment == nil ||
					!sameOrLegacyTenant(dup.Attachment.TenantID, newP.Attachment.TenantID) {
					logs.Warnf("[ProcessDiff][DupConverge] skip duplicate of other tenant, bizID=%d, "+
						"ccProcessID=%d, processID=%d, tenant=%s, currentTenant=%s",
						dup.Attachment.BizID, dup.Attachment.CcProcessID, dup.ID,
						dup.Attachment.TenantID, newP.Attachment.TenantID)
					continue
				}
				if !convergeDuplicateProcess(ctx, dup) {
					continue
				}
				diff.ToDeleteProcesses = append(diff.ToDeleteProcesses, dup)
				diff.ModulesToReorder[ModuleAliasKey{
					ModuleID: int(dup.Attachment.ModuleID),
					Alias:    dup.Spec.Alias,
				}] = struct{}{}
			}
		}

		// 2.1 新增进程
		if !exists {
			// 同 cc_process_id 存在软删除记录（此前误删或别名重建残留）：
			// 原地恢复该记录，否则会在恢复后出现同 cc_process_id 的重复存活行。
			// 显式属于其他租户的记录不恢复，避免跨租户把外部数据改成本租户数据
			delP, hasDeleted := deletedProcessByCCID[newP.Attachment.CcProcessID]
			if hasDeleted && (delP.Attachment == nil ||
				!sameOrLegacyTenant(delP.Attachment.TenantID, newP.Attachment.TenantID)) {
				logs.Warnf("[ProcessDiff][RestoreDeleted] skip deleted record of other tenant, bizID=%d, "+
					"ccProcessID=%d, processID=%d, tenant=%s, currentTenant=%s",
					delP.Attachment.BizID, newP.Attachment.CcProcessID, delP.ID,
					delP.Attachment.TenantID, newP.Attachment.TenantID)
				hasDeleted = false
			}
			if hasDeleted {
				restored, toAdd, toDelete, reorder, rErr := restoreDeletedProcess(ctx, delP, newP)
				if rErr != nil {
					logs.Errorf("[ProcessDiff][RestoreDeleted] bizID=%d ccProcessID=%d failed: %v",
						newP.Attachment.BizID, newP.Attachment.CcProcessID, rErr)
					return nil, rErr
				}
				// 该软删除记录已被本次恢复占用：同 cc_process_id 的后续条目不能再复用它，
				// 否则会对同一行重复产出更新、重复创建实例
				delete(deletedProcessByCCID, newP.Attachment.CcProcessID)
				diff.ToUpdateProcesses = append(diff.ToUpdateProcesses, restored)
				diff.ToAddInstances = append(diff.ToAddInstances, toAdd...)
				diff.ToDeleteInstanceIDs = append(diff.ToDeleteInstanceIDs, toDelete...)
				if reorder != nil {
					diff.ModulesToReorder[*reorder] = struct{}{}
				}
				continue
			}

			newP.Revision = &table.Revision{CreatedAt: ctx.Now}
			diff.ToAddProcesses = append(diff.ToAddProcesses, newP)

			// 查询同模块同别名的已有进程，获取最大 ModuleInstSeq
			maxModuleInstSeq := 0
			sameAliasProcessIDs, err := ctx.Dao.Process().ListByModuleIDAndAliasWithTx(
				ctx.Kit, ctx.Tx, newP.Attachment.BizID, newP.Attachment.ModuleID, newP.Spec.Alias)
			if err != nil {
				logs.Errorf("[ProcessDiff][ListByModuleIDAndAlias] bizID=%d moduleID=%d alias=%s failed: %v",
					newP.Attachment.BizID, newP.Attachment.ModuleID, newP.Spec.Alias, err)
				return nil, err
			}
			if len(sameAliasProcessIDs) > 0 {
				maxModuleInstSeq, err = ctx.Dao.ProcessInstance().GetMaxModuleInstSeqByProcessIDsWithTx(
					ctx.Kit, ctx.Tx, newP.Attachment.BizID, sameAliasProcessIDs)
				if err != nil {
					logs.Errorf("[ProcessDiff][GetMaxModuleInstSeq] bizID=%d processIDs=%v failed: %v",
						newP.Attachment.BizID, sameAliasProcessIDs, err)
					return nil, err
				}
			}

			// 为新增进程构建初始实例
			insts := buildInstances(ctx, &BuildInstancesParams{
				BizID:            newP.Attachment.BizID,
				HostID:           newP.Attachment.HostID,
				ModuleID:         newP.Attachment.ModuleID,
				CcProcessID:      newP.Attachment.CcProcessID,
				ProcNum:          int(newP.Spec.ProcNum),
				ExistCount:       0,
				MaxModuleInstSeq: maxModuleInstSeq,
				MaxHostInstSeq:   0,
				Alias:            newP.Spec.Alias,
			})

			diff.ToAddInstances = append(diff.ToAddInstances, insts...)
			continue
		}

		// 2.2 已存在进程，计算变更
		changeResult, err := BuildProcessChanges(ctx, &BuildProcessChangesParams{
			NewProcess: newP,
			OldProcess: oldP,
		})
		if err != nil {
			logs.Errorf(
				"[ProcessDiff][BuildProcessChanges] ccProcessID=%d processID=%d failed: %v",
				newP.Attachment.CcProcessID,
				oldP.ID,
				err,
			)
			return nil, err
		}

		if changeResult.ToAddProcess != nil {
			diff.ToAddProcesses = append(diff.ToAddProcesses, changeResult.ToAddProcess)
		}
		if changeResult.ToUpdateProcess != nil {
			diff.ToUpdateProcesses = append(diff.ToUpdateProcesses, changeResult.ToUpdateProcess)
		}
		if changeResult.ToDeleteProcess != nil {
			diff.ToDeleteProcesses = append(diff.ToDeleteProcesses, changeResult.ToDeleteProcess)
		}

		diff.ToAddInstances = append(diff.ToAddInstances, changeResult.ToAddInstances...)
		diff.ToDeleteInstanceIDs = append(diff.ToDeleteInstanceIDs, changeResult.ToDeleteInstanceIDs...)

		// 收集需要重新编排的模块
		if changeResult.ModuleToReorder != nil {
			diff.ModulesToReorder[*changeResult.ModuleToReorder] = struct{}{}
		}
	}

	// 3. db 中存在，但 new 中不存在 → 删除进程
	// 租户一致性保护与 2.0 收敛、restoreDeletedProcess 走同一判定：多租户模式下列表已按租户过滤，
	// 这层是重复保护；单租户模式下 syncProcessesWithTx 的显式过滤不生效（s.tenantID 为空），
	// 这里才是实际生效的一层，禁止把显式属于其他租户的行按删除语义收敛。
	// legacy 空租户行（归属历史数据）照常参与，见 sameOrLegacyTenant。
	tenantID := syncTenantID(ctx.Kit)
	for _, oldP := range dbProcesses {
		if _, ok := newProcessByCCID[oldP.Attachment.CcProcessID]; ok {
			continue
		}
		if !sameOrLegacyTenant(oldP.Attachment.TenantID, tenantID) {
			logs.Warnf("[ProcessDiff][Delete] skip process of other tenant, bizID=%d, ccProcessID=%d, "+
				"processID=%d, tenant=%s, currentTenant=%s",
				oldP.Attachment.BizID, oldP.Attachment.CcProcessID, oldP.ID,
				oldP.Attachment.TenantID, tenantID)
			continue
		}
		// 已软删除的行不需要重复标记：全量清理（如业务进程被清空）后这些行会长期留在库内，
		// 每轮同步都对它们重写 cc_sync_status 会让无变更的轮次持续产生写放大
		if oldP.Spec.CcSyncStatus == table.Deleted {
			continue
		}
		diff.ToDeleteProcesses = append(diff.ToDeleteProcesses, oldP)
	}

	return diff, nil
}

// sameOrLegacyTenant 判断库内行的租户是否可与当前同步数据合并：
// 空租户视为默认租户的历史遗留数据，与 DAO 租户钩子保持一致；
// 显式属于其他租户的行禁止参与删除/恢复，避免单租户模式（列表不按租户过滤）跨租户改动数据。
func sameOrLegacyTenant(oldTenant, newTenant string) bool {
	return oldTenant == newTenant || (oldTenant == "" && newTenant == constant.DefaultTenantID)
}

// pickProcessMatch 从同 cc_process_id 的存活行中确定性挑选 diff 基准行：
// 优先别名与 CMDB 一致的一条（避免把「别名已变更」误判为配置变更），
// 否则取 ID 最小的一条；不依赖数据库返回顺序，保证多次同步结果一致。
func pickProcessMatch(matches []*table.Process, alias string) (*table.Process, bool) {
	if len(matches) == 0 {
		return nil, false
	}
	picked := matches[0]
	for _, p := range matches {
		if p.Spec != nil && p.Spec.Alias == alias {
			return p, true
		}
		if p.ID < picked.ID {
			picked = p
		}
	}
	return picked, true
}

// convergeDuplicateProcess 判断同 cc_process_id 的多余存活行能否安全收敛为 deleted：
// 仅当其实例全部处于「已停止/无状态」且「未托管/无托管状态」时允许收敛，
// 否则保留并告警（可能存在运行中实例，删除会影响业务）。
func convergeDuplicateProcess(ctx *SyncContext, dup *table.Process) bool {
	safe, err := isSafeToUpdateProcess(ctx, dup.Attachment.BizID, dup.ID)
	if err != nil {
		logs.Errorf("[ProcessDiff][DupConverge] check safe failed, bizID=%d, processID=%d: %v",
			dup.Attachment.BizID, dup.ID, err)
		return false
	}
	if !safe {
		logs.Warnf("[ProcessDiff][DupConverge] keep duplicate process, bizID=%d, ccProcessID=%d, "+
			"processID=%d, alias=%s: running or managed instances exist",
			dup.Attachment.BizID, dup.Attachment.CcProcessID, dup.ID, dup.Spec.Alias)
		return false
	}

	logs.Infof("[ProcessDiff][DupConverge] duplicate process marked deleted, bizID=%d, ccProcessID=%d, "+
		"processID=%d, alias=%s",
		dup.Attachment.BizID, dup.Attachment.CcProcessID, dup.ID, dup.Spec.Alias)
	return true
}

// restoreDeletedProcess 原地恢复软删除的进程记录：以 CMDB 当前数据刷新规格与拓扑附件，
// 并按 ProcNum 对存量实例扩缩容。返回恢复后的进程（作为更新落库，保留原进程 ID 与实例历史）。
// 适用场景：此前误删/别名重建残留的软删除记录，在 CMDB 侧进程仍存在时的全量同步恢复
func restoreDeletedProcess(ctx *SyncContext, delP, newP *table.Process) (
	*table.Process, []*table.ProcessInstance, []uint32, *ModuleAliasKey, error) {

	// 整体覆盖 Spec 前先留存需要继承的字段
	prevSourceData := delP.Spec.SourceData
	prevOsType := delP.Spec.OsType
	prevStateSyncedAt := delP.Spec.ProcessStateSyncedAt

	delP.Spec = newP.Spec
	delP.Spec.PrevData = prevSourceData
	// 清除删除时记录的新别名，恢复为已同步状态
	delP.Spec.NewAlias = ""
	delP.Spec.CcSyncStatus = table.Synced
	// 同步取不到 os_type 时沿用旧值，避免恢复时把已有类型覆盖为空
	delP.Spec.OsType = resolveOsType(newP.Spec.OsType, prevOsType)
	// 保留原有的状态同步时间，避免恢复后监控出现「从未同步」的假象
	delP.Spec.ProcessStateSyncedAt = prevStateSyncedAt
	delP.Attachment = newP.Attachment
	delP.Revision = &table.Revision{UpdatedAt: ctx.Now}

	// 查询恢复进程的存量实例（标记删除时仅清理 stopped/unmanaged 实例，运行中的实例仍在）
	existingInsts, err := processInstances(ctx, delP.Attachment.BizID, delP.ID)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// 按 ProcNum 扩缩容实例
	res, err := reconcileProcessInstances(ctx, &ReconcileInstancesParams{
		BizID:       delP.Attachment.BizID,
		ProcessID:   delP.ID,
		HostID:      delP.Attachment.HostID,
		ModuleID:    delP.Attachment.ModuleID,
		CcProcessID: delP.Attachment.CcProcessID,
		Alias:       newP.Spec.Alias,
		OldNum:      len(existingInsts),
		NewNum:      int(newP.Spec.ProcNum),
	})
	if err != nil {
		return nil, nil, nil, nil, err
	}

	var reorder *ModuleAliasKey
	if len(res.ToAdd) > 0 || len(res.ToDelete) > 0 {
		reorder = &ModuleAliasKey{
			ModuleID: int(delP.Attachment.ModuleID),
			Alias:    newP.Spec.Alias,
		}
	}
	return delP, res.ToAdd, res.ToDelete, reorder, nil
}

// compareProcessInfo returns true if jsonStr (旧的 JSON 字符串) 等价于 jsonStr2 (新的 JSON 字符串).
func compareProcessInfo(jsonStr, jsonStr2 string) (bool, error) {
	var oldInfo table.ProcessInfo
	if err := json.Unmarshal([]byte(jsonStr), &oldInfo); err != nil {
		return false, err
	}

	var newInfo table.ProcessInfo
	if err := json.Unmarshal([]byte(jsonStr2), &newInfo); err != nil {
		return false, err
	}

	return reflect.DeepEqual(oldInfo, newInfo), nil
}

// InstanceReconcileResult 扩缩容实例
type InstanceReconcileResult struct {
	ToAdd    []*table.ProcessInstance
	ToDelete []uint32
}

// reconcileProcessInstances 处理进程实例扩缩容
func reconcileProcessInstances(ctx *SyncContext, params *ReconcileInstancesParams) (*InstanceReconcileResult, error) {
	res := &InstanceReconcileResult{}

	// 数量一致不做处理
	if params.NewNum == params.OldNum {
		return res, nil
	}

	// 缩容
	if params.NewNum < params.OldNum {
		needDelete := params.OldNum - params.NewNum
		insts, err := ctx.Dao.ProcessInstance().ListByProcessIDOrderBySeqDescTx(
			ctx.Kit, ctx.Tx, params.BizID, params.ProcessID, needDelete)
		if err != nil {
			return nil, err
		}

		for _, inst := range insts {
			res.ToDelete = append(res.ToDelete, inst.ID)
		}
	}

	// 扩容：查询同模块同别名的最大序列号
	if params.NewNum > params.OldNum {
		// 获取同模块同别名的所有进程ID
		sameAliasProcessIDs, err := ctx.Dao.Process().ListByModuleIDAndAliasWithTx(
			ctx.Kit, ctx.Tx, params.BizID, params.ModuleID, params.Alias)
		if err != nil {
			return nil, err
		}

		maxModuleSeq := 0
		if len(sameAliasProcessIDs) > 0 {
			maxModuleSeq, err = ctx.Dao.ProcessInstance().GetMaxModuleInstSeqByProcessIDsWithTx(
				ctx.Kit, ctx.Tx, params.BizID, sameAliasProcessIDs)
			if err != nil {
				return nil, err
			}
		}

		maxHostSeq, err := ctx.Dao.ProcessInstance().GetMaxHostInstSeqTx(
			ctx.Kit, ctx.Tx, params.BizID, []uint32{params.ProcessID})
		if err != nil {
			return nil, err
		}

		// 查询当前进程在该主机上已有的 HostInstSeq 列表
		existInsts, err := processInstances(ctx, params.BizID, params.ProcessID)
		if err != nil {
			return nil, err
		}
		usedHostInstSeqs := make([]int, 0, len(existInsts))
		for _, inst := range existInsts {
			usedHostInstSeqs = append(usedHostInstSeqs, int(inst.Spec.HostInstSeq))
		}

		res.ToAdd = buildInstances(ctx, &BuildInstancesParams{
			BizID:            params.BizID,
			HostID:           params.HostID,
			ModuleID:         params.ModuleID,
			CcProcessID:      params.CcProcessID,
			ProcNum:          params.NewNum,
			ExistCount:       params.OldNum,
			MaxModuleInstSeq: maxModuleSeq,
			MaxHostInstSeq:   maxHostSeq,
			Alias:            params.Alias,
			UsedHostInstSeqs: usedHostInstSeqs,
		})
	}

	return res, nil
}

// BuildProcessChangesResult 包含 BuildProcessChanges 的返回结果
type BuildProcessChangesResult struct {
	ToAddProcess        *table.Process
	ToUpdateProcess     *table.Process
	ToDeleteProcess     *table.Process
	ToAddInstances      []*table.ProcessInstance
	ToDeleteInstanceIDs []uint32
	// 需要重新编排的模块 (moduleID, alias)
	ModuleToReorder *ModuleAliasKey
}

// BuildProcessChanges 根据 CMDB 新旧进程数据，计算进程及其实例的变更结果：
//   - 是否需要新增进程（ToAddProcess）
//   - 是否需要更新进程（ToUpdateProcess）
//   - 是否需要删除旧进程（ToDeleteProcess，仅别名变更重建场景）
//   - 是否需要新增/删除进程实例（扩缩容），以及是否需要重排 ModuleInstSeq
//
// 变更检测维度（任一变化即进入更新流程，全部未变则返回空结果）：
// 别名 / source_data 配置内容 / proc_num / agent_id / agent 状态 / os_type /
// 集群名、模块名、服务实例名与集群环境类型（拓扑）/ 启动优先级。
//
// 注意：本函数会**原地修改** oldP（把 CMDB 新值写入旧记录再落库），并可能在
// 别名变更时产出一对「新增新别名进程 + 删除旧别名进程」的重建结果；
// 重建是否安全由 isSafeToUpdateProcess 判定（存量实例全部停止且未托管才允许原地改别名）。
//
// nolint: funlen,gocyclo
func BuildProcessChanges(ctx *SyncContext, params *BuildProcessChangesParams) (*BuildProcessChangesResult, error) {
	newP := params.NewProcess
	oldP := params.OldProcess
	result := &BuildProcessChangesResult{}

	equal, err := compareProcessInfo(newP.Spec.SourceData, oldP.Spec.SourceData)
	if err != nil {
		return nil, err
	}

	agentID := newP.Attachment.AgentID
	agentIDChanged := agentID != oldP.Attachment.AgentID
	newP.Spec.AgentStatus = resolveAgentStatus(agentID, newP.Spec.AgentStatus)

	nameChanged := newP.Spec.Alias != oldP.Spec.Alias
	infoChanged := !equal
	numChanged := newP.Spec.ProcNum != oldP.Spec.ProcNum
	agentStatusChanged := newP.Spec.AgentStatus != "" && newP.Spec.AgentStatus != oldP.Spec.AgentStatus
	osTypeChanged := newP.Spec.OsType != "" && newP.Spec.OsType != oldP.Spec.OsType
	// 启动优先级只影响操作编排顺序，不属于进程启动配置，因此单独 diff 而不并入 source_data 的一致性对比
	priorityChanged := newP.Spec.Priority != oldP.Spec.Priority
	// 拓扑字段同步集群名称、模块名称、服务实例名称与集群环境类型：watch 链已不再订阅 set 资源
	// （resources() 固定为 module/process），集群名变更只能由全量同步的 diff 覆盖；
	// 模块名虽由 module 事件单独更新（见 handleModuleEvent），但事件处理失败时游标仍会推进，
	// 漏掉的一次改名不会自行修复，因此同样纳入全量同步 diff 兜底；代价是一次改名会让该模块下
	// 所有进程产出更新并各派发一次状态同步任务，改名频率极低，可接受
	topoChanged := newP.Spec.SetName != oldP.Spec.SetName ||
		newP.Spec.ModuleName != oldP.Spec.ModuleName ||
		newP.Spec.ServiceName != oldP.Spec.ServiceName ||
		newP.Spec.Environment != oldP.Spec.Environment

	if !nameChanged && !infoChanged && !numChanged && !osTypeChanged &&
		!agentStatusChanged && !agentIDChanged && !topoChanged && !priorityChanged {
		return result, nil
	}

	if agentIDChanged {
		oldP.Attachment.AgentID = agentID
	}

	if osTypeChanged {
		oldP.Spec.OsType = newP.Spec.OsType
	}

	// 直接以 CMDB 值覆盖（含覆盖为空）
	if topoChanged {
		oldP.Spec.SetName = newP.Spec.SetName
		oldP.Spec.ModuleName = newP.Spec.ModuleName
		oldP.Spec.ServiceName = newP.Spec.ServiceName
		oldP.Spec.Environment = newP.Spec.Environment
	}

	if agentStatusChanged {
		oldP.Spec.AgentStatus = newP.Spec.AgentStatus
	}

	if priorityChanged {
		oldP.Spec.Priority = newP.Spec.Priority
	}

	// 是否安全
	safe, err := isSafeToUpdateProcess(ctx, oldP.Attachment.BizID, oldP.ID)
	if err != nil {
		return nil, err
	}

	newProcNum := int(newP.Spec.ProcNum)

	// 1. 别名变更：检查是否有同别名的 deleted 记录可以复用
	if nameChanged {
		// 查找同 CcProcessID + 同新别名
		reusableProc, err := ctx.Dao.Process().GetByCcProcessIDAndAliasTx(
			ctx.Kit, ctx.Tx, oldP.Attachment.BizID, oldP.Attachment.CcProcessID, newP.Spec.Alias)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		// 待删除的数据需要记录新进程别名
		oldP.Spec.NewAlias = newP.Spec.Alias

		// 恢复 deleted 记录为 synced，并更新其元数据
		if reusableProc != nil {
			reusableProc.Spec.NewAlias = "" // 清空 NewAlias 字段
			reusableProc.Spec.PrevData = oldP.Spec.SourceData
			reusableProc.Spec.SourceData = newP.Spec.SourceData
			reusableProc.Spec.CcSyncStatus = table.Synced
			reusableProc.Spec.ProcNum = newP.Spec.ProcNum
			// 新值非空时采用 CMDB 最新 os_type，否则沿用旧进程值，
			// 避免恢复 deleted 记录时保留其陈旧/空的 os_type
			reusableProc.Spec.OsType = resolveOsType(newP.Spec.OsType, oldP.Spec.OsType)
			// 刷新 deleted 记录上的陈旧状态，空值不覆盖
			if newP.Spec.AgentStatus != "" {
				reusableProc.Spec.AgentStatus = newP.Spec.AgentStatus
			}
			// 恢复 deleted 记录时同步刷新拓扑字段，避免残留旧值
			reusableProc.Spec.SetName = newP.Spec.SetName
			reusableProc.Spec.ModuleName = newP.Spec.ModuleName
			reusableProc.Spec.ServiceName = newP.Spec.ServiceName
			reusableProc.Spec.Environment = newP.Spec.Environment
			reusableProc.Spec.Priority = newP.Spec.Priority
			reusableProc.Attachment = newP.Attachment
			reusableProc.Revision = &table.Revision{UpdatedAt: ctx.Now}

			// 查询恢复进程的现有实例
			existingInsts, err := processInstances(ctx, reusableProc.Attachment.BizID, reusableProc.ID)
			if err != nil {
				return nil, err
			}

			// 根据 ProcNum 扩缩容实例
			res, err := reconcileProcessInstances(ctx, &ReconcileInstancesParams{
				BizID:       reusableProc.Attachment.BizID,
				ProcessID:   reusableProc.ID,
				HostID:      reusableProc.Attachment.HostID,
				ModuleID:    reusableProc.Attachment.ModuleID,
				CcProcessID: reusableProc.Attachment.CcProcessID,
				Alias:       newP.Spec.Alias,
				OldNum:      len(existingInsts),
				NewNum:      newProcNum,
			})
			if err != nil {
				return nil, err
			}

			// 返回：恢复的进程作为更新，旧进程标记为删除
			result.ToUpdateProcess = reusableProc
			result.ToDeleteProcess = oldP
			result.ToAddInstances = res.ToAdd
			result.ToDeleteInstanceIDs = res.ToDelete
			// 只有实际发生扩缩容时才需要重新排序
			if len(res.ToAdd) > 0 || len(res.ToDelete) > 0 {
				result.ModuleToReorder = &ModuleAliasKey{
					ModuleID: int(reusableProc.Attachment.ModuleID),
					Alias:    newP.Spec.Alias,
				}
			}
			return result, nil
		}

		// 没有可复用的 deleted 记录且不安全：创建新进程，标记旧进程为删除
		if !safe {
			newP.Spec.PrevData = oldP.Spec.SourceData
			newP.Spec.CcSyncStatus = table.Synced
			// 新值为空时沿用旧进程的 os_type，避免重建时把已有类型覆盖为空
			newP.Spec.OsType = resolveOsType(newP.Spec.OsType, oldP.Spec.OsType)

			toAdd := &table.Process{
				Attachment: newP.Attachment,
				Spec:       newP.Spec,
				Revision:   &table.Revision{CreatedAt: ctx.Now},
			}

			// 查询同模块同别名的最大 ModuleInstSeq
			maxModuleInstSeq := 0
			sameAliasProcessIDs, err := ctx.Dao.Process().ListByModuleIDAndAliasWithTx(
				ctx.Kit, ctx.Tx, toAdd.Attachment.BizID, toAdd.Attachment.ModuleID, newP.Spec.Alias)
			if err != nil {
				return nil, err
			}
			if len(sameAliasProcessIDs) > 0 {
				maxModuleInstSeq, err = ctx.Dao.ProcessInstance().GetMaxModuleInstSeqByProcessIDsWithTx(
					ctx.Kit, ctx.Tx, toAdd.Attachment.BizID, sameAliasProcessIDs)
				if err != nil {
					return nil, err
				}
			}

			insts := buildInstances(ctx, &BuildInstancesParams{
				BizID:            toAdd.Attachment.BizID,
				HostID:           toAdd.Attachment.HostID,
				ModuleID:         toAdd.Attachment.ModuleID,
				CcProcessID:      toAdd.Attachment.CcProcessID,
				ProcNum:          newProcNum,
				ExistCount:       0,
				MaxModuleInstSeq: maxModuleInstSeq,
				MaxHostInstSeq:   0,
				Alias:            newP.Spec.Alias,
			})

			result.ToAddProcess = toAdd
			result.ToDeleteProcess = oldP
			result.ToAddInstances = insts
			return result, nil
		}

		// 安全且没有可复用记录：原地更新别名
		oldP.Spec.Alias = newP.Spec.Alias
	}
	if infoChanged {
		oldP.Spec.PrevData = oldP.Spec.SourceData
		oldP.Spec.SourceData = newP.Spec.SourceData
		// 进程变更 + 安全 就是已同步的状态
		if safe {
			oldP.Spec.CcSyncStatus = table.Synced
		} else {
			oldP.Spec.CcSyncStatus = table.Updated
		}
	}

	// 实例调整逻辑
	if numChanged {
		// 更新进程的 ProcNum 字段
		oldP.Spec.ProcNum = newP.Spec.ProcNum

		// 真实实例数
		allInsts, err := processInstances(ctx, oldP.Attachment.BizID, oldP.ID)
		if err != nil {
			return nil, err
		}

		res, err := reconcileProcessInstances(ctx, &ReconcileInstancesParams{
			BizID:       oldP.Attachment.BizID,
			ProcessID:   oldP.ID,
			HostID:      oldP.Attachment.HostID,
			ModuleID:    oldP.Attachment.ModuleID,
			CcProcessID: oldP.Attachment.CcProcessID,
			Alias:       oldP.Spec.Alias,
			OldNum:      len(allInsts),
			NewNum:      newProcNum,
		})
		if err != nil {
			return nil, err
		}

		result.ToAddInstances = res.ToAdd
		result.ToDeleteInstanceIDs = res.ToDelete

		// 只有实际发生扩缩容时才需要重新排序
		if len(result.ToAddInstances) > 0 || len(result.ToDeleteInstanceIDs) > 0 {
			result.ModuleToReorder = &ModuleAliasKey{
				ModuleID: int(oldP.Attachment.ModuleID),
				Alias:    oldP.Spec.Alias,
			}
		}
	}

	toUpdate := &table.Process{
		ID:         oldP.ID,
		Attachment: oldP.Attachment,
		Spec:       oldP.Spec,
		Revision:   &table.Revision{UpdatedAt: ctx.Now},
	}

	result.ToUpdateProcess = toUpdate
	return result, nil
}

// buildInstances 根据进程数量生成进程实例
// moduleCounter 的 key 为 ModuleAliasKey，使同模块同别名的进程共享 ModuleInstSeq 递增序列
func buildInstances(ctx *SyncContext, params *BuildInstancesParams) []*table.ProcessInstance {
	// 如果新的进程数量 <= 已存在数量，则无需新增实例
	if params.ProcNum <= params.ExistCount {
		return nil
	}

	// 需要新增的实例数量
	newCount := params.ProcNum - params.ExistCount
	if newCount <= 0 {
		return nil
	}

	instances := make([]*table.ProcessInstance, 0, newCount)

	// 维度 key：(moduleID, alias) 用于 ModuleInstSeq，使同模块同别名共享序列
	modKey := ModuleAliasKey{ModuleID: int(params.ModuleID), Alias: params.Alias}

	startModuleInstSeq := ctx.ModuleCounter[modKey]
	// 如果缓存未初始化，则从数据库最大值开始
	if startModuleInstSeq == 0 {
		startModuleInstSeq = params.MaxModuleInstSeq
		ctx.ModuleCounter[modKey] = startModuleInstSeq
	}

	// HostInstSeq：计算已用序号的空洞，优先填洞
	hostSeqsToAssign := calcHostInstSeqs(params.UsedHostInstSeqs, params.MaxHostInstSeq, newCount)

	instTenantID := ctx.Kit.TenantID
	if instTenantID == "" {
		instTenantID = "default"
	}

	for i := range newCount {
		ctx.ModuleCounter[modKey]++

		instances = append(instances, &table.ProcessInstance{
			Attachment: &table.ProcessInstanceAttachment{
				TenantID:    instTenantID,
				BizID:       params.BizID,
				CcProcessID: params.CcProcessID,
			},
			Spec: &table.ProcessInstanceSpec{
				StatusUpdatedAt: ctx.Now,
				HostInstSeq:     uint32(hostSeqsToAssign[i]), // 填洞后的连续序号
				ModuleInstSeq:   uint32(ctx.ModuleCounter[modKey]),
			},
			Revision: &table.Revision{
				CreatedAt: ctx.Now,
				UpdatedAt: ctx.Now,
			},
		})
	}

	return instances
}

// calcHostInstSeqs 基于已用序号，优先填洞，不够再续接最大值
//
// 示例：
//
//	usedSeqs = [1, 4, 5]，maxUsed=5，needAdd=3
//	gaps     = [2, 3]
//	续接      = [6]
//	返回      = [2, 3, 6]
func calcHostInstSeqs(usedSeqs []int, maxHostInstSeq, needAdd int) []int {
	// 构建已用集合，找最大值
	usedSet := make(map[int]struct{}, len(usedSeqs))
	maxUsed := maxHostInstSeq
	for _, seq := range usedSeqs {
		usedSet[seq] = struct{}{}
		if seq > maxUsed {
			maxUsed = seq
		}
	}

	result := make([]int, 0, needAdd)

	// 1. 优先填空洞（1 ~ maxUsed 范围内未占用的序号）
	for i := 1; i <= maxUsed && len(result) < needAdd; i++ {
		if _, used := usedSet[i]; !used {
			result = append(result, i)
		}
	}

	// 2. 空洞不够，续接最大值
	next := maxUsed
	for len(result) < needAdd {
		next++
		result = append(result, next)
	}

	return result
}

// ProcessDiff 进程差异对比
type ProcessDiff struct {
	// 进程级
	ToAddProcesses    []*table.Process
	ToUpdateProcesses []*table.Process
	ToDeleteProcesses []*table.Process

	// 实例级
	ToAddInstances      []*table.ProcessInstance
	ToDeleteInstanceIDs []uint32

	// 需要重新编排 ModuleInstSeq 的模块 (moduleID, alias) 集合
	ModulesToReorder map[ModuleAliasKey]struct{}
}

// SyncProcessData 对比并同步进程及进程实例数据
//
// 同步策略：
//  1. 基于 oldProcesses / newProcesses 做差异计算（diffProcesses 内部会识别软删除记录，
//     CMDB 进程命中软删除记录时走原地恢复而不是新增）
//  2. 先删除实例（缩容）
//  3. 再删除进程（包含兜底删除 stopped / unmanaged 实例）
//  4. 新增进程
//  5. 更新进程（含原地恢复的软删除记录）
//  6. 回填 ProcessID 到新增实例
//  7. 新增实例（扩容 / 新进程）
//  8. 重排受影响模块的 ModuleInstSeq
//
// 入参 newProcesses 必须是业务在 CMDB 的全量进程列表，空列表表示「该业务已无进程」：
// 此时 diff 会把库内全部存活行按删除语义收敛（软删 + 清理 stopped/unmanaged 实例），
// 不能把空列表当成「本轮无数据」提前返回，否则最后一个进程被删除后本地行会永久残留。
// 允许调用方用空列表触发清理的前提是「能确认 CMDB 侧确实为空」，由调用方保证
// （CMDB 接口失败、分页无法确认取满时必须返回错误，不得以空列表交给本函数）。
//
// 返回值：
//   - SyncProcessResult：本次新增/更新的进程及其新增实例
//   - error：任一阶段失败则直接返回
//
// nolint:funlen,gocyclo
func SyncProcessData(kit *kit.Kit, dao dao.Set, tx *gen.QueryTx, bizID uint32, oldProcesses,
	newProcesses []*table.Process) (*SyncProcessResult, error) {

	// 创建同步上下文
	ctx := &SyncContext{
		Kit:           kit,
		Dao:           dao,
		Tx:            tx,
		Now:           time.Now().UTC(),
		ModuleCounter: make(map[ModuleAliasKey]int),
	}

	// 预取旧进程的实例快照：diff 阶段需要按进程判定「是否可安全变更」和实例数量，
	// 逐进程查库在万级进程业务上会产生上万次查询，显著拉长事务
	if err := prefetchProcessInstances(ctx, bizID, oldProcesses); err != nil {
		logs.Errorf("[ProcessSync][PrefetchInstances] bizID=%d failed: %v", bizID, err)
		return nil, err
	}

	diff, err := diffProcesses(ctx, oldProcesses, newProcesses)
	if err != nil {
		logs.Errorf(
			"[ProcessSync][DiffProcesses] bizID=%d failed: %v",
			bizID, err,
		)
		return nil, err
	}

	// 无任何变更时直接返回，跳过后续写库（事件驱动的全量同步绝大多数是无变更场景）
	result := &SyncProcessResult{
		Changed: len(diff.ToAddProcesses) > 0 || len(diff.ToUpdateProcesses) > 0 ||
			len(diff.ToDeleteProcesses) > 0 || len(diff.ToAddInstances) > 0 ||
			len(diff.ToDeleteInstanceIDs) > 0 || len(diff.ModulesToReorder) > 0,
	}
	if !result.Changed {
		return result, nil
	}

	// 1. 先删实例（缩容）
	if len(diff.ToDeleteInstanceIDs) > 0 {
		if err := dao.ProcessInstance().BatchDeleteByIDsWithTx(kit, tx, diff.ToDeleteInstanceIDs); err != nil {
			logs.Errorf(
				"[ProcessSync][DeleteInstance] bizID=%d instanceIDs=%v failed: %v",
				bizID, diff.ToDeleteInstanceIDs, err,
			)
			return nil, err
		}
	}

	// 2. 删除进程（并兜底清理 stopped / unmanaged 实例）
	if len(diff.ToDeleteProcesses) > 0 {
		if err := replaceProcessAndCleanInstancesTx(kit, dao, tx, bizID, diff.ToDeleteProcesses); err != nil {
			return nil, err
		}
	}

	// 3. 新增进程
	if len(diff.ToAddProcesses) > 0 {
		if err := dao.Process().BatchCreateWithTx(kit, tx, diff.ToAddProcesses); err != nil {
			logs.Errorf(
				"[ProcessSync][CreateProcess] bizID=%d processCount=%d failed: %v",
				bizID, len(diff.ToAddProcesses), err,
			)
			return nil, err
		}
	}

	// 4. 更新进程
	if len(diff.ToUpdateProcesses) > 0 {
		if err := dao.Process().BatchUpdateWithTx(kit, tx, diff.ToUpdateProcesses); err != nil {
			logs.Errorf(
				"[ProcessSync][UpdateProcess] bizID=%d processCount=%d failed: %v",
				bizID, len(diff.ToUpdateProcesses), err,
			)
			return nil, err
		}
	}

	// 5. 构建 (tenantID + bizID + ccProcessID) -> ProcessID 映射
	// 用于给新增实例回填 ProcessID
	processIDByKey := make(map[string]uint32)
	buildKey := func(tenantID string, bizID uint32, ccProcessID uint32) string {
		return fmt.Sprintf("%s-%d-%d", tenantID, bizID, ccProcessID)
	}
	// 实例表中按 cc_process_id 关联进程，同 key 出现多条进程（不同别名的重建残留）时
	// 无法确定实例归属，直接失败回滚，避免把实例挂到错误的进程上
	addProcessID := func(p *table.Process) error {
		key := buildKey(p.Attachment.TenantID, bizID, p.Attachment.CcProcessID)
		if existID, ok := processIDByKey[key]; ok && existID != p.ID {
			return fmt.Errorf(
				"[ProcessSync][FillProcessID] bizID=%d ccProcessID=%d matched multiple processes: %d, %d",
				bizID, p.Attachment.CcProcessID, existID, p.ID)
		}
		processIDByKey[key] = p.ID
		return nil
	}
	for _, p := range diff.ToAddProcesses {
		if err := addProcessID(p); err != nil {
			return nil, err
		}
	}

	for _, p := range diff.ToUpdateProcesses {
		if err := addProcessID(p); err != nil {
			return nil, err
		}
	}

	// 6. 回填 ProcessID 到新增实例
	for _, inst := range diff.ToAddInstances {
		key := buildKey(inst.Attachment.TenantID, bizID, inst.Attachment.CcProcessID)
		if pid := processIDByKey[key]; pid != 0 {
			inst.Attachment.ProcessID = pid
		} else {
			logs.Warnf(
				"[ProcessSync][FillProcessID] bizID=%d ccProcessID=%d instanceID=%d process not found",
				bizID,
				inst.Attachment.CcProcessID,
				inst.ID,
			)
		}
	}

	// 7. 新增实例（扩容 / 新进程）
	if len(diff.ToAddInstances) > 0 {
		if err := dao.ProcessInstance().
			BatchCreateWithTx(kit, tx, diff.ToAddInstances); err != nil {
			logs.Errorf(
				"[ProcessSync][CreateInstance] bizID=%d instanceCount=%d failed: %v",
				bizID, len(diff.ToAddInstances), err,
			)
			return nil, err
		}
	}

	// 8. 重新编排受影响模块的 ModuleInstSeq（在所有实例入库后统一执行）
	for modKey := range diff.ModulesToReorder {
		toUpdate, err := calcModuleInstSeqUpdatesTx(
			ctx.Kit,
			ctx.Dao,
			ctx.Tx,
			&ReorderParams{
				BizID:    bizID,
				ModuleID: uint32(modKey.ModuleID),
				Alias:    modKey.Alias,
			},
		)
		if err != nil {
			logs.Errorf(
				"[ProcessSync][ReorderModuleInstSeq] bizID=%d moduleID=%d alias=%s failed: %v",
				bizID, modKey.ModuleID, modKey.Alias, err,
			)
			return nil, err
		}

		if len(toUpdate) > 0 {
			if err := dao.ProcessInstance().BatchUpdateWithTx(kit, tx, toUpdate); err != nil {
				logs.Errorf(
					"[ProcessSync][UpdateModuleInstSeq] bizID=%d instanceCount=%d failed: %v",
					bizID, len(toUpdate), err,
				)
				return nil, err
			}
		}
	}

	// 构建 ProcessID -> 新增实例 列表映射
	newInstancesByProcessID := make(map[uint32][]*table.ProcessInstance)
	for _, inst := range diff.ToAddInstances {
		if inst.Attachment.ProcessID != 0 {
			newInstancesByProcessID[inst.Attachment.ProcessID] =
				append(newInstancesByProcessID[inst.Attachment.ProcessID], inst)
		}
	}

	// 新增进程
	for _, p := range diff.ToAddProcesses {
		result.Items = append(result.Items, &ProcessWithInstances{
			Process:   p,
			Instances: newInstancesByProcessID[p.ID],
		})
	}

	// 更新进程
	for _, p := range diff.ToUpdateProcesses {
		result.Items = append(result.Items, &ProcessWithInstances{
			Process:   p,
			Instances: newInstancesByProcessID[p.ID],
		})
	}

	return result, nil
}

// replaceProcessAndCleanInstancesTx 将一批进程标记为软删除（cc_sync_status=deleted），
// 并把别名更新为删除时记录的 new_alias（别名重建场景用于后续按新别名恢复）。
// 同时清理这些进程下 stopped/unmanaged 状态的实例；运行中的实例保留，
// 待进程后续被恢复（restoreDeletedProcess）时按 ProcNum 重新对齐。
func replaceProcessAndCleanInstancesTx(kit *kit.Kit, dao dao.Set, tx *gen.QueryTx, bizID uint32,
	process []*table.Process) error {
	// 1. 将 process 表中的数据标记为 deleted 状态 且更新别名为新别名
	err := dao.Process().UpdateSyncStatusAndAliasTx(kit, tx, process)
	if err != nil {
		return err
	}

	// 2. 删除process_instances表中的数据
	var processIDs []uint32
	for _, p := range process {
		processIDs = append(processIDs, p.ID)
	}
	err = dao.ProcessInstance().DeleteStoppedUnmanagedWithTx(kit, tx, bizID, processIDs)
	if err != nil {
		return err
	}

	return nil
}

// MarkProcessDeletedAndCleanInstancesTx 标记进程为 deleted 状态并清理相关实例
func MarkProcessDeletedAndCleanInstancesTx(kit *kit.Kit, dao dao.Set, tx *gen.QueryTx, bizID uint32, processIDs []uint32) error {
	// 1. 删除processes表中的数据

	err := dao.Process().UpdateSyncStatusWithTx(kit, tx, table.Deleted.String(), processIDs)
	if err != nil {
		return err
	}

	err = dao.ProcessInstance().DeleteStoppedUnmanagedWithTx(kit, tx, bizID, processIDs)
	if err != nil {
		return err
	}

	return nil
}

// processInstancePrefetchBatchLimit 预取实例时单次 IN 查询的进程数量上限
const processInstancePrefetchBatchLimit = 500

// prefetchProcessInstances 一次性预取旧进程的实例快照，供 diff 阶段内存判定。
// 逐进程查库在万级进程业务上会产生上万次事务内查询，显著拉长事务与锁持有时间。
func prefetchProcessInstances(ctx *SyncContext, bizID uint32, processes []*table.Process) error {
	processIDs := make([]uint32, 0, len(processes))
	for _, p := range processes {
		if p == nil || p.Attachment == nil || p.ID == 0 {
			continue
		}
		processIDs = append(processIDs, p.ID)
	}
	if len(processIDs) == 0 {
		ctx.InstancesByProcessID = make(map[uint32][]*table.ProcessInstance)
		return nil
	}

	grouped := make(map[uint32][]*table.ProcessInstance, len(processIDs))
	for start := 0; start < len(processIDs); start += processInstancePrefetchBatchLimit {
		end := min(start+processInstancePrefetchBatchLimit, len(processIDs))
		insts, err := ctx.Dao.ProcessInstance().ListByProcessIDsWithTx(ctx.Kit, ctx.Tx, bizID, processIDs[start:end])
		if err != nil {
			return err
		}
		for _, inst := range insts {
			if inst == nil || inst.Attachment == nil {
				continue
			}
			grouped[inst.Attachment.ProcessID] = append(grouped[inst.Attachment.ProcessID], inst)
		}
	}

	ctx.InstancesByProcessID = grouped
	return nil
}

// processInstances 返回某进程的实例列表：优先使用预取快照（未预取时为 nil map 则回退查库）。
func processInstances(ctx *SyncContext, bizID, processID uint32) ([]*table.ProcessInstance, error) {
	if ctx.InstancesByProcessID != nil {
		return ctx.InstancesByProcessID[processID], nil
	}
	return ctx.Dao.ProcessInstance().ListByProcessIDTx(ctx.Kit, ctx.Tx, bizID, processID)
}

// isSafeToUpdateProcess 判断进程是否可以安全地原地变更（别名重建/原地更新）：
// 仅当其全部实例都处于「已停止或无状态」且「未托管或无托管状态」时视为安全；
// 存在运行中或已托管实例时返回 false，调用方应走「新增新记录 + 标记旧记录删除」的重建路径，
// 避免影响正在运行的实例。
func isSafeToUpdateProcess(ctx *SyncContext, bizID, processID uint32) (bool, error) {
	insts, err := processInstances(ctx, bizID, processID)
	if err != nil {
		return false, err
	}

	for _, inst := range insts {
		if inst == nil || inst.Spec == nil {
			continue
		}
		// 进程不是停止和空都是不安全
		if inst.Spec.Status != table.ProcessStatusStopped && inst.Spec.Status != "" {
			return false, nil
		}
		// 托管不是未托管和空都是不安全
		if inst.Spec.ManagedStatus != table.ProcessManagedStatusUnmanaged &&
			inst.Spec.ManagedStatus != "" {
			return false, nil
		}
	}

	return true, nil
}

// CheckAndMarkHostAliasConflicts 检测同主机同别名冲突并标记状态
//
// 逻辑：
//  1. 查询同 biz 下 (host_id, alias) 重复的进程 ID 列表
//  2. 将冲突进程标记为 abnormal
//  3. 将之前标记为 abnormal 但已不再冲突的进程恢复为 synced
func CheckAndMarkHostAliasConflicts(kit *kit.Kit, dao dao.Set, tx *gen.QueryTx, bizID uint32) error {
	conflictIDs, err := dao.Process().ListDuplicateHostAliasWithTx(kit, tx, bizID)
	if err != nil {
		return fmt.Errorf("[ConflictCheck] list duplicate host alias failed, bizID=%d: %w", bizID, err)
	}

	if len(conflictIDs) > 0 {
		if err := dao.Process().UpdateSyncStatusWithTx(kit, tx, table.Abnormal.String(), conflictIDs); err != nil {
			return fmt.Errorf("[ConflictCheck] mark abnormal failed, bizID=%d, ids=%v: %w", bizID, conflictIDs, err)
		}
		logs.Warnf("[ConflictCheck] bizID=%d marked %d processes as abnormal (duplicate host+alias)", bizID, len(conflictIDs))
	}

	if err := dao.Process().RestoreAbnormalWithTx(kit, tx, bizID, conflictIDs); err != nil {
		return fmt.Errorf("[ConflictCheck] restore abnormal failed, bizID=%d: %w", bizID, err)
	}

	return nil
}

// ComputeModuleInstSeqUpdates 重新排序模块实例序列，确保同一模块下相同别名的进程实例的 ModuleInstSeq 是连续的
// 适用场景：当某个模块下的进程实例发生变更（删除）时，调用此函数重新计算并更新 ModuleInstSeq 字段，保持序列连续且有序
func ComputeModuleInstSeqUpdates(kit *kit.Kit, dao dao.Set, bizID, processID uint32) error {
	tx := dao.GenQuery().Begin()

	committed := false
	defer func() {
		if !committed {
			if errR := tx.Rollback(); errR != nil {
				logs.Errorf(
					"[ComputeModuleInstSeqUpdates ERROR] biz %d: rollback failed, err=%v",
					bizID, errR,
				)
			}
		}
	}()

	process, err := dao.Process().GetByIDWithTx(kit, tx, bizID, processID)
	if err != nil {
		return err
	}

	updates, err := calcModuleInstSeqUpdatesTx(
		kit,
		dao,
		tx,
		&ReorderParams{
			BizID:    bizID,
			ModuleID: process.Attachment.ModuleID,
			Alias:    process.Spec.Alias,
		},
	)
	if err != nil {
		return err
	}

	if len(updates) > 0 {
		err = dao.ProcessInstance().BatchUpdateWithTx(kit, tx, updates)
		if err != nil {
			return fmt.Errorf("failed to update process instances: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		logs.Errorf("[ComputeModuleInstSeqUpdates ERROR] biz %d: commit failed, err=%v", bizID, err)
		return err
	}

	committed = true

	return nil
}

// collectSeqUpdates 收集需要更新 ModuleInstSeq 的实例，确保同一模块下相同别名的进程实例的 ModuleInstSeq 是连续的
func collectSeqUpdates(instances []*table.ProcessInstance) []*table.ProcessInstance {
	updates := make([]*table.ProcessInstance, 0)
	for i, inst := range instances {
		if expectedSeq := uint32(i + 1); inst.Spec.ModuleInstSeq != expectedSeq {
			inst.Spec.ModuleInstSeq = expectedSeq
			updates = append(updates, inst)
		}
	}
	return updates
}

func calcModuleInstSeqUpdatesTx(kit *kit.Kit, dao dao.Set, tx *gen.QueryTx, params *ReorderParams) ([]*table.ProcessInstance, error) {

	// 1 查询 processes
	processIDs, err := dao.Process().ListByModuleIDAndAliasWithTx(
		kit, tx, params.BizID, params.ModuleID, params.Alias,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list processes by module ID and alias: %w", err)
	}

	if len(processIDs) == 0 {
		return nil, nil
	}

	// 2 查询 instances
	instances, err := dao.ProcessInstance().ListByProcessIDsWithTx(
		kit, tx, params.BizID, processIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list process instances: %w", err)
	}

	if len(instances) == 0 {
		return nil, nil
	}

	// 3 排序
	sort.Slice(instances, func(i, j int) bool {
		if instances[i].Attachment.CcProcessID != instances[j].Attachment.CcProcessID {
			return instances[i].Attachment.CcProcessID < instances[j].Attachment.CcProcessID
		}
		return instances[i].Spec.HostInstSeq < instances[j].Spec.HostInstSeq
	})

	return collectSeqUpdates(instances), nil
}

// fetchAllProcessRelatedInfo 分页拉取业务全量进程关联信息
func (s *syncCMDBService) fetchAllProcessRelatedInfo(ctx context.Context) ([]*bkcmdb.ProcessRelatedInfoItem, error) {
	return PageFetcher(func(page *bkcmdb.PageParam) ([]*bkcmdb.ProcessRelatedInfoItem, int, error) {
		req := &bkcmdb.ListProcessRelatedInfoReq{
			BkBizID: s.bizID,
			Page:    page,
		}
		resp, err := s.svc.ListProcessRelatedInfo(ctx, req)
		if err != nil {
			return nil, 0, err
		}

		return resp.Info, resp.Count, nil
	})
}
