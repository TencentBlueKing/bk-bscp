<template>
  <bk-loading :loading="loading">
    <bk-table
      class="config-table"
      :border="['outer']"
      :data="configList"
      :remote-pagination="true"
      :pagination="pagination"
      :key="versionData.id"
      selection-key="id"
      row-key="id"
      :row-class="getRowCls"
      show-overflow-tooltip
      @page-limit-change="handlePageLimitChange"
      @page-value-change="refresh($event, true, true)"
      @column-sort="handleSort"
      @column-filter="handleFilter">
      <template #prepend v-if="versionData.id === 0">
        <render-table-tip />
      </template>
      <bk-table-column
        v-if="versionData.id === 0"
        :width="74"
        :min-width="74"
        :label="renderSelection"
        :show-overflow-tooltip="false">
        <template #default="{ row }">
          <across-check-box :checked="isChecked(row)" :handle-change="() => handleSelectionChange(row)" />
        </template>
      </bk-table-column>
      <bk-table-column :label="t('配置项名称')" prop="spec.key" :min-width="240">
        <template #default="{ row }">
          <div v-if="row.spec" class="config-name">
            <bk-overflow-title
              :disabled="row.kv_state === 'DELETE'"
              type="tips"
              class="key-name"
              @click="handleView(row)">
              {{ row.spec.key }}
            </bk-overflow-title>
            <exclamation-circle-shape
              v-if="row.spec.secret_type === 'certificate' && row.spec.certificate_info?.remainingDays < 30"
              :class="['warn-icon', { error: row.spec?.certificate_info?.isExpiration }]"
              v-bk-tooltips="{ content: row.spec?.certificate_info?.tooltips }" />
          </div>
        </template>
      </bk-table-column>
      <bk-table-column :label="t('配置项值预览')" prop="spec.value">
        <template #default="{ row }">
          <kvValuePreview
            v-if="row.spec"
            :is-visible="!row.spec.secret_hidden"
            :key="row.id"
            :value="row.spec.value"
            :type="row.spec.kv_type"
            :is-kv-value="true"
            @view-all="handleView(row)" />
        </template>
      </bk-table-column>
      <bk-table-column :label="t('配置项描述')">
        <template #default="{ row }">
          <span v-if="row.spec">{{ row.spec.memo || '--' }}</span>
        </template>
      </bk-table-column>
      <bk-table-column
        prop="spec.kv_type"
        :label="t('数据类型')"
        :filter="{ filterFn: () => true, list: typeFilterList, checked: typeFilterChecked }"
        :width="120">
        <template #default="{ row }">
          <span v-if="row.spec">{{ row.spec.kv_type === 'secret' ? t('敏感信息') : row.spec.kv_type }}</span>
        </template>
      </bk-table-column>
      <bk-table-column :label="t('创建人')" prop="revision.creator" :width="150"></bk-table-column>
      <bk-table-column :label="t('修改人')" prop="revision.reviser" :width="150"></bk-table-column>
      <bk-table-column :label="t('修改时间')" :sort="true" :width="180">
        <template #default="{ row }">
          <span v-if="row.revision">{{ datetimeFormat(row.revision.update_at) }}</span>
        </template>
      </bk-table-column>
      <bk-table-column
        v-if="versionData.id === 0"
        :label="t('变更状态')"
        :filter="{ filterFn: () => true, list: statusFilterList, checked: statusFilterChecked }"
        :width="140">
        <template #default="{ row }">
          <StatusTag :status="row.kv_state" />
        </template>
      </bk-table-column>
      <bk-table-column :label="t('操作')" fixed="right" :width="220">
        <template #default="{ row }">
          <div class="operate-action-btns">
            <bk-button
              v-if="row.kv_state === 'DELETE'"
              v-cursor="{ active: !hasEditServicePerm }"
              :class="{ 'bk-text-with-no-perm': !hasEditServicePerm }"
              :disabled="!hasEditServicePerm"
              text
              theme="primary"
              @click="handleUndelete(row)">
              {{ t('恢复') }}
            </bk-button>
            <template v-else>
              <bk-button
                v-cursor="{ active: !hasEditServicePerm }"
                :class="{ 'bk-text-with-no-perm': versionData.id === 0 && !hasEditServicePerm }"
                :disabled="versionData.id === 0 && !hasEditServicePerm"
                text
                theme="primary"
                @click="handleEditOrView(row)">
                {{ versionData.id === 0 ? t('编辑') : t('查看') }}
              </bk-button>
              <bk-button
                v-cursor="{ active: !hasEditServicePerm }"
                v-if="row.kv_state === 'REVISE'"
                :class="{ 'bk-text-with-no-perm': !hasEditServicePerm }"
                :disabled="!hasEditServicePerm"
                text
                theme="primary"
                @click="handleUnModify(row)">
                {{ t('撤销') }}
              </bk-button>
              <bk-button
                v-if="versionData.status.publish_status !== 'editing'"
                text
                theme="primary"
                @click="handleDiff(row)">
                {{ t('对比') }}
              </bk-button>
              <bk-button
                v-cursor="{ active: !hasEditServicePerm }"
                v-if="versionData.id === 0"
                :class="{ 'bk-text-with-no-perm': !hasEditServicePerm }"
                :disabled="!hasEditServicePerm"
                text
                theme="primary"
                @click="handleDel(row)">
                {{ t('删除') }}
              </bk-button>
            </template>
          </div>
        </template>
      </bk-table-column>
      <template #empty>
        <TableEmpty :is-search-empty="isSearchEmpty" @clear="emits('clearStr')" style="width: 100%" />
      </template>
    </bk-table>
  </bk-loading>
  <edit-config
    v-model:show="editPanelShow"
    :config="activeConfig.spec as IConfigKvItem"
    :bk-biz-id="props.bkBizId"
    :app-id="props.appId"
    :editable="true"
    @confirm="getListData" />
  <ViewConfigKv
    v-model:show="viewPanelShow"
    :config="activeConfig"
    :show-edit-btn="isUnNamedVersion"
    @open-edit="handleSwitchToEdit" />
  <VersionDiff v-model:show="isDiffPanelShow" :current-version="versionData" :selected-kv-config-id="diffConfig" />
  <DeleteConfirmDialog
    v-model:is-show="isDeleteConfigDialogShow"
    :title="t('确认删除该配置项？')"
    @confirm="handleDeleteConfigConfirm">
    <div style="margin-bottom: 8px">
      {{ t('配置项') }}：<span style="color: #313238">{{ deleteConfig?.spec.key }}</span>
    </div>
    <div>{{ deleteConfigTips }}</div>
  </DeleteConfirmDialog>
  <DeleteConfirmDialog
    v-model:is-show="isRecoverConfigDialogShow"
    :title="t('确认恢复该配置项?')"
    :confirm-text="t('恢复')"
    @confirm="handleRecoverConfigConfirm">
    <div style="margin-bottom: 8px">
      {{ t('配置项') }}：<span style="color: #313238">{{ recoverConfig?.spec.key }}</span>
    </div>
    <div>{{ t(`配置项恢复后，将覆盖新添加的配置项`) + recoverConfig?.spec.key }}</div>
  </DeleteConfirmDialog>
</template>
<script lang="ts" setup>
  import { ref, watch, onMounted, computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import Message from 'bkui-vue/lib/message';
  import useConfigStore from '../../../../../../../../store/config';
  import useServiceStore from '../../../../../../../../store/service';
  import { ICommonQuery } from '../../../../../../../../../types/index';
  import { IConfigKvItem, IConfigKvType } from '../../../../../../../../../types/config';
  import { getKvList, deleteKv, getReleaseKvList, undeleteKv, unModifyKv } from '../../../../../../../../api/config';
  import useTablePagination from '../../../../../../../../utils/hooks/use-table-pagination';
  import { datetimeFormat } from '../../../../../../../../utils/index';
  import { getDefaultKvItem } from '../../../../../../../../utils/config';
  import { CONFIG_KV_TYPE } from '../../../../../../../../constants/config';
  import { ExclamationCircleShape } from 'bkui-vue/lib/icon';
  import StatusTag from './status-tag';
  import EditConfig from '../edit-config-kv.vue';
  import ViewConfigKv from '../view-config-kv.vue';
  import kvValuePreview from './kv-value-preview.vue';
  import VersionDiff from '../../../components/version-diff/index.vue';
  import TableEmpty from '../../../../../../../../components/table/table-empty.vue';
  import DeleteConfirmDialog from '../../../../../../../../components/delete-confirm-dialog.vue';
  import useTableAcrossCheck from '../../../../../../../../utils/hooks/use-table-acrosscheck';
  import acrossCheckBox from '../../../../../../../../components/across-checkbox.vue';
  import CheckType from '../../../../../../../../../types/across-checked';
  import dayjs from 'dayjs';

  const configStore = useConfigStore();
  const serviceStore = useServiceStore();
  const { versionData } = storeToRefs(configStore);
  const { checkPermBeforeOperate } = serviceStore;
  const { permCheckLoading, hasEditServicePerm, topIds } = storeToRefs(serviceStore);
  const { t } = useI18n();
  const { pagination, updatePagination } = useTablePagination('tableWithKv');

  const props = defineProps<{
    bkBizId: string;
    appId: number;
    searchStr: string;
  }>();

  const emits = defineEmits(['clearStr', 'sendTableDataCount', 'updateSelectedItems']);

  const loading = ref(false);
  const configList = ref<IConfigKvType[]>([]);
  const configsCount = ref(0);
  const editPanelShow = ref(false);
  const viewPanelShow = ref(false);
  const activeConfig = ref<IConfigKvType>(getDefaultKvItem());
  const deleteConfig = ref<IConfigKvType>();
  const selectedConfigIds = ref<number[]>([]);
  const selectedConfigKeys = ref<string[]>([]);
  const isDiffPanelShow = ref(false);
  const diffConfig = ref(0);
  const isSearchEmpty = ref(false);
  const isDeleteConfigDialogShow = ref(false);
  const typeFilterChecked = ref<string[]>([]);
  const statusFilterChecked = ref<string[]>([]);
  const updateSortType = ref('null');
  const recoverConfig = ref<IConfigKvType>();
  const isRecoverConfigDialogShow = ref(false);
  const isAcrossChecked = ref(false);
  const selecTableDataCount = ref(0);
  const allDeleteConfigCount = ref(0);
  const allExistConfigCount = ref(0);

  const typeFilterList = computed(() =>
    CONFIG_KV_TYPE.map((item) => ({
      value: item.id,
      text: item.name,
    })),
  );
  const statusFilterList = computed(() => {
    return [
      {
        value: 'ADD',
        text: t('新增'),
      },
      {
        value: 'REVISE',
        text: t('修改'),
      },
      {
        value: 'DELETE',
        text: t('删除'),
      },
      {
        value: 'UNCHANGE',
        text: t('无修改'),
      },
    ];
  });

  const deleteConfigTips = computed(() => {
    if (deleteConfig.value) {
      return deleteConfig.value.kv_state === 'ADD'
        ? t('一旦删除，该操作将无法撤销，请谨慎操作')
        : t('配置项删除后，可以通过恢复按钮撤销删除');
    }
    return '';
  });

  // 跨页全选
  const crossPageSelect = computed(
    () => pagination.value.limit < pagination.value.count && selecTableDataCount.value !== 0,
  );
  const { selectType, selections, renderSelection, renderTableTip, handleRowCheckChange, handleClearSelection } =
    useTableAcrossCheck({
      dataCount: selecTableDataCount, // 总数
      curPageData: configList, // 当前页数据
      rowKey: ['id'],
      crossPageSelect, // 是否提供跨页全选功能
    });

  watch(
    () => versionData.value.id,
    () => {
      refresh();
    },
  );

  watch(
    () => props.searchStr,
    () => {
      props.searchStr ? (isSearchEmpty.value = true) : (isSearchEmpty.value = false);
      refresh();
    },
  );

  watch(
    () => configList.value,
    () => {
      configStore.$patch((state) => {
        state.allConfigCount = configsCount.value;
      });
    },
    { immediate: true, deep: true },
  );

  watch(
    selections,
    () => {
      isAcrossChecked.value = [CheckType.HalfAcrossChecked, CheckType.AcrossChecked].includes(selectType.value);
      let selectedDeleteCount = 0;
      let selectedExistCount = 0;
      if (isAcrossChecked.value) {
        // 全选状态 selections为排除项
        selectedDeleteCount =
          allDeleteConfigCount.value - selections.value.filter((item) => item.kv_state === 'DELETE').length;
        selectedExistCount =
          allExistConfigCount.value - selections.value.filter((item) => item.kv_state !== 'DELETE').length;
      } else {
        // 非全选状态 selections为选中项
        selectedDeleteCount = selections.value.filter((item) => item.kv_state === 'DELETE').length;
        selectedExistCount = selections.value.filter((item) => item.kv_state !== 'DELETE').length;
      }
      selectedConfigIds.value = selections.value.map((item) => item.id);
      selectedConfigKeys.value = selections.value.map((item) => item.spec.key);
      emits('updateSelectedItems', {
        selectedConfigIds: selectedConfigIds.value,
        selectedConfigKeys: selectedConfigKeys.value,
        isAcrossChecked: isAcrossChecked.value,
        selectedDeleteCount,
        selectedExistCount,
      });
    },
    {
      deep: true,
    },
  );

  const isUnNamedVersion = computed(() => versionData.value.id === 0);

  onMounted(() => {
    getListData();
  });

  const getListData = async (createConfig = false) => {
    loading.value = true;
    try {
      const params: ICommonQuery = {
        start: (pagination.value.current - 1) * pagination.value.limit,
        limit: pagination.value.limit,
        with_status: true,
      };
      if (!createConfig) {
        serviceStore.$patch((state) => {
          state.topIds = [];
        });
      }
      if (props.searchStr) {
        params.search_fields = 'key,revister,creator';
        params.search_key = props.searchStr;
      }
      if (typeFilterChecked.value!.length > 0) {
        params.kv_type = typeFilterChecked.value;
      }

      if (updateSortType.value !== 'null') {
        params.sort = 'updated_at';
        params.order = updateSortType.value.toUpperCase();
      }
      let res;
      if (isUnNamedVersion.value) {
        if (topIds.value.length > 0) params.top_ids = topIds.value;
        if (statusFilterChecked.value!.length > 0) {
          params.status = statusFilterChecked.value;
        }
        res = await getKvList(props.bkBizId, props.appId, params);
      } else {
        res = await getReleaseKvList(props.bkBizId, props.appId, versionData.value.id, params);
      }
      configList.value = res.details.sort((a: IConfigKvType, b: IConfigKvType) => {
        if (a.kv_state === 'DELETE' && b.kv_state !== 'DELETE') {
          return 1;
        }
        if (a.kv_state !== 'DELETE' && b.kv_state === 'DELETE') {
          return -1;
        }
        return 0;
      });
      configList.value.forEach((item) => {
        if (item.spec.secret_type === 'certificate' && item.spec.certificate_expiration_date) {
          item.spec.certificate_info = getCertificateInfo(item.spec.certificate_expiration_date);
        }
      });
      configsCount.value = res.count;
      allExistConfigCount.value = res.exclusion_count;
      allDeleteConfigCount.value = res.count - res.exclusion_count;
      configStore.$patch((state) => {
        state.allConfigCount = res.count;
        state.allExistConfigCount = res.exclusion_count;
      });
      selecTableDataCount.value = Number(res.count);
      emits('sendTableDataCount', selecTableDataCount.value);
      pagination.value.count = res.count;
    } catch (e) {
      console.error(e);
    } finally {
      loading.value = false;
    }
  };

  const getCertificateInfo = (date: string) => {
    const expirationTime = datetimeFormat(date);
    const givenTime = dayjs(expirationTime, 'YYYY-MM-DD HH:mm:ss');
    const remainingSeconds = givenTime.diff(dayjs(), 'second');
    const remainingDays = Math.ceil(remainingSeconds / 60 / 60 / 24);
    if (remainingSeconds > 0) {
      return {
        tooltips: t('此证书将于 {n} 到期，距离到期仅剩 {m} 天', { n: expirationTime, m: remainingDays }),
        isExpiration: false,
        remainingDays,
      };
    }
    return {
      tooltips: t('此证书已于 {n} 过期，请尽快更换证书', { n: expirationTime }),
      isExpiration: true,
      remainingDays,
    };
  };

  // 选中状态
  const isChecked = (row: IConfigKvType) => {
    if (![CheckType.AcrossChecked, CheckType.HalfAcrossChecked].includes(selectType.value)) {
      // 当前页状态传递
      return selections.value.some((item) => item.id === row.id);
    }
    // 跨页状态传递
    return !selections.value.some((item) => item.id === row.id);
  };

  // 表格行选择事件
  const handleSelectionChange = (row: IConfigKvType) => {
    const isSelected = selections.value.some((item) => item.id === row.id);
    // 根据选择类型决定传递的状态
    const shouldBeChecked = isAcrossChecked.value ? isSelected : !isSelected;
    handleRowCheckChange(shouldBeChecked, row);
  };

  const handleEditOrView = (config: IConfigKvType) => {
    activeConfig.value = config;
    if (isUnNamedVersion.value) {
      if (permCheckLoading.value || !checkPermBeforeOperate('update')) {
        return;
      }
      editPanelShow.value = true;
    } else {
      viewPanelShow.value = true;
    }
  };

  const handleView = (config: IConfigKvType) => {
    activeConfig.value = config;
    viewPanelShow.value = true;
  };

  const handleDiff = (config: IConfigKvType) => {
    diffConfig.value = config.id;
    isDiffPanelShow.value = true;
  };

  const handleDel = (config: IConfigKvType) => {
    if (permCheckLoading.value || !checkPermBeforeOperate('update')) {
      return;
    }
    isDeleteConfigDialogShow.value = true;
    deleteConfig.value = config;
  };

  const handleUnModify = async (config: IConfigKvType) => {
    if (permCheckLoading.value || !checkPermBeforeOperate('update')) {
      return;
    }
    await unModifyKv(props.bkBizId, props.appId, config.spec.key);
    Message({ theme: 'success', message: t('撤销修改配置项成功') });
    refresh();
  };

  // 由查看态切换为编辑态
  const handleSwitchToEdit = () => {
    if (!permCheckLoading.value && checkPermBeforeOperate('update')) {
      editPanelShow.value = true;
      viewPanelShow.value = false;
    }
  };

  // 删除单个配置项
  const handleDeleteConfigConfirm = async () => {
    if (!deleteConfig.value) {
      return;
    }
    await deleteKv(props.bkBizId, props.appId, deleteConfig.value.id);

    // 删除的配置项如果在多选列表里，需要去掉
    const index = selectedConfigIds.value.findIndex((id) => id === deleteConfig.value?.id);
    if (index > -1) {
      selectedConfigIds.value.splice(index, 1);
    }

    // 新增的配置项被删除后，检查是否需要往前翻一页
    if (deleteConfig.value.kv_state === 'ADD') {
      if (configList.value.length === 1 && pagination.value.current > 1) {
        pagination.value.current -= 1;
      }
    }

    Message({
      theme: 'success',
      message: t('删除配置项成功'),
    });
    refresh(pagination.value.current);
    isDeleteConfigDialogShow.value = false;
  };

  // 撤销删除单个配置项
  const handleUndelete = async (config: IConfigKvType) => {
    if (permCheckLoading.value || !checkPermBeforeOperate('update')) {
      return;
    }
    recoverConfig.value = config;
    const index = configList.value.findIndex((item) => item.spec.key === config.spec.key && item.kv_state !== 'DELETE');
    if (index === -1) {
      handleRecoverConfigConfirm();
    } else {
      isRecoverConfigDialogShow.value = true;
    }
  };

  const handleRecoverConfigConfirm = async () => {
    await undeleteKv(props.bkBizId, props.appId, recoverConfig.value!.spec.key);
    Message({ theme: 'success', message: t('恢复配置项成功') });
    const index = configList.value.findIndex(
      (item) => item.spec.key === recoverConfig.value?.spec.key && item.kv_state !== 'DELETE',
    );
    if (index !== -1) {
      configList.value.splice(index, 1);
    }
    recoverConfig.value!.kv_state = 'UNCHANGE';
    isRecoverConfigDialogShow.value = false;

    const res = await getKvList(props.bkBizId, props.appId, { start: 0, all: true });
    configStore.$patch((state) => {
      state.allConfigCount = res.count;
      state.allExistConfigCount = res.exclusion_count;
    });
  };

  // 批量删除配置项后刷新配置项列表
  const refreshAfterBatchSet = () => {
    if (selectedConfigIds.value.length === configList.value.length && pagination.value.current > 1) {
      pagination.value.current -= 1;
    }

    selectedConfigIds.value = [];
    selectedConfigKeys.value = [];
    emits('updateSelectedItems', {
      selectedConfigIds: selectedConfigIds.value,
      selectedConfigKeys: selectedConfigKeys.value,
      isAcrossChecked: false,
    });
    refresh(pagination.value.current);
  };

  // page-limit
  const handlePageLimitChange = (limit: number) => {
    updatePagination('limit', limit);
    refresh();
  };

  const refresh = (current = 1, pageChange = false, createConfig = false) => {
    // 非跨页全选/半选 需要重置全选状态
    if (![CheckType.HalfAcrossChecked, CheckType.AcrossChecked].includes(selectType.value) || !pageChange) {
      handleClearSelection();
    }
    pagination.value.current = current;
    getListData(createConfig);
  };

  const handleFilter = ({ checked, index }: any) => {
    if (index === 4) {
      // 调整数据类型筛选条件
      typeFilterChecked.value = checked;
    } else {
      // 调整状态筛选条件
      statusFilterChecked.value = checked;
    }
    refresh();
  };

  const handleSort = ({ type }: any) => {
    updateSortType.value = type;
    refresh();
  };

  // 判断当前行是否是删除行
  const getRowCls = (config: IConfigKvType) => {
    if (config.kv_state === 'DELETE') return 'delete-row';
    if (topIds.value.includes(config.id)) {
      return 'new-row-marked';
    }
  };

  defineExpose({
    refresh,
    refreshAfterBatchSet,
  });
</script>
<style lang="scss" scoped>
  .operate-action-btns {
    .bk-button:not(:last-of-type) {
      margin-right: 8px;
    }
  }
  .config-table {
    .config-name {
      display: flex;
      align-items: center;
      gap: 8px;
      .key-name {
        color: #3a84ff;
        max-width: calc(100% - 20px);
        cursor: pointer;
      }
      .warn-icon {
        font-size: 14px;
        color: #ff9c01;
        &.error {
          color: #ea3636;
        }
      }
    }
    :deep(.bk-table-body) {
      max-height: calc(100vh - 280px);
      overflow: auto;
      tr.delete-row td {
        background: #fafbfd !important;
        .cell {
          color: #c4c6cc !important;
        }
      }
      tr.new-row-marked td {
        background: #f2fff4 !important;
      }
    }
  }
</style>
