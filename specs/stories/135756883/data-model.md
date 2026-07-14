# 数据模型与契约 — Story 135756883

## 1. proto 消息（新增）

### config-server（`pkg/protocol/config-server/config_service.proto`）

```proto
// RPC（进程管理分组内，紧邻 ListProcess）
rpc ListProcessInnerIPs(ListProcessInnerIPsReq) returns (ListProcessInnerIPsResp) {
  option (google.api.method_visibility).restriction = "INTERNAL,BKAPIGW";
  option (google.api.http) = {
    post: "/api/v1/config/biz_id/{biz_id}/process/inner_ips"
    body: "*"
    additional_bindings {
      post: "/api/v1/inner/config/biz_id/{biz_id}/process/inner_ips"
      body: "*"
    }
  };
}

message ListProcessInnerIPsReq {
  uint32 biz_id = 1;
  pbproc.ProcessSearchCondition search = 2;
}

message ListProcessInnerIPsResp {
  repeated string ips = 1;
}
```

### data-service（`pkg/protocol/data-service/data_service.proto`）

```proto
rpc ListProcessInnerIPs(ListProcessInnerIPsReq) returns (ListProcessInnerIPsResp) {}

message ListProcessInnerIPsReq {
  uint32 biz_id = 1;
  pbproc.ProcessSearchCondition search = 2;
}

message ListProcessInnerIPsResp {
  repeated string ips = 1;
}
```

> `pbproc` 为 `pkg/protocol/core/process/process.proto` 的 import 别名（两个 proto 均已 import）。

## 2. 复用的现有契约（不改动）

`ProcessSearchCondition`（`process.proto:88-113`）关键字段：

| 字段 | 说明 | 本接口是否使用 |
|------|------|--------------|
| `environment` (10) | 环境类型（1/2/3） | 是（表达式范围下必填） |
| `expression_scope` (12) | 五段表达式范围 | 是（主过滤条件） |
| 其余（sets/modules/inner_ips/...） | 其它过滤维度 | 透传，允许但变量插件不使用 |

`ExpressionScope`（`process.proto:154-166`）：`set_name` / `module_name` / `service_name` / `process_alias` / `process_id`。

## 3. 领域取值

- 返回值 `ips`：来源 `table.Process.Spec.InnerIP`，去重、保序、跳过空串。
- 数据范围：严格限定入参 `biz_id`（`dao.List` 内置 `m.BizID.Eq(bizID)`）。

## 4. 错误码

| 场景 | 错误 |
|------|------|
| `biz_id` 为 0 / 非法 | `errf.InvalidParameter` |
| 表达式范围模式缺 `environment` | `errf.InvalidParameter`（"environment is required for expression scope"） |
| 非法表达式 | `errf.InvalidParameter`（由 `filterProcessesByExpressionScope` 抛出，透传） |
