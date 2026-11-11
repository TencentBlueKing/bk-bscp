# 说明
合并至[bkapigw.swagger.json](../bkapigw.swagger.json)和[apiserver.json](../apiserver/swagger.json)

## operationId 命名

operationId 即蓝鲸网关的资源名，也是网关 MCP 的工具名。`scripts/bk_gateway/inject_bk_gateway.py` 会把
`additional_bindings` 生成的 “主绑定名 + 序号” 改为按路径特征命名，主绑定名保持不变：

| 路径特征 | 名称 | 示例 |
| --- | --- | --- |
| 主绑定 | `Config_{Rpc}` | `Config_ListKvs` |
| 含 `/inner/` | `Config_Inner_{Rpc}` | `Config_Inner_ListKvs` |
| 含 `/projects/{projectId}/envs/{envId}` | `Config_Env_{Rpc}` | `Config_Env_ListKvs` |
| 仅含 `/projects/{projectId}` | `Config_Project_{Rpc}` | `Config_Project_ListHooks` |
| 以上组合 | `Config_Inner_Env_{Rpc}` | `Config_Inner_Env_Publish` |

作用域标识取路径中最内层的维度：环境必定归属于某个项目，因此带项目和环境的路由只标 `Env`。

同一 rpc 的多个绑定改名后重名时脚本报错，需调整 proto 中的绑定路径使其可区分。

蓝鲸网关要求 “MCP Server 名称 + 工具名称” 不超过 64 字符，bscp 的 MCP Server 名称最长 24 字符
（如 `bk-bscp-prod-file-config`），因此非 `/inner/` 接口的 operationId 不超过 40 字符：

- 改名后超长时，脚本按缩写表压缩 rpc 部分（目前为 `Revisions` → `Revs`），
  如 `Config_Env_ListReleasedAppBoundTmplRevs`；
- 主绑定名或压缩后仍超长时脚本报错，需缩短 rpc 名或补充缩写；
- `/inner/` 接口仅供内部系统以应用身份调用，不注册为 MCP 工具，不受长度限制。

## MCP Server

`mcp_servers.yaml` 声明随 `init-apigateway` 同步到网关的 MCP Server（`bk-bscp-{stage}-kv-config`、
`bk-bscp-{stage}-file-config`），需在配置中开启 `apiGateway.mcp.enabled`。同步后网关按资源维度为
`esb.appCode` 与 `apiGateway.mcp.grantAppCodes` 授予 `resource_names` 并集的访问权限。

- `resource_names` 中的资源名即 MCP 工具名，被 MCP 使用后视为对外契约，不再改名：网关导入时资源改名等同
  删除旧资源再新建，引用旧名的 MCP 工具和按资源授予的权限都会丢失；
- `internal/iam/apigw` 的单测校验声明中的资源都存在于 `swagger.json` 且有请求参数、工具名长度不超限。
