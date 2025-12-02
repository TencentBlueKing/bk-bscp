import http from '../request';

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
export const getPreviewProcessInstance = (biz_id: string, config_template_id: number) =>
  http
    .get(`/config/biz_id/${biz_id}/config_template/${config_template_id}/preview_bind_process_instance`)
    .then((res) => res.data);
