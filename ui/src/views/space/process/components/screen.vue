<template>
  <div class="screen-wrap">
    <div class="env-tabs">
      <div
        v-for="env in envList"
        :key="env.value"
        :class="['env', { active: activeEnv === env.value }]"
        @click="activeEnv = env.value">
        {{ env.label }}
      </div>
    </div>
    <div class="screen">
      <bk-select v-for="screen in screenList" :key="screen.value" :placeholder="screen.label" class="bk-select">
      </bk-select>
      <bk-button class="transfer-button" text theme="primary"><transfer class="icon"/>{{ t('表达式') }}</bk-button>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { Transfer } from 'bkui-vue/lib/icon';
  import { useI18n } from 'vue-i18n';
  const { t } = useI18n();
  const envList = [
    {
      label: t('正式'),
      value: 'prod',
    },
    {
      label: t('体验'),
      value: 'stag',
    },
    {
      label: t('测试'),
      value: 'test',
    },
  ];
  const screenList = ref([
    {
      label: t('全部集群 (*)'),
      value: 'set_name',
    },
    {
      label: t('全部模块 (*)'),
      value: 'module_name',
    },
    {
      label: t('全部服务实例 (*)'),
      value: 'service_instance',
    },
    {
      label: t('全部进程别名 (*)'),
      value: 'process_name',
    },
    {
      label: t('全部 CC 进程 ID (*)'),
      value: 'cc_process_id',
    },
  ]);
  const activeEnv = ref('prod');
</script>

<style scoped lang="scss">
.screen-wrap {
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
  .screen {
    display: flex;
    align-items: center;
    gap: 10px;
    .bk-select {
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
