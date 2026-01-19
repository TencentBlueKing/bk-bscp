import { ref } from 'vue';
import { defineStore } from 'pinia';

export default defineStore('configTemplate', () => {
  const createVerson = ref(false);
  const isAssociated = ref(false);
  return { createVerson, isAssociated };
});
