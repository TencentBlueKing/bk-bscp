import http from '../request';
import { IProcessFilterQuery } from '../../types/process';

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
export const getProcessFilter = (biz_id: string, params: IProcessFilterQuery) =>
  http.get(`/config/biz_id/${biz_id}/process/filter_options`, { params }).then((res) => res.data);

/**
 * 进程操作
 * @param bizId 业务ID
 * @param query 查询参数
 * @returns
 */
export const processOperate = (biz_id: string, query: any) =>
  http.post(`/config/biz_id/${biz_id}/process/operate`, query).then((res) => res.data);

/**
 * 更新托管信息
 * @param bizId 业务ID
 * @param processId 进程ID
 * @param enableProcessRestart 是否启停进程：默认为 false
 * @returns
 */
export const updateRegisterProcess = (biz_id: string, process_id: number, enable_process_restart = false) =>
  http
    .post(`/config/biz_id/${biz_id}/process/update_register`, { process_id, enable_process_restart })
    .then((res) => res.data);

/**
 * 一键清除
 * @param bizId 业务ID
 * @param processId 进程ID
 * @returns
 */
export const deleteProcess = (biz_id: string, process_id: number) =>
  http.post(`/config/biz_id/${biz_id}/process/delete`, { process_id }).then((res) => res.data);
