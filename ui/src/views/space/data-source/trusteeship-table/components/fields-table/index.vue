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
    <tbody>
      <template v-if="filedsList.length">
        <tr
          v-for="(item, index) in filedsList"
          :key="item.id"
          :draggable="!item.primary"
          @dragstart="draggedIndex = index"
          @dragover.prevent
          @drop="handleDrop(index)">
          <td class="fields-name-td">
            <bk-input v-model="item.name">
              <template #prefix>
                <span :class="['drag-icon', { disabled: item.primary }]">
                  <grag-fill v-show="!item.primary" />
                </span>
              </template>
            </bk-input>
          </td>
          <td><bk-input v-model="item.alias"></bk-input></td>
          <td>
            <bk-select
              v-if="item"
              class="type-select"
              auto-focus
              :filterable="false"
              @select="item.column_type = $event">
              <bk-option v-for="type in dataType" :id="type.value" :key="type.value" :name="type.label" />
            </bk-select>
          </td>
          <td>
            <bk-input v-if="item.column_type !== 'enum'" v-model="item.default_value"></bk-input>
            <div v-else class="enum-type">
              <bk-select
                :multiple="item.enumType === 'multiple'"
                multiple-mode="tag"
                class="type-select"
                :no-data-text="$t('请先设置枚举值')"
                :popover-options="{ width: 240 }">
                <bk-option
                  v-for="enumItem in item.enumList"
                  :id="enumItem.value"
                  :key="enumItem.value"
                  :name="enumItem.text" />
              </bk-select>
              <EnumSetPop />
            </div>
          </td>
          <td class="check">
            <input
              :class="['radio-input', { checked: item.primary }]"
              type="radio"
              :checked="item.primary"
              @change="handleChangePrimaryKey(item, index)" />
          </td>
          <td class="check">
            <bk-checkbox v-model="item.nullable"></bk-checkbox>
          </td>
          <td class="check">
            <bk-checkbox v-model="item.unique"></bk-checkbox>
          </td>
          <td class="check">
            <bk-checkbox v-model="item.auto_increment"></bk-checkbox>
          </td>
          <td class="check">
            <bk-checkbox v-model="item.read_only"></bk-checkbox>
          </td>
          <td class="check">
            <i
              :class="['bk-bscp-icon', 'icon-reduce', 'delete-icon', { disabled: item.primary }]"
              @click="handleDelete(item, index)" />
          </td>
        </tr>
      </template>
      <tr v-else class="empty-tr">
        <td colspan="10">
          <bk-exception :title="$t('暂无数据')" :description="$t('请先添加字段')" scene="part" type="empty" />
        </td>
      </tr>
    </tbody>
  </table>
</template>

<script lang="ts" setup>
  import { ref, watch } from 'vue';
  import { GragFill } from 'bkui-vue/lib/icon';
  import { IFiledsItem } from '../../../../../../../types/kv-table';
  import { useI18n } from 'vue-i18n';
  import EnumSetPop from './enum-set-pop.vue';

  const { t } = useI18n();
  withDefaults(
    defineProps<{
      isView?: boolean;
    }>(),
    {
      isView: false,
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

  const filedsList = ref<IFiledsItem[]>([]);

  const draggedIndex = ref();

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

  watch(
    () => filedsList.value,
    () => emits('change', filedsList.value),
    { deep: true },
  );

  const handleChangePrimaryKey = (item: IFiledsItem, index: number) => {
    filedsList.value.forEach((filed) => {
      filed.primary = false;
    });
    filedsList.value.splice(index, 1);
    filedsList.value.unshift(item);

    filedsList.value[0].primary = true;
  };

  const handleDelete = (item: IFiledsItem, index: number) => {
    if (item.primary) return;
    filedsList.value.splice(index, 1);
  };

  // 表格拖拽
  const handleDrop = (dropIndex: number) => {
    // 禁止将行放置到第一行
    if (dropIndex === 0 || draggedIndex.value === null || draggedIndex.value === dropIndex) return;

    // 交换拖拽的行位置
    const draggedItem = filedsList.value[draggedIndex.value];
    filedsList.value.splice(draggedIndex.value, 1); // 移除拖拽项
    filedsList.value.splice(dropIndex, 0, draggedItem); // 插入到目标位置
    draggedIndex.value = null; // 重置拖拽索引
  };

  const addFields = () => {
    filedsList.value.push({
      name: '',
      alias: '',
      column_type: '',
      default_value: '',
      primary: filedsList.value.length === 0,
      nullable: false,
      unique: false,
      auto_increment: false,
      read_only: false,
      isShowSettingEnumPopover: false,
      id: Date.now(),
    });
  };

  defineExpose({
    addFields,
  });
</script>

<style scoped lang="scss">
  .fileds-table {
    border: 1px solid #dcdee5;
    border-collapse: collapse;
    th,
    td {
      border: 1px solid #dcdee5;
      height: 42px;
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
    .fields-name-td {
      .bk-input {
        height: 42px;
      }
      .drag-icon {
        margin-left: 16px;
        line-height: 42px;
        font-size: 14px;
        color: #c4c6cc;
        cursor: pointer;
        &.disabled {
          margin-left: 29px;
        }
      }
    }

    :deep(.bk-input) {
      box-sizing: border-box;
      border: none;
      height: 100%;
      .bk-input--text {
        padding-left: 16px;
      }
      &.is-focused {
        border: 1px solid #3a84ff;
      }
    }
    .check {
      text-align: center;
      .delete-icon {
        font-size: 16px;
        cursor: pointer;
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
  }
  .type-select {
    :deep(.bk-input) {
      height: 42px;
      border: none;
      .bk-input--text {
        padding-left: 16px;
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
