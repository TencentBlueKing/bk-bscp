# Validate-Security Report — Story 135633598

## Verdict
LGTM

## Checked artifacts
- cmd/api-server/service/repo.go（`DownloadFileURL` handler + `DownloadURLResponse` DTO）
- cmd/api-server/service/routers.go（`/api/v1/biz/{biz_id}/content/download_url` 路由注册）
- cmd/api-server/service/repo_test.go（handler 单元测试）
- internal/dal/repository/bkrepo.go（`TempDownloadURLExpireSeconds` 常量、`DownloadLink` 预签名实现）
- internal/dal/repository/repository.go（`GetFileSign` 输入校验、`Metadata`/`DownloadLink` 接口）
- internal/iam/auth/middleware.go（`ContentVerified` 鉴权中间件）

## Reference baselines
- .claude/skills/bk-security-redlines/SKILL.md（三大红线：输入校验 / 鉴权 / 敏感数据加密）
- specs/stories/135633598/spec.md（SC-004/005/006、AC-P01/S01/T01/T02）
- specs/stories/135633598/req.md（安全与合规、AC-S01/AC-T01）
- specs/stories/135633598/research.md（R3 内容存在性预检 / R4 鉴权链路）

## 安全校验结论（按四维）

### 1) 输入校验（红线 1）
- `sign` 经 `repository.GetFileSign` 强约束：`strings.ToLower` + 长度必须 `== 64`，否则 400；与
  `UploadFile`/`DownloadFile` 复用同一校验路径，行为一致。
- `Metadata` 预检对 bkrepo 返回的 `X-Checksum-Sha256` 做 `sha256 != sign` 断言，非匹配对象直接报错，
  为下载 URL 生成提供了「对象真实存在且校验和一致」的二次约束。
- 结论：满足红线 1。仅存 1 处非阻断建议项（见 Findings A1，sign 未做 hex 字符集白名单）。

### 2) 鉴权（红线 2）
- IAM 用户级：handler 内 `Authorize`（Biz `FindBusinessResource` + App `View`），与 `DownloadFile`
  完全一致，先于任何 URL 生成执行。
- 中间件链：路由挂在 `/content` 下载组 `UnifiedAuthentication + BizVerified + ContentVerified`，
  与老 `/content/download` 同组，业务级 + 服务/内容级鉴权一致（满足 AC-S01、FR-009/FR-010）。
- 横向越权：`ContentVerified` 校验 sign 与 app/template_space 归属，`BizVerified` 锁定 BizID，
  防跨业务/跨服务读取他人内容。
- 鉴权失败拦截点在返回 URL 之前（`Authorize` 位于 handler 首步），`TestDownloadFileURL_Unauthorized`
  断言非 200 且 `DownloadLink` 未被调用、响应体不含 URL。
- 结论：满足红线 2，无越权与鉴权绕过。

### 3) 敏感数据（红线 3）
- 返回临时预签名 URL（bkrepo `type=DOWNLOAD`），有效期 `TempDownloadURLExpireSeconds=3600`，到期失效；
  `expire_seconds` 一并返回，符合 SC-005/AC-T02。响应体仅含 URL 与有效期，不透传文件字节（AC-P01）。
- 无硬编码密钥/凭证：bkrepo `Username/Password` 由配置注入（`newBKRepoClient`），非硬编码。
- 预签名 URL 内含临时下载 token 属对象存储标准做法且到期失效，非长期凭证外泄，符合设计预期。
- 结论：满足红线 3。

### 4) 常见风险
- 路径穿越：`sign` 仅经 provider 拼接 bkrepo node 路径；长度锁定 64 + 小写化 + `Metadata` 校验和预检
  形成纵深防御，实际穿越面低（字符集白名单缺失见 A1）。
- SSRF：下载 URL 由 bkrepo 服务端生成，host 来自配置，非用户可控，无 SSRF。
- 空对象 URL（AC-T01）：handler 在生成 URL 前先 `Metadata` 预检，命中 `errf.ErrFileContentNotFound`
  即返回「内容未上传」错误且**不调用** `DownloadLink`；`TestDownloadFileURL_ContentNotUploaded`
  断言不调用 `DownloadLink` 且响应体不泄露 URL。核心风险已妥善处理。

## Findings

### A1
- **类别**：Security / 输入校验
- **严重性**：MEDIUM
- **位置**：internal/dal/repository/repository.go:111-118（`GetFileSign`）
- **总结**：`sign` 仅校验长度 `== 64` 与小写化，未做 hex 字符集（`^[0-9a-f]{64}$`）白名单校验，
  理论上非 hex 的 64 字符（含 `/`、`.` 等）可进入下游 bkrepo 路径拼接。
- **根因**：code-self（继承既有 `GetFileSign`，非本次新增引入）
- **修改建议**：为纵深防御可在 `GetFileSign` 增加 hex 白名单正则校验（`regexp.MustCompile(\`^[0-9a-f]{64}$\`)`）。
  当前因 `Metadata` 预检对 `X-Checksum-Sha256 != sign` 断言 + 路径固定 64 长度双重约束，实际可利用性低，
  故判 MEDIUM 建议、不阻断本次发布；若采纳请同步覆盖 `UploadFile`/`DownloadFile` 等复用方，避免行为分叉。

### A2
- **类别**：Security / 敏感数据（信息泄露）
- **严重性**：LOW
- **位置**：cmd/api-server/service/repo.go:256-264（`Metadata`/`DownloadLink` 错误分支 `rest.BadRequest(err)`）
- **总结**：provider 原始 error 直接经 `rest.BadRequest(err)` 回显给调用方，可能带出少量内部状态描述。
- **根因**：code-self（与既有 `DownloadFile` 错误处理一致）
- **修改建议**：非阻断。现有 provider 错误文案（如 `metadata status %d != 200`）不含 host/凭证，泄露面小；
  如需收敛可统一为脱敏文案，但与既有接口保持一致亦可接受。

## 判定说明
无 CRITICAL/HIGH 级 [必须] 项；三大红线（输入校验 / 鉴权 / 敏感数据）与 AC-S01/AC-T01/AC-T02/AC-P01
均满足。A1（MEDIUM）/A2（LOW）为纵深防御建议项，不阻断本次交付。据 report-template validate 段规则：
无 CRITICAL/HIGH → **LGTM**。
