import http from '../request';

/**
 * 获取任务历史列表
 * @param bizId 业务ID
 * @param query 查询参数
 * @returns
 */

export const getTaskHistoryList = (biz_id: string, params: any) =>
  http.get(`/config/biz_id/${biz_id}/task_batch/list`, { params }).then((res) => res.data);
