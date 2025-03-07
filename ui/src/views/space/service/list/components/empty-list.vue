<template>
  <div class="empty-wrap">
    <tableEmpty v-if="isSearchEmpty" :is-search-empty="true" @clear="emits('clear')" />
    <bk-exception v-else class="exception-wrap-item" type="empty" :description="$t('你尚未创建或加入任何服务')">
      <div class="exception-actions">
        <bk-button
          text
          theme="primary"
          :class="{ 'bk-button-with-no-perm': permCheckLoading || !hasCreateServicePerm }"
          @click="emits('create')">
          {{ $t('立即创建') }}
        </bk-button>
        <span class="divider-middle"></span>
        <!-- <bk-button text theme="primary">{{ t("申请权限") }}</bk-button> -->
      </div>
    </bk-exception>
  </div>
</template>

<script lang="ts" setup>
  import tableEmpty from '../../../../../components/table/table-empty.vue';
  defineProps<{
    isSearchEmpty: boolean;
    permCheckLoading: boolean;
    hasCreateServicePerm: boolean;
  }>();

  const emits = defineEmits(['clear', 'create']);
</script>

<style scoped lang="scss">
  .empty-wrap {
    padding: 24px;
    width: 100%;
  }
</style>
