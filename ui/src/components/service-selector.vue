<template>
  <bk-select
    v-model="localVal"
    ref="selectorRef"
    class="service-selector"
    :popover-options="{ theme: 'light bk-select-popover service-selector-popover' }"
    :popover-min-width="320"
    :filterable="true"
    :input-search="false"
    :clearable="false"
    :loading="loading"
    v-bind="$attrs"
    @clear="emits('clear')">
    <template v-if="customTrigger" #trigger>
      <slot name="trigger"></slot>
    </template>
    <template #prefix>
      <slot name="prefix"></slot>
    </template>
    <bk-option-group v-for="group in serviceGroup" :key="group.label" :label="group.label" collapsible>
      <bk-option v-for="item in group.list" :key="item.id" :value="item.id" :label="item.spec.name">
        <div
          v-cursor="{
            active: !item.permissions.view,
          }"
          :class="['service-option-item', { 'no-perm': !item.permissions.view }]"
          @click="handleOptionClick(item, $event)">
          <span class="name-text">{{ item.spec.alias }}</span>
          <span class="name-text">{{ item.spec.name }}</span>
        </div>
      </bk-option>
    </bk-option-group>
    <template #extension>
      <div class="selector-extensition">
        <div class="content" @click="router.push({ name: 'service-all' })">
          <i class="bk-bscp-icon icon-back-line app-icon"></i>
          {{ t('服务列表') }}
        </div>
      </div>
    </template>
  </bk-select>
</template>
<script setup lang="ts">
  import { ref, watch, onBeforeMount, computed } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { storeToRefs } from 'pinia';
  import useGlobalStore from '../store/global';
  import { IAppItem } from '../../types/app';
  import { getAppList } from '../api';
  import { useI18n } from 'vue-i18n';

  const route = useRoute();
  const router = useRouter();
  const { t } = useI18n();

  const { showApplyPermDialog, permissionQuery } = storeToRefs(useGlobalStore());

  const bizId = route.params.spaceId as string;

  const props = withDefaults(
    defineProps<{
      value?: number;
      customTrigger?: boolean;
      isRecord?: boolean;
    }>(),
    {
      customTrigger: true,
      isRecord: false,
    },
  );

  const emits = defineEmits(['change', 'clear']);

  const serviceList = ref<IAppItem[]>([]);
  const loading = ref(false);
  const localVal = ref(props.value);
  const selectorRef = ref();
  const noPermisionIds = ref<number[]>([]);

  const serviceGroup = computed(() => {
    const fileServices = serviceList.value.filter((service: IAppItem) => service.spec.config_type === 'file');
    const kvServices = serviceList.value.filter((service: IAppItem) => service.spec.config_type === 'kv');
    return [
      { label: t('文件型'), list: fileServices ? fileServices : [] },
      { label: t('键值型'), list: kvServices ? kvServices : [] },
    ];
  });

  watch(
    () => props.value,
    (val) => {
      localVal.value = val;
    },
  );

  onBeforeMount(async () => {
    await loadServiceList();
    noPermisionIds.value = serviceList.value
      .filter((service) => !service.permissions.view)
      .map((service) => service.id!);
    let service;
    if (props.value) {
      localVal.value = props.value;
      service = serviceList.value.find((service) => service.id === localVal.value);
    } else {
      if (props.isRecord) {
        // 如果是记录页面，默认选择当前路由参数中的服务
        service = serviceList.value.find((service) => service.id === Number(route.params.appId));
      } else {
        // 如果不是记录页面，默认选择第一个有查看权限的服务
        service = serviceList.value.find((service) => service.permissions.view);
      }
      localVal.value = service ? service.id : undefined;
    }
    emits('change', service);
  });

  const loadServiceList = async () => {
    loading.value = true;
    try {
      const query = {
        start: 0,
        all: true,
      };
      const resp = await getAppList(bizId, query);
      serviceList.value = resp.details;
    } catch (e) {
      console.error(e);
    } finally {
      loading.value = false;
    }
  };

  // 点击无查看权限的选项，弹出申请权限弹窗
  const handleOptionClick = (service: IAppItem, event: Event) => {
    if (!service.permissions.view) {
      selectorRef.value.hidePopover();
      event.stopPropagation();
      permissionQuery.value = {
        resources: [
          {
            biz_id: service.biz_id,
            basic: {
              type: 'app',
              action: 'view',
              resource_id: service.id,
            },
          },
        ],
      };

      showApplyPermDialog.value = true;
    } else {
      emits('change', service);
    }
  };

  defineExpose({
    reloadService: async () => {
      await loadServiceList();
      const service = serviceList.value.find((service) => service.id === localVal.value);
      emits('change', service);
    },
    noPermisionIds,
  });
</script>
<style lang="scss" scoped>
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
  }
  .selector-trigger {
    display: inline-flex;
    align-items: stretch;
    width: 100%;
    height: 32px;
    font-size: 12px;
    border-radius: 2px;
    transition: all 0.3s;
    & > input {
      flex: 1;
      width: 100%;
      padding: 0 24px 0 10px;
      line-height: 1;
      font-size: 14px;
      color: #313238;
      background: #f0f1f5;
      border-radius: 2px;
      border: none;
      outline: none;
      transition: all 0.3s;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      cursor: pointer;
    }
    .arrow-icon {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      position: absolute;
      right: 4px;
      top: 0;
      width: 20px;
      height: 100%;
      transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
      color: #979ba5;
      &.arrow-line {
        font-size: 20px;
      }
    }
  }

  .service-option-item {
    flex: 1;
    height: 100%;
    display: flex;
    flex-direction: column;
    justify-content: center;
    padding: 0 12px 0 28px;
    &.no-perm {
      color: #cccccc !important;
    }
    .name-text {
      white-space: nowrap;
      text-overflow: ellipsis;
      overflow: hidden;
      line-height: normal;
    }
  }
  .selector-extensition {
    flex: 1;
    .content {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 6px;
      height: 40px;
      line-height: 40px;
      text-align: center;
      background: #fafbfd;
      color: #4d4f56;
      cursor: pointer;
      .app-icon {
        color: #979ba5;
      }
      &:hover {
        color: #3a84ff;
        .app-icon {
          color: #3a84ff;
        }
      }
    }
  }
</style>
<style lang="scss">
  .service-selector-popover {
    .bk-select-option {
      padding: 0px !important;
      height: 48px !important;
      &:nth-child(odd) {
        background-color: #fafbfd;
      }
      &:nth-child(even) {
        background-color: #ffffff;
      }
    }
  }
</style>
