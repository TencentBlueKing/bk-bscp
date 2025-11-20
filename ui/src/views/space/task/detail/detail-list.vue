<template>
  <div class="detail-list-wrap">
    <div class="op-wrap">
      <bk-button class="retry-btn" :disabled="failureCount === 0" @click="handleRetry">
        {{ $t('重试所有失败') }}
      </bk-button>
      <searchSelector
        ref="searchSelectorRef"
        :search-field="searchField"
        :placeholder="$t('搜索 集群/模块/服务实例/进程别名/CC 进程 ID/Inst_id/内网 IP')"
        class="search-select"
        @search="handleSearch" />
    </div>
    <div class="list-wrap">
      <div class="panels-list">
        <div
          v-for="panel in panels"
          :key="panel.status"
          :class="['panel', { active: activePanels === panel.status }]"
          @click="handleChangePanel(panel.status)">
          <spinner v-if="panel.status === 'RUNNING'" class="spinner-icon" />
          <span v-else :class="['dot', panel.status]"></span>
          <span>{{ panel.label }}</span>
          <div class="count">{{ panel.count }}</div>
        </div>
      </div>
      <div ref="tableRef" class="list-content">
        <PrimaryTable
          :data="detailList"
          :loading="loading"
          :max-height="tableMaxHeight"
          :ellipsis="true"
          class="border"
          row-key="id"
          cell-empty-content="--">
          <TableColumn :title="$t('集群')" col-key="process_payload.set_name">
            <template #default="{ row }">
              <bk-button theme="primary" text>{{ row.process_payload.set_name || '--' }}</bk-button>
            </template>
          </TableColumn>
          <TableColumn :title="$t('模块')" col-key="process_payload.module_name"></TableColumn>
          <TableColumn :title="$t('服务实例')" col-key="process_payload.service_name"></TableColumn>
          <TableColumn :title="$t('进程别名')" col-key="process_payload.alias"></TableColumn>
          <TableColumn :title="$t('CC 进程 ID')" col-key="process_payload.cc_process_id"></TableColumn>
          <TableColumn :title="$t('模块下唯一标识')" col-key="process_payload.module_inst_seq"></TableColumn>
          <TableColumn :title="$t('内网 IP')" col-key="process_payload.inner_ip"></TableColumn>
          <TableColumn :title="$t('执行耗时')" col-key="execution_time">
            <template #default="{ row }"> {{ row.execution_time }}s </template>
          </TableColumn>
          <TableColumn :title="$t('执行结果')" col-key="status">
            <template #default="{ row }">
              <div class="status">
                <Spinner v-if="row.status === 'RUNNING'" class="spinner-icon" />
                <span v-else :class="['dot', row.status]"></span>
                <span>{{ TASK_DETAIL_STATUS_MAP[row.status as keyof typeof TASK_DETAIL_STATUS_MAP] }}</span>
                <info-line
                  v-if="row.status === 'FAILURE'"
                  class="info-icon"
                  v-bk-tooltips="{ content: row.message || '--' }" />
              </div>
            </template>
          </TableColumn>
          <TableColumn :title="$t('操作')" col-key="operation">
            <template #default="{ row }">
              <bk-button
                v-if="['FAILURE', 'SUCCESS'].includes(row.status) && taskDetail.task_object !== 'process'"
                theme="primary"
                text>
                {{ $t('查看配置') }}
              </bk-button>
              <span v-else>--</span>
            </template>
          </TableColumn>
          <template #empty>
            <TableEmpty :is-search-empty="isSearchEmpty" @clear="handleClearSearch"></TableEmpty>
          </template>
          <template #loading>
            <bk-loading />
          </template>
        </PrimaryTable>
        <bk-pagination
          class="table-pagination"
          :model-value="pagination.current"
          :count="pagination.count"
          :limit="pagination.limit"
          location="left"
          :layout="['total', 'limit', 'list']"
          @change="handlePageChange"
          @limit-change="handlePageLimitChange" />
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onBeforeMount, computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { Spinner, InfoLine } from 'bkui-vue/lib/icon';
  import { getTaskDetailList, retryTask } from '../../../../api/task';
  import { TASK_DETAIL_STATUS_MAP } from '../../../../constants/task';
  import useTablePagination from '../../../../utils/hooks/use-table-pagination';
  import TableEmpty from '../../../../components/table/table-empty.vue';
  import { useRoute } from 'vue-router';
  import searchSelector from '../../../../components/search-selector.vue';

  defineProps<{
    taskDetail: Record<string, any>;
  }>();

  const { pagination, updatePagination } = useTablePagination('taskList');
  const { t } = useI18n();
  const route = useRoute();

  const searchField = ref([
    {
      label: t('集群'),
      field: 'set_name',
      children: [],
    },
    {
      label: t('模块'),
      field: 'module_name',
      children: [],
    },
    {
      label: t('服务实例'),
      field: 'service_name',
      children: [],
    },
    {
      label: t('进程别名'),
      field: 'alias',
      children: [],
    },
    {
      label: t('CC 进程 ID'),
      field: 'cc_process_id',
      children: [],
    },
    {
      label: t('模块下唯一标识'),
      field: 'module_inst_seq',
      children: [],
    },
    {
      label: t('内网 IP'),
      field: 'ip',
      children: [],
    },
  ]);
  const panels = ref([
    {
      status: 'INITIALIZING',
      label: t('等待执行'),
      count: 0,
    },
    {
      label: t('执行成功'),
      status: 'SUCCESS',
      count: 0,
    },
    {
      label: t('执行失败'),
      status: 'FAILURE',
      count: 0,
    },
    {
      label: t('正在执行'),
      status: 'RUNNING',
      count: 0,
    },
  ]);
  const bkBizId = ref(String(route.params.spaceId));
  const taskId = ref(Number(route.params.taskId));
  const searchValue = ref();
  const activePanels = ref('INITIALIZING');
  const isSearchEmpty = ref(false);
  const detailList = ref<any[]>([]);
  const loading = ref(false);
  const failureCount = ref(0);
  const tableRef = ref();
  const loadPanelsFlag = ref(true);
  const searchSelectorRef = ref();

  const tableMaxHeight = computed(() => {
    return tableRef.value && tableRef.value.clientHeight - 150;
  });

  onBeforeMount(async () => {
    loadTaskList();
  });

  const loadTaskList = async () => {
    try {
      loading.value = true;
      const res = await getTaskDetailList(bkBizId.value, taskId.value, {
        status: activePanels.value,
        start: pagination.value.limit * (pagination.value.current - 1),
        limit: pagination.value.limit,
        ...searchValue.value,
      });
      detailList.value = res.tasks;
      pagination.value.count = res.count;
      panels.value.forEach((panel) => {
        panel.count = res.statistics.find((item: any) => item.status === panel.status).count;
        if (panel.status === 'FAILURE') {
          failureCount.value = panel.count;
        }
      });
      if (loadPanelsFlag.value) {
        loadPanelsFlag.value = false;
        activePanels.value = panels.value.find((item: any) => item.count > 0)?.status || 'INITIALIZING';
        loadTaskList();
      }
      searchField.value.forEach((item) => {
        item.children = res.filter_options[`${item.field}_choices`];
      });
    } catch (error) {
      console.error(error);
    } finally {
      loading.value = false;
    }
  };

  const handleChangePanel = (status: string) => {
    if (activePanels.value === status) return;
    activePanels.value = status;
    updatePagination('current', 1);
    loadTaskList();
  };

  const handlePageChange = (page: number) => {
    updatePagination('current', page);
    loadTaskList();
  };

  const handlePageLimitChange = (limit: number) => {
    updatePagination('limit', limit);
    loadTaskList();
  };

  const handleSearch = (list: { [key: string]: string | string[] }) => {
    searchValue.value = {
      setNames: list.set_name || [],
      moduleNames: list.module_name || [],
      serviceNames: list.service_name || [],
      processAliases: list.alias || [],
      ccProcessIds: list.cc_process_id || [],
      instIds: list.module_inst_seq || [],
      ips: list.ip || [],
    };
    isSearchEmpty.value = Object.keys(list).length > 0;
    pagination.value.current = 1;
    updatePagination('limit', 10);
    loadTaskList();
  };

  // 重试所有失败任务
  const handleRetry = async () => {
    try {
      await retryTask(bkBizId.value, taskId.value);
      loadTaskList();
    } catch (error) {
      console.error(error);
    }
  };

  const handleClearSearch = () => {
    searchValue.value = {};
    isSearchEmpty.value = false;
    searchSelectorRef.value.clear();
    loadTaskList();
  };
</script>

<style scoped lang="scss">
  .detail-list-wrap {
    display: flex;
    flex-direction: column;
    height: calc(100% - 108px);
  }
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
    flex: 1;
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
      height: calc(100% - 42px);
    }
    .status {
      display: flex;
      align-items: center;
      gap: 8px;
      .info-icon {
        font-size: 14px;
        color: #979ba5;
      }
    }
    .dot {
      width: 8px;
      height: 8px;
      background: #f0f1f5;
      border: 1px solid #c4c6cc;
      border-radius: 50%;
      &.SUCCESS {
        background: #cbf0da;
        border-color: #2caf5e;
      }
      &.FAILURE {
        background: #ffdddd;
        border-color: #ea3636;
      }
    }
    .spinner-icon {
      color: #3a84ff;
    }
    .list-content {
      padding: 24px;
      background-color: #fff;
      height: 100%;
    }
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
  :deep(.t-loading__gradient-conic) {
    display: none;
  }
</style>
