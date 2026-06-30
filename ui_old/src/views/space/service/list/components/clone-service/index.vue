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
        v-show="stepsStatus.curStep === 1"
        ref="serviceFormRef"
        :form-data="serviceEditForm"
        clone-mode
        @change="handleServiceChange" />
      <ImportConfig
        v-show="stepsStatus.curStep === 2"
        ref="configRef"
        :bk-biz-id="spaceId"
        :service="service"
        @select="handleSelectConfig" />
      <ImportScript v-show="stepsStatus.curStep === 3" :app-id="service.id!" clone-mode @select="handleSelectScript" />
    </div>
    <div class="clone-app-footer">
      <bk-button v-if="stepsStatus.curStep > 1" @click="stepsStatus.curStep--">
        {{ t('上一步') }}
      </bk-button>
      <bk-button v-if="stepsStatus.curStep < maxStep" theme="primary" @click="handleNextStep">
        {{ t('下一步') }}
      </bk-button>
      <bk-button
        v-if="stepsStatus.curStep === maxStep"
        theme="primary"
        :loading="pending"
        :disabled="pending"
        @click="handleCloneApp">
        {{ t('创建') }}
      </bk-button>
      <bk-button @click="close">{{ t('取消') }}</bk-button>
    </div>
  </bk-sideslider>
  <CreateSuccessDialog
    v-model:is-show="isShowConfirmDialog"
    :bk-biz-id="spaceId"
    :app-id="appId"
    :service-data="serviceEditForm"
    :is-create="false" />
</template>

<script lang="ts" setup>
  import { ref, watch, computed } from 'vue';
  import { IAppItem } from '../../../../../../../types/app';
  import { cloneApp } from '../../../../../../api';
  import type { IServiceEditForm } from '../../../../../../../types/service';
  import type { IConfigImportItem, IConfigKvItem } from '../../../../../../../types/config';
  import type { ImportTemplateConfigItem } from '../../../../../../../types/template';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import useGlobalStore from '../../../../../../store/global';
  import useModalCloseConfirmation from '../../../../../../utils/hooks/use-modal-close-confirmation';
  import SearviceForm from '../service-form.vue';
  import ImportConfig from './import-config.vue';
  import ImportScript from '../../../detail/init-script/index.vue';
  import CreateSuccessDialog from '../create-success-dialog.vue';

  const { t } = useI18n();

  const props = defineProps<{
    show: boolean;
    service: IAppItem;
  }>();
  const emits = defineEmits(['update:show', 'reload']);

  const isFormChange = ref(false);
  const pending = ref(false);

  const { spaceId } = storeToRefs(useGlobalStore());
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
  const scriptIds = ref({ pre_hook_id: 0, post_hook_id: 0 });
  const serviceFormRef = ref();
  const configRef = ref();
  const configList = ref<IConfigImportItem[]>([]);
  const kvConfigList = ref<IConfigKvItem[]>([]);
  const templateConfigList = ref<ImportTemplateConfigItem[]>([]);
  const isShowConfirmDialog = ref(false);
  const isFileType = computed(() => props.service.spec.config_type === 'file');
  const maxStep = computed(() => (isFileType.value ? 3 : 2));
  const stepsStatus = ref({
    objectSteps: isFileType.value
      ? [{ title: t('填写服务信息') }, { title: t('导入配置项') }, { title: t('导入脚本') }]
      : [{ title: t('填写服务信息') }, { title: t('导入配置项') }],
    curStep: 1,
    controllable: true,
  });
  const appId = ref();

  watch(
    () => props.show,
    (val) => {
      if (val) {
        isFormChange.value = false;
        stepsStatus.value.curStep = 1;
        stepsStatus.value.objectSteps = isFileType.value
          ? [{ title: t('填写服务信息') }, { title: t('导入配置项') }, { title: t('导入脚本') }]
          : [{ title: t('填写服务信息') }, { title: t('导入配置项') }];
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
        configList.value = [];
        templateConfigList.value = [];
        kvConfigList.value = [];
        scriptIds.value = { pre_hook_id: 0, post_hook_id: 0 };
      }
    },
  );

  const handleServiceChange = (val: IServiceEditForm) => {
    isFormChange.value = true;
    serviceEditForm.value = val;
  };

  const handleNextStep = async () => {
    if (stepsStatus.value.curStep === 1) {
      serviceFormRef.value.validateApprover();
      await serviceFormRef.value.validate();
    }
    if (stepsStatus.value.curStep === 2) {
      await configRef.value.validate();
    }
    stepsStatus.value.curStep += 1;
  };

  const handleSelectConfig = (
    selectConfigList: IConfigImportItem[] | IConfigKvItem[],
    selectTemplateConfigList: ImportTemplateConfigItem[],
  ) => {
    if (isFileType.value) {
      configList.value = selectConfigList as IConfigImportItem[];
      templateConfigList.value = selectTemplateConfigList;
    } else {
      kvConfigList.value = selectConfigList as IConfigKvItem[];
    }
    isFormChange.value = true;
  };

  const handleSelectScript = (ids: { pre_hook_id: number; post_hook_id: number }) => {
    scriptIds.value = ids;
    isFormChange.value = true;
  };

  const handleCloneApp = async () => {
    pending.value = true;
    try {
      if (isFileType.value) {
        let allVariables: {
          default_val: string;
          memo: string;
          name: string;
          type: string;
        }[] = [];
        const allConfigList: any[] = [];
        const allTemplateConfigList: any[] = [];
        templateConfigList.value.forEach((templateConfig) => {
          const { template_set_id, template_revisions, template_space_id } = templateConfig;
          template_revisions.forEach((revision) => {
            allVariables = [...allVariables, ...revision.variables];
          });
          allTemplateConfigList.push({
            template_space_id,
            template_binding: {
              template_set_id,
              template_revisions: template_revisions.map((revision) => {
                const { template_id, template_revision_id, is_latest } = revision;
                return {
                  template_id,
                  template_revision_id,
                  is_latest,
                };
              }),
            },
          });
        });
        configList.value.forEach((config) => {
          const { variables, ...rest } = config;
          if (variables) {
            allVariables = [...allVariables, ...config.variables];
          }
          allConfigList.push({
            ...rest,
          });
        });
        const query = {
          ...serviceEditForm.value,
          bindings: allTemplateConfigList,
          config_items: allConfigList,
          variables: allVariables,
          ...scriptIds.value,
        };
        const res = await cloneApp(spaceId.value, query);
        appId.value = res.id;
      } else {
        await configRef.value.validate();
        const query = {
          ...serviceEditForm.value,
          kv_items: kvConfigList.value,
        };
        const res = await cloneApp(spaceId.value, query);
        appId.value = res.id;
      }
      emits('reload');
      isShowConfirmDialog.value = true;
      close();
    } catch (error) {
      console.log(error);
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
