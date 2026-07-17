import http from '../request';
import { ICommonQuery } from '../../types/index';
import { IVariableEditParams, IVariableImportParams } from '../../types/variable';

/**
 * 查询变量列表
 * @param biz_id 业务 ID
 * @param project_id 项目 ID（可选，无项目概念时传空字符串，自动使用无项目路径）
 * @param params
 * @returns
 */
export const getVariableList = (biz_id: string, project_id: string, params: ICommonQuery) => {
  // 兼容处理：若 project_id 为空，使用无项目路径（配置模板等 space-level 场景）
  const path = project_id
    ? `/config/biz/${biz_id}/projects/${project_id}/template_variables:list`
    : `/config/biz/${biz_id}/template_variables/list`;
  return http.post(path, params).then((res) => res.data);
};

/**
 * 创建变量
 * @param biz_id 业务ID
 * @param project_id 项目ID
 * @param params 创建参数
 * @returns
 */
export const createVariable = (biz_id: string, project_id: string, params: IVariableEditParams) =>
  http.post(`/config/biz/${biz_id}/projects/${project_id}/template_variables`, params);

/**
 * 编辑变量
 * @param biz_id 业务ID
 * @param project_id 项目ID
 * @param template_variable_id 变量ID
 * @param params 编辑参数
 * @returns
 */
export const updateVariable = (
  biz_id: string,
  project_id: string,
  template_variable_id: number,
  params: { default_val: string; memo: string },
) => http.put(`/config/biz/${biz_id}/projects/${project_id}/template_variables/${template_variable_id}`, params);

/**
 * 删除变量
 * @param biz_id 业务ID
 * @param project_id 项目ID
 * @param template_variable_id 变量ID
 * @returns
 */
export const deleteVariable = (biz_id: string, project_id: string, template_variable_id: number) =>
  http.delete(`/config/biz/${biz_id}/projects/${project_id}/template_variables/${template_variable_id}`);

/**
 * 批量删除变量
 * @param bizId 业务ID
 * @param project_id 项目ID
 * @param ids 变量ID列表
 * @param exclusion_operation 是否跨页
 */
export const batchDeleteVariable = (biz_id: string, project_id: string, ids: number[], exclusion_operation: boolean) =>
  http.post(`/config/biz/${biz_id}/projects/${project_id}/template_variables:batch_delete`, { ids, exclusion_operation });

/**
 * 获取未命名版本服务变量列表
 * @param biz_id 业务ID
 * @param project_id 项目ID
 * @param env_id 环境ID
 * @param app_id 应用ID
 * @returns
 */
export const getUnReleasedAppVariables = (biz_id: string, project_id: string, env_id: string, app_id: number) =>
  http.get(`/config/biz/${biz_id}/projects/${project_id}/envs/${env_id}/apps/${app_id}/template_variables`).then((res) => res.data);

/**
 * 更新未命名版本服务变量列表
 * @param biz_id 业务ID
 * @param project_id 项目ID
 * @param env_id 环境ID
 * @param app_id 应用ID
 * @returns
 */
export const updateUnReleasedAppVariables = (
  biz_id: string,
  project_id: string,
  env_id: string,
  app_id: number,
  variables: IVariableEditParams[]
) =>
  http.put(`/config/biz/${biz_id}/projects/${project_id}/envs/${env_id}/apps/${app_id}/template_variables`, { variables }).then((res) => res.data);

/**
 * 获取服务某个版本的变量列表
 * @param biz_id 业务ID
 * @param project_id 项目ID
 * @param env_id 环境ID
 * @param app_id 应用ID
 * @param release_id 服务版本ID
 * @returns
 */
export const getReleasedAppVariables = (
  biz_id: string,
  project_id: string,
  env_id: string,
  app_id: number,
  release_id: number) =>
  http
    .get(`/config/biz/${biz_id}/projects/${project_id}/envs/${env_id}/apps/${app_id}/releases/${release_id}/template_variables`, { params: {} })
    .then((res) => res.data);

/**
 * 查询未命名版本服务中变量被配置文件引用详情
 * @param biz_id 业务ID
 * @param project_id 项目ID
 * @param env_id 环境ID
 * @param app_id 应用ID
 * @returns
 */
export const getUnReleasedAppVariablesCitedDetail = (
  biz_id: string,
  project_id: string,
  env_id: string,
  app_id: number) =>
  http.get(`/config/biz/${biz_id}/projects/${project_id}/envs/${env_id}/apps/${app_id}/template_variables_references`).then((res) => res.data);

/**
 * 查询服务某个版本的变量被配置文件引用详情
 * @param biz_id 业务ID
 * @param project_id 项目ID
 * @param env_id 环境ID
 * @param app_id 应用ID
 * @param release_id 服务版本ID
 * @returns
 */
export const getReleasedAppVariablesCitedDetail = (
  biz_id: string,
  project_id: string,
  env_id: string,
  app_id: number,
  release_id: number) =>
  http
    .get(`/config/biz/${biz_id}/projects/${project_id}/envs/${env_id}/apps/${app_id}/releases/${release_id}/template_variables_references`)
    .then((res) => res.data);

/**
 * 批量导入变量文本
 * @param biz_id 业务ID
 * @param project_id 项目ID
 * @param params 导入参数
 * @returns
 */
export const importVariablesText = (biz_id: string, project_id: string, params: IVariableImportParams) =>
  http.post(`config/biz/${biz_id}/projects/${project_id}/template_variables:import`, params);

/**
 * 批量导入JSON
 * @param biz_id 业务ID
 * @param project_id 项目ID
 * @param content JSON内容
 * @returns
 */
export const importVariablesJSON = (biz_id: string, project_id: string, content: string) =>
  http.post(`config/biz/${biz_id}/projects/${project_id}/template_variables:import/json`, { data: content });

/**
 * 批量导入YAML
 * @param biz_id 业务ID
 * @param project_id 项目ID
 * @param content YAML内容
 * @returns
 */
export const importVariablesYaml = (biz_id: string, project_id: string, content: string) =>
  http.post(`config/biz/${biz_id}/projects/${project_id}/template_variables:import/yaml`, { data: content });

/**
 * 导出变量
 * @param biz_id 业务ID
 * @param project_id 项目ID
 * @param type 导出格式
 * @returns
 */
export const exportVariables = (biz_id: string, project_id: string, type: string) =>
  http.get(`config/biz/${biz_id}/projects/${project_id}/variables/export?format=${type}`);

/**
 * 导出未命名版本变量
 * @param bizId 业务ID
 * @param project_id 项目ID
 * @param env_id 环境ID
 * @param appId 应用ID
 * @param format 导出格式
 * @returns
 */
export const exportUnReleasedVariables = (
  bizId: string,
  project_id: string,
  env_id: string,
  appId: number,
  format: string) =>
  http.get(`config/biz/${bizId}/projects/${project_id}/envs/${env_id}/apps/${appId}/variables/export`, { params: { format } });

/**
 * 导出已发布版本变量
 * @param bizId 业务ID
 * @param project_id 项目ID
 * @param env_id 环境ID
 * @param appId 应用ID
 * @param release_id 服务版本ID
 * @param format 导出格式
 * @returns
 */
export const exportReleasedVaribles = (
  bizId: string,
  project_id: string,
  env_id: string,
  appId: number,
  release_id: number,
  format: string
) =>
  http.get(`config/biz/${bizId}/projects/${project_id}/envs/${env_id}/apps/${appId}/releases/${release_id}/variables/export`, { params: { format } });
