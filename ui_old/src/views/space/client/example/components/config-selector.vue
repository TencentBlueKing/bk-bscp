<template>
  <bk-select
    v-model="configName"
    ref="selectorRef"
    :class="['config-selector', { 'select-error': isError }]"
    :popover-options="{ theme: 'light bk-select-popover' }"
    :popover-min-width="360"
    :filterable="true"
    :input-search="false"
    :clearable="false"
    :loading="loading"
    :multiple="multipleExample"
    :search-placeholder="basicInfo?.serviceType.value === 'file' ? $t('配置文件名') : $t('配置项名称')"
    :no-data-text="$t('暂无可用配置')"
    :no-match-text="$t('搜索结果为空')"
    @change="handleConfigChange">
    <template #trigger>
      <div class="selector-trigger">
        <bk-overflow-title v-if="configName" class="config-name" type="tips">
          {{ originalConfigName }}
        </bk-overflow-title>
        <span v-else class="empty">{{ $t('请选择') }}</span>
        <AngleUpFill class="arrow-icon arrow-fill" />
      </div>
    </template>
    <!-- 非模板配置和套餐合并后的数据（configList），配置项名称（文件名）可能一样，这里添加/index做唯一区分，后续使用删除/index -->
    <bk-option v-if="multipleExample && configList.length > 0" value="*" :label="$t('全部配置项')"></bk-option>
    <bk-option v-for="item in configList" :key="item.sign" :value="item.name" :label="item.name">
      <div class="config-option-item">
        <div class="name-text">{{ item.name }}</div>
      </div>
    </bk-option>
  </bk-select>
</template>

<script lang="ts" setup>
  import { ref, onMounted, inject, Ref, computed } from 'vue';
  import { useRoute } from 'vue-router';
  import { getAllReleasedConfigList } from '../../../../../api/config';
  import { AngleUpFill } from 'bkui-vue/lib/icon';
  import { useI18n } from 'vue-i18n';

  const props = defineProps<{
    templateName: string;
    serviceType: string;
  }>();

  const { t } = useI18n();
  const emits = defineEmits(['select-config']);

  const route = useRoute();

  const basicInfo = inject<{ serviceName: Ref<string>; serviceType: Ref<string> }>('basicInfo');

  const loading = ref(false);
  const isError = ref(false);
  const configName = ref<string | string[]>();
  const bizId = ref(String(route.params.spaceId));
  const appId = ref(route.params.appId);
  const configList = ref<{ name: string; sign: string }[]>([]);

  // 选中的配置项（文件名）原始名称
  const originalConfigName = computed(() => {
    if (Array.isArray(configName.value)) {
      if (configName.value.includes('*')) {
        return t('全部配置项');
      }
      return configName.value.join(',');
    }
    return configName.value;
  });

  // 可多选的示例类型
  const multipleExample = computed(
    () => props.templateName === 'python' || (props.templateName === 'go' && props.serviceType === 'kv'),
  );

  onMounted(async () => {
    await loadConfigList();
  });

  // 载入服务列表
  const loadConfigList = async () => {
    loading.value = true;
    try {
      const res = await getAllReleasedConfigList(bizId.value, Number(appId.value));
      //  过滤掉重复的配置项
      const map = new Map();
      res.data.items.forEach((item: { name: string; sign: string }) => map.set(item.name, item));
      Array.from(map.values());
      configList.value = Array.from(map.values());
      if (multipleExample.value && configList.value.length > 0) {
        configName.value = ['*'];
        handleConfigChange(configName.value);
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading.value = false;
    }
  };

  // 下拉列表操作
  const handleConfigChange = async (val: string | string[]) => {
    configName.value = Array.isArray(val) ? val.filter((item) => item) : val;
    if (Array.isArray(configName.value)) {
      if (val.length === 0) {
        configName.value = '';
      }
      if (configName.value[configName.value.length - 1] === '*') {
        configName.value = ['*'];
      } else if (configName.value.length > 1 && configName.value[0] === '*') {
        configName.value = configName.value.slice(1);
      }
    }
    emits('select-config', configName.value);
    validateConfig();
  };

  // 表单校验失败检查配置项是否为空
  const validateConfig = () => {
    isError.value = !configName.value;
    return !isError.value;
  };

  defineExpose({
    validateConfig,
  });
</script>

<style scoped lang="scss">
  .config-selector {
    &.select-error .selector-trigger {
      border-color: #ea3636;
    }
    &.popover-show .selector-trigger {
      border-color: #3a84ff;
      box-shadow: 0 0 3px #a3c5fd;
      .arrow-icon {
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
      background: #ffffff;
      font-size: 12px;
      border: 1px solid #c4c6cc;
      .config-name {
        max-width: 480px;
        color: #63656e;
      }
      .empty {
        font-size: 12px;
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
