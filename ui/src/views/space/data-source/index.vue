<template>
  <div class="data-source-page">
    <div class="data-source-header">
      <span class="title">{{ t('表格数据源管理') }}</span>
      <div class="tabs">
        <div :class="['line', { manage: activeTab === 'manage' }]"></div>
        <div
          v-for="tab in tabs"
          :key="tab.name"
          :class="['tab-item', { active: activeTab === tab.name }]"
          @click="handleRouterLink(tab.routeName)">
          {{ tab.label }}
        </div>
      </div>
    </div>
    <div class="data-source-content">
      <router-view></router-view>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRoute, useRouter } from 'vue-router';

  const router = useRouter();
  const route = useRoute();
  const { t } = useI18n();

  const tabs = [
    { name: 'table', label: t('托管表格'), routeName: 'trusteeship-table-list' },
    { name: 'manage', label: t('外部数据源'), routeName: 'data-source-manage' },
  ];

  watch(
    () => route.name,
    () => {
      activeTab.value = getDefaultTab();
    },
  );

  const getDefaultTab = () => {
    const tab = tabs.find((item) => item.routeName === route.name);
    return tab ? tab.name : 'table';
  };
  const activeTab = ref(getDefaultTab());

  const handleRouterLink = (routeName: string) => {
    router.push({ name: routeName });
  };
</script>

<style scoped lang="scss">
  .data-source-page {
    height: 100%;
    background: #f5f7fa;
    overflow: auto;
  }
  .data-source-header {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    height: 52px;
    background: #ffffff;
    box-shadow: 0 3px 4px 0 #0000000a;
    .title {
      position: absolute;
      left: 24px;
      font-size: 16px;
      color: #313238;
      letter-spacing: 0;
    }
    .tabs {
      position: relative;
      display: flex;
      height: 100%;
      .tab-item {
        width: 118px;
        height: 100%;
        text-align: center;
        line-height: 52px;
        font-size: 14px;
        color: #63656e;
        cursor: pointer;
        &.active {
          color: #3a84ff;
          background: #f0f5ff;
        }
      }
      .line {
        position: absolute;
        width: 118px;
        height: 3px;
        background: #3a84ff;
        left: 0;
        transition: all ease-in 0.3s;
        &.manage {
          left: 118px;
        }
      }
    }
  }
  .data-source-content {
    padding: 24px;
  }
</style>
