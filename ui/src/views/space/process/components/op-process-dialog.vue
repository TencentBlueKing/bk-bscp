<template>
  <bk-dialog
    :is-show="isShow"
    :title="$t('确认{n}进程?', { n: info.label })"
    ext-cls="op-process-dialog"
    :theme="'primary'"
    :dialog-type="'operation'"
    header-align="center"
    footer-align="center"
    :draggable="false"
    :quick-close="false">
    <div class="dialog-content">
      <div class="process-name">
        <span class="label">{{ $t('进程别名') }}：</span>
        <span class="name">{{ info.name }}</span>
      </div>
      <div class="command">
        {{ $t('将执行{n}命令', { n: info.label }) }}
        <div class="content">
          <bk-overflow-title type="tips"> {{ info.command }}</bk-overflow-title>
        </div>
      </div>
    </div>
    <template #footer>
      <div class="dialog-footer">
        <bk-button :theme="info.op === 'start' ? 'primary' : 'danger'" @click="handleConfirm">
          {{ props.info.label }}
        </bk-button>
        <bk-button @click="emits('close')">{{ $t('取消') }}</bk-button>
      </div>
    </template>
  </bk-dialog>
</template>

<script lang="ts" setup>
  const props = defineProps<{
    isShow: boolean;
    info: {
      op: string;
      label: string;
      name: string;
      command: string;
    };
  }>();
  const emits = defineEmits(['close', 'confirm']);

  const handleConfirm = () => {
    emits('confirm', props.info.op);
    emits('close');
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
