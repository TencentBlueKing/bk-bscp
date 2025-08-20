<template>
  <div class="service-list-content">
    <div class="head-section">
      <bk-button
        v-cursor="{ active: props.permCheckLoading || !props.hasCreateServicePerm }"
        theme="primary"
        :class="{ 'bk-button-with-no-perm': props.permCheckLoading || !props.hasCreateServicePerm }"
        :disabled="props.permCheckLoading"
        @click="handleCreateServiceClick">
        <Plus class="create-icon" />
        {{ t('新建服务') }}
      </bk-button>
      <div class="head-right">
        <bk-checkbox v-model="onlyShowMyService" style="font-size: 12px" @change="handleChangeShowService">
          {{ $t('只显示我创建的服务') }}
        </bk-checkbox>
        <div class="panel-wrap">
          <div
            v-for="panel in typePanels"
            :key="panel.label"
            :class="['panel', { active: activeType === panel.name }]"
            @click="handlePanelChange(panel.name)">
            <span class="text">{{ `${panel.label}(${panel.count})` }}</span>
          </div>
        </div>
        <SearchSelector
          ref="searchSelectorRef"
          :search-filed="searchFiled"
          :user-filed="['reviser', 'creator']"
          :placeholder="t('搜索 服务别名、服务名称、服务描述、创建人、更新人')"
          class="search-app-name"
          @search="handleSearch" />
        <div class="panel-wrap">
          <div
            v-for="panel in showPanels"
            :key="panel.name"
            :class="['panel-icon', { active: activeShow === panel.name }]"
            @click="handleShowChange(panel.name)">
            <span :class="['bk-bscp-icon', panel.icon]"></span>
          </div>
        </div>
      </div>
    </div>
    <div class="content-body">
      <!-- 卡片视图 -->
      <template v-if="activeShow === 'card'">
        <bk-loading :style="{ height: '100%', width: '100%' }" :loading="isLoading">
          <!-- 空状态 -->
          <EmptyList
            v-if="isEmpty"
            :is-search-empty="isSearchEmpty"
            :has-create-service-perm="props.hasCreateServicePerm"
            :perm-check-loading="props.permCheckLoading"
            @create="handleCreateServiceClick"
            @clear="handleClearsearchQuery" />
          <template v-else>
            <div class="serving-list">
              <Card
                v-for="service in serviceList"
                :key="service.id"
                :service="service"
                @edit="handleEditService"
                @delete="handleDeleteService"
                @update="handleDeletedUpdate" />
            </div>
            <bk-pagination
              v-model="pagination.current"
              class="service-list-pagination"
              location="left"
              :layout="['total', 'limit', 'list']"
              :count="pagination.count"
              :limit="pagination.limit"
              @change="loadAppList"
              @limit-change="handleLimitChange" />
          </template>
        </bk-loading>
      </template>
      <!-- 表格视图 -->
      <ServiceTable
        v-else
        :space-id="props.spaceId"
        :data="serviceList"
        :pagination="pagination"
        :loading="isLoading"
        @page-change="handleTablePageChange"
        @limit-change="handleLimitChange"
        @edit="handleEditService"
        @delete="handleDeleteService">
        <template #empty>
          <EmptyList
            :is-search-empty="isSearchEmpty"
            :has-create-service-perm="props.hasCreateServicePerm"
            :perm-check-loading="props.permCheckLoading"
            @create="handleCreateServiceClick"
            @clear="handleClearsearchQuery" />
        </template>
      </ServiceTable>
    </div>
    <CreateService v-model:show="isCreateServiceOpen" @reload="loadAppList" />
    <EditService v-model:show="isEditServiceOpen" :service="editingService" @reload="loadAppList" />
    <bk-dialog
      v-model:is-show="isShowDeleteDialog"
      ext-cls="delete-service-dialog"
      :theme="'primary'"
      :dialog-type="'operation'"
      header-align="center"
      footer-align="center"
      @value-change="dialogInputStr = ''"
      :draggable="false"
      :quick-close="false">
      <div class="dialog-content">
        <div class="dialog-title">{{ t('确认删除服务？') }}</div>
        <div class="dialog-input">
          <div class="dialog-info">
            <div>
              {{ t('删除的服务') }}<span>{{ t('无法找回') }}</span>
              {{ t(',请谨慎操作!') }}
            </div>
            <div>{{ t('同时会删除服务密钥对服务的关联规则') }}</div>
          </div>
          <div class="tips">
            {{ t('请输入服务名') }} <span>{{ deleteService!.spec.name }}</span> {{ t('以确认删除') }}
          </div>
          <bk-input v-model="dialogInputStr" :placeholder="t('请输入')" />
        </div>
      </div>
      <template #footer>
        <div class="dialog-footer">
          <bk-button
            theme="danger"
            style="margin-right: 20px"
            :disabled="dialogInputStr !== deleteService!.spec.name"
            @click="handleDeleteConfirm">
            {{ t('删除') }}
          </bk-button>
          <bk-button @click="isShowDeleteDialog = false">{{ t('取消') }}</bk-button>
        </div>
      </template>
    </bk-dialog>
  </div>
</template>
<script setup lang="ts">
  import { ref, computed, watch, onMounted } from 'vue';
  import { storeToRefs } from 'pinia';
  import { useI18n } from 'vue-i18n';
  import { Plus } from 'bkui-vue/lib/icon';
  import useGlobalStore from '../../../../../store/global';
  import useUserStore from '../../../../../store/user';
  import { getAppList, getAppsConfigData, deleteApp } from '../../../../../api/index';
  import { IAppItem, IAppListQuery } from '../../../../../../types/app';
  import Card from './card.vue';
  import CreateService from './create-service.vue';
  import EditService from './edit-service.vue';
  import Message from 'bkui-vue/lib/message';
  // import { debounce } from 'lodash';
  import ServiceTable from './service-table.vue';
  import EmptyList from './empty-list.vue';
  import SearchSelector from '../../../../../components/search-selector.vue';

  const { permissionQuery, showApplyPermDialog } = storeToRefs(useGlobalStore());
  const { userInfo } = storeToRefs(useUserStore());
  const { t } = useI18n();

  const props = defineProps<{
    spaceId: string;
    permCheckLoading: boolean;
    hasCreateServicePerm: boolean;
  }>();

  const serviceList = ref<IAppItem[]>([]);
  const isLoading = ref(true);
  const searchQuery = ref<{ [key: string]: string }>({});
  const isCreateServiceOpen = ref(false);
  const isEditServiceOpen = ref(false);
  const dialogInputStr = ref('');
  const isShowDeleteDialog = ref(false);
  const onlyShowMyService = ref(localStorage.getItem('onlyShowMyService') === 'true');
  const deleteService = ref<IAppItem>();
  const editingService = ref<IAppItem>({
    id: 0,
    biz_id: 0,
    space_id: '',
    spec: {
      name: '',
      config_type: '',
      memo: '',
      alias: '',
      data_type: '',
      is_approve: true,
      approver: '',
      approve_type: 'or_sign',
    },
    revision: {
      creator: '',
      reviser: '',
      create_at: '',
      update_at: '',
    },
    permissions: {},
  });
  const pagination = ref({
    current: 1,
    limit: 50,
    count: 0,
  });
  const isSearchEmpty = ref(false);
  const typePanels = ref([
    { name: 'all', label: t('全部服务'), count: 0 },
    { name: 'file', label: t('文件型'), count: 0 },
    { name: 'kv', label: t('键值型'), count: 0 },
  ]);
  const showPanels = [
    { icon: 'icon-app-store', name: 'card' },
    { icon: 'icon-list', name: 'table' },
  ];
  const activeType = ref('all');
  const activeShow = ref('table');
  const searchFiled = [
    { field: 'alias', label: t('服务别名') },
    { field: 'name', label: t('服务名称') },
    { field: 'memo', label: t('服务描述') },
    { field: 'creator', label: t('创建人') },
    { field: 'reviser', label: t('更新人') },
  ];
  const searchSelectorRef = ref();

  // 查询条件
  const filters = computed(() => {
    const { current, limit } = pagination.value;

    const rules: IAppListQuery = {
      start: (current - 1) * limit,
      limit,
    };
    rules.search = searchQuery.value;
    if (activeType.value) {
      rules.config_type = activeType.value === 'all' ? '' : activeType.value;
    }
    return rules;
  });
  const isEmpty = computed(() => serviceList.value.length === 0);

  watch(
    () => [props.spaceId, activeType.value],
    () => {
      refreshSeviceList();
    },
  );

  onMounted(() => {
    loadAppList();
  });

  // 加载服务列表
  const loadAppList = async () => {
    isLoading.value = true;
    try {
      const bizId = props.spaceId;
      const resp = await getAppList(bizId, filters.value);
      const { file_apps_count, kv_apps_count, details, count } = resp;
      if (details.length > 0) {
        const appIds = details.map((item: IAppItem) => item.id);
        const appsConfigData = await getAppsConfigData(bizId, appIds);
        details.forEach((item: IAppItem, index: number) => {
          const { count, update_at } = appsConfigData.details[index];
          item.config = { count, update_at };
        });
      }
      serviceList.value = details;
      pagination.value.count = count;
      typePanels.value.forEach((panel) => {
        if (panel.name === 'all') {
          panel.count = file_apps_count + kv_apps_count;
        } else if (panel.name === 'file') {
          panel.count = file_apps_count;
        } else if (panel.name === 'kv') {
          panel.count = kv_apps_count;
        }
      });
    } catch (e) {
      console.error(e);
    } finally {
      isLoading.value = false;
    }
  };

  const handleCreateServiceClick = () => {
    if (props.hasCreateServicePerm) {
      isCreateServiceOpen.value = true;
    } else {
      permissionQuery.value = {
        resources: [
          {
            biz_id: props.spaceId,
            basic: {
              type: 'app',
              action: 'create',
            },
          },
        ],
      };
      showApplyPermDialog.value = true;
    }
  };

  // 编辑服务
  const handleEditService = (service: IAppItem) => {
    editingService.value = service;
    isEditServiceOpen.value = true;
  };

  // 刷新服务列表
  const refreshSeviceList = () => {
    pagination.value.current = 1;
    loadAppList();
  };

  // 删除服务
  const handleDeleteService = (service: IAppItem) => {
    deleteService.value = service;
    isShowDeleteDialog.value = true;
  };
  const handleDeleteConfirm = async () => {
    await deleteApp(deleteService.value!.id as number, deleteService.value!.biz_id);
    Message({
      message: t('删除服务成功'),
      theme: 'success',
    });
    loadAppList();
    isShowDeleteDialog.value = false;
  };

  // 删除服务后更新列表
  const handleDeletedUpdate = () => {
    if (serviceList.value.length === 1 && pagination.value.current > 1) {
      pagination.value.current -= 1;
    }
    loadAppList();
  };

  // 切换展示服务表格
  const handlePanelChange = (panel: string) => {
    activeType.value = panel;
  };

  // 切换展示服务类型
  const handleShowChange = (show: string) => {
    activeShow.value = show;
  };

  // 切换展示我创建的服务
  const handleChangeShowService = (val: boolean) => {
    if (val) {
      // 勾选只显示我的服务 清除创建人搜索项
      searchSelectorRef.value.clearCreator();
      searchQuery.value.creator = userInfo.value.username;
    } else {
      searchQuery.value.creator = '';
    }
    localStorage.setItem('onlyShowMyService', val.toString());
    refreshSeviceList();
  };

  const handleLimitChange = (limit: number) => {
    pagination.value.limit = limit;
    loadAppList();
  };

  // 表格当前页改变
  const handleTablePageChange = (page: number) => {
    pagination.value.current = page;
    loadAppList();
  };

  const handleSearch = (list: { [key: string]: string }) => {
    // 搜索项中包含创建人 取消勾选只显示我创建的服务
    if (list.creator) {
      onlyShowMyService.value = false;
      localStorage.setItem('onlyShowMyService', 'false');
    }
    searchQuery.value = list;
    isSearchEmpty.value = true;
    refreshSeviceList();
  };
  const handleClearsearchQuery = () => {
    searchQuery.value = {};
    searchSelectorRef.value.clear();
    isSearchEmpty.value = false;
    refreshSeviceList();
  };
</script>
<style lang="scss" scoped>
  .service-list-content {
    height: calc(100% - 90px);
    padding: 0 24px;
  }
  .head-section {
    display: flex;
    justify-content: space-between;
    padding: 24px 0;
    width: 100%;
    .create-icon {
      font-size: 22px;
    }
    .head-right {
      display: flex;
      gap: 12px;
      .search-app-name {
        width: 480px;
      }
    }
    .panel-wrap {
      display: flex;
      align-items: center;
      padding: 2px;
      height: 32px;
      line-height: 32px;
      background: #eaebf0;
      border-radius: 2px;
      color: #4d4f56;
      .panel {
        display: flex;
        align-items: center;
        height: 24px;
        padding: 0 12px;
        font-size: 12px;
        cursor: pointer;
        color: #4d4f56;
        &.active {
          background-color: #fff;
          color: #3a84ff;
        }
      }
      .panel-icon {
        display: flex;
        justify-content: center;
        align-items: center;
        font-size: 16px;
        height: 24px;
        width: 32px;
        cursor: pointer;
        &.active {
          background-color: #fff;
          color: #3a84ff;
        }
      }
    }
  }
  .content-body {
    display: flex;
    justify-content: center;
    padding-bottom: 24px;
    height: calc(100% - 64px);
    overflow-y: auto;
    overflow-x: hidden;
    .serving-list {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(304px, 1fr));
      gap: 24px;
      align-items: start;
      :deep(.bk-exception-description) {
        margin-top: 5px;
        font-size: 12px;
        color: #979ba5;
      }
      :deep(.bk-exception-footer) {
        margin-top: 5px;
      }
      .exception-actions {
        display: flex;
        font-size: 12px;
        color: #3a84ff;
        .divider-middle {
          display: inline-block;
          margin: 0 16px;
          width: 1px;
          height: 16px;
          background: #dcdee5;
        }
      }
    }
  }
  .service-list-pagination {
    margin-top: 16px;
    padding: 0 8px;
    :deep(.bk-pagination-list.is-last) {
      margin-left: auto;
    }
  }

  .dialog-content {
    text-align: center;
    margin-top: 48px;
    .dialog-title {
      font-size: 20px;
      color: #313238;
      line-height: 32px;
    }
    .dialog-input {
      margin-top: 16px;
      text-align: start;
      padding: 20px;
      background-color: #f4f7fa;
      .dialog-info {
        margin-bottom: 16px;
        span {
          color: red;
        }
      }
      .tips {
        margin-bottom: 8px;
        span {
          font-weight: bolder;
        }
      }
    }
  }
  .dialog-footer {
    .bk-button {
      width: 100px;
    }
  }
</style>

<style lang="scss">
  .delete-service-dialog {
    top: 40% !important;
    .bk-modal-body {
      padding-bottom: 104px !important;
    }
    .bk-modal-header {
      display: none;
    }
    .bk-modal-footer {
      height: auto !important;
      background-color: #fff !important;
      border-top: none !important;
      padding: 24px 24px 48px !important;
    }
  }
</style>
