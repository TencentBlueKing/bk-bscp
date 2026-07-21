# 技术规范 — 进程 IP 查询 inner 接口（Story 135756883）

## 1. 目标

为 BSCP 提供一个**过滤语义对齐 gsekit `process/process_status`** 的进程 IP 查询接口，以 **inner（内部调用、网关侧不走用户认证）** 方式暴露，直接返回**去重后的内网 IP 列表**，供标准运维「GSEKit IP 选择器」变量插件切换数据源使用。

## 2. 技术澄清结论

需求文档（`req.md`）已完成 3 轮业务澄清。本阶段针对实现方案的 4 个技术问题，均由文档与现有源码自答：

| # | 问题 | 结论 | 依据 |
|---|------|------|------|
| TC-1 | 复用 `ListProcess` 还是新增独立 inner RPC？（对应 req Q-002） | **新增独立 RPC `ListProcessInnerIPs`** | `ListProcess` 返回完整进程对象 + 实例 + filter_options + CMDB URL，与「仅返回去重 IP 列表」语义不符；新增专用 RPC 更贴合需求、响应更轻、便于下游变量插件直接消费 |
| TC-2 | inner 接口如何暴露？ | 沿用现有范式：proto `additional_bindings` 增加 `/api/v1/inner/...` 路径；BK APIGW 侧 `userVerifiedRequired:false`（由 `inject_bk_gateway.py` 生成） | `ListProcess`/`OperateProcess`/`GenerateConfig` 均为该范式；无独立 inner server |
| TC-3 | 过滤逻辑是否重写？ | **完全复用** `dao.Process().List` + `handleSearch` + `process_expression.go` 的 `ExpressionScope` 内存匹配 | 已落地且与 gsekit 对齐；不引入重复过滤逻辑（约束条件要求） |
| TC-4 | AC-005「表达式范围下环境类型必填」在现有 List 路径是否已校验？ | **未校验**——`handleSearch` 走的 `matchedCcIDsByExpressionScope → loadExpressionCandidates` 对空 environment 不报错。故新接口需在入口显式补充该校验 | `internal/dal/dao/process.go:490-496`、`process_expression.go:30-40`；对齐 `getByExpressionScope` 的 `environment is required for expression scope` 报错语义 |

### 2.1 config-server 鉴权取舍

新 RPC 的 config-server handler **保留与 `ListProcess` 一致的 IAM 鉴权**（`FindBusinessResource` + `ProcConfigMgmt.View`）。"inner 不走用户认证"由 **BK APIGW 层** `userVerifiedRequired:false` 保证（对齐 AC-S01），与现有 inner 接口（`GenerateConfig` 等）规范一致；不在本需求内改动 config-server 的 IAM 逻辑，避免越权引入鉴权行为差异。

### 2.2 scope 边界：下游独立仓库不在本次提交内

`bk-sops/`（变量插件 `var_gse_kit_ip_selector.py`）与 `bscp-proc-cfg/`（客户端 `client.go`）在本工作区中均为**独立的嵌套 git 仓库**（各自 `.git`，未被 bk-bscp 跟踪）。本需求在 **bk-bscp 仓库**内落地 IP 查询 inner 接口；bk-sops 变量插件改调 BSCP、bscp-proc-cfg 新增客户端方法属**下游独立仓库改动**，不纳入本次 bk-bscp 提交（如需，另行在对应仓库处理）。

## 3. 接口契约

### 3.1 新增 RPC：`ListProcessInnerIPs`

- config-server（`config_service.proto`）与 data-service（`data_service.proto`）各新增同名 RPC。
- HTTP 绑定（config-server）：
  - public：`POST /api/v1/config/biz_id/{biz_id}/process/inner_ips`
  - inner：`POST /api/v1/inner/config/biz_id/{biz_id}/process/inner_ips`（`additional_bindings`）
- `method_visibility` = `INTERNAL,BKAPIGW`（对齐 `ListProcess`）。

### 3.2 请求 / 响应

复用 `pbproc.ProcessSearchCondition` 作为过滤条件（变量插件设置其 `environment` + `expression_scope` 两个字段即可），避免新增重复的过滤入参抽象。

```proto
message ListProcessInnerIPsReq {
  uint32 biz_id = 1;
  pbproc.ProcessSearchCondition search = 2;
}

message ListProcessInnerIPsResp {
  repeated string ips = 1;  // 去重后的内网 IP 列表
}
```

## 4. 功能规范映射

| 需求功能点 | 规范实现 |
|-----------|---------|
| F-001 IP 查询接口（表达式过滤 + 去重 IP 列表） | 新 RPC；data-service 复用 `dao.List(All=true)` 拿命中进程 → 提取 `Spec.InnerIP` → 去重返回 |
| F-002 inner 暴露 | proto `additional_bindings` inner 路径 + APIGW `userVerifiedRequired:false` |
| F-003 全量返回不分页 | data-service 固定 `types.BasePage{All: true}` |
| R-001 过滤语义对齐 gsekit | 复用 `ExpressionScope` 匹配（`process_expression.go`），语义不变 |
| R-003 返回去重 IP 列表 | 服务层对 `inner_ip` 去重（保序、跳过空值） |

## 5. 验收标准落地

| AC | 落地方式 | 验证手段 |
|----|---------|---------|
| AC-001 命中进程集合与 gsekit 等价 | 复用既有 `ExpressionScope` 过滤（已与 gsekit 对齐，本次不改） | 继承既有 dao 表达式测试 + 新增单测 |
| AC-002 返回 IP 集合 = 命中进程 `inner_ip` 去重 | 服务层去重逻辑 | 单元测试 `dedupInnerIPs` |
| AC-003 同主机多进程 IP 只出现一次 | 去重保序，重复 IP 跳过 | 单元测试 |
| AC-004 命中为空返回空列表 | 命中为空 → 空 IP 列表（不降级全选，依赖既有 `IN 空集` 行为） | 单元测试（空进程集 → 空列表） |
| AC-005 表达式范围下缺环境类型 → 参数错误 | 入口校验：`expression_scope != nil && environment == ""` → `InvalidParameter` | 单元测试校验函数 |
| AC-006 全量返回不翻页 | `BasePage{All: true}` | 代码审查 + 单测（构造多于单页的 IP） |
| AC-007 不跨业务 | `dao.List` 强制 `biz_id` 过滤（既有） | 代码审查（复用既有 biz 约束） |
| AC-S01 inner 不经公开网关用户认证 | APIGW inner 路由 `userVerifiedRequired:false`（inject 脚本 + 文档生成） | 检查 `docs/swagger/bkapigw` diff |

## 6. 非目标（本期不做）

- 不新增 IP 选择器 UI 组件（属标准运维侧）。
- 不支持 IPv6（仅返回 `inner_ip`，不含 `inner_ip_v6`）。
- 不实时回源 CMDB；仅用本地 `processes` 表。
- 接口自身不分页。
- 不改动 bk-sops / bscp-proc-cfg（独立仓库，下游另行切换）。
