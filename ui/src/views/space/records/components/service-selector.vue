<template>
  <bk-select
    v-model="localApp.id"
    ref="selectorRef"
    class="service-selector-record"
    multiple-mode="tag"
    :placeholder="$t('全部')"
    :popover-options="{ theme: 'light bk-select-popover' }"
    :popover-min-width="360"
    :filterable="true"
    :input-search="false"
    :clearable="false"
    :loading="loading"
    :search-placeholder="$t('请输入关键字')"
    @change="handleAppChange">
    <template #prefix>
      <span class="prefix-content">{{ $t('服务') }}</span>
    </template>
    <bk-option v-for="item in serviceList" :key="item.id ? item.id : 'all'" :value="item.id" :label="item.spec.name">
      <div
        v-cursor="{
          active: !item.permissions.view,
        }"
        :class="['service-option-item', { 'no-perm': !item.permissions.view }]">
        <div class="name-text">{{ item.spec.name }}</div>
        <div class="type-tag" :class="{ 'type-tag--en': locale === 'en' }">
          {{ item.spec.config_type === 'file' ? $t('文件型') : $t('键值型') }}
        </div>
      </div>
    </bk-option>
  </bk-select>
</template>

<script lang="ts" setup>
  import { ref, onBeforeMount } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { IAppItem } from '../../../../../types/app';
  import { getAppList } from '../../../../api';
  import { useI18n } from 'vue-i18n';

  const { locale } = useI18n();
  const route = useRoute();
  const router = useRouter();

  const loading = ref(false);
  const localApp = ref<{
    name?: string;
    id?: number;
    serviceType?: string;
  }>({});
  const bizId = ref(String(route.params.spaceId));
  const serviceList = ref<IAppItem[]>([]);

  onBeforeMount(async () => {
    await loadServiceList();
    const service = serviceList.value.find((service) => service.id === Number(route.params.appId));
    if (service) {
      localApp.value = {
        name: service.spec.name,
        id: service.id!,
        serviceType: service.spec.config_type!,
      };
      setLastAccessedServiceDetail(service.id!);
    }
  });

  // 载入服务列表
  const loadServiceList = async () => {
    loading.value = true;
    try {
      const query = {
        start: 0,
        all: true,
      };
      const resp = await getAppList(bizId.value, query);
      serviceList.value = resp.details;
    } catch (e) {
      console.error(e);
    } finally {
      loading.value = false;
    }
  };

  // 下拉列表操作
  const handleAppChange = async (appId: number) => {
    console.log(localApp.value, 'appId');

    const service = serviceList.value.find((service) => service.id === Number(appId));
    // 重新选择服务后不再精确查询
    const query = route.query;
    delete query.id;
    delete query.limit;
    if (service) {
      localApp.value = {
        name: service.spec.name,
        id: service.id!,
        serviceType: service.spec.config_type!,
      };
      setLastAccessedServiceDetail(appId);
      await router.push({ name: 'records-app', params: { spaceId: bizId.value, appId }, query });
    } else {
      localApp.value = {};
      await router.push({ name: 'records-all', params: { spaceId: bizId.value }, query });
    }
  };

  const setLastAccessedServiceDetail = (appId: number) => {
    localStorage.setItem('lastAccessedServiceDetail', JSON.stringify({ spaceId: bizId.value, appId }));
  };
</script>

<style scoped lang="scss">
  .service-selector-record {
    width: 280px;
    .prefix-content {
      margin-right: 10px;
      padding: 0 12px;
      line-height: 32px;
      border-right: 1px solid #c4c6cc;
      background-color: #fafcfe;
    }
  }

  .service-option-item {
    display: flex;
    justify-content: flex-start;
    align-items: center;
    width: 100%;
    .name-text {
      margin-right: 5px;
      flex: 1;
      white-space: nowrap;
      text-overflow: ellipsis;
      overflow: hidden;
    }
    .type-tag {
      flex-shrink: 0;
      width: 52px;
      height: 22px;
      line-height: 22px;
      color: #63656e;
      font-size: 12px;
      text-align: center;
      background: #f0f1f5;
      border-radius: 2px;
      &--en {
        width: 96px;
      }
    }
  }
</style>

<style lang="scss">
  .service-selector-record {
    .bk-select-trigger .bk-select-tag:not(.is-disabled):hover {
      border-color: #c4c6cc;
    }
  }
</style>
