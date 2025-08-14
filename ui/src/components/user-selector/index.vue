<template>
  <!-- 多选模式 -->
  <BkUserSelector
    v-if="spaceFeatureFlags.ENABLE_MULTI_TENANT_MODE"
    :model-value="modelValue"
    :api-base-url="api"
    :tenant-id="tenantId"
    :multiple="true"
    :draggable="true"
    @change="handleUsersChange" />
  <BkMemberSelector
    v-else
    :model-value="modelValue"
    :api="api"
    :class="{ 'input-error': isError }"
    @change="emits('change', $event)">
    <template v-for="(obj, name) in $slots" #[name]="data">
      <slot :name="name" v-bind="data" />
    </template>
  </BkMemberSelector>
</template>

<script lang="ts" setup>
  import BkUserSelector from '@blueking/bk-user-selector';
  import BkMemberSelector from './user-selector-origin/index';
  import '@blueking/bk-user-selector/vue3/vue3.css';
  import { ref, onBeforeMount } from 'vue';
  import useUserStore from '../../store/user';
  import useGlobalStore from '../../store/global';
  import { storeToRefs } from 'pinia';
  import { ITenantUser } from '../../../types/index';

  withDefaults(
    defineProps<{
      isError: boolean;
      modelValue: string[];
    }>(),
    {},
  );

  const { userInfo } = storeToRefs(useUserStore());
  const { spaceFeatureFlags } = storeToRefs(useGlobalStore());

  const emits = defineEmits(['change']);
  const api = ref(''); // API 基础路径

  // 租户 ID
  const tenantId = ref(userInfo.value.tenant_id);


  onBeforeMount(() => {
    if (spaceFeatureFlags.value.ENABLE_MULTI_TENANT_MODE) {
      api.value = `${(window as any).USER_MAN_HOST}`;
    } else {
      api.value = `${(window as any).USER_MAN_HOST}/fs_list_users/?app_code=bk-magicbox&page_size=1000&page=1`;
    }
  });

  // 处理多选模式下的值变化
  const handleUsersChange = (users: ITenantUser[]) => {
    const userList = users.map((user: ITenantUser) => user.bk_username);
    emits('change', userList);
  };
</script>

<style lang="scss" scoped>
  .input-error {
    :deep(.user-selector-layout) {
      .user-selector-container {
        transition: all 0.3s;
        border-color: #ea3636;
      }
    }
  }
</style>
