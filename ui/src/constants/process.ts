import { localT } from '../i18n';

// cc 同步状态
export const CC_SYNC_STATUS = {
  synced: localT('正常'),
  deleted: localT('已删除'),
  updated: localT('有更新'),
  abnormal: localT('异常'),
};

// 进程状态
export const PROCESS_STATUS_MAP = {
  running: localT('运行中'),
  partly_running: localT('部分运行'),
  starting: localT('启动中'),
  restarting: localT('重启中'),
  stopping: localT('停止中'),
  reloading: localT('重载中'),
  stopped: localT('未运行'),
};

// 进程托管状态
export const PROCESS_MANAGED_STATUS_MAP = {
  starting: localT('启动托管中'),
  stopping: localT('停止托管中'),
  managed: localT('托管中'),
  unmanaged: localT('未托管'),
  partly_managed: localT('部分托管中'),
};

// 进程按钮禁用提示
export const PROCESS_BUTTON_DISABLED_TIPS = {
  TASK_RUNNING: localT('任务正在执行，请稍候'),
  UNKNOWN_PROCESS_STATUS: localT('进程状态异常'),
  NO_NEED_OPERATE: localT('当前状态无需执行该操作'),
  PROCESS_DELETED: localT('进程已删除，无法操作'),
  NO_REGISTER_UPDATE: localT('无需更新托管信息'),
  PROCESS_ABNORMAL: localT('进程异常'),
};
