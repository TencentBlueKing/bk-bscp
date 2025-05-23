<template>
  <section class="version-detail-table">
    <div class="service-selector-wrapper">
      <ServiceSelector :value="props.appId" />
    </div>
    <div class="content-container">
      <div class="head-operate-wrapper">
        <div class="type-tabs">
          <div :class="['tab-item', { active: currentTab === 'avaliable' }]" @click="handleTabChange('avaliable')">
            {{ t('可用版本') }}
          </div>
          <div class="split-line"></div>
          <div :class="['tab-item', { active: currentTab === 'deprecate' }]" @click="handleTabChange('deprecate')">
            {{ t('废弃版本') }}
          </div>
        </div>
        <SearchInput
          v-model="searchStr"
          class="version-search-input"
          :placeholder="t('版本名称/版本描述/修改人')"
          :width="320"
          @search="handleSearch" />
      </div>
      <bk-loading :loading="listLoading">
        <bk-table
          :border="['outer']"
          :data="versionList"
          :row-class="getRowCls"
          :remote-pagination="true"
          :pagination="pagination"
          show-overflow-tooltip
          @row-click="handleSelectVersion"
          @page-limit-change="handlePageLimitChange"
          @page-value-change="refreshVersionList($event)">
          <bk-table-column :label="t('版本')" prop="spec.name" show-overflow-tooltip></bk-table-column>
          <bk-table-column :label="t('版本描述')" prop="spec.memo" show-overflow-tooltip>
            <template #default="{ row }">
              {{ row.spec?.memo || '--' }}
            </template>
          </bk-table-column>
          <bk-table-column :label="t('已上线分组')" show-overflow-tooltip>
            <template #default="{ row }">
              <template v-if="row.status">
                <template v-if="row.status.publish_status !== 'partial_released'">{{ getGroupNames(row) }}</template>
                <ReleasedGroupViewer
                  v-else
                  placement="bottom-start"
                  :bk-biz-id="props.bkBizId"
                  :app-id="props.appId"
                  :groups="row.status.released_groups">
                  <div>{{ getGroupNames(row) }}</div>
                </ReleasedGroupViewer>
              </template>
            </template>
          </bk-table-column>
          <bk-table-column :label="t('创建人')">
            <template #default="{ row }">
              {{ row.revision?.creator || '--' }}
            </template>
          </bk-table-column>
          <bk-table-column :label="t('生成时间')" width="220">
            <template #default="{ row }">
              <span v-if="row.revision">{{
                row.revision.create_at ? datetimeFormat(row.revision.create_at) : '--'
              }}</span>
            </template>
          </bk-table-column>
          <bk-table-column :label="t('状态')">
            <template #default="{ row }">
              <div v-if="row.spec && row.spec.deprecated" class="status-tag deprecated">{{ t('已废弃') }}</div>
              <template v-else-if="row.status">
                <template v-if="!VERSION_STATUS_MAP[row.status.publish_status as keyof typeof VERSION_STATUS_MAP]">
                  --
                </template>
                <div v-else :class="['status-tag', row.status.publish_status]">
                  {{ row.status.publish_status === 'not_released' ? t('未上线') : t('已上线') }}
                </div>
              </template>
            </template>
          </bk-table-column>
          <bk-table-column :label="t('操作')" :width="locale === 'zh-cn' ? '200' : '270'">
            <template #default="{ row }">
              <template v-if="row.status">
                <template v-if="currentTab === 'avaliable'">
                  <template v-if="row.status.publish_status === 'editing'">--</template>
                  <div v-else class="actions-wrapper">
                    <bk-button text theme="primary" @click.stop="handleOpenDiff(row)">
                      {{ t('版本对比') }}
                    </bk-button>
                    <bk-button
                      v-bk-tooltips="{
                        disabled:
                          row.status.publish_status === 'not_released' ||
                          row.status.strategy_status !== 'pending_approval' ||
                          row.status.publish_status !== 'pending_publish',
                        placement: 'bottom',
                        content: t('只支持未上线和未待审批版本'),
                      }"
                      text
                      theme="primary"
                      :disabled="
                        row.status.publish_status !== 'not_released' ||
                        row.status.strategy_status === 'pending_approval' ||
                        row.status.publish_status === 'pending_publish'
                      "
                      @click.stop="handleDeprecate(row)">
                      {{ t('版本废弃') }}
                    </bk-button>
                  </div>
                </template>
                <div v-else class="actions-wrapper">
                  <bk-button text theme="primary" @click.stop="handleUndeprecate(row)">{{ t('恢复') }}</bk-button>
                  <bk-button text theme="primary" @click.stop="handleDelete(row)">{{ t('删除') }}</bk-button>
                </div>
              </template>
            </template>
          </bk-table-column>
          <template #empty>
            <tableEmpty :is-search-empty="isSearchEmpty" @clear="handleClearSearchStr"></tableEmpty>
          </template>
        </bk-table>
      </bk-loading>
    </div>
    <VersionDiff v-model:show="showDiffPanel" :current-version="diffVersion" />
    <VersionOperateConfirmDialog
      v-model:show="operateConfirmDialog.open"
      :title="operateConfirmDialog.title"
      :tips="operateConfirmDialog.tips"
      :confirm-fn="operateConfirmDialog.confirmFn"
      :version="operateConfirmDialog.version" />
  </section>
</template>
<script setup lang="ts">
  import { ref, computed, watch, onMounted } from 'vue';
  import { useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { Message } from 'bkui-vue';
  import useConfigStore from '../../../../../../store/config';
  import {
    getConfigVersionList,
    deprecateVersion,
    undeprecateVersion,
    deleteVersion,
  } from '../../../../../../api/config';
  import { datetimeFormat } from '../../../../../../utils/index';
  import { VERSION_STATUS_MAP, GET_UNNAMED_VERSION_DATA } from '../../../../../../constants/config';
  import { IConfigVersion, IConfigVersionQueryParams } from '../../../../../../../types/config';
  import useTablePagination from '../../../../../../utils/hooks/use-table-pagination';
  import ServiceSelector from '../../components/service-selector.vue';
  import SearchInput from '../../../../../../components/search-input.vue';
  import VersionDiff from '../../config/components/version-diff/index.vue';
  import tableEmpty from '../../../../../../components/table/table-empty.vue';
  import ReleasedGroupViewer from '../components/released-group-viewer.vue';
  import VersionOperateConfirmDialog from './version-operate-confirm-dialog.vue';

  const configStore = useConfigStore();
  const { versionData } = storeToRefs(configStore);

  const router = useRouter();
  const { t, locale } = useI18n();
  const { pagination, updatePagination } = useTablePagination('serviceVersionTableList');

  const props = defineProps<{
    bkBizId: string;
    appId: number;
  }>();

  const UN_NAMED_VERSION = GET_UNNAMED_VERSION_DATA();

  const listLoading = ref(true);
  const versionList = ref<Array<IConfigVersion>>([]);
  const currentTab = ref('avaliable');
  const searchStr = ref('');
  const showDiffPanel = ref(false);
  const diffVersion = ref();
  const isSearchEmpty = ref(false);
  const operateConfirmDialog = ref({
    open: false,
    version: UN_NAMED_VERSION,
    title: '',
    tips: '',
    confirmFn: () => {},
  });

  // 可用版本非搜索查看视图
  const isAvaliableView = computed(() => currentTab.value === 'avaliable' && searchStr.value === '');

  watch(
    () => props.appId,
    () => {
      getVersionList();
    },
  );

  onMounted(() => {
    getVersionList();
  });

  const getVersionList = async () => {
    listLoading.value = true;
    const { current, limit } = pagination.value;
    const notFirstPageStart = isAvaliableView.value ? (current - 1) * limit - 1 : (current - 1) * limit;
    const params: IConfigVersionQueryParams = {
      start: current === 1 ? 0 : notFirstPageStart,
      limit: current === 1 && isAvaliableView.value ? limit - 1 : limit,
      deprecated: currentTab.value !== 'avaliable',
    };
    if (searchStr.value) {
      params.searchKey = searchStr.value;
    }
    const res = await getConfigVersionList(props.bkBizId, props.appId, params);
    const count = isAvaliableView.value ? res.data.count + 1 : res.data.count;
    if (isAvaliableView.value && current === 1) {
      versionList.value = [UN_NAMED_VERSION, ...res.data.details];
    } else {
      versionList.value = res.data.details;
    }
    pagination.value.count = count;
    listLoading.value = false;
  };

  const getRowCls = (data: IConfigVersion) => {
    if (data.id === versionData.value.id) {
      return 'selected';
    }
    return '';
  };

  const getGroupNames = (data: IConfigVersion) => {
    const status = data.status?.publish_status;
    if (status === 'partial_released') {
      return data.status.released_groups.map((item) => item.name).join('; ');
    }
    if (status === 'full_released') {
      return t('全部实例');
    }
    return '--';
  };

  const handleTabChange = (tab: string) => {
    currentTab.value = tab;
    pagination.value.current = 1;
    refreshVersionList();
  };

  // 选择某个版本
  const handleSelectVersion = (event: Event | undefined, data: IConfigVersion) => {
    configStore.$patch((state) => {
      state.versionData = data;
    });
    const params: { spaceId: string; appId: number; versionId?: number } = {
      spaceId: props.bkBizId,
      appId: props.appId,
    };
    if (data.id !== 0) {
      params.versionId = data.id;
    }
    router.push({ name: 'service-config', params });
  };

  // 打开版本对比
  const handleOpenDiff = (version: IConfigVersion) => {
    showDiffPanel.value = true;
    diffVersion.value = version;
  };

  // 废弃
  const handleDeprecate = (version: IConfigVersion) => {
    operateConfirmDialog.value.open = true;
    operateConfirmDialog.value.title = t('确认废弃该版本');
    operateConfirmDialog.value.tips = t('此操作不会删除版本，如需找回或彻底删除请去版本详情的废弃版本列表操作');
    operateConfirmDialog.value.version = version;
    operateConfirmDialog.value.confirmFn = () =>
      new Promise(() => {
        deprecateVersion(props.bkBizId, props.appId, version.id).then(() => {
          operateConfirmDialog.value.open = false;
          Message({
            theme: 'success',
            message: t('版本废弃成功'),
          });
          updateListAndSetVersionAfterOperate(version.id);
        });
      });
  };

  // 恢复
  const handleUndeprecate = (version: IConfigVersion) => {
    operateConfirmDialog.value.open = true;
    operateConfirmDialog.value.title = t('确认恢复该版本');
    operateConfirmDialog.value.tips = t('此操作会把改版本恢复至可用版本列表');
    operateConfirmDialog.value.version = version;
    operateConfirmDialog.value.confirmFn = () =>
      new Promise(() => {
        undeprecateVersion(props.bkBizId, props.appId, version.id).then(() => {
          operateConfirmDialog.value.open = false;
          Message({
            theme: 'success',
            message: t('版本恢复成功'),
          });
          updateListAndSetVersionAfterOperate(version.id);
        });
      });
  };

  // 删除
  const handleDelete = (version: IConfigVersion) => {
    operateConfirmDialog.value.open = true;
    operateConfirmDialog.value.title = t('确认删除该版本');
    operateConfirmDialog.value.tips = t('一旦删除，该操作将无法撤销，请谨慎操作');
    operateConfirmDialog.value.version = version;
    operateConfirmDialog.value.confirmFn = () =>
      new Promise(() => {
        deleteVersion(props.bkBizId, props.appId, version.id).then(() => {
          operateConfirmDialog.value.open = false;
          Message({
            theme: 'success',
            message: t('版本删除成功'),
          });
          updateListAndSetVersionAfterOperate(version.id);
        });
      });
  };

  // 更新列表数据以及设置选中版本
  const updateListAndSetVersionAfterOperate = async (id: number) => {
    const index = versionList.value.findIndex((item) => item.id === id);
    const currentPage = pagination.value.current;
    pagination.value.current = versionList.value.length === 1 && currentPage > 1 ? currentPage - 1 : currentPage;
    await getVersionList();
    if (id === versionData.value.id) {
      const len = versionList.value.length;
      if (len > 0) {
        const version = len - 1 >= index ? versionList.value[index] : versionList.value[len - 1];
        handleSelectVersion(undefined, version);
      } else {
        handleSelectVersion(undefined, UN_NAMED_VERSION);
      }
    }
  };

  const handlePageLimitChange = (limit: number) => {
    updatePagination('limit', limit);
    refreshVersionList();
  };

  const refreshVersionList = (current = 1) => {
    pagination.value.current = current;
    getVersionList();
  };

  const handleSearch = () => {
    isSearchEmpty.value = true;
    refreshVersionList();
  };

  const handleClearSearchStr = () => {
    searchStr.value = '';
    isSearchEmpty.value = false;
    refreshVersionList();
  };
</script>
<style lang="scss" scoped>
  .version-detail-table {
    height: 100%;
    background: #ffffff;
  }
  .service-selector-wrapper {
    padding: 10px 24px;
    border-bottom: 1px solid #eaebf0;
    :deep(.service-selector) {
      width: 264px;
    }
  }
  .content-container {
    padding: 12px 24px;
    height: calc(100% - 53px);
    overflow: auto;
  }
  .head-operate-wrapper {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
  }
  .type-tabs {
    display: flex;
    align-items: center;
    padding: 3px 4px;
    background: #f0f1f5;
    border-radius: 4px;
    .tab-item {
      padding: 6px 14px;
      font-size: 12px;
      line-height: 14px;
      color: #63656e;
      border-radius: 4px;
      cursor: pointer;
      &.active {
        color: #3a84ff;
        background: #ffffff;
      }
    }
    .split-line {
      margin: 0 4px;
      width: 1px;
      height: 14px;
      background: #dcdee5;
    }
  }
  .bk-table {
    :deep(.bk-table-body) {
      tr {
        cursor: pointer;
        &.selected td {
          background: #e1ecff !important;
        }
      }
    }
  }
  .status-tag {
    display: inline-block;
    padding: 0 10px;
    line-height: 20px;
    font-size: 12px;
    border: 1px solid #cccccc;
    border-radius: 11px;
    text-align: center;
    &.deprecated {
      color: #ea3536;
      background-color: #feebea;
      border-color: #ea35364d;
    }
    &.not_released {
      color: #fe9000;
      background: #ffe8c3;
      border-color: rgba(254, 156, 0, 0.3);
    }
    &.full_released,
    &.partial_released {
      color: #14a568;
      background: #e4faf0;
      border-color: rgba(20, 165, 104, 0.3);
    }
  }
  .actions-wrapper {
    .bk-button:not(:first-child) {
      margin-left: 8px;
    }
  }
  .header-wrapper {
    display: flex;
    align-items: center;
    padding: 0 24px;
    height: 100%;
    font-size: 12px;
    line-height: 1;
  }
  .header-name {
    display: flex;
    align-items: center;
    font-size: 12px;
    color: #3a84ff;
    cursor: pointer;
  }
  .arrow-left {
    font-size: 26px;
    color: #3884ff;
  }
  .arrow-right {
    font-size: 24px;
    color: #c4c6cc;
  }
  .diff-left-panel-head {
    padding: 0 24px;
    font-size: 12px;
  }
</style>
