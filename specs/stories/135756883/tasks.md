# 任务清单 — 进程 IP 查询 inner 接口（Story 135756883）

> TDD 顺序执行。每个任务标注对应验收标准（AC）与落点文件。

## Phase 0：proto 契约

- [ ] **T001** 在 `pkg/protocol/config-server/config_service.proto` 进程管理分组内（紧邻 `ListProcess`）新增 `ListProcessInnerIPs` RPC（`INTERNAL,BKAPIGW` + public/inner 双 binding）及 `ListProcessInnerIPsReq/Resp` 消息。（F-001/F-002/F-003）
- [ ] **T002** 在 `pkg/protocol/data-service/data_service.proto` 新增 `ListProcessInnerIPs` RPC 及同名消息。（F-001）
- [ ] **T003** 运行 `make pb` 重新生成 config-server / data-service 的 `*.pb.go`/`*_grpc.pb.go`/`*.pb.gw.go`；`go build ./...` 通过；`git diff` 核对为纯新增、未改既有接口。

## Phase 1-2：核心逻辑（TDD）

- [ ] **T004（红）** 新增 `cmd/data-service/service/process_test.go`：
  - `TestDedupInnerIPs`：多进程同 IP 去重保序、跳过空串、命中为空返回空列表（AC-002/AC-003/AC-004/AC-006）。
  - `TestValidateExpressionEnv`：表达式范围 + 空 environment → `InvalidParameter`；有 environment / 无 expression_scope → 通过（AC-005）。
  - 先写测试，运行 `go test` 确认红（未实现）。
- [ ] **T005（绿）** 在 `cmd/data-service/service/process.go` 实现 `dedupInnerIPs` 与 `validateExpressionEnv`，使 T004 转绿。
- [ ] **T006（绿）** 在 `cmd/data-service/service/process.go` 实现 `ListProcessInnerIPs` handler：环境校验 → `dao.Process().List(All=true)` → `dedupInnerIPs` → 返回 `{Ips}`（F-001/F-003/AC-007 复用既有 biz 约束）。

## Phase 3：config-server 接入

- [ ] **T007（绿）** 在 `cmd/config-server/service/process.go` 实现 `ListProcessInnerIPs` handler：IAM 鉴权（对齐 `ListProcess`）→ 转发 `s.client.DS.ListProcessInnerIPs` → 返回 `{Ips}`。`go build ./cmd/...` 通过。

## Phase 4：网关 / 文档

- [ ] **T008** 运行 `make api_docs` + `make bkapigw_docs` 生成 `docs/swagger`；核对新增 inner 路径 `/api/v1/inner/config/biz_id/{biz_id}/process/inner_ips` 的 `userVerifiedRequired:false`、`isPublic:false`（AC-S01）；`git diff docs/swagger` 为纯新增。

## Phase 5：质量门禁

- [ ] **T009** `gofmt -w` 所有改动 Go 文件；`golangci-lint`（docker `golangci/golangci-lint:v2.8.0`）对改动文件 0 issue。
- [ ] **T010** 全量 `go build ./...` + `go test ./cmd/data-service/service/...` 通过；产出交付清单，回填 `process.log`。

## 交付边界

- **本次提交范围**：bk-bscp 仓库内 proto / 生成物 / config-server / data-service / 单测 / swagger 文档。
- **不包含**：bk-sops 变量插件、bscp-proc-cfg 客户端（独立仓库，下游另行切换数据源）。
