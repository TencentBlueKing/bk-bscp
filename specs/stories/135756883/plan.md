# 实现计划 — 进程 IP 查询 inner 接口（Story 135756883）

## 0. 落点总览

| 层 | 文件 | 改动 |
|----|------|------|
| proto（cs） | `pkg/protocol/config-server/config_service.proto` | 新增 `ListProcessInnerIPs` RPC + 2 消息 + inner binding |
| proto（ds） | `pkg/protocol/data-service/data_service.proto` | 新增 `ListProcessInnerIPs` RPC + 2 消息 |
| 生成物 | `pkg/protocol/config-server/**`、`pkg/protocol/data-service/**` | `make pb` 重新生成 |
| service（ds） | `cmd/data-service/service/process.go` | 新增 handler + `dedupInnerIPs` + 环境校验 helper |
| service（cs） | `cmd/config-server/service/process.go` | 新增 handler（IAM 鉴权 + 转发 DS） |
| test | `cmd/data-service/service/process_test.go` | 新增单元测试（去重 / 环境校验 / 空集） |
| 文档 | `docs/swagger/**` | `make api_docs` / `make bkapigw_docs` 生成（含 inner authConfig） |

## 1. 核心新逻辑（可 TDD 的单元）

1. `dedupInnerIPs(processes []*table.Process) []string`：遍历命中进程，取 `Spec.InnerIP`，保序去重、跳过空串。覆盖 AC-002/AC-003/AC-004/AC-006。
2. 环境校验（表达式范围下 `environment` 必填）：对齐既有 `"environment is required for expression scope"`。覆盖 AC-005。

## 2. data-service handler 逻辑

```go
func (s *Service) ListProcessInnerIPs(ctx, req *pbds.ListProcessInnerIPsReq) (*pbds.ListProcessInnerIPsResp, error) {
    kt := kit.FromGrpcContext(ctx)
    search := req.GetSearch()
    // 表达式范围下环境类型必填（AC-005）
    if err := validateExpressionEnv(search); err != nil {
        return nil, err
    }
    procs, _, err := s.dao.Process().List(kt, req.GetBizId(), search, &types.BasePage{All: true}) // 全量（F-003/AC-006）
    if err != nil {
        return nil, err
    }
    return &pbds.ListProcessInnerIPsResp{Ips: dedupInnerIPs(procs)}, nil
}
```

## 3. config-server handler 逻辑

对齐 `ListProcess`：IAM 鉴权（`FindBusinessResource` + `ProcConfigMgmt.View`）→ 转发 `s.client.DS.ListProcessInnerIPs` → 返回 `{Ips}`。

## 4. TDD 执行顺序（Phase）

- **Phase 0**：proto 新增 + `make pb` 生成 → `go build ./...` 通过（红：新 handler 未实现时 pbds 客户端方法已存在）。
- **Phase 1（红）**：写 `dedupInnerIPs` / `validateExpressionEnv` 单元测试，断言签名与行为；此时函数未实现，编译失败（红）。
- **Phase 2（绿）**：实现 `dedupInnerIPs` / `validateExpressionEnv` + data-service handler；单测转绿。
- **Phase 3（绿）**：实现 config-server handler；`go build ./cmd/...` 通过。
- **Phase 4**：`make api_docs` + `make bkapigw_docs` 生成文档，检查 diff 中新 inner 路径 `userVerifiedRequired:false`。
- **Phase 5**：`gofmt` + `golangci-lint`（docker v2.8.0）+ 全量 `go build`。

## 5. 验证策略

- 单元测试：`go test ./cmd/data-service/service/ -run TestListProcessInnerIPs -run TestDedupInnerIPs`（去重、空集、环境校验）。
- 表达式过滤等价性（AC-001）：继承既有 `process_expression` 相关测试，不重复实现。
- 构建：`go build ./...`。
- 静态检查：`gofmt -l`、golangci-lint。
- 文档：`git diff docs/swagger` 核对纯新增。

## 6. 不做

- 不改 `ListProcess` 及既有过滤逻辑。
- 不改 bk-sops / bscp-proc-cfg（独立仓库）。
- 不加分页、不加 IPv6、不回源 CMDB。
