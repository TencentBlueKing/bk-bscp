<template>
  <bk-select
    v-model="selectedEnvId"
    :popover-options="{ extCls: 'env-selector-popover', offset }"
    :clearable="false"
    :filterable="true"
    search-placeholder="搜索环境名称"
    :input-search="false"
    :disabled="props.disabled"
    :popover-min-width="240"
    @change="handleChange"
    @toggle="handleToggle">
    <!-- 自定义触发器：useDefaultTrigger 为 true 或传入了 #trigger 插槽时使用 -->
    <template v-if="useDefaultTrigger || hasTriggerSlot" #trigger>
      <slot
        name="trigger"
        :select-info="selectedEnvInfo"
        :is-open="isOpen">
        <!-- 默认触发器样式 -->
        <div class="env-select-trigger">
          <div v-if="selectedEnvInfo" class="env-display">
            <i
              :class="`
                bk-bscp-icon ${ENV_TYPE_CONFIG[selectedEnvInfo.group.type].iconClass} env-icon
              `"
              :style="{ color: ENV_TYPE_CONFIG[selectedEnvInfo.group.type]?.iconColor || '#979BA5' }">
            </i>
            <span>{{ selectedEnvInfo.env?.spec?.name || '' }}</span>
          </div>
          <div v-else class="placeholder-cls">{{ props.placeholder }}</div>
          <AngleDown :class="['angle-down-icon', { 'icon-rotate': isOpen }]" />
        </div>
      </slot>
    </template>
    <!-- 分组环境列表 -->
    <bk-option-group
      v-for="group in envGroups"
      :key="group.type"
      :label="group.name"
      collapsible>
      <template #label>
        <div class="env-group-label" @click.stop="toggleGroupCollapse(group.type)">
          <AngleUpFill class="collapse-icon" :class="{ 'is-collapsed': collapsedGroups[group.type] }" />
          <div
            class="group-name"
            :style="{
              backgroundColor: ENV_TYPE_CONFIG[group.type]?.bgColor || '#F5F7FA',
              color: ENV_TYPE_CONFIG[group.type]?.textColor || '#63656E',
            }">
            <i
              :class="`bk-bscp-icon ${ENV_TYPE_CONFIG[group.type]?.iconClass || ''} env-icon`"
              :style="{ color: ENV_TYPE_CONFIG[group.type]?.iconColor || '#979BA5' }"></i>
            <span>{{ group.name }}</span>
          </div>
        </div>
      </template>
      <bk-option
        v-for="env in group.envs"
        v-if="!collapsedGroups[group.type]"
        :key="env.id"
        :id="String(env.id)"
        :name="env.spec?.name || ''">
      </bk-option>
    </bk-option-group>
    <!-- 底部操作 -->
    <template #extension>
      <div class="env-footer" @click="handleToEnvManage">
        <i class="bk-bscp-icon icon-setting footer-icon"></i>
        <span>{{ t('环境管理') }}</span>
      </div>
    </template>
  </bk-select>
</template>

<script setup lang="ts">
  import { ref, computed, watch, useSlots } from 'vue';
  import { storeToRefs } from 'pinia';
  import { useI18n } from 'vue-i18n';
  import { useRouter } from 'vue-router';
  import { AngleUpFill, AngleDown } from 'bkui-vue/lib/icon';
  import { EnvType, IEnvItem, IEnvGroupItem } from '../../types/env';
  import { ENV_TYPE_CONFIG } from '../constants/env';
  import { getEnvList } from '../api/env';
  import useGlobalStore from '../store/global';

  const slots = useSlots();
  const hasTriggerSlot = computed(() => !!slots.trigger);

  const { t } = useI18n();
  const router = useRouter();
  const globalStore = useGlobalStore();
  const { spaceId: bizId, projectId } = storeToRefs(globalStore);

  const props = withDefaults(
    defineProps<{
      modelValue?: string;
      placeholder?: string;
      disabled?: boolean;
      offset?: number;
      useDefaultTrigger?: boolean; // 是否使用默认触发器样式
    }>(),
    {
      modelValue: '',
      placeholder: '请选择环境',
      disabled: false,
      offset: 6,
      useDefaultTrigger: false, // 默认 false，保持现有逻辑（使用 bk-select 原生触发器）
    },
  );

  // eslint-disable-next-line func-call-spacing
  const emits = defineEmits<{
    (e: 'update:modelValue', value: string): void;
    (e: 'change', env: IEnvItem): void;
  }>();

  const selectedEnvId = ref<string>(props.modelValue);

  // 每个 group 的折叠状态，默认全部展开
  const collapsedGroups = ref<Record<EnvType, boolean>>({
    [EnvType.PRODUCTION]: false,
    [EnvType.STAGING]: false,
    [EnvType.TESTING]: false,
    [EnvType.DEVELOPMENT]: false,
  });

  const toggleGroupCollapse = (groupType: EnvType) => {
    collapsedGroups.value[groupType] = !collapsedGroups.value[groupType];
  };

  // 环境类型名称映射
  const getEnvTypeName = (type: EnvType): string => {
    const nameMap: Record<EnvType, string> = {
      [EnvType.PRODUCTION]: t('生产环境'),
      [EnvType.STAGING]: t('预发布环境'),
      [EnvType.TESTING]: t('测试环境'),
      [EnvType.DEVELOPMENT]: t('开发环境'),
    };
    return nameMap[type] || type;
  };

  // 环境分组数据
  const envGroups = ref<IEnvGroupItem[]>([]);
  const isLoading = ref(false);

  // 获取环境列表并分组
  const fetchEnvGroups = async () => {
    if (!bizId.value || !projectId.value) {
      // 如果缺少参数，不使用内部数据，依赖外部传入的 envGroups
      return;
    }

    isLoading.value = true;
    try {
      const res = await getEnvList(bizId.value, projectId.value, { all: true });
      const data = res.data || {};

      const groups: IEnvGroupItem[] = [];

      // 字段名到 EnvType 的映射
      const fieldMapping: Array<{ field: string; type: EnvType }> = [
        { field: 'prod_environments', type: EnvType.PRODUCTION },
        { field: 'staging_environments', type: EnvType.STAGING },
        { field: 'test_environments', type: EnvType.TESTING },
        { field: 'dev_environments', type: EnvType.DEVELOPMENT },
      ];

      for (const { field, type } of fieldMapping) {
        const envList = data[field] as IEnvItem[] | undefined;
        if (Array.isArray(envList)) {
          groups.push({
            type,
            name: getEnvTypeName(type),
            envs: envList,
          });
        }
      }

      envGroups.value = groups;

      const firstGroup = groups.find((g) => g.envs.length > 0);
      if (firstGroup && !selectedEnvId.value) {
        selectedEnvId.value = String(firstGroup.envs[0].id);
      }
      handleChange(selectedEnvId.value);
    } catch (e) {
      console.error('获取环境列表失败', e);
      envGroups.value = [];
    } finally {
      isLoading.value = false;
    }
  };

  // 监听 bizId 和 projectId 变化，重新获取环境列表
  watch(
    () => [bizId.value, projectId.value],
    () => {
      fetchEnvGroups();
    },
    { immediate: true },
  );

  const selectedEnvInfo = computed(() => {
    const groups = envGroups.value;
    let result: { env: IEnvItem; group: IEnvGroupItem } | null = null;

    // eslint-disable-next-line no-restricted-syntax
    for (const group of groups) {
      const env = group.envs.find((e: IEnvItem) => String(e.id) === selectedEnvId.value);
      if (env) {
        result = { env, group };
        break;
      }
    }

    return result;
  });

  watch(
    () => props.modelValue,
    (val) => {
      selectedEnvId.value = val;
    }
  );

  const handleChange = (value: string) => {
    emits('update:modelValue', value);
    const info = selectedEnvInfo.value;
    if (info) {
      emits('change', info.env);
    }
  };

  const handleToEnvManage = () => {
    router.push({ name: 'env-manage' });
  };

  const isOpen = ref(false);
  const handleToggle = (val: boolean) => {
    isOpen.value = val;
  };
</script>

<style lang="scss" scoped>
  // 默认触发器样式
  .env-select-trigger {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    padding: 0 4px 0 12px;
    height: 32px;
    border: 1px solid #c4c6cc;
    border-radius: 2px;
    background: #fff;
    cursor: pointer;
    transition: border-color 0.2s;
    box-sizing: border-box;

    &:hover {
      border-color: #979ba5;
    }

    .env-display {
      display: flex;
      align-items: center;
      gap: 6px;
      overflow: hidden;

      .env-icon {
        font-size: 20px;
        flex-shrink: 0;
      }

      span {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        font-size: 12px;
        color: #63656e;
      }
    }

    .placeholder-cls {
      color: #c4c6cc;
      font-size: 12px;
    }

    .angle-down-icon {
      font-size: 20px;
      color: #979ba5;
      transition: transform 0.2s;
      flex-shrink: 0;

      &.icon-rotate {
        transform: rotate(-180deg);
      }
    }
  }
  .bk-select.is-focus .bk-select-trigger .env-select-trigger {
    border-color: #3A84FF;
    box-shadow: 0 0 3px #a3c5fd;
  }
  .bk-form-item.is-error .env-select-trigger {
    border-color: #ea3636;
    transition: all .15s;
  }
  .bk-select.is-disabled .bk-select-trigger .env-select-trigger {
    cursor: not-allowed;
    background-color: #fafbfd;
    border-color: #dcdee5;
  }
</style>

<style lang="scss">
  .env-selector-popover {
    border-radius: 4px !important;
    box-shadow: 0 2px 4px 0 #1919290d !important;
    .bk-select-option {
      padding: 0 12px !important;
    }
    .env-option-item {
      display: flex;
      align-items: center;
      gap: 8px;
      .env-type-dot {
        width: 8px;
        height: 8px;
        border-radius: 50%;
        flex-shrink: 0;
      }
      .env-name {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
    }
    .env-group-label {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      cursor: pointer;
      .collapse-icon {
        color: #979ba5;
        transition: transform 0.2s;
        &.is-collapsed {
          transform: rotate(-90deg);
        }
      }
      .group-name {
        display: flex;
        padding: 2px 6px 2px 4px;
        justify-content: center;
        align-items: center;
        gap: 4px;
        border-radius: 4px;
        line-height: 20px;
        .env-icon {
          font-size: 20px;
        }
        & > span {
          font-size: 12px;
          font-weight: 400;
        }
      }
    }
    .bk-select-extension {
      height: 32px !important;
      background-color: #F5F7FA !important;
      border-radius: 0 0 4px 4px !important;
    }
    .env-footer {
      flex: 1;
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      color: #4D4F56;
      font-size: 12px;
      cursor: pointer;
      .footer-icon {
        font-size: 16px;
        color: #979ba5;
      }
      &:hover {
        color: #3a84ff;
        .footer-icon {
          color: #3a84ff;
        }
      }
    }
  }
</style>
