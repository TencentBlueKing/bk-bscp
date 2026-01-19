<template>
  <DetailLayout :name="$t('配置下发')" @close="handleClose">
    <template #header-suffix>
      <div class="steps-wrap">
        <bk-steps class="steps" theme="primary" :cur-step="stepsStatus.curStep" :steps="stepsStatus.steps" />
      </div>
    </template>
    <template #content>
      <div class="content">
        <SelectRange
          v-show="stepsStatus.curStep === 1"
          :bk-biz-id="spaceId"
          @select-template="handleSelectTemplate"
          @remove-template="handleRemoveTemplate"
          @clear-template="handleClearTemplate"
          @select-range="filterConditions = $event" />
        <div v-show="stepsStatus.curStep === 2" class="batch-op-btns">
          <bk-button class="retry-generate" @click="handleConfigGenerate(true)">{{ $t('全部重新生成') }}</bk-button>
          <bk-button class="retry-fail" @click="handleRetryAll">{{ $t('重试所有失败项') }}</bk-button>
        </div>
        <bk-loading class="process-table-wrap" :loading="pending">
          <div class="process-table-list">
            <ProcessTable
              v-for="template in templateProcessList"
              :bk-biz-id="spaceId"
              :key="template.id"
              :template-process="template"
              :is-generate="stepsStatus.curStep === 2"
              @select="handleSelectVersion(template, $event)"
              @regenerate="handleConfigRegenerateOrRetry($event, 'regenerate')"
              @retry="handleConfigRegenerateOrRetry($event, 'retry')" />
          </div>
        </bk-loading>
      </div>
    </template>
    <template #footer>
      <div class="op-btns">
        <bk-button
          v-if="stepsStatus.curStep === 1"
          theme="primary"
          :disabled="templateProcessList.length === 0"
          @click="handleConfigGenerate">
          {{ t('下一步') }}
        </bk-button>
        <template v-else>
          <bk-button @click="stepsStatus.curStep = 1">{{ t('上一步') }}</bk-button>
          <bk-button :disabled="pending" :loading="pending" theme="primary" @click="handleIssue">
            {{ t('立即下发') }}
          </bk-button>
        </template>
        <bk-button @click="handleClose">{{ t('取消') }}</bk-button>
      </div>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import { ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { useRouter, useRoute } from 'vue-router';
  import {
    generateConfig,
    getConfigInstanceList,
    getGenerateStatus,
    issueConfig,
    retryGenerateConfig,
  } from '../../../../api/config-template';
  import type {
    IGenerateConfigStatus,
    ITemplateProcess,
    ITemplateProcessItem,
  } from '../../../../../types/config-template';
  import DetailLayout from '../../scripts/components/detail-layout.vue';
  import SelectRange from './select-range.vue';
  import useGlobalStore from '../../../../store/global';
  import ProcessTable from './process-table.vue';

  const { t } = useI18n();
  const { spaceId } = storeToRefs(useGlobalStore());
  const router = useRouter();
  const route = useRoute();
  const templateProcessList = ref<ITemplateProcess[]>([]);
  const filterConditions = ref<Record<string, any>>({});
  const selectedTemplateIds = ref<number[]>([]);
  const stepsStatus = ref({
    steps: [{ title: t('选择范围') }, { title: t('配置生成') }],
    curStep: 1,
    controllable: true,
  });
  const pending = ref(false);
  const batchId = ref(0);
  const statusTimer = ref();
  const needGenerate = ref(true);

  watch(
    () => filterConditions.value,
    () => {
      reloadAllTemplateProcess();
    },
  );

  const handleClose = () => {
    if (route.query.processIds) {
      // 进程管理跳转配置下发
      router.push({
        name: 'process-management',
      });
    } else {
      router.push({
        name: 'config-template-list',
      });
    }
  };

  const handleSelectVersion = (template: ITemplateProcess, revisionId: number[]) => {
    loadTemplateInstanceList(template.id, revisionId);
    needGenerate.value = true;
  };

  // 配置生成(全部)
  const handleConfigGenerate = async (isRetry = false) => {
    try {
      stepsStatus.value.curStep = 2;
      if (!needGenerate.value && !isRetry) return;
      pending.value = true;
      const data = {
        configTemplateGroups: templateProcessList.value!.map((templateProcess) => {
          return {
            configTemplateId: templateProcess.id,
            configTemplateVersionId: templateProcess.revisionId,
            ccProcessIds: [...new Set(templateProcess.list.map((item) => item.cc_process_id))],
          };
        }),
      };
      const res = await generateConfig(spaceId.value, data);
      batchId.value = res.batch_id;
      loadGenerateStatus();
      needGenerate.value = false;
    } catch (error) {
      console.error(error);
    } finally {
      pending.value = false;
    }
  };

  // 配置生成重新生成或重试
  const handleConfigRegenerateOrRetry = async (templateProcess: ITemplateProcessItem, type: string) => {
    try {
      pending.value = true;
      const data = {
        batch_id: batchId.value,
        task_id: templateProcess.task_id,
        operation_type: type,
      };
      await retryGenerateConfig(spaceId.value, data);
      loadGenerateStatus();
    } catch (error) {
      console.error(error);
    } finally {
      pending.value = false;
    }
  };

  const handleRetryAll = async () => {
    try {
      pending.value = true;
      const data = {
        batch_id: batchId.value,
        operation_type: 'retry',
      };
      await retryGenerateConfig(spaceId.value, data);
      loadGenerateStatus();
    } catch (error) {
      console.error(error);
    } finally {
      pending.value = false;
    }
  };

  // 加载配置生成状态数据添加到表格中
  const loadGenerateStatus = async () => {
    try {
      if (statusTimer.value) {
        clearTimeout(statusTimer.value);
      }
      const res = await getGenerateStatus(spaceId.value, batchId.value);
      const allStatus = res.config_generate_statuses;
      allStatus.forEach((item: any) => {
        const [templateId, processId, instId] = item.config_instance_key.split('-').map(Number);
        templateProcessList.value.forEach((templateProcess) => {
          templateProcess.list.forEach((process) => {
            if (
              templateProcess.id === templateId &&
              process.cc_process_id === processId &&
              process.module_inst_seq === instId
            ) {
              process.status = item.status;
              process.generation_time = item.generation_time;
              process.task_id = item.task_id;
            }
          });
        });
      });
      if (
        allStatus.some((item: IGenerateConfigStatus) => item.status === 'INITIALIZING' || item.status === 'RUNNING')
      ) {
        statusTimer.value = setTimeout(() => {
          loadGenerateStatus();
        }, 2000);
      }
    } catch (error) {
      console.error(error);
    }
  };

  // 获取单个配置模板实例列表
  const loadTemplateInstanceList = async (templateId: number, versionIds: number[] = []) => {
    try {
      pending.value = true;
      const params = {
        configTemplateId: templateId,
        configTemplateVersionIds: versionIds,
        search: {
          ...filterConditions.value,
        },
        start: 0,
        all: true,
      };
      const res = await getConfigInstanceList(spaceId.value, params);
      const findItem = templateProcessList.value.find((p) => p.id === templateId);
      if (findItem) {
        findItem.list = res.config_instances;
      } else {
        templateProcessList.value.push({
          list: res.config_instances,
          versions: res.filter_options.template_version_choices,
          id: templateId,
          revisionId: res.filter_options.latest_template_revision_id,
          revisionName: res.filter_options.latest_template_revision_name,
        });
      }
    } catch (error) {
      console.error(error);
    } finally {
      pending.value = false;
    }
  };

  // 重新获取所有模板进程列表
  const reloadAllTemplateProcess = () => {
    templateProcessList.value = [];
    selectedTemplateIds.value.forEach((id) => {
      loadTemplateInstanceList(id);
    });
  };

  const handleSelectTemplate = (id: number | number[]) => {
    if (Array.isArray(id)) {
      id.forEach((tid) => {
        if (!selectedTemplateIds.value.includes(tid)) {
          selectedTemplateIds.value.push(tid);
          loadTemplateInstanceList(tid);
        }
      });
      return;
    }
    selectedTemplateIds.value.push(id);
    loadTemplateInstanceList(id);
    needGenerate.value = true;
  };

  const handleRemoveTemplate = (id: number) => {
    selectedTemplateIds.value = selectedTemplateIds.value.filter((tid) => tid !== id);
    templateProcessList.value = templateProcessList.value.filter((t) => t.id !== id);
    needGenerate.value = true;
  };

  const handleClearTemplate = () => {
    selectedTemplateIds.value = [];
    templateProcessList.value = [];
    needGenerate.value = true;
  };

  // 配置下发
  const handleIssue = async () => {
    try {
      pending.value = true;
      const res = await issueConfig(spaceId.value, batchId.value);
      router.push({ name: 'task-detail', params: { taskId: res.batch_id } });
    } catch (error) {
      console.error(error);
    } finally {
      pending.value = false;
    }
  };
</script>

<style scoped lang="scss">
  .steps-wrap {
    flex: 1;
    .steps {
      width: 400px;
      margin: 0 auto;
    }
  }
  .content {
    padding: 24px;
    background: #f5f7fa;
    height: 100%;
  }
  .op-btns {
    display: flex;
    justify-content: center;
    gap: 8px;
    .bk-button {
      width: 88px;
    }
  }

  .batch-op-btns {
    margin-bottom: 16px;
    .retry-generate {
      width: 116px;
      margin-right: 8px;
    }
    .retry-fail {
      width: 130px;
    }
  }
  .process-table-wrap {
    height: calc(100% - 96px);
    overflow: auto;
  }
  .process-table-list {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
</style>
