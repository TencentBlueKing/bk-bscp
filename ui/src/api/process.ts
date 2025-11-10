import http from '../request';

/**
 * 获取进程列表
 * @param bizId 业务ID
 * @param query 查询参数
 * @returns
 */
export const getProcessList = (biz_id: string, query: any) =>
  http.post(`/config/biz_id/${biz_id}/process/list`, query).then((res) => res.data);

/**
 * 一键同步数据
 * @param bizId 业务ID
 * @returns
 */
export const syncProcessStatus = (biz_id: string) =>
  http.post(`/config/biz_id/${biz_id}/sync/cmdb_gse_status`).then((res) => res.data);

/**
 * 获取同步cc状态
 * @param bizId 业务ID
 * @returns
 */
export const getSyncStatus = (biz_id: string) =>
  http.get(`/config/biz_id/${biz_id}/sync/sync_status`).then((res) => res.data);

/**
 * 获取进程过滤条件
 * @param bizId 业务ID
 * @returns
 */
export const getProcessFilter = (biz_id: string) =>
  http.get(`/config/biz_id/${biz_id}/process/filter_options`).then((res) => res.data);

/**
 * 进程操作
 * @param bizId 业务ID
 * @param query 查询参数
 * @returns
 */
export const processOperate = (biz_id: string, query: any) =>
  http.post(`/config/biz_id/${biz_id}/process/operate`, query).then((res) => res.data);
