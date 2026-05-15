<template>
  <div class="sync-status">
    <span class="title">{{ $t('进程管理') }}</span>
    <div class="line"></div>
    <div class="status">
      <bk-button
        class="sync-button"
        text
        theme="primary"
        :disabled="syncStatus === 'Running'"
        @click="handleSyncStatus">
        <right-turn-line class="icon" />{{ $t('一键同步状态') }}
      </bk-button>
      <span v-if="syncStatus === 'Success' || syncStatus === 'Failure'" class="sync-time">
        {{ $t('最近一次同步：{n}', { n: time }) }}
        <span :class="syncStatus">[{{ syncStatus === 'Success' ? $t('成功') : $t('失败') }}]</span>
      </span>
      <span v-else-if="syncStatus === 'Running'">
        <Spinner class="spinner-icon" /><span class="loading-text">{{ $t('数据同步中，请耐心等待刷新…') }}</span>
      </span>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onMounted, onBeforeUnmount } from 'vue';
  import { RightTurnLine, Spinner } from 'bkui-vue/lib/icon';
  import { getSyncStatus, syncProcessStatus } from '../../../../api/process';
  import { datetimeFormat } from '../../../../utils';

  const props = defineProps<{
    bizId: string;
  }>();
  const emits = defineEmits(['refresh']);

  const syncStatus = ref('NeverSynced');
  const time = ref('');
  const statusTimer = ref(0);
  const firstSync = ref(true);

  onMounted(() => {
    handleGetSyncStatus();
  });

  onBeforeUnmount(() => {
    if (statusTimer.value) {
      clearTimeout(statusTimer.value);
    }
  });

  const handleGetSyncStatus = async () => {
    try {
      if (statusTimer.value) {
        clearTimeout(statusTimer.value);
      }
      const res = await getSyncStatus(props.bizId);
      time.value = datetimeFormat(res.last_sync_time);
      syncStatus.value = res.status;

      // 首次请求仅更新，不触发 refresh
      if (firstSync.value) {
        firstSync.value = false;
      } else if (syncStatus.value === 'Success' || syncStatus.value === 'Failure') {
        emits('refresh');
      }

      // 同步中，继续轮询
      if (syncStatus.value === 'Running') {
        statusTimer.value = setTimeout(() => {
          handleGetSyncStatus();
        }, 5000);
      }
    } catch (error) {
      console.error(error);
    }
  };

  const handleSyncStatus = async () => {
    if (syncStatus.value === 'RUNNING') return;
    try {
      await syncProcessStatus(props.bizId);
      await handleGetSyncStatus();
    } catch (error) {
      console.error(error);
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
      .Success {
        color: #3fc06d;
      }
      .Failure {
        color: #e24343;
      }
    }
  }
</style>
