<template>
  <section class="project-manage-page">
    <div class="project-manage-title">{{ t('项目管理') }}</div>
    <div class="project-manage-content">
        <div class="operate-area">
          <div class="btns">
            <bk-button theme="primary" @click="handleCreateProject">
              <Plus class="button-icon" />
              {{ t('新增项目') }}
            </bk-button>
          </div>
          <div class="filter-actions">
            <SearchSelector
              ref="searchSelectorRef"
              class="search-input"
              :search-field="searchField"
              :user-field="['creator']"
              :placeholder="t('项目名称/项目描述/创建人')"
              @search="handleSearch" />
          </div>
        </div>
        <div class="table-wrapper">
          <bk-loading style="min-height: 300px" :loading="listLoading">
            <bk-table
              class="project-table"
              show-overflow-tooltip
              :border="['outer']"
              :data="tableData">
              <bk-table-column :label="t('项目名称')" :min-width="280">
                <template #default="{ row }">
                  <div class="project-name-cell">
                    <span v-overflow-title class="name">{{ row.spec?.name }}</span>
                    <span class="code">{{ row.spec?.key }}</span>
                  </div>
                </template>
              </bk-table-column>
              <bk-table-column :label="t('项目描述')" :min-width="630">
                <template #default="{ row }">{{ row.spec?.memo || '--' }}</template>
              </bk-table-column>
              <bk-table-column :label="t('环境数')" :width="100">
                <template #default="{ row }">
                  <span class="env-count">{{ row.spec?.env_count }}</span>
                </template>
              </bk-table-column>
              <bk-table-column :label="t('服务点数')" :width="100">
                <template #default="{ row }">{{ row.spec?.app_count }}</template>
              </bk-table-column>
              <bk-table-column :label="t('创建人')" :width="160">
                <template #default="{ row }">{{ row.revision?.creator }}</template>
              </bk-table-column>
              <bk-table-column :label="t('创建时间')" :width="200">
                <template #default="{ row }">{{ datetimeFormat(row.revision?.create_at) }}</template>
              </bk-table-column>
              <bk-table-column
                :label="t('操作')"
                :width="140"
                :show-overflow-tooltip="false" fixed="right">
                <template #default="{ row }">
                  <div class="action-btns">
                    <bk-button text theme="primary" @click="handleEditProject(row)">{{ t('编辑项目') }}</bk-button>
                    <bk-button
                      text
                      theme="primary"
                      :disbaled="row.spec?.is_default"
                      @click="handleDeleteProject(row)">{{ t('删除项目') }}</bk-button>
                  </div>
                </template>
              </bk-table-column>
              <template #empty>
                <tableEmpty :is-search-empty="isSearchEmpty" @clear="clearSearchInfo" />
              </template>
            </bk-table>
            <bk-pagination
              v-model="pagination.current"
              class="table-pagination"
              location="left"
              :limit="pagination.limit"
              :layout="['total', 'limit', 'list']"
              :count="pagination.count"
              @change="handlePageChange"
              @limit-change="handlePageLimitChange" />
          </bk-loading>
        </div>
    </div>

    <!-- 创建/编辑项目弹窗 -->
    <ProjectFormDialog
      v-model="isFormDialogShow"
      :editing-item="editingItem"
      @success="loadProjectList" />

  </section>
</template>

<script setup lang="ts">
  import { ref, onMounted, watch, h } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { Plus } from 'bkui-vue/lib/icon';
  import { storeToRefs } from 'pinia';
  import Message from 'bkui-vue/lib/message';
  import useGlobalStore from '../../../store/global';
  import useTablePagination from '../../../utils/hooks/use-table-pagination';
  import tableEmpty from '../../../components/table/table-empty.vue';
  import ProjectFormDialog from './components/project-form-dialog.vue';
  import SearchSelector from '../../../components/search-selector.vue';
  import { InfoBox } from 'bkui-vue';
  import { getProjectList, deleteProject } from '../../../api/project';
  import type { IProjectItem } from '../../../../types/project';
  import { datetimeFormat } from '../../../utils';

  const { t } = useI18n();
  const { spaceId } = storeToRefs(useGlobalStore());
  const { pagination, updatePagination } = useTablePagination('projectList');

  const listLoading = ref(false);
  const tableData = ref<IProjectItem[]>([]);
  const isSearchEmpty = ref(false);
  const isFormDialogShow = ref(false);
  const editingItem = ref<Partial<IProjectItem>>({});
  const searchSelectorRef = ref();
  const searchField = [
    { field: 'name', label: t('项目名称') },
    { field: 'memo', label: t('项目描述') },
    { field: 'creator', label: t('创建人') },
  ];

  watch(
    () => spaceId.value,
    async () => {
      pagination.value.current = 1;
      await loadProjectList();
    },
  );

  onMounted(() => {
    loadProjectList();
  });

  // 加载项目列表
  const loadProjectList = async (searchConditions?: { [key: string]: string }) => {
    try {
      listLoading.value = true;
      const start = pagination.value.limit * (pagination.value.current - 1);
      const params = {
        start,
        limit: pagination.value.limit,
        search_condition: searchConditions || {},
      };
      const res = await getProjectList(spaceId.value, params);
      tableData.value = res.data?.projects || [];
      pagination.value.count = res.data?.count || 0;
    } catch (e) {
      console.error(e);
      tableData.value = [];
    } finally {
      listLoading.value = false;
    }
  };

  // 搜索
  const handleSearch = (searchConditions: { [key: string]: string }) => {
    pagination.value.current = 1;
    isSearchEmpty.value = Object.keys(searchConditions).length > 0;
    // 过滤掉空值，只传递有值的搜索条件
    const condition: { [key: string]: string } = {};
    for (const [key, value] of Object.entries(searchConditions)) {
      if (value) {
        condition[key] = value;
      }
    }
    loadProjectList(Object.keys(condition).length > 0 ? condition : undefined);
  };

  const clearSearchInfo = () => {
    searchSelectorRef.value?.clear();
    isSearchEmpty.value = false;
  };

  // 创建项目
  const handleCreateProject = () => {
    editingItem.value = {};
    isFormDialogShow.value = true;
  };

  // 编辑项目
  const handleEditProject = (row: IProjectItem) => {
    editingItem.value = { ...row };
    isFormDialogShow.value = true;
  };

  // 删除项目
  const handleDeleteProject = (row: IProjectItem) => {
    InfoBox({
      title: t('确认删除该项目'),
      subTitle: () => (
        h('div', [
          h('div', { class: 'pro-delete-title' }, `${t('项目名称')}：${row.spec.name}`),
          h('div', { class: 'pro-delete-tip' }, t('删除该项目后将无法恢复，请谨慎操作')),
        ])
      ),
      'ext-cls': 'pro-info-box',
      confirmText: t('删除'),
      cancelText: t('取消'),
      onConfirm: async () => {
        try {
          await deleteProject(spaceId.value, row.id as string);
          Message({ theme: 'success', message: t('删除项目成功') });
          if (tableData.value.length === 1 && pagination.value.current > 1) {
            pagination.value.current -= 1;
          }
          loadProjectList();
        } catch (e) {
          console.error(e);
        }
      },
    });
  };

  const handlePageChange = (val: number) => {
    pagination.value.current = val;
    loadProjectList();
  };

  const handlePageLimitChange = (val: number) => {
    updatePagination('limit', val);
    loadProjectList();
  };
</script>

<style lang="scss" scoped>
  .project-manage-page {
    background: #f5f7fa;
    height: 100%;
    display: flex;
    flex-direction: column;
  }

  .project-manage-title {
    padding: 14px 24px;
    height: 52px;
    background-color: #fff;
    line-height: 24px;
    flex-shrink: 0;
    box-shadow: 0 2px 4px #0D191929;
  }

  .project-manage-content {
    padding: 24px;
  }

  .operate-area {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;

    .button-icon {
      font-size: 18px;
    }
  }

  .search-input {
    width: 320px;
    background-color: #fff;
  }

  .project-name-cell {
    display: flex;
    align-items: center;
    width: 100%;
    overflow: hidden;
    .name {
      display: block;
      color: #313238;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      max-width: calc(100% - 100px);
    }
    .code {
      flex-shrink: 0;
      margin-left: 16px;
      color: #979ba5;
      font-size: 12px;
    }
  }

  .env-count {
    color: #3A84FF;
  }

  .action-btns {
    .bk-button:not(:last-child) {
      margin-right: 8px;
    }
  }

  .table-pagination {
    display: flex;
    align-items: center;
    padding: 12px;
    border: 1px solid #dcdee5;
    border-top: none;
    border-radius: 0 0 2px 2px;
    background: #ffffff;

    :deep(.bk-pagination-list.is-last) {
      margin-left: auto;
    }
  }
</style>
<style lang="scss">
  .pro-info-box {
    .bk-modal-footer {
      padding: 0 32px 24px !important;
      height: 56px !important;
    }
    .bk-dialog-footer{
      .bk-button.bk-button-primary {
        background-color: #ea3636;
        border-color: #ea3636;
        &:hover {
          background-color: #ff5656;
          border-color: #ff5656;
        }
      }
    }
    .bk-dialog-header {
        padding: 24px 32px 0 !important;
        .bk-dialog-title {
            margin: 16px 0 20px 0 !important;
        }
    }
    .bk-modal-content {
        padding: 0 32px 24px !important;
    }
    .bk-info-sub-title {
        text-align: left !important;
        line-height: 22px;
    }
    .pro-delete-title {
      font-size: 14px;
      color: #313238;
      margin-bottom: 16px;
    }
    .pro-delete-tip {
      font-size: 14px;
      color: #4D4F56;
      background-color: #F5F7FA;
      padding: 12px 16px;
    }
  }
</style>
