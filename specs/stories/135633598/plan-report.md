# Plan Report — Story 135633598

## Verdict
pass

## Checked artifacts
- specs/stories/135633598/plan.md
- specs/stories/135633598/research.md
- specs/stories/135633598/data-model.md

## Reference baselines
- specs/stories/135633598/spec.md（F-001~F-008 / FR-001~FR-012 / R-001~R-004 / AC-*）
- CLAUDE.md（Go 规范、gofmt、.golangci.yml、不引入不必要抽象）
- .claude/skills/bk-security-redlines/SKILL.md（安全三大红线）
- .specify/memory/constitution.md（未填充模板，降级以 CLAUDE.md 为准）
- cmd/api-server/service/repo.go、routers.go；internal/dal/repository/{repository,bkrepo,cos}.go；Makefile；scripts/bk_gateway/inject_bk_gateway.py

## 维度核对结论

### 完整度
plan 覆盖 spec 全部需求：FR-001~FR-006/FR-012（skill 编排）、FR-007~FR-009（新 handler：
预检 Metadata → DownloadLink → 取首个 URL + 3600s + 鉴权链）、FR-010（老接口保留）、
FR-011（`make docs` 生成 + inject 网关）。F-001~F-008 均有落点与验证方式；AC-005/AC-006/
AC-P01/AC-S01/AC-T01/AC-T02 均映射到测试计划。无遗漏。

### research 合规
技术选型不违反分层依赖（复用 api-server 既有 Provider，不跨层新增 config-server 到存储的依赖）、
不违反鉴权红线（统一认证 + IAM + ContentVerified 防越权；内容存在性服务端强校验）、
不违反编码规范（gofmt/golangci、不引入不必要抽象、仅 1 handler+1 路由+1 常量导出）。

### 项目宪章
`.specify/memory/constitution.md` 为未填充模板（占位符未替换），按 pipeline 约定降级以
CLAUDE.md Go/安全约束为基线核对，无硬约束冲突。

## Findings

### A1
- **类别**：Completeness
- **严重性**：LOW
- **位置**：specs/stories/135633598/spec.md（技术方案 F-006）/ plan.md Phase 1
- **总结**：spec 未显式要求下载 URL handler 内先做内容存在性预检；plan 依据 AC-T01 与
  bkrepo/cos `DownloadLink` 不校验对象存在的事实，补充了 `Metadata` 预检步骤。
- **根因**：plan-self
- **修改建议**：已在 plan.md/research.md(R3) 落地为实现要点，tasks 阶段将其作为独立可测分支即可，无需回退 spec。

### A2
- **类别**：Completeness
- **严重性**：MEDIUM
- **位置**：spec.md / req.md（F-008 描述用 `make sg`）；Makefile 实际无 `sg` 目标
- **总结**：spec/req 引用的网关文档生成命令 `make sg` 在仓库 Makefile 中不存在，真实目标为
  `make docs`（= api_docs + bkapigw_docs + markdown_docs）或单独 `make markdown_docs`。
- **根因**：plan-self（plan 已识别并更正，未使 plan 不可执行）
- **修改建议**：tasks/skill 阶段统一使用 `make docs`/`make markdown_docs`；不使用 `make sg`。

### A3
- **类别**：Testability
- **严重性**：MEDIUM
- **位置**：spec.md「测试策略 · 单元测试」（提到 handler 覆盖「非 file 型服务 R-002 报错分支」）
- **总结**：下载 URL handler 是内容级接口（按 sign 操作），本身不感知 app `config_type`；
  R-002（文件型 only）由 skill 编排层在操作前校验 `config_type=file` 落地。将 R-002 归到 handler
  单测会得到一个 handler 实际不实现的分支。
- **根因**：plan-self
- **修改建议**：plan/research(R8) 已澄清 R-002 归属 skill/E2E 层；handler 单测不含该分支，
  tasks 阶段据此拆分测试职责。

### A4
- **类别**：Architecture
- **严重性**：LOW
- **位置**：internal/dal/repository/bkrepo.go（`tempDownloadURLExpireSeconds`）
- **总结**：为避免 `expire_seconds` 在 handler 硬编码 3600 与存储层重复，plan 计划将该常量导出。
- **根因**：plan-self
- **修改建议**：导出为 `TempDownloadURLExpireSeconds`（值不变），仅包内引用，向后兼容，无外部破坏。

## 结论
无 HIGH/CRITICAL finding，全部为 MEDIUM/LOW 且均已在 plan/research 中给出处置。按 report-template
plan 阶段判定规则，Verdict = **pass**，可进入 tasks 阶段。
