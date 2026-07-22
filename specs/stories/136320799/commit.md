# Commit 记录

## Commit Message

（本次代码由用户手动提交，原始信息如下）

```
支持集群环境类型、服务实例名更新
```

> 建议（Conventional Commits 规范，供后续参考）：
> `fix(cmdb): 一键同步增量更新集群环境类型与服务实例名称`
> body：修复 BuildProcessChanges 未检测/写回 service_name、environment 导致一键同步不刷新的缺陷；
> 早退守卫纳入 topoChanged，覆盖写回（含空值），reusable 恢复分支同步回填；集群名称/模块名称本期不同步。
> footer：`--story=1020451610136320799`

## Commit Hash

- 本地/远端：`9194a67cc6b7ecd5abefc7339453151a863c9e40`
- 分支：`feat/update-set-serviceinstance`（已推送 origin）
- 基线：`788381c9a29883ed92b321b987abbf4cb3f2e619`

## 变更统计

| 指标 | 值 |
|------|-----|
| 总变更行数 | 1950 |
| 新增代码 | 1949 |
| 删除代码 | 1 |
| 逻辑代码 | 14（internal/processor/cmdb/sync_cmdb.go）|
| 测试代码 | 196（internal/processor/cmdb/sync_cmdb_topo_test.go）|
| 文档变更 | 1404（docs/ + specs/ 文档与流水线产物）|
| 变更文件数 | 22 |

## 核心代码改动

- `internal/processor/cmdb/sync_cmdb.go` `BuildProcessChanges`：
  - 新增 `topoChanged := newP.Spec.ServiceName != oldP.Spec.ServiceName || newP.Spec.Environment != oldP.Spec.Environment`
  - 纳入早退守卫（`&& !topoChanged`）
  - 早退守卫后直接覆盖写回 `ServiceName`/`Environment`（含空值，无空值保护，Q-002）
  - reusable 恢复分支显式回填两字段（TR-001）
  - **未**对 `set_name`/`module_name` 做 diff/写回（范围决策 2026-07-21）
- `internal/processor/cmdb/sync_cmdb_topo_test.go`：表驱动单测 T1~T6
- `docs/cmdb-sync-capability-gsekit-vs-bscp.md`：F-002 gsekit vs bscp CMDB 同步能力对比清单

## 验证

- `go test ./internal/processor/cmdb/`：全绿（含 TestBuildProcessChangesTopoFields T1~T5、TestBuildProcessChangesReusableRefreshesTopoFields T6）
- `gofmt -l`：无输出
- validate 三段（架构 / 安全 / CodeReview）：均 LGTM，无 CRITICAL/HIGH

## 时间

- 开始时间：2026-07-21T21:12:00+08:00
- 完成时间：2026-07-21T21:56:00+08:00
