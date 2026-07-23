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
    <div class="filter-scroll">
      <div class="filter">
      <template v-if="filterType === 'filter'">
        <bk-select
          v-model="filterValues[filter.value as keyof typeof filterValues]"
          v-for="filter in filterList"
          :class="['bk-select', { issued: isIssued }]"
          :key="filter.value"
          :placeholder="filter.label"
          multiple
          @change="triggerSearch">
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
        <bk-button v-if="!isIssued" class="op-btn" text theme="primary" @click="handleSwitchType('expression')">
          <transfer class="icon" />{{ t('表达式') }}
        </bk-button>
      </template>
      <template v-else>
        <bk-input
          :model-value="expressionValues[filter.value]"
          v-for="filter in filterList"
          :key="filter.value"
          :class="['bk-input', { issued: isIssued }]"
          placeholder="*"
          show-overflow-tooltips
          @change="handleInputChange(filter.value, $event)" />
        <bk-popover theme="light" placement="bottom-end" trigger="hover" :width="300">
          <span class="expr-help">
            <HelpDocumentFill class="icon" />
          </span>
          <template #content>
            <div class="expr-tip">
              <div class="expr-tip-title">{{ t('支持 gsekit 表达式语法') }}</div>
              <div v-for="tip in expressionTips" :key="tip" class="expr-tip-item">{{ tip }}</div>
            </div>
          </template>
        </bk-popover>
        <bk-button class="op-btn" text theme="primary" @click="handleSwitchType('filter')">
          <transfer class="icon" />{{ t('筛选') }}
        </bk-button>
      </template>
      <bk-button v-if="isIssued" class="op-btn" text theme="primary" @click="handleClearFilter">
        <Del class="icon" />
        {{ t('清空') }}
      </bk-button>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onMounted, computed } from 'vue';
  import { Transfer, Del, HelpDocumentFill } from 'bkui-vue/lib/icon';
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
    }>(),
    {
      isIssued: false,
    },
  );
  const emits = defineEmits(['search']);


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
  // 表达式模式各字段的输入值，key 与 filterList 的 value 保持一致，缺省匹配任意（*）。
  const expressionValues = ref<Record<string, string>>({
    sets: '',
    modules: '',
    service_instances: '',
    process_aliases: '',
    cc_process_ids: '',
  });
  // 前端字段 → 后端 ExpressionScope 五段字段的映射。
  const EXPRESSION_FIELD_MAP: Record<string, string> = {
    sets: 'set_name',
    modules: 'module_name',
    service_instances: 'service_name',
    process_aliases: 'process_alias',
    cc_process_ids: 'process_id',
  };
  const expressionTips = computed(() => [
    t('通配符：proc* / proc?'),
    t('枚举：[a, b]'),
    t('数字范围：[1-100]'),
    t('字母范围：[a-f]'),
    t('排除：[!ab]'),
    t('前缀组合：4[6, 8, 9]'),
    t('切片（仅 CC 进程ID）：[0:10]、[-5:]'),
    t('留空默认匹配任意（*）'),
  ]);

  // 按当前模式构造并抛出搜索条件：
  // - 筛选模式沿用等值多选（sets/modules/... 数组）；
  // - 表达式模式发送 expression_scope 五段，语义与 gsekit 对齐；全为空时不带该字段（等价不过滤）。
  const triggerSearch = () => {
    if (filterType.value === 'expression') {
      const scope: Record<string, string> = {};
      let hasExpression = false;
      Object.keys(EXPRESSION_FIELD_MAP).forEach((key) => {
        const val = (expressionValues.value[key] || '').trim();
        scope[EXPRESSION_FIELD_MAP[key]] = val || '*';
        if (val && val !== '*') hasExpression = true;
      });
      emits('search', {
        environment: activeEnv.value,
        ...(hasExpression ? { expression_scope: scope } : {}),
      });
      return;
    }
    emits('search', { ...filterValues.value, environment: activeEnv.value });
  };

  const handleSwitchType = (type: 'filter' | 'expression') => {
    filterType.value = type;
    triggerSearch();
  };

  onMounted(() => {
    if (route.query.processIds) {
      const processIds = Array.isArray(route.query.processIds) ? route.query.processIds : [route.query.processIds];
      filterValues.value.cc_process_ids = processIds.map(Number);
      triggerSearch();
    }
    if (filterFlag.value) {
      // 任务详情跳转：操作范围为五段表达式字符串，切到表达式模式按 expression_scope 过滤。
      const { operate_range } = taskDetail.value;
      filterType.value = 'expression';
      expressionValues.value = {
        sets: operate_range.set_name || '',
        modules: operate_range.module_name || '',
        service_instances: operate_range.service_name || '',
        process_aliases: operate_range.process_alias || '',
        cc_process_ids: operate_range.process_id || '',
      };
      taskStore.$patch({ filterFlag: false });
      triggerSearch();
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
    triggerSearch();
  };

  const handleClearFilter = () => {
    filterValues.value = {
      sets: [],
      modules: [],
      service_instances: [],
      process_aliases: [],
      cc_process_ids: [],
    };
    expressionValues.value = {
      sets: '',
      modules: '',
      service_instances: '',
      process_aliases: '',
      cc_process_ids: '',
    };
    triggerSearch();
  };

  const handleInputChange = (key: string, value: string) => {
    expressionValues.value[key] = value;
    triggerSearch();
  };

  defineExpose({
    clear: handleClearFilter,
  });
</script>

<style scoped lang="scss">
  .filter-wrap {
    display: flex;
    flex: 0 0 auto;
    flex-wrap: nowrap;
    align-items: center;
    gap: 8px;
    margin-left: auto;
  }
  .filter-scroll {
    flex: 0 1 auto;
    min-width: 0;
    max-width: 100%;
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
    scrollbar-width: thin;
    scrollbar-color: rgba(0, 0, 0, 0.15) transparent;
    &::-webkit-scrollbar {
      height: 4px;
    }
    &::-webkit-scrollbar-thumb {
      background: rgba(0, 0, 0, 0.15);
      border-radius: 2px;
    }
  }
  .env-tabs {
    display: flex;
    flex-shrink: 0;
    flex-wrap: nowrap;
    align-items: center;
    box-sizing: border-box;
    padding: 4px;
    height: 32px;
    background: #f0f1f5;
    border-radius: 2px;
    color: #4d4f56;
    font-size: 12px;
    .env {
      flex-shrink: 0;
      height: 24px;
      line-height: 24px;
      padding: 0 12px;
      white-space: nowrap;
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
    flex-shrink: 0;
    flex-wrap: nowrap;
    align-items: center;
    gap: 10px;
    .bk-select,
    .bk-input {
      flex-shrink: 0;
      width: 136px;
      &.issued {
        width: 162px;
      }
    }
    .op-btn {
      flex-shrink: 0;
      font-size: 14px;
      white-space: nowrap;
      .icon {
        margin-right: 8px;
      }
    }
    .expr-help {
      display: flex;
      flex-shrink: 0;
      align-items: center;
      color: #979ba5;
      cursor: pointer;
      .icon {
        font-size: 16px;
      }
      &:hover {
        color: #3a84ff;
      }
    }
  }

  .expr-tip {
    font-size: 12px;
    line-height: 20px;
    color: #63656e;
    .expr-tip-title {
      margin-bottom: 4px;
      font-weight: 700;
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
