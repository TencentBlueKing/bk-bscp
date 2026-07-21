# Clarification Questions — Story 136320799

## Q1 [dropped] — 来源：技术澄清
**问题**：需求优先级（High/Middle/Low）未在 TAPD 标注，需产品确认（对应 req.md Q-001）。
**影响**：影响排期，不影响技术方案与实现路径；非阻塞。
**建议候选**：无
**提出方**：主会话 / attempt=1 / round=1 / ts=2026-07-21T21:05:00+08:00
**放弃理由**：属产品排期决策，非技术阻塞项，本技术澄清阶段不处理，交由产品在 TAPD 标注。

## Q2 [resolved_by_doc] — 来源：subagent(speckit.specify)
**问题**：当 CMDB 侧字段被清空（如服务实例名称为空）时，bscp 是覆盖为空还是保留旧值（对应 req.md Q-002）？
**影响**：决定 F-001 四字段增量更新的空值语义；非阻塞（有明确对标基准可自答）。
**建议候选**：
- A. 直接以 CMDB 值覆盖（含覆盖为空）（推荐：对齐 gsekit 对标基准 R-001）
- B. 空值保留旧值（仿 os_type/agent_status 的空值保护）
**提出方**：subagent(speckit.specify) / attempt=1 / round=1 / ts=2026-07-21T21:05:00+08:00
**答复**：采用 A——直接以 CMDB 值覆盖。依据：①gsekit `sync_biz_process` 对 to_be_updated_process 执行 `bulk_update`，直接用 CMDB `list_process_related_info` 的 set/module/service_instance 名称与 `bk_set_env` 覆盖，无空值保护；②这四个字段与「别名/进程属性/实例数」同属 `process_related_info` 这一份权威数据源，不同于 os_type（来自 list_hosts）、agent_status（来自 gse ListAgentState）这类**独立且可能失败**的接口——bscp 现有 `resolveOsType` 与 `agentStatusChanged != ""` 的空值保护正是为规避「接口失败→空值→误覆盖」而设，四字段不存在该失败语义，机械套用空值保护反而会导致 bscp 与 CMDB 长期不一致，违背「与 CMDB 保持一致」的验收目标（AC-001~003）。若产品后续要求空值保护，可覆盖此结论。
**答复方**：subagent(自答) / ts=2026-07-21T21:05:00+08:00
**文档来源**：bk-process-config-manager/apps/gsekit/process/handlers/process.py（sync_biz_process，L472-544）；internal/processor/cmdb/sync_cmdb.go（BuildProcessChanges/resolveOsType，L1610-1820）

## Q3 [resolved_by_doc] — 来源：subagent(speckit.specify)
**问题**：F-003 已确认缺口补齐的具体范围（对应 req.md Q-003）？
**影响**：影响 F-003 工作量与 spec 范围；非阻塞（依赖 F-002 产出，可延后）。
**建议候选**：
- A. 待 F-002 对比清单产出 + 用户确认后再界定范围（推荐）
**提出方**：subagent(speckit.specify) / attempt=1 / round=1 / ts=2026-07-21T21:05:00+08:00
**答复**：F-003 范围以 F-002 对比清单结论 + 用户确认为准，本澄清阶段不阻塞收敛；F-001（四字段增量同步）根因明确、可独立实现与验收，先行推进。F-002 产出后若发现额外缺口，需回到需求侧补充范围并重估工时（与 req.md RICE 备注一致）。
**答复方**：subagent(自答) / ts=2026-07-21T21:05:00+08:00
**文档来源**：specs/stories/136320799/req.md（F-002/F-003、RICE 评分明细备注）
