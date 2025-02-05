<template>
  <vxe-table
    ref="tableRef"
    border
    :max-height="tableHeight"
    :data="tableData"
    show-overflow
    :header-row-height="50"
    :edit-config="{ mode: 'cell', trigger: 'click', showStatus: true }"
    :row-config="{ height: 42 }"
    :scroll-y="{ enabled: true, gt: 0 }"
    :row-class-name="rowClassName"
    :cell-class-name="cellClassName"
    :edit-rules="validRules"
    keep-source>
    <vxe-column
      v-for="(field, index) in fieldsList"
      :key="index"
      min-width="200"
      :fixed="field.primary ? 'left' : null"
      :field="field.name">
      <template #header>
        <div class="fields-cell">
          <div class="fields-content">
            <div class="show-name">{{ field.alias }}</div>
            <div class="fields-name">{{ field.name }}</div>
          </div>
          <BatchSetPop
            v-if="!field.primary"
            :type="field.column_type"
            :is-multiple="field.selected"
            :enum-value="field.enum_value"
            @confirm="handleConfirmBatchSet(field.name, $event)" />
        </div>
      </template>
      <template #default="{ row }">
        <vxe-select
          v-if="field.column_type === 'enum'"
          v-model="row.content[field.name]"
          :options="field.enum_value"
          :multiple="field.selected"
          :disabled="row.status === 'DELETE'"
          @change="emits('change', tableData)" />
        <vxe-input
          v-else-if="field.column_type === 'string'"
          v-model="row.content[field.name]"
          :disabled="row.status === 'DELETE'"
          @change="emits('change', tableData)">
          <template v-if="field.primary && row.status !== 'REVISE' && row.status !== 'UNCHANGE'" #suffix>
            <div class="tag-wrap">
              <bk-tag size="small" :theme="row.status === 'ADD' ? 'success' : 'danger'">
                {{ row.status === 'ADD' ? $t('新增') : $t('删除') }}
              </bk-tag>
            </div>
          </template>
        </vxe-input>
        <vxe-number-input
          v-else
          v-model="row.content[field.name]"
          :disabled="row.status === 'DELETE'"
          @change="emits('change', tableData)">
          <template v-if="field.primary && row.status !== 'REVISE' && row.status !== 'UNCHANGE'" #suffix>
            <div class="tag-wrap">
              <bk-tag size="small" :theme="row.status === 'ADD' ? 'success' : 'danger'">
                {{ row.status === 'ADD' ? $t('新增') : $t('删除') }}
              </bk-tag>
            </div>
          </template>
        </vxe-number-input>
      </template>
    </vxe-column>
    <vxe-column width="120" :fixed="'right'">
      <template #header>
        <span class="operation-header">{{ $t('操作') }}</span>
      </template>
      <template #default="{ row, rowIndex }">
        <div class="action-btns" v-if="row.status !== 'DELETE'">
          <i class="bk-bscp-icon icon-plus-circle-shape" @click="handleAddData(rowIndex)"></i>
          <i class="bk-bscp-icon icon-minus-circle-shape" @click="handleDeleteData(row, rowIndex)"></i>
        </div>
      </template>
    </vxe-column>
    <template #empty>
      <tableEmpty :is-search-empty="false" />
    </template>
  </vxe-table>
</template>

<script lang="ts" setup>
  import { watch, ref, onMounted } from 'vue';
  import {
    IFiledsItemEditing,
    IFieldItem,
    ILocalTableEditData,
    ILocalTableDataItem,
  } from '../../../../../../types/kv-table';
  import BatchSetPop from './batch-set-pop.vue';
  import { cloneDeep } from 'lodash';
  import tableEmpty from '../../../../../components/table/table-empty.vue';

  const props = defineProps<{
    fields: IFieldItem[];
    data: ILocalTableDataItem[];
  }>();

  const emits = defineEmits(['change', 'delete']);

  const fieldsList = ref<IFiledsItemEditing[]>([]);
  const tableData = ref<ILocalTableEditData[]>([]);
  const publishData = ref<ILocalTableEditData[]>([]); // 已上线的数据
  // const errorCells = ref<{ row: number; col: number }[]>([]);
  const tableHeight = ref(0);
  const validRules = ref<{ [key: string]: any }>({});
  const tableRef = ref();

  watch(
    () => props.fields,
    () => {
      fieldsList.value = props.fields.map((item, index) => {
        let default_value: string | string[] | undefined;
        let enum_value;
        if (item.column_type === 'enum' && item.enum_value !== '') {
          enum_value = JSON.parse(item.enum_value);
          if (enum_value.every((item: any) => typeof item === 'string')) {
            // 字符串数组，显示名和实际值按一致处理
            enum_value = enum_value.map((value: string) => {
              return {
                label: value,
                value,
              };
            });
          }
        } else {
          enum_value = item.enum_value;
        }
        if (item.column_type === 'enum') {
          const isMultiSelect = item.selected; // 是否多选
          const hasDefaultValue = !!item.default_value;
          if (isMultiSelect) {
            // 多选情况下，解析为数组或赋值为空数组
            default_value = hasDefaultValue ? JSON.parse(item.default_value as string) : [];
          } else {
            // 单选情况下，直接赋值或设置为 undefined select组件tag模式设置空字符串会有空tag
            default_value = hasDefaultValue ? item.default_value : undefined;
          }
        } else {
          // 非枚举类型直接赋值
          default_value = item.default_value;
        }
        return {
          ...item,
          enum_value,
          default_value,
          id: Date.now() + index,
        };
      });
      getValidRules();
    },
  );

  watch(
    () => props.data,
    () => {
      if (props.data.length) {
        tableData.value = props.data.map((item) => {
          return {
            id: item.id,
            custom_id: item.id,
            ...item.spec,
          };
        });
        publishData.value = cloneDeep(
          tableData.value.filter((item) => item.status === 'REVISE' || item.status === 'UNCHANGE'),
        );
      }
    },
  );

  onMounted(() => {
    calculateTableHeight();
  });

  const calculateTableHeight = () => {
    const screenHeight = window.innerHeight; // 屏幕高度
    const reservedHeight = 150; // 预留顶部/底部空间（例如头部和分页组件高度）
    tableHeight.value = (screenHeight - reservedHeight) * 0.8;
  };

  const handleAddData = (index?: number) => {
    const content: { [key: string]: any } = {};
    fieldsList.value.forEach((item) => {
      if (item.column_type === 'number') {
        content[item.name] = Number(item.default_value) || null;
      } else {
        content[item.name] = item.default_value || '';
      }
    });
    if (index) {
      tableData.value.splice(index + 1, 0, { custom_id: Date.now(), content, status: 'ADD', id: 0 });
    } else {
      tableData.value.push({ custom_id: Date.now(), content, status: 'ADD', id: 0 });
    }
    emits('change', tableData.value);
  };

  const handleDeleteData = (data: ILocalTableEditData, index: number) => {
    if (data.status === 'REVISE' || data.status === 'UNCHANGE') {
      data.status = 'DELETE';
    }
    tableData.value.splice(index, 1);
    emits('change', tableData.value);
  };

  const handleConfirmBatchSet = (field: string, val: any) => {
    tableData.value.forEach((item) => {
      item.content[field] = val;
    });
  };

  const rowClassName = ({ row }: { row: ILocalTableEditData }) => {
    return row.status;
  };

  const cellClassName = ({ column }: any) => {
    const primaryField = fieldsList.value.find((item) => item.primary)?.name;
    if (column.field === primaryField) {
      return 'primary';
    }
    return null;
  };

  const getValidRules = () => {
    fieldsList.value.forEach((item) => {
      const rules = [];
      if (item.not_null) {
        rules.push({
          validator({ row }: any) {
            if (row.status === 'DELETE') return;
            if (row.content[item.name] === '' || row.content[item.name] === null) {
              return new Error('不能为空');
            }
          },
        });
      }
      if (item.unique) {
        rules.push({
          validator({ row, column }: any) {
            if (row.status === 'DELETE') return;
            const values = tableData.value
              .filter((item) => item.status !== 'DELETE')
              .map((item) => item.content[column.field]);
            const occurrences = values.filter((value) => value === row.content[item.name]);
            if (occurrences.length > 1) {
              return new Error('不能重复');
            }
          },
        });
      }
      validRules.value[item.name] = rules;
    });
  };

  const fullValidEvent = async () => {
    const $table = tableRef.value;
    if ($table) {
      const errMap = await $table.fullValidate(true);
      if (errMap) {
        return false;
      }
      return true;
    }
  };

  defineExpose({
    fullValidEvent,
    handleAddData,
  });
</script>

<style scoped lang="scss">
  .table-wrap {
    position: relative;
    overflow-x: auto;
    max-width: 100%;
  }
  .fields-cell {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: space-between;
    .fields-content {
      font-size: 12px;
      line-height: 20px;
      .show-name {
        color: #313238;
      }
      .fields-name {
        color: #979ba5;
      }
    }
    .edit-line {
      position: absolute;
      right: 0;
      top: 10px;
      color: #3a84ff;
    }
  }
  .action-btns {
    display: flex;
    gap: 18px;
    padding-left: 16px;
    width: 38px;
    font-size: 14px;
    color: #c4c6cc;
    cursor: pointer;
    i:hover {
      color: #3a84ff;
    }
  }

  .vxe-body--column.col--valid-error {
    .vxe-input,
    .vxe-number-input {
      border: 1px solid #ff4d4f;
    }
  }

  .vxe-input:not(.is--active) {
    border: none;
  }
  .vxe-select:not(.is--active) {
    :deep(.vxe-input) {
      border: none;
    }
  }
  .vxe-number-input {
    // :deep(.vxe-input--control-icon) {
    //   display: none;
    // }
    &:not(.is--active) {
      border: none;
    }
  }
  .operation-header {
    display: inline-block;
    height: 40px;
    line-height: 40px;
  }
  .vxe-table {
    :deep(.vxe-table--body) {
      .vxe-input-inner {
        padding: 0;
      }
      .DELETE {
        .vxe-input {
          text-decoration: line-through;
          .vxe-input--inner,
          .vxe-input--suffix {
            background: #fff;
          }
        }
        .vxe-number-input {
          text-decoration: line-through;
          .vxe-number-input--inner,
          .vxe-number-input--suffix {
            background: #fff;
          }
        }
        .vxe-body--column:not(.primary) {
          background: #ffeeee;
          .vxe-input {
            text-decoration: line-through;
            .vxe-input--inner,
            .vxe-input--suffix {
              background: #ffeeee;
            }
          }
          .vxe-number-input {
            text-decoration: line-through;
            .vxe-number-input--inner,
            .vxe-number-input--suffix {
              background: #ffeeee;
            }
          }
        }
      }
      .ADD {
        .vxe-body--column:not(.primary) {
          background-color: #f2fff4;
          .vxe-input {
            .vxe-input--inner,
            .vxe-input--suffix {
              background: #f2fff4;
            }
          }
          .vxe-number-input {
            .vxe-number-input--inner,
            .vxe-number-input--suffix {
              background: #f2fff4;
            }
          }
        }
      }
      .delete-content {
        padding: 0 7px;
        color: #c4c6cc;
      }
    }
  }
</style>

<style>
  .popover-wrap {
    padding: 0 !important;
  }
  .vxe-select--panel {
    z-index: 9999 !important;
  }
</style>
