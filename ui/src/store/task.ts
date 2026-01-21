import { ref } from 'vue';
import { defineStore } from 'pinia';

export default defineStore('task', () => {
  const taskDetail = ref({
    id: 0,
    task_type: '',
    environment: '',
    operate_range: {
      cc_process_ids: [],
      cc_process_names: [],
      module_names: [],
      service_names: [],
      set_names: [],
    },
    creator: '',
    start_at: '',
    end_at: '',
    execution_time: '',
    task_object: '',
    status: '',
  });
  const filterFlag = ref(false); // 任务详情跳转到进程管理时，触发过滤
  return { taskDetail, filterFlag };
});
