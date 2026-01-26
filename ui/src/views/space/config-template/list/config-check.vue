<template>
  <DetailLayout v-if="isShow" :name="$t('配置检查')" :show-footer="false" @close="emits('update:isShow', false)">
    <template #content>
      <div class="content">
        <div class="process-range">
          <span class="label">{{ $t('进程范围') }}</span>
          <FilterProcess :bk-biz-id="bkBizId" :is-issued="true" @search="handleSelectProcessRange" />
        </div>
        <bk-loading class="process-table-wrap" :loading="pending">
          <ProcessTable
            v-if="templateProcessList.length > 0"
            :bk-biz-id="props.bkBizId"
            :is-check="true"
            :template-process="templateProcessList[0]" />
        </bk-loading>
        <div class="op-btns">
          <bk-button theme="primary" :disabled="templateProcessList[0]?.list.length === 0" @click="handleCheckConfig">
            {{ $t('立即执行') }}
          </bk-button>
          <bk-button @click="emits('update:isShow', false)">{{ $t('取消') }}</bk-button>
        </div>
      </div>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import { ref, watch } from 'vue';
  import { useRouter } from 'vue-router';
  import { getConfigInstanceList, checkConfig } from '../../../../api/config-template';
  import DetailLayout from '../../scripts/components/detail-layout.vue';
  import FilterProcess from '../../process/components/filter-process.vue';
  import ProcessTable from '../config-issued/process-table.vue';
  import { ITemplateProcess } from '../../../../../types/config-template';

  const router = useRouter();

  const props = defineProps<{
    bkBizId: string;
    isShow: boolean;
    id: number;
  }>();
  const emits = defineEmits(['update:isShow']);

  const filterConditions = ref<Record<string, any>>({});
  const pending = ref(false);
  const templateProcessList = ref<ITemplateProcess[]>([]);

  watch(
    () => props.isShow,
    (val) => {
      if (val) {
        loadTemplateInstanceList();
      }
    },
  );

  const handleSelectProcessRange = (filters: Record<string, any>) => {
    filterConditions.value = filters;
    loadTemplateInstanceList();
  };

  const loadTemplateInstanceList = async () => {
    try {
      pending.value = true;
      const params = {
        configTemplateId: props.id,
        configTemplateVersionIds: [],
        search: {
          ...filterConditions.value,
        },
        start: 0,
        all: true,
      };
      const res = await getConfigInstanceList(props.bkBizId, params);
      templateProcessList.value = [
        {
          list: res.config_instances,
          versions: res.filter_options.template_version_choices,
          id: props.id,
          revisionId: res.filter_options.latest_template_revision_id,
          revisionName: res.filter_options.latest_template_revision_name,
        },
      ];
    } catch (error) {
      console.error(error);
    } finally {
      pending.value = false;
    }
  };

  const handleCheckConfig = async () => {
    try {
      const data = {
        configTemplateGroups: templateProcessList.value!.map((templateProcess) => {
          return {
            configTemplateId: templateProcess.id,
            configTemplateVersionId: templateProcess.revisionId,
            ccProcessIds: [...new Set(templateProcess.list.map((item) => item.cc_process_id))],
          };
        }),
      };
      const res = await checkConfig(props.bkBizId, data);
      router.push({ name: 'task-detail', params: { taskId: res.batch_id } });
    } catch (error) {
      console.error(error);
    }
  };
</script>

<style scoped lang="scss">
  .content {
    padding: 24px;
    background: #f5f7fa;
    height: 100%;
  }
  .process-range {
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
  .op-btns {
    display: flex;
    justify-content: center;
    margin-top: 24px;
    gap: 8px;
  }
  .process-table-wrap {
    height: calc(100% - 96px);
    overflow: auto;
  }
</style>
