<template>
  <bk-form-item :label="t('服务可见范围')" required property="env_apps">
    <bk-radio-group v-model="localPublic" @change="handlePublicChange">
      <bk-radio :label="true">{{ t('公开') }}</bk-radio>
      <bk-radio :label="false">{{ t('指定服务') }}</bk-radio>
    </bk-radio-group>
    <div v-if="!localPublic" class="env-apps-container">
      <div class="env-apps-content">
        <div v-for="(group, index) in localEnvApps" :key="`${index}_env_apps`" class="env-service-row">
          <div class="env-selector">
            <env-selector
              :class="{'is-error': showEnvError(index)}"
              :model-value="String(group.env_id)"
              :placeholder="t('请选择环境')"
              :use-default-trigger="true"
              :disabled-env-ids="getDisabledEnvIds(index)"
              @change="(env, isManual) => handleEnvInfoChange(index, env, isManual)">
              <template #trigger-prefix>
                <div class="selector-prefix-area">
                  <div class="add-require">
                    <span>{{ t('环境') }}</span>
                  </div>
                </div>
              </template>
            </env-selector>
            <div v-if="showEnvError(index)" class="error-msg">{{ t('环境不能为空') }}</div>
          </div>
          <div class="service-selector">
            <bk-select
              :class="{'is-error': showServiceError(index)}"
              :model-value="group.app_ids"
              multiple
              filterable
              :placeholder="t('请选择服务')"
              :input-search="false"
              :loading="getServiceLoadingState(index)"
              @change="(ids: number[]) => handleServiceChange(index, ids)">
              <template #prefix>
                <div
                  class="selector-prefix-area"
                  :style="{ width: '78px' }">
                  <div class="add-require">
                    <span>{{ t('绑定服务') }}</span>
                  </div>
                </div>
              </template>
              <bk-option
                v-for="service in getServiceListByIndex(index)"
                :key="service.id"
                :label="service.spec.name"
                :value="service.id" />
            </bk-select>
            <div v-if="showServiceError(index)" class="error-msg">{{ t('绑定服务不能为空') }}</div>
          </div>
          <div class="action-btns">
            <i
              class="bk-bscp-icon icon-add"
              :class="{ 'is-disabled': !canAddMore }"
              @click="handleAddGroup"></i>
            <i
              v-if="index"
              class="bk-bscp-icon icon-reduce"
              @click="handleRemoveGroup(index)"></i>
          </div>
        </div>
      </div>
      <p v-if="showRemovedTip && deletedApps.length > 0" class="tips">
        {{ t('提醒：修改可见范围后，服务') }}
        <span v-for="item in deletedApps" :key="item.id">【{{ item.spec.name }}】</span>
        {{ t('将不再引用此套餐') }}
      </p>
    </div>
  </bk-form-item>
</template>

<script setup lang="ts">
  import { ref, computed, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { getAppList } from '../api/index';
  import { IAppItem } from '../../types/app';
  import EnvSelector from './env-selector.vue';
  import { IEnvItem } from '../../types/env';

  const { t } = useI18n();

  interface EnvAppGroup {
    env_id: string;
    app_ids: number[];
  }

  const props = defineProps<{
    formData: {
      public: boolean;
      env_apps?: EnvAppGroup[];
    };
    spaceId: string;
    projectId: string;
    configType?: 'file' | '';
    showRemovedTip?: boolean;
    apps?: number[];
    maxGroups?: number; // 最大组数限制，不传则无限制
  }>();

  // eslint-disable-next-line func-call-spacing
  const emits = defineEmits<{
    (e: 'update:formData' | 'change', formData: any): void;
  }>();

  const serviceLoadingStates = ref<Record<number, boolean>>({});
  const serviceLists = ref<Record<number, IAppItem[]>>({});
  const deletedApps = ref<IAppItem[]>([]);
  const hasValidated = ref(false); // 是否已触发过校验

  const localPublic = computed({
    get: () => props.formData.public,
    set: (value) => {
      emits('update:formData', { ...props.formData, public: value });
    },
  });

  const localEnvApps = computed<EnvAppGroup[]>({
    get: () => props.formData?.env_apps || [],
    set: (value: EnvAppGroup[]) => {
      emits('update:formData', { ...props.formData, env_apps: value });
    },
  });

  const canAddMore = computed(() => {
    if (!props.maxGroups) return true;
    return localEnvApps.value.length < props.maxGroups;
  });

  // 获取当前组之外其他组已选的环境 ID 列表（用于禁用）
  const getDisabledEnvIds = (index: number): string[] => {
    return localEnvApps.value
      .filter((_, i) => i !== index)
      .map((group) => String(group.env_id));
  };

  const getServiceLoadingState = (index: number): boolean => {
    return serviceLoadingStates.value[index] || false;
  };

  const getServiceListByIndex = (index: number): IAppItem[] => {
    return serviceLists.value[index] || [];
  };

  const loadServiceList = async (index: number, envId: string) => {
    if (!envId) {
      serviceLists.value[index] = [];
      return;
    }
    serviceLoadingStates.value[index] = true;
    try {
      const query = {
        all: true,
      };
      const resp = await getAppList(props.spaceId, props.projectId, envId, query);
      let services = resp.details as IAppItem[];
      if (props.configType === 'file') {
        services = services.filter((service: IAppItem) => service.spec.config_type === 'file');
      }
      serviceLists.value[index] = services;
    } catch (e) {
      console.error(e);
      serviceLists.value[index] = [];
    } finally {
      serviceLoadingStates.value[index] = false;
    }
  };

  const handlePublicChange = (value: boolean) => {
    if (!value && localEnvApps.value.length === 0) {
      // 首次切换到"指定服务"时，添加一组空配置
      emits('update:formData', { ...props.formData, public: value, env_apps: [{ env_id: '', app_ids: [] }] });
      return;
    }
    emits('change', props.formData);
  };

  const handleEnvInfoChange = (index: number, env: IEnvItem, isManual?: boolean) => {
    const newEnvApps = [...localEnvApps.value];
    const envId = String(env.id);
    if (isManual && env) {
      newEnvApps[index] = { env_id: envId, app_ids: [] };
    } else if (env) {
      newEnvApps[index] = { env_id: envId, app_ids: newEnvApps[index].app_ids };
    }
    localEnvApps.value = newEnvApps;

    // 用户修改后清除错误提示
    hasValidated.value = false;

    if (env) {
      loadServiceList(index, envId);
    }
    emits('change', props.formData);
  };

  const handleServiceChange = (index: number, ids: number[]) => {
    const newEnvApps = [...localEnvApps.value];
    newEnvApps[index] = { ...newEnvApps[index], app_ids: ids };
    localEnvApps.value = newEnvApps;

    // 用户修改后清除错误提示
    hasValidated.value = false;

    if (props.showRemovedTip && props.apps) {
      const changed: IAppItem[] = [];
      const allSelectedApps = new Set<number>();
      newEnvApps.forEach((group) => group.app_ids.forEach((id) => allSelectedApps.add(id)));

      props.apps.forEach((id) => {
        if (!allSelectedApps.has(id)) {
          // 需要从所有组的服务列表中查找
          for (const list of Object.values(serviceLists.value)) {
            const app = list.find((item) => item.id === id);
            if (app) {
              changed.push(app);
              break;
            }
          }
        }
      });
      deletedApps.value = changed;
    }
    emits('change', props.formData);
  };

  const handleAddGroup = () => {
    if (!canAddMore.value) return;
    localEnvApps.value = [...localEnvApps.value, { env_id: '', app_ids: [] }];
    emits('change', props.formData);
  };

  const handleRemoveGroup = (index: number) => {
    if (localEnvApps.value.length <= 1) return;
    const newEnvApps = localEnvApps.value.filter((_, i) => i !== index);
    localEnvApps.value = newEnvApps;

    // 清理对应的服务列表缓存
    const newServiceLists = { ...serviceLists.value };
    delete newServiceLists[index];
    serviceLists.value = newServiceLists;

    emits('change', props.formData);
  };

  // 校验环境是否为空（仅在调用 validate 后显示错误）
  const showEnvError = (index: number): boolean => {
    if (!hasValidated.value || localPublic.value) return false;
    const group = localEnvApps.value[index];
    return !group?.env_id;
  };

  // 校验服务是否为空（仅在调用 validate 后显示错误）
  const showServiceError = (index: number): boolean => {
    if (!hasValidated.value || localPublic.value) return false;
    const group = localEnvApps.value[index];
    return !group?.app_ids || group.app_ids.length === 0;
  };

  // 初始化时加载第一组的服务列表
  watch(
    () => localEnvApps.value,
    (groups) => {
      groups.forEach((group, index) => {
        if (group.env_id && !serviceLists.value[index]) {
          loadServiceList(index, group.env_id);
        }
      });
    },
    { immediate: true, deep: true },
  );

  // 校验函数
  const validate = (): boolean => {
    if (localPublic.value) {
      return true;
    }
    // 标记已触发校验，开始显示错误提示
    hasValidated.value = true;
    // 指定服务模式下，需要校验每组配置
    if (!localEnvApps.value || localEnvApps.value.length === 0) {
      return false;
    }
    // 校验每组：env_id 和 app_ids 都不能为空
    return localEnvApps.value.every((group) => {
      return group.env_id && group.app_ids && group.app_ids.length > 0;
    });
  };

  defineExpose({
    validate,
  });
</script>

<style lang="scss" scoped>
  .tips {
    margin: 8px 0;
    line-height: 16px;
    font-size: 12px;
    color: #ff9c01;
  }
  .env-apps-container {
    margin-top: 8px;
  }
  .env-apps-content {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 12px 16px;
    border-radius: 2px;
    background-color: #f5f7fa;
  }
  .env-service-row {
    display: flex;
    gap: 12px;
    .env-selector {
      flex: 1;
      :deep(.bk-select) {
        &.is-error .env-select-trigger-wapper {
          border-color: #ea3636 !important;
        }
      }
    }
    .service-selector {
      flex: 2;
    }
    .selector-prefix-area {
      display: flex;
      align-items: center;
      height: 100%;
      padding: 0 8px;
      color: #4D4F56;
      font-size: 12px;
      background-color: #FAFBFD;
      border-right: 1px solid #c4c6cc;
      min-width: 54px;
      .add-require {
        position: relative;
        &:after {
          position: absolute;
          top: 0;
          width: 12px;
          color: #ea3636;
          margin-left: 2px;
          content: "*";
        }
      }
    }
    .is-error {
      :deep(.env-select-trigger-wapper) {
        border-color: #ea3636;
      }
      :deep(.bk-input) {
        border-color: #ea3636;
      }
    }
    .error-msg {
      font-size: 12px;
      line-height: 14px;
      white-space: normal;
      word-wrap: break-word;
      color: #ea3636;
      animation: form-error-appear-animation 0.15s;
      margin-top: 8px;
      &.is--key {
        white-space: nowrap;
      }
    }
    @keyframes form-error-appear-animation {
      0% {
        opacity: 0;
        transform: translateY(-30%);
      }
      100% {
        opacity: 1;
        transform: translateY(0);
      }
    }
    .action-btns {
      display: flex;
      align-items: center;
      justify-content: space-between;
      width: 38px;
      height: 32px;
      font-size: 14px;
      color: #979ba5;
      .bk-bscp-icon {
        cursor: pointer;
        &.is-disabled {
          cursor: not-allowed;
          opacity: 0.5;
        }
      }
      i:hover:not(.is-disabled) {
        color: #3a84ff;
      }
    }
  }
</style>
