import http from '../request';
import { IProjectQuery, IProjectEditArg } from '../../types/project';

/**
 * 获取项目列表
 * @param biz_id 空间ID
 * @param params 查询参数
 * @returns
 */
export const getProjectList = (biz_id: string, params: IProjectQuery) =>
  http.post(`/config/biz/${biz_id}/projects/list`, params);

/**
 * 创建项目
 * @param biz_id 空间ID
 * @param params 项目信息
 * @returns
 */
export const createProject = (biz_id: string, params: IProjectEditArg) =>
  http.post(`/config/biz/${biz_id}/projects`, params);

/**
 * 更新项目
 * @param biz_id 空间ID
 * @param project_id 项目ID
 * @param params 项目信息
 * @returns
 */
export const updateProject = (biz_id: string, project_id: string, params: IProjectEditArg) =>
  http.put(`/config/biz/${biz_id}/projects/${project_id}`, params);

/**
 * 删除项目
 * @param biz_id 空间ID
 * @param project_id 项目ID
 * @returns
 */
export const deleteProject = (biz_id: string, project_id: string) =>
  http.delete(`/config/biz/${biz_id}/projects/${project_id}`);
