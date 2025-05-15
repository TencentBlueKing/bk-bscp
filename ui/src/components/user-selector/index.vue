<template>
  <!-- 多选模式 -->
  <BkUserSelector
    v-model="selectedUsers"
    :api-base-url="api"
    :tenant-id="tenantId"
    :multiple="true"
    :draggable="true"
    @change="handleUsersChange" />
</template>

<script lang="ts" setup>
  import BkUserSelector from '@blueking/bk-user-selector';
  import '@blueking/bk-user-selector/vue3/vue3.css';
  import { ref } from 'vue';
  import useUserStore from '../../store/user';
  import { storeToRefs } from 'pinia';
  import { getApproverListApi } from '../../api';

  const { userInfo } = storeToRefs(useUserStore());

  const emits = defineEmits(['change']);

  const api = ref(getApproverListApi());

  // 租户 ID
  const tenantId = ref(userInfo.value.tenant_id);

  // 多选选中值
  const selectedUsers = ref([]);

  // 处理多选模式下的值变化
  const handleUsersChange = (users: any) => {
    emits('change', users);
  };
</script>

<style scoped lang="scss"></style>
