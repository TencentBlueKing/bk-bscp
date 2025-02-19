<template>
  <div class="table-structure-form">
    <Card :title="$t('字段设置')">
      <template v-if="isManualCreate" #suffix>
        <div class="add-fields" @click="handleAddFields">
          <Plus class="add-icon" />
          <span class="text">{{ $t('添加字段') }}</span>
        </div>
      </template>
      <FieldsTable
        ref="tableRef"
        :is-edit="props.isEdit"
        :has-table-data="hasTableData"
        :list="fieldsColumns"
        @change="handleFieldsChange" />
    </Card>
  </div>
</template>

<script lang="ts" setup>
  import { ref, watch, nextTick } from 'vue';
  import { IFieldsItemEditing, IFieldItem } from '../../../../../../types/kv-table';
  import { Plus } from 'bkui-vue/lib/icon';
  import Card from '../../component/card.vue';
  import FieldsTable from './../components/fields-table/manual.vue';

  const props = defineProps<{
    bkBizId: string;
    isManualCreate: boolean;
    isEdit: boolean;
    columns: IFieldItem[];
    hasTableData?: boolean;
  }>();

  const emits = defineEmits(['change']);

  const fieldsColumns = ref<IFieldsItemEditing[]>([]);
  const tableRef = ref();
  const isUpdate = ref(false);

  watch(
    () => props.columns,
    () => {
      if (isUpdate.value) return;
      translateFormData();
    },
  );

  const handleAddFields = () => {
    fieldsColumns.value.push({
      name: '',
      alias: '',
      column_type: fieldsColumns.value.length === 0 ? 'number' : 'string',
      default_value: '',
      primary: fieldsColumns.value.length === 0,
      not_null: fieldsColumns.value.length === 0,
      unique: fieldsColumns.value.length === 0,
      auto_increment: false,
      read_only: false,
      id: Date.now(),
      enum_value: [], // 枚举值设置内容
      selected: false, // 枚举值是否多选
    });
    handleFormChange();
  };

  const handleFieldsChange = (val: IFieldsItemEditing[]) => {
    fieldsColumns.value = val;
    handleFormChange();
  };

  // 接口数据转表单数据
  const translateFormData = () => {
    fieldsColumns.value = props.columns.map((item, index) => {
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
  };

  // 表单数据转接口数据
  const handleFormChange = () => {
    const columns = fieldsColumns.value.map((item) => {
      let default_value;
      if (item.column_type === 'enum' && item.selected && item.default_value) {
        default_value = JSON.stringify(item.default_value);
      } else {
        default_value = String(item.default_value);
        if (item.default_value === null) {
          default_value = '';
        }
      }
      let enum_value;
      if (item.column_type === 'enum' && Array.isArray(item.enum_value)) {
        enum_value = JSON.stringify(item.enum_value);
      } else {
        enum_value = '';
      }
      return {
        default_value,
        enum_value, // 枚举值设置内容
        name: item.name,
        alias: item.alias,
        primary: item.primary,
        column_type: item.column_type,
        not_null: item.not_null,
        unique: item.unique,
        read_only: item.read_only,
        auto_increment: item.auto_increment,
        selected: item.selected,
      };
    });
    isUpdate.value = true;
    nextTick(() => {
      emits('change', columns);
      isUpdate.value = false;
    });
    emits('change', columns);
  };

  defineExpose({
    validate: async () => {
      return tableRef.value.validate();
    },
  });
</script>

<style scoped lang="scss">
  .add-fields {
    display: flex;
    align-items: center;
    height: 16px;
    cursor: pointer;
    .add-icon {
      border-radius: 50%;
      background-color: #3a84ff;
      color: #fff;
      margin-right: 5px;
    }
    .text {
      color: #3a84ff;
      font-size: 12px;
    }
  }

  .table-structure-form {
    .card:not(:last-child) {
      margin-bottom: 16px;
    }
  }

  .exception-wrap-item {
    :deep(.bk-exception-img) {
      width: 280px;
      height: 140px;
    }
    :deep(.bk-exception-title) {
      margin-top: 8px;
      font-size: 14px;
      color: #63656e;
      line-height: 22px;
    }
    :deep(.bk-exception-description) {
      margin-top: 8px;
      font-size: 12px;
      color: #979ba5;
      line-height: 20px;
    }
  }
</style>
