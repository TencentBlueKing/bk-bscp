// 环境类型枚举
export enum EnvType {
  PRODUCTION = 'prod',
  STAGING = 'staging',
  TESTING = 'test',
  DEVELOPMENT = 'dev',
}

export interface IUpdateEnvItem {
  type: EnvType;
  name: string;
  memo: string;
}

// 环境数据实体
export interface IEnvItem {
  id: string | number;
  spec: {
    name: string;
    type: EnvType;
    memo: string;
    protected?: boolean;
    display_order?: number;
    app_count?: number;
    is_default: boolean;
  };
  attachment: {
    tenant_id?: string;
    biz_id: number;
    project_id: number;
  };
  revision: {
    creator: string;
    reviser?: string;
    create_at: string;
    update_at?: string;
  };
}

// 按类型分组的环境列表项（用于环境选择器）
// type 使用 EnvType 枚举值，展示时可通过 ENV_TYPE_CONFIG 取对应文案/图标
export interface IEnvGroupItem {
  type: EnvType;
  name: string;
  envs: IEnvItem[];
}

// 查询参数
export interface IEnvQuery {
  start?: number;
  limit?: number;
  all?: boolean;
  search_condition?: {
    [key: string]: string;
  };
}
