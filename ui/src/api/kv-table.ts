import { ICommonQuery } from '../../types/index';
import http from '../request';

/**
 * 新建托管表格数据源
 * @param biz_id 空间ID
 * @param app_id 应用ID
 * @param query 查询参数
 * @returns
 */
export const createLocalTableItem = (biz_id: string, query: any) => http.post(`/config/biz/${biz_id}/table`, query);

/**
 * 获取表格数据
 * @param biz_id 空间ID
 * @param app_id 应用ID
 * @param release_id 版本ID
 * @param params 查询参数
 * @returns
 */
export const getLocalTableData = (biz_id: string, params: ICommonQuery) =>
  http.get(`/config/biz/${biz_id}/table`, { params });
