<template>
  <section class="task-list-wrap">
    <div class="title">{{ $t('任务历史') }}</div>
    <SearchSelector
      ref="searchSelectorRef"
      :search-field="searchField"
      :placeholder="t('任务对象/动作/执行账户/执行结果')"
      class="search-select"
      @search="handleSearch" />
    <div ref="tableRef" class="task-list">
      <PrimaryTable
        class="border"
        row-key="id"
        cell-empty-content="--"
        :data="tableList"
        :loading="loading"
        :max-height="tableMaxHeight">
        <TableColumn col-key="id" title="ID">
          <template #default="{ row }: { row: ITaskHistoryItem }">
            <bk-button theme="primary" text @click="handleJump(row)">{{ row.id }}</bk-button>
          </template>
        </TableColumn>
        <TableColumn col-key="task_object" :title="t('任务对象')">
          <template #default="{ row }: { row: ITaskHistoryItem }">
            {{ row.task_object === 'process' ? $t('进程') : $t('配置文件') }}
          </template>
        </TableColumn>
        <TableColumn col-key="task_action" :title="t('动作')">
          <template #default="{ row }: { row: ITaskHistoryItem }">
            {{ TASK_ACTION_MAP[row.task_action as keyof typeof TASK_ACTION_MAP] }}
          </template>
        </TableColumn>
        <TableColumn col-key="task_data.environment" :title="t('环境类型')" />
        <TableColumn col-key="task_data.operate_range" :title="t('操作范围')" width="180" ellipsis>
          <template #default="{ row }: { row: ITaskHistoryItem }">
            {{ mergeOpRange(row.task_data.operate_range) }}
          </template>
        </TableColumn>
        <TableColumn col-key="id" :title="t('执行账户')">
          <template #default="{ row }: { row: ITaskHistoryItem }">
            <UserName :name="row.creator" />
          </template>
        </TableColumn>
        <TableColumn col-key="start_at" :title="t('开始时间')">
          <template #default="{ row }: { row: ITaskHistoryItem }">
            {{ datetimeFormat(row.start_at) }}
          </template>
        </TableColumn>
        <TableColumn col-key="end_at" :title="t('结束时间')">
          <template #default="{ row }: { row: ITaskHistoryItem }">
            {{ datetimeFormat(row.end_at) }}
          </template>
        </TableColumn>
        <TableColumn col-key="execution_time" :title="t('执行耗时')">
          <template #default="{ row }: { row: ITaskHistoryItem }"> {{ row.execution_time }}s </template>
        </TableColumn>
        <TableColumn col-key="status" :title="t('执行结果')">
          <template #default="{ row }: { row: ITaskHistoryItem }">
            <div v-if="row.status" class="status">
              <span :class="['dot', row.status]"></span>
              <span>{{ TASK_STATUS_MAP[row.status as keyof typeof TASK_STATUS_MAP] }}</span>
            </div>
            <span v-else>--</span>
          </template>
        </TableColumn>
        <TableColumn :title="t('操作')" :width="200" fixed="right" col-key="operation">
          <template #default="{ row }: { row: ITaskHistoryItem }">
            <bk-button
              v-if="row.status !== 'running'"
              :disabled="row.status === 'success'"
              theme="primary"
              text
              @click="handleRetry(row)">
              {{ t('重试') }}
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
  </section>
</template>

<script lang="ts" setup>
  import { ref, onMounted, computed } from 'vue';
  import { useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import { getTaskHistoryList, retryTask } from '../../../../api/task';
  import { storeToRefs } from 'pinia';
  import { TASK_ACTION_MAP, TASK_STATUS_MAP } from '../../../../constants/task';
  import type { ITaskHistoryItem, IOperateRange } from '../../../../../types/task';
  import { datetimeFormat } from '../../../../utils';
  import useTablePagination from '../../../../utils/hooks/use-table-pagination';
  import useGlobalStore from '../../../../store/global';
  import TableEmpty from '../../../../components/table/table-empty.vue';
  import SearchSelector from '../../../../components/search-selector.vue';
  import UserName from '../../../../components/user-name.vue';
  import useTaskStore from '../../../../store/task';

  const { t } = useI18n();
  const taskStore = useTaskStore();
  const router = useRouter();
  const { pagination, updatePagination } = useTablePagination('taskList');
  const { spaceId } = storeToRefs(useGlobalStore());

  const isSearchEmpty = ref(false);
  const searchSelectorRef = ref();
  const searchField = ref([
    { field: 'task_object', label: t('任务对象'), children: [] },
    { field: 'task_action', label: t('动作'), children: [] },
    { field: 'executor', label: t('执行账户'), children: [] },
    { field: 'status', label: t('执行结果'), children: [] },
  ]);
  const tableList = ref<ITaskHistoryItem[]>([]);
  const loading = ref(false);
  const tableRef = ref();
  const searchValue = ref<{ [key: string]: string | string[] }>();

  const tableMaxHeight = computed(() => {
    return tableRef.value && tableRef.value.clientHeight - 60;
  });

  onMounted(() => {
    loadTaskList();
  });

  const handleRetry = async (row: ITaskHistoryItem) => {
    try {
      await retryTask(spaceId.value, row.id);
      loadTaskList();
    } catch (error) {
      console.error(error);
    }
  };

  const loadTaskList = async () => {
    try {
      loading.value = true;
      const params = {
        start: (pagination.value.current - 1) * pagination.value.limit,
        limit: pagination.value.limit,
        ...searchValue.value,
      };
      const res = await getTaskHistoryList(spaceId.value, params);
      tableList.value = res.list;
      pagination.value.count = res.count;
      searchField.value.forEach((item) => {
        item.children = res.filter_options[`${item.field}_choices`];
      });
    } catch (error) {
      console.error(error);
    } finally {
      loading.value = false;
    }
  };

  const handleSearch = (list: { [key: string]: string | string[] }) => {
    searchValue.value = {
      taskActions: list.task_action || [],
      taskObjects: list.task_object || [],
      executors: list.executor || [],
      statuses: list.status || [],
    };
    isSearchEmpty.value = Object.keys(list).length > 0;
    pagination.value.current = 1;
    updatePagination('limit', 10);
    loadTaskList();
  };

  const mergeOpRange = (operateRange: IOperateRange) => {
    return Object.values(operateRange)
      .map((arr) => (arr.length ? `[${arr.join(',')}]` : '*'))
      .join('.');
  };

  const handleJump = (data: ITaskHistoryItem) => {
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
    } = data;

    const actionText = TASK_ACTION_MAP[task_action as keyof typeof TASK_ACTION_MAP];
    const typePrefix = task_object === 'process' ? t('进程') : t('配置文件');

    taskStore.$patch({
      taskDetail: {
        id,
        task_type: `${typePrefix}${actionText}`,
        task_object,
        environment,
        operate_range: mergeOpRange(operate_range),
        creator,
        start_at: datetimeFormat(start_at),
        end_at: datetimeFormat(end_at),
        execution_time: `${execution_time}s`,
        status,
      },
    });

    router.push({ name: 'task-detail', params: { taskId: id } });
  };

  const handlePageChange = (page: number) => {
    updatePagination('current', page);
    loadTaskList();
  };

  const handlePageLimitChange = (limit: number) => {
    updatePagination('limit', limit);
    loadTaskList();
  };

  const handleClearSearch = () => {
    searchValue.value = {};
    isSearchEmpty.value = false;
    searchSelectorRef.value.clear();
    loadTaskList();
  };
</script>

<style scoped lang="scss">
  .task-list-wrap {
    display: flex;
    flex-direction: column;
    padding: 28px 24px;
    background-color: #f5f7fa;
    height: 100%;
    .task-list {
      flex: 1;
    }
  }
  .title {
    font-weight: 700;
    font-size: 16px;
    color: #4d4f56;
    line-height: 24px;
  }
  .search-select {
    margin: 16px 0;
    width: 400px;
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
  .status {
    display: flex;
    align-items: center;
    gap: 8px;
    .dot {
      width: 8px;
      height: 8px;
      background: #f0f1f5;
      border: 1px solid #c4c6cc;
      border-radius: 50%;
      &.succeed {
        background: #cbf0da;
        border-color: #2caf5e;
      }
      &.failed {
        background: #ffdddd;
        border-color: #ea3636;
      }
      &.partly_failed {
        background: #fce5c0;
        border-color: #f59500;
      }
    }
  }
</style>
