<template>
  <bk-dialog
    :is-show="isShow"
    :title="$t('确认一键清除实例记录?')"
    ext-cls="op-process-dialog"
    :theme="'primary'"
    :dialog-type="'operation'"
    header-align="center"
    footer-align="center"
    :draggable="false"
    :quick-close="false"
    width="480px">
    <div class="dialog-content">
      <div class="process-name">
        {{ $t('共 {n} 条实例记录待清除', { n: props.count }) }}
      </div>
      <div class="command">
        {{ $t('清除操作将对进程执行停止或者取消托管操作。') }}
      </div>
    </div>
    <template #footer>
      <div class="dialog-footer">
        <bk-button theme="danger" @click="handleConfirm">
          {{ '清除' }}
        </bk-button>
        <bk-button @click="emits('close')">{{ $t('取消') }}</bk-button>
      </div>
    </template>
  </bk-dialog>
</template>

<script lang="ts" setup>
  const props = defineProps<{
    isShow: boolean;
    count: number;
  }>();
  const emits = defineEmits(['close', 'confirm']);

  const handleConfirm = () => {
    emits('close');
    setTimeout(() => {
      emits('confirm');
    }, 300);
  };
</script>

<style scoped lang="scss">
  .dialog-content {
    font-size: 14px;
    .process-name {
      .label {
        color: #4d4f56;
      }
      .name {
        color: #313238;
      }
    }
    .command {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-top: 16px;
      padding: 0 16px;
      height: 46px;
      background: #f5f7fa;
      border-radius: 2px;
      line-height: 46px;
      .content {
        width: 240px;
      }
    }
  }
  .dialog-footer {
    .bk-button {
      width: 88px;
      margin-right: 8px;
    }
  }
</style>

<style lang="scss">
  .op-process-dialog {
    .bk-dialog-header {
      padding-top: 48px !important;
    }
    .bk-modal-content {
      padding: 0 32px !important;
    }
    .bk-modal-footer {
      height: auto !important;
      background-color: #fff !important;
      border-top: none !important;
      padding: 24px !important;
    }
  }
</style>
