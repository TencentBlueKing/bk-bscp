export interface ITopoTreeNodeRes {
  bk_inst_id: number;
  bk_inst_name: string;
  bk_obj_icon: string;
  bk_obj_id: string; // biz | set | module | ...
  bk_obj_name: string;
  child: ITopoTreeNodeRes[];
  default: number;
  host_count: number;
  process_count: number;
  service_template_id: number;
}

// 前端加工后的拓扑节点（只保留需要字段）
export interface ITopoTreeNode {
  child: ITopoTreeNode[];
  topoParentName: string;
  topoParent: ITopoTreeNode | null;
  topoVisible: boolean;
  topoExpand: boolean;
  topoLoading: boolean;
  topoLevel: number;
  topoType: string;
  topoProcess: boolean;
  topoChecked: boolean;
  topoName: string;
  service_template_id: number;
  bk_inst_id?: number;
  topoProcessCount?: number;
  service_instance_id?: number;
  processId?: number;
}

export interface ITemplateTreeNodeRes {
  bk_biz_id: number;
  bk_supplier_account: string;
  create_time: string;
  creator: string;
  host_apply_enabled: boolean;
  id: number;
  last_time: string;
  modifier: string;
  name: string;
  service_category_id: number;
  process_count: number;
}

export interface IProcessPreviewItem {
  __IS_RECOVER: boolean;
  id: number;
  topoName: string;
  topoParentName: string;
  topoNode?: ITopoTreeNode;
}

export interface IConfigTemplateItem {
  id: number;
  attachment: {
    biz_id: number;
    cc_process_ids: number[];
    cc_template_process_ids: number[];
    template_id: number;
    tenant_id: string;
  };
  revision: {
    create_at: string;
    creator: string;
    reviser: string;
    update_at: string;
  };
  spec: {
    file_name: string;
    name: string;
  };
  instCount?: number;
  templateCount?: number;
  templateName?: string;
  is_config_released: boolean;
  is_proc_bound: boolean;
}

export interface IConfigTemplateEditParams {
  name: string;
  memo: string;
  file_type: string;
  charset: string;
  file_name: string;
  file_path: string;
  file_mode: string;
  user: string;
  user_group: string;
  privilege: string;
  fileAP: string;
  revision_name: string;
  highlight_style: string;
}

export interface ITemplateProcessItem {
  biz_id: number;
  cc_process_id: number;
  config_template_id: number;
  config_template_name: string;
  config_version_memo: string;
  config_version_name: string;
  file_name: string;
  latest_template_revision_name: string;
  module: string;
  module_inst_seq: number;
  process_alias: string;
  service_instance: string;
  set: string;
  status: string;
  generation_time: string;
  latest_template_revision_id: number;
  task_id: string;
  revision: {
    creator: string;
    reviser: string;
    createAt: string;
    updateAt: string;
  } | null;
}

export interface ITemplateProcess {
  list: ITemplateProcessItem[];
  versions: {
    id: string;
    name: string;
  }[];
  id: number;
  revisionId: number;
  revisionName: string;
}

// 配置生成请求体
export interface IGenerateConfigParams {
  configTemplateGroups: {
    configTemplateId: number;
    configTemplateVersionId: number;
    ccProcessIds: number[];
  }[];
}

// 配置生成状态
export interface IGenerateConfigStatus {
  config_instance_key: string;
  status: string;
  task_id: string;
}

// 配置对比
export interface ICompareConfigType {
  oldConfigContent: {
    content: string;
    createTime: string;
  };
  newConfigContent: {
    content: string;
    createTime: string;
  };
}
