<template>
  <bk-dialog
    class="associated-process-dialog"
    width="960"
    :is-show="isShow"
    :confirm-text="$t('保存')"
    @closed="handleClose"
    @confirm="handleConfirm">
    <SelectProcess :bk-biz-id="bkBizId" :template="template" />
  </bk-dialog>
</template>

<script lang="ts" setup>
  import { IConfigTemplateItem } from '../../../../../../types/config-template';
  import SelectProcess from './select-process.vue';

  defineProps<{
    isShow: boolean;
    bkBizId: string;
    template: IConfigTemplateItem;
  }>();
  const emits = defineEmits(['update:isShow', 'confirm']);

  const handleClose = () => {
    emits('update:isShow', false);
  };

  const handleConfirm = () => {
    emits('confirm');
  };
</script>

<style scoped lang="scss"></style>

<style lang="scss">
  .associated-process-dialog {
    .bk-dialog-header {
      display: none;
    }
    .bk-modal-wrapper .bk-modal-content {
      padding: 0;
    }
  }
</style>
