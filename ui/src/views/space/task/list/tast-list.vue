<template>
  <div class="title">{{ $t('任务历史') }}</div>
  <bk-search-select
    v-model="searchValue"
    class="search-select"
    :data="searchList"
    :placeholder="t('模板名称/文件名/更新人')"
    unique-select />
  <PrimaryTable class="border">
    <TableColumn col-key="id" title="ID"></TableColumn>
    <TableColumn col-key="id" :title="t('任务对象')"></TableColumn>
    <TableColumn col-key="id" :title="t('动作')"></TableColumn>
    <TableColumn col-key="id" :title="t('环境类型')"></TableColumn>
    <TableColumn col-key="id" :title="t('操作范围')"></TableColumn>
    <TableColumn col-key="id" :title="t('执行账户')"></TableColumn>
    <TableColumn col-key="id" :title="t('开始时间')"></TableColumn>
    <TableColumn col-key="id" :title="t('结束时间')"></TableColumn>
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
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import useTablePagination from '../../../../utils/hooks/use-table-pagination';
  import TableEmpty from '../../../../components/table/table-empty.vue';

  const { t } = useI18n();
  const { pagination, updatePagination } = useTablePagination('clientSearch');

  const searchValue = ref([]);
  const isSearchEmpty = ref(false);

  const searchList = [
    {
      name: t('模板名称'),
      id: 'name',
    },
    {
      name: t('文件名'),
      id: 'file-name',
    },
    {
      name: t('更新人'),
      id: 'updater',
    },
  ];

  const handleRetry = (row: any) => {
    console.log(row);
  };
</script>

<style scoped lang="scss">
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
