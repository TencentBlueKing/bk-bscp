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

// 操作范围（gsekit 风格五段表达式，缺省段为 "*"）
export interface IOperateRange {
  set_name: string;
  module_name: string;
  service_name: string;
  process_alias: string;
  process_id: string;
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
