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
          :class="['bk-select', { issued: isIssued }]"
          :key="filter.value"
          :placeholder="filter.label"
          multiple
          @change="emits('search', { ...filterValues, environment: activeEnv })">
          <bk-option
            v-for="item in filter.list"
            :key="item.id"
            :value="filter.value === 'cc_process_ids' ? item.id : item.name"
            :name="item.name"
            :class="['range-select-option', { issued: isIssued }]">
            <div class="name-text">
              <bk-overflow-title type="tips" resizeable>{{ item.name }}</bk-overflow-title>
            </div>
          </bk-option>
        </bk-select>
        <bk-button class="op-btn" text theme="primary" @click="filterType = 'expression'">
          <transfer class="icon" />{{ t('表达式') }}
        </bk-button>
      </template>
      <template v-else>
        <bk-input
          :model-value="filterValues[filter.value as keyof typeof filterValues]"
          v-for="filter in filterList"
          :key="filter.value"
          :class="['bk-input', { issued: isIssued }]"
          placeholder="*"
          show-overflow-tooltips
          @change="handleInputChange(filter.value, $event)" />
        <bk-button class="op-btn" text theme="primary" @click="filterType = 'filter'">
          <transfer class="icon" />{{ t('筛选') }}
        </bk-button>
      </template>
      <bk-button v-if="isIssued" class="op-btn" text theme="primary" @click="handleClearFilter">
        <Del class="icon" />
        {{ t('清空') }}
      </bk-button>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onMounted, computed, watch } from 'vue';
  import { Transfer, Del } from 'bkui-vue/lib/icon';
  import { getProcessFilter } from '../../../../api/process';
  import type { IProcessFilterItem } from '../../../../../types/process';
  import { useI18n } from 'vue-i18n';
  import { useRoute } from 'vue-router';
  import { storeToRefs } from 'pinia';
  import useTaskStore from '../../../../store/task';

  const { t } = useI18n();
  const route = useRoute();

  const taskStore = useTaskStore();
  const { taskDetail, filterFlag } = storeToRefs(taskStore);

  const props = withDefaults(
    defineProps<{
      bkBizId: string;
      isIssued?: boolean; // 是否是配置下发
      processIds?: string[];
    }>(),
    {
      isIssued: false,
    },
  );
  const emits = defineEmits(['search']);

  watch(
    () => props.processIds,
    () => {
      if (props.processIds && props.processIds?.length > 0) {
        filterValues.value.cc_process_ids = props.processIds.map((id) => Number(id));
        emits('search', { ...filterValues.value, env: activeEnv.value });
      }
    },
  );

  const envList = computed(() => {
    if (props.isIssued) {
      return [
        {
          label: t('正式'),
          value: '3',
        },
        {
          label: t('体验'),
          value: '2',
        },
      ];
    }
    return [
      {
        label: t('正式'),
        value: '3',
      },
      {
        label: t('体验'),
        value: '2',
      },
      {
        label: t('测试'),
        value: '1',
      },
    ];
  });
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
      label: t('全部进程 (*)'),
      value: 'process_aliases',
      list: [],
    },
    {
      label: t('全部 process_id (*)'),
      value: 'cc_process_ids',
      list: [],
    },
  ]);
  const activeEnv = ref('3');
  const filterValues = ref<{
    sets: string[];
    modules: string[];
    service_instances: string[];
    process_aliases: string[];
    cc_process_ids: number[];
  }>({
    sets: [],
    modules: [],
    service_instances: [],
    process_aliases: [],
    cc_process_ids: [],
  });
  const filterType = ref('filter');

  onMounted(() => {
    if (route.query.cc_process_id) {
      filterValues.value.cc_process_ids.push(Number(route.query.cc_process_id));
      emits('search', { ...filterValues.value, environment: activeEnv.value });
    }
    if (filterFlag.value) {
      console.log('taskDetail filter', taskDetail.value.operate_range);
      const {operate_range} = taskDetail.value;
      filterValues.value = {
        sets: operate_range.set_names,
        modules: operate_range.module_names,
        service_instances: operate_range.service_names,
        process_aliases: operate_range.cc_process_names,
        cc_process_ids: operate_range.cc_process_ids,
      };
      taskStore.$patch({ filterFlag: false });
      emits('search', { ...filterValues.value, environment: activeEnv.value });
    }
    loadPerocessFilterList();
  });

  const loadPerocessFilterList = async () => {
    try {
      const res = await getProcessFilter(props.bkBizId);
      filterList.value.map((filter: IProcessFilterItem) => {
        filter.list = res[filter.value as keyof typeof res] as Array<{ name: string; id: number }>;
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
    // @ts-ignore
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
      &.issued {
        width: 162px;
      }
    }
    .op-btn {
      font-size: 14px;
      .icon {
        margin-right: 8px;
      }
    }
  }

  .range-select-option {
    .name-text {
      max-width: 86px;
    }
    &.issued {
      .name-text {
        max-width: 112px;
      }
    }
  }
</style>
