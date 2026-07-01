<template>
  <div>
    <bk-form ref="formRef" class="import-config-form" :model="formData" :rules="rules" :label-width="70">
      <div class="select-wrap">
        <bk-form-item :label="$t('选择服务')">
          <bk-select :model-value="formData.service" class="service-select" disabled>
            <bk-option :id="service.id" :name="service.spec.name" />
          </bk-select>
        </bk-form-item>
        <bk-form-item :label="$t('选择版本')" property="version">
          <bk-select
            v-model="formData.version"
            class="version-select"
            :loading="versionListLoading"
            filterable
            auto-focus
            :clearable="false"
            @select="handleSelectVersion">
            <bk-option v-for="item in versionList" :id="item.id" :key="item.id" :name="item.spec.name" />
          </bk-select>
        </bk-form-item>
        <ConfigSelector
          class="config-select"
          :type="isFileType ? 'file' : 'kv'"
          :file-config-list="configList"
          :template-config-list="templateConfigList"
          :kv-config-list="kvConfigList"
          :selected-config-ids="selectedConfigIds"
          @select="handleSelectConfig" />
      </div>
    </bk-form>
    <bk-loading
      :loading="tableLoading"
      class="config-table-loading"
      mode="spin"
      theme="primary"
      size="small"
      :opacity="0.7">
      <template v-if="props.service.spec.config_type === 'file'">
        <ConfigTable
          v-if="importConfigList.length"
          :table-data="importConfigList"
          is-clone
          @change="handleConfigTableChange" />
        <TemplateConfigTable
          v-if="importTemplateConfigList.length"
          :table-data="importTemplateConfigList"
          is-clone
          @change="handleTemplateTableChange" />
      </template>
      <KvConfigTable
        v-else-if="importKvConfigList.length"
        :table-data="importKvConfigList"
        is-clone
        @change="handleKvConfigTableChange" />
    </bk-loading>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onMounted, computed } from 'vue';
  import type { IAppItem } from '../../../../../../../types/app';
  import type { IConfigVersion, IConfigImportItem, IConfigKvItem } from '../../../../../../../types/config';
  import {
    getConfigVersionList,
    importFromHistoryVersion,
    importKvFromHistoryVersion,
  } from '../../../../../../api/config';
  import type { ImportTemplateConfigItem } from '../../../../../../../types/template';
  import ConfigSelector from '../../../../../../components/config-selector.vue';
  import ConfigTable from '../../../../templates/list/package-detail/operations/add-configs/import-configs/config-table.vue';
  import TemplateConfigTable from '../../../detail/config/config-list/config-table-list/create-config/import-file/template-config-table.vue';
  import KvConfigTable from '../../../detail/config/config-list/config-table-list/create-config/import-kv/kv-config-table.vue';

  const props = defineProps<{
    service: IAppItem;
    bkBizId: string;
  }>();

  const emits = defineEmits(['select']);

  const formData = ref({
    service: props.service.id,
    version: null as number | null,
  });
  const versionListLoading = ref(false);
  const versionList = ref<IConfigVersion[]>([]);
  const configList = ref<IConfigImportItem[]>([]);
  const templateConfigList = ref<ImportTemplateConfigItem[]>([]);
  const importConfigList = ref<IConfigImportItem[]>([]);
  const importTemplateConfigList = ref<ImportTemplateConfigItem[]>([]);
  const kvConfigList = ref<IConfigKvItem[]>([]);
  const importKvConfigList = ref<IConfigKvItem[]>([]);
  const selectedConfigIds = ref<(string | number)[]>([]);
  const tableLoading = ref(false);
  const formRef = ref();

  const rules = {
    version: [{ required: true, message: '请选择版本', trigger: 'change' }],
  };

  const isFileType = computed(() => props.service.spec.config_type === 'file');

  onMounted(() => {
    getVersionList();
  });

  const getVersionList = async () => {
    try {
      versionListLoading.value = true;
      const params = {
        start: 0,
        all: true,
      };
      const res = await getConfigVersionList(props.bkBizId, props.service.id!, params);
      versionList.value = res.data.details;
    } catch (e) {
      console.error(e);
    } finally {
      versionListLoading.value = false;
    }
  };

  const handleClearTable = () => {
    configList.value = [];
    templateConfigList.value = [];
    selectedConfigIds.value = [];
    importConfigList.value = [];
    importTemplateConfigList.value = [];
    importKvConfigList.value = [];
    handleChange();
  };

  const handleSelectVersion = async (id: number) => {
    tableLoading.value = true;
    try {
      handleClearTable();
      const params = {
        other_app_id: props.service.id!,
        release_id: id,
      };
      if (isFileType.value) {
        const res = await importFromHistoryVersion(props.bkBizId, props.service.id!, params);
        res.data.non_template_configs.forEach((item: any) => {
          const config = {
            ...item,
            ...item.config_item_spec,
            ...item.config_item_spec.permission,
            sign: item.signature,
          };
          delete config.config_item_spec;
          delete config.permission;
          delete config.signature;

          configList.value.push(config);
          importConfigList.value.push(config);
          selectedConfigIds.value.push(item.id);
        });
        res.data.template_configs.forEach((item: ImportTemplateConfigItem) => {
          selectedConfigIds.value.push(`${item.template_space_id} - ${item.template_set_id}`);
          templateConfigList.value.push(item);
          importTemplateConfigList.value.push(item);
        });
      } else {
        const res = await importKvFromHistoryVersion(props.bkBizId, props.service.id!, params);
        res.data.exist.forEach((item: IConfigKvItem) => {
          kvConfigList.value.push(item);
          importKvConfigList.value.push(item);
          selectedConfigIds.value.push(item.key);
        });
        console.log(importKvConfigList.value);
      }
      handleChange();
    } catch (e) {
      console.error(e);
    } finally {
      tableLoading.value = false;
    }
  };

  const handleConfigTableChange = (data: IConfigImportItem[]) => {
    importConfigList.value = data;
    selectedConfigIds.value = selectedConfigIds.value.filter((id) => {
      if (typeof id === 'number') {
        return data.some((config) => config.id === id);
      }
      return true;
    });
    handleChange();
  };

  const handleTemplateTableChange = (deleteId: string) => {
    const index = importTemplateConfigList.value.findIndex(
      (config) => `${config.template_space_id} - ${config.template_set_id}` === deleteId,
    );
    importTemplateConfigList.value.splice(index, 1);
    selectedConfigIds.value = selectedConfigIds.value.filter((id) => id !== deleteId);
    handleChange();
  };

  const handleKvConfigTableChange = (data: IConfigKvItem[]) => {
    importKvConfigList.value = data;
    selectedConfigIds.value = selectedConfigIds.value.filter((id) => {
      return data.some((config) => config.key === id);
    });
    handleChange();
  };

  const handleSelectConfig = (ids: (string | number)[]) => {
    selectedConfigIds.value = ids;
    selectedConfigIds.value.forEach((id) => {
      // 配置文件被删除后重新添加
      if (typeof id === 'number') {
        // 非模板配置文件
        const findConfig = importConfigList.value.find((config) => config.id === id);
        if (!findConfig) {
          const config = configList.value.find((config) => config.id === id);
          importConfigList.value.push(config!);
        }
      } else {
        // 模板配置文件
        const findConfig = importTemplateConfigList.value.find(
          (config) => `${config.template_space_id} - ${config.template_set_id}` === id,
        );
        if (!findConfig) {
          const config = templateConfigList.value.find(
            (config) => `${config.template_space_id} - ${config.template_set_id}` === id,
          );
          importTemplateConfigList.value.push(config!);
        }
      }
    });

    // 删除已选配置文件
    importConfigList.value.forEach((config) => {
      if (!selectedConfigIds.value.includes(config.id)) {
        importConfigList.value = importConfigList.value.filter((item) => item.id !== config.id);
      }
    });
    importTemplateConfigList.value.forEach((config) => {
      if (!selectedConfigIds.value.includes(`${config.template_space_id} - ${config.template_set_id}`)) {
        importTemplateConfigList.value = importTemplateConfigList.value.filter((item) => {
          return (
            `${item.template_space_id} - ${item.template_set_id}` !==
            `${config.template_space_id} - ${config.template_set_id}`
          );
        });
      }
    });
    handleChange();
  };

  const handleChange = () => {
    if (isFileType.value) {
      emits('select', importConfigList.value, importTemplateConfigList.value);
    } else {
      emits('select', importKvConfigList.value, []);
    }
  };
  defineExpose({
    validate: () => formRef.value.validate(),
  });
</script>

<style scoped lang="scss">
  .select-wrap {
    display: flex;
    align-items: center;
    gap: 24px;
    margin-bottom: 16px;
    .bk-form-item {
      margin: 0;
      font-size: 12px;
      :deep(.bk-form-label) {
        font-size: 12px;
      }
    }
    .service-select {
      width: 362px;
    }
    .version-select {
      width: 260px;
    }
  }

  .config-table-loading {
    min-height: 80px;
    :deep(.bk-loading-primary) {
      top: 60px;
      align-items: center;
    }
  }
</style>
