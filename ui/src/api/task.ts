import http from '../request';

/**
 * 获取任务历史列表
 * @param bizId 业务ID
 * @param params 查询参数
 * @returns
 */
export const getTaskHistoryList = (biz_id: string, params: any) =>
  http.get(`/config/biz_id/${biz_id}/task_batch/list`, { params }).then((res) => res.data);

/**
 * 获取任务详情状态统计
 * @param bizId 业务ID
 * @param taskId 任务id
 * @returns
 */
export const getTaskDetailStatus = (biz_id: string, taskId: number) =>
  http.get(`/config/biz_id/${biz_id}/task_batch/${taskId}/statistics`).then((res) => res.data);

/**
 * 获取任务详情列表
 * @param bizId 业务ID
 * @param taskId 任务id
 * @param param 查询参数
 */
export const getTaskDetailList = (biz_id: string, taskId: number, params: any) =>
  http.get(`/config/biz_id/${biz_id}/task_batch/${taskId}/detail`, { params }).then((res) => res.data);

/**
 * 重试失败任务
 * @param bizId 业务ID
 * @param taskId 任务id
 */
export const retryTask = (biz_id: string, taskId: number) =>
  http.post(`/config/biz_id/${biz_id}/task_batch/${taskId}/retry`).then((res) => res.data);
