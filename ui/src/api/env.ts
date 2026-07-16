import http from '../request';
import type { IUpdateEnvItem, IEnvQuery } from '../../types/env';

/**
 * 获取环境列表
 */
export function getEnvList(biz_id: string, project_id: string, query: IEnvQuery) {
  return http.post(`/config/biz/${biz_id}/projects/${project_id}/envs:list`, {
    ...query,
  });
}

/**
 * 创建环境
 */
export function createEnv(biz_id: string, project_id: string, data: IUpdateEnvItem) {
  return http.post(`/config/biz/${biz_id}/projects/${project_id}/envs`, data);
}

/**
 * 更新环境
 */
export function updateEnv(biz_id: string, project_id: string, env_id: string, data: IUpdateEnvItem) {
  return http.put(`/config/biz/${biz_id}/projects/${project_id}/envs/${env_id}`, data);
}

/**
 * 删除环境
 */
export function deleteEnv(biz_id: string, project_id: string, env_id: string) {
  return http.delete(`/config/biz/${biz_id}/projects/${project_id}/envs/${env_id}`);
}
