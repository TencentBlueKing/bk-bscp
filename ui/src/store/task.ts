import { ref } from 'vue';
import { defineStore } from 'pinia';

export default defineStore('task', () => {
  const taskDetail = ref({
    id: 0,
    task_type: '',
    environment: '',
    operate_range: {
      set_name: '',
      module_name: '',
      service_name: '',
      process_alias: '',
      process_id: '',
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
