<template>
  <bk-alert v-if="isFileType && conflictFileCount > 0" theme="warning">
    <template #title>
      <span>
        {{
          t('模板套餐导入完成，存在 {n} 个冲突配置项，请修改配置项信息或删除对应模板套餐，否则无法生成版本。', {
            n: conflictFileCount,
          })
        }}
        <bk-button theme="primary" text @click="handleToggleViewConfig">
          {{ onlyViewConflict ? t('查看全部配置项') : t('只看冲突配置项') }}
        </bk-button>
      </span>
    </template>
  </bk-alert>
  <section :class="['config-list-wrapper', { 'has-conflict': isFileType && conflictFileCount > 0 }]">
    <div class="operate-area">
      <div class="operate-btns">
        <template v-if="versionData.status.publish_status === 'editing'">
          <CreateConfig
            :bk-biz-id="props.bkBizId"
            :app-id="props.appId"
            @created="refreshConfigList(true)"
            @imported="handleImported" />
          <CountTips
            :max="spaceFeatureFlags.RESOURCE_LIMIT.AppConfigCnt"
            :current="allExistConfigCount"
            :is-temp="false"
            :is-file-type="isFileType" />
          <EditVariables v-if="isFileType" ref="editVariablesRef" :bk-biz-id="props.bkBizId" :app-id="props.appId" />
        </template>
        <ViewVariables
          v-else-if="isFileType"
          :bk-biz-id="props.bkBizId"
          :app-id="props.appId"
          :verision-id="versionData.id" />
        <ConfigExport
          :bk-biz-id="props.bkBizId"
          :app-id="props.appId"
          :version-id="versionData.id"
          :version-name="versionData.spec.name" />
        <BatchOperationBtn
          v-if="versionData.status.publish_status === 'editing'"
          :bk-biz-id="props.bkBizId"
          :app-id="props.appId"
          :selected-ids="selectedIds"
          :is-file-type="isFileType"
          :selected-items="selectedItems"
          :is-across-checked="isAcrossChecked"
          :selected-keys="selectedKeys"
          :data-count="selecTableDataCount"
          :selected-delete-count="selectedDeleteCount"
          :selected-exist-count="selectedExistCount"
          @deleted="handleBatchDeleted" />
      </div>
      <SearchSelector
        ref="searchSelectorRef"
        class="config-search-input"
        :search-filed="searchFiled"
        :user-filed="['reviser']"
        :placeholder="searchPlaceholder"
        @search="searchQuery = $event" />
    </div>
    <section class="config-list-table">
      <TableWithTemplates
        v-if="isFileType"
        ref="tableRef"
        :bk-biz-id="props.bkBizId"
        :app-id="props.appId"
        :search-query="searchQuery"
        @clear-str="handleClearsearchQuery"
        @delete-config="refreshVariable"
        @update-selected-items="handleTmpSelectedItems" />
      <TableWithKv
        v-else
        ref="tableRef"
        :bk-biz-id="props.bkBizId"
        :app-id="props.appId"
        :search-query="searchQuery"
        @send-table-data-count="selecTableDataCount = $event"
        @clear-str="handleClearsearchQuery"
        @update-selected-items="
          (data) => {
            selectedKeys = data.selectedConfigKeys;
            selectedIds = data.selectedConfigIds;
            isAcrossChecked = data.isAcrossChecked;
            selectedExistCount = data.selectedExistCount;
            selectedDeleteCount = data.selectedDeleteCount;
          }
        " />
    </section>
  </section>
</template>
<script setup lang="ts">
  import { ref, computed, watch } from 'vue';
  import { storeToRefs } from 'pinia';
  import { useI18n } from 'vue-i18n';
  import useConfigStore from '../../../../../../../store/config';
  import useServiceStore from '../../../../../../../store/service';
  import useGlobalStore from '../../../../../../../store/global';
  import CreateConfig from './create-config/index.vue';
  import EditVariables from './variables/edit-variables.vue';
  import ViewVariables from './variables/view-variables.vue';
  import TableWithTemplates from './tables/table-with-templates.vue';
  import TableWithKv from './tables/table-with-kv.vue';
  import ConfigExport from './config-export.vue';
  import BatchOperationBtn from './batch-operation-btn.vue';
  import CountTips from '../../components/count-tips.vue';
  import SearchSelector from '../../../../../../../components/search-selector.vue';

  const configStore = useConfigStore();
  const serviceStore = useServiceStore();
  const { versionData, conflictFileCount, onlyViewConflict, allExistConfigCount } = storeToRefs(configStore);
  const { spaceFeatureFlags } = storeToRefs(useGlobalStore());
  const { isFileType } = storeToRefs(serviceStore);
  const { t } = useI18n();

  const props = defineProps<{
    bkBizId: string;
    appId: number;
  }>();

  const tableRef = ref();
  const editVariablesRef = ref();
  const selectedIds = ref<number[]>([]);
  const selectedItems = ref<any[]>([]);
  const isAcrossChecked = ref(false);
  const selecTableDataCount = ref(0);
  const selectedKeys = ref<string[]>([]);
  const selectedExistCount = ref(0); // 选中的删除项个数
  const selectedDeleteCount = ref(0); // 选中的恢复项个数

  const searchQuery = ref<{ [key: string]: string }>({});
  const searchSelectorRef = ref();

  const searchFiled = computed(() => {
    if (isFileType.value) {
      return [
        { field: 'path_name', label: t('配置文件名') },
        { field: 'creator', label: t('创建人') },
        { field: 'reviser', label: t('修改人') },
      ];
    }
    return [
      { field: 'key', label: t('配置项名称') },
      { field: 'creator', label: t('创建人') },
      { field: 'reviser', label: t('修改人') },
    ];
  });

  const searchPlaceholder = computed(() => {
    if (isFileType.value) {
      return t('配置文件名/创建人/修改人');
    }
    return t('配置项名称/创建人/修改人');
  });

  watch(
    () => versionData.value.id,
    () => {
      handleClearsearchQuery();
    },
  );

  const refreshConfigList = (createConfig = false) => {
    if (isFileType.value) {
      tableRef.value.refresh(createConfig);
      refreshVariable();
    } else {
      tableRef.value.refresh(1, false, createConfig);
    }
  };

  const handleImported = () => {
    if (isFileType.value) {
      tableRef.value.refreshBindingId();
    }
    refreshConfigList(true);
  };

  const refreshVariable = () => {
    editVariablesRef.value.getVariableList();
  };

  const handleClearsearchQuery = () => {
    searchQuery.value = {};
    searchSelectorRef.value.clear();
  };

  // 批量删除配置项回调
  const handleBatchDeleted = () => {
    tableRef.value.refreshAfterBatchSet();
  };

  const handleToggleViewConfig = () => {
    configStore.$patch((state) => {
      state.onlyViewConflict = !state.onlyViewConflict;
    });
  };

  const handleTmpSelectedItems = (items: any[]) => {
    selectedItems.value = items;
    selectedIds.value = items.map((item) => item.id);
    selectedExistCount.value = items.filter((item) => item.file_state !== 'DELETE').length;
    selectedDeleteCount.value = selectedIds.value.length - selectedExistCount.value;
  };

  defineExpose({
    refreshConfigList,
  });
</script>
<style lang="scss" scoped>
  .config-list-wrapper {
    position: relative;
    padding: 0 24px 24px 24px;
    height: 100%;
    &.has-conflict {
      height: calc(100% - 34px);
    }
  }
  .operate-area {
    display: flex;
    align-items: center;
    padding: 16px 0;
    .operate-btns {
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .config-search-input {
      width: 280px;
      margin-left: auto;
    }
  }
  .config-list-table {
    height: calc(100% - 64px);
  }
</style>
