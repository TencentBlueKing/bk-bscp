<template>
  <div class="use-package-apps">
    <div
      v-if="!showForm"
      :class="['select-app-trigger', { disabled: tplCounts === 0 }]"
      v-bk-tooltips="{
        disabled: tplCounts > 0,
        content: t('该套餐中没有可用配置文件，无法被导入到服务配置中'),
      }"
      @click="handleTriggerClick">
      <Plus class="plus-icon" />
      {{ t('新服务中使用') }}
    </div>
    <div v-else class="form-panel">
      <bk-form ref="formRef" :model="formData" :rules="rules" form-type="vertical">
        <!-- 所属环境 -->
        <bk-form-item :label="t('所属环境')" required property="envId">
          <env-selector
            v-model="formData.envId"
            :placeholder="t('请选择环境')"
            :use-default-trigger="true"
            @change="handleEnvChange" />
        </bk-form-item>
        <!-- 待使用模板套餐的服务 -->
        <bk-form-item :label="t('待使用模板套餐的服务')" property="appId" required>
          <bk-select
            v-model="formData.appId"
            :placeholder="formData.envId ? t('请选择服务') : t('请先选择环境')"
            :disabled="!formData.envId"
            :filterable="true"
            :input-search="false">
            <bk-option
              v-for="app in unBoundApps"
              :key="app.id"
              :id="app.id"
              :label="app.spec.name" />
          </bk-select>
        </bk-form-item>
      </bk-form>
      <!-- 按钮区域 -->
      <div class="form-actions">
        <bk-button
          theme="primary"
          :disabled="!formData.envId || !formData.appId"
          @click="handleConfirm">
            <LinkToApp class="link-icon" :id="formData.appId as number" />
            <span>{{ t('前往服务使用') }}</span>
        </bk-button>
        <bk-button @click="handleCancel">{{ t('取消') }}</bk-button>
      </div>
    </div>
    <div class="table-wrapper">
      <bk-loading :loading="boundAppsLoading">
        <div class="refresh-header">
          <span class="text">{{ t('当前使用此套餐的服务') }}</span>
          <right-turn-line class="refresh-button" :class="{ rotate: boundAppsLoading }" @click="getBoundApps" />
        </div>
        <bk-table :border="['outer']" :data="boundApps" :thead="{ isShow: false }" :empty-text="t('暂无数据')">
          <bk-table-column label="">
            <template #default="{ row }">
              <div v-if="row.app_id" class="app-info">
                <div class="app-info-left" @click="goToConfigPageImport(row.app_id)">
                  <div v-overflow-title class="name-text">{{ row.app_name }}</div>
                  <LinkToApp class="link-icon" :id="row.app_id" />
                </div>
                 <!-- 环境标签 -->
                <div
                  v-if="row.env_display"
                  class="env-tag"
                  :style="{
                    backgroundColor: ENV_TYPE_CONFIG?.[getEnvObj(row.env_display).type]?.bgColor || '#F5F7FA',
                    color: ENV_TYPE_CONFIG?.[getEnvObj(row.env_display).type]?.textColor || '#63656E',
                  }">
                  <i
                    :class="
                      `bk-bscp-icon ${ENV_TYPE_CONFIG?.[getEnvObj(row.env_display).type]?.iconClass || ''} env-icon`"
                    :style="{ color: ENV_TYPE_CONFIG?.[getEnvObj(row.env_display).type]?.iconColor || '#979BA5' }"></i>
                  {{ getEnvObj(row.env_display).envName }}
                </div>
              </div>
            </template>
          </bk-table-column>
        </bk-table>
        <bk-pagination class="table-pagination" small align="center" :show-limit="false" :show-total-count="false">
        </bk-pagination>
      </bk-loading>
    </div>
  </div>
</template>
<script lang="ts" setup>
  import { computed, onMounted, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRouter } from 'vue-router';
  import { storeToRefs } from 'pinia';
  import { Plus, RightTurnLine } from 'bkui-vue/lib/icon';
  import useGlobalStore from '../../../../../store/global';
  // import useUserStore from '../../../../../store/user';
  import useTemplateStore from '../../../../../store/template';
  import { getAppList } from '../../../../../api/index';
  import { getUnNamedVersionAppsBoundByPackage } from '../../../../../api/template';
  import { IAppItem } from '../../../../../../types/app';
  import { IPackageCitedByApps } from '../../../../../../types/template';
  import LinkToApp from '../components/link-to-app.vue';
  import EnvSelector from '../../../../../components/env-selector.vue';
  import { ENV_TYPE_CONFIG } from '../../../../../constants/env';
  import { getEnvObj } from '../../../../../utils/env';

  const router = useRouter();
  const { t } = useI18n();

  const { spaceId, projectId } = storeToRefs(useGlobalStore());
  // const { userInfo } = storeToRefs(useUserStore());
  const templateStore = useTemplateStore();
  const { currentTemplateSpace, currentPkg } = storeToRefs(templateStore);

  const props = defineProps<{
    tplCounts: number;
  }>();

  // 表单相关状态
  const showForm = ref(false);
  const formRef = ref<any>(null);
  const formData = ref<{
    envId: string;
    appId: number | undefined;
  }>({
    envId: '',
    appId: undefined,
  });
  const rules = {
    env: [
      {
        validator: (value: string) => !!value,
        message: t('请选择所属环境'),
        trigger: 'blur',
      },
    ],
    appId: [
      {
        validator: (value: number | undefined) => value !== undefined,
        message: t('请选择待使用模板套餐的服务'),
        trigger: 'blur',
      },
    ],
  };

  const userApps = ref<IAppItem[]>([]);
  const userAppListLoading = ref(false);
  const boundApps = ref<IPackageCitedByApps[]>([]);
  const boundAppsLoading = ref(false);

  const unBoundApps = computed(() => {
    const res = userApps.value.filter(
      (app) => boundApps.value.findIndex((item) => item.app_id === app.id) === -1 && app.spec.config_type === 'file',
    );
    return res;
  });

  watch(
    () => currentPkg.value,
    () => {
      boundApps.value = [];
      getBoundApps();
    },
  );

  onMounted(() => {
    getBoundApps();
  });

  const getUserApps = async () => {
    userAppListLoading.value = true;
    const params = {
      start: 0,
      all: true,
    };
    const res = await getAppList(spaceId.value, projectId.value, formData.value.envId,  params);
    userApps.value = res.details;
    userAppListLoading.value = false;
  };

  const getBoundApps = async () => {
    if (typeof currentPkg.value !== 'number') return;
    boundAppsLoading.value = true;
    const params = {
      start: 0,
      all: true,
    };
    const res = await getUnNamedVersionAppsBoundByPackage(
      spaceId.value,
      projectId.value,
      currentTemplateSpace.value,
      currentPkg.value as number,
      params,
    );
    boundApps.value = res.details;
    boundAppsLoading.value = false;
  };

  const goToConfigPageImport = (id: number) => {
    const { href } = router.resolve({
      name: 'service-config',
      params: { appId: id },
      query: { pkg_id: currentPkg.value },
    });
    window.open(href, '_blank');
  };

  // 点击「新服务中使用」按钮
  const handleTriggerClick = () => {
    if (props.tplCounts === 0) return;
    showForm.value = true;
  };

  // 环境变化
  const handleEnvChange = () => {
    formData.value.appId = undefined;
    getUserApps();
  };

  // 确认：前往服务使用
  const handleConfirm = async () => {
    const result = await formRef.value?.validate();
    if (!result) return;
    // 跳转至服务配置页面
    goToConfigPageImport(formData.value.appId as number);
    handleCancel();
  };

  // 取消：返回默认状态
  const handleCancel = () => {
    showForm.value = false;
    formRef.value?.clearValidate();
    formData.value = {
      envId: '',
      appId: undefined,
    };
  };
</script>
<style lang="scss" scoped>
  .use-package-apps {
    padding: 16px 24px;
    width: 280px;
    height: 100%;
    background: #ffffff;
  }
  .select-app-trigger {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 5px;
    height: 32px;
    line-height: 22px;
    border: 1px solid #c4c6cc;
    border-radius: 2px;
    color: #63656e;
    font-size: 14px;
    overflow: hidden;
    cursor: pointer;
    &.disabled {
      color: #dcdee5;
      border-color: #dcdee5;
      cursor: not-allowed;
    }
    .plus-icon {
      font-size: 20px;
    }
  }
  .app-info {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .app-info-left {
    flex: 1;
  }
  .app-option,
  .app-info-left {
    display: flex;
    align-items: center;
    overflow: hidden;
    cursor: pointer;
  }
  .table-wrapper {
    margin-top: 16px;
    .app-info-left {
      display: flex;
      align-items: center;
      overflow: hidden;
      &:hover {
        .link-icon {
          visibility: visible;
        }
      }
    }
    .link-icon {
      visibility: hidden;
      flex-shrink: 0;
      margin-left: 10px;
    }
    .table-pagination {
      margin-top: 16px;
    }
    .refresh-header {
      display: flex;
      align-items: center;
      padding: 0 16px;
      font-size: 12px;
      height: 41px;
      border: 1px solid #dcdee5;
      border-bottom: none;
      &:hover {
        background-color: #f0f1f5;
      }
      .text {
        margin-right: 16px;
      }
      .refresh-button {
        color: #3a84ff;
        font-size: 16px;
        cursor: pointer;
      }
    }
    :deep(.bk-exception-img) {
      height: 80px;
    }
  }
  .name-text {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  // 环境标签样式
  .env-tag {
    padding: 2px 6px;
    border-radius: 4px;
    font-size: 12px;
    line-height: 20px;
    white-space: nowrap;

    .env-icon {
      font-size: 16px;
    }
  }
  .rotate {
    animation: rotate 0.5s infinite linear;
  }

  @keyframes rotate {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(360deg);
    }
  }

  // 表单面板样式
  .form-panel {
    padding: 12px 16px 16px;
    border-radius: 2px;
    background-color: #F5F7FA;
    .form-title {
      margin-bottom: 16px;
      font-size: 14px;
      font-weight: 600;
      color: #313238;
    }
    .form-actions {
      display: flex;
      gap: 8px;
      margin-top: 24px;
      .bk-button + .bk-button {
        flex: 1;
      }
      .link-icon {
        color: #fff;
        margin-left: 0;
        margin-right: 6px;
      }
    }
  }
</style>
