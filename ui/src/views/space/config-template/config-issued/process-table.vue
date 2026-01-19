<template>
  <div class="table-wrap">
    <div class="head">
      <div class="head-left" @click="isShow = !isShow">
        <AngleDownFill :class="['angle-icon', isShow && 'expanded']" />
        <span class="template-name">{{ templateName }}</span>
        <span class="version">
          ({{ isCheck ? $t('最后一次下发') : $t('即将下发') }} <span>{{ `#${templateProcess.revisionName}` }}</span>
          {{ $t('版本') }})
        </span>
      </div>
      <div class="head-right">
        <template v-if="isCheck">
          <span>
            {{ $t('已选') }}
            <span class="count">{{ templateProcess.list.length }}</span>
            {{ $t('个') }}
          </span>
        </template>
        <template v-else-if="isGenerate">
          <div class="status-total">
            <div class="success-count">
              <span class="dot SUCCESS"></span>
              <span class="count">
                {{ templateProcess.list.filter((process) => process.status === 'SUCCESS').length }}
              </span>
              <span>{{ $t('个已生成') }}</span>
            </div>
            <div class="failure-count">
              <span class="dot FAILURE"></span>
              <span class="count">
                {{ templateProcess.list.filter((process) => process.status === 'FAILURE').length }}
              </span>
              <span>{{ $t('个生成失败') }}</span>
            </div>
          </div>
        </template>
        <template v-else>
          <bk-popover theme="light" trigger="click" placement="bottom">
            <span :class="['version-select-trigger', { checked: checkedVersion.length }]">
              <funnel class="funnel-icon" />
              <span v-if="checkedVersion.length">{{
                $t('已选{n}/{m}个版本', { n: checkedVersion.length, m: templateProcess.versions.length })
              }}</span>
              <span v-else>{{ $t('按版本选择') }}</span>
            </span>
            <template #content>
              <div class="version-select-content">
                <div class="info">{{ $t('根据配置模板历史版本，筛选对应的实例') }}</div>
                <bk-checkbox-group v-model="checkedVersion" @change="handleSelectVersion">
                  <bk-checkbox v-for="version in templateProcess.versions" :key="version.id" :label="version.id">
                    {{ version.name }}
                  </bk-checkbox>
                </bk-checkbox-group>
              </div>
            </template>
          </bk-popover>
          <span class="line"></span>
          <span>
            {{ $t('已选') }}
            <span class="count">{{ checkedVersion.length }}</span>
            {{ $t('个') }}
          </span>
        </template>
      </div>
    </div>
    <div v-show="isShow">
      <PrimaryTable class="border" :data="templateProcess.list" row-key="cc_process_id">
        <TableColumn :title="$t('进程别名')" col-key="process_alias" width="180" />
        <TableColumn :title="$t('所属拓扑')" ellipsis>
          <template #default="{ row }: { row: ITemplateProcessItem }">
            {{ `${row.set} / ${row.module} / ${row.service_instance}` }}
          </template>
        </TableColumn>
        <TableColumn col-key="cc_process_id" width="133">
          <template #title>
            <span class="tips-title" v-bk-tooltips="{ content: $t('对应 CMDB 中唯一 ID'), placement: 'top' }">
              {{ $t('CC 进程ID') }}
            </span>
          </template>
        </TableColumn>
        <TableColumn col-key="module_inst_seq" width="133">
          <template #title>
            <span class="tips-title" v-bk-tooltips="{ content: $t('模块下唯一标识'), placement: 'top' }">
              ModuleInstSeq
            </span>
          </template>
        </TableColumn>
        <template v-if="isGenerate">
          <TableColumn :title="$t('状态')">
            <template #default="{ row }: { row: ITemplateProcessItem }">
              <span class="status">
                <Spinner v-if="row.status === 'INITIALIZING' || row.status === 'RUNNING'" class="spinner-icon" />
                <span v-else :class="['dot', row.status]"></span>
                {{ GENERATE_STATUS[row.status as keyof typeof GENERATE_STATUS] }}
              </span>
            </template>
          </TableColumn>
          <TableColumn :title="$t('生成时间')">
            <template #default="{ row }: { row: ITemplateProcessItem }">
              <span>
                {{ getGenrateTime(row.status, row.generation_time) }}
              </span>
            </template>
          </TableColumn>
        </template>
        <template v-else>
          <TableColumn :title="$t('版本号')" col-key="config_version_name" width="140" />
          <TableColumn :title="$t('版本描述')" col-key="config_version_memo" ellipsis />
        </template>
        <TableColumn :title="$t('操作')" width="196">
          <template #default="{ row }: { row: ITemplateProcessItem }">
            <bk-button
              v-if="isCheck"
              theme="primary"
              text
              :disabled="row.config_version_name === '-'"
              @click="handleView(row)">
              {{ $t('查看配置') }}
            </bk-button>
            <div v-else-if="isGenerate" class="op-btns">
              <bk-button v-if="row.status === 'SUCCESS'" theme="primary" text @click="emits('regenerate', row)">
                {{ $t('重新生成') }}
              </bk-button>
              <bk-button v-if="row.status === 'FAILURE'" theme="primary" text @click="emits('retry', row)">
                {{ $t('重试') }}
              </bk-button>
              <bk-button theme="primary" text @click="handleView(row)">{{ $t('查看') }}</bk-button>
            </div>
            <bk-button v-else theme="primary" text @click="handleDiff(row)"> {{ $t('配置对比') }}</bk-button>
          </template>
        </TableColumn>
      </PrimaryTable>
    </div>
  </div>
  <ConfigDiff
    v-model:show="diffSliderData.open"
    :space-id="props.bkBizId"
    :instance="diffSliderData.data" />
  <ConfigDetail
    v-model:is-show="detailSliderData.open"
    :is-check="isCheck"
    :bk-biz-id="bkBizId"
    :data="detailSliderData.data" />
</template>

<script lang="ts" setup>
  import { ref, computed } from 'vue';
  import { ITemplateProcess, ITemplateProcessItem } from '../../../../../types/config-template';
  import { AngleDownFill, Funnel, Spinner } from 'bkui-vue/lib/icon';
  import { GENERATE_STATUS } from '../../../../constants/config-template';
  import { datetimeFormat } from '../../../../utils';
  import ConfigDiff from './config-diff.vue';
  import ConfigDetail from './config-detail.vue';

  const props = defineProps<{
    templateProcess: ITemplateProcess;
    isGenerate?: boolean;
    isCheck?: boolean;
    bkBizId: string;
  }>();
  const emits = defineEmits(['select', 'regenerate', 'retry']);

  const isShow = ref(true);
  const checkedVersion = ref<string[]>([]);
  const diffSliderData = ref<{
    open: boolean;
    data: { ccProcessId: number; moduleInstSeq: number; configVersionId: number; configTemplateId: number };
    filePath: string;
  }>({
    open: false,
    data: { ccProcessId: 0, moduleInstSeq: 0, configVersionId: 0, configTemplateId: 0 },
    filePath: '',
  });
  const detailSliderData = ref<{
    open: boolean;
    data: {
      ccProcessId: number;
      moduleInstSeq: number;
      configTemplateId: number;
      taskId: string;
    };
  }>({
    open: false,
    data: { ccProcessId: 0, moduleInstSeq: 0, configTemplateId: 0, taskId: '' },
  });

  const templateName = computed(() => {
    if (!props.templateProcess.list.length) return '';
    return `${props.templateProcess.list[0].config_template_name} / ${props.templateProcess.list[0].file_name}`;
  });

  const handleDiff = async (row: ITemplateProcessItem) => {
    diffSliderData.value = {
      open: true,
      filePath: row.file_name,
      data: {
        ccProcessId: row.cc_process_id,
        moduleInstSeq: row.module_inst_seq,
        configVersionId: props.templateProcess.revisionId,
        configTemplateId: row.config_template_id,
      },
    };
  };

  const handleSelectVersion = (val: string[]) => {
    emits('select', val);
  };

  // 查看配置文件详情
  const handleView = (row: ITemplateProcessItem) => {
    detailSliderData.value = {
      open: true,
      data: {
        ccProcessId: row.cc_process_id,
        moduleInstSeq: row.module_inst_seq,
        configTemplateId: row.config_template_id,
        taskId: row.task_id,
      },
    };
  };

  const getGenrateTime = (status: string, generationTime: string) => {
    if (status === 'SUCCESS' || status === 'FAILURE') {
      return datetimeFormat(generationTime);
    }
    return '--';
  };
</script>

<style scoped lang="scss">
  .table-wrap {
    .head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 0 16px;
      height: 42px;
      background: #dcdee5;
      .head-left {
        display: flex;
        align-items: center;
        gap: 8px;
        cursor: pointer;
        .angle-icon {
          margin-right: 4px;
          transition: transform 0.3s;
          color: #63656e;
          transform: rotate(90deg);
          &.expanded {
            transform: rotate(180deg);
            transition: transform 0.3s;
          }
        }
        .template-name {
          color: #63656e;
          font-weight: 700;
        }
        .version {
          font-size: 12px;
          color: #979ba5;
          font-weight: 700;
          span {
            color: #3a84ff;
          }
        }
      }
      .head-right {
        display: flex;
        align-items: center;
        font-size: 12px;
        .version-select-trigger {
          display: flex;
          align-items: center;
          gap: 4px;
          font-size: 12px;
          cursor: pointer;
          color: #4d4f56;
          .funnel-icon {
            color: #979ba5;
            font-size: 14px;
          }
          &.checked {
            color: #3a84ff;
            .funnel-icon {
              color: #3a84ff;
            }
          }
        }
        .line {
          width: 1px;
          height: 16px;
          background: #c4c6cc;
          margin: 0 12px;
        }
        .count {
          color: #3a84ff;
          font-weight: bold;
        }
        .status-total {
          display: flex;
          gap: 20px;
          .success-count,
          .failure-count {
            display: flex;
            align-items: center;
            gap: 4px;
          }
        }
      }
    }
  }
  .version-select-content {
    width: 260px;
    .info {
      margin-bottom: 12px;
    }
  }
  .status {
    display: flex;
    align-items: center;
    gap: 8px;
    .spinner-icon {
      font-size: 14px;
      color: #3a84ff;
    }
  }

  .dot {
    width: 8px;
    height: 8px;
    background: #f0f1f5;
    border: 1px solid #c4c6cc;
    border-radius: 50%;
    &.SUCCESS {
      background: #cbf0da;
      border-color: #2caf5e;
    }
    &.FAILURE {
      background: #ffdddd;
      border-color: #ea3636;
    }
  }
  .op-btns {
    display: flex;
    gap: 8px;
  }
</style>
