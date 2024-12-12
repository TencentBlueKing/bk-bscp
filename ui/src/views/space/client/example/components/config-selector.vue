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
    :multiple="props.templateName === 'python'"
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
    <bk-option
      v-if="props.templateName === 'python' && configList.length > 0"
      value="*"
      :label="$t('全部配置项')"></bk-option>
    <bk-option
      v-for="(item, index) in configList"
      :key="item.id + index"
      :value="`${item.config}/${index}`"
      :label="item.config">
      <div class="config-option-item">
        <div class="name-text">{{ item.config }}</div>
      </div>
    </bk-option>
  </bk-select>
</template>

<script lang="ts" setup>
  import { ref, onMounted, inject, Ref, computed } from 'vue';
  import { useRoute } from 'vue-router';
  import { getConfigList, getBoundTemplates, getKvList } from '../../../../../api/config';
  import { AngleUpFill } from 'bkui-vue/lib/icon';
  import { IConfigItem, IBoundTemplateGroup, IConfigKvType } from '../../../../../../types/config';
  import { useI18n } from 'vue-i18n';

  const props = defineProps<{
    templateName: string;
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
  const configList = ref<{ id: number; config: string }[]>([]);

  // 选中的配置项（文件名）原始名称
  const originalConfigName = computed(() => {
    if (Array.isArray(configName.value)) {
      if (configName.value.includes('*')) {
        return t('全部配置项');
      }
      return configName.value.map((item) => item.split('/')[0]).join(',');
    }
    return String(configName.value).replace(/\/[^\\/]*$/, '');
  });

  onMounted(async () => {
    await loadConfigList();
  });

  // 载入服务列表
  const loadConfigList = async () => {
    loading.value = true;
    try {
      const query = {
        start: 0,
        all: true,
      };
      if (basicInfo?.serviceType.value === 'file') {
        // 文件型配置项
        const [configRes, boundTempRes] = await Promise.all([
          getConfigList(bizId.value, Number(appId.value), query), // 非模板配置
          getBoundTemplates(bizId.value, Number(appId.value), query), // 套餐
        ]);
        // 提取非模板配置的信息
        const configResult = configRes.details.map((item: IConfigItem) => {
          const { path, name } = item.spec;
          return {
            id: item.id,
            config: path.endsWith('/') ? `${path}${name}` : `${path}/${name}`,
          };
        });
        // 提取套餐配置信息
        const boundTempResult = boundTempRes.details.map((group: IBoundTemplateGroup) => {
          // 遍历套餐
          return group.template_revisions.map((item) => {
            const { template_id, path, name } = item;
            return {
              id: template_id,
              config: path.endsWith('/') ? `${path}${name}` : `${path}/${name}`,
            };
          });
        });
        // 合并配置信息
        configList.value = [...configResult, ...boundTempResult].flat();
      } else {
        // 键值型配置项
        const res = await getKvList(bizId.value, Number(appId.value), query);
        configList.value = res.details.map((item: IConfigKvType) => {
          return {
            id: item.id,
            config: item.spec.key,
          };
        });
        if (props.templateName === 'python' && configList.value.length > 0) {
          configName.value = ['*'];
          handleConfigChange(configName.value);
        }
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading.value = false;
    }
  };

  // 下拉列表操作
  const handleConfigChange = async (val: string | string[]) => {
    configName.value = val;
    let selectConfig;
    if (Array.isArray(configName.value)) {
      if (val.length === 0) {
        configName.value = '';
      }
      if (configName.value[configName.value.length - 1] === '*') {
        configName.value = ['*'];
        selectConfig = '*';
      } else if (configName.value.length > 1 && configName.value[0] === '*') {
        configName.value = configName.value.slice(1);
        selectConfig = originalConfigName.value;
      } else {
        selectConfig = originalConfigName.value;
      }
    } else {
      selectConfig = originalConfigName.value;
    }
    emits('select-config', selectConfig);
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
