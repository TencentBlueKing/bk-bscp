<template>
  <div class="content-wrap">
    <div class="process-range">
      <span class="label">{{ $t('进程范围') }}</span>
      <FilterProcess :bk-biz-id="bkBizId" :is-issued="true" :process-ids="ccProcessIds" @search="handleSelectProcess" />
    </div>
    <div class="config-template">
      <span class="label">{{ $t('配置模板') }}</span>
      <bk-select
        class="bk-select"
        v-model="selectedTemplate"
        multiple-mode="tag"
        filterable
        multiple
        @select="handleSelectTemplate"
        @deselect="handleRemoveTemplate"
        @tag-remove="handleRemoveTemplate">
        <bk-option v-for="item in templateList" :id="item.id" :key="item.id" :name="item.spec.name" />
      </bk-select>
    </div>
    <div class="process-table-list">
      <template v-for="template in templateProcessList" :key="template.id">
        <ProcessTable
          v-if="template.list.length"
          :list="template.list"
          :versions="template.versions"
          @select="handleSelectVersion(template, $event)" />
      </template>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { useRoute } from 'vue-router';
  import type { IConfigTemplateItem, ITemplateProcess } from '../../../../../../types/config-template';
  import { getConfigTemplateList, getConfigInstanceList } from '../../../../../api/config-template';
  import ProcessTable from './process-table.vue';
  import FilterProcess from '../../../process/components/filter-process.vue';

  const route = useRoute();

  const props = defineProps<{
    bkBizId: string;
  }>();

  const selectedTemplate = ref<number[]>([]);
  const templateList = ref<IConfigTemplateItem[]>();
  const filterConditions = ref<Record<string, any>>({});
  const templateProcessList = ref<ITemplateProcess[]>([]);
  const ccProcessIds = ref<string[]>([]);

  onMounted(async () => {
    await loadConfigTemplateList();
    const { processIds, templateIds } = route.query;

    if (Array.isArray(processIds) && processIds.length) {
      ccProcessIds.value = processIds as string[];
    }

    if (Array.isArray(templateIds) && templateIds.length) {
      selectedTemplate.value = templateIds.map(Number);
    }
    if (selectedTemplate.value.length > 0 && ccProcessIds.value.length === 0) {
      reloadAllTemplateProcess();
    }
  });

  const loadConfigTemplateList = async () => {
    try {
      const params = {
        start: 0,
        all: true,
      };
      const res = await getConfigTemplateList(props.bkBizId, params);
      templateList.value = res.details.filter((item: IConfigTemplateItem) => {
        return item.attachment.cc_process_ids.length + item.attachment.cc_template_process_ids.length > 0;
      });
    } catch (error) {
      console.error(error);
    }
  };

  // 获取单个配置模板实例列表
  const loadTemplateInstanceList = async (templateId: number, versionIds: string[] = []) => {
    try {
      const params = {
        configTemplateId: templateId,
        configTemplateVersionIds: versionIds,
        search: {
          ...filterConditions.value,
        },
        start: 0,
        all: true,
      };
      const res = await getConfigInstanceList(props.bkBizId, params);
      const findItem = templateProcessList.value.find((p) => p.id === templateId);
      if (findItem) {
        findItem.list = res.config_instances;
      } else {
        templateProcessList.value.push({
          list: res.config_instances,
          versions: res.filter_options.template_version_choices,
          id: templateId,
        });
      }
    } catch (error) {
      console.error(error);
    }
  };

  const handleSelectProcess = (filters: Record<string, any>) => {
    filterConditions.value = filters;
    reloadAllTemplateProcess();
  };

  const handleSelectTemplate = (id: number) => {
    loadTemplateInstanceList(id);
  };

  const handleRemoveTemplate = (id: number) => {
    templateProcessList.value = templateProcessList.value.filter((t) => t.id !== id);
  };

  // 重新获取所有模板进程列表
  const reloadAllTemplateProcess = () => {
    templateProcessList.value = [];
    selectedTemplate.value.forEach((id) => {
      loadTemplateInstanceList(id);
    });
  };

  const handleSelectVersion = (template: ITemplateProcess, version: string[]) => {
    loadTemplateInstanceList(template.id, version);
  };
</script>

<style scoped lang="scss">
  .content-wrap {
    height: 100%;
    .process-range,
    .config-template {
      display: flex;
      align-items: center;
      margin-bottom: 16px;
      .label {
        position: relative;
        width: 74px;
        margin-right: 8px;
        &::after {
          content: '*';
          position: absolute;
          right: 0;
          top: 50%;
          transform: translateY(-50%);
          font-size: 12px;
          color: #ea3636;
        }
      }
      .bk-select {
        width: 962px;
      }
    }
    .process-table-list {
      height: calc(100% - 96px);
      overflow: auto;
      .table-wrap {
        &:not(:last-child) {
          margin-bottom: 16px;
        }
      }
    }
  }
</style>
