> 此文档自动化生成，请勿修改

## 基本信息

开发注意：
- 记录返回的 `x-request-id` header,可排查用
- 返回格式默认为 `application/json`

## 接口列表

### config

| Method  | URI     | Name   | Summary |
|---------|---------|--------|---------|
| POST | /api/v1/config/biz_id/{bizId}/app_id/{appId}/release_id/{releaseId}/approval_callback | [Config_ApprovalCallback](#config-approval-callback) | itsm v4 回调接口 |
| POST | /api/v1/config/biz_id/{bizId}/app_id/{appId}/release_id/{releaseId}/approve | [Config_Approve](#config-approve) | 审批同步，其中v2版本中itsm也是复用这个接口进行回调 |
| PUT | /api/v1/config/biz/{bizId}/apps/{appId}/config_items | [Config_BatchUpsertConfigItems](#config-batch-upsert-config-items) | 批量创建或更新文件配置项 |
| POST | /api/v1/config/biz_id/{bizId}/config_template/{configTemplateId}/bind_process_instance | [Config_BindProcessInstance](#config-bind-process-instance) | 绑定配置模板与进程实例 |
| GET | /api/v1/config/biz_id/{bizId}/topo | [Config_BizTopo](#config-biz-topo) | 根据业务查询拓扑 |
| GET | /api/v1/config/biz_id/{bizId}/config_template/variable | [Config_ConfigTemplateVariable](#config-config-template-variable) | 配置模板变量 |
| POST | /api/v1/config/biz_id/{bizId}/config_template | [Config_CreateConfigTemplate](#config-create-config-template) | 创建配置模板 |
| POST | /api/v1/config/biz/{bizId}/apps/{appId}/kvs | [Config_CreateKv](#config-create-kv) | 创建键值配置项 |
| POST | /api/v1/config/create/release/release/app_id/{appId}/biz_id/{bizId} | [Config_CreateRelease](#config-create-release) | 生成版本 |
| DELETE | /api/v1/config/biz/{bizId}/apps/{appId}/kvs/{id} | [Config_DeleteKv](#config-delete-kv) | 删除键值配置项 |
| POST | /api/v1/config/biz/{bizId}/apps/{appId}/publish | [Config_GenerateReleaseAndPublish](#config-generate-release-and-publish) | 生成版本并发布 |
| GET | /api/v1/config/biz_id/{bizId}/config_template/{configTemplateId} | [Config_GetConfigTemplate](#config-get-config-template) | 获取配置模板 |
| POST | /api/v1/config/biz_id/{bizId}/config_template/list | [Config_ListConfigTemplate](#config-list-config-template) | 配置模板列表 |
| POST | /api/v1/config/biz/{bizId}/apps/{appId}/kvs/list | [Config_ListKvs](#config-list-kvs) | 获取键值配置项列表 |
| GET | /api/v1/config/biz_id/{bizId}/config_template/{configTemplateId}/preview_bind_process_instance | [Config_PreviewBindProcessInstance](#config-preview-bind-process-instance) | 预览绑定配置模板与进程实例 |
| GET | /api/v1/config/biz_id/{bizId}/process_instance/{serviceInstanceId} | [Config_ProcessInstance](#config-process-instance) | 根据服务实例查询实例进程列表 |
| GET | /api/v1/config/biz_id/{bizId}/process_template/{serviceTemplateId} | [Config_ProcessTemplate](#config-process-template) | 根据服务模板查询模板进程列表 |
| POST | /api/v1/config/update/strategy/publish/publish/release_id/{releaseId}/app_id/{appId}/biz_id/{bizId} | [Config_Publish](#config-publish) | 发布指定版本 |
| GET | /api/v1/config/biz_id/{bizId}/service_instance/{moduleId} | [Config_ServiceInstance](#config-service-instance) | 根据模块获取服务实例列表 |
| GET | /api/v1/config/biz_id/{bizId}/service_template | [Config_ServiceTemplate](#config-service-template) | 根据业务查询服务模板列表 |
| PUT | /api/v1/config/biz_id/{bizId}/config_template/{configTemplateId} | [Config_UpdateConfigTemplate](#config-update-config-template) | 编辑配置模板 |
| PUT | /api/v1/config/biz/{bizId}/apps/{appId}/kvs/{key} | [Config_UpdateKv](#config-update-kv) | 更新键值配置项 |

### healthz

| Method  | URI     | Name   | Summary |
|---------|---------|--------|---------|
| GET | /healthz | [Healthz](#healthz) | Healthz 接口 |

### 文件相关

| Method  | URI     | Name   | Summary |
|---------|---------|--------|---------|
| GET | /api/v1/biz/{biz_id}/content/download | [download_content](#download-content) | 下载文件内容 |
| GET | /api/v1/biz/{biz_id}/content/metadata | [get_content_metadata](#get-content-metadata) | 获取文件内容元数据 |
| PUT | /api/v1/biz/{biz_id}/content/upload | [upload_content](#upload-content) | 上传文件内容 |

## 接口详情

### <span id="config-approval-callback"></span> itsm v4 回调接口 (*Config_ApprovalCallback*)

```
POST /api/v1/config/biz_id/{bizId}/app_id/{appId}/release_id/{releaseId}/approval_callback
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| appId | int64 (formatted integer) | ✓ | 服务ID |
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| releaseId | int64 (formatted integer) | ✓ | 服务版本ID |
| callbackToken | string |  |  |
| ticket | [PbreleaseTicket](#pbrelease-ticket) |  |  |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
POST /api/v1/config/biz_id/{bizId}/app_id/{appId}/release_id/{releaseId}/approval_callback HTTP/1.1
Content-Type: application/json

{
  "callbackToken": "",
  "ticket": {
    "approveResult": false,
    "callbackResult": {},
    "createdAt": "",
    "currentProcessors": [
      {}
    ],
    "currentSteps": [
      {
        "activityKey": "",
        "name": "",
        "ticketId": ""
      }
    ],
    "endAt": "",
    "formData": {
      "ticketTitle": ""
    },
    "frontendUrl": "",
    "id": "",
    "portalId": "",
    "serviceId": "",
    "sn": "",
    "status": "",
    "statusDisplay": "",
    "systemId": "",
    "title": "",
    "updatedAt": "",
    "workflowId": ""
  }
}
```

#### 输出示例

```json
{}
```

### <span id="config-approve"></span> 审批同步，其中v2版本中itsm也是复用这个接口进行回调 (*Config_Approve*)

```
POST /api/v1/config/biz_id/{bizId}/app_id/{appId}/release_id/{releaseId}/approve
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| appId | int64 (formatted integer) | ✓ |  |
| bizId | int64 (formatted integer) | ✓ |  |
| releaseId | int64 (formatted integer) | ✓ |  |
| publishStatus | string |  |  |
| reason | string |  |  |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
POST /api/v1/config/biz_id/{bizId}/app_id/{appId}/release_id/{releaseId}/approve HTTP/1.1
Content-Type: application/json

{
  "publishStatus": "",
  "reason": ""
}
```

#### 输出示例

```json
{}
```

### <span id="config-batch-upsert-config-items"></span> 批量创建或更新文件配置项 (*Config_BatchUpsertConfigItems*)

```
PUT /api/v1/config/biz/{bizId}/apps/{appId}/config_items
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| appId | int64 (formatted integer) | ✓ | 服务ID |
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| bindings | \[\][PbcsBatchUpsertConfigItemsReqTemplateBinding](#pbcs-batch-upsert-config-items-req-template-binding) |  |  |
| items | \[\][PbcsBatchUpsertConfigItemsReqConfigItem](#pbcs-batch-upsert-config-items-req-config-item) |  |  |
| replaceAll | boolean |  | 是否替换全部：如果为true会覆盖已有的文件，不存在的则删除 |
| variables | \[\][PbtvTemplateVariableSpec](#pbtv-template-variable-spec) |  |  |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
PUT /api/v1/config/biz/{bizId}/apps/{appId}/config_items HTTP/1.1
Content-Type: application/json

{
  "bindings": [
    {
      "templateBinding": {
        "templateRevisions": [
          {
            "isLatest": false,
            "templateId": 0,
            "templateRevisionId": 0
          }
        ],
        "templateSetId": 0
      },
      "templateSpaceId": 0
    }
  ],
  "items": [
    {
      "byteSize": "",
      "charset": "",
      "fileMode": "",
      "fileType": "",
      "md5": "",
      "memo": "",
      "name": "",
      "path": "",
      "privilege": "",
      "sign": "",
      "user": "",
      "userGroup": ""
    }
  ],
  "replaceAll": false,
  "variables": [
    {
      "defaultVal": "",
      "memo": "",
      "name": "",
      "type": ""
    }
  ]
}
```

#### 输出示例

```json
{}
```

### <span id="config-bind-process-instance"></span> 绑定配置模板与进程实例 (*Config_BindProcessInstance*)

```
POST /api/v1/config/biz_id/{bizId}/config_template/{configTemplateId}/bind_process_instance
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| configTemplateId | int64 (formatted integer) | ✓ | 配置模版ID |
| ccProcessIds | []int64 (formatted integer) |  | 配置模版关联的CC进程实例ID列表 |
| ccTemplateProcessIds | []int64 (formatted integer) |  | 配置模版关联的CC模板进程ID列表 |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
POST /api/v1/config/biz_id/{bizId}/config_template/{configTemplateId}/bind_process_instance HTTP/1.1
Content-Type: application/json

{
  "ccProcessIds": [
    {}
  ],
  "ccTemplateProcessIds": [
    {}
  ]
}
```

#### 输出示例

```json
{}
```

### <span id="config-biz-topo"></span> 根据业务查询拓扑 (*Config_BizTopo*)

```
GET /api/v1/config/biz_id/{bizId}/topo
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| bizId | int64 (formatted integer) | ✓ | 业务ID |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
GET /api/v1/config/biz_id/{bizId}/topo HTTP/1.1
Content-Type: application/json


```

#### 输出示例

```json
{}
```

### <span id="config-config-template-variable"></span> 配置模板变量 (*Config_ConfigTemplateVariable*)

```
GET /api/v1/config/biz_id/{bizId}/config_template/variable
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| bizId | int64 (formatted integer) | ✓ | 业务ID |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
GET /api/v1/config/biz_id/{bizId}/config_template/variable HTTP/1.1
Content-Type: application/json


```

#### 输出示例

```json
{}
```

### <span id="config-create-config-template"></span> 创建配置模板 (*Config_CreateConfigTemplate*)

```
POST /api/v1/config/biz_id/{bizId}/config_template
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| byteSize | uint64 (formatted string) |  | 文件大小 |
| charset | string |  | 文件编码 |
| fileMode | string |  | 文件模式 |
| fileName | string |  | 文件名 |
| filePath | string |  | 文件路径 |
| highlightStyle | string |  | 高亮风格 |
| md5 | string |  | 文件md5 |
| memo | string |  | 配置模版描述 |
| name | string |  | 配置模版名称 |
| privilege | string |  | 文件权限 |
| revisionName | string |  | 模板文件版本号 |
| sign | string |  | 文件sha256 |
| templateSpaceId | int64 (formatted integer) |  | 模板空间ID |
| user | string |  | 用户权限名 |
| userGroup | string |  | 用户组权限名 |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
POST /api/v1/config/biz_id/{bizId}/config_template HTTP/1.1
Content-Type: application/json

{
  "byteSize": "",
  "charset": "",
  "fileMode": "",
  "fileName": "",
  "filePath": "",
  "highlightStyle": "",
  "md5": "",
  "memo": "",
  "name": "",
  "privilege": "",
  "revisionName": "",
  "sign": "",
  "templateSpaceId": 0,
  "user": "",
  "userGroup": ""
}
```

#### 输出示例

```json
{}
```

### <span id="config-create-kv"></span> 创建键值配置项 (*Config_CreateKv*)

```
POST /api/v1/config/biz/{bizId}/apps/{appId}/kvs
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| appId | int64 (formatted integer) | ✓ | 服务ID |
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| certificateExpirationDate | string |  | 证书过期时间 |
| key | string | ✓ | 配置项名 |
| kvType | string | ✓ | 键值类型：(any、string、number、text、json、yaml、xml、secret) |
| memo | string |  | 描述 |
| secretHidden | boolean |  | 是否隐藏值：是=true，否=false |
| secretType | string |  | 密钥类型：(password、、certificate、secret_key、token、custom)，如果kv_type=secret必填项 |
| value | string | ✓ | 配置项值 |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
POST /api/v1/config/biz/{bizId}/apps/{appId}/kvs HTTP/1.1
Content-Type: application/json

{
  "certificateExpirationDate": "",
  "key": "",
  "kvType": "",
  "memo": "",
  "secretHidden": false,
  "secretType": "",
  "value": ""
}
```

#### 输出示例

```json
{}
```

### <span id="config-create-release"></span> 生成版本 (*Config_CreateRelease*)

```
POST /api/v1/config/create/release/release/app_id/{appId}/biz_id/{bizId}
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| appId | int64 (formatted integer) | ✓ | 服务ID |
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| memo | string |  | 版本描述 |
| name | string |  | 版本名称 |
| variables | \[\][PbtvTemplateVariableSpec](#pbtv-template-variable-spec) |  |  |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
POST /api/v1/config/create/release/release/app_id/{appId}/biz_id/{bizId} HTTP/1.1
Content-Type: application/json

{
  "memo": "",
  "name": "",
  "variables": [
    {
      "defaultVal": "",
      "memo": "",
      "name": "",
      "type": ""
    }
  ]
}
```

#### 输出示例

```json
{}
```

### <span id="config-delete-kv"></span> 删除键值配置项 (*Config_DeleteKv*)

```
DELETE /api/v1/config/biz/{bizId}/apps/{appId}/kvs/{id}
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| appId | int64 (formatted integer) | ✓ | 服务ID |
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| id | int64 (formatted integer) | ✓ | 键值配置项ID |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
DELETE /api/v1/config/biz/{bizId}/apps/{appId}/kvs/{id} HTTP/1.1
Content-Type: application/json


```

#### 输出示例

```json
{}
```

### <span id="config-generate-release-and-publish"></span> 生成版本并发布 (*Config_GenerateReleaseAndPublish*)

```
POST /api/v1/config/biz/{bizId}/apps/{appId}/publish
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| appId | int64 (formatted integer) | ✓ | 服务ID |
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| all | boolean |  | 全部实例上线：是=true，否=false |
| grayPublishMode | string |  | 灰度发布模式，仅在 all 为 false 时有效，枚举值：publish_by_labels,publish_by_groups |
| groupName | string |  | 在 gray_publish_mode 为 publish_by_labels 时生效，用于根据 labels 生成一个分组时对其命名，如果有服务有可用的（绑定了服务）同 labels 的分组存在，则复用旧的分组，不会新创建分组 |
| groups | []string |  | 分组上线：分组ID，如果有值那么all必须是false |
| labels | \[\][interface{}](#interface) |  | 要发布的标签列表，仅在 gray_publish_mode 为 publish_by_labels 时生效 |
| releaseMemo | string |  | 版本描述 |
| releaseName | string |  | 服务版本名 |
| variables | \[\][PbtvTemplateVariableSpec](#pbtv-template-variable-spec) |  |  |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
POST /api/v1/config/biz/{bizId}/apps/{appId}/publish HTTP/1.1
Content-Type: application/json

{
  "all": false,
  "grayPublishMode": "",
  "groupName": "",
  "groups": [
    {}
  ],
  "labels": [
    {}
  ],
  "releaseMemo": "",
  "releaseName": "",
  "variables": [
    {
      "defaultVal": "",
      "memo": "",
      "name": "",
      "type": ""
    }
  ]
}
```

#### 输出示例

```json
{}
```

### <span id="config-get-config-template"></span> 获取配置模板 (*Config_GetConfigTemplate*)

```
GET /api/v1/config/biz_id/{bizId}/config_template/{configTemplateId}
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| configTemplateId | int64 (formatted integer) | ✓ | 配置模版ID |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
GET /api/v1/config/biz_id/{bizId}/config_template/{configTemplateId} HTTP/1.1
Content-Type: application/json


```

#### 输出示例

```json
{}
```

### <span id="config-list-config-template"></span> 配置模板列表 (*Config_ListConfigTemplate*)

```
POST /api/v1/config/biz_id/{bizId}/config_template/list
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| all | boolean |  | 是否获取所有 |
| limit | int64 (formatted integer) |  | 每页条数 |
| search | [PbctTemplateSearchCond](#pbct-template-search-cond) |  |  |
| start | int64 (formatted integer) |  | 当前页码 |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
POST /api/v1/config/biz_id/{bizId}/config_template/list HTTP/1.1
Content-Type: application/json

{
  "all": false,
  "limit": 0,
  "search": {
    "fileName": "",
    "reviser": "",
    "templateName": ""
  },
  "start": 0
}
```

#### 输出示例

```json
{}
```

### <span id="config-list-kvs"></span> 获取键值配置项列表 (*Config_ListKvs*)

```
POST /api/v1/config/biz/{bizId}/apps/{appId}/kvs/list
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| appId | int64 (formatted integer) | ✓ | 服务ID |
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| all | boolean |  | 是否获取所有 |
| key | []string |  | 查询特定的配置项名 |
| kvType | []string |  | 键值类型：(any、string、number、text、json、yaml、xml、secret) |
| limit | int64 (formatted integer) |  | 每页条数 |
| order | string |  | 排序类型：desc |
| search | [interface{}](#interface) |  | 搜索的值 |
| sort | string |  | 排序的值，例如：key |
| start | int64 (formatted integer) |  | 当前页码 |
| status | []string |  | 键值配置项状态：(ADD、DELETE、REVISE、UNCHANGE) |
| topIds | []int64 (formatted integer) |  | 需要置顶ID |
| withStatus | boolean |  | 暂时未用到 |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
POST /api/v1/config/biz/{bizId}/apps/{appId}/kvs/list HTTP/1.1
Content-Type: application/json

{
  "all": false,
  "key": [
    {}
  ],
  "kvType": [
    {}
  ],
  "limit": 0,
  "order": "",
  "search": {},
  "sort": "",
  "start": 0,
  "status": [
    {}
  ],
  "topIds": [
    {}
  ],
  "withStatus": false
}
```

#### 输出示例

```json
{}
```

### <span id="config-preview-bind-process-instance"></span> 预览绑定配置模板与进程实例 (*Config_PreviewBindProcessInstance*)

```
GET /api/v1/config/biz_id/{bizId}/config_template/{configTemplateId}/preview_bind_process_instance
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| configTemplateId | int64 (formatted integer) | ✓ | 配置模版ID |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
GET /api/v1/config/biz_id/{bizId}/config_template/{configTemplateId}/preview_bind_process_instance HTTP/1.1
Content-Type: application/json


```

#### 输出示例

```json
{}
```

### <span id="config-process-instance"></span> 根据服务实例查询实例进程列表 (*Config_ProcessInstance*)

```
GET /api/v1/config/biz_id/{bizId}/process_instance/{serviceInstanceId}
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| serviceInstanceId | int64 (formatted integer) | ✓ | 服务实例ID |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
GET /api/v1/config/biz_id/{bizId}/process_instance/{serviceInstanceId} HTTP/1.1
Content-Type: application/json


```

#### 输出示例

```json
{}
```

### <span id="config-process-template"></span> 根据服务模板查询模板进程列表 (*Config_ProcessTemplate*)

```
GET /api/v1/config/biz_id/{bizId}/process_template/{serviceTemplateId}
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| serviceTemplateId | int64 (formatted integer) | ✓ | 服务模板ID |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
GET /api/v1/config/biz_id/{bizId}/process_template/{serviceTemplateId} HTTP/1.1
Content-Type: application/json


```

#### 输出示例

```json
{}
```

### <span id="config-publish"></span> 发布指定版本 (*Config_Publish*)

```
POST /api/v1/config/update/strategy/publish/publish/release_id/{releaseId}/app_id/{appId}/biz_id/{bizId}
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| appId | int64 (formatted integer) | ✓ | 服务ID |
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| releaseId | int64 (formatted integer) | ✓ | 服务版本ID |
| all | boolean |  | 全部实例上线：是=true，否=false |
| default | boolean |  |  |
| grayPublishMode | string |  | 灰度发布模式，仅在 all 为 false 时有效，枚举值：publish_by_labels,publish_by_groups |
| groupName | string |  | 在 gray_publish_mode 为 publish_by_labels 时生效，用于根据 labels 生成一个分组时对其命名，如果有服务有可用的（绑定了服务）同 labels 的分组存在，则复用旧的分组，不会新创建分组 |
| groups | []int64 (formatted integer) |  | 分组上线：分组ID，如果有值那么all必须是false |
| labels | \[\][interface{}](#interface) |  | 要发布的标签列表，仅在 gray_publish_mode 为 publish_by_labels 时生效 |
| memo | string |  | 上线说明 |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
POST /api/v1/config/update/strategy/publish/publish/release_id/{releaseId}/app_id/{appId}/biz_id/{bizId} HTTP/1.1
Content-Type: application/json

{
  "all": false,
  "default": false,
  "grayPublishMode": "",
  "groupName": "",
  "groups": [
    {}
  ],
  "labels": [
    {}
  ],
  "memo": ""
}
```

#### 输出示例

```json
{}
```

### <span id="config-service-instance"></span> 根据模块获取服务实例列表 (*Config_ServiceInstance*)

```
GET /api/v1/config/biz_id/{bizId}/service_instance/{moduleId}
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| moduleId | int64 (formatted integer) | ✓ | 模块ID |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
GET /api/v1/config/biz_id/{bizId}/service_instance/{moduleId} HTTP/1.1
Content-Type: application/json


```

#### 输出示例

```json
{}
```

### <span id="config-service-template"></span> 根据业务查询服务模板列表 (*Config_ServiceTemplate*)

```
GET /api/v1/config/biz_id/{bizId}/service_template
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| bizId | int64 (formatted integer) | ✓ | 业务ID |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
GET /api/v1/config/biz_id/{bizId}/service_template HTTP/1.1
Content-Type: application/json


```

#### 输出示例

```json
{}
```

### <span id="config-update-config-template"></span> 编辑配置模板 (*Config_UpdateConfigTemplate*)

```
PUT /api/v1/config/biz_id/{bizId}/config_template/{configTemplateId}
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| configTemplateId | int64 (formatted integer) | ✓ | 配置模版ID |
| byteSize | uint64 (formatted string) |  | 文件大小 |
| charset | string |  | 文件编码 |
| fileMode | string |  | 文件模式 |
| highlightStyle | string |  | 高亮风格 |
| md5 | string |  | 文件md5 |
| memo | string |  | 配置模版描述 |
| name | string |  | 配置模版名称 |
| privilege | string |  | 文件权限 |
| revisionName | string |  | 模板文件版本号 |
| sign | string |  | 文件sha256 |
| user | string |  | 用户权限名 |
| userGroup | string |  | 用户组权限名 |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
PUT /api/v1/config/biz_id/{bizId}/config_template/{configTemplateId} HTTP/1.1
Content-Type: application/json

{
  "byteSize": "",
  "charset": "",
  "fileMode": "",
  "highlightStyle": "",
  "md5": "",
  "memo": "",
  "name": "",
  "privilege": "",
  "revisionName": "",
  "sign": "",
  "user": "",
  "userGroup": ""
}
```

#### 输出示例

```json
{}
```

### <span id="config-update-kv"></span> 更新键值配置项 (*Config_UpdateKv*)

```
PUT /api/v1/config/biz/{bizId}/apps/{appId}/kvs/{key}
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| appId | int64 (formatted integer) | ✓ | 服务ID |
| bizId | int64 (formatted integer) | ✓ | 业务ID |
| key | string | ✓ | 配置项名 |
| memo | string |  | 描述 |
| secretHidden | boolean |  | 是否隐藏值：是=true，否=false |
| secretType | string |  | 密钥类型：(password、、certificate、secret_key、token、custom)，如果kv_type=secret必填项 |
| value | string | ✓ | 配置项值 |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
PUT /api/v1/config/biz/{bizId}/apps/{appId}/kvs/{key} HTTP/1.1
Content-Type: application/json

{
  "memo": "",
  "secretHidden": false,
  "secretType": "",
  "value": ""
}
```

#### 输出示例

```json
{}
```

### <span id="healthz"></span> Healthz 接口 (*Healthz*)

```
GET /healthz
```

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|

#### 输入示例

```bash
GET /healthz HTTP/1.1
Content-Type: application/json


```

#### 输出示例

```json
{}
```

### <span id="download-content"></span> 下载文件内容 (*download_content*)

```
GET /api/v1/biz/{biz_id}/content/download
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| biz_id | integer | ✓ | 业务ID |
| X-Bkapi-File-Content-Id | string | ✓ | 上传文件内容的SHA256值 |
| X-Bscp-App-Id | integer |  | 如果是应用配置项，则设置该应用ID |
| X-Bscp-Template-Space-Id | integer |  | 如果是模版配置项，则设置该模版空间ID |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|
| code | integer | 返回码 |
| message | string | 返回消息 |
| code | integer | 返回码 |
| message | string | 返回消息 |
| data | object | 返回body |
| data.byte_size | integer |  |
| data.md5 | string |  |
| data.sha256 | string |  |



#### 输入示例

```bash
GET /api/v1/biz/{biz_id}/content/download HTTP/1.1
Content-Type: application/json


```

#### 输出示例

```json
{
  "data": {
    "byte_size": 0,
    "md5": "",
    "sha256": ""
  }
}
```

### <span id="get-content-metadata"></span> 获取文件内容元数据 (*get_content_metadata*)

```
GET /api/v1/biz/{biz_id}/content/metadata
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| biz_id | integer | ✓ | 业务ID |
| X-Bkapi-File-Content-Id | string | ✓ | 上传文件内容的SHA256值 |
| app-id | integer | ✓ | 如果是应用配置项，则设置该应用ID |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|
| code | integer | 返回码 |
| message | string | 返回消息 |
| code | integer | 返回码 |
| message | string | 返回消息 |
| data | object | 返回body |
| data.byte_size | integer |  |
| data.md5 | string |  |
| data.sha256 | string |  |



#### 输入示例

```bash
GET /api/v1/biz/{biz_id}/content/metadata HTTP/1.1
Content-Type: application/json


```

#### 输出示例

```json
{
  "data": {
    "byte_size": 0,
    "md5": "",
    "sha256": ""
  }
}
```

### <span id="upload-content"></span> 上传文件内容 (*upload_content*)

```
PUT /api/v1/biz/{biz_id}/content/upload
```

#### 输入参数

| 参数名称 | 类型 | 是否必填 | 描述 |
|------|--------|------|---------|
| biz_id | integer | ✓ | 业务ID |
| X-Bkapi-File-Content-Id | string | ✓ | 上传文件内容的SHA256值 |
| X-Bscp-App-Id | integer |  | 如果是应用配置项，则设置该应用ID |
| X-Bscp-Template-Space-Id | integer |  | 如果是模版配置项，则设置该模版空间ID |

#### 输出参数

| 参数名称 | 类型 | 描述 |
|------|--------|---------|
| code | integer | 返回码 |
| message | string | 返回消息 |
| code | integer | 返回码 |
| message | string | 返回消息 |
| data | object | 返回body |
| data.byte_size | integer |  |
| data.md5 | string |  |
| data.sha256 | string |  |



#### 输入示例

```bash
PUT /api/v1/biz/{biz_id}/content/upload HTTP/1.1
Content-Type: application/json


```

#### 输出示例

```json
{
  "data": {
    "byte_size": 0,
    "md5": "",
    "sha256": ""
  }
}
```

## Models

### <span id="config-approval-callback-body"></span> ConfigApprovalCallbackBody


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| callbackToken | string| `string` |  | |  |  |
| ticket | [PbreleaseTicket](#pbrelease-ticket)| `PbreleaseTicket` |  | |  |  |



### <span id="config-approve-body"></span> ConfigApproveBody


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| publishStatus | string| `string` |  | |  |  |
| reason | string| `string` |  | |  |  |



### <span id="config-batch-upsert-config-items-body"></span> ConfigBatchUpsertConfigItemsBody


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| bindings | \[\][PbcsBatchUpsertConfigItemsReqTemplateBinding](#pbcs-batch-upsert-config-items-req-template-binding)| `[]*PbcsBatchUpsertConfigItemsReqTemplateBinding` |  | |  |  |
| items | \[\][PbcsBatchUpsertConfigItemsReqConfigItem](#pbcs-batch-upsert-config-items-req-config-item)| `[]*PbcsBatchUpsertConfigItemsReqConfigItem` |  | |  |  |
| replaceAll | boolean| `bool` |  | | 是否替换全部：如果为true会覆盖已有的文件，不存在的则删除 |  |
| variables | \[\][PbtvTemplateVariableSpec](#pbtv-template-variable-spec)| `[]*PbtvTemplateVariableSpec` |  | |  |  |



### <span id="config-create-config-template-body"></span> ConfigCreateConfigTemplateBody


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| byteSize | uint64 (formatted string)| `string` |  | | 文件大小 |  |
| charset | string| `string` |  | | 文件编码 |  |
| fileMode | string| `string` |  | `"unix"`| 文件模式 |  |
| fileName | string| `string` |  | | 文件名 |  |
| filePath | string| `string` |  | | 文件路径 |  |
| highlightStyle | string| `string` |  | | 高亮风格 |  |
| md5 | string| `string` |  | | 文件md5 |  |
| memo | string| `string` |  | | 配置模版描述 |  |
| name | string| `string` |  | | 配置模版名称 |  |
| privilege | string| `string` |  | | 文件权限 |  |
| revisionName | string| `string` |  | | 模板文件版本号 |  |
| sign | string| `string` |  | | 文件sha256 |  |
| templateSpaceId | int64 (formatted integer)| `int64` |  | | 模板空间ID |  |
| user | string| `string` |  | | 用户权限名 |  |
| userGroup | string| `string` |  | | 用户组权限名 |  |



### <span id="config-create-kv-body"></span> ConfigCreateKvBody


> 请求参数
  





**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| certificateExpirationDate | string| `string` |  | | 证书过期时间 |  |
| key | string| `string` | ✓ | | 配置项名 |  |
| kvType | string| `string` | ✓ | | 键值类型：(any、string、number、text、json、yaml、xml、secret) |  |
| memo | string| `string` |  | | 描述 |  |
| secretHidden | boolean| `bool` |  | | 是否隐藏值：是=true，否=false |  |
| secretType | string| `string` |  | | 密钥类型：(password、、certificate、secret_key、token、custom)，如果kv_type=secret必填项 |  |
| value | string| `string` | ✓ | | 配置项值 |  |



### <span id="config-create-release-body"></span> ConfigCreateReleaseBody


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| memo | string| `string` |  | | 版本描述 |  |
| name | string| `string` |  | | 版本名称 |  |
| variables | \[\][PbtvTemplateVariableSpec](#pbtv-template-variable-spec)| `[]*PbtvTemplateVariableSpec` |  | |  |  |



### <span id="config-generate-release-and-publish-body"></span> ConfigGenerateReleaseAndPublishBody


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| all | boolean| `bool` |  | | 全部实例上线：是=true，否=false |  |
| grayPublishMode | string| `string` |  | | 灰度发布模式，仅在 all 为 false 时有效，枚举值：publish_by_labels,publish_by_groups |  |
| groupName | string| `string` |  | | 在 gray_publish_mode 为 publish_by_labels 时生效，用于根据 labels 生成一个分组时对其命名，如果有服务有可用的（绑定了服务）同 labels 的分组存在，则复用旧的分组，不会新创建分组 |  |
| groups | []string| `[]string` |  | | 分组上线：分组ID，如果有值那么all必须是false |  |
| labels | \[\][interface{}](#interface)| `[]interface{}` |  | | 要发布的标签列表，仅在 gray_publish_mode 为 publish_by_labels 时生效 |  |
| releaseMemo | string| `string` |  | | 版本描述 |  |
| releaseName | string| `string` |  | | 服务版本名 |  |
| variables | \[\][PbtvTemplateVariableSpec](#pbtv-template-variable-spec)| `[]*PbtvTemplateVariableSpec` |  | |  |  |



### <span id="config-list-config-template-body"></span> ConfigListConfigTemplateBody


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| all | boolean| `bool` |  | | 是否获取所有 |  |
| limit | int64 (formatted integer)| `int64` |  | | 每页条数 |  |
| search | [PbctTemplateSearchCond](#pbct-template-search-cond)| `PbctTemplateSearchCond` |  | |  |  |
| start | int64 (formatted integer)| `int64` |  | | 当前页码 |  |



### <span id="config-list-kvs-body"></span> ConfigListKvsBody


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| all | boolean| `bool` |  | | 是否获取所有 |  |
| key | []string| `[]string` |  | | 查询特定的配置项名 |  |
| kvType | []string| `[]string` |  | | 键值类型：(any、string、number、text、json、yaml、xml、secret) |  |
| limit | int64 (formatted integer)| `int64` |  | | 每页条数 |  |
| order | string| `string` |  | | 排序类型：desc |  |
| search | [interface{}](#interface)| `interface{}` |  | | 搜索的值 |  |
| sort | string| `string` |  | | 排序的值，例如：key |  |
| start | int64 (formatted integer)| `int64` |  | | 当前页码 |  |
| status | []string| `[]string` |  | | 键值配置项状态：(ADD、DELETE、REVISE、UNCHANGE) |  |
| topIds | []int64 (formatted integer)| `[]int64` |  | | 需要置顶ID |  |
| withStatus | boolean| `bool` |  | | 暂时未用到 |  |



### <span id="config-publish-body"></span> ConfigPublishBody


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| all | boolean| `bool` |  | | 全部实例上线：是=true，否=false |  |
| default | boolean| `bool` |  | |  |  |
| grayPublishMode | string| `string` |  | | 灰度发布模式，仅在 all 为 false 时有效，枚举值：publish_by_labels,publish_by_groups |  |
| groupName | string| `string` |  | | 在 gray_publish_mode 为 publish_by_labels 时生效，用于根据 labels 生成一个分组时对其命名，如果有服务有可用的（绑定了服务）同 labels 的分组存在，则复用旧的分组，不会新创建分组 |  |
| groups | []int64 (formatted integer)| `[]int64` |  | | 分组上线：分组ID，如果有值那么all必须是false |  |
| labels | \[\][interface{}](#interface)| `[]interface{}` |  | | 要发布的标签列表，仅在 gray_publish_mode 为 publish_by_labels 时生效 |  |
| memo | string| `string` |  | | 上线说明 |  |



### <span id="config-update-config-template-body"></span> ConfigUpdateConfigTemplateBody


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| byteSize | uint64 (formatted string)| `string` |  | | 文件大小 |  |
| charset | string| `string` |  | | 文件编码 |  |
| fileMode | string| `string` |  | `"unix"`| 文件模式 |  |
| highlightStyle | string| `string` |  | | 高亮风格 |  |
| md5 | string| `string` |  | | 文件md5 |  |
| memo | string| `string` |  | | 配置模版描述 |  |
| name | string| `string` |  | | 配置模版名称 |  |
| privilege | string| `string` |  | | 文件权限 |  |
| revisionName | string| `string` |  | | 模板文件版本号 |  |
| sign | string| `string` |  | | 文件sha256 |  |
| user | string| `string` |  | | 用户权限名 |  |
| userGroup | string| `string` |  | | 用户组权限名 |  |



### <span id="config-update-kv-body"></span> ConfigUpdateKvBody


> 请求参数
  





**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| memo | string| `string` |  | | 描述 |  |
| secretHidden | boolean| `bool` |  | | 是否隐藏值：是=true，否=false |  |
| secretType | string| `string` |  | | 密钥类型：(password、、certificate、secret_key、token、custom)，如果kv_type=secret必填项 |  |
| value | string| `string` | ✓ | | 配置项值 |  |



### <span id="pbatb-template-binding"></span> pbatbTemplateBinding


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| templateRevisions | \[\][PbatbTemplateRevisionBinding](#pbatb-template-revision-binding)| `[]*PbatbTemplateRevisionBinding` |  | |  |  |
| templateSetId | int64 (formatted integer)| `int64` |  | | 模板套餐ID |  |



### <span id="pbatb-template-revision-binding"></span> pbatbTemplateRevisionBinding


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| isLatest | boolean| `bool` |  | | 是否是最新：模板文件版本ID在该模板文件中是最新的一个版本 |  |
| templateId | int64 (formatted integer)| `int64` |  | | 模板文件ID |  |
| templateRevisionId | int64 (formatted integer)| `int64` |  | | 模板文件版本ID |  |



### <span id="pbbase-revision"></span> pbbaseRevision


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| createAt | string| `string` |  | | 创建时间 |  |
| creator | string| `string` |  | | 创建人 |  |
| reviser | string| `string` |  | | 更新人 |  |
| updateAt | string| `string` |  | | 更新时间 |  |



### <span id="pbcontent-content-spec"></span> pbcontentContentSpec


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| byteSize | uint64 (formatted string)| `string` |  | | 文件大小 |  |
| md5 | string| `string` |  | | 文件md5 |  |
| signature | string| `string` |  | | 文件sha256 |  |



### <span id="pbcs-approval-callback-resp"></span> pbcsApprovalCallbackResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| message | string| `string` |  | | 消息 |  |
| result | boolean| `bool` |  | | 结果 |  |



### <span id="pbcs-approve-resp"></span> pbcsApproveResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| code | int32 (formatted integer)| `int32` |  | |  |  |
| haveCredentials | boolean| `bool` |  | |  |  |
| havePull | boolean| `bool` |  | |  |  |
| message | string| `string` |  | |  |  |



### <span id="pbcs-batch-upsert-config-items-req-config-item"></span> pbcsBatchUpsertConfigItemsReqConfigItem


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| byteSize | uint64 (formatted string)| `string` |  | | 文件大小 |  |
| charset | string| `string` |  | | 文件编码 |  |
| fileMode | string| `string` |  | `"unix"`| 文件模式 |  |
| fileType | string| `string` |  | | 配置文件格式：文本文件=file, 二进制文件=binary |  |
| md5 | string| `string` |  | | 文件md5 |  |
| memo | string| `string` |  | | 文件描述 |  |
| name | string| `string` |  | | 文件名 |  |
| path | string| `string` |  | | 文件路径 |  |
| privilege | string| `string` |  | | 文件权限 |  |
| sign | string| `string` |  | | 文件sha256 |  |
| user | string| `string` |  | | 用户权限名 |  |
| userGroup | string| `string` |  | | 用户组权限名 |  |



### <span id="pbcs-batch-upsert-config-items-req-template-binding"></span> pbcsBatchUpsertConfigItemsReqTemplateBinding


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| templateBinding | [PbatbTemplateBinding](#pbatb-template-binding)| `PbatbTemplateBinding` |  | |  |  |
| templateSpaceId | int64 (formatted integer)| `int64` |  | | 模板空间ID |  |



### <span id="pbcs-batch-upsert-config-items-resp"></span> pbcsBatchUpsertConfigItemsResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| ids | []int64 (formatted integer)| `[]int64` |  | | 文件配置项ID |  |



### <span id="pbcs-bind-process-instance-resp"></span> pbcsBindProcessInstanceResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| id | int64 (formatted integer)| `int64` |  | | 配置模版ID |  |



### <span id="pbcs-biz-topo-resp"></span> pbcsBizTopoResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| bizTopoNodes | \[\][PbctBizTopoNode](#pbct-biz-topo-node)| `[]*PbctBizTopoNode` |  | |  |  |



### <span id="pbcs-config-bind-process-instance-body"></span> pbcsConfigBindProcessInstanceBody


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| ccProcessIds | []int64 (formatted integer)| `[]int64` |  | | 配置模版关联的CC进程实例ID列表 |  |
| ccTemplateProcessIds | []int64 (formatted integer)| `[]int64` |  | | 配置模版关联的CC模板进程ID列表 |  |



### <span id="pbcs-config-template-variable-resp"></span> pbcsConfigTemplateVariableResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| configTemplateVariables | \[\][PbctConfigTemplateVariable](#pbct-config-template-variable)| `[]*PbctConfigTemplateVariable` |  | |  |  |



### <span id="pbcs-create-config-template-resp"></span> pbcsCreateConfigTemplateResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| id | int64 (formatted integer)| `int64` |  | | 配置模版ID |  |



### <span id="pbcs-create-kv-resp"></span> pbcsCreateKvResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| id | int64 (formatted integer)| `int64` |  | | 键值配置项ID |  |



### <span id="pbcs-create-release-resp"></span> pbcsCreateReleaseResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| id | int64 (formatted integer)| `int64` |  | | 生成配置服务版本ID |  |



### <span id="pbcs-delete-kv-resp"></span> pbcsDeleteKvResp


  

[interface{}](#interface)

### <span id="pbcs-get-config-template-resp"></span> pbcsGetConfigTemplateResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| bindTemplate | [PbctBindTemplate](#pbct-bind-template)| `PbctBindTemplate` |  | |  |  |



### <span id="pbcs-list-config-template-resp"></span> pbcsListConfigTemplateResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| count | int64 (formatted integer)| `int64` |  | | 总数 |  |
| details | \[\][PbctConfigTemplate](#pbct-config-template)| `[]*PbctConfigTemplate` |  | |  |  |
| templateSet | [PbcsListConfigTemplateRespItem](#pbcs-list-config-template-resp-item)| `PbcsListConfigTemplateRespItem` |  | |  |  |
| templateSpace | [PbcsListConfigTemplateRespItem](#pbcs-list-config-template-resp-item)| `PbcsListConfigTemplateRespItem` |  | |  |  |



### <span id="pbcs-list-config-template-resp-item"></span> pbcsListConfigTemplateRespItem


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| id | int64 (formatted integer)| `int64` |  | |  |  |
| name | string| `string` |  | |  |  |



### <span id="pbcs-list-kvs-resp"></span> pbcsListKvsResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| count | int64 (formatted integer)| `int64` |  | | 总数 |  |
| details | \[\][PbkvKv](#pbkv-kv)| `[]*PbkvKv` |  | |  |  |
| exclusionCount | int64 (formatted integer)| `int64` |  | | 排除删除后的数量 |  |
| isCertExpired | boolean| `bool` |  | | 是否有证书过期：是=true，否=false |  |



### <span id="pbcs-preview-bind-process-instance-resp"></span> pbcsPreviewBindProcessInstanceResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| instanceProcesses | \[\][PbctBindProcessInstance](#pbct-bind-process-instance)| `[]*PbctBindProcessInstance` |  | |  |  |
| templateProcesses | \[\][PbctBindProcessInstance](#pbct-bind-process-instance)| `[]*PbctBindProcessInstance` |  | |  |  |



### <span id="pbcs-process-instance-resp"></span> pbcsProcessInstanceResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| processInstances | \[\][PbctListProcessInstance](#pbct-list-process-instance)| `[]*PbctListProcessInstance` |  | |  |  |



### <span id="pbcs-process-template-resp"></span> pbcsProcessTemplateResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| processTemplates | \[\][PbctProcTemplate](#pbct-proc-template)| `[]*PbctProcTemplate` |  | |  |  |



### <span id="pbcs-publish-resp"></span> pbcsPublishResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| haveCredentials | boolean| `bool` |  | | 是否有关联密钥 |  |
| havePull | boolean| `bool` |  | | 是否被客户端拉取过 |  |
| id | int64 (formatted integer)| `int64` |  | | 版本发布后的ID |  |



### <span id="pbcs-service-instance-resp"></span> pbcsServiceInstanceResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| serviceInstances | \[\][PbctServiceInstanceInfo](#pbct-service-instance-info)| `[]*PbctServiceInstanceInfo` |  | |  |  |



### <span id="pbcs-service-template-resp"></span> pbcsServiceTemplateResp


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| serviceTemplates | \[\][PbctServiceTemplate](#pbct-service-template)| `[]*PbctServiceTemplate` |  | |  |  |



### <span id="pbcs-update-config-template-resp"></span> pbcsUpdateConfigTemplateResp


  

[interface{}](#interface)

### <span id="pbcs-update-kv-resp"></span> pbcsUpdateKvResp


  

[interface{}](#interface)

### <span id="pbct-bind-process-instance"></span> pbctBindProcessInstance


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| id | int64 (formatted integer)| `int64` |  | |  |  |
| name | string| `string` |  | |  |  |
| processName | string| `string` |  | |  |  |



### <span id="pbct-bind-template"></span> pbctBindTemplate


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| byteSize | uint64 (formatted string)| `string` |  | | 文件大小 |  |
| ccProcessIds | []int64 (formatted integer)| `[]int64` |  | | 关联cc中未通过服务模板创建的进程实例ID |  |
| ccTemplateProcessIds | []int64 (formatted integer)| `[]int64` |  | | 关联cc服务模版下的模板进程ID |  |
| charset | string| `string` |  | | 文件编码 |  |
| fileMode | string| `string` |  | `"unix"`| 文件模式 |  |
| fileName | string| `string` |  | | 文件名 |  |
| filePath | string| `string` |  | | 文件路径 |  |
| highlightStyle | string| `string` |  | | 高亮风格 |  |
| md5 | string| `string` |  | | 文件md5 |  |
| memo | string| `string` |  | | 配置模版描述 |  |
| name | string| `string` |  | | 配置模版名称 |  |
| privilege | string| `string` |  | | 文件权限 |  |
| revisionName | string| `string` |  | | 模板文件版本号 |  |
| sign | string| `string` |  | | 文件sha256 |  |
| templateSetName | string| `string` |  | | 模板套餐名称 |  |
| templateSpaceName | string| `string` |  | | 模板空间名称 |  |
| user | string| `string` |  | | 用户权限名 |  |
| userGroup | string| `string` |  | | 用户组权限名 |  |



### <span id="pbct-biz-topo-node"></span> pbctBizTopoNode


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| bkInstId | int64 (formatted integer)| `int64` |  | |  |  |
| bkInstName | string| `string` |  | |  |  |
| bkObjIcon | string| `string` |  | |  |  |
| bkObjId | string| `string` |  | |  |  |
| bkObjName | string| `string` |  | |  |  |
| child | \[\][PbctBizTopoNode](#pbct-biz-topo-node)| `[]*PbctBizTopoNode` |  | |  |  |
| default | int64 (formatted integer)| `int64` |  | |  |  |
| processCount | int64 (formatted integer)| `int64` |  | |  |  |
| serviceTemplateId | int64 (formatted integer)| `int64` |  | |  |  |



### <span id="pbct-config-template"></span> pbctConfigTemplate


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| attachment | [PbctConfigTemplateAttachment](#pbct-config-template-attachment)| `PbctConfigTemplateAttachment` |  | |  |  |
| id | int64 (formatted integer)| `int64` |  | | 配置模板ID |  |
| revision | [PbbaseRevision](#pbbase-revision)| `PbbaseRevision` |  | |  |  |
| spec | [PbctConfigTemplateSpec](#pbct-config-template-spec)| `PbctConfigTemplateSpec` |  | |  |  |



### <span id="pbct-config-template-attachment"></span> pbctConfigTemplateAttachment


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| bizId | int64 (formatted integer)| `int64` |  | | 业务ID |  |
| ccProcessIds | []int64 (formatted integer)| `[]int64` |  | | 关联cc中未通过服务模板创建的进程实例ID |  |
| ccTemplateProcessIds | []int64 (formatted integer)| `[]int64` |  | | 关联cc服务模版下的模板进程ID |  |
| templateId | int64 (formatted integer)| `int64` |  | | 关联的模板ID |  |
| tenantId | string| `string` |  | | 租户ID |  |



### <span id="pbct-config-template-spec"></span> pbctConfigTemplateSpec


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| fileName | string| `string` |  | | 文件名 |  |
| highlightStyle | string| `string` |  | | 语法高亮 |  |
| name | string| `string` |  | | 配置模版名称 |  |



### <span id="pbct-config-template-variable"></span> pbctConfigTemplateVariable


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| key | string| `string` |  | | 变量键 |  |
| memo | string| `string` |  | | 变量描述 |  |
| type | string| `string` |  | | 变量类型 |  |



### <span id="pbct-list-process-instance"></span> pbctListProcessInstance


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| property | [PbctProcessInfo](#pbct-process-info)| `PbctProcessInfo` |  | |  |  |
| relation | [PbctRelation](#pbct-relation)| `PbctRelation` |  | |  |  |



### <span id="pbct-proc-template"></span> pbctProcTemplate


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| bkBizId | int64 (formatted integer)| `int64` |  | |  |  |
| bkProcessName | string| `string` |  | |  |  |
| id | int64 (formatted integer)| `int64` |  | |  |  |
| serviceTemplateId | int64 (formatted integer)| `int64` |  | |  |  |



### <span id="pbct-process-info"></span> pbctProcessInfo


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| bkFuncName | string| `string` |  | |  |  |
| bkProcessId | int32 (formatted integer)| `int32` |  | |  |  |
| bkProcessName | string| `string` |  | |  |  |



### <span id="pbct-relation"></span> pbctRelation


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| bkBizId | int32 (formatted integer)| `int32` |  | |  |  |
| bkHostId | int32 (formatted integer)| `int32` |  | |  |  |
| bkProcessId | int32 (formatted integer)| `int32` |  | |  |  |
| bkSupplierAccount | string| `string` |  | |  |  |
| processTemplateId | int32 (formatted integer)| `int32` |  | |  |  |
| serviceInstanceId | int32 (formatted integer)| `int32` |  | |  |  |



### <span id="pbct-service-instance-info"></span> pbctServiceInstanceInfo


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| id | int32 (formatted integer)| `int32` |  | |  |  |
| name | string| `string` |  | |  |  |
| processCount | int64 (formatted integer)| `int64` |  | |  |  |



### <span id="pbct-service-template"></span> pbctServiceTemplate


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| id | int64 (formatted integer)| `int64` |  | |  |  |
| name | string| `string` |  | |  |  |



### <span id="pbct-template-search-cond"></span> pbctTemplateSearchCond


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| fileName | string| `string` |  | |  |  |
| reviser | string| `string` |  | |  |  |
| templateName | string| `string` |  | |  |  |



### <span id="pbkv-kv"></span> pbkvKv


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| attachment | [PbkvKvAttachment](#pbkv-kv-attachment)| `PbkvKvAttachment` |  | |  |  |
| contentSpec | [PbcontentContentSpec](#pbcontent-content-spec)| `PbcontentContentSpec` |  | |  |  |
| id | int64 (formatted integer)| `int64` |  | | 键值配置项ID |  |
| kvState | string| `string` |  | | 键值配置项状态：(ADD、DELETE、REVISE、UNCHANGE) |  |
| revision | [PbbaseRevision](#pbbase-revision)| `PbbaseRevision` |  | |  |  |
| spec | [PbkvKvSpec](#pbkv-kv-spec)| `PbkvKvSpec` |  | |  |  |



### <span id="pbkv-kv-attachment"></span> pbkvKvAttachment


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| appId | int64 (formatted integer)| `int64` |  | | 服务ID |  |
| bizId | int64 (formatted integer)| `int64` |  | | 业务ID |  |



### <span id="pbkv-kv-spec"></span> pbkvKvSpec


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| certificateExpirationDate | string| `string` |  | | 证书过期时间 |  |
| key | string| `string` |  | | 配置项名 |  |
| kvType | string| `string` |  | | 键值类型：(any、string、number、text、json、yaml、xml、secret) |  |
| memo | string| `string` |  | | 描述 |  |
| secretHidden | boolean| `bool` |  | | 是否隐藏值：是=true，否=false |  |
| secretType | string| `string` |  | | 密钥类型：(password、、certificate、secret_key、token、custom) |  |
| value | string| `string` |  | | 配置项值 |  |



### <span id="pbrelease-callback-result"></span> pbreleaseCallbackResult


  

[interface{}](#interface)

### <span id="pbrelease-current-steps"></span> pbreleaseCurrentSteps


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| activityKey | string| `string` |  | |  |  |
| name | string| `string` |  | |  |  |
| ticketId | string| `string` |  | |  |  |



### <span id="pbrelease-form-data"></span> pbreleaseFormData


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| ticketTitle | string| `string` |  | |  |  |



### <span id="pbrelease-ticket"></span> pbreleaseTicket


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| approveResult | boolean| `bool` |  | |  |  |
| callbackResult | [PbreleaseCallbackResult](#pbrelease-callback-result)| `PbreleaseCallbackResult` |  | |  |  |
| createdAt | string| `string` |  | |  |  |
| currentProcessors | \[\][interface{}](#interface)| `[]interface{}` |  | |  |  |
| currentSteps | \[\][PbreleaseCurrentSteps](#pbrelease-current-steps)| `[]*PbreleaseCurrentSteps` |  | |  |  |
| endAt | string| `string` |  | |  |  |
| formData | [PbreleaseFormData](#pbrelease-form-data)| `PbreleaseFormData` |  | |  |  |
| frontendUrl | string| `string` |  | |  |  |
| id | string| `string` |  | |  |  |
| portalId | string| `string` |  | |  |  |
| serviceId | string| `string` |  | |  |  |
| sn | string| `string` |  | |  |  |
| status | string| `string` |  | |  |  |
| statusDisplay | string| `string` |  | |  |  |
| systemId | string| `string` |  | |  |  |
| title | string| `string` |  | |  |  |
| updatedAt | string| `string` |  | |  |  |
| workflowId | string| `string` |  | |  |  |



### <span id="pbtv-template-variable-spec"></span> pbtvTemplateVariableSpec


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| defaultVal | string| `string` |  | | 默认值 |  |
| memo | string| `string` |  | | 变量描述 |  |
| name | string| `string` |  | | 变量名称 |  |
| type | string| `string` |  | | 变量类型：string、number |  |



### <span id="protobuf-any"></span> protobufAny


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| @type | string| `string` |  | |  |  |



**Additional Properties**

any

### <span id="protobuf-null-value"></span> protobufNullValue


> `NullValue` is a singleton enumeration to represent the null value for the
`Value` type union.

 The JSON representation for `NullValue` is JSON `null`.

 - NULL_VALUE: Null value.
  



| Name | Type | Go type | Default | Description | Example |
|------|------|---------| ------- |-------------|---------|
| protobufNullValue | string| string | `"NULL_VALUE"`| `NullValue` is a singleton enumeration to represent the null value for the</br>`Value` type union.</br></br> The JSON representation for `NullValue` is JSON `null`.</br></br> - NULL_VALUE: Null value. |  |



### <span id="repository-object-metadata"></span> repository.ObjectMetadata


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| byte_size | integer| `int64` |  | |  |  |
| md5 | string| `string` |  | |  |  |
| sha256 | string| `string` |  | |  |  |



### <span id="rest-o-k-response"></span> rest.OKResponse


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| data | [interface{}](#interface)| `interface{}` |  | |  |  |



### <span id="rpc-status"></span> rpcStatus


  



**Properties**

| Name | Type | Go type | Required | Default | Description | Example |
|------|------|---------|:--------:| ------- |-------------|---------|
| code | int32 (formatted integer)| `int32` |  | |  |  |
| details | \[\][ProtobufAny](#protobuf-any)| `[]*ProtobufAny` |  | |  |  |
| message | string| `string` |  | |  |  |


