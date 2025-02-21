import { ICommonQuery } from '../../types/index';
import http from '../request';

/**
 * 手动新建表格
 * @param biz_id 空间ID
 * @param query 数据内容
 * @returns
 */
export const manualCreateTable = (biz_id: string, query: any) => http.post(`/config/biz/${biz_id}/table`, query);

/**
 * 导入创建表结构和数据
 * @param biz_id 空间ID
 * @param query 数据内容
 * @returns
 */
export const importCreateTable = (biz_id: string, query: any) =>
  http.post(`config/biz/${biz_id}/table/struct_and_content`, query);

/**
 * 编辑表格
 * @param biz_id 空间ID
 * @param id 表结构ID
 * @param query 数据内容
 * @returns
 */
export const editTable = (biz_id: string, id: number, query: any) =>
  http.put(`/config/biz/${biz_id}/table/${id}`, query);

/**
 * 删除表格结构
 * @param biz_id 空间ID
 * @param  id 表结构ID
 * @returns
 */
export const deleteTableStructure = (biz_id: string, id: number) =>
  http.delete(`/config/biz/${biz_id}/table/${id}`).then((res) => res.data);

/**
 * 获取托管表格列表
 * @param biz_id 空间ID
 * @param params 查询参数
 * @returns
 */
export const getLocalTableList = (biz_id: string, params: ICommonQuery) =>
  http.get(`/config/biz/${biz_id}/table`, { params }).then((res) => res.data);

/**
 * 获取表格结构
 * @param biz_id 空间ID
 * @param id 表结构ID
 * @returns
 */
export const getTableStructure = (biz_id: string, id: number) =>
  http.get(`/config/biz/${biz_id}/table/${id}`).then((res) => res.data);

/**
 * 获取表格数据
 * @param biz_id 空间ID
 * @param id 表结构ID
 * @returns
 */
export const getTableData = (biz_id: string, id: number, query: ICommonQuery) =>
  http.post(`/config/biz/${biz_id}/table/${id}/content/list`, query).then((res) => res.data);

/**
 * 编辑表格数据
 * @param biz_id 空间ID
 * @param id 表结构ID
 * @param query 数据内容
 * @returns
 */
export const editTableData = (biz_id: string, id: number, query: any) =>
  http.put(`/config/biz/${biz_id}/table/${id}/content`, query);

/**
 * 检测表格结构是否已有数据
 * @param biz_id 空间ID
 * @param id 表结构ID
 * @param query 数据内容
 * @returns
 */
export const getTableStructureHasData = (biz_id: string, id: number) =>
  http.get(`/config/biz/${biz_id}/table/${id}/field/email`).then((res) => res.data);

/**
 * 导入表结构和表数据
 * @param biz_id 空间ID
 * @param id 表结构ID
 * @param query 数据内容
 * @returns
 */
export const importTable = (biz_id: string, id: number, format: string, file: File, progress?: Function) =>
  http
    .post(`/config/biz/${biz_id}/table/${id}/${format}/import`, file, {
      onUploadProgress: (progressEvent: any) => {
        if (progress) {
          const percentCompleted = Math.round((progressEvent.loaded * 100) / progressEvent.total);
          progress(percentCompleted);
        }
      },
    })
    .then((res) => res.data);
