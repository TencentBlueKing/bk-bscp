<template>
  <bk-dialog
    class="associated-process-dialog"
    width="960"
    :is-show="isShow"
    :confirm-text="$t('保存')"
    @closed="handleClose"
    @confirm="handleConfirm">
    <SelectProcess
      :bk-biz-id="bkBizId"
      :template-name="templateName"
      :template-id="templateId"
      @change="selectProcess = $event" />
  </bk-dialog>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { bindProcessInstance } from '../../../../../api/config-template';
  import SelectProcess from './select-process.vue';

  const props = defineProps<{
    isShow: boolean;
    bkBizId: string;
    templateId: number;
    templateName: string;
    updatePerm: boolean;
  }>();
  const emits = defineEmits(['update:isShow', 'confirm', 'noPerm']);
  const selectProcess = ref<{
    cc_process_ids: number[];
    cc_template_process_ids: number[];
  }>();

  const handleClose = () => {
    emits('update:isShow', false);
  };

  const handleConfirm = async () => {
    try {
      if (!props.updatePerm) {
        emits('update:isShow', false);
        emits('noPerm');
        return;
      }
      await bindProcessInstance(props.bkBizId, props.templateId, selectProcess.value);
      emits('update:isShow', false);
      emits('confirm');
    } catch (error) {
      console.error(error);
    }
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
