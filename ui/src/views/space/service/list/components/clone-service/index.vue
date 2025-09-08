<template>
  <bk-sideslider
    width="960"
    quick-close
    :is-show="props.show"
    :title="t('克隆服务')"
    :before-close="handleBeforeClose"
    @closed="close">
    <div class="steps-wrap">
      <bk-steps class="setps" theme="primary" :cur-step="stepsStatus.curStep" :steps="stepsStatus.objectSteps" />
    </div>
    <div class="clone-app-content">
      <SearviceForm
        v-if="stepsStatus.curStep === 1"
        ref="formCompRef"
        :form-data="serviceEditForm"
        :approver-api="getApproverListApi()"
        @change="handleChange" />
      <ImportConfig v-else-if="stepsStatus.curStep === 2" :service="service"/>
    </div>
    <div class="clone-app-footer">
      <bk-button v-if="stepsStatus.curStep > 1" @click="stepsStatus.curStep--">{{ t('上一步') }}</bk-button>
      <bk-button v-if="stepsStatus.curStep < 3" theme="primary" :loading="pending" @click="stepsStatus.curStep++">
        {{ t('下一步') }}
      </bk-button>
      <bk-button v-if="stepsStatus.curStep === 3" @click="handleCloneApp">{{ t('创建') }}</bk-button>
      <bk-button @click="close">{{ t('取消') }}</bk-button>
    </div>
  </bk-sideslider>
</template>

<script lang="ts" setup>
  import { ref, watch } from 'vue';
  import { IAppItem } from '../../../../../../../types/app';
  import { getApproverListApi } from '../../../../../../api';
  import { IServiceEditForm } from '../../../../../../../types/service';
  import { useI18n } from 'vue-i18n';
  import useModalCloseConfirmation from '../../../../../../utils/hooks/use-modal-close-confirmation';
  import SearviceForm from '../service-form.vue';
  import ImportConfig from './import-config.vue';
  const { t } = useI18n();

  const props = defineProps<{
    show: boolean;
    service: IAppItem;
  }>();
  const emits = defineEmits(['update:show', 'reload']);

  const isFormChange = ref(false);
  const pending = ref(false);
  const stepsStatus = ref({
    objectSteps: [{ title: t('填写服务信息') }, { title: t('导入配置项') }, { title: t('导入脚本') }],
    curStep: 1,
    controllable: true,
  });
  const serviceEditForm = ref<IServiceEditForm>({
    name: '',
    alias: '',
    config_type: 'file',
    data_type: 'any',
    memo: '',
    is_approve: true,
    approver: '',
    approve_type: 'or_sign',
  });

  watch(
    () => props.show,
    (val) => {
      if (val) {
        isFormChange.value = false;
        const { spec } = props.service;
        const { name, memo, config_type, data_type, alias, is_approve, approver, approve_type } = spec;
        serviceEditForm.value = {
          name: `${name}_copy`,
          memo,
          config_type,
          data_type,
          alias: `${alias}_copy`,
          is_approve,
          approver,
          approve_type,
        };
      }
    },
  );

  const handleChange = (val: IServiceEditForm) => {
    isFormChange.value = true;
    serviceEditForm.value = val;
  };

  const handleCloneApp = async () => {};

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

<style scoped lang="scss">
  .steps-wrap {
    display: flex;
    justify-content: center;
    width: 100%;
    margin: 20px 0 4px;
    .setps {
      width: 630px;
    }
  }
  .clone-app-content {
    padding: 20px 24px;
    height: calc(100vh - 170px);
    overflow: auto;
  }

  .clone-app-footer {
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
