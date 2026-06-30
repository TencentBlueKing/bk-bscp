<template>
  <section class="record-management-page">
    <div class="operate-area">
      <ServiceSelector
        ref="serviceSelectorRef"
        class="service-selector-record"
        :custom-trigger="false"
        :placeholder="$t('全部')"
        :clearable="true"
        :is-record="true"
        @change="handleAppChange"
        @clear="handleAppChange">
        <template #prefix>
          <span class="prefix-content">{{ $t('服务') }}</span>
        </template>
      </ServiceSelector>
      <date-picker class="date-picker" @change-time="updateParams" />
      <search-option ref="searchOptionRef" @send-search-data="updateParams" />
    </div>
    <record-table :space-id="spaceId" :search-params="searchParams" @handle-table-filter="optionParams = $event" />
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

  const route = useRoute();
  const router = useRouter();

  const spaceId = ref(String(route.params.spaceId));
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
    if (service) {
      localStorage.setItem('lastAccessedServiceDetail', JSON.stringify({ spaceId: spaceId.value, appId: service.id }));
      await router.push({ name: 'records-app', params: { spaceId: spaceId.value, appId: service.id }, query });
    } else {
      await router.push({ name: 'records-all', params: { spaceId: spaceId.value }, query });
    }
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
  .prefix-content {
    padding: 0 12px;
    line-height: 32px;
    border-right: 1px solid #c4c6cc;
    background-color: #fafcfe;
  }
</style>

<style lang="scss">
  .service-selector-record {
    width: 280px;
    .bk-select-trigger .bk-select-tag:not(.is-disabled):hover {
      border-color: #c4c6cc;
    }
  }
</style>
