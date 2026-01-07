export interface IProcessItem {
  id: number;
  attachment: {
    app_id: number;
    biz_id: number;
    cc_process_id: number;
  };
  proc_inst: IProcInst[];
  spec: {
    actions: Record<string, { enabled: boolean; reason: string }>;
    alias: string;
    cc_sync_status: string;
    cc_sync_updated_at: string;
    environment: string;
    inner_ip: string;
    module_name: string;
    service_name: string;
    set_name: string;
    source_data: string;
    managed_status: string;
    status: string;
    prev_data: string;
    proc_num: number;
    bind_template_ids: number[];
    process_config_view_url: string;
  };
}

export interface IProcInst {
  id: number;
  num?: number;
  spec: {
    actions: {
      stop?: boolean;
      unregister?: boolean;
    };
    host_inst_seq: string;
    module_inst_seq: string;
    status: string;
    managed_status: string;
    status_updated_at: string;
    name: string;
  };
  attachment: {
    biz_id: number;
    tenant_id: string;
    process_id: number;
    cc_process_id: number;
  };
}

export interface IProcessFilterItem {
  label: string;
  value: string;
  list: Array<{ name: string; id: number }>;
}
