<template>
  <div class="filter-wrap">
    <div class="env-tabs">
      <div
        v-for="env in envList"
        :key="env.value"
        :class="['env', { active: activeEnv === env.value }]"
        @click="handleChangeEnv(env.value)">
        {{ env.label }}
      </div>
    </div>
    <div class="filter">
      <template v-if="filterType === 'filter'">
        <bk-select
          v-model="filterValues[filter.value as keyof typeof filterValues]"
          v-for="filter in filterList"
          class="bk-select"
          :key="filter.value"
          :placeholder="filter.label"
          multiple
          @change="emits('search', { ...filterValues, environment: activeEnv })">
          <bk-option v-for="item in filter.list" :key="item.id" :value="item.name" :name="item.name">
            {{ item.name }}
          </bk-option>
        </bk-select>
        <bk-button class="transfer-button" text theme="primary" @click="filterType = 'expression'">
          <transfer class="icon" />{{ t('表达式') }}
        </bk-button>
      </template>
      <template v-else>
        <bk-input
          :model-value="filterValues[filter.value as keyof typeof filterValues]"
          v-for="filter in filterList"
          :key="filter.value"
          class="bk-input"
          placeholder="*"
          show-overflow-tooltips
          @change="handleInputChange(filter.value, $event)" />
        <bk-button class="transfer-button" text theme="primary" @click="filterType = 'filter'">
          <transfer class="icon" />{{ t('筛选') }}
        </bk-button>
      </template>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { Transfer } from 'bkui-vue/lib/icon';
  import { getProcessFilter } from '../../../../api/process';
  import type { IProcessFilterItem } from '../../../../../types/process';
  import { useI18n } from 'vue-i18n';

  const { t } = useI18n();

  const props = defineProps<{
    bizId: string;
  }>();
  const emits = defineEmits(['search']);

  const envList = [
    {
      label: t('正式'),
      value: '正式',
    },
    {
      label: t('体验'),
      value: '体验',
    },
    {
      label: t('测试'),
      value: '测试',
    },
  ];
  const filterList = ref<IProcessFilterItem[]>([
    {
      label: t('全部集群 (*)'),
      value: 'sets',
      list: [],
    },
    {
      label: t('全部模块 (*)'),
      value: 'modules',
      list: [],
    },
    {
      label: t('全部服务实例 (*)'),
      value: 'service_instances',
      list: [],
    },
    {
      label: t('全部进程别名 (*)'),
      value: 'process_aliases',
      list: [],
    },
    {
      label: t('全部 CC 进程 ID (*)'),
      value: 'cc_process_ids',
      list: [],
    },
  ]);
  const activeEnv = ref('正式');
  const filterValues = ref<{
    sets: string[];
    modules: string[];
    service_instances: string[];
    process_aliases: string[];
    cc_process_ids: string[];
  }>({
    sets: [],
    modules: [],
    service_instances: [],
    process_aliases: [],
    cc_process_ids: [],
  });
  const filterType = ref('filter');

  onMounted(() => {
    loadPerocessFilterList();
  });

  const loadPerocessFilterList = async () => {
    try {
      const res = await getProcessFilter(props.bizId);
      filterList.value.map((filter: IProcessFilterItem) => {
        filter.list = res[filter.value as keyof typeof res] as Array<{ name: string; id: string }>;
        return filter;
      });
    } catch (error) {
      console.error(error);
    }
  };

  const handleChangeEnv = (environment: string) => {
    activeEnv.value = environment;
    emits('search', { ...filterValues.value, environment });
  };

  const handleClearFilter = () => {
    filterValues.value = {
      sets: [],
      modules: [],
      service_instances: [],
      process_aliases: [],
      cc_process_ids: [],
    };
    emits('search', { ...filterValues.value, environment: activeEnv.value });
  };

  const handleInputChange = (key: string, value: string) => {
    filterValues.value[key as keyof typeof filterValues.value] = value.length > 0 ? value.split(',') : [];
    emits('search', { ...filterValues.value, environment: activeEnv.value });
  };

  defineExpose({
    clear: handleClearFilter,
  });
</script>

<style scoped lang="scss">
  .filter-wrap {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .env-tabs {
    display: flex;
    align-items: center;
    padding: 4px;
    height: 32px;
    line-height: 32px;
    background: #f0f1f5;
    border-radius: 2px;
    color: #4d4f56;
    font-size: 12px;
    .env {
      height: 24px;
      line-height: 24px;
      padding: 0 12px;
      cursor: pointer;
      color: #4d4f56;
      &.active {
        background-color: #fff;
        color: #3a84ff;
      }
    }
  }
  .filter {
    display: flex;
    align-items: center;
    gap: 10px;
    .bk-select,
    .bk-input {
      width: 136px;
    }
    .transfer-button {
      font-size: 14px;
      .icon {
        margin-right: 8px;
      }
    }
  }
</style>
