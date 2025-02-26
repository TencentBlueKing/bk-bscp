<template>
  <div class="preview-wrap">
    <div class="header">
      <div class="header-left">
        <span class="title">{{ $t('数据预览') }}</span>
        <span class="refresh" @click="loadData">
          <right-turn-line class="icon" />
          <span>{{ $t('刷新数据') }}</span>
        </span>
      </div>
      <div class="header-right">
        <bk-button :class="['button', { active: isShowFieldSetting }]" @click="handleOperation('set')">
          <cog-shape class="icon" />
          {{ $t('字段设置') }}
        </bk-button>
        <bk-button :class="['button', { active: isShowDataClean }]" @click="handleOperation('clear')">
          <funnel class="icon" />
          {{ $t('数据清洗') }}
          <span>{{ `(${ruleList.length})` }}</span>
        </bk-button>
      </div>
    </div>
    <fieldSetting
      v-if="isShowFieldSetting"
      :list="allfieldList"
      :select-list="selectFieldList"
      @change="handleChangeField"
      @close="isShowFieldSetting = false" />
    <dataClean
      v-if="isShowDataClean"
      :fields="allfieldList"
      :rule-list="ruleList"
      @change="handleDataClean"
      @close="isShowDataClean = false" />
    <vxe-table
      :data="tableData"
      border
      ref="tableRef"
      max-height="500"
      :loading="tableLoading"
      :column-config="{ resizable: true }"
      :scroll-y="{ enabled: true, gt: 0 }"
      :scroll-x="{ enabled: true, gt: 0 }">
      <vxe-column v-for="field in selectFieldList" :key="field.name" min-width="200" min-height="48">
        <template #header>
          <div class="head">
            <div class="alias">{{ field.alias }}</div>
            <div class="name">{{ field.name }}</div>
          </div>
        </template>
        <template #default="{ row }">
          <template v-if="row.spec.content[field.name]">
            <div v-if="Array.isArray(row.spec.content[field.name])" class="tag-list">
              <bk-tag v-for="tag in row.spec.content[field.name]" :key="tag" radius="4px">
                {{ tag }}
              </bk-tag>
            </div>
            <div v-else>{{ row.spec.content[field.name] }}</div>
          </template>
          <span v-else>--</span>
        </template>
      </vxe-column>
    </vxe-table>
  </div>
</template>

<script lang="ts" setup>
  import { ref, watch, onMounted } from 'vue';
  import {
    ILocalTableEditData,
    IDataCleanItem,
    IConfigTableForm,
    IFieldItem,
  } from '../../../../../../../../../../../types/kv-table';
  import { getTableData } from '../../../../../../../../../../api/kv-table';
  import { RightTurnLine, CogShape, Funnel } from 'bkui-vue/lib/icon';
  import fieldSetting from './field-setting.vue';
  import dataClean from './data-clean.vue';

  const props = defineProps<{
    bkBizId: string;
    tableForm: IConfigTableForm;
  }>();
  const emits = defineEmits(['change']);

  const tableLoading = ref(false);
  const tableData = ref<ILocalTableEditData[]>([]);
  const allfieldList = ref<IFieldItem[]>([]);
  const selectFieldList = ref<IFieldItem[]>([]);
  const isShowFieldSetting = ref(false);
  const isShowDataClean = ref(false);
  const ruleList = ref<IDataCleanItem[]>([]);

  onMounted(async () => {
    ruleList.value = props.tableForm.filter_condition?.labels_and || [];
    await loadData();
    if (props.tableForm.filter_fields!.length > 0) {
      selectFieldList.value = allfieldList.value.filter((item) => props.tableForm.filter_fields?.includes(item.name));
    }
  });

  const loadData = async () => {
    try {
      tableLoading.value = true;
      const query: any = { start: 0, all: true };
      if (ruleList.value.length) {
        query.filter_condition = { labels_and: ruleList.value };
      }
      const res = await getTableData(props.bkBizId, props.tableForm.managed_table_id!, query);
      allfieldList.value = res.fields;
      selectFieldList.value = res.fields;
      tableData.value = res.details;
    } catch (error) {
      console.error(error);
    } finally {
      tableLoading.value = false;
    }
  };

  watch(
    () => props.tableForm.managed_table_id,
    () => {
      isShowFieldSetting.value = false;
      isShowDataClean.value = false;
      ruleList.value = [];
      loadData();
    },
  );

  const handleOperation = (type: string) => {
    if (type === 'set') {
      isShowDataClean.value = false;
      isShowFieldSetting.value = !isShowFieldSetting.value;
    } else {
      isShowFieldSetting.value = false;
      isShowDataClean.value = !isShowDataClean.value;
    }
  };

  const handleDataClean = (list: IDataCleanItem[]) => {
    ruleList.value = list;
    loadData();
    const fieldsList = selectFieldList.value.map((item) => item.name);
    emits('change', fieldsList, ruleList.value);
  };

  const handleChangeField = (list: IFieldItem[]) => {
    selectFieldList.value = list;
    const fieldsList = list.map((item) => item.name);
    emits('change', fieldsList, ruleList.value);
  };
</script>

<style scoped lang="scss">
  .preview-wrap {
    border-top: 1px solid #dcdee5;
    .header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin: 8px 0 16px;
      .header-left {
        display: flex;
        .title {
          margin-right: 16px;
          font-weight: 700;
          font-size: 14px;
          color: #63656e;
        }
        .refresh {
          display: flex;
          align-items: center;
          gap: 4px;
          color: #3a84ff;
          font-size: 12px;
          line-height: 20px;
          cursor: pointer;
          .icon {
            font-size: 14px;
          }
        }
      }
      .header-right {
        display: flex;
        gap: 8px;
        .button {
          .icon {
            margin-right: 6px;
            color: #979ba5;
          }
        }
        .active {
          border: 1px solid #3a84ff;
          color: #3a84ff;
          .icon {
            color: #3a84ff;
          }
        }
      }
    }
  }
  .head {
    .alias {
      color: #313238;
    }
    .name {
      color: #979ba5;
    }
  }
  .tag-list {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
  }
  .exception-part {
    min-width: 100%;
    border: 1px solid #dcdee5;
    border-top: none;
    height: 200px;
  }
  .vxe-table--render-default {
    overflow: hidden;
  }
  :deep(.vxe-cell) {
    padding: 0 16px;
  }
  .vxe-table {
    :deep(.vxe-table--render-wrapper) {
      .vxe-cell {
        padding: 0 16px;
      }
    }
  }
</style>
