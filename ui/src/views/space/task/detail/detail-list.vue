<template>
  <div class="op-wrap">
    <bk-button class="retry-btn">{{ $t('重试所有失败') }}</bk-button>
    <bk-search-select
      v-model="searchValue"
      class="search-select"
      :data="searchList"
      :placeholder="$t('搜索 集群/模块/服务实例/进程别名/CC 进程 ID/Inst_id/内网 IP/执行结果')"
      unique-select />
  </div>
  <div class="list-wrap">
    <div class="panels-list">
      <div
        v-for="panel in panels"
        :key="panel.name"
        :class="['panel', { active: activePanels === panel.name }]"
        @click="activePanels = panel.name">
        <spinner v-if="panel.name === 'running'" class="spinner-icon" />
        <span v-else :class="['dot', panel.name]"></span>
        <span>{{ panel.label }}</span>
        <div class="count">{{ 111 }}</div>
      </div>
    </div>
    <div class="list-content">
      <PrimaryTable class="border" row-key="id">
        <TableColumn :title="$t('集群')" col-key="processPayload.setName">
          <template #default="{ row }">
            <bk-button>{{ row.processPayload.setName }}</bk-button>
          </template>
        </TableColumn>
        <TableColumn :title="$t('模块')" col-key="processPayload.setModule"></TableColumn>
        <TableColumn :title="$t('服务实例')" col-key="processPayload.serviceName"></TableColumn>
        <TableColumn :title="$t('进程别名')" col-key="processPayload.alias"></TableColumn>
        <TableColumn :title="$t('CC 进程 ID')" col-key="processPayload.ccProcessId"></TableColumn>
        <TableColumn :title="$t('Inst_id')" col-key="processPayload.instId"></TableColumn>
        <TableColumn :title="$t('内网 IP')" col-key="processPayload.innerIp"></TableColumn>
        <TableColumn :title="$t('执行耗时')" col-key="result"></TableColumn>
        <TableColumn :title="$t('执行结果')" col-key="result"></TableColumn>
        <TableColumn :title="$t('操作')" col-key="operation">
          <template #default>
            <bk-button text>{{ $t('查看配置') }}</bk-button>
          </template>
        </TableColumn>
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
            :layout="['total', 'limit', 'list']"
            @change="updatePagination" />
        </template>
      </PrimaryTable>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { Spinner } from 'bkui-vue/lib/icon';
  import useTablePagination from '../../../../utils/hooks/use-table-pagination';
  import TableEmpty from '../../../../components/table/table-empty.vue';

  const { pagination, updatePagination } = useTablePagination('taskList');
  const { t } = useI18n();

  const searchList = [
    {
      name: t('集群'),
      id: 'cluster',
    },
    {
      name: t('模块'),
      id: 'module',
    },
    {
      name: t('服务实例'),
      id: 'service',
    },
    {
      name: t('进程别名'),
      id: 'process',
    },
    {
      name: t('CC 进程 ID'),
      id: 'cc_process_id',
    },
    {
      name: t('Inst_id'),
      id: 'inst_id',
    },
    {
      name: t('内网 IP'),
      id: 'ip',
    },
    {
      name: t('执行结果'),
      id: 'result',
    },
  ];
  const panels = [
    {
      name: 'wait',
      label: t('等待执行'),
    },
    {
      label: t('执行成功'),
      name: 'success',
    },
    {
      label: t('执行失败'),
      name: 'failed',
    },
    {
      label: t('正在执行'),
      name: 'running',
    },
  ];
  const searchValue = ref([]);
  const activePanels = ref('wait');
  const isSearchEmpty = ref(false);
</script>

<style scoped lang="scss">
  .op-wrap {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 24px;
    .search-select {
      width: 520px;
    }
  }
  .list-wrap {
    margin-top: 16px;
    .panels-list {
      display: flex;
      align-items: center;
      margin: 0 24px;
      background: #f0f1f5;
      font-size: 14px;
      .panel {
        position: relative;
        display: flex;
        align-items: center;
        height: 42px;
        padding: 0 16px 0 12px;
        gap: 8px;
        border-radius: 4px 4px 0 0;
        cursor: pointer;
        &.active {
          background: #ffffff;
          color: #3a84ff;
          &::after {
            background: #fff;
          }
        }
        &::after {
          position: absolute;
          display: block;
          content: '';
          width: 1px;
          height: 16px;
          background: #c4c6cc;
          right: 0;
        }
        .dot {
          width: 8px;
          height: 8px;
          background: #f0f1f5;
          border: 1px solid #c4c6cc;
          border-radius: 50%;
          &.success {
            background: #cbf0da;
            border-color: #2caf5e;
          }
          &.failed {
            background: #ffdddd;
            border-color: #ea3636;
          }
        }
        .spinner-icon {
          color: #3a84ff;
        }
        .count {
          padding: 0 8px;
          height: 22px;
          line-height: 22px;
          background: #fafbfd;
          border: 1px solid #dcdee5;
          border-radius: 11px;
          color: #4d4f56;
        }
      }
    }
    .list-content {
      padding: 24px;
      background-color: #fff;
    }
  }
  .table-pagination {
    padding: 14px 16px;
    height: 60px;
    background: #fff;
    border-bottom: 1px solid #e8eaec;
    border-top: none;
    :deep(.bk-pagination-list.is-last) {
      margin-left: auto;
    }
  }
</style>
