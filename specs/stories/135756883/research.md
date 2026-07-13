# 技术调研 — 进程 IP 查询 inner 接口（Story 135756883）

## 1. 现有 ListProcess 链路（复用基础）

```
api-server(UnifiedAuthentication) → config-server.ListProcess(IAM 鉴权) → data-service.ListProcess → dao.Process().List → handleSearch
```

- `cmd/config-server/service/process.go:53-82`：`ListProcess` 做 IAM 鉴权后透传 `search` 给 data-service。
- `cmd/data-service/service/process.go`：`ListProcess` 调 `dao.Process().List(kt, bizID, search, &types.BasePage{...})`，随后组装进程实例/模板等重对象。
- `internal/dal/dao/process.go:392-499`：`List` + `handleSearch`。`search.expression_scope` 非空时经 `matchedCcIDsByExpressionScope` 归一为命中 CC 进程 ID，再 `CcProcessID IN (...)`；命中为空即空结果集（不降级全选）。

## 2. inner 接口暴露机制（无独立 inner server）

inner 是**同一 service 的双 HTTP binding**：

1. proto：`google.api.http` 主路径 + `additional_bindings { post: "/api/v1/inner/..." }`（见 `config_service.proto:1536-1543` 的 `ListProcess`）。
2. grpc-gateway 生成 `*_0`（public）/ `*_1`（inner）两个 handler。
3. BK APIGW：`scripts/bk_gateway/inject_bk_gateway.py` 为 `/inner/` 路径注入 `isPublic:false`、`userVerifiedRequired:false`、`appVerifiedRequired:true`，并将 backend path 去掉 `/inner/` 前缀转发到 config-server。
4. config-server 内 IAM 鉴权对 inner/public 无分支（`ListProcess`/`GenerateConfig` 均保留 authorize）。

**结论**：新 RPC 照抄 `ListProcess` 的 binding 与 visibility 即可获得 inner 能力；"关闭用户认证"由 APIGW 侧完成。

## 3. ExpressionScope 过滤（对齐 gsekit，本次不改）

`internal/dal/dao/process_expression.go`：

- `loadExpressionCandidates`：按 `biz_id`+`environment`（排除 deleted）加载候选。
- `filterProcessesByExpressionScope`：候选按 `CcProcessID` 升序（对齐 gsekit 主键序）→ 五段拼 `expression.JoinProcessExpression` → `expression.ScopeToCcIDs` 内存匹配 → 非法表达式返回 `InvalidParameter`。
- `matchedCcIDsByExpressionScope`：上述组合，返回命中 CC 进程 ID。

**AC-005 缺口确认**：`handleSearch` 的表达式分支调用 `matchedCcIDsByExpressionScope(..., search.GetEnvironment(), es)`，其 `loadExpressionCandidates` 对空 environment 只是"不加环境过滤"，**不报错**。而 `OperateProcess` 走的 `getByExpressionScope`（`process.go:585-596`）显式要求 `environment != ""`。因此新接口须在入口补 `environment` 必填校验，报错语义对齐 `"environment is required for expression scope"`。

## 4. 进程表字段

`pkg/dal/table/process.go` `ProcessSpec`：`InnerIP`（`inner_ip`）、`InnerIPV6`、`SetName`、`ModuleName`、`ServiceName`、`Environment`、`Alias`。`Attachment.CcProcessID` 为 CC 进程 ID。IP 取 `Spec.InnerIP`。

## 5. proto 生成与文档链路

- `make pb`：`clang-format` 格式化所有 proto → `cd pkg/protocol && make clean && make` 生成 `*.pb.go`/`*_grpc.pb.go`/`*.pb.gw.go`。
- `make api_docs` / `make bkapigw_docs`：生成 `docs/swagger/api` 与 `docs/swagger/bkapigw`。
- 工具链已确认可用：`/usr/local/bin/protoc`、`protoc-gen-go`、`make`。
- config-server 只有单一 proto 文件 `pkg/protocol/config-server/config_service.proto`。

## 6. 去重方案

命中进程集合可能同主机多进程（同一 `inner_ip` 多条）。服务层遍历命中进程，按出现顺序去重（`map[string]struct{}` 判重 + 保序 slice），跳过空 `inner_ip`。不使用 SQL `DISTINCT`（因表达式匹配是加载候选后的内存后过滤，无法在单条 SQL 内表达）。

## 7. 风险与对策

| 风险 | 对策 |
|------|------|
| proto 重新生成产生大量 diff | 仅新增 RPC/消息，生成物 diff 应为纯新增；生成后 `git diff` 核对无对既有接口改动 |
| data-service handler 需 DB 才能端到端测 | 核心新逻辑（去重、环境校验）抽为纯函数/小函数做单元测试；DB 依赖路径以既有 dao 测试与构建为可跑范围证据 |
| inner 网关 authConfig 需重新生成 | `make bkapigw_docs` + inject 脚本自动产出；检查 diff 中新路径 `userVerifiedRequired:false` |
| config-server IAM 对无用户的 inner 调用 | 与既有 `ListProcess`/`GenerateConfig` inner 行为一致，不在本需求改动，保持规范统一 |
