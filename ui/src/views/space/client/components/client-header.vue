<template>
  <div class="head">
    <div class="head-left">
      <span class="title">{{ title }}</span>
      <div class="line"></div>
      <EnvSelector v-model="localEnvId" @change="handleEnvChange">
        <template #trigger="{ selectInfo, isOpen }">
          <div
            class="env-selector-trigger"
            :style="{
              backgroundColor: getEnvBgColor(selectInfo?.group?.type),
              color: getEnvTextColor(selectInfo?.group?.type) }">
            <div v-if="selectInfo" class="env-val-cls">
              <i
                :class="`bk-bscp-icon ${getEnvIconClass(selectInfo.group.type)} env-icon`"
                :style="{ color: getEnvIconColor(selectInfo.group.type) }">
              </i>
              <span class="env-name">{{ selectInfo.env?.spec?.name || '' }}</span>
            </div>
            <div v-else class="no-env">{{ $t('暂无环境') }}</div>
            <AngleUpFill
              :class="['env-arrow', { 'icon-rotate': isOpen }]"
              :style="{ color: getEnvIconColor(selectInfo?.group?.type) }" />
          </div>
        </template>
      </EnvSelector>
      <ServiceSelector :value="appId" :env-id="localEnvId" @change="handleAppChange">
        <template #trigger>
          <div class="selector-trigger">
            <bk-overflow-title v-if="localApp.name" class="app-name" type="tips">
              {{ localApp?.name }}
            </bk-overflow-title>
            <span v-else class="no-app">{{ $t('暂无服务') }}</span>
            <AngleUpFill class="arrow-icon arrow-fill" />
          </div>
        </template>
      </ServiceSelector>
    </div>
    <div class="head-right">
      <div class="selector-tips">{{ $t('最后心跳时间') }}</div>
      <bk-select
        v-model="heartbeatTime"
        class="heartbeat-selector"
        :clearable="false"
        :filterable="false"
        @change="handleHeartbeatTimeChange">
        <bk-option v-for="item in heartbeatTimeList" :id="item.value" :key="item.value" :name="item.label" />
      </bk-select>
      <SearchSelector :bk-biz-id="bizId" :project-id="projectId" :env-id="localEnvId" :app-id="localApp.id" />
      <bk-button theme="primary" style="margin-left: 8px" :disabled="!localApp.name" @click="emits('search')">
        <Search class="search-icon" />
        {{ $t('查询') }}
      </bk-button>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onMounted, watch, computed } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { AngleUpFill, Search } from 'bkui-vue/lib/icon';
  import { CLIENT_HEARTBEAT_LIST } from '../../../../constants/client';
  import { IAppItem } from '../../../../../types/app';
  import { IEnvItem, EnvType } from '../../../../../types/env';
  import { ENV_TYPE_CONFIG } from '../../../../constants/env';
  import useClientStore from '../../../../store/client';
  import SearchSelector from './search-selector.vue';
  import { storeToRefs } from 'pinia';
  import ServiceSelector from '../../../../components/service-selector.vue';
  import EnvSelector from '../../../../components/env-selector.vue';

  const clientStore = useClientStore();
  const { searchQuery } = storeToRefs(useClientStore());
  defineProps<{
    title: string;
    appId: number;
  }>();

  const emits = defineEmits(['search']);

  const route = useRoute();
  const router = useRouter();

  const localApp = ref<{ name: string; id: number | undefined}>({
    name: '',
    id: Number(route.params.appId),
  });
  const heartbeatTime = ref(searchQuery.value.last_heartbeat_time);
  const heartbeatTimeList = ref(CLIENT_HEARTBEAT_LIST);

  const bizId = ref(String(route.params.spaceId));
  const projectId = ref(String(route.params.projectId));
  const localEnvId = ref(String(route.params.envId || ''));

  const routeParams = computed(() => ({
    spaceId: bizId.value,
    projectId: projectId.value,
    envId: localEnvId.value,
    appId: localApp.value.id,
  }));

  watch(
    () => heartbeatTime.value,
    (val) => {
      router.replace({
        query: {
          ...route.query,
          heartTime: val,
        },
      });
    },
  );

  onMounted(async () => {
    if (Object.keys(route.query).find((key) => key === 'heartTime')) {
      heartbeatTime.value = Number(route.query.heartTime) || searchQuery.value.last_heartbeat_time;
      handleHeartbeatTimeChange(heartbeatTime.value);
    }
  });

  const handleAppChange = async (service: IAppItem) => {
    localApp.value = {
      name: service?.spec?.name,
      id: service?.id,
    };
    setLastAccessedService();
    await router.push({ name: route.name!, params: routeParams.value });
    heartbeatTime.value = 1;
    handleHeartbeatTimeChange(1);
  };

  const handleEnvChange = (env: IEnvItem) => {
    // TODO: 处理环境变更逻辑
    console.log('环境变更:', env);
  };

  // 获取环境类型图标 class
  const getEnvIconClass = (type: EnvType | undefined) => {
    if (!type) return;
    return ENV_TYPE_CONFIG[type]?.iconClass || '';
  };
  // 获取环境图标颜色
  const getEnvIconColor = (type: EnvType | undefined) => {
    if (!type) return;
    return ENV_TYPE_CONFIG[type]?.iconColor || '#979ba5';
  };
  // 获取环境文字颜色
  const getEnvTextColor = (type: EnvType | undefined) => {
    if (!type) return;
    return ENV_TYPE_CONFIG[type]?.textColor || '#63656E';
  };
  const getEnvBgColor = (type: EnvType | undefined) => {
    if (!type) return;
    return ENV_TYPE_CONFIG[type]?.bgColor || 'F0F1F5';
  };

  const handleHeartbeatTimeChange = (value: number) => {
    clientStore.$patch((state) => {
      state.searchQuery.last_heartbeat_time = value;
    });
    if (!localApp.value.name) return;
    emits('search');
  };

  const setLastAccessedService = () => {
    localStorage.setItem('lastAccessedServiceDetail', JSON.stringify(routeParams.value));
  };
</script>

<style scoped lang="scss">
  .head {
    display: flex;
    font-size: 20px;
    line-height: 28px;
    min-height: 32px;
    .head-left {
      height: 32px;
      display: flex;
      align-items: center;
      .line {
        width: 1px;
        height: 24px;
        background-color: #dcdee5;
        margin: 0 16px;
      }
      .title {
        position: relative;
        color: #313238;
      }
      .selector-trigger {
        width: 260px;
        height: 32px;
        cursor: pointer;
        display: flex;
        align-items: center;
        border-radius: 2px;
        transition: all 0.3s;
        font-size: 20px;
        .app-name {
          max-width: 220px;
          color: #63656e;
        }
        .no-app {
          font-size: 16px;
          color: #c4c6cc;
        }
        .arrow-icon {
          font-size: 16px;
          margin-left: 13.5px;
          color: #979ba5;
          transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
        }
      }
      .env-selector-trigger {
        display: flex;
        align-items: center;
        justify-content: space-between;
        width: 140px;
        height: 36px;
        cursor: pointer;
        padding: 0 8px;
        border-radius: 4px;
        transition: all 0.3s;
        margin-right: 16px;
        & > div {
          height: 100%;
          line-height: 36px;
        }
        .env-val-cls {
          flex: 1;
          display: flex;
          align-items: center;
          gap: 4px;
        }
        .env-type-icon {
          font-size: 20px;
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
          &.icon-rotate {
            transform: rotate(-180deg);
          }
        }
      }
      .no-env {
        font-size: 16px;
        color: #c4c6cc;
      }
    }
    .head-right {
      margin-left: calc(27% - 393px);
      display: flex;
      font-size: 12px;
      .selector-tips {
        min-width: 88px;
        height: 32px;
        background: #fafbfd;
        border: 1px solid #c4c6cc;
        border-radius: 2px 0 0 2px;
        line-height: 32px;
        text-align: center;
        border-right: none;
        color: #63656e;
      }
      .heartbeat-selector {
        width: 112px;
        margin-right: 8px;
        :deep(.bk-input--default) {
          border-radius: 0 2px 2px 0;
        }
      }
      .search-icon {
        margin-right: 8px;
      }
    }
  }
</style>
