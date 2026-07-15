import http from '../request';
import { IGroupCategoriesQuery, IGroupEditArg, IGroupItemInService } from '../../types/group';
import { ICommonQuery } from '../../types/index';
import { localT } from '../i18n';

/**
 * 获取分类列表
 * @param app_id 应用ID
 * @param params 查询参数
 * @returns
 */
export const getGroupCategories = (app_id: number, params: IGroupCategoriesQuery) =>
  http.get(`/config/apps/${app_id}/group_categories`, { params }).then((res) => res.data);

/**
 * 新增分类
 * @param app_id 应用ID
 * @param name 分类名称
 * @returns
 */
export const createCategory = (app_id: number, name: string) =>
  http.post(`/config/apps/${app_id}/group_categories`, { name }).then((res) => res.data);

/**
 * 删除分类
 * @param app_id 应用ID
 * @param group_category_id 分类ID
 * @returns
 */
export const delCategory = (app_id: number, group_category_id: number) =>
  http.delete(`/config/apps/${app_id}/groups/${group_category_id}`).then((res) => res.data);

/**
 * 获取服务下分组列表
 * @param biz_id 空间ID
 * @param app_id 应用ID
 * @param projectId 项目ID
 * @param envId 环境ID
 * @returns
 */
export const getServiceGroupList = (biz_id: string, app_id: number, projectId: string, envId: string) =>
  http.get(`/config/biz/${biz_id}/projects/${projectId}/envs/${envId}/apps/${app_id}/groups`).then((res) => {
    const defaultGroup = res.data.details.find((item: IGroupItemInService) => item.group_id === 0);
    if (defaultGroup) {
      defaultGroup.group_name = localT('全部实例');
    }
    return res.data;
  });

/**
 * 获取空间下分组
 * @param biz_id 空间ID
 * @param projectId 项目ID
 * @returns
 */
export const getSpaceGroupList = (biz_id: string, projectId?: string, topId?: number) =>
  http.get(`/config/biz/${biz_id}/projects/${projectId}/groups`, { params: { top_ids: topId } }).then((res) => res.data);

/**
 * 新增分组
 * @param biz_id 空间ID
 * @param projectId 项目ID
 * @param params 分组编辑参数
 * @returns
 */
export const createGroup = (biz_id: string, projectId?: string, params: IGroupEditArg) =>
  http.post(`/config/biz/${biz_id}/projects/${projectId}/groups`, params).then((res) => res.data);

/**
 * 编辑分组
 * @param biz_id 空间ID
 * @param group_id 分组ID
 * @param projectId 项目ID
 * @param params 分组编辑参数
 * @returns
 */
export const updateGroup = (biz_id: string, group_id: number, projectId?: string, params: IGroupEditArg) =>
  http.put(`/config/biz/${biz_id}/projects/${projectId}/groups/${group_id}`, params).then((res) => res.data);

/**
 * 删除分组
 * @param biz_id 空间ID
 * @param group_id 分组ID
 * @param projectId 项目ID
 * @returns
 */
export const deleteGroup = (biz_id: string, group_id: number, projectId?: string) =>
  http.delete(`/config/biz/${biz_id}/projects/${projectId}/groups/${group_id}`);

/**
 * 批量删除分组
 * @param biz_id 空间ID
 * @param projectId 项目ID
 * @param ids 分组ID列表
 * @returns
 */
export const batchDeleteGroup = (biz_id: string, projectId?: string, ids: number[]) =>
  http.post(`/config/biz/${biz_id}/projects/${projectId}/groups:batch_delete`, { ids });

/**
 * 获取分组已上线服务
 * @param biz_id 空间ID
 * @param group_id 分组ID
 * @param params 查询参数
 * @param projectId 项目ID
 * @returns
 */
export const getGroupReleasedApps = (biz_id: string, group_id: number, params: ICommonQuery, projectId?: string) =>
  http.get(`/config/biz/${biz_id}/projects/${projectId}/groups/${group_id}/released_apps`, { params }).then((res) => res.data);

/**
 * 获取分组选择器
 * @param biz_id 空间ID
 * @param label_name 标签名称
 * @param projectId 项目ID
 * @returns
 */
export const getGroupSelector = (biz_id: string, label_name: string, projectId?: string) =>
  http.get(`/config/biz/${biz_id}/projects/${projectId}/groups:selector/${label_name}`).then((res) => res.data);
