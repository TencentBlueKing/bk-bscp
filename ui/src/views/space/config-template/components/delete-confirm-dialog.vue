<template>
  <bk-dialog
    :is-show="isShow"
    :title="title"
    :theme="'primary'"
    width="400px"
    quick-close
    ext-cls="delete-confirm-dialog"
    @closed="handleClose">
    <div class="dialog-header">{{ title }}</div>
    <slot></slot>
    <div class="dialog-footer">
      <bk-button theme="danger" @click="emits('confirm')">{{ t('删除') }}</bk-button>
      <bk-button @click="handleClose">{{ t('取消') }}</bk-button>
    </div>
  </bk-dialog>
</template>

<script lang="ts" setup>
  import { useI18n } from 'vue-i18n';
  const { t } = useI18n();
  withDefaults(
    defineProps<{
      isShow: boolean;
      title: string;
      pending?: boolean;
      confirmText?: string;
    }>(),
    {
      pending: false,
    },
  );

  const handleClose = () => {
    emits('close');
    emits('update:isShow', false);
  };
  const emits = defineEmits(['update:isShow', 'confirm', 'close']);
</script>

<style lang="scss">
  .delete-confirm-dialog {
    .bk-modal-content {
      padding: 48px 40px 24px 40px !important;
      .dialog-header {
        margin-bottom: 16px;
        font-size: 20px;
        color: #313238;
        letter-spacing: 0;
        text-align: center;
        line-height: 32px;
      }
      .dialog-footer {
        margin-top: 24px;
        display: flex;
        justify-content: center;
        .bk-button {
          width: 88px;
          margin-right: 8px;
        }
      }
    }

    .bk-modal-header {
      display: none;
    }
    .bk-modal-body {
      padding-bottom: 0 !important;
    }
    .bk-modal-footer {
      display: none;
    }
  }
</style>
