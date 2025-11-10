<template>
  <bk-dialog
    :is-show="isShow"
    ext-cls="batch-op-process-dialog"
    :dialog-type="'operation'"
    header-align="center"
    :draggable="false"
    :quick-close="false">
    <template #header>
      <div class="tip-icon__wrap">
        <exclamation-circle-shape class="tip-icon" />
      </div>
      <div class="title">{{ $t('确认批量{n}所选进程？', { n: info.label }) }}?</div>
    </template>
    <div class="dialog-content">{{ $t('已选择 {n} 个进程，将立即{m}运行', { n: info.count, m: info.label }) }}</div>
    <div class="dialog-footer">
      <bk-button theme="primary" @click="handleConfirm">
        {{ $t('继续执行') }}
      </bk-button>
      <bk-button @click="emits('close')">{{ $t('取消') }}</bk-button>
    </div>
  </bk-dialog>
</template>

<script lang="ts" setup>
  import { ExclamationCircleShape } from 'bkui-vue/lib/icon';
  const props = defineProps<{
    isShow: boolean;
    info: {
      op: string;
      label: string;
      count: number;
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
    padding: 0 16px;
    height: 46px;
    background: #f5f7fa;
    border-radius: 2px;
    line-height: 46px;
    color: #4d4f56;
  }
  .dialog-footer {
    text-align: center;
    margin-top: 24px;
    .bk-button {
      margin-right: 8px;
      width: 88px;
    }
  }
  .tip-icon__wrap {
    margin: 0 auto;
    width: 42px;
    height: 42px;
    position: relative;
    &::after {
      content: '';
      position: absolute;
      z-index: -1;
      top: 50%;
      left: 50%;
      transform: translate3d(-50%, -50%, 0);
      width: 30px;
      height: 30px;
      border-radius: 50%;
      background-color: #ff9c01;
    }
    .tip-icon {
      font-size: 42px;
      line-height: 42px;
      vertical-align: middle;
      color: #ffe8c3;
    }
  }
  .title {
    margin: 20px 0 16px;
  }
</style>

<style lang="scss">
  .batch-op-process-dialog {
    .bk-dialog-header {
      padding-top: 24px !important;
    }
    .bk-modal-content {
      padding: 0 32px !important;
      height: auto !important;
      overflow: auto;
      min-height: 0 !important;
    }
    .bk-modal-footer {
      display: none;
    }
  }
</style>
