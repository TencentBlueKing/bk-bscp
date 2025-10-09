<template>
  <bk-sideslider
    width="640"
    :is-show="props.show"
    :title="t('新建服务')"
    :before-close="handleBeforeClose"
    @closed="close">
    <div class="create-app-form">
      <SearviceForm
        ref="formCompRef"
        :form-data="serviceData"
        @change="handleChange" />
    </div>
    <div class="create-app-footer">
      <bk-button theme="primary" :loading="pending" @click="handleCreateConfirm">
        {{ t('提交') }}
      </bk-button>
      <bk-button @click="close">{{ t('取消') }}</bk-button>
    </div>
  </bk-sideslider>
  <CreateSuccessDialog
    v-model:is-show="isShowConfirmDialog"
    :bk-biz-id="spaceId"
    :app-id="appId"
    :service-data="serviceData" />
</template>
<script setup lang="ts">
  import { ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import useGlobalStore from '../../../../../store/global';
  import { createApp } from '../../../../../api';
  import { IServiceEditForm } from '../../../../../../types/service';
  import useModalCloseConfirmation from '../../../../../utils/hooks/use-modal-close-confirmation';
  import SearviceForm from './service-form.vue';
  import CreateSuccessDialog from './create-success-dialog.vue';

  const { t } = useI18n();

  const props = defineProps<{
    show: boolean;
  }>();
  const emits = defineEmits(['update:show', 'reload']);

  const { spaceId } = storeToRefs(useGlobalStore());

  const serviceData = ref<IServiceEditForm>({
    name: '',
    alias: '',
    config_type: 'file',
    data_type: '',
    memo: '', // @todo 包含换行符后接口会报错
    is_approve: true,
    approver: '',
    approve_type: 'or_sign',
    // encryptionSwtich: false,
    // encryptionKey: '',
  });
  const formCompRef = ref();
  const pending = ref(false);
  const isFormChange = ref(false);
  const isShowConfirmDialog = ref(false);
  const appId = ref();

  watch(
    () => props.show,
    (val) => {
      if (val) {
        isFormChange.value = false;
        serviceData.value = {
          name: '',
          alias: '',
          config_type: 'file',
          data_type: '',
          memo: '',
          is_approve: true,
          approver: '',
          approve_type: 'or_sign',
          // encryptionSwtich: false,
          // encryptionKey: '',
        };
      }
    },
  );

  const handleChange = (val: IServiceEditForm) => {
    isFormChange.value = true;
    serviceData.value = val;
  };

  const handleCreateConfirm = async () => {
    formCompRef.value.validateApprover();
    await formCompRef.value.validate();
    pending.value = false;
    try {
      const resp = await createApp(spaceId.value, serviceData.value);
      appId.value = resp.id;
      emits('reload');
      isShowConfirmDialog.value = true;
      close();
    } catch (e) {
      console.error(e);
    } finally {
      pending.value = false;
    }
  };

  const handleBeforeClose = async () => {
    if (isFormChange.value) {
      const result = await useModalCloseConfirmation();
      return result;
    }
    return true;
  };

  const close = () => {
    emits('update:show', false);
  };
</script>
<style lang="scss" scoped>
  .create-app-form {
    padding: 20px 24px;
    height: calc(100vh - 101px);
    overflow: auto;
  }
  .create-app-footer {
    padding: 8px 24px;
    height: 48px;
    width: 100%;
    background: #fafbfd;
    border-top: 1px solid #dcdee5;
    box-shadow: none;
    button {
      margin-right: 8px;
      min-width: 88px;
    }
  }
</style>
