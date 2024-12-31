<template>
  <bk-table
    :data="filedsList"
    class="fields-setting-table"
    :border="['row', 'col', 'outer']"
    row-hover="auto"
    :cell-class="getCellCls"
    :show-overflow-tooltip="true">
    <bk-table-column :label="$t('显示名')" prop="alias" :width="183" />
    <bk-table-column :label="$t('字段名')" prop="name" :width="156" :show-overflow-tooltip="{ disabled: true }" />
    <bk-table-column :label="$t('数据类型')" prop="column_type" :width="136" />
    <bk-table-column :label="$t('默认值/枚举值')" :width="198">
      <template #default="{ row }">
        <!-- <span v-if="row.type !== 'enum'">{{ row.default_value }}</span>
        <div v-else class="enum-type">
          <bk-tag v-for="item in row.de" :key="item.value">{{ item.text }}</bk-tag>
        </div> -->
        <div v-if="Array.isArray(row.default_value)" class="tag-list">
          <bk-tag v-for="tag in row.default_value" :key="tag" radius="4px">
            {{ tag }}
          </bk-tag>
        </div>
        <div v-else>{{ row.default_value }}</div>
      </template>
    </bk-table-column>
    <bk-table-column :label="$t('主键')" property="primaryKey" :width="57">
      <template #default="{ row }">
        <input
          :class="['radio-input', 'disabled', { checked: row.primaryKey }]"
          type="radio"
          :checked="row.primary"
          disabled />
      </template>
    </bk-table-column>
    <bk-table-column :label="$t('非空')" property="nonempty" :width="57">
      <template #default="{ row }">
        <bk-checkbox v-model="row.not_null" disabled></bk-checkbox>
      </template>
    </bk-table-column>
    <bk-table-column :label="$t('唯一')" property="only" :width="57">
      <template #default="{ row }">
        <bk-checkbox v-model="row.unique" disabled></bk-checkbox>
      </template>
    </bk-table-column>
    <bk-table-column :label="$t('自增')" property="autoIncrement" :width="57">
      <template #default="{ row }">
        <bk-checkbox v-model="row.auto_increment" disabled></bk-checkbox>
      </template>
    </bk-table-column>
    <bk-table-column :label="$t('只读')" property="readonly" :width="57">
      <template #default="{ row }">
        <bk-checkbox v-model="row.read_only" disabled></bk-checkbox>
      </template>
    </bk-table-column>
  </bk-table>
</template>

<script lang="ts" setup>
  import { IFiledsItemEditing } from '../../../../../.././../types/kv-table';

  defineProps<{
    filedsList: IFiledsItemEditing[];
  }>();
  // 添加自定义单元格class
  const getCellCls = ({ property }: { property: string }) => {
    return ['primaryKey', 'nonempty', 'only', 'autoIncrement', 'readonly', 'delete'].includes(property)
      ? 'check-cell'
      : '';
  };
</script>

<style scoped lang="scss">
  .fields-setting-table {
    :deep(.bk-table-head) {
      colgroup col {
        min-width: 30px !important;
      }
    }
    :deep(.bk-table-body) {
      overflow: hidden;
      colgroup col {
        min-width: 30px !important;
      }
      .enum-type {
        display: flex;
        gap: 4px;
        align-items: center;
        height: 100%;
      }
      .check-cell {
        .cell {
          width: 100%;
          display: flex;
          align-items: center;
          justify-content: space-around;
        }
        .delete-icon {
          font-size: 16px;
          cursor: pointer;
          &:hover {
            color: #3a84ff;
          }
        }
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
    &.disabled {
      cursor: not-allowed;
      padding: 3px;
      color: #a3c5fd;
      background-color: #a3c5fd;
      border-color: #a3c5fd;
    }
  }
</style>
