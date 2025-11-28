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
}

export interface IConfigTemplateItem {
  attachment: {
    biz_id: number;
    cc_process_instance_ids: number[];
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
}
