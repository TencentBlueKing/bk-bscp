# Context for Story 135756883

## Stage
tasks

## Source artifacts
- specs/stories/135756883/req.md            # 所有阶段必读：原始需求 + 3 轮澄清记录
- specs/stories/135756883/spec.md           # 技术规范
- specs/stories/135756883/plan.md           # 实现计划（TDD 顺序 / 落点 / Phase）
- specs/stories/135756883/research.md       # 技术调研（落点 / inner 暴露机制 / 表达式过滤）
- specs/stories/135756883/data-model.md     # proto 契约 / DTO 定义

## Project background
- CLAUDE.md                                                   # 用途：仓库协作规则（中文、Go 规范、gofmt、golangci）
- pkg/protocol/config-server/config_service.proto             # 用途：config-server 接口契约 + inner additional_bindings 范式
- pkg/protocol/data-service/data_service.proto                # 用途：data-service 接口契约
- pkg/protocol/core/process/process.proto                     # 用途：ProcessSearchCondition / ExpressionScope / Process 定义
- cmd/config-server/service/process.go                        # 用途：ListProcess/OperateProcess handler 范式（鉴权+转发）
- cmd/data-service/service/process.go                         # 用途：data-service ListProcess handler 范式（调 dao）
- internal/dal/dao/process.go                                 # 用途：dao List + handleSearch（复用过滤入口）
- internal/dal/dao/process_expression.go                      # 用途：ExpressionScope 内存过滤实现
- pkg/dal/table/process.go                                    # 用途：ProcessSpec.InnerIP 等字段
- scripts/bk_gateway/inject_bk_gateway.py                     # 用途：inner 路由网关 authConfig 注入
- Makefile                                                    # 用途：make pb / api_docs / bkapigw_docs 生成命令

## Code scope
- pkg/protocol/config-server/config_service.proto            # 新增 ListProcessInnerIPs RPC + 消息 + inner binding
- pkg/protocol/data-service/data_service.proto               # 新增 ListProcessInnerIPs RPC + 消息
- pkg/protocol/config-server/**                              # make pb 生成物（pb.go / grpc.pb.go / gw.go）
- pkg/protocol/data-service/**                               # make pb 生成物
- cmd/config-server/service/process.go                       # 新增 config-server handler（鉴权 + 转发）
- cmd/data-service/service/process.go                        # 新增 data-service handler（校验 + 查询 + 去重）
- cmd/data-service/service/process_test.go                   # 新增单元测试（TDD）
- docs/swagger/**                                            # make api_docs / bkapigw_docs 生成物（检查 diff）

## Improvement notes
无
