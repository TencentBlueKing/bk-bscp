<template>
  <div class="status-and-screen">
    <SyncStatus :biz-id="spaceId" @refresh="handlePageChange(1)" />
    <FilterProcess ref="filterRef" :bk-biz-id="spaceId" @search="handleFilter" />
  </div>
  <div class="op-wrap">
    <BatchOpBtns :count="selectedIds.length" @click="handleBatchOpProcess" />
    <SearchSelector
      ref="searchSelectorRef"
      :search-field="searchField"
      :placeholder="t('内网IP/进程状态/托管状态/CC 同步状态')"
      class="search-select"
      @search="handleSearch" />
  </div>
  <div class="table-wrap" ref="tableRef">
    <PrimaryTable
      class="border process-table"
      :data="processList"
      row-key="id"
      :row-class-name="getRowClassName"
      :loading="tableLoading"
      :max-height="tableMaxHeight"
      :expanded-row-keys="expandedRowKeys"
      @select-change="handleSelectChange">
      <TableColumn col-key="row-select" type="multiple" width="32"></TableColumn>
      <TableColumn :title="t('集群')" col-key="spec.set_name" width="183">
        <template #default="{ row }: { row: IProcessItem }">
          <bk-button text theme="primary" @click="handleExpandRow(row)">{{ row.spec.set_name }}</bk-button>
        </template>
      </TableColumn>
      <TableColumn col-key="spec.module_name" :title="t('模块')" width="172" ellipsis />
      <TableColumn col-key="spec.service_name" :title="t('服务实例')" ellipsis />
      <TableColumn col-key="spec.alias" :title="t('进程别名')" ellipsis />
      <TableColumn col-key="attachment.cc_process_id">
        <template #title>
          <span class="tips-title" v-bk-tooltips="{ content: t('对应 CMDB 中唯一 ID'), placement: 'top' }">
            {{ t('CC 进程ID') }}
          </span>
        </template>
      </TableColumn>
      <TableColumn col-key="spec.inner_ip" :title="t('内网 IP')" />
      <TableColumn col-key="spec.status" :title="t('进程状态')">
        <template #default="{ row }: { row: IProcessItem }">
          <div v-if="row.spec.status" class="process-status">
            <Spinner v-if="['starting', 'restarting', 'reloading'].includes(row.spec.status)" class="spinner-icon" />
            <span v-else :class="['dot', row.spec.status]"></span>
            {{ PROCESS_STATUS_MAP[row.spec.status as keyof typeof PROCESS_STATUS_MAP] }}
          </div>
          <span v-else>--</span>
        </template>
      </TableColumn>
      <TableColumn col-key="spec.managed_status" :title="t('托管状态')" width="152">
        <template #default="{ row }: { row: IProcessItem }">
          <bk-tag v-if="row.spec.managed_status" :theme="getManagedStatusTheme(row.spec.managed_status)">
            <span class="process-status">
              <Spinner v-if="['starting', 'stopping'].includes(row.spec.managed_status)" class="spinner-icon" />
              {{ PROCESS_MANAGED_STATUS_MAP[row.spec.managed_status as keyof typeof PROCESS_MANAGED_STATUS_MAP] }}
            </span>
          </bk-tag>
          <span v-else>--</span>
        </template>
      </TableColumn>
      <TableColumn col-key="spec.cc_sync_updated_at" :title="t('状态获取时间')">
        <template #default="{ row }: { row: IProcessItem }">
          {{ timeAgo(row.spec.cc_sync_updated_at) }}
        </template>
      </TableColumn>
      <TableColumn col-key="spec.cc_sync_status" :title="t('CC 同步状态')">
        <template #default="{ row }: { row: IProcessItem }">
          <span :class="['cc-sync-status', row.spec.cc_sync_status]">
            {{ CC_SYNC_STATUS[row.spec.cc_sync_status as keyof typeof CC_SYNC_STATUS] }}
          </span>
        </template>
      </TableColumn>
      <TableColumn :title="t('操作')" :width="220" fixed="right" col-key="operation">
        <template #default="{ row }: { row: IProcessItem }">
          <div class="op-btns">
            <TableBtnTooltips
              v-if="row.spec.cc_sync_status === 'updated'"
              :disabled="row.spec.actions.unregister.enabled"
              :reason="row.spec.actions.unregister.reason"
              :link="cmdbUrl">
              <bk-badge position="top-right" theme="danger" dot>
                <bk-button
                  text
                  theme="primary"
                  :disabled="!row.spec.actions.unregister.enabled"
                  @click="handleUpdateManagedInfo(row)">
                  {{ t('更新托管信息') }}
                </bk-button>
              </bk-badge>
            </TableBtnTooltips>
            <TableBtnTooltips
              v-else
              v-for="action in ['start', 'stop']"
              :key="action"
              :disabled="row.spec.actions[action].enabled"
              :reason="row.spec.actions[action].reason"
              :link="cmdbUrl">
              <bk-button
                text
                theme="primary"
                :disabled="!row.spec.actions[action].enabled"
                @click="handleOpProcess(row, action)">
                {{ action === 'start' ? t('启动') : t('停止') }}
              </bk-button>
            </TableBtnTooltips>
            <bk-button text theme="primary" :disabled="!row.spec.actions.push" @click="handleConfigIssued(row)">
              {{ t('配置下发') }}
            </bk-button>
            <TableMoreAction
              :enabled="{ ...row.spec.actions, view: { enabled: true, reason: '' } }"
              :operation-list="operationList"
              @operation="handleMoreActionClick(row, $event)" />
          </div>
        </template>
      </TableColumn>
      <template #expandedRow="{ row }: { row: IProcessItem }">
        <div class="second-table">
          <PrimaryTable
            class="instance-table"
            :data="row.proc_inst"
            row-key="id"
            :row-class-name="getSecondTableRowClass">
            <TableColumn col-key="spec.host_inst_seq" :title="t('实例')">
              <template #default="{ row: rowData, rowIndex }: { row: IProcInst; rowIndex: number }">
                <div class="instance">
                  <span>{{ rowData.spec.name }}</span>
                  <span
                    v-if="rowIndex + 1 > rowData.num!"
                    class="error-icon"
                    v-bk-tooltips="{ content: t('CC 中更新了数量，已不存在这条实例记录，建议停止') }">
                    !
                  </span>
                </div>
              </template>
            </TableColumn>
            <TableColumn col-key="spec.host_inst_seq">
              <template #title>
                <span class="tips-title" v-bk-tooltips="{ content: t('主机下唯一标识'), placement: 'top' }">
                  HostInstSeq
                </span>
              </template>
            </TableColumn>
            <TableColumn col-key="spec.module_inst_seq">
              <template #title>
                <span class="tips-title" v-bk-tooltips="{ content: t('模块下唯一标识'), placement: 'top' }">
                  ModuleInstSeq
                </span>
              </template>
            </TableColumn>
            <TableColumn col-key="spec.status" :title="t('进程状态')">
              <template #default="{ row: rowData }: { row: IProcInst }">
                <div v-if="rowData.spec.status" class="process-status">
                  <Spinner
                    v-if="['starting', 'restarting', 'reloading'].includes(rowData.spec.status)"
                    class="spinner-icon" />
                  <span v-else :class="['dot', rowData.spec.status]"></span>
                  {{ PROCESS_STATUS_MAP[rowData.spec.status as keyof typeof PROCESS_STATUS_MAP] }}
                </div>
                <span v-else>--</span>
              </template>
            </TableColumn>
            <TableColumn col-key="spec.managed_status" :title="t('托管状态')">
              <template #default="{ row: rowData }: { row: IProcInst }">
                <bk-tag v-if="rowData.spec.managed_status" :theme="getManagedStatusTheme(rowData.spec.managed_status)">
                  <span class="process-status">
                    <Spinner v-if="['starting', 'stopping'].includes(row.spec.managed_status)" class="spinner-icon" />
                    {{
                      PROCESS_MANAGED_STATUS_MAP[rowData.spec.managed_status as keyof typeof PROCESS_MANAGED_STATUS_MAP]
                    }}
                  </span>
                </bk-tag>
                <span v-else>--</span>
              </template>
            </TableColumn>
            <TableColumn>
              <template #default="{ row: rowData }: { row: IProcInst }">
                <div v-if="rowData.spec.actions" class="op-btns">
                  <bk-button
                    text
                    theme="primary"
                    :disabled="!rowData.spec.actions.stop"
                    @click="handleOpInst(row.id, rowData.id, 'stop')">
                    {{ t('停止') }}
                  </bk-button>
                  <bk-button
                    text
                    theme="primary"
                    :disabled="!rowData.spec.actions.unregister"
                    @click="handleOpInst(row.id, rowData.id, 'unregister')">
                    {{ t('取消托管') }}
                  </bk-button>
                </div>
                <span v-else>--</span>
              </template>
            </TableColumn>
          </PrimaryTable>
        </div>
      </template>
      <template #expand-icon="{ expanded, row }">
        <angle-up-fill :class="['expand-icon', { expanded }]" @click="handleExpandRow(row)" />
      </template>
      <template #empty>
        <TableEmpty :is-search-empty="isSearchEmpty" @clear="handleClearFilter"></TableEmpty>
      </template>
      <template #loading>
        <bk-loading />
      </template>
    </PrimaryTable>
    <bk-pagination
      v-show="pagination.count > 0"
      class="table-pagination"
      :model-value="pagination.current"
      :count="pagination.count"
      :limit="pagination.limit"
      location="left"
      :layout="['total', 'limit', 'list']"
      @change="handlePageChange"
      @limit-change="handlePageLimitChange" />
  </div>
  <UpdateManagedInfo
    :is-show="isShowUpdateManagedInfo"
    :managed-info="managedInfo"
    @update="handleConfirmOp('update_register')"
    @close="isShowUpdateManagedInfo = false" />
  <OpProcessDialog
    :is-show="isShowOpProcess"
    :info="opProcessInfo"
    @close="isShowOpProcess = false"
    @confirm="handleConfirmOp" />
  <BatchOpProcessDialog
    :is-show="isShowBatchOpProcess"
    :info="batchOpProcessInfo"
    @close="isShowBatchOpProcess = false"
    @confirm="handleConfirmOp" />
</template>

<script lang="ts" setup>
  import { ref, onMounted, computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { AngleUpFill, Spinner } from 'bkui-vue/lib/icon';
  import { getProcessList, processOperate } from '../../../../api/process';
  import type { IProcessItem, IProcInst } from '../../../../../types/process';
  import { CC_SYNC_STATUS, PROCESS_STATUS_MAP, PROCESS_MANAGED_STATUS_MAP } from '../../../../constants/process';
  import { storeToRefs } from 'pinia';
  import { timeAgo } from '../../../../utils';
  import { useRouter } from 'vue-router';
  import BatchOpBtns from './batch-op-btns.vue';
  import TableEmpty from '../../../../components/table/table-empty.vue';
  import UpdateManagedInfo from './update-managed-info.vue';
  import OpProcessDialog from './op-process-dialog.vue';
  import BatchOpProcessDialog from './batch-op-process-dialog.vue';
  import TableMoreAction from './table-more-actions.vue';
  import useGlobalStore from '../../../../store/global';
  import useTablePagination from '../../../../utils/hooks/use-table-pagination';
  import SyncStatus from './sync-status.vue';
  import FilterProcess from './filter-process.vue';
  import SearchSelector from '../../../../components/search-selector.vue';
  import TableBtnTooltips from './table-btn-tooltips.vue';

  const { spaceId } = storeToRefs(useGlobalStore());
  const { pagination, updatePagination } = useTablePagination('clientSearch');
  const { t } = useI18n();
  const router = useRouter();

  const searchField = ref([
    {
      label: t('内网IP'),
      field: 'inner_ips',
      children: [],
    },
    {
      label: t('进程状态'),
      field: 'process_statuses',
      children: [],
    },
    {
      label: t('托管状态'),
      field: 'managed_statuses',
      children: [],
    },
    {
      label: t('CC 同步状态'),
      field: 'cc_sync_statuses',
      children: [],
    },
  ]);

  const operationList = [
    {
      name: t('重启'),
      id: 'restart',
    },
    {
      name: t('重载'),
      id: 'reload',
    },
    {
      name: t('强制停止'),
      id: 'kill',
    },
    {
      name: t('托管'),
      id: 'register',
    },
    {
      name: t('取消托管'),
      id: 'unregister',
    },
    {
      name: t('查看进程配置'),
      id: 'view',
    },
  ];

  const processList = ref<IProcessItem[]>([]);
  const isSearchEmpty = ref(false);
  const isShowUpdateManagedInfo = ref(false);
  const isShowOpProcess = ref(false);
  const isShowBatchOpProcess = ref(false);
  const opProcessInfo = ref({
    op: 'start',
    label: '启动',
    name: '',
    command: '',
  });
  const batchOpProcessInfo = ref({
    op: 'start',
    label: '启动',
    count: 0,
  });
  const filterConditions = ref<Record<string, any>>({});
  const managedInfo = ref({
    old: '',
    new: '',
  });
  const processIds = ref<number[]>([]);
  const processInstanceId = ref(0);
  const filterRef = ref();
  const searchValue = ref<{ [key: string]: string[] }>();
  const selectedIds = ref<number[]>([]);
  const tableLoading = ref(false);
  const tableRef = ref();
  const expandedRowKeys = ref<number[]>([]);
  const cmdbUrl = ref('');

  const tableMaxHeight = computed(() => {
    return tableRef.value && tableRef.value.clientHeight - 60;
  });

  onMounted(() => {
    loadProcessList();
  });

  const loadProcessList = async () => {
    try {
      tableLoading.value = true;
      expandedRowKeys.value = [];
      const params = {
        search: { ...filterConditions.value, ...searchValue.value },
        start: (pagination.value.current - 1) * pagination.value.limit,
        limit: pagination.value.limit,
      };
      const res = await getProcessList(spaceId.value, params);
      processList.value = res.process.map((item: IProcessItem) => ({
        ...item,
        proc_inst: item.proc_inst.map((proc) => ({
          ...proc,
          num: item.spec.proc_num,
        })),
      }));
      cmdbUrl.value = res.cmdb_process_config_url;
      updatePagination('count', res.count);
      searchField.value.forEach((item) => {
        item.children = res.filter_options[item.field];
      });
    } catch (error) {
      console.error(error);
    } finally {
      tableLoading.value = false;
    }
  };

  const handleSelectChange = (ids: number[]) => {
    selectedIds.value = ids;
  };

  // 进程表格操作
  const handleOpProcess = (data: IProcessItem, op: string) => {
    const cmd = JSON.parse(data.spec.source_data);
    processIds.value = [data.id];
    if (op === 'start') {
      opProcessInfo.value = {
        op: 'start',
        label: t('启动'),
        name: data.spec.alias,
        command: cmd.start_cmd,
      };
    } else if (op === 'stop') {
      opProcessInfo.value = {
        op: 'stop',
        label: t('停止'),
        name: data.spec.alias,
        command: cmd.stop_cmd,
      };
    } else if (op === 'kill') {
      opProcessInfo.value = {
        op: 'kill',
        label: t('强制停止'),
        name: data.spec.alias,
        command: cmd.face_stop_cmd,
      };
    }
    isShowOpProcess.value = true;
  };

  // 实例表格操作
  const handleOpInst = (processId: number, id: number, op: string) => {
    processIds.value = [processId];
    processInstanceId.value = id;
    handleConfirmOp(op);
  };

  const handleBatchOpProcess = (op: string) => {
    processIds.value = selectedIds.value;
    if (op === 'start') {
      batchOpProcessInfo.value = {
        op: 'start',
        label: t('启动'),
        count: selectedIds.value.length,
      };
      isShowBatchOpProcess.value = true;
    } else if (op === 'stop') {
      batchOpProcessInfo.value = {
        op: 'stop',
        label: t('停止'),
        count: selectedIds.value.length,
      };
      isShowBatchOpProcess.value = true;
    } else if (op === 'issue') {
      handleBatchConfigIssued();
    } else {
      handleConfirmOp(op);
    }
  };

  const handleSearch = (list: { [key: string]: string[] }) => {
    searchValue.value = list;
    isSearchEmpty.value = Object.keys(list).length > 0;
    loadProcessList();
  };

  const handleFilter = (filters: Record<string, any>) => {
    isSearchEmpty.value = Object.keys(filters).some((filter) => {
      return filter !== 'environment' && filters[filter].length > 0;
    });
    filterConditions.value = filters;
    loadProcessList();
  };

  const getRowClassName = ({ row }: { row: IProcessItem }) => {
    if (row.spec.cc_sync_status === 'deleted') return 'deleted';
  };

  const getSecondTableRowClass = ({ row, rowIndex }: { row: IProcInst; rowIndex: number }) => {
    if (row.num && rowIndex + 1 > row.num) {
      return 'warn';
    }
    return 'default-row';
  };

  const getManagedStatusTheme = (status: string) => {
    const themeMap: Record<string, string> = {
      managed: 'success',
      partly_managed: 'info',
      starting: 'warning',
      stopping: 'warning',
    };
    return themeMap[status] ?? 'default';
  };

  const handleUpdateManagedInfo = (data: IProcessItem) => {
    processIds.value = [data.id];
    managedInfo.value.old = data.spec.prev_data;
    managedInfo.value.new = data.spec.source_data;
    isShowUpdateManagedInfo.value = true;
  };

  const handleMoreActionClick = (data: IProcessItem, op: string) => {
    if (op === 'view') {
      window.open(data.spec.process_config_view_url);
      return;
    }
    if (op === 'link') {
      window.open(cmdbUrl.value);
      return;
    }
    if (op === 'kill') {
      handleOpProcess(data, 'kill');
      return;
    }
    processIds.value = [data.id];
    handleConfirmOp(op);
  };

  const handleConfirmOp = async (op: string) => {
    try {
      const query = {
        processIds: processIds.value,
        processInstanceId: processInstanceId.value,
        operateType: op,
      };
      const res = await processOperate(spaceId.value, query);
      isShowOpProcess.value = false;
      // 进程操作跳转任务详情页
      setTimeout(() => {
        router.push({ name: 'task-detail', params: { taskId: res.batchID } });
      }, 300);
    } catch (error) {
      console.error(error);
    } finally {
      processIds.value = [];
      processInstanceId.value = 0;
    }
  };

  const handlePageChange = (page: number) => {
    updatePagination('current', page);
    loadProcessList();
  };

  const handlePageLimitChange = (limit: number) => {
    updatePagination('limit', limit);
    loadProcessList();
  };

  // 清空筛选条件
  const handleClearFilter = () => {
    filterRef.value.clear();
  };

  // 配置下发
  const handleConfigIssued = (process: IProcessItem) => {
    router.push({
      name: 'config-issued',
      query: {
        processIds: [process.attachment.cc_process_id],
        templateIds: process.spec.bind_template_ids,
      },
    });
  };

  // 批量配置下发
  const handleBatchConfigIssued = () => {
    const selectedProcess = processList.value.filter((item) => selectedIds.value.includes(item.id));

    const processIds = new Set(selectedProcess.map((p) => p.attachment.cc_process_id));

    const templateIds = new Set(selectedProcess.flatMap((p) => p.spec.bind_template_ids));

    router.push({
      name: 'config-issued',
      query: {
        processIds: [...processIds],
        templateIds: [...templateIds],
      },
    });
  };

  // 表格下拉展开收起
  const handleExpandRow = (row: IProcessItem) => {
    const index = expandedRowKeys.value.indexOf(row.id);
    if (index > -1) {
      expandedRowKeys.value.splice(index, 1);
    } else {
      expandedRowKeys.value.push(row.id);
    }
  };
</script>

<style lang="scss" scoped>
  .status-and-screen {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
  }
  .op-wrap {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
    .search-select {
      width: 957px;
    }
  }
  .table-wrap {
    flex: 1;
  }
  .second-table {
    padding: 0 180px 0 62px;
  }
  .expand-icon {
    font-size: 14px;
    cursor: pointer;
    transition: transform 0.3s;
    color: #c4c6cc;
    transform: rotate(-90deg);
    &.expanded {
      transform: rotate(0deg);
    }
    &:hover {
      color: #3a84ff;
    }
  }
  .op-btns {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .table-pagination {
    padding: 14px 16px;
    height: 60px;
    background: #fff;
    border: 1px solid #e8eaec;
    border-top: none;
    :deep(.bk-pagination-list.is-last) {
      margin-left: auto;
    }
  }
  .cc-sync-status {
    &.deleted {
      color: #e71818;
    }
    &.updated {
      color: #e38b02;
    }
  }
  .spinner-icon {
    font-size: 14px;
    color: #3a84ff;
  }
  .process-status {
    display: flex;
    align-items: center;
    gap: 8px;
    .dot {
      width: 13px;
      height: 13px;
      border-radius: 50%;
      &.running {
        border: 3px solid #daf6e5;
        background: #3fc06d;
      }
      &.stopped,
      &.stopping {
        border: 3px solid #ffebeb;
        background: #ea3636;
      }
      &.partly_running {
        border: 3px solid #cbddfe;
        background: #699df4;
      }
    }
  }
  .instance {
    display: flex;
    align-items: center;
    gap: 8px;
    .error-icon {
      font-size: 14px;
      line-height: 14px;
      vertical-align: middle;
      color: #e71818;
      cursor: pointer;
      font-weight: bold;
    }
  }
  .no-cmd-content {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .primary {
    display: flex;
    align-items: center;
    gap: 4px;
    color: #3a84ff;
    cursor: pointer;
  }
</style>

<style lang="scss">
  .process-table {
    .deleted {
      background: #fff0f0;
    }
    .warn {
      background: #fdf4e8;
    }
    .default {
      background: #fafbfd;
    }
    .t-table__row-full-element {
      padding: 0;
    }
    .t-table__expanded-row {
      background: #fafbfd;
    }
  }

  .instance-table {
    .t-table__header {
      tr {
        th {
          height: 32px;
          padding: 0 !important;
          line-height: 32px !important;
        }
      }
    }
    .default-row {
      background: #fafbfd !important;
      td {
        height: 32px;
        padding: 0 !important;
        line-height: 32px !important;
      }
      &:last-child {
        border-bottom: none;
      }
    }
    .t-table__empty-row {
      background: #fafbfd !important;
    }
    .t-table__body tr:last-child td {
      border-bottom: none;
    }
  }
</style>
