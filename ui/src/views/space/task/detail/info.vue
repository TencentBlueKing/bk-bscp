<template>
  <div class="info-wrap">
    <div v-for="item in infoList" :key="item.value" class="info-item">
      <div class="label">{{ item.label }}：</div>
      <div class="value">
        <bk-overflow-title :class="{ theme: item.value === 'operate_range' }" type="tips">
          {{ taskDetail![item.value as keyof typeof taskDetail] }}
        </bk-overflow-title>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { onBeforeMount } from 'vue';
  import { useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';

  const props = defineProps<{
    taskDetail: Record<string, any>;
  }>();

  const { t } = useI18n();
  const router = useRouter();

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

  onBeforeMount(() => {
    if (!props.taskDetail.id) {
      router.push({ name: 'task-list' });
    }
  });
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
        width: 200px;
        color: #313238;
      }
      .theme {
        color: #3a84ff;
      }
    }
  }
</style>
