<template>
  <div>
    <div class="process-range">
      <span class="label">{{ $t('进程范围') }}</span>
      <FilterProcess
        :bk-biz-id="bkBizId"
        :is-issued="true"
        :process-ids="ccProcessIds"
        @search="handleSelectProcessRange" />
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
        @tag-remove="handleRemoveTemplate"
        @clear="emits('clearTemplate')">
        <bk-option v-for="item in templateList" :id="item.id" :key="item.id" :name="item.spec.name" />
      </bk-select>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { useRoute } from 'vue-router';
  import type { IConfigTemplateItem } from '../../../../../types/config-template';
  import { getConfigTemplateList } from '../../../../api/config-template';
  import FilterProcess from '../../process/components/filter-process.vue';

  const route = useRoute();

  const props = defineProps<{
    bkBizId: string;
  }>();
  const emits = defineEmits(['selectRange', 'selectTemplate', 'removeTemplate', 'clearTemplate']);

  const selectedTemplate = ref<number[]>([]);
  const templateList = ref<IConfigTemplateItem[]>();
  const filterConditions = ref<Record<string, any>>({});
  const ccProcessIds = ref<string[]>([]);

  onMounted(async () => {
    await loadConfigTemplateList();
    const { processIds, templateIds } = route.query;

    if (Array.isArray(processIds) && processIds.length) {
      ccProcessIds.value = processIds as string[];
    }

    if (Array.isArray(templateIds) && templateIds.length) {
      selectedTemplate.value = templateIds.map(Number);
      emits('selectTemplate', selectedTemplate.value);
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

  const handleSelectProcessRange = (filters: Record<string, any>) => {
    filterConditions.value = filters;
    emits('selectRange', filters);
  };

  const handleSelectTemplate = (id: number) => {
    emits('selectTemplate', id);
  };

  const handleRemoveTemplate = (id: number) => {
    emits('removeTemplate', id);
  };
</script>

<style scoped lang="scss">
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
</style>
