// 项目列表查询参数
export interface IProjectQuery {
  start?: number;
  limit?: number;
  all?: boolean;
  search_condition?: {
    [key:string]: string;
  }
}

// 项目详情
export interface IProjectItem {
  id: string | number;
  spec: {
    name: string;
    key: string;
    memo: string;
    protected?: boolean;
    env_count: number;
    app_count: number;
    is_default: boolean;
  };
  attachment: {
    tenant_id?: string;
    biz_id: number;
  };
  revision: {
    creator: string;
    reviser?: string;
    create_at: string;
    update_at?: string;
  };
}

// 项目创建/编辑参数
export interface IProjectEditArg {
  id?: number;
  name: string;
  code?: string;
  description?: string;
}

export interface ISpaceProject {
  spaceId: string;
  projectId: string;
}