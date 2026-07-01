<template>
  <bk-select
    ref="configSelectRef"
    v-model="configIds"
    selected-style="checkbox"
    :popover-options="{ theme: 'light bk-select-popover config-selector-popover', placement: 'bottom-end' }"
    collapse-tags
    filterable
    multiple
    show-select-all
    @toggle="handleToggleConfigSelectShow"
    @blur="handleCloseConfigSelect">
    <template #trigger>
      <div class="select-btn">{{ type === 'file' ? $t('选择配置文件') : $t('选择配置项') }}</div>
    </template>
    <template v-if="type === 'file'">
      <bk-option-group :label="$t('配置文件')" collapsible>
        <bk-option
          v-for="(item, index) in fileConfigList"
          :id="item.id"
          :key="index"
          :label="joinPathName(item.path, item.name)" />
      </bk-option-group>
      <bk-option-group :label="$t('模板套餐')" collapsible>
        <bk-option
          v-for="(item, index) in templateConfigList"
          :id="`${item.template_space_id} - ${item.template_set_id}`"
          :key="index"
          :label="`${item.template_space_name} - ${item.template_set_name}`" />
      </bk-option-group>
    </template>
    <template v-else>
      <bk-option v-for="(item, index) in kvConfigList" :id="item.key" :key="index" :label="item.key" />
    </template>
    <template #extension>
      <div class="config-select-btns">
        <bk-button theme="primary" @click="handleConfirmSelect">{{ $t('确定') }}</bk-button>
        <bk-button @click="handleCloseConfigSelect">{{ $t('取消') }}</bk-button>
      </div>
    </template>
  </bk-select>
</template>

<script lang="ts" setup>
  import { ref, watch } from 'vue';
  import { cloneDeep } from 'lodash';
  import type { IConfigImportItem, IConfigKvItem } from '../../types/config';
  import type { ImportTemplateConfigItem } from '../../types/template';
  import { joinPathName } from '../utils/config';
  const props = defineProps<{
    type: 'file' | 'kv';
    selectedConfigIds: Array<string | number>;
    kvConfigList?: IConfigKvItem[];
    fileConfigList?: IConfigImportItem[];
    templateConfigList?: ImportTemplateConfigItem[];
  }>();

  const emits = defineEmits(['select']);

  const configIds = ref<(string | number)[]>([]);
  const lastSelectedConfigIds = ref<(string | number)[]>([]);
  const configSelectRef = ref();

  watch(
    () => props.selectedConfigIds,
    () => {
      configIds.value = cloneDeep(props.selectedConfigIds);
    },
    { deep: true, immediate: true },
  );

  const handleToggleConfigSelectShow = (isShow: boolean) => {
    if (isShow) {
      lastSelectedConfigIds.value = cloneDeep(configIds.value);
    }
  };

  const handleCloseConfigSelect = () => {
    configSelectRef.value.hidePopover();
    configIds.value = cloneDeep(lastSelectedConfigIds.value);
  };

  const handleConfirmSelect = () => {
    configSelectRef.value.hidePopover();
    emits('select', configIds.value);
  };
</script>

<style scoped lang="scss">
  .select-btn {
    min-width: 102px;
    height: 32px;
    background: #ffffff;
    border: 1px solid #c4c6cc;
    border-radius: 2px;
    font-size: 14px;
    color: #63656e;
    line-height: 32px;
    text-align: center;
    cursor: pointer;
  }

  .config-select-btns {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 16px;
    justify-content: flex-end;
    width: 100%;
    height: 100%;
    background: #fafbfd;
  }
</style>

<style>
  .config-selector-popover {
    width: 238px !important;
    .bk-select-option {
      padding: 0 12px !important;
    }
  }
</style>
