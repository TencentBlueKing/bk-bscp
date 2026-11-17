# bscp AIDev Skill 同步包

部署时由 Helm hook Job `bk-bscp-bkai-sync` 运行 `bkai-init`，把 bscp 的 Skill 同步到 AIDev。
本包只包含 Skill，不包含智能体、知识库，也不创建 MCP；MCP 由 `api-server migrate init-apigateway`
按 `docs/swagger/bkapigw/mcp_servers.yaml` 同步到网关。

协议版本：bkai-init 0.1.6。

## 目录

```text
scripts/bkai-syncing/
├── README.md
└── package/
    ├── bkai.yaml
    └── skills/
        ├── bkbscp-kv-config/
        │   └── skill.yaml
        ├── bkbscp-file-config/
        │   └── skill.yaml
        └── bkbscp-process-config/
            └── skill.yaml
```

`SKILL.md` 不在本目录维护，构建 hyper 镜像时从 `.agents/skills/bkbscp-*/SKILL.md` 复制进来。

| Skill code | 名称 | 配套 MCP |
|-----------|------|---------|
| `bkbscp-kv-config` | 服务配置中心-KV配置管理 | `bk-bscp-prod-kv-config` |
| `bkbscp-file-config` | 服务配置中心-文件配置查看 | `bk-bscp-prod-file-config` |
| `bkbscp-process-config` | 服务配置中心-进程配置管理 | `bk-bscp-prod-process-config` |

## 运行方式

bscp 不单独构建同步镜像：

1. hyper 镜像（`bk-bscp-hyper`）在 `/bk-bscp/bkai/` 下携带完整同步包（本目录 `package/` + 各 skill 的 `SKILL.md`）。
2. Helm Job 的 initContainer 使用 hyper 镜像，把 `/bk-bscp/bkai/` 复制到共享 emptyDir。
3. 主容器直接使用 AIDev 平台提供的 `bk-aidev-init` 镜像（地址在 chart `bkai.image` 中配置），执行
   `sync --confirm -f <emptyDir>/bkai.yaml`。

这样 Skill 内容随 bscp 版本发布，`SKILL.md` 只在本仓库维护一份。

## 本地校验

在仓库根目录构建 hyper 镜像后，把同步包取出来，用 `bk-aidev-init` 校验：

```bash
make docker
docker run --rm -v /tmp/bkai-package:/out --entrypoint cp bk-bscp-hyper:latest -r /bk-bscp/bkai/. /out/
docker run --rm -v /tmp/bkai-package:/pkg:ro hub.bktencent.com/blueking/bk-aidev-init:0.1.6 \
  validate -f /pkg/bkai.yaml --space bkaidev
```

`validate` 只检查本地配置。同步需要以下环境变量（Helm Job 中自动注入）：

- `BK_API_URL_TMPL`：须包含 `{api_name}/{stage}`，如 `https://bkapi.example.com/api/{api_name}/{stage}`
- `BKAI_APP_CODE`、`BKAI_APP_SECRET`：已开通 `bk-aidev` 网关接口权限及目标空间权限的应用
- `BKPAAS_MULTI_TENANT_MODE`、`BKAI_TENANT_ID`：多租户模式与租户 ID

Skill 不填版本号，由平台按内容 hash 递增；内容不变时重复同步不会升版本。
