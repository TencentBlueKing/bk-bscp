# bscp AIDev Skill 同步包

部署时由 Helm hook Job `bk-bscp-bkai-sync` 运行 `bkai-init`，把 bscp 的 Skill 同步到 AIDev。
本包只包含 Skill，不包含智能体、知识库，也不创建 MCP；MCP 由 `api-server migrate init-apigateway`
按 `docs/swagger/bkapigw/mcp_servers.yaml` 同步到网关。

协议版本：bkai-init 0.1.6。

## 目录

```text
scripts/bkai-syncing/
├── Dockerfile
├── Dockerfile.dockerignore
├── README.md
└── package/
    ├── bkai.yaml
    └── skills/
        ├── bkbscp-kv-config/
        │   └── skill.yaml
        └── bkbscp-file-config/
            └── skill.yaml
```

`SKILL.md` 不在本目录维护，构建镜像时从 `.agents/skills/bkbscp-*/SKILL.md` 复制到 `/app/package/skills/`。

| Skill code | 名称 | 配套 MCP |
|-----------|------|---------|
| `bkbscp-kv-config` | 服务配置中心-KV配置管理 | `bk-bscp-prod-kv-config` |
| `bkbscp-file-config` | 服务配置中心-文件配置查看 | `bk-bscp-prod-file-config` |

## 构建与校验

在仓库根目录执行：

```bash
make bkai-image
docker run --rm bk-bscp-bkai-init:latest validate -f /app/package/bkai.yaml --space bkaidev
```

`validate` 只检查本地配置。同步需要以下环境变量（Helm Job 中自动注入）：

- `BK_API_URL_TMPL`：须包含 `{api_name}/{stage}`，如 `https://bkapi.example.com/api/{api_name}/{stage}`
- `BKAI_APP_CODE`、`BKAI_APP_SECRET`：已开通 `bk-aidev` 网关接口权限及目标空间权限的应用
- `BKPAAS_MULTI_TENANT_MODE`、`BKAI_TENANT_ID`：多租户模式与租户 ID

```bash
docker run --rm --env BK_API_URL_TMPL --env BKAI_APP_CODE --env BKAI_APP_SECRET \
  bk-bscp-bkai-init:latest sync --confirm -f /app/package/bkai.yaml --space bkaidev
```

Skill 不填版本号，由平台按内容 hash 递增；内容不变时重复同步不会升版本。
