import http from '../request';
import { IRuleUpdateParams, IPreviewRuleParams } from '../../types/credential';

/**
 * 创建新密钥
 * @param biz_id 空间 ID
 * @param project_id 项目 ID
 * @param params 请求参数
 * @returns
 */
export const createCredential = (biz_id: string, project_id: string, params: { memo: string }) =>
  http.post(`/config/biz/${biz_id}/projects/${project_id}/credentials`, params).then((res) => res.data);

/**
 * 获取密钥列表
 * @param biz_id 空间 ID
 * @param project_id 项目 ID
 * @param query 查询参数
 * @returns
 */
export const getCredentialList = (
  biz_id: string,
  project_id: string,
  query: { limit?: number; start: number; searchKey?: string; enable?: boolean; all?: boolean },
) => http.get(`/config/biz/${biz_id}/projects/${project_id}/credentials`, { params: query }).then((res) => res.data);

/**
 * 删除密钥
 * @param biz_id 空间 ID
 * @param project_id 项目 ID
 * @param id 密钥 ID
 * @returns
 */
export const deleteCredential = (biz_id: string, project_id: string, id: number) =>
  http.delete(`/config/biz/${biz_id}/projects/${project_id}/credential/${id}`).then((res) => res.data);

/**
 * 更新密钥
 * @param biz_id 空间 ID
 * @param project_id 项目 ID
 * @param params 请求参数（包含 id、memo、enable、name）
 * @returns
 */
export const updateCredential = (
  biz_id: string,
  project_id: string,
  params: { id: number; memo?: string; enable?: boolean; name?: string },
) => http.put(`/config/biz/${biz_id}/projects/${project_id}/credential/${params.id}`, params).then((res) => res.data);

/**
 * 获取密钥关联的配置文件规则
 * @param biz_id 空间 ID
 * @param project_id 项目 ID
 * @param credential_id 密钥 ID
 * @returns
 */
export const getCredentialScopes = (biz_id: string, project_id: string, credential_id: number) =>
  http.get(`/config/biz/${biz_id}/projects/${project_id}/credential/${credential_id}/scopes`).then((res) => res.data);

/**
 * 更新密钥关联的配置文件规则
 * @param biz_id 空间 ID
 * @param project_id 项目 ID
 * @param credential_id 密钥 ID
 * @param params 规则更新参数
 * @returns
 */
export const updateCredentialScopes = (
  biz_id: string,
  project_id: string,
  credential_id: number,
  params: IRuleUpdateParams,
) => http.put(`/config/biz/${biz_id}/projects/${project_id}/credential/${credential_id}/scope`, params).then((res) => res.data);

/**
 * 获取密钥名称是否存在
 * @param biz_id 空间 ID
 * @param project_id 项目 ID
 * @param credential_name 密钥名称
 * @returns
 */
export const getCredentialExist = (biz_id: string, project_id: string, credential_name: string) =>
  http.get(`/config/biz/${biz_id}/projects/${project_id}/credential/${credential_name}/check`);

/**
 * 获取密钥配置预览项
 * @param biz_id 空间 ID
 * @param project_id 项目 ID
 * @param params 预览参数
 * @returns
 */
export const getCredentialPreview = (biz_id: string, project_id: string, params: IPreviewRuleParams) =>
  http.get(`/config/biz/${biz_id}/projects/${project_id}/credential/scope/preview`, { params });
