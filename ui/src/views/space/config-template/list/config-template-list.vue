<template>
  <section class="list-wrap">
    <div class="title">{{ $t('配置模板管理') }}</div>
    <div class="op-wrap">
      <bk-button class="create-btn" theme="primary" @click="isShowCreateTemplate = true">{{ $t('新建') }}</bk-button>
      <SearchSelector
        ref="searchSelectorRef"
        :search-field="searchField"
        :user-field="['reviser']"
        :placeholder="$t('模板名称/文件名/更新人')"
        class="search-selector"
        @search="handleSearch" />
    </div>
    <div class="list-content">
      <PrimaryTable :data="tableList" class="border" row-key="id" cell-empty-content="--">
        <TableColumn :title="t('模板名称')">
          <template #default="{ row }">
            <bk-button theme="primary" text @click="handleViewTemplate(row)">{{ row.template }}</bk-button>
          </template>
        </TableColumn>
        <TableColumn :title="t('文件名')">
          <template #default="{ row }">
            {{ row.file }}
          </template>
        </TableColumn>
        <TableColumn :title="t('关联进程实例')">
          <template #default="{ row }">
            <div class="associated-instance">
              <bk-button theme="primary" text :disabled="row.processInstance === 0">
                {{ row.processInstance }}
              </bk-button>
              <bk-tag
                v-if="row.processInstance === 0"
                :class="['associated-btn']"
                theme="info"
                @click="isShowAssociatedProcess = true">
                {{ t('立即关联') }}
              </bk-tag>
            </div>
          </template>
        </TableColumn>
        <TableColumn :title="t('更新人')"></TableColumn>
        <TableColumn :title="t('更新时间')"></TableColumn>
        <TableColumn :title="t('操作')">
          <template #default>
            <div class="op-btns">
              <bk-button theme="primary" text>{{ t('编辑') }}</bk-button>
              <bk-button theme="primary" text>{{ t('配置下发') }}</bk-button>
              <bk-button theme="primary" text>{{ t('版本管理') }}</bk-button>
              <bk-popover ref="opPopRef" theme="light" placement="bottom-end" :arrow="false">
                <div class="more-actions">
                  <Ellipsis class="ellipsis-icon" />
                </div>
                <template #content>
                  <div class="delete-btn">{{ t('删除') }}</div>
                </template>
              </bk-popover>
            </div>
          </template>
        </TableColumn>
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
  <AssociatedProcess :is-show="isShowAssociatedProcess" :bk-biz-id="spaceId" />
  <CreateConfigTemplate v-if="isShowCreateTemplate" @close="isShowCreateTemplate = false" />
  <ConfigTemplateDetails v-if="isShowDetails" @close="isShowDetails = false" />
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { Ellipsis } from 'bkui-vue/lib/icon';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import SearchSelector from '../../../../components/search-selector.vue';
  import useTablePagination from '../../../../utils/hooks/use-table-pagination';
  import AssociatedProcess from './associated-process/index.vue';
  import useGlobalStore from '../../../../store/global';
  import CreateConfigTemplate from './create-config-template.vue';
  import ConfigTemplateDetails from './config-template-details.vue';

  const { t } = useI18n();
  const { pagination, updatePagination } = useTablePagination('configTemplateList');
  const { spaceId } = storeToRefs(useGlobalStore());

  const searchField = [
    { field: 'template', label: t('模板名称') },
    { field: 'file', label: t('文件名') },
    { field: 'reviser', label: t('更新人') },
  ];
  const searchQuery = ref<{ [key: string]: string }>({});
  const isSearchEmpty = ref(false);
  const opPopRef = ref();
  const isShowAssociatedProcess = ref(false);
  const isShowCreateTemplate = ref(false);
  const isShowDetails = ref(false);

  const handleSearch = (list: { [key: string]: string }) => {
    searchQuery.value = list;
    isSearchEmpty.value = Object.keys(list).length > 0;
  };

  const handlePageChange = (page: number) => {
    updatePagination('current', page);
  };

  const handlePageLimitChange = (limit: number) => {
    updatePagination('limit', limit);
  };

  // 查看模板详情
  const handleViewTemplate = (row: { [key: string]: any }) => {
    console.log('view template', row);
    isShowDetails.value = true;
  };

  const tableList = ref([
    {
      id: 1,
      template: '配置模板A',
      file: 'config_a.yaml',
      processInstance: 0,
      reviser: '张三',
      updateTime: '2024-06-01 10:00:00',
    },
    {
      id: 2,
      template: '配置模板B',
      file: 'config_b.json',
      processInstance: 3,
      reviser: '李四',
      updateTime: '2024-06-02 14:30:00',
    },
    {
      id: 3,
      template: '配置模板C',
      file: 'config_c.ini',
      processInstance: 8,
      reviser: '王五',
      updateTime: '2024-06-03 09:15:00',
    },
  ]);
</script>

<style scoped lang="scss">
  .list-wrap {
    padding: 28px 24px;
    background: #f5f7fa;
    height: 100%;
    .title {
      font-weight: 700;
      font-size: 16px;
      color: #4d4f56;
      line-height: 24px;
    }
    .op-wrap {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin: 16px 0;
      .create-btn {
        width: 88px;
      }
      .search-selector {
        width: 400px;
      }
    }
  }
  .associated-instance {
    display: flex;
    align-items: center;
    gap: 8px;
    &:hover {
      .associated-btn {
        display: block;
      }
    }
    .associated-btn {
      display: none;
      line-height: 20px;
      height: 20px;
      cursor: pointer;
    }
  }
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
    .ellipsis-icon {
      font-size: 16px;
      transform: rotate(90deg);
      cursor: pointer;
    }
  }
  .delete-btn {
    margin: -12px;
    color: #4d4f56;
    width: 70px;
    cursor: pointer;
    line-height: 32px;
    height: 32px;
    padding: 0 12px;
    &:hover {
      background: #f5f7fa;
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
  .op-btns {
    display: flex;
    align-items: center;
    gap: 8px;
  }
</style>
