<template>
  <section class="configuration-example-page">
    <div class="example-aside">
      <!-- 选择环境 -->
      <EnvSelector v-model="localEnvId" class="sel-env">
        <template #trigger="{ selectInfo, isOpen }">
          <div
            v-if="selectInfo"
            class="env-selector-trigger"
            :style="{
              backgroundColor: ENV_TYPE_CONFIG[selectInfo.group.type]?.bgColor || '#F0F1F5',
              color: ENV_TYPE_CONFIG[selectInfo.group.type]?.textColor || '#63656E' }">
            <div class="env-val-cls">
              <i
                :class="`bk-bscp-icon ${ENV_TYPE_CONFIG[selectInfo.group.type]?.iconClass || ''} env-icon`"
                :style="{ color: ENV_TYPE_CONFIG[selectInfo.group.type]?.iconColor || '#979BA5' }">
              </i>
              <span class="env-name">{{ selectInfo.env?.spec.name }}</span>
            </div>
            <AngleUpFill
              :class="['env-arrow', { 'icon-rotate': isOpen }]"
              :style="{ color: ENV_TYPE_CONFIG[selectInfo.group.type]?.iconColor || '#979BA5' }" />
          </div>
        </template>
      </EnvSelector>
      <!-- 选择服务 -->
      <ServiceSelector class="sel-service" :env-id="localEnvId" @change="selectService">
        <template #trigger>
          <div class="selector-trigger">
            <bk-overflow-title v-if="serviceName" class="app-name" type="tips">
              {{ serviceName }}
            </bk-overflow-title>
            <span v-else class="no-app">{{ $t('暂无服务') }}</span>
            <AngleUpFill class="arrow-icon arrow-fill" />
          </div>
        </template>
      </ServiceSelector>
      <!-- 示例列表 -->
      <div class="type-wrap" v-show="serviceName && serviceType">
        <bk-menu :active-key="renderComponent" @update:active-key="changeTypeItem">
          <bk-menu-item v-for="item in navList" :key="item.val" :need-icon="false">
            {{ item.name }}
          </bk-menu-item>
        </bk-menu>
      </div>
    </div>
    <!-- 右侧区域 -->
    <div class="example-main" ref="exampleMainRef">
      <bk-alert v-show="(serviceType === 'file' || renderComponent === 'shell') && topTip" theme="info">
        <div class="alert-tips">
          <p>{{ topTip }}</p>
        </div>
      </bk-alert>
      <div class="content-wrap">
        <bk-loading style="height: 100%" :loading="loading">
          <component
            :is="currentTemplate"
            :template-name="renderComponent"
            :content-scroll-top="contentScrollTop"
            :selected-key-data="selectedClientKey"
            :key="renderComponent"
            @selected-key-data="selectedClientKey = $event" />
        </bk-loading>
      </div>
    </div>
  </section>
</template>

<script lang="ts" setup>
  import { computed, ref, nextTick, provide } from 'vue';
  import ServiceSelector from '../../../../components/service-selector.vue';
  import EnvSelector from '../../../../components/env-selector.vue';
  import { useI18n } from 'vue-i18n';
  import useGlobalStore from '../../../../store/global';
  import { storeToRefs } from 'pinia';
  import ContainerExample from './components/content/container-example.vue';
  import NodeManaExample from './components/content/node-mana-example.vue';
  import CmdExample from './components/content/cmd-example.vue';
  import DefaultExample from './components/content/default-example.vue';
  import Exception from '../components/exception.vue';
  import { IAppItem } from '../../../../../types/app';
  import { useRoute, useRouter } from 'vue-router';
  import { AngleUpFill } from 'bkui-vue/lib/icon';
  import { ENV_TYPE_CONFIG } from '../../../../constants/env';

  const { t } = useI18n();
  const route = useRoute();
  const router = useRouter();

  const globalStore = useGlobalStore();
  const { spaceFeatureFlags } = storeToRefs(globalStore);

  interface INavItem {
    name: string;
    val: string;
    hidden?: boolean;
  }

  const fileTypeArr: INavItem[] = [
    { name: t('Sidecar容器'), val: 'sidecar' },
    { name: t('节点管理插件'), val: 'node' },
    { name: t('HTTP(S)接口调用'), val: 'http' }, // 文件型也有http(s)接口，页面结构和键值型一样，但脚本内容、部分文案不一样
    { name: t('命令行工具'), val: 'shell' },
    { name: 'Go SDK', val: 'go' },
  ];
  const kvTypeArr: INavItem[] = [
    { name: 'Python SDK', val: 'python' },
    { name: 'Go SDK', val: 'go' },
    { name: 'Java SDK', val: 'java' },
    { name: 'C++ SDK', val: 'cpp' },
    { name: 'tRPC-Go Plugin', val: 'trpc' },
    { name: t('HTTP(S)接口调用'), val: 'http' },
    { name: t('命令行工具'), val: 'shell' },
  ];

  const bizId = ref(String(route.params.spaceId));
  const selectedClientKey = ref(); // 记忆选择的客户端密钥信息,用于切换不同示例时默认选中密钥
  const exampleMainRef = ref();
  const renderComponent = ref(''); // 渲染的示例组件
  const serviceName = ref('');
  const serviceType = ref('');
  const projectId = ref(String(route.params.projectId));
  const localEnvId = ref(String(route.params.envId));
  const topTip = ref('');
  const loading = ref(true);
  provide('basicInfo', { serviceName, serviceType });

  const navList = computed(() => {
    if (serviceType.value === 'kv') {
      // 如果 TRPC_GO_PLUGIN 未启用，移除 trpc 示例
      if (!spaceFeatureFlags.value.TRPC_GO_PLUGIN.enable) {
        return kvTypeArr.filter((type: INavItem) => type.val !== 'trpc');
      }
      return kvTypeArr;
    }
    return fileTypeArr;
  });
  // 展示的示例组件与顶部提示语
  const currentTemplate = computed(() => {
    if (serviceType.value && !loading.value) {
      switch (renderComponent.value) {
        case 'sidecar':
          topTip.value = t('Sidecar 容器客户端用于容器化应用程序拉取文件型配置场景。');
          return ContainerExample;
        case 'node':
          topTip.value = t('节点管理插件客户端用于非容器化应用程序 (传统主机) 拉取文件型配置场景。');
          return NodeManaExample;
        case 'shell':
          topTip.value = t(
            '命令行工具通常用于在脚本 (如 Bash、Python 等) 中手动拉取应用程序配置，同时支持文件型和键值型配置的获取。',
          );
          return CmdExample;
        default:
          // 默认模板
          return DefaultExample;
      }
    }
    // 无数据模板
    return Exception;
  });

  // 服务切换
  const selectService = async (service: IAppItem) => {
    // 重置已选择的密钥信息
    selectedClientKey.value = null;
    if (service) {
      const routeParams = {
        spaceId: bizId.value,
        projectId: projectId.value,
        envId: localEnvId.value,
        appId: service.id };
      await router.push({ name: route.name!, params: routeParams });
      localStorage.setItem('lastAccessedServiceDetail', JSON.stringify(routeParams));
    } else {
      loading.value = false;
    }
    if (serviceName.value !== service.spec.name || serviceType.value !== service.spec.config_type) {
      loading.value = true;
      serviceName.value = service.spec.name;
      serviceType.value = service.spec.config_type;
    }
    changeTypeItem(navList.value[0].val);
  };

  // 服务的子类型切换
  const changeTypeItem = (data: string) => {
    renderComponent.value = data;
    nextTick(() => {
      loading.value = false;
    });
  };
  // 返回顶部
  const contentScrollTop = () => {
    if (exampleMainRef.value.scrollTop > 64) {
      exampleMainRef.value.scrollTo({ top: 0, behavior: 'smooth' });
    }
  };
</script>

<style scoped lang="scss">
  .configuration-example-page {
    display: flex;
    justify-content: flex-start;
    align-items: flex-start;
    width: 100%;
    height: 100%;
    background: #f5f7fa;
  }
  .example-aside {
    display: flex;
    flex-direction: column;
    justify-content: flex-start;
    align-items: center;
    flex-shrink: 0;
    width: 240px;
    height: 100%;
    border-right: 1px solid #dcdee5;
    background-color: #fff;
  }
  .example-main {
    flex: 1;
    height: 100%;
    overflow-y: auto;
    :deep(.bk-alert-wraper) {
      align-items: center;
    }
  }
  .alert-tips {
    display: flex;
    > p {
      margin: 0;
      line-height: 20px;
    }
  }
  .sel-env {
    flex-shrink: 0;
    padding: 10px 8px 0;
    width: 239px;
    :deep(.env-selector-trigger) {
      display: flex;
      align-items: center;
      justify-content: space-between;
      height: 32px;
      cursor: pointer;
      padding: 0 8px;
      border-radius: 4px;
      transition: all 0.3s;
      .env-val-cls {
        flex: 1;
        display: flex;
        align-items: center;
        gap: 4px;
      }
      .env-icon {
        font-size: 16px;
        flex-shrink: 0;
      }
      .env-name {
        flex: 1;
        font-size: 16px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .env-arrow {
        margin-left: 8px;
        font-size: 16px;
        color: #F8B4B4;
        transition: transform 0.2s;
        flex-shrink: 0;
        &.icon-rotate {
          transform: rotate(-180deg);
        }
      }
    }
  }
  .sel-service {
    flex-shrink: 0;
    padding: 4px 8px 10px;
    width: 239px;
    border-bottom: 1px solid #f0f1f5;
  }
  .type-wrap {
    margin-top: 12px;
    width: 100%;
    flex: 1;
    overflow-y: auto;
  }
  .bk-menu {
    width: 239px;
    background: #fff;
    .bk-menu-item {
      padding: 0 22px;
      margin: 0;
      color: #63656e;
      &.is-active {
        color: #3a84ff;
        background: #e1ecff;
        &:hover {
          color: #3a84ff;
        }
      }
      &:hover {
        color: #63656e;
      }
    }
  }
  .content-wrap {
    margin: 24px;
    padding: 24px;
    box-shadow: 0 2px 4px 0 #1919290d;
    overflow: auto;
    background-color: #fff;
    flex: 1;
    min-height: 1px;
  }

  .service-selector {
    &.popover-show {
      .selector-trigger .arrow-icon {
        transform: rotate(-180deg);
      }
    }
    &.is-focus {
      .selector-trigger {
        outline: 0;
      }
    }
    .selector-trigger {
      padding: 0 10px 0;
      height: 32px;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: space-between;
      border-radius: 2px;
      transition: all 0.3s;
      background: #f0f1f5;
      font-size: 14px;
      .app-name {
        max-width: 220px;
        color: #313238;
      }
      .no-app {
        font-size: 16px;
        color: #c4c6cc;
      }
      .arrow-icon {
        margin-left: 13.5px;
        color: #979ba5;
        transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
      }
    }
  }
</style>
