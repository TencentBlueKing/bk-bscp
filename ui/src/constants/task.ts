import { localT } from '../i18n';

export const TASK_ACTION_MAP = {
  register: localT('托管'),
  unregister: localT('取消托管'),
  start: localT('启动'),
  stop: localT('停止'),
  restart: localT('重启'),
  reload: localT('重载'),
  kill: localT('强制停止'),
  config_generate: localT('生成'),
  config_publish: localT('下发'),
  config_check: localT('巡检'),
};

export const TASK_STATUS_MAP = {
  // 执行结果
  failed: localT('执行失败'),
  succeed: localT('执行成功'),
  partly_failed: localT('部分失败'),
  running: localT('正在执行'),
};

export const TASK_DETAIL_STATUS_MAP = {
  FAILURE: localT('执行失败'),
  SUCCESS: localT('执行成功'),
  INITIALIZING: localT('等待执行'),
  RUNNING: localT('正在执行'),
};

export const TASK_DETAIL_COMPARE_STATUS_MAP = {
  NEVER_PUBLISHED: localT('从未下发'),
  SAME: localT('一致'),
  DIFFERENT: localT('不一致'),
  UNKNOWN: localT('未知'),
};
