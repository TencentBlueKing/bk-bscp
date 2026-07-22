# Context for Story 136320799

## Stage
validate

## Source artifacts
- specs/stories/136320799/req.md                       # 所有阶段必读
- specs/stories/136320799/spec.md                      # plan 阶段起必读
- specs/stories/136320799/questions.md                 # 澄清结论（answered / resolved_by_doc）
- specs/stories/136320799/plan.md                      # tasks 阶段起必读
- specs/stories/136320799/research.md                  # tasks 阶段起必读
- specs/stories/136320799/data-model.md                # tasks 阶段起必读
- specs/stories/136320799/tasks.md                     # implement 阶段起必读

## Project background
- AGENTS.md                                            # 用途：仓库协作规则（Go 代码要求 / 工作区约束 / 语言规范）
- .golangci.yml                                        # 用途：Go 代码 lint 规则
- .claude/skills/bk-security-redlines/SKILL.md         # 用途：三大安全红线（输入校验 / 鉴权 / 加密；validate 安全维度必读）
- internal/processor/cmdb/sync_cmdb.go                 # 用途：bscp 一键同步入口与进程 diff 逻辑（SyncProcessData / diffProcesses / BuildProcessChanges）
- internal/processor/cmdb/cc_topo.go                   # 用途：CMDB 拓扑拉取与字段解析
- internal/processor/cmdb/cc_topo_types.go             # 用途：CMDB 拓扑数据结构定义
- pkg/dal/table                                        # 用途：进程表 ProcessSpec 字段定义（service_name/environment/set_name/module_name）
- bk-process-config-manager/apps/gsekit/utils/solution_maker.py  # 用途：gsekit 一键同步对标（SyncProcessSolutionMaker）
- bk-process-config-manager/apps/gsekit/process/handlers/process.py  # 用途：gsekit sync_biz_process 同步字段行为对标

## Code scope
- internal/processor/cmdb/sync_cmdb.go                 # 改：BuildProcessChanges 四字段检测+写回 / reusable 分支刷新
- internal/processor/cmdb/*_test.go                    # 改/增：四字段增量更新表驱动单测（复用 fake DAO 模式）
- docs/**                                              # 增：F-002 gsekit vs bscp CMDB 同步能力对比清单
- pkg/dal/table/process.go                             # 只读：ProcessSpec 四字段定义（不改）

## Improvement notes
无
