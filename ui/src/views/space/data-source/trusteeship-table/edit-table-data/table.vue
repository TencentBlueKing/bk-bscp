<template>
  <div class="table-wrap">
    <table class="data-table">
      <thead>
        <tr>
          <th v-for="(item, index) in fieldsList" :key="index" :class="[{ 'left-sticky': index === 0 }]">
            <div class="fields-cell">
              <div class="fields-content">
                <div class="show-name">{{ item.alias }}</div>
                <div class="fields-name">{{ item.name }}</div>
              </div>
              <BatchSetPop
                v-if="!item.primary"
                :type="item.column_type"
                :is-multiple="item.selected"
                :enum-value="item.enum_value"
                @confirm="handleConfirmBatchSet(item.name, $event)" />
            </div>
          </th>
          <th class="operation right-sticky">{{ $t('操作') }}</th>
        </tr>
      </thead>
      <tbody class="table-body">
        <tr v-for="(tableItem, index) in tableData" :key="index">
          <td
            v-for="(field, fieldIndex) in fieldsList"
            :key="field.name"
            :class="[
              tableItem.status,
              { primary: field.primary },
              getCellCls(tableItem, field),
              { 'left-sticky': fieldIndex === 0 },
            ]">
            <bk-select
              v-if="field.column_type === 'enum'"
              class="enum-select"
              v-model="tableItem.content[field.name]"
              auto-focus
              :multiple="field.selected"
              clearable
              :filterable="false"
              :disabled="tableItem.status === 'DELETE'"
              @change="emits('change', tableData)">
              <bk-option
                v-for="(enumItem, i) in field.enum_value"
                :id="enumItem.value"
                :key="i"
                :name="enumItem.text" />
            </bk-select>
            <bk-input
              v-else
              v-model="tableItem.content[field.name]"
              :disabled="tableItem.status === 'DELETE'"
              @change="emits('change', tableData)">
              <template v-if="field.primary && tableItem.status !== 'REVISE'" #suffix>
                <div class="tag-wrap">
                  <bk-tag size="small" :theme="tableItem.status === 'ADD' ? 'success' : 'danger'">
                    {{ tableItem.status === 'ADD' ? $t('新增') : $t('删除') }}
                  </bk-tag>
                </div>
              </template>
            </bk-input>
          </td>
          <td class="operation right-sticky">
            <div class="action-btns" v-if="tableItem.status !== 'DELETE'">
              <i class="bk-bscp-icon icon-add" @click="handleAddData(index)"></i>
              <i class="bk-bscp-icon icon-reduce" @click="handleDeleteData(tableItem, index)"></i>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script lang="ts" setup>
  import { watch, ref } from 'vue';
  import {
    IFiledsItemEditing,
    IFieldItem,
    ILocalTableEditData,
    ILocalTableDataItem,
  } from '../../../../../../types/kv-table';
  import BatchSetPop from './batch-set-pop.vue';
  import { cloneDeep, isEqual } from 'lodash';

  const props = defineProps<{
    fields: IFieldItem[];
    data: ILocalTableDataItem[];
  }>();

  const emits = defineEmits(['change', 'delete']);

  const fieldsList = ref<IFiledsItemEditing[]>([]);
  const tableData = ref<ILocalTableEditData[]>([]);
  const publishData = ref<ILocalTableEditData[]>([]); // 已上线的数据
  // const errorCells = ref<{ row: number; col: number }[]>([]);

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
                text: value,
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
        publishData.value = cloneDeep(tableData.value.filter((item) => item.status === 'REVISE'));
      } else {
        handleAddData(0);
      }
    },
  );

  const handleAddData = (index: number) => {
    const content: { [key: string]: string | string[] } = {};
    fieldsList.value.forEach((item) => {
      content[item.name] = item.default_value || '';
    });
    tableData.value.splice(index + 1, 0, { custom_id: Date.now(), content, status: 'ADD', id: 0 });
    emits('change', tableData.value);
  };

  const handleDeleteData = (data: ILocalTableEditData, index: number) => {
    if (data.status === 'REVISE') {
      data.status = 'DELETE';
    } else if (tableData.value.length > 1) {
      tableData.value.splice(index, 1);
    }
    emits('change', tableData.value);
  };

  const handleConfirmBatchSet = (field: string, val: any) => {
    tableData.value.forEach((item) => {
      item.content[field] = val;
    });
  };

  const getCellCls = (data: ILocalTableEditData, field: IFiledsItemEditing) => {
    // 修改状态
    const oldValue = publishData.value.find((item) => item.id === data.id)?.content[field.name];
    if (oldValue && !isEqual(oldValue, data.content[field.name])) {
      return 'change';
    }
  };
</script>

<style scoped lang="scss">
  .table-wrap {
    position: relative;
    overflow-x: auto;
    max-width: 100%;
  }
  .data-table {
    border-collapse: collapse;
    table-layout: fixed;
    th,
    td {
      border: 1px solid #dddddd;
      text-align: left;
      padding: 8px 16px;
      height: 42px;
      white-space: nowrap;
      position: relative;
    }
    th.left-sticky,
    td.left-sticky {
      position: sticky;
      top: 0;
      z-index: 999;
      background-color: #fff;
      left: 0;
    }

    th.right-sticky,
    td.right-sticky {
      position: sticky;
      top: 0;
      z-index: 999;
      background-color: #fff;
      right: 0;
    }

    th.left-sticky,
    th.right-sticky {
      background-color: #f0f1f5;
    }
    thead {
      width: fit-content;
      background: #f0f1f5;
      th {
        min-width: 200px;
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
      .operation {
        min-width: 120px;
      }
    }
    tbody {
      td {
        position: relative;
        padding: 0;
        :deep(.bk-input) {
          position: absolute;
          top: 0;
          left: 0;
          width: 100%;
          height: 100%;
          .bk-input--text {
            padding-left: 16px;
          }
          &:not(.is-focused) {
            border: none;
          }
          .tag-wrap {
            display: flex;
            align-items: center;
            height: 100%;
            background: #fff;
            padding-right: 16px;
          }
        }
        .enum-select {
          height: 100%;
          display: flex;
          align-items: center;
          width: 100%;
          :deep(.bk-select-trigger) {
            height: 100%;
            width: 100%;
            .bk-select-tag {
              box-sizing: border-box;
              border: none;
              height: 100%;
              font-size: 12px;
              padding-left: 16px;
            }
          }
        }
        :deep(.is-focus .bk-select-tag) {
          border: 1px solid #3a84ff !important;
        }
        &.ADD:not(.primary) {
          :deep(.bk-input--text, .bk-select-tag) {
            background-color: #f2fff4;
          }
        }
        &.DELETE:not(.primary) {
          :deep(.bk-input--text, .bk-select-tag) {
            background-color: #ffeeee;
            text-decoration: line-through;
          }
        }
        &.DELETE,
        .primary {
          :deep(.bk-input--text, .bk-select-tag) {
            text-decoration: line-through;
          }
        }
        &.change {
          :deep(.bk-input--text, .bk-select-tag) {
            background-color: #fff3e1;
          }
        }
      }

      .operation {
        .action-btns {
          display: flex;
          gap: 18px;
          padding-left: 16px;
          width: 38px;
          font-size: 14px;
          color: #979ba5;
          cursor: pointer;
          i:hover {
            color: #3a84ff;
          }
        }
      }
    }
  }
</style>

<style>
  .popover-wrap {
    padding: 0 !important;
  }
</style>
