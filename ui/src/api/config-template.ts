import http from '../request';
import { IGenerateConfigParams } from '../../types/config-template';

/**
 * 获取拓扑树节点
 * @param biz_id
 * @returns
 */
export const getTopoTreeNodes = (biz_id: string) => http.get(`/config/biz_id/${biz_id}/topo`).then((res) => res.data);

/**
 * 获取服务模板树节点
 * @param biz_id
 * @returns
 */
export const getServiceTemplateTreeNodes = (biz_id: string) =>
  http.get(`/config/biz_id/${biz_id}/service_template`).then((res) => res.data);

/**
 * 根据模块获取服务实例列表
 * @param biz_id
 * @param module_id
 */
export const getServiceInstanceFormModule = (biz_id: string, module_id: number) =>
  http.get(`/config/biz_id/${biz_id}/service_instance/${module_id}`).then((res) => res.data);

/**
 * 根据服务实例查询实例进程列表
 * @param biz_id
 * @param service_template_id
 */
export const getProcessListFormServiceInstance = (biz_id: string, service_instance_id: number) =>
  http.get(`/config/biz_id/${biz_id}/process_instance/${service_instance_id}`).then((res) => res.data);

/**
 * 根据服务模板查询实例进程列表
 * @param biz_id
 * @param service_template_id
 */
export const getProcessListFormServiceTemplate = (biz_id: string, service_template_id: number) =>
  http.get(`/config/biz_id/${biz_id}/process_template/${service_template_id}`).then((res) => res.data);

/**
 * 获取配置模板列表
 * @param biz_id
 */
export const getConfigTemplateList = (biz_id: string, query: any) =>
  http.post(`/config/biz_id/${biz_id}/config_template/list`, query).then((res) => res.data);

/**
 * 创建配置模板
 * @param biz_id
 * @param data
 */
export const createConfigTemplate = (biz_id: string, data: any) =>
  http.post(`/config/biz_id/${biz_id}/config_template`, data).then((res) => res.data);

/**
 * 删除配置模板
 * @param biz_id
 * @param config_template_id
 */
export const deleteConfigTemplate = (biz_id: string, config_template_id: number) =>
  http.delete(`/config/biz_id/${biz_id}/config_template/${config_template_id}`).then((res) => res.data);

/**
 * 新建配置模板版本版本
 * @param biz_id
 * @param config_template_id
 * @param data
 */
export const createConfigTemplateVersion = (biz_id: string, config_template_id: number, data: any) =>
  http.put(`/config/biz_id/${biz_id}/config_template/${config_template_id}`, data).then((res) => res.data);

/**
 * 获取配置模板详情
 * @param biz_id
 * @param config_template_id
 */
export const getConfigTemplateDetail = (biz_id: string, config_template_id: number) =>
  http.get(`/config/biz_id/${biz_id}/config_template/${config_template_id}`).then((res) => res.data);

/**
 * 获取配置模板变量
 * @param biz_id
 */
export const getConfigTemplateVariable = (biz_id: string) =>
  http.get(`/config/biz_id/${biz_id}/config_template/variable`).then((res) => res.data);

/**
 * 绑定进程实例
 * @param biz_id
 * @param config_template_id
 * @param data
 */
export const bindProcessInstance = (biz_id: string, config_template_id: number, data: any) =>
  http
    .post(`/config/biz_id/${biz_id}/config_template/${config_template_id}/bind_process_instance`, data)
    .then((res) => res.data);

/**
 * 预览绑定进程实例
 * @param biz_id
 * @param config_template_id
 */
export const getBindProcessInstance = (biz_id: string, config_template_id: number) =>
  http
    .get(`/config/biz_id/${biz_id}/config_template/${config_template_id}/preview_bind_process_instance`)
    .then((res) => res.data);

/**
 * 获取配置实例列表
 * @param biz_id
 * @param query
 */
export const getConfigInstanceList = (biz_id: string, query: any) =>
  http.post(`/config/biz_id/${biz_id}/config_instances/list`, query).then((res) => res.data);

/**
 * 配置对比
 * @param biz_id
 */
export const compareConfigInstance = (biz_id: string, data: any) =>
  http.post(`/config/biz_id/${biz_id}/config_instances/compare`, data).then((res) => res.data);

/**
 * 配置生成
 * @param biz_id
 * @param data
 */
export const generateConfig = (biz_id: string, data: IGenerateConfigParams) =>
  http.post(`/config/biz_id/${biz_id}/config_instances/generate`, data).then((res) => res.data);

/**
 * 查看配置生成状态
 * @param biz_id
 */
export const getGenerateStatus = (biz_id: string, batch_id: number) =>
  http.post(`/config/biz_id/${biz_id}/config_generate/status`, { batch_id }).then((res) => res.data);

/**
 * 配置下发
 * @param biz_id
 * @param batch_id
 */
export const issueConfig = (biz_id: string, batch_id: number) =>
  http.post(`/config/biz_id/${biz_id}/config_instances/push`, { batch_id }).then((res) => res.data);

/**
 * 查看配置生成结果
 * @param biz_id
 * @param task_id
 */
export const getGenerateResult = (biz_id: string, task_id: string) =>
  http.get(`/config/biz_id/${biz_id}/render_task/${task_id}/result`).then((res) => res.data);

/**
 * 重试配置生成
 * @param biz_id
 * @param data
 */
export const retryGenerateConfig = (biz_id: string, data: any) =>
  http.post(`/config/biz_id/${biz_id}/config_instances/operate`, data).then((res) => res.data);

/**
 * 配置预览
 * @param biz_id
 * @param data
 */
export const previewConfig = (biz_id: string, data: any) =>
  http.post(`/config/biz_id/${biz_id}/config_instances/preview`, data).then((res) => res.data);

/**
 * 配置检查
 * @param biz_id
 * @param data
 */
export const checkConfig = (biz_id: string, data: IGenerateConfigParams) =>
  http.post(`/config/biz_id/${biz_id}/config_instances/check`, data).then((res) => res.data);

/**
 * 配置检查查看
 * @param biz_id
 * @param params
 */
export const checkConfigView = (biz_id: string, params: any) =>
  http.get(`/config/biz_id/${biz_id}/config_instances/view`, { params }).then((res) => res.data);

/**
 * 获取进程实例拓扑树节点
 */
export const getProcessInstanceTopoTreeNodes = (biz_id: string) =>
  http.get(`/config/biz_id/${biz_id}/process_instance_topo`).then((res) => res.data);
