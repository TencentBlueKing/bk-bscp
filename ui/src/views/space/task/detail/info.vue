<template>
  <div class="info-wrap">
    <div v-for="item in infoList" :key="item.value" class="info-item">
      <div class="label">{{ item.label }}：</div>
      <div class="value" @click="handleGoProcess(item.value)">
        <bk-overflow-title :class="{ theme: item.value === 'operate_range' }" type="tips">
          <span v-if="item.value === 'environment'">{{
            ENV_TYPE_MAP[Number(taskDetail.environment) as keyof typeof ENV_TYPE_MAP]
          }}</span>
          <span v-else-if="item.value === 'operate_range'">
            {{ mergeOpRange(taskDetail.operate_range) }}
          </span>
          <span v-else>{{ taskDetail![item.value as keyof typeof taskDetail] }}</span>
        </bk-overflow-title>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { useI18n } from 'vue-i18n';
  import { ENV_TYPE_MAP } from '../../../../constants/task';
  import { IOperateRange } from '../../../../../types/task';
  import { useRouter } from 'vue-router';
  import useTaskStore from '../../../../store/task';

  defineProps<{
    taskDetail: Record<string, any>;
  }>();

  const { t } = useI18n();
  const router = useRouter();
  const taskStore = useTaskStore();

  const infoList = [
    {
      label: t('任务ID'),
      value: 'id',
    },
    {
      label: t('任务类型'),
      value: 'task_type',
    },
    {
      label: t('环境类型'),
      value: 'environment',
    },
    {
      label: t('操作范围'),
      value: 'operate_range',
    },
    {
      label: t('执行账号'),
      value: 'creator',
    },
    {
      label: t('执行时间'),
      value: 'execution_time',
    },
    {
      label: t('开始时间'),
      value: 'start_at',
    },
    {
      label: t('结束时间'),
      value: 'end_at',
    },
  ];

  const OP_RANGE_ORDER: (keyof IOperateRange)[] = [
    'set_names',
    'module_names',
    'service_names',
    'cc_process_names',
    'cc_process_ids',
  ];

  const mergeOpRange = (operateRange: IOperateRange) => {
    return OP_RANGE_ORDER.map((key) => {
      const arr = operateRange[key];
      return arr.length ? `[${arr.join(',')}]` : '*';
    }).join('.');
  };

  const handleGoProcess = (value: string) => {
    if (value !== 'operate_range') return;
    taskStore.$patch((state) => {
      state.filterFlag = true;
    });
    router.push({
      name: 'process-management',
    });
  };
</script>

<style scoped lang="scss">
  .info-wrap {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    grid-template-rows: repeat(2, auto);
    width: 60%;
    padding: 22px 64px;
    .info-item {
      display: flex;
      align-items: center;
      line-height: 32px;
      font-size: 12px;
      height: 32px;
      .label {
        margin-right: 8px;
        width: 70px;
        text-align: right;
        color: #4d4f56;
      }
      .value {
        min-width: 200px;
        max-width: calc(100vw - 800px);
        color: #313238;
      }
      .theme {
        color: #3a84ff;
        cursor: pointer;
      }
    }
  }
</style>
