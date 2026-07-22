import http from '../request';
import { IRecordQuery } from '../../types/record';

/**
 * 获取操作记录列表
 * @param bizId 空间 ID
 * @param projectId 项目 ID
 * @param params 查询参数
 * @returns
 */
export const getRecordList = (bizId: string, projectId: string, params: IRecordQuery) =>
  http.get(`/config/biz/${bizId}/projects/${projectId}/audits`, { params }).then((res) => res.data);

/**
 * 审批操作：撤销/驳回/通过/手动上线
 * @param bizId 空间 ID
 * @param projectId 项目 ID
 * @param envId 环境 ID
 * @param appId 服务 ID
 * @param releaseId 版本 ID
 * @param params 参数
 * @returns
 */
export const approve = (
  biz_id: string,
  projectId: string,
  envId: string,
  app_id: number,
  release_id: number,
  params: { publish_status: string; reason?: string },
) =>
  http
    .post(
      `/config/biz/${biz_id}/projects/${projectId}/envs/${envId}/apps/${app_id}/releases/${release_id}/approve`,
      { ...params },
    )
    .then((res) => res.data);
