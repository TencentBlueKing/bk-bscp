import { ref } from 'vue';
import { defineStore } from 'pinia';

export default defineStore('task', () => {
  const taskDetail = ref({
    id: 0,
    task_type: '',
    environment: '',
    operate_range: '',
    creator: '',
    start_at: '',
    end_at: '',
    execution_time: '',
    task_object: '',
    status: '',
  });
  return { taskDetail };
});
