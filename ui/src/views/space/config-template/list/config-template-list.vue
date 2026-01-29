<template>
  <section class="list-wrap">
    <div class="title">{{ $t('配置模板管理') }}</div>
    <div class="op-wrap">
      <bk-button class="create-btn" theme="primary" @click="handleCreate">{{ $t('新建') }}</bk-button>
      <SearchSelector
        ref="searchSelectorRef"
        :search-field="searchField"
        :user-field="['reviser']"
        :placeholder="$t('模板名称/文件名/更新人')"
        class="search-selector"
        @search="handleSearch" />
    </div>
    <div class="list-content">
      <PrimaryTable :data="templateList" :loading="tableLoading" class="border" row-key="id" cell-empty-content="--">
        <TableColumn :title="t('模板名称')">
          <template #default="{ row }: { row: IConfigTemplateItem }">
            <bk-button theme="primary" text @click="handleViewTemplate(row)">{{ row.spec.name }}</bk-button>
          </template>
        </TableColumn>
        <TableColumn :title="t('文件名')" col-key="spec.file_name" ellipsis> </TableColumn>
        <TableColumn :title="t('关联进程实例')">
          <template #default="{ row }: { row: IConfigTemplateItem }">
            <div class="associated-instance">
              <bk-button
                theme="primary"
                text
                :disabled="!row.is_proc_bound"
                v-bk-tooltips="{
                  content: `${t('模板进程')}: ${row.templateCount}\n${t('实例进程')}: ${row.instCount}`,
                  disabled: !row.is_proc_bound,
                  placement: 'right',
                }"
                @click="handleAssociatedProcess(row)">
                {{ row.instCount! + row.templateCount! }}
              </bk-button>
              <bk-tag
                v-if="!row.is_proc_bound"
                class="associated-btn"
                theme="info"
                @click="handleAssociatedProcess(row)">
                {{ t('立即关联') }}
              </bk-tag>
            </div>
          </template>
        </TableColumn>
        <TableColumn :title="t('更新人')">
          <template #default="{ row }: { row: IConfigTemplateItem }">
            <UserName :name="row.revision.reviser || '--'" />
          </template>
        </TableColumn>
        <TableColumn :title="t('更新时间')">
          <template #default="{ row }: { row: IConfigTemplateItem }">
            <span>{{ datetimeFormat(row.revision.update_at) }}</span>
          </template>
        </TableColumn>
        <TableColumn :title="t('操作')">
          <template #default="{ row }: { row: IConfigTemplateItem }">
            <div class="op-btns">
              <bk-button theme="primary" text @click="handleEdit(row)">{{ t('编辑') }}</bk-button>
              <bk-button
                theme="primary"
                :disabled="!row.is_proc_bound"
                text
                v-bk-tooltips="{
                  content: $t('未关联进程，无法进行配置下发'),
                  disabled: row.is_proc_bound,
                }"
                @click="handleConfigIssue(row.id)">
                {{ t('配置下发') }}
              </bk-button>
              <bk-button
                theme="primary"
                :disabled="!row.is_config_released"
                text
                v-bk-tooltips="{
                  content: $t('未下发配置，无法进行配置检查'),
                  disabled: row.is_config_released,
                }"
                @click="handleConfigCheck(row)">
                {{ t('配置检查') }}
              </bk-button>
              <TableMoreActions :operation-list="tableMoreOperationList" @operation="handleMoreActions(row, $event)" />
            </div>
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
  <AssociatedProcess
    v-model:is-show="isShowAssociatedProcess"
    :bk-biz-id="spaceId"
    :template-id="opTemplate.id"
    :template-name="opTemplate.templateName"
    :update-perm="perms.update"
    @no-perm="checkOpPerm('update')"
    @confirm="refresh" />
  <CreateConfigTemplate
    v-if="isShowCreateTemplate"
    :attribution="attribution"
    :bk-biz-id="spaceId"
    :template-space-id="templateSpaceId"
    @close="isShowCreateTemplate = false"
    @created="refresh" />
  <ConfigTemplateDetails
    v-if="isShowDetails"
    :bk-biz-id="spaceId"
    :template-id="opTemplate.id"
    :template-space-id="templateSpaceId"
    :is-associated="opTemplate.isAssociated"
    @operate="handleDetailOperate"
    @close="isShowDetails = false" />
  <DeleteConfirmDialog
    v-model:is-show="isShowDeleteDialog"
    :title="t('确认删除模板文件?')"
    @confirm="handleDeletConfirm">
    <div class="delete-content">
      <span class="label">{{ t('模板名称') }} :</span>
      <span class="value">{{ opTemplate.templateName }}</span>
    </div>
  </DeleteConfirmDialog>
  <ConfigCheck v-model:is-show="isShowConfigCheck" :bk-biz-id="spaceId" :id="opTemplate.id" />
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { getConfigTemplateList, deleteConfigTemplate } from '../../../../api/config-template';
  import { permissionCheck } from '../../../../api';
  import type { IConfigTemplateItem } from '../../../../../types/config-template';
  import { datetimeFormat } from '../../../../utils';
  import { useRouter } from 'vue-router';
  import SearchSelector from '../../../../components/search-selector.vue';
  import useTablePagination from '../../../../utils/hooks/use-table-pagination';
  import AssociatedProcess from './associated-process/index.vue';
  import useGlobalStore from '../../../../store/global';
  import CreateConfigTemplate from './create-config-template.vue';
  import ConfigTemplateDetails from './config-template-details.vue';
  import TableEmpty from '../../../../components/table/table-empty.vue';
  import UserName from '../../../../components/user-name.vue';
  import DeleteConfirmDialog from '../components/delete-confirm-dialog.vue';
  import useConfigTemplateStore from '../../../../store/config-template';
  import TableMoreActions from '../../../../components/table/table-more-actions.vue';
  import ConfigCheck from './config-check.vue';

  const { t } = useI18n();
  const { pagination, updatePagination } = useTablePagination('configTemplateList');
  const { spaceId, permissionQuery, showApplyPermDialog } = storeToRefs(useGlobalStore());
  const configTemplateStore = useConfigTemplateStore();
  const { perms } = storeToRefs(configTemplateStore);
  const router = useRouter();

  const searchField = [
    { field: 'template_name', label: t('模板名称') },
    { field: 'file_name', label: t('文件名') },
    { field: 'reviser', label: t('更新人') },
  ];
  const tableMoreOperationList = [
    {
      id: 'versionManage',
      name: t('版本管理'),
    },
    {
      id: 'delete',
      name: t('删除'),
    },
  ];
  const searchQuery = ref<{ [key: string]: string }>({});
  const isSearchEmpty = ref(false);
  const isShowAssociatedProcess = ref(false);
  const isShowCreateTemplate = ref(false);
  const isShowDetails = ref(false);
  const templateList = ref<IConfigTemplateItem[]>([]);
  const searchValue = ref<{ [key: string]: string }>();
  const searchSelectorRef = ref();
  const tableLoading = ref(false);
  const opTemplate = ref({
    id: 0,
    templateName: '',
    isAssociated: false,
  });
  const attribution = ref('');
  const templateSpaceId = ref(0);
  const isShowDeleteDialog = ref(false);
  const deletePendding = ref(false);
  const isShowConfigCheck = ref(false);
  const permCheckLoading = ref(false);

  onMounted(() => {
    loadConfigTemplateList();
    getPermData();
  });

  const loadConfigTemplateList = async () => {
    try {
      tableLoading.value = true;
      const paramas = {
        start: (pagination.value.current - 1) * pagination.value.limit,
        limit: pagination.value.limit,
        search: searchQuery.value,
      };
      const res = await getConfigTemplateList(spaceId.value, paramas);
      templateList.value = res.details.map((item: IConfigTemplateItem) => {
        return {
          ...item,
          instCount: item.attachment.cc_process_ids.length,
          templateCount: item.attachment.cc_template_process_ids.length,
        };
      });
      attribution.value = `${res.template_space.name}/${res.template_set.name}`;
      pagination.value.count = res.count;
      templateSpaceId.value = res.template_space.id;
    } catch (error) {
      console.error(error);
    } finally {
      tableLoading.value = false;
    }
  };

  const getPermData = async () => {
    permCheckLoading.value = true;
    const [createRes, updateRes, deleteRes, issuedRes] = await Promise.all([
      permissionCheck({
        resources: [
          {
            biz_id: spaceId.value,
            basic: {
              type: 'process_and_config_management',
              action: 'create',
            },
          },
        ],
      }),
      permissionCheck({
        resources: [
          {
            biz_id: spaceId.value,
            basic: {
              type: 'process_and_config_management',
              action: 'update',
            },
          },
        ],
      }),
      permissionCheck({
        resources: [
          {
            biz_id: spaceId.value,
            basic: {
              type: 'process_and_config_management',
              action: 'delete',
            },
          },
        ],
      }),
      permissionCheck({
        resources: [
          {
            biz_id: spaceId.value,
            basic: {
              type: 'process_and_config_management',
              action: 'generate_config',
            },
          },
        ],
      }),
    ]);
    perms.value.create = createRes.is_allowed;
    perms.value.update = updateRes.is_allowed;
    perms.value.delete = deleteRes.is_allowed;
    perms.value.issued = issuedRes.is_allowed;
    permCheckLoading.value = false;
  };

  const checkOpPerm = (perm: 'create' | 'update' | 'delete' | 'issued') => {
    if (permCheckLoading.value) return false;
    if (!perms.value[perm]) {
      permissionQuery.value = {
        resources: [
          {
            biz_id: spaceId.value,
            basic: {
              type: 'process_and_config_management',
              action: perm === 'issued' ? 'generate_config' : perm,
            },
          },
        ],
      };
      showApplyPermDialog.value = true;
      return false;
    }
    return true;
  };

  const handleSearch = (list: { [key: string]: string }) => {
    searchQuery.value = list;
    isSearchEmpty.value = Object.keys(list).length > 0;
    refresh();
  };

  const handlePageChange = (page: number) => {
    pagination.value.current = page;
  };

  const handlePageLimitChange = (limit: number) => {
    updatePagination('limit', limit);
    loadConfigTemplateList();
  };

  const handleCreate = () => {
    if (!checkOpPerm('create')) return;
    isShowCreateTemplate.value = true;
  };

  // 查看模板详情
  const handleViewTemplate = (template: IConfigTemplateItem) => {
    opTemplate.value = {
      id: template.id,
      templateName: `${template.spec.name} (${template.spec.file_name})`,
      isAssociated: template.is_proc_bound,
    };
    isShowDetails.value = true;
  };

  const handleClearSearch = () => {
    searchValue.value = {};
    isSearchEmpty.value = false;
    searchSelectorRef.value.clear();
  };

  const handleAssociatedProcess = (template: IConfigTemplateItem) => {
    opTemplate.value = {
      id: template.id,
      templateName: `${template.spec.name} (${template.spec.file_name})`,
      isAssociated: template.is_proc_bound,
    };
    isShowAssociatedProcess.value = true;
  };

  const handleEdit = (configTemplate: IConfigTemplateItem) => {
    if (!checkOpPerm('update')) return;
    configTemplateStore.$patch((state) => {
      state.createVerson = true;
    });
    // 跳转到版本管理新建版本
    router.push({
      name: 'config-template-version-manage',
      params: {
        templateSpaceId: templateSpaceId.value,
        templateId: configTemplate.attachment.template_id,
        configTemplateId: configTemplate.id,
      },
    });
  };

  // 配置下发
  const handleConfigIssue = (id: number) => {
    if (!checkOpPerm('issued')) return;
    router.push({
      name: 'config-issued',
      query: {
        templateIds: [id],
      },
    });
  };

  const handleGoVersionManage = (configTemplate: IConfigTemplateItem) => {
    configTemplateStore.$patch((state) => {
      state.isAssociated = configTemplate.is_proc_bound;
    });
    router.push({
      name: 'config-template-version-manage',
      params: {
        templateSpaceId: templateSpaceId.value,
        templateId: configTemplate.attachment.template_id,
        configTemplateId: configTemplate.id,
      },
    });
  };

  const refresh = () => {
    pagination.value.current = 1;
    loadConfigTemplateList();
  };

  const handleMoreActions = (template: IConfigTemplateItem, op: string) => {
    if (op === 'delete') {
      if (!checkOpPerm('delete')) return;
      handleDelete(template);
    } else {
      handleGoVersionManage(template);
    }
  };

  const handleDelete = (template: IConfigTemplateItem) => {
    isShowDeleteDialog.value = true;
    opTemplate.value = {
      id: template.id,
      templateName: template.spec.name,
      isAssociated: template.is_proc_bound,
    };
  };

  const handleConfigCheck = (template: IConfigTemplateItem) => {
    isShowConfigCheck.value = true;
    opTemplate.value = {
      id: template.id,
      templateName: template.spec.name,
      isAssociated: template.is_proc_bound,
    };
  };

  const handleDeletConfirm = async () => {
    try {
      deletePendding.value = true;
      await deleteConfigTemplate(spaceId.value, opTemplate.value.id);
      isShowDeleteDialog.value = false;
      refresh();
    } catch (error) {
      console.error(error);
    } finally {
      deletePendding.value = false;
    }
  };

  // 详情页操作
  const handleDetailOperate = (op: string) => {
    const configTemplate = templateList.value.find((template) => template.id === opTemplate.value.id);
    if (!configTemplate) return;
    if (op === 'edit') {
      handleEdit(configTemplate);
    } else if (op === 'delete') {
      handleDelete(configTemplate);
    } else if (op === 'issue') {
      handleConfigIssue(configTemplate.id);
    } else if (op === 'version-manage') {
      handleGoVersionManage(configTemplate);
    }
  };
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
    .associated-btn {
      line-height: 20px;
      height: 20px;
      cursor: pointer;
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

  .delete-content {
    text-align: center;
    .label {
      margin-right: 8px;
    }
    .value {
      color: #313238;
    }
  }
</style>
