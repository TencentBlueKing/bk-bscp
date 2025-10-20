<template>
  <div class="sync-status">
    <span class="title">{{ $t('进程管理') }}</span>
    <div class="line"></div>
    <div class="status">
      <bk-button class="sync-button" text :disabled="syncLoading">
        <right-turn-line />{{ $t('一键同步状态') }}
      </bk-button>
      <span v-if="syncLoading">
        <Spinner class="spinner-icon" /><span class="loading-text">{{ $t('数据同步中，请耐心等待刷新…') }}</span>
      </span>
      <span v-else class="sync-time">{{ $t('最近一次同步：2025-09-01  10:00:00') }}</span>
    </div>
  </div>
  <PrimartTable></PrimartTable>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { RightTurnLine, Spinner } from 'bkui-vue/lib/icon';
  const syncLoading = ref(true);
</script>

<style scoped lang="scss">
  .sync-status {
    display: flex;
    align-items: center;
  }
  .title {
    font-size: 16px;
    color: #4d4f56;
    line-height: 24px;
    font-weight: 700;
  }
  .line {
    margin: 0 16px;
    width: 1px;
    height: 16px;
    background: #dcdee5;
  }
  .status {
    display: flex;
    align-items: center;
    gap: 24px;
    font-size: 12px;
    .spinner-icon {
      color: #3a84ff;
      font-size: 14px;
      margin-right: 6px;
    }
    .loading-text {
      color: #e38b02;
    }
    .sync-time {
      color: #979ba5;
    }
  }
</style>
