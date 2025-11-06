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
