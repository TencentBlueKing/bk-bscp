<template>
  <table class="fileds-table">
    <thead>
      <tr>
        <th v-for="(item, index) in theadList" :key="index" :style="{ width: item.width + 'px' }" :class="item.class">
          <span v-bk-tooltips="{ content: item.tips, disabled: !item.tips }">{{ item.label }}</span>
        </th>
        <th :style="{ width: '50px' }"></th>
      </tr>
    </thead>
    <template v-if="fieldsList.length">
      <draggable
        v-model="fieldsList"
        tag="tbody"
        item-key="id"
        ghost-class="ghost"
        handle=".drag-icon"
        :animation="500"
        :move="handleDrag"
        @end="emits('change', fieldsList)">
        <template #item="{ element, index }">
          <tr>
            <td :class="getCellCLs(index, 'name')">
              <bk-input
                v-model="element.name"
                @change="emits('change', fieldsList)"
                @blur="validateField(index, 'name')">
                <template #prefix>
                  <span :class="['drag-icon', { disabled: element.primary }]">
                    <grag-fill v-show="!element.primary" />
                  </span>
                </template>
              </bk-input>
            </td>
            <td :class="getCellCLs(index, 'alias')">
              <bk-input
                v-model="element.alias"
                @change="emits('change', fieldsList)"
                @blur="validateField(index, 'alias')" />
            </td>
            <td :class="getCellCLs(index, 'column_type')">
              <bk-select
                v-if="element"
                v-model="element.column_type"
                class="type-select"
                auto-focus
                :filterable="false"
                :clearable="false"
                :disabled="element.primary"
                @change="handleSelectType(element, $event)"
                @blur="validateField(index, 'column_type')">
                <bk-option v-for="type in dataType" :id="type.value" :key="type.value" :name="type.label" />
              </bk-select>
            </td>
            <td class="edit-cell">
              <bk-input
                v-if="element.column_type !== 'enum'"
                v-model="element.default_value"
                :type="element.column_type === 'number' ? 'number' : 'text'"
                @change="emits('change', fieldsList)" />
              <div v-else class="enum-type">
                <bk-select
                  v-model="element.default_value"
                  :multiple="element.selected"
                  multiple-mode="tag"
                  class="type-select"
                  :no-data-text="$t('请先设置枚举值')"
                  :popover-options="{ width: 240 }"
                  :clearable="element.selected"
                  @change="emits('change', fieldsList)">
                  <bk-option
                    v-for="(enumItem, i) in element.enum_value"
                    :id="enumItem.value"
                    :key="i"
                    :name="enumItem.label" />
                </bk-select>
                <EnumSetPop
                  :has-table-data="hasTableData"
                  :is-multiple="element.selected"
                  :enum-list="element.enum_value"
                  @change="handleSetEnum(element, $event)" />
              </div>
            </td>
            <td class="check">
              <input
                :class="['radio-input', { checked: element.primary }]"
                type="radio"
                :checked="element.primary"
                @change="handleChangePrimaryKey(element, index)" />
            </td>
            <td class="check">
              <bk-checkbox v-model="element.not_null" @change="emits('change', fieldsList)"></bk-checkbox>
            </td>
            <td class="check">
              <bk-checkbox
                v-model="element.unique"
                :disabled="element.primary"
                @change="emits('change', fieldsList)"></bk-checkbox>
            </td>
            <td class="check">
              <bk-checkbox v-model="element.auto_increment" @change="emits('change', fieldsList)"></bk-checkbox>
            </td>
            <td class="check">
              <bk-checkbox v-model="element.read_only" @change="emits('change', fieldsList)"></bk-checkbox>
            </td>
            <td class="check">
              <i
                :class="['bk-bscp-icon', 'icon-minus-circle-shape', 'delete-icon', { disabled: element.primary }]"
                @click="handleDelete(element, index)" />
            </td>
          </tr>
        </template>
      </draggable>
    </template>
    <tr v-else class="empty-tr">
      <td colspan="10">
        <bk-exception :title="$t('暂无数据')" :description="$t('请先添加字段')" scene="part" type="empty" />
      </td>
    </tr>
  </table>
  <DeleteFieldDialog
    v-model:show="isShowDeleteDialog"
    :field-name="deleteField?.alias"
    @confirm="handleDeleteConfirm" />
</template>

<script lang="ts" setup>
  import draggable from 'vuedraggable';
  import { ref, watch, computed } from 'vue';
  import { GragFill } from 'bkui-vue/lib/icon';
  import { IFiledsItemEditing, IEnumItem } from '../../../../../../../types/kv-table';
  import { useI18n } from 'vue-i18n';
  import EnumSetPop from './enum-set-pop.vue';
  import DeleteFieldDialog from './delete-field-dialog.vue';

  const { t } = useI18n();
  const props = withDefaults(
    defineProps<{
      list: IFiledsItemEditing[];
      isEdit?: boolean; // 是否为编辑态
      hasTableData?: boolean; // 是否已有表格数据
    }>(),
    {
      isEdit: false,
    },
  );

  const emits = defineEmits(['change']);

  const theadList = [
    {
      label: t('字段名'),
      class: 'required describe drag',
      width: '170',
      tips: t('字段名包含字母、数字、下划线 ( _ ) 和美元符号 ( $ )，长度不超过 64 字符'),
    },
    { label: t('显示名'), class: 'required', width: '144', tips: '' },
    { label: t('数据类型'), class: 'required', width: '126', tips: '' },
    { label: t('默认值/枚举值'), class: 'describe', width: '183', tips: t('可设置字段默认值；ENUM 类型请设置枚举值') },
    { label: t('主键'), class: 'required', width: '56', tips: '' },
    { label: t('非空'), class: 'check-th', width: '56', tips: '' },
    { label: t('唯一'), class: 'check-th', width: '56', tips: '' },
    { label: t('自增'), class: 'check-th', width: '56', tips: '' },
    { label: t('只读'), class: 'check-th', width: '56', tips: '' },
  ];
  const fieldsList = ref<IFiledsItemEditing[]>([]);
  const dataType = [
    {
      value: 'string',
      label: 'String',
    },
    {
      value: 'number',
      label: 'Number',
    },
    {
      value: 'enum',
      label: 'ENUM',
    },
  ];
  const deleteField = ref<IFiledsItemEditing>();
  const deleteFieldIndex = ref(0);
  const isShowDeleteDialog = ref(false);
  const errors = ref<any[]>([]);

  watch(
    () => props.list,
    () => {
      fieldsList.value = [...props.list];
    },
    { deep: true },
  );

  const hasErrors = computed(() => {
    return errors.value.some((error) => Object.values(error).some((isError) => isError));
  });

  // 选择主键
  const handleChangePrimaryKey = (item: IFiledsItemEditing, index: number) => {
    // 重置主键状态
    fieldsList.value.forEach((filed) => {
      filed.primary = false;
    });

    // 标记新的主键
    item.primary = true;
    item.unique = true;
    item.column_type = 'number';

    // 将选中的项移到第一个位置
    fieldsList.value = [item, ...fieldsList.value.filter((_, idx) => idx !== index)];
    errors.value = [];
    emits('change', fieldsList.value);
  };

  const handleDelete = (item: IFiledsItemEditing, index: number) => {
    if (item.primary) return;
    deleteField.value = item;
    deleteFieldIndex.value = index;
    if (props.hasTableData) {
      isShowDeleteDialog.value = true;
    } else {
      handleDeleteConfirm();
    }
  };

  const handleDeleteConfirm = () => {
    isShowDeleteDialog.value = false;
    fieldsList.value.splice(deleteFieldIndex.value, 1);
    // 删除主键后 选择第一个字段为主键
    if (deleteField.value!.primary) {
      fieldsList.value[0].primary = true;
      fieldsList.value[0].unique = true;
    }
    emits('change', fieldsList.value);
  };

  // 表格拖拽
  const handleDrag = (event: any) => {
    const { relatedContext, draggedContext } = event;
    // 禁止将其他行插入到第一行
    if (relatedContext.index === 0) {
      return false;
    }

    if (draggedContext.index === 0) {
      return false;
    }

    return true;
  };

  // 设置枚举值
  const handleSetEnum = (
    filed: IFiledsItemEditing,
    [enumList, isMultiple]: [enumList: IEnumItem[], isMultiple: boolean],
  ) => {
    Object.assign(filed, {
      enumList,
      selected: isMultiple,
      enum_value: enumList,
    });
    emits('change', fieldsList.value);
  };

  const handleSelectType = (filed: IFiledsItemEditing, type: string) => {
    if (type === 'enum') {
      filed.default_value = [];
    }
    emits('change', fieldsList.value);
  };

  // 校验单个字段
  const validateField = (rowIndex: number, field: string) => {
    const value = fieldsList.value[rowIndex][field as keyof IFiledsItemEditing];
    const error = !value;
    if (!errors.value[rowIndex]) errors.value[rowIndex] = {};
    if (error) {
      errors.value[rowIndex][field] = error;
    } else {
      delete errors.value[rowIndex][field];
    }
  };

  const validateAllFields = () => {
    errors.value = fieldsList.value.map((row) => {
      const rowErrors: Record<string, boolean> = {};
      // 只校验 name 和 alias 字段
      ['name', 'alias', 'column_type'].forEach((field) => {
        const value = row[field as keyof IFiledsItemEditing];
        rowErrors[field] = !value; // 如果字段为空，标记为错误
      });

      return rowErrors;
    });
    return !hasErrors.value;
  };

  const getCellCLs = (index: number, field: string) => {
    const error = errors.value[index]?.[field] ?? false;
    const cls = ['edit-cell', { error: errors.value[index]?.[field] ?? false }];
    if (field === 'name') {
      cls.push('fields-name-td');
      return ['fields-name-td', 'edit-cell', { error }];
    }
    if (field === 'column_type') {
      return { error };
    }
    return ['edit-cell', { error }];
  };

  defineExpose({
    validate: validateAllFields,
  });
</script>

<style scoped lang="scss">
  .fileds-table {
    border: 1px solid #dcdee5;
    border-collapse: collapse;
    th,
    td {
      position: relative;
      border: 1px solid #dcdee5;
      height: 42px;
      &.error {
        :deep(.bk-input) {
          border: 1px solid red !important;
        }
      }
    }
    th {
      font-size: 12px;
      color: #313238;
      text-align: left;
      padding-left: 16px;
      font-weight: 400;
      &.required span {
        position: relative;
        &::before {
          position: absolute;
          content: '*';
          color: #ea3636;
          left: -10px;
        }
      }
      &.describe span {
        text-decoration: underline;
        text-decoration-style: dashed;
        cursor: pointer;
      }
      &.drag {
        padding-left: 46px;
      }
      &.check-th {
        text-align: center;
        padding-left: 0;
      }
    }
    td.edit-cell {
      position: relative;
      :deep(.bk-input) {
        position: absolute;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        box-sizing: border-box;
        border: none;
        &.is-focused {
          border: 1px solid #3a84ff;
        }
        .bk-input--text {
          padding-left: 16px;
        }
      }
    }
  }
  .fields-name-td {
    height: 42px;
    .drag-icon {
      display: flex;
      align-items: center;
      margin-left: 16px;
      font-size: 14px;
      color: #c4c6cc;
      cursor: pointer;
      &.disabled {
        margin-left: 29px;
      }
    }
  }

  .check {
    text-align: center;
    .delete-icon {
      font-size: 16px;
      cursor: pointer;
      color: #c4c6cc;
      &:hover {
        color: #3a84ff;
      }
      &.disabled {
        color: #eaebf0;
        cursor: not-allowed;
      }
    }
  }
  .enum-type {
    display: flex;
    align-items: center;
    height: 100%;
    :deep(.bk-select .bk-select-trigger .bk-select-tag) {
      border: none !important;
      box-shadow: none !important;
      height: 100%;
      width: 150px;
      font-size: 12px;
      padding-left: 16px;
    }
  }
  .type-select {
    :deep(.bk-input) {
      height: 42px;
      border: none;
      .bk-input--text {
        padding-left: 16px;
      }
      &.is-focused {
        border: 1px solid #3a84ff;
      }
    }
  }

  .radio-input {
    position: relative;
    display: inline-block;
    width: 16px;
    height: 16px;
    color: #fff;
    vertical-align: middle;
    cursor: pointer;
    background-color: #fff;
    border: 1px solid #c4c6cc;
    border-radius: 50%;
    outline: none;
    visibility: visible;
    transition: all 0.3s;
    background-clip: content-box;
    -webkit-appearance: none;
    -moz-appearance: none;
    appearance: none;
    &.checked {
      padding: 3px;
      color: #3a84ff;
      background-color: #3a84ff;
      border-color: #3a84ff;
    }
  }

  .empty-tr {
    :deep(.bk-exception) {
      width: 100%;
      margin-bottom: 16px;
    }
  }
</style>

<style lang="scss">
  .bk-popover.bk-pop2-content.setting-enum-popover {
    padding-bottom: 54px;
  }
</style>
