// 任务历史列表项
export interface ITaskHistoryItem {
  id: number;
  creator: string;
  task_object: string;
  task_action: string;
  status: string;
  start_at: string;
  end_at: string;
  execution_time: number;
  task_data: {
    environment: string;
    operate_range: IOperateRange;
  };
}

export interface IOperateRange {
  cc_process_ids: string[];
  cc_process_names: string[];
  module_names: string[];
  service_names: string[];
  set_names: string[];
}

// 任务详情列表
export interface ITaskDetailItem {
  creator: string;
  execution_time: number;
  message: string;
  status: string;
  task_id: string;
  task_payload: {
    agent_id: string;
    alias: string;
    cc_process_id: number;
    config_data: string;
    environment: string;
    func_name: string;
    host_inst_seq: number;
    inner_ip: string;
    module_inst_seq: number;
    module_name: string;
    service_name: string;
    set_name: string;
  };
}
