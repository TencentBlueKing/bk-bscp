<template>
  <section class="task-list-wrap">
    <div class="title">{{ $t('任务历史') }}</div>
    <SearchSelector
      ref="searchSelectorRef"
      :search-filed="searchFiled"
      :user-filed="['reviser']"
      :placeholder="t('模板名称/文件名/更新人')"
      class="search-select"
      @search="handleSearch" />
    <PrimaryTable :data="tableList" class="border" row-key="id" :ellipsis="true">
      <TableColumn col-key="id" title="ID">
        <template #default="{ row }: { row: ITaskHistoryItem }">
          <bk-button theme="primary" text>{{ row.id }}</bk-button>
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
      <TableColumn col-key="task_data.environment" :title="t('环境类型')"></TableColumn>
      <TableColumn col-key="task_data.operate_range" :title="t('操作范围')" width="180">
        <template #default="{ row }: { row: ITaskHistoryItem }">
          {{ mergeOpRange(row.task_data.operate_range) }}
        </template>
      </TableColumn>
      <TableColumn col-key="id" :title="t('执行账户')">
        <template #default="{ row }: { row: ITaskHistoryItem }">
          <UserName :name="row.creator" />
        </template>
      </TableColumn>
      <TableColumn col-key="start_at" :title="t('开始时间')"></TableColumn>
      <TableColumn col-key="end_at" :title="t('结束时间')"></TableColumn>
      <TableColumn col-key="id" :title="t('执行耗时')"></TableColumn>
      <TableColumn col-key="id" :title="t('执行结果')"></TableColumn>
      <TableColumn :title="t('操作')" :width="200" fixed="right">
        <template #default="{ row }">
          <bk-button theme="primary" text @click="handleRetry(row)">{{ t('重试') }}</bk-button>
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
  </section>
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { getTaskHistoryList } from '../../../../api/task';
  import { storeToRefs } from 'pinia';
  import { TASK_ACTION_MAP } from '../../../../constants/task';
  import type { ITaskHistoryItem, IOperateRange } from '../../../../../types/task';
  import useTablePagination from '../../../../utils/hooks/use-table-pagination';
  import useGlobalStore from '../../../../store/global';
  import TableEmpty from '../../../../components/table/table-empty.vue';
  import SearchSelector from '../../../../components/search-selector.vue';
  import UserName from '../../../../components/user-name.vue';

  const { t } = useI18n();
  const { pagination, updatePagination } = useTablePagination('taskList');
  const { spaceId } = storeToRefs(useGlobalStore());

  const isSearchEmpty = ref(false);
  const searchSelectorRef = ref();
  const searchFiled = [
    { field: 'alias', label: t('模板名称') },
    { field: 'name', label: t('文件名') },
    { field: 'reviser', label: t('更新人') },
  ];
  const tableList = ref<ITaskHistoryItem[]>([]);

  onMounted(() => {
    loadTaskList();
  });

  const handleRetry = (row: any) => {
    console.log(row);
  };

  const loadTaskList = async () => {
    try {
      const params = {
        start: 0,
        limit: pagination.value.limit,
      };
      const res = await getTaskHistoryList(spaceId.value, params);
      tableList.value = res.list;
      pagination.value.count = res.count;
    } catch (error) {
      console.error(error);
    }
  };

  const handleSearch = (list: { [key: string]: string }) => {
    console.log(list);
  };

  const mergeOpRange = (operateRange: IOperateRange) => {
    return Object.values(operateRange)
      .map((arr) => (arr.length ? arr.join(',') : '*'))
      .join('.');
  };

  // const tableList = [
  //   {
  //     id: 1020,
  //     task_object: 'process',
  //     task_action: 'reload',
  //     task_data: {
  //       environment: 'production',
  //       operate_range: {
  //         set_ids: [1, 2],
  //         module_ids: [11, 12],
  //         service_ids: [101, 102],
  //         cc_process_ids: [1001, 1002],
  //       },
  //     },
  //     status: 'succeed',
  //     start_at: '2025-10-29 10:45:22',
  //     end_at: '2025-10-29 10:46:30',
  //     execution_time: '68',
  //   },
  //   {
  //     id: 1019,
  //     task_object: 'process',
  //     task_action: 'restart',
  //     task_data: {
  //       environment: 'staging',
  //       operate_range: {
  //         set_ids: [2, 3, 4],
  //         module_ids: [21, 22, 23],
  //         service_ids: [201, 202, 203],
  //         cc_process_ids: [2001, 2002, 2003],
  //       },
  //     },
  //     status: 'failed',
  //     start_at: '2025-10-29 09:42:00',
  //     end_at: '2025-10-29 09:45:20',
  //     execution_time: '200',
  //   },
  //   {
  //     id: 1018,
  //     task_object: 'process',
  //     task_action: 'start',
  //     task_data: {
  //       environment: 'testing',
  //       operate_range: {
  //         set_ids: [1],
  //         module_ids: [10],
  //         service_ids: [100],
  //         cc_process_ids: [1000],
  //       },
  //     },
  //     status: 'succeed',
  //     start_at: '2025-10-29 09:15:12',
  //     end_at: '2025-10-29 09:15:13',
  //     execution_time: '1',
  //   },
  //   {
  //     id: 1017,
  //     task_object: 'process',
  //     task_action: 'stop',
  //     task_data: {
  //       environment: 'production',
  //       operate_range: {
  //         set_ids: [5, 6],
  //         module_ids: [31, 32],
  //         service_ids: [301, 302],
  //         cc_process_ids: [3001, 3002],
  //       },
  //     },
  //     status: 'succeed',
  //     start_at: '2025-10-29 09:00:00',
  //     end_at: '2025-10-29 09:02:11',
  //     execution_time: '131',
  //   },
  //   {
  //     id: 1016,
  //     task_object: 'process',
  //     task_action: 'reload',
  //     task_data: {
  //       environment: 'testing',
  //       operate_range: {
  //         set_ids: [3, 4],
  //         module_ids: [23, 24],
  //         service_ids: [203, 204],
  //         cc_process_ids: [2003, 2004],
  //       },
  //     },
  //     status: 'running',
  //     start_at: '2025-10-29 08:58:30',
  //     end_at: '',
  //     execution_time: '0',
  //   },
  //   {
  //     id: 1015,
  //     task_object: 'process',
  //     task_action: 'restart',
  //     task_data: {
  //       environment: 'production',
  //       operate_range: {
  //         set_ids: [2, 3],
  //         module_ids: [21, 22],
  //         service_ids: [201, 202],
  //         cc_process_ids: [2001, 2002],
  //       },
  //     },
  //     status: 'succeed',
  //     start_at: '2025-10-29 08:45:50',
  //     end_at: '2025-10-29 08:47:50',
  //     execution_time: '120',
  //   },
  //   {
  //     id: 1014,
  //     task_object: 'process',
  //     task_action: 'stop',
  //     task_data: {
  //       environment: 'staging',
  //       operate_range: {
  //         set_ids: [6],
  //         module_ids: [36],
  //         service_ids: [306],
  //         cc_process_ids: [3006],
  //       },
  //     },
  //     status: 'failed',
  //     start_at: '2025-10-29 08:33:00',
  //     end_at: '2025-10-29 08:34:10',
  //     execution_time: '70',
  //   },
  //   {
  //     id: 1013,
  //     task_object: 'process',
  //     task_action: 'start',
  //     task_data: {
  //       environment: 'production',
  //       operate_range: {
  //         set_ids: [1, 2, 3],
  //         module_ids: [10, 20, 30],
  //         service_ids: [100, 200, 300],
  //         cc_process_ids: [1000, 2000, 3000],
  //       },
  //     },
  //     status: 'succeed',
  //     start_at: '2025-10-29 08:20:00',
  //     end_at: '2025-10-29 08:20:00',
  //     execution_time: '0',
  //   },
  //   {
  //     id: 1012,
  //     task_object: 'process',
  //     task_action: 'reload',
  //     task_data: {
  //       environment: 'testing',
  //       operate_range: {
  //         set_ids: [8, 9],
  //         module_ids: [40, 41],
  //         service_ids: [400, 401],
  //         cc_process_ids: [4000, 4001],
  //       },
  //     },
  //     status: 'succeed',
  //     start_at: '2025-10-29 08:10:10',
  //     end_at: '2025-10-29 08:11:00',
  //     execution_time: '50',
  //   },
  //   {
  //     id: 1011,
  //     task_object: 'process',
  //     task_action: 'restart',
  //     task_data: {
  //       environment: 'staging',
  //       operate_range: {
  //         set_ids: [3],
  //         module_ids: [22],
  //         service_ids: [202],
  //         cc_process_ids: [2002],
  //       },
  //     },
  //     status: 'succeed',
  //     start_at: '2025-10-29 07:58:00',
  //     end_at: '2025-10-29 07:59:00',
  //     execution_time: '60',
  //   },
  //   {
  //     id: 1010,
  //     task_object: 'process',
  //     task_action: 'stop',
  //     task_data: {
  //       environment: 'production',
  //       operate_range: {
  //         set_ids: [1, 2],
  //         module_ids: [10, 11],
  //         service_ids: [100, 101],
  //         cc_process_ids: [1000, 1001],
  //       },
  //     },
  //     status: 'succeed',
  //     start_at: '2025-10-29 07:50:00',
  //     end_at: '2025-10-29 07:51:05',
  //     execution_time: '65',
  //   },
  //   {
  //     id: 1009,
  //     task_object: 'process',
  //     task_action: 'start',
  //     task_data: {
  //       environment: 'production',
  //       operate_range: {
  //         set_ids: [4, 5],
  //         module_ids: [24, 25],
  //         service_ids: [204, 205],
  //         cc_process_ids: [2004, 2005],
  //       },
  //     },
  //     status: 'failed',
  //     start_at: '2025-10-29 07:40:00',
  //     end_at: '2025-10-29 07:41:00',
  //     execution_time: '60',
  //   },
  //   {
  //     id: 1008,
  //     task_object: 'process',
  //     task_action: 'restart',
  //     task_data: {
  //       environment: 'testing',
  //       operate_range: {
  //         set_ids: [7, 8],
  //         module_ids: [37, 38],
  //         service_ids: [307, 308],
  //         cc_process_ids: [3007, 3008],
  //       },
  //     },
  //     status: 'succeed',
  //     start_at: '2025-10-29 07:30:30',
  //     end_at: '2025-10-29 07:32:00',
  //     execution_time: '90',
  //   },
  //   {
  //     id: 1007,
  //     task_object: 'process',
  //     task_action: 'reload',
  //     task_data: {
  //       environment: 'production',
  //       operate_range: {
  //         set_ids: [1, 2, 3],
  //         module_ids: [10, 11, 12],
  //         service_ids: [100, 101, 102],
  //         cc_process_ids: [1000, 1001, 1002],
  //       },
  //     },
  //     status: 'succeed',
  //     start_at: '2025-10-29 07:20:00',
  //     end_at: '2025-10-29 07:22:00',
  //     execution_time: '120',
  //   },
  //   {
  //     id: 1006,
  //     task_object: 'process',
  //     task_action: 'stop',
  //     task_data: {
  //       environment: 'staging',
  //       operate_range: {
  //         set_ids: [3, 4],
  //         module_ids: [22, 23],
  //         service_ids: [202, 203],
  //         cc_process_ids: [2002, 2003],
  //       },
  //     },
  //     status: 'running',
  //     start_at: '2025-10-29 07:10:00',
  //     end_at: '',
  //     execution_time: '0',
  //   },
  //   {
  //     id: 1005,
  //     task_object: 'process',
  //     task_action: 'restart',
  //     task_data: {
  //       environment: 'production',
  //       operate_range: {
  //         set_ids: [5],
  //         module_ids: [25],
  //         service_ids: [205],
  //         cc_process_ids: [2005],
  //       },
  //     },
  //     status: 'succeed',
  //     start_at: '2025-10-29 07:00:00',
  //     end_at: '2025-10-29 07:02:00',
  //     execution_time: '120',
  //   },
  //   {
  //     id: 1004,
  //     task_object: 'process',
  //     task_action: 'start',
  //     task_data: {
  //       environment: 'testing',
  //       operate_range: {
  //         set_ids: [6, 7],
  //         module_ids: [30, 31],
  //         service_ids: [300, 301],
  //         cc_process_ids: [3000, 3001],
  //       },
  //     },
  //     status: 'failed',
  //     start_at: '2025-10-29 06:45:00',
  //     end_at: '2025-10-29 06:45:30',
  //     execution_time: '30',
  //   },
  //   {
  //     id: 1003,
  //     task_object: 'process',
  //     task_action: 'reload',
  //     task_data: {
  //       environment: 'production',
  //       operate_range: {
  //         set_ids: [1, 2, 3],
  //         module_ids: [10, 20, 30],
  //         service_ids: [100, 200, 300],
  //         cc_process_ids: [1000, 2000, 3000],
  //       },
  //     },
  //     status: 'succeed',
  //     start_at: '2025-10-29 06:30:00',
  //     end_at: '2025-10-29 06:31:00',
  //     execution_time: '60',
  //   },
  //   {
  //     id: 1002,
  //     task_object: 'process',
  //     task_action: 'restart',
  //     task_data: {
  //       environment: 'production',
  //       operate_range: {
  //         set_ids: [1, 2, 3],
  //         module_ids: [10, 20, 30],
  //         service_ids: [100, 200, 300],
  //         cc_process_ids: [1000, 2000, 3000],
  //       },
  //     },
  //     status: 'succeed',
  //     start_at: '2025-10-29 06:00:00',
  //     end_at: '2025-10-29 06:02:00',
  //     execution_time: '120',
  //   },
  //   {
  //     id: 1001,
  //     task_object: 'process',
  //     task_action: 'start',
  //     task_data: {
  //       environment: 'production',
  //       operate_range: {
  //         set_ids: [1, 2, 3],
  //         module_ids: [10, 20, 30],
  //         service_ids: [100, 200, 300],
  //         cc_process_ids: [1000, 2000, 3000],
  //       },
  //     },
  //     status: 'succeed',
  //     start_at: '2025-10-29 05:50:00',
  //     end_at: '2025-10-29 05:50:00',
  //     execution_time: '0',
  //   },
  // ];
</script>

<style scoped lang="scss">
  .task-list-wrap {
    padding: 28px 24px;
    background-color: #f5f7fa;
    height: 100%;
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
</style>
