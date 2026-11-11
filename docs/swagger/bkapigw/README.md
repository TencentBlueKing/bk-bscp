# 说明
合并至[bkapigw.swagger.json](../bkapigw.swagger.json)和[apiserver.json](../apiserver/swagger.json)

## operationId 命名

operationId 即蓝鲸网关的资源名，也是网关 MCP 的工具名。`scripts/bk_gateway/inject_bk_gateway.py` 会把
`additional_bindings` 生成的 “主绑定名 + 序号” 改为按路径特征命名，主绑定名保持不变：

| 路径特征 | 名称 | 示例 |
| --- | --- | --- |
| 主绑定 | `Config_{Rpc}` | `Config_ListKvs` |
| 含 `/inner/` | `Config_Inner_{Rpc}` | `Config_Inner_ListKvs` |
| 含 `/projects/{projectId}/envs/{envId}` | `Config_ProjectEnv_{Rpc}` | `Config_ProjectEnv_ListKvs` |
| 仅含 `/projects/{projectId}` | `Config_Project_{Rpc}` | `Config_Project_ListHooks` |
| 以上组合 | `Config_Inner_ProjectEnv_{Rpc}` | `Config_Inner_ProjectEnv_Publish` |

同一 rpc 的多个绑定改名后重名时脚本报错，需调整 proto 中的绑定路径使其可区分。
