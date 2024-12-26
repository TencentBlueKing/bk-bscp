import { localT } from '../i18n';

// 资源类型
export const RECORD_RES_TYPE = {
  app: localT('服务'), // 2024.9 第一版只有这个字段
  config: localT('配置'),
  hook: localT('脚本'),
  variable: localT('变量'),
  release: localT('版本'),
  group: localT('分组'),
  template: localT('模板'),
  credential: localT('客户端密钥'),
  instance: localT('客户端实例'),
};

// 操作行为
export const ACTION = {
  create: localT('创建'),
  publish: localT('上线'),
  update: localT('更新'),
  delete: localT('删除'),
};

// 资源实例
export const INSTANCE = {
  app_name: localT('服务名称'),
  config_file_absolute_path: localT('配置文件绝对路径'),
  config_item_name: localT('配置项名称'),
  hook_name: localT('脚本名称'),
  variable_name: localT('变量名称'),
  config_release_name: localT('配置版本名称'),
  config_release_scope: localT('配置上线范围'),
  group_name: localT('分组名称'),
  template_space_name: localT('模版空间名称'),
  template_set_name: localT('模版套餐名称'),
  template_absolute_path: localT('模版文件绝对路径'),
  template_revision: localT('模版版本号'),
  credential_name: localT('密钥名称'),
  hook_revision_name: localT('脚本版本名称'),
  reference_pre_hook_name: localT('引用前置脚本名称'),
  replace_pre_hook_name: localT('更换前置脚本名称'),
  cancel_pre_hook_name: localT('取消前置脚本名称'),
  reference_post_hook_name: localT('引用后置脚本名称'),
  replace_post_hook_name: localT('更换后置脚本名称'),
  cancel_post_hook_name: localT('取消后置脚本名称'),
  obsolete_config_release_name: localT('废弃配置版本名称'),
  restore_config_release_name: localT('恢复配置版本名称'),
  delete_config_release_name: localT('删除配置版本名称'),
  credential_enable_name: localT('启用密钥名称'),
  credential_unable_name: localT('禁用密钥名称'),
  associated_app_config_credential_name: localT('关联服务配置密钥名称'),
  config_retry_client_uid: localT('配置重新拉取客户端UID'),
  config_retry_client_ip: localT('配置重新拉取客户端IP'),
  // operate_objects: localT('等 xx 个对象进行操作'),
};

// 操作途径
export const OPERATE_WAY = {
  WebUI: 'WebUI',
  API: 'API',
};

// 状态
export const STATUS = {
  pending_approval: localT('待审批'),
  pending_publish: localT('待上线'),
  revoked_publish: localT('撤销上线'),
  rejected_approval: localT('审批驳回'),
  already_publish: localT('已上线'),
  failure: localT('失败'),
  success: localT('成功'),
};

// 版本状态
export enum APPROVE_STATUS {
  pending_approval = 'pending_approval', // 待审批
  pending_publish = 'pending_publish', // 待上线
  revoked_publish = 'revoked_publish', // 撤销上线
  rejected_approval = 'rejected_approval', // 审批驳回
  already_publish = 'already_publish', // 已上线
  failure = 'failure',
  success = 'success',
}

// 版本上线方式
export enum ONLINE_TYPE {
  manually = 'manually', // 手动上线
  automatically = 'automatically', // 审批通过后自动上线
  scheduled = 'scheduled', // 定时上线
  immediately = 'immediately', // 立即上线
}

// 过滤的Key
export enum FILTER_KEY {
  publish = 'publish', // 上线
  failure = 'failure', // 失败
}

// 操作记录搜索字段
export enum SEARCH_ID {
  resource_type = 'resource_type', // 资源类型
  action = 'action', // 操作行为
  status = 'status', // 状态
  // service = 'service', // 所属服务
  res_instance = 'res_instance', // 资源实例
  operator = 'operator', // 操作人
  operate_way = 'operate_way', // 操作途径
}
