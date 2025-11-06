<template>
  <div class="sync-status">
    <span class="title">{{ $t('进程管理') }}</span>
    <div class="line"></div>
    <div class="status">
      <bk-button class="sync-button" text :disabled="syncStatus === 'loading'" @click="handleSyncStatus">
        <right-turn-line class="icon" />{{ $t('一键同步状态') }}
      </bk-button>
      <span v-if="syncStatus === 'loading'">
        <Spinner class="spinner-icon" /><span class="loading-text">{{ $t('数据同步中，请耐心等待刷新…') }}</span>
      </span>
      <span v-else class="sync-time">{{ $t('最近一次同步：{n}', { n: time }) }}</span>
    </div>
  </div>
  <PrimartTable></PrimartTable>
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { RightTurnLine, Spinner } from 'bkui-vue/lib/icon';
  import { getSyncStatus, syncProcessStatus } from '../../../../api/process';
  import { datetimeFormat } from '../../../../utils';

  const props = defineProps<{
    bizId: string;
  }>();

  const syncStatus = ref('success');
  const time = ref('');

  onMounted(() => {
    handleGetSyncStatus();
  });

  const handleGetSyncStatus = async () => {
    try {
      const res = await getSyncStatus(props.bizId);
      time.value = datetimeFormat(res.last_sync_time);
      syncStatus.value = res.status;
    } catch (error) {
      console.error(error);
    }
  };

  const handleSyncStatus = async () => {
    if (syncStatus.value === 'loading') return;
    try {
      await syncProcessStatus(props.bizId);
      await handleGetSyncStatus();
    } catch (error) {
      console.error(error);
      syncStatus.value = 'error';
    }
  };
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
  .sync-button {
    color: #3a84ff;
    .icon {
      font-size: 14px;
      margin-right: 4px;
    }
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
