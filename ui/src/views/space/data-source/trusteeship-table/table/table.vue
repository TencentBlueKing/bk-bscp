<template>
  <bk-loading style="min-height: 300px" :loading="tableLoading">
    <bk-table
      :data="tableData"
      :border="['outer']"
      :remote-pagination="true"
      :pagination="pagination"
      @page-limit-change="handlePageLimitChange"
      @page-value-change="loadTableList">
      <bk-table-column :label="$t('表格名称')">
        <template #default="{ row }">
          <bk-button
            v-if="row.spec"
            text
            theme="primary"
            @click="
              router.push({
                name: 'trusteeship-table-data-preview',
                params: { spaceId, id: row.id },
                query: { name: row.spec.table_name },
              })
            ">
            {{ row.spec.table_name }}
          </bk-button>
        </template>
      </bk-table-column>
      <bk-table-column :label="$t('表格描述')" prop="spec.table_memo" />
      <bk-table-column :label="$t('关联配置项')">
        <template #default="{ row }">
          <bk-button v-if="row.spec" text theme="primary" :disabled="row.citations === 0">
            {{ row.citations }}
          </bk-button>
        </template>
      </bk-table-column>
      <bk-table-column :label="$t('最近更新人')" prop="updator" />
      <bk-table-column :label="$t('最近更新时间')" prop="updatedAt" />
      <bk-table-column :label="$t('操作')">
        <template #default="{ row }">
          <div class="action-btns">
            <bk-button
              text
              theme="primary"
              @click="
                router.push({
                  name: 'edit-table-data',
                  params: { spaceId, id: row.id },
                  query: { name: row.spec.table_name },
                })
              ">
              {{ $t('编辑数据') }}
            </bk-button>
            <bk-button
              text
              theme="primary"
              @click="
                router.push({
                  name: 'edit-table-structure',
                  params: { spaceId, id: row.id },
                  query: { name: row.spec.table_name },
                })
              ">
              {{ $t('编辑表结构') }}
            </bk-button>
            <bk-popover
              theme="light trusteeship-table-actions-popover"
              placement="bottom-end"
              :popover-delay="[0, 100]"
              :arrow="false">
              <div class="more-actions">
                <Ellipsis class="ellipsis-icon" />
              </div>
              <template #content>
                <div class="config-actions">
                  <div class="action-item" @click="handleImportTable(row)">{{ $t('导入表格') }}</div>
                  <div class="action-item" @click="handleExportTable(row)">{{ $t('导出表格') }}</div>
                  <div :class="['action-item', { disabled: row.citations !== 0 }]" @click="handleDeleteTable(row)">
                    {{ $t('删除') }}
                  </div>
                </div>
              </template>
            </bk-popover>
          </div>
        </template>
      </bk-table-column>
    </bk-table>
  </bk-loading>
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { Ellipsis } from 'bkui-vue/lib/icon';
  import { getLocalTableList, deleteTableStructure } from '../../../../../api/kv-table';
  import { storeToRefs } from 'pinia';
  import { ILocalTableItem } from '../../../../../../types/kv-table';
  import { useRouter } from 'vue-router';
  import useGlobalStore from '../../../../../store/global';
  import useTablePagination from '../../../../../utils/hooks/use-table-pagination';
  import BkMessage from 'bkui-vue/lib/message';
  import { useI18n } from 'vue-i18n';

  const { t } = useI18n();
  const { pagination, updatePagination } = useTablePagination('trusteeship-table');
  const { spaceId } = storeToRefs(useGlobalStore());

  defineProps<{
    bkBizId: string;
  }>();

  const router = useRouter();

  const tableLoading = ref(false);
  const tableData = ref<ILocalTableItem[]>();

  onMounted(() => {
    loadTableList();
  });

  // 加载表格数据
  const loadTableList = async () => {
    tableLoading.value = true;
    try {
      const params = {
        start: (pagination.value.current - 1) * pagination.value.limit,
        limit: pagination.value.limit,
      };
      const res = await getLocalTableList(spaceId.value, params);
      pagination.value.count = Number(res.count);
      tableData.value = res.details;
    } catch (e) {
      console.log(e);
    } finally {
      tableLoading.value = false;
    }
  };

  const refresh = () => {
    pagination.value.current = 1;
    loadTableList();
  };

  const handlePageLimitChange = (val: number) => {
    updatePagination('limit', val);
  };

  const handleImportTable = (tableItem: ILocalTableItem) => {
    console.log(tableItem);
  };

  const handleExportTable = (tableItem: ILocalTableItem) => {
    console.log(tableItem);
  };

  const handleDeleteTable = async (tableItem: ILocalTableItem) => {
    if (tableItem.citations !== 0) return;
    try {
      await deleteTableStructure(spaceId.value, tableItem.id);
      refresh();
      BkMessage({ theme: 'success', message: t('删除表格成功') });
    } catch (error) {
      console.error(error);
    }
  };

  defineExpose({
    refresh,
  });
</script>

<style scoped lang="scss">
  .action-btns {
    display: flex;
    align-items: center;
    height: 100%;
    display: flex;
    align-items: center;
    height: 100%;
    .more-actions {
      display: flex;
      align-items: center;
      justify-content: center;
      margin-left: 8px;
      width: 16px;
      height: 16px;
      border-radius: 50%;
      cursor: pointer;
      &:hover {
        background: #dcdee5;
        color: #3a84ff;
      }
    }
    .ellipsis-icon {
      transform: rotate(90deg);
    }
    .bk-button {
      margin-right: 8px;
    }
  }
</style>

<style lang="scss">
  .trusteeship-table-actions-popover.bk-popover.bk-pop2-content {
    padding: 4px 0;
    border: 1px solid #dcdee5;
    box-shadow: 0 2px 6px 0 #0000001a;
    .config-actions {
      .action-item {
        padding: 0 12px;
        min-width: 58px;
        height: 32px;
        line-height: 32px;
        color: #63656e;
        font-size: 12px;
        cursor: pointer;
        &:hover {
          background: #f5f7fa;
        }

        &.disabled {
          color: #c4c6cc;
          cursor: not-allowed;
        }
      }
    }
  }
</style>
