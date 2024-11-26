### 描述

该接口提供版本：v1.0.0+

分段上传文件内容

### 输入参数

| 参数名称 | 参数类型 | 必选 | 描述   |
| -------- | -------- | ---- | ------ |
| biz_id   | uint32   | 是   | 业务ID |

#### HEADER设置

| 参数名称                 | 参数类型 | 必选 | 描述                                 |
| ------------------------ | -------- | ---- | ------------------------------------ |
| X-Bscp-App-Id            | uint32   | 否   | 如果是应用配置项，则设置该应用ID     |
| X-Bscp-Template-Space-Id | uint32   | 否   | 如果是模版配置项，则设置该模版空间ID |
| X-Bscp-Upload-Id         | uint32   | 否   | 分块上传 ID |
| X-Bscp-Part-Num          | uint32   | 是   | 分块序号，从1开始               |
| X-Bkapi-File-Content-Id  | string   | 是   | 上传文件内容的SHA256值               |

**说明**：X-Bscp-App-Id和X-Bscp-Template-Space-Id有且只能设置其中一个，设置两个或都不设置将报错

### 响应示例

```json
{
  "data": {}
}
```
