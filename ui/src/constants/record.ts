import { localT } from '../i18n';

// 资源类型
export const RECORD_RES_TYPE = {
  app_config: localT('服务配置'), // 2024.9 第一版只有这个字段
};

// 操作行为
export const ACTION = {
  create_app: localT('创建服务'),
  publish_app: localT('上线服务'),
  update_app: localT('更新服务'),
  delete_app: localT('删除服务'),
  publish_release_config: localT('上线版本配置'),
};

// 资源实例
export const INSTANCE = {
  releases_name: localT('配置版本名称'),
  group: localT('配置上线范围'),
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
  publish_release_config = 'publish_release_config', // 上线版本配置
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
