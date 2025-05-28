<template>
  <!-- 多选模式 -->
  <BkUserSelector
    v-if="spaceFeatureFlags.ENABLE_MULTI_TENANT_MODE"
    v-model="selectedUsers"
    :api-base-url="api"
    :tenant-id="tenantId"
    :multiple="true"
    :draggable="true"
    @change="handleUsersChange" />
  <BkMemberSelector v-else v-bind="$attrs" :api="api" :class="{ 'input-error': isError }">
    <template v-for="(obj, name) in $slots" #[name]="data">
      <slot :name="name" v-bind="data" />
    </template>
  </BkMemberSelector>
</template>

<script lang="ts" setup>
  import BkUserSelector from '@blueking/bk-user-selector';
  import BkMemberSelector from './user-selector-origin/index';
  import '@blueking/bk-user-selector/vue3/vue3.css';
  import { ref } from 'vue';
  import useUserStore from '../../store/user';
  import useGlobalStore from '../../store/global';
  import { storeToRefs } from 'pinia';

  withDefaults(
    defineProps<{
      isError: boolean;
      api: string;
    }>(),
    {},
  );

  const { userInfo } = storeToRefs(useUserStore());
  const { spaceFeatureFlags } = storeToRefs(useGlobalStore());

  const emits = defineEmits(['change']);


  // 租户 ID
  const tenantId = ref(userInfo.value.tenant_id);

  // 多选选中值
  const selectedUsers = ref([]);

  // 处理多选模式下的值变化
  const handleUsersChange = (users: any) => {
    emits('change', users);
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
