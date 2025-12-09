import { ref } from 'vue';
import { defineStore } from 'pinia';

export default defineStore('configTemplate', () => {
  const createVerson = ref(false);
  return { createVerson };
});
