<template>
  <div class="data-preview-wrap">
    <SheetList :list="dataList" :view-sheet="viewSheet" @change="viewSheet = $event" />
    <div class="sheet-content">
      <div class="content-header">
        <div class="head-left">
          <div class="sheet-name">{{ viewSheet }}</div>
          <bk-button
            theme="primary"
            @click="
              router.push({ name: 'edit-table-data', params: { spaceId, id }, query: { name: route.query.name } })
            ">
            {{ $t('编辑数据') }}
          </bk-button>
          <bk-button @click="isShowImportTable = true">{{ $t('导入') }}</bk-button>
          <!-- <bk-button>{{ $t('导出') }}</bk-button> -->
        </div>
        <div class="head-right">
          <bk-input class="search-input">
            <template #suffix>
              <Search class="search-input-icon" />
            </template>
          </bk-input>
          <bk-select class="select-input" v-model="selectedValue" auto-focus filterable :placeholder="$t('常用查询')">
            <bk-option v-for="(item, index) in commonList" :id="item" :key="index" :name="item" />
          </bk-select>
          <div class="search-type-wrap">
            <div :class="['search-type-item', { active: searchType === 'basics' }]">{{ $t('基础查询') }}</div>
            <div :class="['search-type-item', { active: searchType === 'advanced' }]">{{ $t('高级查询') }}</div>
          </div>
        </div>
      </div>
      <bk-loading :loading="tableLoading" :min-height="300">
        <bk-table
          :data="tableData"
          :remote-pagination="true"
          :pagination="pagination"
          :border="['outer']"
          class="preview-data-table"
          show-overflow-tooltip
          @page-limit-change="handlePageLimitChange"
          @page-value-change="loadData">
          <bk-table-column v-for="item in fieldList" :key="item.name" :label="item.alias" :min-width="150">
            <template #default="{ row }">
              <div v-if="row.spec" style="height: 100%">
                <div v-if="Array.isArray(row.spec.content[item.name])" class="tag-list">
                  <bk-tag v-for="tag in row.spec.content[item.name]" :key="tag" radius="4px">
                    {{ tag }}
                  </bk-tag>
                </div>
                <span v-else-if="row.spec.content[item.name]">{{ row.spec.content[item.name] }}</span>
                <span v-else>--</span>
              </div>
            </template>
          </bk-table-column>
        </bk-table>
      </bk-loading>
    </div>
  </div>
  <ImportTable v-model:show="isShowImportTable" :bk-biz-id="spaceId" :id="id" :name="name" @refresh="refresh" />
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { getTableData } from '../../../../../../api/kv-table';
  import { ILocalTableEditData } from '../../../../../../../types/kv-table';
  import { Search } from 'bkui-vue/lib/icon';
  import SheetList from './sheet-list.vue';
  import useTablePagination from '../../../../../../utils/hooks/use-table-pagination';
  import ImportTable from '../../edit-table-data/import-table.vue';

  const { pagination, updatePagination } = useTablePagination('trusteeship-table-preview');

  const route = useRoute();
  const router = useRouter();

  const spaceId = ref(String(route.params.spaceId));
  const id = ref(Number(route.params.id));
  const name = ref(String(route.query.name));

  // @todo 手动创建表结构 无工作表 用表格名称作为工作表
  const dataList = ref([{ name: String(route.query.name) }]);

  const selectedValue = ref('');
  const searchType = ref('basics');
  const fieldList = ref<{ name: string; alias: string }[]>([]);
  const tableData = ref<ILocalTableEditData[]>([]);
  const tableLoading = ref(false);
  const commonList = ref([]);
  const isShowImportTable = ref(false);

  const viewSheet = ref(String(route.query.name));

  onMounted(() => {
    loadData();
  });

  const loadData = async () => {
    try {
      tableLoading.value = true;
      const query = {
        start: (pagination.value.current - 1) * pagination.value.limit,
        limit: pagination.value.limit,
      };
      const res = await getTableData(spaceId.value, id.value, query);
      fieldList.value = res.fields;
      tableData.value = res.details;
      updatePagination('count', Number(res.count));
    } catch (error) {
      console.error(error);
    } finally {
      tableLoading.value = false;
    }
  };

  const handlePageLimitChange = (val: number) => {
    updatePagination('limit', val);
    loadData();
  };

  const refresh = () => {
    updatePagination('current', 1);
    loadData();
  };
</script>

<style scoped lang="scss">
  .data-preview-wrap {
    display: flex;
    padding: 8px 0;
    background-color: #fff;
    height: calc(100% - 42px);
    .sheet-content {
      width: calc(100% - 220px);
      padding: 24px 24px 0 24px;
      .content-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 16px;
      }
      .head-left {
        display: flex;
        align-items: center;
        .sheet-name {
          padding: 0 16px;
          height: 32px;
          line-height: 32px;
          text-align: center;
          background: #f0f1f5;
          border-radius: 16px;
          font-weight: 700;
          font-size: 14px;
          color: #63656e;
          margin-right: 16px;
        }
        .bk-button:not(:last-child) {
          margin-right: 8px;
        }
      }
      .head-right {
        display: flex;
        gap: 8px;
        .search-input {
          width: 320px;
          .search-input-icon {
            padding-right: 10px;
            color: #979ba5;
            background: #ffffff;
          }
        }
        .select-input {
          width: 120px;
        }
        .search-type-wrap {
          display: flex;
          padding: 4px;
          background: #f0f1f5;
          height: 32px;
          .search-type-item {
            min-width: 72px;
            height: 24px;
            line-height: 24px;
            text-align: center;
            font-size: 12px;
            border-radius: 2px;
            color: #63656e;
            cursor: pointer;
            &.active {
              background: #fff;
              color: #3a84ff;
            }
          }
        }
      }
    }
  }

  .tag-list {
    padding: 8px 0;
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 4px;
    height: 100%;
  }
</style>
