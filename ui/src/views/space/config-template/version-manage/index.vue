<template>
  <div class="template-version-manage-page">
    <div class="page-header">
      <ArrowsLeft class="arrow-icon" @click="goToTemplateListPage" />
      <div v-if="templateName" class="title-name">
        {{ t('版本管理') }} <span class="line"></span> {{ templateName }}
      </div>
    </div>
    <div class="operation-area">
      <bk-button theme="primary" @click="openSelectVersionDialog">
        {{ t('新建版本') }}
      </bk-button>
      <SearchSelector
        v-show="versionDetailModeData.type !== 'create'"
        ref="searchSelectorRef"
        class="search-input"
        :search-field="searchField"
        :user-field="['reviser']"
        :placeholder="t('版本号/版本说明/创建人')"
        @search="handleSearch" />
    </div>
    <div class="version-content-area">
      <VersionFullTable
        v-if="!versionDetailModeData.open"
        :space-id="spaceId"
        :template-space-id="templateSpaceId"
        :template-id="templateId"
        :config-template-id="configTemplateId"
        :list="versionList"
        :pagination="pagination"
        :is-associated="isAssociated"
        @page-value-change="pagination.current = $event"
        @page-limit-change="handlePageLimitChange"
        @deleted="handleVersionDeleted"
        @select="handleOpenDetailTable($event, 'view')"
        @create="handleCreateVersion" />
      <VersionDetailTable
        v-else
        :space-id="spaceId"
        :template-space-id="templateSpaceId"
        :template-id="templateId"
        :config-template-id="configTemplateId"
        :list="versionList"
        :pagination="pagination"
        :type="versionDetailModeData.type"
        :version-id="versionDetailModeData.id"
        :is-associated="isAssociated"
        @created="handleCreatedVersion"
        @close="versionDetailModeData.open = false"
        @select="handleOpenDetailTable($event, 'view')" />
    </div>
    <bk-dialog
      :title="t('新建版本')"
      width="480"
      dialog-type="operation"
      :is-show="selectVersionDialog.open"
      :is-loading="allVersionListLoading"
      @confirm="handleSelectVersionConfirm"
      @closed="selectVersionDialog.open = false">
      <bk-form ref="selectVersionFormRef" form-type="vertical" :model="{ id: selectVersionDialog.id }">
        <bk-form-item :label="t('选择载入版本')" required property="id">
          <bk-select v-model="selectVersionDialog.id" :clearable="false" :filterable="true" :input-search="false">
            <bk-option v-for="item in allVersionList" v-overflow-title :key="item.id" :id="item.id" :label="item.name">
            </bk-option>
          </bk-select>
        </bk-form-item>
      </bk-form>
    </bk-dialog>
  </div>
</template>
<script lang="ts" setup>
  import { ref, computed, onMounted } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { useRoute, useRouter } from 'vue-router';
  import { ArrowsLeft } from 'bkui-vue/lib/icon';
  import useGlobalStore from '../../../../store/global';
  import useTablePagination from '../../../../utils/hooks/use-table-pagination';
  import { ITemplateVersionItem } from '../../../../../types/template';
  import { ICommonQuery } from '../../../../../types/index';
  import { getTemplateVersionList } from '../../../../api/template';
  import { getConfigTemplateDetail } from '../../../../api/config-template';
  import VersionFullTable from './version-full-table.vue';
  import VersionDetailTable from './version-detail/version-detail-table.vue';
  import SearchSelector from '../../../../components/search-selector.vue';
  import useConfigTemplateStore from '../../../../store/config-template';

  const { t } = useI18n();
  const { pagination, updatePagination } = useTablePagination('templateVersionManage');
  const { spaceId, showApplyPermDialog, permissionQuery } = storeToRefs(useGlobalStore());
  const configTemplateStore = useConfigTemplateStore();
  const { createVerson, isAssociated, perms } = storeToRefs(configTemplateStore);

  const route = useRoute();
  const router = useRouter();
  const templateName = ref('');
  const versionListLoading = ref(false);
  const versionList = ref<ITemplateVersionItem[]>([]);
  const allVersionListLoading = ref(false);
  const allVersionList = ref<{ id: number; name: string }[]>([]); // 全量版本列表，选择载入版本使用
  const selectVersionFormRef = ref();
  const selectVersionDialog = ref<{ open: boolean; id: number | string }>({
    open: false,
    id: '',
  });
  const versionDetailModeData = ref<{ open: boolean; type: string; id: number }>({
    open: false,
    type: 'view',
    id: 0,
  });
  const searchQuery = ref<{ [key: string]: string }>({});
  const searchField = [
    { field: 'revision_name', label: t('版本号') },
    { field: 'revision_memo', label: t('版本说明') },
    { field: 'creator', label: t('创建人') },
  ];

  const getRouteId = (id: string) => {
    if (id && typeof Number(id) === 'number') {
      return Number(id);
    }
    return 0;
  };

  const templateId = computed(() => getRouteId(route.params.templateId as string));
  const templateSpaceId = computed(() => getRouteId(route.params.templateSpaceId as string));
  const configTemplateId = computed(() => getRouteId(route.params.configTemplateId as string));

  onMounted(async () => {
    getTemplateDetail();
    await getVersionList();
    if (createVerson.value) {
      openSelectVersionDialog();
      configTemplateStore.$patch((state) => {
        state.createVerson = false;
      });
    }
  });

  const getTemplateDetail = async () => {
    try {
      const res = await getConfigTemplateDetail(spaceId.value, configTemplateId.value);
      templateName.value = res.bind_template.name;
    } catch (error) {
      console.error(error);
    }
  };

  const getVersionList = async () => {
    versionListLoading.value = true;
    const params: ICommonQuery = {
      start: (pagination.value.current - 1) * pagination.value.limit,
      limit: pagination.value.limit,
    };
    params.search = searchQuery.value;
    const res = await getTemplateVersionList(spaceId.value, templateSpaceId.value, templateId.value, params);
    versionList.value = res.details;
    pagination.value.count = res.count;
    versionListLoading.value = false;
  };

  const getAllVersionList = async () => {
    allVersionListLoading.value = true;
    const res = await getTemplateVersionList(spaceId.value, templateSpaceId.value, templateId.value, {
      start: 0,
      all: true,
    });
    allVersionList.value = res.details.map((item: ITemplateVersionItem) => {
      const { id, spec } = item;
      const name = spec.revision_memo ? `${spec.revision_name}(${spec.revision_memo})` : spec.revision_name;
      return { id, name };
    });
    allVersionListLoading.value = false;
  };

  const goToTemplateListPage = () => {
    router.push({
      name: 'config-template-list',
    });
  };

  const openSelectVersionDialog = async () => {
    if (!perms.value.update) {
      permissionQuery.value = {
        resources: [
          {
            biz_id: spaceId.value,
            basic: {
              type: 'process_and_config_management',
              action: 'update',
            },
          },
        ],
      };
      showApplyPermDialog.value = true;
      return;
    }
    selectVersionDialog.value.open = true;
    await getAllVersionList();
    selectVersionDialog.value.id = allVersionList.value.length > 0 ? allVersionList.value[0].id : '';
  };

  const handleVersionMenuSelect = (id: number) => {
    if (id === 0) {
      handleOpenDetailTable(0, 'create');
    } else {
      handleOpenDetailTable(id, 'view');
    }
  };

  const handleSelectVersionConfirm = async () => {
    await selectVersionFormRef.value.validate();
    handleOpenDetailTable(selectVersionDialog.value.id as number, 'create');
    selectVersionDialog.value.open = false;
  };

  const handleOpenDetailTable = (id: number, type: string) => {
    versionDetailModeData.value = {
      open: true,
      type,
      id,
    };
  };

  const handleVersionDeleted = () => {
    if (versionList.value.length === 1 && pagination.value.current > 1) {
      pagination.value.current -= 1;
    }
    getVersionList();
  };

  // 复制并新建版本
  const handleCreateVersion = (id: number) => {
    if (!perms.value.update) {
      permissionQuery.value = {
        resources: [
          {
            biz_id: spaceId.value,
            basic: {
              type: 'process_and_config_management',
              action: 'update',
            },
          },
        ],
      };
      showApplyPermDialog.value = true;
      return;
    }
    handleOpenDetailTable(id, 'create');
    selectVersionDialog.value.open = false;
  };

  // 新建版本成功后，重新拉取版本列表，并选中最新版本
  const handleCreatedVersion = async () => {
    pagination.value.current = 1;
    await getVersionList();
    handleVersionMenuSelect(versionList.value[0].id);
  };

  const handlePageLimitChange = (val: number) => {
    updatePagination('limit', val);
    refreshList();
  };

  const handleSearch = (list: { [key: string]: string }) => {
    searchQuery.value = list;
    refreshList();
  };

  const refreshList = (current = 1) => {
    pagination.value.current = current;
    getVersionList();
  };
</script>
<style lang="scss" scoped>
  .template-version-manage-page {
    height: 100%;
    background: #f5f7fa;
  }
  .page-header {
    display: flex;
    align-items: center;
    padding: 0 24px;
    height: 52px;
    background: #ffffff;
    box-shadow: 0 3px 4px 0 #0000000a;
    .arrow-icon {
      font-size: 24px;
      color: #3a84ff;
      cursor: pointer;
    }
    .title-name {
      padding: 14px 0;
      font-size: 16px;
      line-height: 24px;
      color: #313238;
      .line {
        display: inline-block;
        width: 1px;
        height: 16px;
        background: #dcdee5;
        margin: 0 12px;
      }
    }
  }
  .operation-area {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: 24px;
    padding: 0 24px;
    .bk-button {
      width: 104px;
    }
    .search-input {
      width: 320px;
    }
    .search-input-icon {
      padding-right: 10px;
      color: #979ba5;
      background: #ffffff;
    }
  }
  .version-content-area {
    padding: 16px 24px 24px;
    height: calc(100% - 110px);
    overflow: hidden;
  }
</style>
