<template>
  <div class="info-wrap">
    <div v-for="item in infoList" :key="item.value" class="info-item">
      <div class="label">{{ item.label }}：</div>
      <div class="value">{{ item.value }}</div>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { useI18n } from 'vue-i18n';

  const { t } = useI18n();
  const infoList = [
    {
      label: t('任务ID'),
      value: 'id',
    },
    {
      label: t('任务类型'),
      value: 'type',
    },
    {
      label: t('环境类型'),
      value: 'type',
    },
    {
      label: t('操作范围'),
      value: 'status',
    },
    {
      label: t('执行账号'),
      value: 'createTime',
    },
    {
      label: t('执行时间'),
      value: 'type',
    },
    {
      label: t('开始时间'),
      value: 'status',
    },
    {
      label: t('结束时间'),
      value: 'createTime',
    },
  ];
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
      height: 32px;
      .label {
        margin-right: 8px;
        width: 70px;
        text-align: right;
        color: #4d4f56;
      }
      .value {
        color: #313238;
      }
    }
  }
</style>
