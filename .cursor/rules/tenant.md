# 多租户开发规范

## 基本要求
- 所有功能都要兼容多租户模式
- 如果参数中需要传递租户ID，将租户ID放在第一个参数中（如果有 kit 或 context 则放在它们后面作为第二个参数）
  - 示例: `func DoSomething(tenantID string, bizID uint32)` 
  - 示例: `func DoSomething(ctx context.Context, tenantID string, bizID uint32)`
  - 示例: `func DoSomething(kit *kit.Kit, tenantID string, bizID uint32)`

## GORM Hook 机制
- 项目已在 `internal/dal/dao/set_tenant_id.go` 中实现 GORM hook，自动处理多租户过滤
- 查询时自动添加 `WHERE tenant_id = ?` 条件
- 创建/更新时自动设置 `tenant_id` 字段
- **关键**: hook 只在 `kit.TenantID` 非空时生效，因此调用方必须确保 kit 中设置了正确的 TenantID

## 定时任务多租户处理
- 定时任务使用 `kit.New()` 创建的 kit 默认没有 TenantID
- 多租户模式下需要按租户轮询执行：
  - 进程管理相关同步（sync_cmdb、cmdb_resource_watcher）：使用 `bkuser.ListEnabledTenants()` 获取全量租户
  - 业务主机同步（sync_biz_host）：使用 `App.GetDistinctTenantIDs()` 从 app 表获取租户
- 示例模式：
```go
if cc.DataService().FeatureFlags.EnableMultiTenantMode {
    tenants, _ := bkuser.ListEnabledTenants(kt.Ctx)
    for _, tenant := range tenants {
        kt.TenantID = tenant.ID
        // 执行租户相关操作
    }
}
```

## 表结构要求
- 新增表必须包含 `tenant_id` 字段
- `tenant_id` 应作为联合主键或唯一索引的一部分
- 参考 `processes`、`process_instances` 等表的设计

## 外部 API 调用
- 调用蓝鲸组件 API 时需要在请求头中携带租户信息：`X-Bk-Tenant-Id: {tenantID}`
- 参考 `internal/components/bkuser/bkuser.go` 中的实现