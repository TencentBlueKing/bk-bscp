<template>
  <bk-dialog
    class="associated-process-dialog"
    width="960"
    :is-show="isShow"
    :confirm-text="$t('保存')"
    @closed="handleClose"
    @confirm="handleConfirm">
    <div class="content-wrap">
      <SelectProcess :bk-biz-id="bkBizId"/>
      <ResultPreivew />
    </div>
  </bk-dialog>
</template>

<script lang="ts" setup>
  import ResultPreivew from './result-preivew.vue';
  import SelectProcess from './select-process.vue';

  defineProps<{
    isShow: boolean;
    bkBizId: string;
  }>();
  const emits = defineEmits(['update:isShow', 'confirm']);

  const handleClose = () => {
    emits('update:isShow', false);
  };

  const handleConfirm = () => {
    emits('confirm');
  };
</script>

<style scoped lang="scss">
  .content-wrap {
    display: flex;
    width: 100%;
    height: 500px;
    .associated-wrap,
    .preview-wrap {
      padding: 20px 24px;
      width: 50%;
      height: 100%;
    }
  }
</style>

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
