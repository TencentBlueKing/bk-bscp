import http from '../request';
import { ISpaceDetail, IPermissionQueryResourceItem } from '../../types/index';
import { IAppItem, IAppListQuery } from '../../types/app';
import useUserStore from '../store/user';
import pinia from '../store/index';

const userStore = useUserStore(pinia);
/**
 * 获取空间、项目列表
 * @param biz_id 业务ID
 * @param params 查询过滤条件
 * @returns
 */

export const getSpaceList = () =>
  http.get('auth/user/spaces').then((resp) => {
    const permissioned: ISpaceDetail[] = [];
    const noPermissions: ISpaceDetail[] = [];
    resp.data.items.forEach((item: ISpaceDetail) => {
      const { space_id } = item;
      // @ts-ignore
      item.permission = resp.web_annotations.perms[space_id].find_business_resource;
      if (item.permission) {
        permissioned.push(item);
      } else {
        noPermissions.push(item);
      }
    });
    resp.data.items = [...permissioned, ...noPermissions];
    return resp.data;
  });

/**
 * 获取业务的特性开关配置
 * @param biz 业务ID
 * @returns
 */
export const getSpaceFeatureFlag = (biz: string) =>
  http.get('feature_flags', { params: { biz } }).then((resp) => resp.data);

/**
 * 获取服务列表
 * @param biz_id 业务ID
 * @param projectId 项目ID
 * @param envId 环境ID
 * @param query 查询过滤条件
 * @returns
 */
export const getAppList = (biz_id: string, projectId: string, envId: string, query: IAppListQuery = {}) =>
  http.post(`config/biz/${biz_id}/projects/${projectId}/envs/${envId}/apps:list`, query).then((resp) => {
    resp.data.details.forEach((item: IAppItem) => {
      // @ts-ignore
      item.permissions = resp.web_annotations.perms[item.id] || {};
    });
    return resp.data;
  });

  /**
 * 获取服务下配置文件数量、更新时间等信息
 * @param biz_id 业务ID
 * @param projectId 项目ID
 * @param envId 环境ID
 * @param app_id 服务ID数组
 * @returns
 */
export const getAppsConfigData = (biz_id: string, projectId: string, envId: string, app_id: number[]) =>
  http.post(`config/biz/${biz_id}/projects/${projectId}/envs/${envId}/config_items/count`, { biz_id, app_id }).then((resp) => resp.data);

/**
 * 获取服务详情
 * @param biz_id 业务ID
 * @param projectId 项目ID
 * @param envId 环境ID
 * @param app_id 服务ID
 * @returns
 */
export const getAppDetail = (biz_id: string, projectId: string, envId: string, app_id: number) =>
  http.get(`config/biz/${biz_id}/projects/${projectId}/envs/${envId}/apps/${app_id}`).then((resp) => resp.data);

/**
 * 删除服务
 * @param biz_id 业务ID
 * @param projectId 项目ID
 * @param envId 环境ID
 * @param id 服务ID
 * @returns
 */
export const deleteApp = (biz_id: string, projectId: string, envId: string, id: number) =>
  http.delete(`config/biz/${biz_id}/projects/${projectId}/envs/${envId}/apps/${id}`);

/**
 * 创建服务
 * @param biz_id 业务ID
 * @param projectId 项目ID
 * @param envId 环境ID
 * @param params 服务参数
 * @returns
 */
export const createApp = (biz_id: string, projectId: string, envId: string, params: any) =>
  http.post(`config/biz/${biz_id}/projects/${projectId}/envs/${envId}/apps`, { ...params, biz_id }).then((resp) => resp.data);

/**
 * 更新服务
 * @param params { id, biz_id, projectId, envId, name?, memo?, reload_type?, reload_file_path? }
 * @returns
 */
export const updateApp = (params: any) => {
  const { biz_id, projectId, envId, id, data } = params;
  return http.put(`config/biz/${biz_id}/projects/${projectId}/envs/${envId}/apps/${id}`, data).then((resp) => resp.data);
};

/**
 * 克隆服务
 * @param biz_id 业务ID
 * @param projectId 项目ID
 * @param envId 环境ID
 * @param params { id, biz_id, name?, memo?, reload_type?, reload_file_path? }
 * @returns
 */
export const cloneApp = (biz_id: string, projectId: string, envId: string, params: any) => {
  return http.post(`config/biz/${biz_id}/projects/${projectId}/envs/${envId}/apps:clone`, { ...params, biz_id }).then((resp) => resp.data);
};

/**
 * 查询资源权限以及返回权限申请链接
 * @param params IPermissionQueryResourceItem 查询参数
 */
export const permissionCheck = (params: { resources: IPermissionQueryResourceItem[] }) =>
  http.post('/auth/iam/permission/check', params).then((resp) => resp.data);

/**
 * 获取消息通知信息
 * @returns
 */
export const getNotice = () => http.get('/announcements').then((resp) => resp.data);

/**
 * 退出登录
 * @returns
 */
export const loginOut = () =>
  http.get('/logout').then((resp) => {
    window.location.href = `${resp.data.login_url}${encodeURIComponent(window.location.href)}&is_from_logout=1`;
  });

/**
 * 获取人员名单
 * @returns
 */
export const getUserList = (keyword: string) =>
  http
    .get(`${(window as any).USER_MAN_HOST}/api/v3/open-web/tenant/users/-/search/`, {
      params: { keyword },
      headers: {
        'X-Bscp-Operate-Way': undefined,
        'X-Bk-Tenant-Id': userStore.userInfo.tenant_id || 'system',
      },
    })
    .then((resp) => resp.data);
