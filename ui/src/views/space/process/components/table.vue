<template>
  <div class="op-wrap">
    <BatchOpBtns @start="handleBatchOpProcess('start')" @stop="handleBatchOpProcess('stop')" />
    <bk-search-select
      v-model="value"
      class="search-select"
      :data="searchList"
      :placeholder="t('内网IP/进程状态/托管状态/CC 同步状态')"
      unique-select />
  </div>
  <PrimaryTable class="border" :data="data" row-key="id" size="small">
    <TableColumn col-key="row-select" type="multiple" width="32"></TableColumn>
    <TableColumn :title="t('集群')" width="183">
      <template #default="{ row }">
        <bk-button text theme="primary">{{ row.spec.set_name }}</bk-button>
      </template>
    </TableColumn>
    <TableColumn col-key="spec.module_name" :title="t('模块')" />
    <TableColumn col-key="spec.service_name" :title="t('服务实例')" />
    <TableColumn col-key="spec.alias" :title="t('进程别名')" />
    <TableColumn col-key="spec.cc_process_id">
      <template #title>
        <span class="tips-title" v-bk-tooltips="{ content: t('对应 CMDB 中唯一 ID'), placement: 'top' }">
          {{ t('CC 进程ID') }}
        </span>
      </template>
    </TableColumn>
    <TableColumn col-key="spec.inner_ip" :title="t('内网 IP')" />
    <TableColumn :title="t('进程状态')">
      <template #default="{ row }">
        {{ row.spec.process_status }}
      </template>
    </TableColumn>
    <TableColumn col-key="spec.process_status" :title="t('托管状态')" />
    <TableColumn col-key="spec.cc_sync_updated_at" :title="t('状态获取时间')" />
    <TableColumn col-key="spec.cc_sync_status" :title="t('CC 同步状态')" />
    <TableColumn :title="t('操作')" :width="200" fixed="right">
      <template #default="{ row }">
        <div class="op-btns">
          <bk-button text theme="primary" @click="handleOpProcess(row, 'start')">{{ t('启动') }}</bk-button>
          <!-- <bk-button text theme="primary" @click="handleOpProcess(row, 'stop')">{{ t('停止') }}</bk-button>
          <bk-button text theme="primary" @click="handleOpProcess(row, 'force-stop')">{{ t('强制停止') }}</bk-button> -->
          <bk-badge position="top-right" theme="danger" dot>
            <bk-button text theme="primary" @click="isShowUpdateManagedInfo = true">
              {{ t('更新托管信息') }}
            </bk-button>
          </bk-badge>
          <!-- <bk-button text theme="primary">{{ t('配置下发') }}</bk-button> -->
          <TableMoreAction />
        </div>
      </template>
    </TableColumn>
    <template #expandedRow="{ row }">
      <div class="second-table">
        <PrimaryTable :data="row.proc_inst" row-key="id" size="small">
          <TableColumn col-key="spec.local_inst_id" :title="t('实例')"> </TableColumn>
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
              <bk-button text theme="primary">{{ t('停止') }}</bk-button>
              <bk-button text theme="primary">{{ t('取消托管') }}</bk-button>
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
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { AngleUpFill } from 'bkui-vue/lib/icon';
  import BatchOpBtns from './batch-op-btns.vue';
  import TableEmpty from '../../../../components/table/table-empty.vue';
  import UpdateManagedInfo from './update-managed-info.vue';
  import OpProcessDialog from './op-process-dialog.vue';
  import BatchOpProcessDialog from './batch-op-process-dialog.vue';
  import TableMoreAction from './table-more-action.vue';

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

  const data = [
    {
      id: 1,
      spec: {
        set_name: 'cluster-A',
        module_name: 'module-X',
        service_name: 'service-01',
        environment: 'production',
        alias: 'proc-alias',
        inner_ip: '10.0.0.1',
        cc_sync_status: 'synced',
        cc_sync_updated_at: '11',
        source_data: '{ "origin": "CC" }',
      },
      attachment: {
        biz_id: 20001,
        tenant_id: 'tenant_abc',
        cc_process_id: 30009,
      },
      revision: {
        creator: 'admin',
        reviser: 'admin',
        created_at: '2025-10-15T08:00:00Z',
        updated_at: '2025-10-16T08:00:00Z',
      },
      proc_inst: [
        {
          id: 1001,
          spec: {
            local_inst_id: 'local-001',
            inst_id: 'inst-001',
            status: 'running',
            managed_status: 'managed',
            status_updated_at: '2025-10-16T08:00:00Z',
          },
          attachment: {
            biz_id: 20001,
            tenant_id: 'tenant_abc',
            process_id: 1,
          },
          revision: {
            creator: 'system',
            reviser: 'system',
            created_at: '2025-10-16T08:00:00Z',
            updated_at: '2025-10-16T09:00:00Z',
          },
        },
      ],
    },
    {
      id: 2,
      spec: {
        set_name: 'cluster-B',
        module_name: 'module-Y',
        service_name: 'service-02',
        environment: 'staging',
        alias: 'proc-alias-2',
        inner_ip: '',
        cc_sync_status: 'out_of_sync',
        cc_sync_updated_at: '22',
        source_data: '{ "origin": "manual" }',
      },
      attachment: {
        biz_id: 20002,
        tenant_id: 'tenant_def',
        cc_process_id: 30010,
      },
      revision: {
        creator: 'user1',
        reviser: 'user2',
        created_at: '2025-10-14T08:00:00Z',
        updated_at: '2025-10-15T08:00:00Z',
      },
      proc_inst: [
        {
          id: 1001,
          spec: {
            local_inst_id: 'local-001',
            inst_id: 'inst-001',
            status: 'running',
            managed_status: 'managed',
            status_updated_at: '2025-10-16T08:00:00Z',
          },
          attachment: {
            biz_id: 20001,
            tenant_id: 'tenant_abc',
            process_id: 1,
          },
          revision: {
            creator: 'system',
            reviser: 'system',
            created_at: '2025-10-16T08:00:00Z',
            updated_at: '2025-10-16T09:00:00Z',
          },
        },
      ],
    },
  ];

  const value = ref();
  const isSearchEmpty = ref(false);
  const isShowUpdateManagedInfo = ref(false);
  const isShowOpProcess = ref(false);
  const isShowBatchOpProcess = ref(false);
  const pagination = ref({
    current: 1,
    count: 50,
    limit: 10,
  });
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
</script>

<style lang="scss" scoped>
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
    padding: 0 180px 0 92px;
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
</style>
