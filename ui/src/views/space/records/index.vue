<template>
  <section class="record-management-page">
    <div class="operate-area">
      <env-selector
        class="env-selector"
        v-model="envId"
        :placeholder="$t('请选择环境')"
        :use-default-trigger="true"
        @change="handleEnvChange" />
      <ServiceSelector
        ref="serviceSelectorRef"
        class="service-selector-record"
        :custom-trigger="false"
        :placeholder="$t('全部')"
        :clearable="true"
        :is-record="true"
        :project-id="projectId"
        :env-id="envId"
        @change="handleAppChange"
        @clear="handleAppChange" />
      <date-picker class="date-picker" @change-time="updateParams" />
      <search-option ref="searchOptionRef" @send-search-data="updateParams" />
    </div>
    <record-table
      :space-id="spaceId"
      :project-id="projectId"
      :env-id="envId"
      :search-params="searchParams"
      @handle-table-filter="optionParams = $event" />
  </section>
</template>
<script setup lang="ts">
  import { ref } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import ServiceSelector from '../../../components/service-selector.vue';
  import datePicker from './components/date-picker.vue';
  import searchOption from './components/search-option.vue';
  import recordTable from './components/record-table.vue';
  import { IRecordQuery } from '../../../../types/record';
  import { IAppItem } from '../../../../types/app';
  import EnvSelector from '../../../components/env-selector.vue';

  const route = useRoute();
  const router = useRouter();

  const spaceId = ref(String(route.params.spaceId));
  const projectId = ref(String(route.params.projectId));
  const envId = ref(String(route.params.envId || ''));
  const searchParams = ref<IRecordQuery>({}); // 外部搜索数据参数汇总
  const dateTimeParams = ref<{ start_time?: string; end_time?: string }>({}); // 日期组件参数
  const optionParams = ref<IRecordQuery>(); // 搜索组件参数
  const init = ref(true);
  const serviceSelectorRef = ref();

  const updateParams = (data: string[] | IRecordQuery) => {
    if (Array.isArray(data)) {
      dateTimeParams.value.start_time = data[0];
      dateTimeParams.value.end_time = data[1];
    } else {
      optionParams.value = data;
    }
    if (!init.value) {
      mergeData();
    }
  };

  const mergeData = () => {
    const params = {
      ...optionParams.value,
      ...dateTimeParams.value,
      app_id: Number(route.params.appId),
      all: Number(route.params.appId) <= -1,
    };
    // 操作记录id
    const id = Number(route.query.id);
    if (id > 0) {
      params.id = id;
    }
    searchParams.value = {
      ...params,
    };
  };

  const handleAppChange = async (service: IAppItem) => {
    if (init.value) {
      mergeData();
      init.value = false;
    }
    // 重新选择服务后不再精确查询
    const query = route.query;
    delete query.id;
    delete query.limit;
    const routeParams = {
      spaceId: spaceId.value,
      projectId: projectId.value,
      envId: envId.value,
    };
    if (service) {
      localStorage.setItem('lastAccessedServiceDetail', JSON.stringify({ ...routeParams, appId: service.id }));
      await router.push({ name: 'records-app', params: { ...routeParams, appId: service.id }, query });
    } else {
      await router.push({ name: 'records-all', params: routeParams, query });
    }
  };

  const handleEnvChange = async () => {
    if (route.params.appId) return;
    const routeParams = {
      spaceId: spaceId.value,
      projectId: projectId.value,
      envId: envId.value,
    };
    await router.push({ name: route.name, params: routeParams, query: route.query });
  };
</script>
<style lang="scss" scoped>
  .record-management-page {
    height: calc(100% - 33px);
    padding: 24px;
    background: #f5f7fa;
    overflow: hidden;
    .date-picker {
      margin-left: 8px;
    }
  }
  .operate-area {
    display: flex;
    align-items: center;
    justify-content: flex-start;
    margin-bottom: 16px;
  }
  .env-selector {
    width: 120px;
  }
</style>

<style lang="scss">
  .service-selector-record {
    width: 280px;
    margin-left: 8px;
    .bk-select-trigger .bk-select-tag:not(.is-disabled):hover {
      border-color: #c4c6cc;
    }
  }
</style>
