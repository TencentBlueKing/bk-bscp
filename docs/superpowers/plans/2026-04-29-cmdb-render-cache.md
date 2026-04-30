# CMDB 渲染缓存 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** 降低配置渲染瞬间对 CMDB 的访问峰值，只缓存渲染聚合结果，并在 CMDB 同步成功后主动失效。

**Architecture:** `data-service` 使用已有 Redis 配置创建共享的 `RenderCache`，缓存 `topo_xml` 和 `biz_global_variables` 两类渲染聚合数据。渲染路径读取缓存，未命中时回源 CMDB 并写入；`SyncCMDB` 步骤成功后按 `tenantID + bizID` 删除缓存；watch 事件不删除缓存，避免高频事件打穿。

**Tech Stack:** Go、Redis、`internal/dal/bedis`、`render`、task executor、`go test`。

---

### Task 1: 配置与缓存接口

**Files:**
- Modify: `pkg/cc/types.go`
- Modify: `internal/processor/cmdb/render_cache.go`
- Test: `internal/processor/cmdb/render_cache_test.go`

- [x] 新增 `cmdb.renderCache.topoXmlTTL`、`cmdb.renderCache.bizGlobalVariablesTTL`、`cmdb.renderCache.buildLockTTL` 配置，默认分别为 `1h`、`5m`、`30s`；`topoXmlTTL` 和 `bizGlobalVariablesTTL` 与旧项目缓存时间保持一致。
- [x] `RenderCache` 增加按业务删除方法，Redis 实现删除 `topo_xml` 和 `biz_global_variables`。
- [x] 测试 Redis key 按 `tenantID + bizID` 隔离，TTL 使用配置值，删除只影响指定业务。

### Task 2: 渲染链路接入聚合缓存

**Files:**
- Modify: `render/context.go`
- Modify: `cmd/data-service/service/service.go`
- Modify: `cmd/data-service/service/config_instance.go`
- Modify: `cmd/data-service/service/config_template.go`
- Modify: `internal/task/executor/config/config_generate.go`
- Modify: `internal/task/register/register.go`
- Modify: `cmd/data-service/app/app.go`
- Test: `render/context_test.go` 或现有 CMDB processor 测试

- [x] `BuildProcessContextParamsFromSource` 支持传入 `RenderCache`，创建 `CCTopoXMLService` 时带上缓存。
- [x] `data-service` 复用已有 Redis client 创建 `RenderCache`，传给 service 和 config generate executor。
- [x] 不缓存 `ListBizHosts`、`FindHostBizRelations` 等底层接口，缓存边界只在 `GetTopoTreeXML` 与 `GetBizObjectAttributes`。

### Task 3: 同步成功后主动失效

**Files:**
- Modify: `internal/task/executor/cmdb_gse/cmdb_sync_gse.go`
- Modify: `internal/task/register/register.go`
- Test: `internal/task/executor/cmdb_gse/cmdb_sync_gse_test.go`

- [x] `SyncCMDB` 成功后调用 `InvalidateBiz(ctx, tenantID, bizID)`。
- [x] `InvalidateBiz` 同步删除该业务的构建锁索引和 lock key，避免同步后残留 lock 触发无意义等待。
- [x] 失效失败只记录 warning，不影响同步任务成功结果。
- [x] watch set/module/host/host_relation 不接入失效逻辑，依赖手动/定时同步主动失效和 TTL 兜底。

### Task 4: 验证

**Files:**
- Test packages touched above

- [x] 运行 `gofmt`。
- [x] 运行目标包测试和编译检查：`go test ./internal/processor/cmdb ./internal/task/executor/cmdb_gse -count=1`、`go test ./render -run TestBuildProcessContextParamsFromSourceUsesRenderCache -count=1`、`go test ./internal/task/executor/config ./cmd/data-service/service ./internal/task/register ./cmd/data-service/app -run TestNonExistent -count=1 -vet=off`。
- [x] 检查 `git diff --check`。
