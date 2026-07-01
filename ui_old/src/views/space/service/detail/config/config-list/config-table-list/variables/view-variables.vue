<template>
  <bk-button @click="handleOpenSlider">{{ t('查看变量') }}</bk-button>
  <bk-sideslider width="960" :title="t('查看变量')" :is-show="isSliderShow" @closed="close">
    <div class="view-variables-container">
      <div class="buttons-wrapper">
        <ExportVariables @export="handleExport" />
      </div>
      <VariablesTable
        class="variables-table-content"
        :list="variableList"
        :cited-list="citedList"
        :editable="false"
        :show-cited="true" />
    </div>
    <section class="action-btns">
      <bk-button @click="close">{{ t('关闭') }}</bk-button>
    </section>
  </bk-sideslider>
</template>
<script lang="ts" setup>
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import VariablesTable from './variables-table.vue';
  import { IVariableEditParams, IVariableCitedByConfigDetailItem } from '../../../../../../../../../types/variable';
  import {
    getReleasedAppVariables,
    getReleasedAppVariablesCitedDetail,
    exportReleasedVaribles,
  } from '../../../../../../../../api/variable';
  import { downloadFile } from '../../../../../../../../utils/index';
  import ExportVariables from './export-variables.vue';

  const { t } = useI18n();
  const props = defineProps<{
    bkBizId: string;
    appId: number;
    verisionId: number;
  }>();

  const isSliderShow = ref(false);
  const loading = ref(false);
  const variableList = ref<IVariableEditParams[]>([]);
  const citedList = ref<IVariableCitedByConfigDetailItem[]>([]);

  const getVariableList = async () => {
    loading.value = true;
    const [variableListRes, citedListRes] = await Promise.all([
      getReleasedAppVariables(props.bkBizId, props.appId, props.verisionId),
      getReleasedAppVariablesCitedDetail(props.bkBizId, props.appId, props.verisionId),
    ]);
    variableList.value = variableListRes.details;
    citedList.value = citedListRes.details;
    loading.value = false;
  };

  // 导出变量
  const handleExport = async (type: string) => {
    const res = await exportReleasedVaribles(props.bkBizId, props.appId, props.verisionId, type);
    let content: any;
    let mimeType: string;
    let extension: string;
    if (type === 'json') {
      content = JSON.stringify(res, null, 2);
      mimeType = 'application/json';
      extension = 'json';
    } else {
      content = res;
      mimeType = 'text/yaml';
      extension = 'yaml';
    }
    downloadFile(content, mimeType, `bscp_variables_${props.bkBizId}.${extension}`);
  };

  const handleOpenSlider = () => {
    isSliderShow.value = true;
    getVariableList();
  };

  const close = () => {
    isSliderShow.value = false;
    variableList.value = [];
  };
</script>
<style lang="scss" scoped>
  .view-variables-container {
    padding: 20px 0;
    height: calc(100vh - 101px);
  }
  .buttons-wrapper {
    margin-bottom: 16px;
    padding: 0 24px;
  }
  :deep(.variables-table-wrapper) {
    padding: 0 24px;
    max-height: calc(100% - 68px);
    overflow: auto;
  }
  .action-btns {
    padding: 8px 24px;
    background: #ffffff;
    border-top: 1px solid #dcdee5;
    .bk-button {
      margin-right: 8px;
      min-width: 88px;
    }
  }
</style>
