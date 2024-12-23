import { ICommonQuery } from '../../types/index';
import http from '../request';

/**
 * 新建托管表格数据源
 * @param biz_id 空间ID
 * @param query 数据内容
 * @returns
 */
export const createLocalTable = (biz_id: string, query: string) => http.post(`/config/biz/${biz_id}/table`, query);

/**
 * 获取表格数据源列表
 * @param biz_id 空间ID
 * @param params 查询参数
 * @returns
 */
export const getLocalTableList = (biz_id: string, params: ICommonQuery) =>
  http.get(`/config/biz/${biz_id}/table`, { params }).then((res) => res.data);

/**
 * 删除表格数据源列表
 * @param biz_id 空间ID
 * @param  id 数据源id
 * @returns
 */
export const deleteLocalTable = (biz_id: string, id: number) =>
  http.delete(`/config/biz/${biz_id}/table/${id}`).then((res) => res.data);

/**
 * 获取表结构字段数据
 * @param biz_id 空间ID
 * @param id 数据源id
 * @returns
 */
export const getTableStructure = (biz_id: string, id: number) =>
  http.get(`/config/biz/${biz_id}/table/${id}`).then((res) => res.data);

/**
 * 获取表结构数据
 * @param biz_id 空间ID
 * @param params 查询参数
 * @param id 表结构ID
 * @returns
 */
export const getTableStructureData = (biz_id: string, id: number) =>
  http.get(`/config/biz/${biz_id}/table/${id}/content`).then((res) => res.data);

/**
 * 编辑表结构数据
 * @param biz_id 空间ID
 * @param params 查询参数
 * @param id 表结构ID
 * @returns
 */
export const editTableStructure = (biz_id: string, id: number, query: string) =>
  http.put(`/config/biz/${biz_id}/table/${id}`, query);
