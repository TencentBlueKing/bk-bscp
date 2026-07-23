# Context for Story 136178657

## Stage
tasks

## Source artifacts
- specs/stories/136178657/req.md                       # 所有阶段必读：需求原文 + 技术澄清章节
- specs/stories/136178657/spec.md                      # plan 阶段起必读：功能规范（FR/AC/关键实体）
- specs/stories/136178657/questions.md                 # 澄清结论（resolved_by_doc：TD-001/TD-002/TD-003）
- specs/stories/136178657/plan.md                      # tasks 阶段起必读：实现计划（P1~P6 + 需求覆盖映射）
- specs/stories/136178657/research.md                  # tasks 阶段起必读：技术调研
- specs/stories/136178657/data-model.md                # tasks 阶段起必读：数据模型（表达式载体字段）

## Project background
- AGENTS.md                                            # 用途：仓库协作规则（Go 代码要求 / 工作区约束 / 语言规范）
- .golangci.yml                                        # 用途：Go 代码 lint 规则
- .claude/skills/bk-security-redlines/SKILL.md         # 用途：三大安全红线（输入校验 / 鉴权 / 加密；validate 安全维度必读）
- pkg/protocol/core/process/process.proto              # 用途：OperateRange / ExpressionScope 协议契约（含新增 expression_scope 字段）
- pkg/protocol/core/task_batch/task_batch.proto        # 用途：任务批次协议契约
- pkg/protocol/core/task_batch/convert.go              # 用途：task_batch proto 与 table 结构互转
- pkg/dal/table/task_batch.go                          # 用途：OperateRange / TaskExecutionData 表结构定义（操作范围承载点）
- cmd/data-service/service/process.go                  # 用途：buildOperateRange / createTaskBatch 服务端记录逻辑（缺陷点）
- cmd/config-server/service/process.go                 # 用途：配置操作链路的操作范围记录入口
- internal/expression/scope.go                         # 用途：五段表达式 Scope 结构与生成/匹配（gsekit 对齐载体）
- ui/src/views/space/task/detail/info.vue              # 用途：前端 mergeOpRange / handleGoProcess 展示与跳转过滤
- ui/types/task.ts                                     # 用途：前端 operate_range 类型定义
- docs/reqs/进程表达式过滤.md                          # 用途：关联需求 135740005 表达式过滤能力（本需求依赖其过滤/解析语义）
- bk-process-config-manager/apps/gsekit/process/handlers/process.py  # 用途：gsekit expression_scope 记录与还原对标（scope_to_expression_scope / expression_scope_to_scope）

## Code scope
- internal/expression/list2expr.go                     # 增：List2Expr/IDsToExpr（对齐 gsekit parse_list2expr）
- internal/expression/list2expr_test.go                # 增：List2Expr 单测
- pkg/dal/table/task_batch.go                          # 改：OperateRange 五段 数组→string
- pkg/protocol/core/task_batch/task_batch.proto        # 改：OperateRange 五段 repeated→string，键名对齐
- pkg/protocol/core/task_batch/task_batch.pb.go        # 改：由 make 重新生成（勿手改）
- pkg/protocol/core/task_batch/convert.go              # 改：PbTaskBatch 五段字符串透传
- pkg/protocol/core/task_batch/convert_test.go         # 增：五段透传单测
- internal/task/executor/common/common.go              # 改：buildScopeText 改读五段字符串 + GenExpression
- cmd/data-service/service/process.go                  # 改：buildOperateRange 插件原样存/非插件 IDsToExpr
- cmd/data-service/service/config_instance.go          # 改：runConfigTask 透传请求 OperateRange，插件原样存/非插件 IDsToExpr
- cmd/data-service/db-migration/migrations/<ts>_migrate_task_batch_operate_range_to_expression.go  # 增：存量 task_data 数组→表达式迁移
- cmd/data-service/service/*_test.go                   # 增/改：buildOperateRange / 配置链路记录单测
- ui/types/task.ts                                     # 改：IOperateRange 数组→五段字符串
- ui/src/store/task.ts                                 # 改：operate_range 默认值字符串化
- ui/src/views/space/task/detail/info.vue              # 改：mergeOpRange 单一路径展示
- ui/src/views/space/process/components/filter-process.vue  # 改：跳转分支切表达式模式过滤

## Improvement notes
无
