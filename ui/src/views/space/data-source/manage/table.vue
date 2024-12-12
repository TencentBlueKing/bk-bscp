<template>
  <bk-loading style="min-height: 300px" :loading="tableLoading">
    <bk-table
      :data="tableData"
      :remote-pagination="true"
      :pagination="pagination"
      @page-limit-change="handlePageLimitChange"
      @page-value-change="handlePageCurrentChange">
      <bk-table-column :label="$t('数据源名称')">
        <template #default="{ row }">
          <div v-if="row.name">
            {{ row.name }}
          </div>
        </template>
      </bk-table-column>
      <bk-table-column :label="$t('描述')" prop="memo" />
      <bk-table-column :label="$t('类型')" prop="type" />
      <bk-table-column :label="$t('关联配置项')">
        <template #default="{ row }">
          <div v-if="row.name" class="hook-name">
            {{ row.config }}
          </div>
        </template>
      </bk-table-column>
      <bk-table-column :label="$t('最近更新人')" prop="updator" />
      <bk-table-column :label="$t('最近更新时间')" prop="updatedAt" />
      <bk-table-column :label="$t('操作')">
        <template #default="{ row }">
          <div class="action-btns">
            <bk-button text theme="primary" @click="handleEditClick(row)">{{ $t('编辑') }}</bk-button>
            <bk-button text theme="primary" @click="handleDeleteDataSource(row)">{{ $t('删除') }}</bk-button>
          </div>
        </template>
      </bk-table-column>
    </bk-table>
  </bk-loading>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import useTablePagination from '../../../../utils/hooks/use-table-pagination';

  const { pagination, updatePagination } = useTablePagination('dataSource');

  const tableLoading = ref(false);
  const tableData = ref([
    {
      name: '张三',
      memo: '123',
      type: 'MySqL',
      config: 1,
      updator: '张三',
      updatedAt: '2022-01-01',
    },
    {
      name: '李四',
      memo: '123',
      type: 'MySqL',
      config: 1,
      updator: '张三',
      updatedAt: '2022-01-01',
    },
    {
      name: '王五',
      memo: '123',
      type: 'MySqL',
      config: 1,
      updator: '张三',
      updatedAt: '2022-01-01',
    },
  ]);

  const handleEditClick = (dataSource: any) => {
    console.log(dataSource);
  };
  const handleDeleteDataSource = (dataSource: any) => {
    console.log(dataSource);
  };

  const handlePageLimitChange = (val: number) => {
    updatePagination('limit', val);
  };

  const handlePageCurrentChange = (val: number) => {
    pagination.value.current = val;
  };
</script>

<style scoped lang="scss"></style>
