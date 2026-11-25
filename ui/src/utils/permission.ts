import { IPermissionQueryResourceItem } from '../../types/index';

/**
 * 新建服务的判权资源，与 config-server 创建服务的鉴权一致：项目查看 + 环境查看 + 目标环境上的服务创建
 * 未选中环境时（如当前项目下没有可见环境）只带项目，由 auth-server 补齐该项目的默认环境
 * @param bizId 业务ID
 * @param projectId 项目ID
 * @param envId 目标环境ID
 * @returns 判权资源列表
 */
export const getCreateServicePermResources = (
  bizId: string | number,
  projectId: string | number,
  envId?: string | number,
): IPermissionQueryResourceItem[] => {
  const scope = {
    project_id: +projectId,
    ...(+(envId || 0) > 0 ? { environment_id: +(envId as string | number) } : {}),
  };
  return [
    { biz_id: bizId, basic: { type: 'project', action: 'view', resource_id: +projectId }, project_id: +projectId },
    { biz_id: bizId, basic: { type: 'environment', action: 'view' }, ...scope },
    { biz_id: bizId, basic: { type: 'app', action: 'create' }, ...scope },
  ];
};

/**
 * 新建环境的判权资源，与 config-server 创建环境的鉴权一致：项目查看 + 所属项目上的环境创建
 * @param bizId 业务ID
 * @param projectId 项目ID
 * @returns 判权资源列表
 */
export const getCreateEnvPermResources = (
  bizId: string | number,
  projectId: string | number,
): IPermissionQueryResourceItem[] => [
  { biz_id: bizId, basic: { type: 'project', action: 'view', resource_id: +projectId }, project_id: +projectId },
  { biz_id: bizId, basic: { type: 'environment', action: 'create' }, project_id: +projectId },
];
