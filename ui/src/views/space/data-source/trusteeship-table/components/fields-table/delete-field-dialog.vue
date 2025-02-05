<template>
  <bk-dialog
    :is-show="show"
    ext-cls="confirm-dialog"
    width="450"
    :close-icon="true"
    :show-mask="true"
    :quick-close="false"
    :multi-instance="false">
    <template #header>
      <div class="tip-icon__wrap">
        <exclamation-circle-shape class="tip-icon" />
      </div>
      <div class="headline">
        {{ $t('确定删除该字段？') }}
      </div>
    </template>
    <div class="field-name">
      {{ $t('字段名:') }} <span class="name">{{ fieldName }}</span>
    </div>
    <div class="content-info">
      {{ $t('删除后，表格中该字段的值也将被清除，请谨慎操作！') }}
    </div>
    <template #footer>
      <div class="operation-btns">
        <bk-button theme="danger" @click="handleDelete">{{ $t('删除') }}</bk-button>
        <bk-button @click="emits('update:show', false)">{{ $t('关闭') }}</bk-button>
      </div>
    </template>
  </bk-dialog>
</template>

<script setup lang="ts">
  import { ExclamationCircleShape } from 'bkui-vue/lib/icon';

  const emits = defineEmits(['update:show', 'confirm']);

  defineProps<{
    show: boolean;
    fieldName?: string;
  }>();

  const handleDelete = () => {
    emits('confirm');
  };
</script>

<style lang="scss" scoped>
  :deep(.confirm-dialog) {
    .bk-modal-body {
      padding-bottom: 0;
    }
    .bk-modal-content {
      padding: 0 32px;
      height: auto;
      max-height: none;
      min-height: auto;
      border-radius: 2px;
    }
    .bk-modal-footer {
      position: relative;
      padding: 24px 0;
      height: auto;
      border: none;
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
  .headline {
    margin-top: 16px;
    text-align: center;
  }
  .field-name {
    font-size: 12px;
    .name {
      color: #313238;
      margin-left: 8px;
    }
  }
  .content-info {
    margin-top: 8px;
    height: 46px;
    padding: 12px 16px;
    background: #f5f6fa;
    border-radius: 2px;
    font-size: 14px;
  }
  .operation-btns {
    display: flex;
    justify-content: center;
    gap: 8px;
    .bk-button {
      width: 88px;
    }
  }
</style>
