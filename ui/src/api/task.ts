import http from '../request';

/**
 * 获取任务历史列表
 * @param bizId 业务ID
 * @param params 查询参数
 * @returns
 */
export const getTaskHistoryList = (biz_id: string, query: any) =>
  http.post(`/config/biz_id/${biz_id}/task_batch/list`, query).then((res) => res.data);

/**
 * 获取任务详情列表
 * @param bizId 业务ID
 * @param taskId 任务id
 * @param param 查询参数
 */
export const getTaskDetailList = (biz_id: string, taskId: number, query: any) =>
  http.post(`/config/biz_id/${biz_id}/task_batch/${taskId}/detail`, query).then((res) => res.data);

/**
 * 重试失败任务
 * @param bizId 业务ID
 * @param taskId 任务id
 */
export const retryTask = (biz_id: string, taskId: number, query: any) =>
  http.post(`/config/biz_id/${biz_id}/task_batch/${taskId}/retry`, query).then((res) => res.data);

/**
 * 任务对比查看
 * @param bizId 业务ID
 * @param taskId 任务id
 */
export const taskCompare = (biz_id: string, taskId: string) =>
  http.get(`/config/biz_id/${biz_id}/config_instances/diff/${taskId}`).then((res) => res.data);
