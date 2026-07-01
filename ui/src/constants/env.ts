import { EnvType } from '../../types/env';
import { localT } from '../i18n';

/**
 * 环境类型展示配置常量
 * 包含各环境类型对应的图标、颜色等 UI 配置
 */

export interface IEnvTypeConfig {
  type: EnvType;
  name: string;
  tip: string;
  iconColor: string;
  iconClass: string;
  textColor: string;
  bgColor: string;
}

/** 按环境类型 key 索引的配置映射 */
export const ENV_TYPE_CONFIG: Record<EnvType, IEnvTypeConfig> = {
  [EnvType.PRODUCTION]: {
    type: EnvType.PRODUCTION,
    name: localT('生产环境'),
    tip: localT('当前处于生产环境的配置管理，请谨慎操作以避免影响线上业务！'),
    iconColor: '#EA3636',
    iconClass: 'icon-shengchanhuanjing',
    textColor: '#E71818',
    bgColor: '#FFF0F0',
  },
  [EnvType.STAGING]: {
    type: EnvType.STAGING,
    name: localT('预发布环境'),
    tip: localT('当前处于预发布环境的配置管理，与生产环境高度一致，请按生产标准进行验证！'),
    iconColor: '#F59500',
    iconClass: 'icon-yufabuhuanjing',
    textColor: '#CC8800',
    bgColor: '#FDF4E8',
  },
  [EnvType.TESTING]: {
    type: EnvType.TESTING,
    name: localT('测试环境'),
    tip: localT('当前处于测试环境的配置管理，仅用于功能测试验证，数据可能会重置。'),
    iconColor: '#3A84FF',
    iconClass: 'icon-ceshihuanjing',
    textColor: '#3A84FF',
    bgColor: '#F0F5FF',
  },
  [EnvType.DEVELOPMENT]: {
    type: EnvType.DEVELOPMENT,
    name: localT('开发环境'),
    tip: localT('当前处于开发环境的配置管理，用于开发调试，服务可能不稳定。'),
    iconColor: '#2CAF5E',
    iconClass: 'icon-kaifahuanjing',
    textColor: '#299E56',
    bgColor: '#EBFAF0',
  },
};

/** 环境类型选项列表（按展示顺序排列） */
export const ENV_TYPE_OPTIONS: IEnvTypeConfig[] = [
  ENV_TYPE_CONFIG[EnvType.PRODUCTION],
  ENV_TYPE_CONFIG[EnvType.STAGING],
  ENV_TYPE_CONFIG[EnvType.TESTING],
  ENV_TYPE_CONFIG[EnvType.DEVELOPMENT],
];
