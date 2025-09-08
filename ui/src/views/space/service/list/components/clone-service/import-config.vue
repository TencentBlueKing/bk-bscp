<template>
  <div class="select-wrap">
    <div class="service-select">
      <span class="label">{{ $t('选择服务') }}</span>
      <bk-select :model-value="service.id" class="select" disabled>
        <bk-option :id="service.id" :name="service.spec.name" />
      </bk-select>
    </div>
    <div class="version-select">
      <span class="label">{{ $t('选择版本') }}</span>
      <bk-select
        class="select"
        :loading="versionListLoading"
        filterable
        auto-focus
        :clearable="false"
        @select="handleSelectVersion">
        <bk-option v-for="item in versionList" :id="item.id" :key="item.id" :name="item.spec.name" />
      </bk-select>
    </div>
    <ConfigSelector
      class="config-select"
      type="file"
      :file-config-list="configList"
      :template-config-list="templateConfigList"
      :selected-config-ids="selectedConfigIds"
      @select="handleSelectConfig" />
  </div>
  <bk-loading
    :loading="tableLoading"
    class="config-table-loading"
    mode="spin"
    theme="primary"
    size="small"
    :opacity="0.7">
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
  </bk-loading>
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import type { IAppItem } from '../../../../../../../types/app';
  import type { IConfigVersion, IConfigImportItem } from '../../../../../../../types/config';
  import { getConfigVersionList, importFromHistoryVersion } from '../../../../../../api/config';
  import type { ImportTemplateConfigItem } from '../../../../../../../types/template';
  import ConfigSelector from '../../../../../../components/config-selector.vue';
  import ConfigTable from '../../../../templates/list/package-detail/operations/add-configs/import-configs/config-table.vue';
  import TemplateConfigTable from '../../../detail/config/config-list/config-table-list/create-config/import-file/template-config-table.vue';

  const props = defineProps<{
    service: IAppItem;
  }>();
  const versionListLoading = ref(false);
  const versionList = ref<IConfigVersion[]>([]);
  const configList = ref<IConfigImportItem[]>([]);
  const templateConfigList = ref<ImportTemplateConfigItem[]>([]);
  const importConfigList = ref<IConfigImportItem[]>([]);
  const importTemplateConfigList = ref<ImportTemplateConfigItem[]>([]);
  const selectedConfigIds = ref<(string | number)[]>([]);
  const tableLoading = ref(false);

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
      const res = await getConfigVersionList(String(props.service.biz_id), props.service.id!, params);
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
  };

  const handleSelectVersion = async (id: number) => {
    tableLoading.value = true;
    try {
      handleClearTable();
      const params = {
        other_app_id: props.service.id!,
        release_id: id,
      };
      const res = await importFromHistoryVersion(String(props.service.biz_id), props.service.id!, params);
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
  };

  const handleTemplateTableChange = (deleteId: string) => {
    const index = templateConfigList.value.findIndex(
      (config) => `${config.template_space_id} - ${config.template_set_id}` === deleteId,
    );
    importTemplateConfigList.value.splice(index, 1);
    selectedConfigIds.value = selectedConfigIds.value.filter((id) => id !== deleteId);
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
  };
</script>

<style scoped lang="scss">
  .select-wrap {
    display: flex;
    align-items: center;
    margin-bottom: 16px;
    .service-select,
    .version-select {
      display: flex;
      align-items: center;
      gap: 22px;
      .label {
        font-size: 12px;
        color: #4d4f56;
      }
    }
    .service-select {
      .select {
        width: 362px;
      }
    }
    .version-select {
      margin: 0 16px 0 24px;
      .select {
        width: 260px;
      }
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
