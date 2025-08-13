<template>
  <div class="head">
    <div class="head-left">
      <span class="title">{{ title }}</span>
      <div class="line"></div>
      <ServiceSelector @change="handleAppChange">
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
      <SearchSelector :bk-biz-id="bizId" :app-id="localApp.id" />
      <bk-button theme="primary" style="margin-left: 8px" :disabled="!localApp.name" @click="emits('search')">
        <Search class="search-icon" />
        {{ $t('查询') }}
      </bk-button>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onMounted, watch } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { AngleUpFill, Search } from 'bkui-vue/lib/icon';
  import { CLIENT_HEARTBEAT_LIST } from '../../../../constants/client';
  import { IAppItem } from '../../../../../types/app';
  import useClientStore from '../../../../store/client';
  import SearchSelector from './search-selector.vue';
  import { storeToRefs } from 'pinia';
  import ServiceSelector from '../../../../components/service-selector.vue';

  const clientStore = useClientStore();
  const { searchQuery } = storeToRefs(useClientStore());
  defineProps<{
    title: string;
  }>();

  const emits = defineEmits(['search']);

  const route = useRoute();
  const router = useRouter();

  const localApp = ref({
    name: '',
    id: Number(route.params.appId),
  });
  const heartbeatTime = ref(searchQuery.value.last_heartbeat_time);
  const heartbeatTimeList = ref(CLIENT_HEARTBEAT_LIST);

  const bizId = ref(String(route.params.spaceId));

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
      name: service.spec.name,
      id: service.id!,
    };
    setLastAccessedService(service.id!);
    await router.push({ name: route.name!, params: { spaceId: bizId.value, appId: service.id } });
    heartbeatTime.value = 1;
    handleHeartbeatTimeChange(1);
  };

  const handleHeartbeatTimeChange = (value: number) => {
    clientStore.$patch((state) => {
      state.searchQuery.last_heartbeat_time = value;
    });
    if (!localApp.value.name) return;
    emits('search');
  };

  const setLastAccessedService = (appId: number) => {
    localStorage.setItem('lastAccessedServiceDetail', JSON.stringify({ spaceId: bizId.value, appId }));
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
