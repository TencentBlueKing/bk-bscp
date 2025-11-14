import http from '../request';

/**
 * 获取进程列表（树状）
 * @param bizId 业务ID
 * @param view_type 查看类型
 */
export const getProcessTree = (biz_id: string, view_type: string) =>
  http.get(`/config/biz_id/${biz_id}/process/${view_type}/tree`).then((res) => res.data);
