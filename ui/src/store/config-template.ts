import { ref } from 'vue';
import { defineStore } from 'pinia';

export default defineStore('configTemplate', () => {
  const createVerson = ref(false);
  const isAssociated = ref(false);
  const perms = ref({
    create: false,
    update: false,
    delete: false,
    issued: false,
  });
  return { createVerson, isAssociated, perms };
});
