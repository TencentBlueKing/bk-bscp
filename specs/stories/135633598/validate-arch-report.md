# Validate-Arch Report — Story 135633598

## Verdict
LGTM

## Checked artifacts
- cmd/api-server/service/repo.go（新增 `DownloadFileURL` handler + `DownloadURLResponse` DTO + swag 注解）
- cmd/api-server/service/routers.go（新增 `/content/download_url` 路由）
- internal/dal/repository/bkrepo.go（导出常量 `TempDownloadURLExpireSeconds`）
- cmd/api-server/service/repo_test.go（新增 8 个 handler 单测）
- .claude/skills/bscp-file-config/SKILL.md（文档产物，非分层对象）
- docs/swagger/**（`make docs` 生成物）

## Reference baselines
- specs/stories/135633598/spec.md / plan.md / research.md / data-model.md（落点决策）
- internal/dal/repository/repository.go（`Provider` / `ObjectDownloader` / `BaseProvider` 接口与 `GetFileSign`）
- cmd/api-server/service/repo.go 既有 `DownloadFile` handler（落点参考）
- CLAUDE.md（不引入不必要抽象/兼容层）

## 架构校验维度

### 1. 分层架构约束（依赖方向）
- 新 handler `DownloadFileURL` 位于 `cmd/api-server/service`（管理面 HTTP 层），仅通过
  `s.authorizer.Authorize` 与 `s.provider.Metadata` / `s.provider.DownloadLink` 调用下游，
  依赖方向为 handler → `internal/dal/repository` 的 `Provider` 接口，与既有 `DownloadFile`
  完全一致，无越层直连 bkrepo/cos 具体实现。
- 常量 `TempDownloadURLExpireSeconds` 由存储层 `bkrepo.go` 导出、handler 引用，属上层依赖下层
  的正向引用，避免 `expire_seconds` 魔法数字在两处重复，符合 data-model.md 约定。

### 2. 循环依赖
- `cmd/api-server/service` → `internal/dal/repository` 为单向依赖；`bkrepo.go` 未反向 import
  service 包。常量导出仅改可见性，不新增跨包边引用，无新增环。

### 3. 模块边界
- 新接口落在管理面 api-server（而非 config-server），与 spec/plan 落点决策一致。
- 复用既有 `repository.Provider` 接口既有方法 `Metadata` / `DownloadLink`（`ObjectDownloader`
  / `BaseProvider` 已声明），未新造 Provider 抽象或新接口方法。
- 复用既有鉴权链：路由挂在既有 `UnifiedAuthentication + BizVerified + ContentVerified` 组内，
  handler 内 IAM `Authorize`（Biz `FindBusinessResource` + App `View`）与 `DownloadFile` 同构。
- 老 `/content/download`、`/content/metadata` 未改动，符合 FR-010 向后兼容。

### 4. Code scope 白名单
- 全部改动文件（repo.go / routers.go / bkrepo.go / repo_test.go / bscp-file-config SKILL.md /
  docs/swagger）均在 context.md 的 Code scope 白名单内，无越界写入。

## Findings

### A1
- **类别**：Architecture
- **严重性**：LOW
- **位置**：cmd/api-server/service/repo.go:211-215（`DownloadURLResponse`）
- **总结**：响应 DTO 放在 service 包，而既有 `MetadataResponse`/`ObjectMetadata` 在 repository 包，风格上不完全就近。
- **根因**：code-self
- **修改建议**：无需修改。plan.md/data-model.md 明确允许"service 包或 repository 包取其一保持一致"；
  将 HTTP 响应 DTO 定义在其唯一使用者所在的 handler 包内，反而避免存储层感知 HTTP 响应形态，
  边界更清晰，是可接受的合理选择。仅登记，不影响 Verdict。

## Verdict 依据
无 CRITICAL/HIGH（无 [必须] 项），分层/循环依赖/模块边界/Code scope 四维度均合规，
复用既有 Provider 与鉴权链、未引入多余抽象，符合 CLAUDE.md 约束 → **LGTM**。
