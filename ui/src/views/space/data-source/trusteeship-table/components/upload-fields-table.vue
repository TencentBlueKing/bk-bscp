<template>
  <bk-table
    :data="filedsList"
    class="fields-setting-table"
    :border="['row', 'col', 'outer']"
    row-hover="auto"
    :cell-class="getCellCls"
    :row-draggable="{ width: 30, label: '' }"
    :show-overflow-tooltip="true">
    <bk-table-column :label="(() => h('span', { class: 'required' }, $t('显示名')))" property="showName" :width="144">
      <template #default="{ row }">
        <div>{{ row.showName }}</div>
      </template>
    </bk-table-column>
    <bk-table-column
      :label="(() => h('span', { class: 'required' }, $t('字段名')))"
      :width="120"
      :show-overflow-tooltip="{ disabled: true }">
      <template #default="{ row }">
        <bk-input v-model="row.fieldsName"></bk-input>
      </template>
    </bk-table-column>
    <bk-table-column :label="(() => h('span', { class: 'required' }, $t('数据类型')))" :width="126">
      <template #default="{ row }">
        <bk-select v-if="row" class="type-select" auto-focus :filterable="false" @select="row.type = $event">
          <bk-option v-for="item in dataType" :id="item.value" :key="item.value" :name="item.label" />
        </bk-select>
      </template>
    </bk-table-column>
    <bk-table-column :label="$t('默认值/枚举值')" :width="183">
      <template #default="{ row }">
        <bk-input v-if="row.type !== 'enum'" v-model="row.default"></bk-input>
        <div v-else class="enum-type">
          <bk-select
            :multiple="row.enumType === 'multiple'"
            multiple-mode="tag"
            class="type-select"
            :no-data-text="$t('请先设置枚举值')"
            :popover-options="{ width: 240 }">
            <bk-option v-for="item in row.enumList" :id="item.value" :key="item.value" :name="item.text" />
          </bk-select>
          <bk-popover
            :width="480"
            placement="bottom-end"
            theme="light"
            trigger="manual"
            :is-show="popoverShow"
            ext-cls="setting-enum-popover">
            <div class="setting-icon" @click="popoverShow = true">
              <cog-shape v-bk-tooltips="{ content: $t('设置枚举值') }" />
            </div>
            <template #content>
              <div v-click-outside="closeSettingEnumPopover" class="setting-enum-wrap">
                <div class="title">{{ $t('设置枚举值') }}</div>
                <bk-radio-group v-model="settingEnumType" class="enum-radio-group">
                  <bk-radio label="single">{{ $t('单选') }}</bk-radio>
                  <bk-radio label="multiple">{{ $t('多选') }}</bk-radio>
                </bk-radio-group>
                <div class="enum-list">
                  <div v-for="(enumItem, index) in settingEnumList" :key="index" class="enum-item">
                    <div class="num">{{ index + 1 }}</div>
                    <bk-input
                      v-model="enumItem.text"
                      :class="{ hasError: enumItem.hasTextError }"
                      :placeholder="$t('显示文本')"
                      @input="enumItem.hasTextError = false" />
                    <bk-input
                      v-model="enumItem.value"
                      :class="{ hasError: enumItem.hasValueError }"
                      :placeholder="$t('实际值')"
                      @input="enumItem.hasValueError = false" />
                    <div class="action-btns">
                      <i class="bk-bscp-icon icon-reduce" @click="handleDelEnumItem(index)"></i>
                      <i class="bk-bscp-icon icon-add" @click="handleAddEnumItem(index)"></i>
                    </div>
                  </div>
                </div>
                <div class="footer">
                  <bk-button theme="primary" @click="handleConfirmSettingEnum(row)">{{ $t('保存') }}</bk-button>
                  <bk-button @click="closeSettingEnumPopover">{{ $t('取消') }}</bk-button>
                </div>
              </div>
            </template>
          </bk-popover>
        </div>
      </template>
    </bk-table-column>
    <bk-table-column :label="(() => h('span', { class: 'required' }, $t('主键')))" property="primaryKey" :width="57">
      <template #default="{ row, index }">
        <input
          :class="['radio-input', { checked: row.primaryKey }]"
          type="radio"
          :checked="row.primaryKey"
          @change="handleChangePrimaryKey(row, index)" />
      </template>
    </bk-table-column>
    <bk-table-column :label="$t('非空')" property="nonempty" :width="57">
      <template #default="{ row }">
        <bk-checkbox v-model="row.nonEmpty"></bk-checkbox>
      </template>
    </bk-table-column>
    <bk-table-column :label="$t('唯一')" property="only" :width="57">
      <template #default="{ row }">
        <bk-checkbox v-model="row.only"></bk-checkbox>
      </template>
    </bk-table-column>
    <bk-table-column :label="$t('自增')" property="autoIncrement" :width="57">
      <template #default="{ row }">
        <bk-checkbox v-model="row.autoIncrement"></bk-checkbox>
      </template>
    </bk-table-column>
    <bk-table-column :label="$t('只读')" property="readonly" :width="57">
      <template #default="{ row }">
        <bk-checkbox v-model="row.readonly"></bk-checkbox>
      </template>
    </bk-table-column>
  </bk-table>
</template>

<script lang="ts" setup>
  import { ref, watch, h } from 'vue';
  import { CogShape } from 'bkui-vue/lib/icon';
  import { IFiledsItem, IEnumItem } from '../../../../../../types/kv-table';
  import { cloneDeep } from 'lodash';

  const props = withDefaults(defineProps<{
    list: IFiledsItem[];
    isView?: boolean;
  }>(), {
    isView: false,
  });

  const filedsList = ref<IFiledsItem[]>([]);
  const settingEnumType = ref('single');
  const settingEnumList = ref<IEnumItem[]>([{ text: '', value: '', hasTextError: false, hasValueError: false }]);
  const popoverShow = ref(false);

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
    () => props.list,
    (val) => {
      filedsList.value = cloneDeep(val);
    },
    { immediate: true, deep: true },
  );

  const handleChangePrimaryKey = (row: IFiledsItem, index: number) => {
    filedsList.value.splice(index, 1);
    filedsList.value.unshift(row);
    filedsList.value.forEach((filed) => {
      filed.primary = false;
    });
    filedsList.value[0].primary = true;
  };

  // 添加自定义单元格class
  const getCellCls = ({ property }: { property: string }) => {
    if (property === 'showName') return 'show-name-cell';
    return ['primaryKey', 'nonempty', 'only', 'autoIncrement', 'readonly'].includes(property)
      ? 'check-cell'
      : '';
  };

  const handleAddEnumItem = (index: number) => {
    settingEnumList.value.splice(index + 1, 0, { text: '', value: '', hasTextError: false, hasValueError: false });
  };

  const handleDelEnumItem = (index: number) => {
    if (settingEnumList.value.length > 1) {
      settingEnumList.value.splice(index, 1);
    }
  };

  const validateEnumSetting = () => {
    settingEnumList.value.forEach((enumItem) => {
      enumItem.hasTextError = !enumItem.text;
      enumItem.hasValueError = !enumItem.value;
    });
    return !settingEnumList.value.some((item) => item.hasTextError || item.hasValueError);
  };

  const handleConfirmSettingEnum = (filedsItem: IFiledsItem) => {
    const isValid = validateEnumSetting();
    if (!isValid) return;
    filedsItem.enumList = settingEnumList.value;
    filedsItem.enumType = settingEnumType.value;
    closeSettingEnumPopover();
  };

  const closeSettingEnumPopover = () => {
    popoverShow.value = false;
    settingEnumList.value = [{ text: '', value: '', hasTextError: false, hasValueError: false }];
    settingEnumType.value = 'single';
  };
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
  .type-select {
    :deep(.bk-input) {
      height: 41px;
      border: none;
      .bk-input--text {
        padding-left: 16px;
      }
    }
  }
  .fields-setting-table {
    width: 100% !important;
    :deep(.bk-table-head) {
      colgroup col {
        min-width: 30px !important;
      }
      .head-text {
        overflow: initial;
        .required {
          position: relative;
          &::before {
            position: absolute;
            left: -8px;
            content: '*';
            color: red;
            z-index: 999;
          }
        }
      }
    }
    :deep(.bk-table-body) {
      colgroup col {
        min-width: 30px !important;
      }
      .cell {
        padding: 0;
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
      .show-name-cell {
        background: #F5F7FA;
        padding: 0 8px;
      }
      .enum-type {
        display: flex;
        align-items: center;
        height: 100%;
        .bk-select .bk-select-trigger .bk-select-tag {
          border: none;
          box-shadow: none;
          height: 100%;
          width: 156px;
        }
        .setting-icon {
          height: 24px;
          display: flex;
          align-items: center;
          justify-content: center;
          border-left: 1px solid #dcdee5;
          width: 30px;
          font-size: 16px;
          color: #a5a8b1;
          cursor: pointer;
          &:hover {
            color: #3a84ff;
          }
        }
      }
    }
    .bk-input {
      border: none;
      height: 100%;
      &.is-focused {
        border: 1px solid #3a84ff;
      }
    }
  }

  .setting-enum-wrap {
    .title {
      font-size: 16px;
      color: #313238;
    }
    .enum-radio-group {
      margin: 12px 8px;
    }
    .enum-list {
      padding: 12px 8px;
      max-height: 300px;
      overflow: auto;
      .enum-item {
        display: flex;
        gap: 12px;
        &:not(:last-child) {
          margin-bottom: 8px;
        }
        .num {
          width: 32px;
          height: 32px;
          background: #f0f1f5;
          border-radius: 2px;
          line-height: 32px;
          text-align: center;
        }
        .bk-input {
          width: 160px;
          height: 32px;
          &.hasError {
            border-color: #ea3636;
          }
        }
        .action-btns {
          padding: 0 8px;
          display: flex;
          align-items: center;
          gap: 8px;
          font-size: 14px;
          color: #979ba5;
          cursor: pointer;
          i:hover {
            color: #3a84ff;
          }
        }
      }
    }
    .footer {
      position: absolute;
      bottom: 0;
      left: 0;
      padding: 0 16px;
      width: 100%;
      height: 42px;
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: 8px;
      background: #fafbfd;
      box-shadow: 0 -1px 0 0 #dcdee5;
      .bk-button {
        width: 64px;
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
</style>

<style lang="scss">
  .bk-popover.bk-pop2-content.setting-enum-popover {
    padding-bottom: 54px;
  }
</style>
