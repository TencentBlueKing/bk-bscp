<template>
  <div class="status-and-screen">
    <SyncStatus :biz-id="spaceId" />
    <FilterProcess :biz-id="spaceId" @search="handleSearch" />
  </div>
  <div class="op-wrap">
    <BatchOpBtns @start="handleBatchOpProcess('start')" @stop="handleBatchOpProcess('stop')" />
    <bk-search-select
      v-model="value"
      class="search-select"
      :data="searchList"
      :placeholder="t('内网IP/进程状态/托管状态/CC 同步状态')"
      unique-select />
  </div>
  <PrimaryTable class="border" :data="processList" row-key="id" size="small">
    <TableColumn col-key="row-select" type="multiple" width="32"></TableColumn>
    <TableColumn :title="t('集群')" col-key="spec.set_name" width="183">
      <template #default="{ row }">
        <bk-button text theme="primary">{{ row.spec.set_name }}</bk-button>
      </template>
    </TableColumn>
    <TableColumn col-key="spec.module_name" :title="t('模块')" />
    <TableColumn col-key="spec.service_name" :title="t('服务实例')" />
    <TableColumn col-key="spec.alias" :title="t('进程别名')" />
    <TableColumn col-key="attachment.cc_process_id">
      <template #title>
        <span class="tips-title" v-bk-tooltips="{ content: t('对应 CMDB 中唯一 ID'), placement: 'top' }">
          {{ t('CC 进程ID') }}
        </span>
      </template>
    </TableColumn>
    <TableColumn col-key="spec.inner_ip" :title="t('内网 IP')" />
    <TableColumn :title="t('进程状态')" col-key="spec.process_satus">
      <template #default="{ row }">
        {{ row.spec.process_status }}
      </template>
    </TableColumn>
    <TableColumn col-key="spec.process_status" :title="t('托管状态')">
      <template #default="{ row }">
        {{ row.spec.managed_status }}
      </template>
    </TableColumn>
    <TableColumn col-key="spec.cc_sync_updated_at" :title="t('状态获取时间')" />
    <TableColumn col-key="spec.cc_sync_status" :title="t('CC 同步状态')">
      <template #default="{ row }">
        <span :class="['cc-sync-status', row.spec.cc_sync_status]">
          {{ CC_SYNC_STATUS[row.spec.cc_sync_status as keyof typeof CC_SYNC_STATUS] }}
        </span>
      </template>
    </TableColumn>
    <TableColumn :title="t('操作')" :width="200" fixed="right">
      <template #default="{ row }">
        <div class="op-btns">
          <template v-if="row.spec.cc_sync_status === 'updated'">
            <bk-badge position="top-right" theme="danger" dot>
              <bk-button text theme="primary" @click="isShowUpdateManagedInfo = true">
                {{ t('更新托管信息') }}
              </bk-button>
            </bk-badge>
            <bk-button text theme="primary">{{ t('配置下发') }}</bk-button>
            <TableMoreAction />
          </template>
          <template v-else-if="row.spec.cc_sync_status === 'deleted'">
            <bk-button text theme="primary" @click="handleOpProcess(row, 'stop')">{{ t('停止') }}</bk-button>
            <bk-button text theme="primary" @click="handleOpProcess(row, 'force-stop')">{{ t('强制停止') }}</bk-button>
            <bk-button text theme="primary">{{ t('取消托管') }}</bk-button>
          </template>
          <template v-else-if="row.spec.status === ''"></template>
          <bk-button text theme="primary" @click="handleOpProcess(row, 'start')">{{ t('启动') }}</bk-button>
        </div>
      </template>
    </TableColumn>
    <template #expandedRow="{ row }">
      <div class="second-table">
        <PrimaryTable :data="row.proc_inst" row-key="id" size="small">
          <TableColumn col-key="spec.inst_id" :title="t('实例')"> </TableColumn>
          <TableColumn col-key="spec.local_inst_id">
            <template #title>
              <span class="tips-title" v-bk-tooltips="{ content: t('主机下唯一标识'), placement: 'top' }">
                LocalInstID
              </span>
            </template>
          </TableColumn>
          <TableColumn col-key="spec.inst_id" :title="t('实例ID')">
            <template #title>
              <span class="tips-title" v-bk-tooltips="{ content: t('模块下唯一标识'), placement: 'top' }">
                InstID
              </span>
            </template>
          </TableColumn>
          <TableColumn :title="t('进程状态')">
            <template #default="{ row: rowData }">
              {{ rowData.spec.process_status }}
            </template>
          </TableColumn>
          <TableColumn col-key="spec.managed_status" :title="t('托管状态')" />
          <TableColumn>
            <template #default>
              <div class="op-btns">
                <bk-button text theme="primary">{{ t('停止') }}</bk-button>
                <bk-button text theme="primary">{{ t('取消托管') }}</bk-button>
              </div>
            </template>
          </TableColumn>
        </PrimaryTable>
      </div>
    </template>
    <template #expand-icon="{ expanded }">
      <angle-up-fill :class="['expand-icon', { expanded }]" />
    </template>
    <template #empty>
      <TableEmpty :is-search-empty="isSearchEmpty"></TableEmpty>
    </template>
    <template #bottom-content>
      <bk-pagination
        class="table-pagination"
        :model-value="pagination.current"
        :count="pagination.count"
        :limit="pagination.limit"
        location="left"
        :layout="['total', 'limit', 'list']" />
    </template>
  </PrimaryTable>
  <UpdateManagedInfo :is-show="isShowUpdateManagedInfo" @close="isShowUpdateManagedInfo = false" />
  <OpProcessDialog :is-show="isShowOpProcess" :info="opProcessInfo" @close="isShowOpProcess = false" />
  <BatchOpProcessDialog
    :is-show="isShowBatchOpProcess"
    :info="batchOpProcessInfo"
    @close="isShowBatchOpProcess = false" />
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { AngleUpFill } from 'bkui-vue/lib/icon';
  import { getProcessList } from '../../../../api/process';
  import type { IProcessItem } from '../../../../../types/process';
  import { CC_SYNC_STATUS } from '../../../../constants/process';
  import { storeToRefs } from 'pinia';
  import BatchOpBtns from './batch-op-btns.vue';
  import TableEmpty from '../../../../components/table/table-empty.vue';
  import UpdateManagedInfo from './update-managed-info.vue';
  import OpProcessDialog from './op-process-dialog.vue';
  import BatchOpProcessDialog from './batch-op-process-dialog.vue';
  import TableMoreAction from './table-more-action.vue';
  import useGlobalStore from '../../../../store/global';
  import useTablePagination from '../../../../utils/hooks/use-table-pagination';
  import SyncStatus from './sync-status.vue';
  import FilterProcess from './filter-process.vue';

  const { spaceId } = storeToRefs(useGlobalStore());

  const { pagination, updatePagination } = useTablePagination('clientSearch');

  const { t } = useI18n();
  const searchList = [
    {
      name: t('内网IP'),
      id: 'ip',
    },
    {
      name: t('进程状态'),
      id: 'process_status',
    },
    {
      name: t('托管状态'),
      id: 'host_status',
    },
    {
      name: t('CC 同步状态'),
      id: 'cc_status',
    },
  ];

  const processList = ref<IProcessItem[]>([]);
  const value = ref();
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

  onMounted(() => {
    loadProcessList();
  });

  const loadProcessList = async () => {
    try {
      const params = {
        search: filterConditions.value,
        start: 0,
        limit: pagination.value.limit,
      };
      const res = await getProcessList(spaceId.value, params);
      processList.value = res.process;
      updatePagination('count', res.count);
    } catch (error) {
      console.error(error);
    }
  };

  const handleOpProcess = (data: any, op: string) => {
    if (op === 'start') {
      opProcessInfo.value = {
        op: 'start',
        label: t('启动'),
        name: data.spec.alias,
        command: '111',
      };
    } else if (op === 'stop') {
      opProcessInfo.value = {
        op: 'stop',
        label: t('停止'),
        name: data.spec.alias,
        command: '222',
      };
    } else if (op === 'force-stop') {
      opProcessInfo.value = {
        op: 'force-stop',
        label: t('强制停止'),
        name: data.spec.alias,
        command: '333',
      };
    }
    isShowOpProcess.value = true;
  };

  const handleBatchOpProcess = (op: string) => {
    if (op === 'start') {
      batchOpProcessInfo.value = {
        op: 'start',
        label: t('启动'),
        count: 1,
      };
    } else if (op === 'stop') {
      batchOpProcessInfo.value = {
        op: 'stop',
        label: t('停止'),
        count: 2,
      };
    }
    isShowBatchOpProcess.value = true;
  };

  const handleSearch = (filters: Record<string, any>) => {
    console.log('搜索条件：', filters);
    filterConditions.value = filters;
    loadProcessList();
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
</style>
