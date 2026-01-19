<template>
  <div class="detail-list-wrap">
    <div class="op-wrap">
      <bk-button class="retry-btn" :disabled="failureCount === 0" @click="handleRetry">
        {{ $t('重试所有失败') }}
      </bk-button>
      <SearchSelector
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
          v-show="panel.count > 0"
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
          <TableColumn :title="$t('集群')" col-key="task_payload.set_name">
            <template #default="{ row }">
              <bk-button theme="primary" text>{{ row.task_payload.set_name || '--' }}</bk-button>
            </template>
          </TableColumn>
          <TableColumn :title="$t('模块')" col-key="task_payload.module_name"></TableColumn>
          <TableColumn :title="$t('服务实例')" col-key="task_payload.service_name" ellipsis></TableColumn>
          <TableColumn :title="$t('进程别名')" col-key="task_payload.alias"></TableColumn>
          <TableColumn :title="$t('CC 进程 ID')" col-key="task_payload.cc_process_id"></TableColumn>
          <TableColumn col-key="task_payload.module_inst_seq">
            <template #title>
              <span class="tips-title" v-bk-tooltips="{ content: $t('模块下唯一标识'), placement: 'top' }">
                ModuleInstSeq
              </span>
            </template>
          </TableColumn>
          <TableColumn :title="$t('内网 IP')" col-key="task_payload.inner_ip"></TableColumn>
          <TableColumn :title="$t('执行耗时')" col-key="execution_time">
            <template #default="{ row }"> {{ row.execution_time }}s </template>
          </TableColumn>
          <TableColumn v-if="action === 'config_check'" :title="$t('检查结果')" col-key="compare_status">
            <template #default="{ row }">
              <span>
                {{ TASK_DETAIL_COMPARE_STATUS_MAP[row.compare_status as keyof typeof TASK_DETAIL_COMPARE_STATUS_MAP] }}
              </span>
            </template>
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
          <TableColumn v-if="showOperationActions.includes(action)" :title="$t('操作')" col-key="operation">
            <template #default="{ row }">
              <template v-if="['FAILURE', 'SUCCESS'].includes(row.status)">
                <bk-button
                  v-if="action === 'config_check' && row.compare_status === 'DIFFERENT'"
                  theme="primary"
                  text
                  @click="handleDiff(row.task_id)">
                  {{ $t('配置对比') }}
                </bk-button>
                <bk-button v-else theme="primary" text @click="handleView(row.task_id)">
                  {{ $t('查看配置') }}
                </bk-button>
              </template>
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
  <ConfigDetail
    :bk-biz-id="bkBizId"
    v-model:is-show="detailSliderData.open"
    :is-check="false"
    :data="detailSliderData.data" />
  <TaskDiff :bk-biz-id="bkBizId" v-model:is-show="diffSliderData.open" :task-id="diffSliderData.taskId" />
</template>

<script lang="ts" setup>
  import { ref, onBeforeMount, computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { Spinner, InfoLine } from 'bkui-vue/lib/icon';
  import { getTaskDetailList, retryTask } from '../../../../api/task';
  import { TASK_DETAIL_STATUS_MAP, TASK_ACTION_MAP, TASK_DETAIL_COMPARE_STATUS_MAP } from '../../../../constants/task';
  import useTablePagination from '../../../../utils/hooks/use-table-pagination';
  import TableEmpty from '../../../../components/table/table-empty.vue';
  import { useRoute } from 'vue-router';
  import SearchSelector from '../../../../components/search-selector.vue';
  import useTaskStore from '../../../../store/task';
  import { datetimeFormat } from '../../../../utils';
  import type { ITaskDetailItem } from '../../../../../types/task';
  import ConfigDetail from '../../config-template/config-issued/config-detail.vue';
  import TaskDiff from './task-diff.vue';

  const taskStore = useTaskStore();
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
    {
      label: t('现网配置异常'),
      status: 'ABNORMAL',
      count: 0,
    },
  ]);
  // 展示操作列表的动作
  const showOperationActions = ['config_generate', 'config_publish', 'config_check'];

  const bkBizId = ref(String(route.params.spaceId));
  const taskId = ref(Number(route.params.taskId));
  const searchValue = ref();
  const activePanels = ref('INITIALIZING');
  const isSearchEmpty = ref(false);
  const detailList = ref<ITaskDetailItem[]>([]);
  const loading = ref(false);
  const failureCount = ref(0);
  const tableRef = ref();
  const loadPanelsFlag = ref(true);
  const searchSelectorRef = ref();
  const action = ref('');
  const detailSliderData = ref({
    open: false,
    data: { ccProcessId: 0, moduleInstSeq: 0, configTemplateId: 0, configVersionId: 0, taskId: '' },
  });
  const diffSliderData = ref({
    open: false,
    taskId: '',
  });
  const timer = ref<number | null>(null);
  const isRequesting = ref(false);

  const tableMaxHeight = computed(() => {
    return tableRef.value && tableRef.value.clientHeight - 150;
  });

  onBeforeMount(async () => {
    loadTaskList();
  });

  const scheduleReload = (delay = 0) => {
    if (timer.value) {
      clearTimeout(timer.value);
      timer.value = null;
    }

    timer.value = setTimeout(() => {
      loadTaskList(delay > 0);
    }, delay);
  };

  const loadTaskList = async (silent = false) => {
    //  防止并发 & 递归触发
    if (isRequesting.value) return;

    try {
      isRequesting.value = true;
      if (!silent) {
        loading.value = true;
      }

      const res = await getTaskDetailList(bkBizId.value, taskId.value, {
        status: activePanels.value,
        start: pagination.value.limit * (pagination.value.current - 1),
        limit: pagination.value.limit,
        ...searchValue.value,
      });

      detailList.value = res.tasks;
      pagination.value.count = res.count;

      let hasRunningTask = false;

      panels.value.forEach((panel) => {
        panel.count = res.statistics.find((item: any) => item.status === panel.status)?.count || 0;

        if (panel.status === 'FAILURE') {
          failureCount.value = panel.count;
        }
        const RUNNING_STATUSES = ['RUNNING', 'INITIALIZING'];
        if (RUNNING_STATUSES.includes(panel.status) && panel.count > 0) {
          hasRunningTask = true;
        }
      });

      // 只允许触发一次 reload
      const activePanel = panels.value.find((panel) => panel.status === activePanels.value);

      if (!activePanel?.count || loadPanelsFlag.value) {
        loadPanelsFlag.value = false;

        const nextPanel = panels.value.find((item: any) => item.count > 0)?.status || 'INITIALIZING';

        if (nextPanel !== activePanels.value) {
          activePanels.value = nextPanel;
          scheduleReload();
          return;
        }
      }

      // filter options
      searchField.value.forEach((item) => {
        item.children = res.filter_options[`${item.field}_choices`];
      });

      // 轮询逻辑（只留一个）
      if (hasRunningTask) {
        scheduleReload(3000);
      } else {
        if (timer.value) {
          clearTimeout(timer.value);
          timer.value = null;
        }
      }

      const {
        id,
        task_object,
        task_data: { environment, operate_range },
        creator,
        start_at,
        end_at,
        execution_time,
        task_action,
        status,
      } = res.task_batch;

      const actionText = TASK_ACTION_MAP[task_action as keyof typeof TASK_ACTION_MAP];
      const typePrefix = task_object === 'process' ? t('进程') : t('配置文件');
      action.value = task_action;

      taskStore.$patch({
        taskDetail: {
          id,
          task_type: `${typePrefix}${actionText}`,
          task_object,
          environment,
          operate_range,
          creator,
          start_at: datetimeFormat(start_at),
          end_at: end_at ? datetimeFormat(end_at) : '--',
          execution_time: `${execution_time}s`,
          status,
        },
      });
    } catch (error) {
      console.error(error);
    } finally {
      isRequesting.value = false;
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

  const handleView = (id: string) => {
    detailSliderData.value = {
      open: true,
      data: { ccProcessId: 0, moduleInstSeq: 0, configTemplateId: 0, taskId: id, configVersionId: 0 },
    };
  };

  const handleDiff = (id: string) => {
    diffSliderData.value = {
      open: true,
      taskId: id,
    };
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
