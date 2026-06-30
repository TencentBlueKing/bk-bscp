<template>
  <section class="env-manage-page">
    <div class="env-manage-title">{{ t('环境管理') }}</div>
    <div class="env-manage-content">
      <div class="operate-area">
        <div class="btns">
          <bk-button theme="primary" @click="handleCreateEnv">
            <Plus class="button-icon" />
            {{ t('新增环境') }}
          </bk-button>
        </div>
        <div class="filter-actions">
          <SearchSelector
            ref="searchSelectorRef"
            class="search-input"
            :search-field="searchField"
            :user-field="['creator']"
            :placeholder="t('环境名称/环境描述/创建人')"
            @search="handleSearch" />
        </div>
      </div>
      <bk-loading :loading="listLoading">
        <div class="env-group-list">
          <div
            v-for="group in envGroupList"
            :key="group.type"
            class="env-group">
            <div
               class="env-group-header"
               :style="{
                 backgroundColor: group.bgColor || '#F5F7FA',
                 color: group.textColor || '#63656E',
               }">
               <i
                :class="`bk-bscp-icon ${group.iconClass || ''} type-icon`"
                :style="{ color: group.iconColor || '#979BA5' }"></i>
               <span class="type-name">{{ group.name }}</span>
            </div>
            <div class="env-group-body">
              <!-- 有环境数据 -->
              <template v-if="group.items.length > 0">
                <div
                  class="env-card-wrapper"
                  v-for="item in group.items"
                  :key="item.id">
                  <div class="env-card">
                    <div class="card-header">
                      <div class="header-left">
                        <div class="env-name" v-overflow-title>{{ item.spec.name }}</div>
                        <div class="service-count">{{ t('共N个服务', { count: item.spec.app_count }) }}</div>
                      </div>
                      <div class="action-btns">
                        <bk-button text theme="primary" @click="handleEditEnv(item)">{{ t('编辑环境') }}</bk-button>
                        <bk-button
                          text
                          theme="primary"
                          :disbaled="item.spec?.is_default"
                          @click="handleDeleteEnv(item)">{{ t('删除环境') }}</bk-button>
                      </div>
                    </div>
                    <div class="card-mid">
                      <div class="card-desc" v-overflow-title >{{ item.spec.memo }}</div>
                      <div class="card-meta">
                          <i class="bk-bscp-icon icon-yonghu-2 meta-icon"></i>
                          {{ item.revision.creator }}
                          <i class="bk-bscp-icon icon-time-2 meta-icon"></i>
                          {{ datetimeFormat(item.revision.create_at) }}
                      </div>
                    </div>
                  </div>
                </div>
              </template>
              <!-- 空状态 -->
              <div v-else class="env-empty">
                <table-empty
                  :is-search-empty="isSearchEmpty"
                  :empty-title="t('暂无N', { type: group.name })"
                  @clear="clearSearchInfo"/>
              </div>
            </div>
          </div>
        </div>
      </bk-loading>
    </div>

    <!-- 创建/编辑环境弹窗 -->
    <EnvFormDialog
      v-model="isFormDialogShow"
      :editing-item="editingItem"
      @success="loadEnvList" />
  </section>
</template>

<script setup lang="ts">
  import { ref, computed, onMounted, watch, h } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { Plus } from 'bkui-vue/lib/icon';
  import { storeToRefs } from 'pinia';
  import Message from 'bkui-vue/lib/message';
  import { InfoBox } from 'bkui-vue';
  import useGlobalStore from '../../../store/global';
  import EnvFormDialog from './components/env-form-dialog.vue';
  import SearchSelector from '../../../components/search-selector.vue';
  import tableEmpty from '../../../components/table/table-empty.vue';
  import { getEnvList, deleteEnv } from '../../../api/env';
  import { IEnvItem, EnvType } from '../../../../types/env';
  import { ENV_TYPE_OPTIONS } from '../../../constants/env';
  import { datetimeFormat } from '../../../utils';

  const { t } = useI18n();
  const { spaceId, projectId } = storeToRefs(useGlobalStore());

  const listLoading = ref(false);
  const isSearchEmpty = ref(false);
  const isFormDialogShow = ref(false);
  const editingItem = ref<Partial<IEnvItem>>({});
  const searchSelectorRef = ref();
  const searchField = [
    { field: 'name', label: t('环境名称') },
    { field: 'memo', label: t('环境描述') },
    { field: 'creator', label: t('创建人') },
  ];

  const INIT_ENV_GROUPS = {
    [EnvType.PRODUCTION]: [],
    [EnvType.STAGING]: [],
    [EnvType.TESTING]: [],
    [EnvType.DEVELOPMENT]: [],
  };

  // 按类型分组的环境列表
  const envGroupData = ref<Record<EnvType, IEnvItem[]>>({
    ...INIT_ENV_GROUPS,
  });

  const envGroupList = computed(() => {
    return ENV_TYPE_OPTIONS.map((config) => ({
      ...config,
      items: envGroupData.value[config.type] || [],
    }));
  });

  watch(
    () => spaceId.value,
    async () => {
      await loadEnvList();
    },
  );

  onMounted(async () => {
    await loadEnvList();
  });

  // 加载环境列表
  const loadEnvList = async (searchCondition?: { [key: string]: string }) => {
    if (!projectId.value) return;
    try {
      listLoading.value = true;
      const query: any = { all: true };
      if (searchCondition && Object.keys(searchCondition).length > 0) {
        query.search_condition = searchCondition;
      }
      const res = await getEnvList(spaceId.value, projectId.value, query);
      const data = res.data || {};
      // 重置所有分组
      envGroupData.value = {
        [EnvType.PRODUCTION]: data.prod_environments || [],
        [EnvType.STAGING]: data.staging_environments || [],
        [EnvType.TESTING]: data.test_environments || [],
        [EnvType.DEVELOPMENT]: data.dev_environments || [],
      };
    } catch (e) {
      console.error(e);
      envGroupData.value = {
        ...INIT_ENV_GROUPS,
      };
    } finally {
      listLoading.value = false;
    }
  };

  // 搜索
  const handleSearch = (searchConditions: { [key: string]: string }) => {
    isSearchEmpty.value = Object.keys(searchConditions).length > 0;
    // 过滤掉空值，只传递有值的搜索条件
    const condition: { [key: string]: string } = {};
    for (const [key, value] of Object.entries(searchConditions)) {
      if (value) {
        condition[key] = value;
      }
    }
    loadEnvList(Object.keys(condition).length > 0 ? condition : undefined);
  };

  const clearSearchInfo = () => {
    searchSelectorRef.value?.clear();
    isSearchEmpty.value = false;
  };

  // 创建环境
  const handleCreateEnv = () => {
    editingItem.value = {};
    isFormDialogShow.value = true;
  };

  // 编辑环境
  const handleEditEnv = (row: IEnvItem) => {
    editingItem.value = { ...row };
    isFormDialogShow.value = true;
  };

  // 删除环境
  const handleDeleteEnv = (row: IEnvItem) => {
    InfoBox({
      title: t('确认删除该环境'),
      subTitle: () => (
        h('div', [
          h('div', { class: 'env-delete-title' }, `${t('环境名称')}：${row.spec.name}`),
          h('div', { class: 'env-delete-tip' }, t('删除该环境后将无法恢复，请谨慎操作')),
        ])
      ),
      'ext-cls': 'env-info-box',
      confirmText: t('删除'),
      cancelText: t('取消'),
      onConfirm: async () => {
        try {
          await deleteEnv(spaceId.value, projectId.value, String(row.id));
          Message({ theme: 'success', message: t('删除环境成功') });
          await loadEnvList();
        } catch (e) {
          console.error(e);
        }
      },
    });
  };
</script>

<style lang="scss" scoped>
  .env-manage-page {
    background: #f5f7fa;
    height: 100%;
  }

  .env-manage-title {
    padding: 14px 24px;
    height: 52px;
    background-color: #fff;
    line-height: 24px;
    box-shadow: 0 2px 4px #0D191929;
  }

  .env-manage-content {
    padding: 24px 200px;
    height: calc(100% - 52px);
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

  .bk-nested-loading {
    height: calc(100% - 48px);
  }

  .env-group-list {
    height: 100%;
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    grid-template-rows: repeat(2, 1fr);
    gap: 24px;
  }

  .env-group {
    display: flex;
    flex-direction: column;
    padding: 4px;
    border-radius: 4px;
    box-shadow: 0 2px 4px #0D191929;
    background-color: #fff;
    overflow: hidden;
  }

  .env-group-header {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 10px 16px;
    margin-bottom: 16px;

    .type-icon {
      font-size: 20px;
    }

    .type-name {
      font-size: 14px;
      font-weight: 700;
      line-height: 22px;
    }
  }

  .env-group-body {
    flex: 1;
  }

  .env-card-wrapper {
    padding: 0 24px 8px;
  }

  .env-card {
    display: flex;
    flex-direction: column;
    margin-top: 8px;
    border-bottom: 1px solid #DCDEE5;
    color: #979BA5;
    line-height: 20px;
  }

  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 8px;
    .header-left {
      flex: 1;
      display: flex;
      align-items: center;
      gap: 8px;
      min-width: 0;
    }
    .env-name {
      font-weight: 600;
      font-size: 14px;
      color: #313238;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      min-width: 0;
    }

    .service-count {
      padding: 0 8px;
      font-size: 12px;
      color: #979ba5;
      background-color: #F0F1F5;
      border-radius: 2px;
      white-space: nowrap;
    }

    .action-btns {
      .bk-button {
        margin-left: 16px;
      }
    }
  }

  .card-mid {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 8px;
    .card-desc {
        flex: 1;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }
    .card-meta {
      display: flex;
      align-items: center;
      .meta-icon {
        font-size: 16px;
        margin-left: 16px;
        margin-right: 4px;
      }
    }
  }

  .env-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 40px 20px;

    .empty-img {
      width: 80px;
      height: auto;
      opacity: 0.6;
    }

    .empty-text {
      margin-top: 12px;
      font-size: 12px;
      color: #979ba5;
    }
  }
</style>
<style lang="scss">
  .env-info-box {
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
    .env-delete-title {
      font-size: 14px;
      color: #313238;
      margin-bottom: 16px;
    }
    .env-delete-tip {
      font-size: 14px;
      color: #4D4F56;
      background-color: #F5F7FA;
      padding: 12px 16px;
    }
  }
</style>
