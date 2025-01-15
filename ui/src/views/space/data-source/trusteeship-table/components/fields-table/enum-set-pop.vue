<template>
  <bk-popover
    :width="480"
    placement="bottom-end"
    theme="light"
    trigger="manual"
    :is-show="isShow"
    ext-cls="setting-enum-popover">
    <div class="setting-icon" @click="isShow = true">
      <cog-shape v-bk-tooltips="{ content: $t('设置枚举值') }" />
    </div>
    <template #content>
      <div v-click-outside="closeSettingEnumPopover" class="setting-enum-wrap">
        <div class="title">{{ $t('设置枚举值') }}</div>
        <bk-radio-group v-model="isMultiple" class="enum-radio-group">
          <bk-radio :label="false">{{ $t('单选') }}</bk-radio>
          <bk-radio :label="true">{{ $t('多选') }}</bk-radio>
        </bk-radio-group>
        <div class="enum-list">
          <div v-for="(enumItem, enumIndex) in settingEnumList" :key="enumIndex" class="enum-item">
            <div class="num">{{ enumIndex + 1 }}</div>
            <bk-input
              v-model="enumItem.label"
              :class="{ hasError: enumItem.hasTextError }"
              :placeholder="$t('显示文本')"
              @input="enumItem.hasTextError = false" />
            <bk-input
              v-model="enumItem.value"
              :class="{ hasError: enumItem.hasValueError }"
              :placeholder="$t('实际值')"
              @input="enumItem.hasValueError = false" />
            <div class="action-btns">
              <i
                :class="[
                  'bk-bscp-icon',
                  'icon-minus-circle-shape',
                  { disabled: hasTableData && existEnumList.find((item) => item.id === enumItem.id) },
                ]"
                @click.stop="handleDelEnumItem(enumItem, enumIndex)"></i>
              <i class="bk-bscp-icon icon-plus-circle-shape" @click="handleAddEnumItem(enumIndex)"></i>
            </div>
          </div>
        </div>
        <div class="footer">
          <bk-button theme="primary" @click="handleConfirmSettingEnum">{{ $t('保存') }}</bk-button>
          <bk-button @click="closeSettingEnumPopover">{{ $t('取消') }}</bk-button>
        </div>
      </div>
    </template>
  </bk-popover>
</template>

<script lang="ts" setup>
  import { ref, watch } from 'vue';
  import { CogShape } from 'bkui-vue/lib/icon';
  import { IEnumItem } from '../../../../../../../types/kv-table';

  const props = defineProps<{
    isMultiple: boolean; // 是否多选
    hasTableData?: boolean; // 表结构是否已有数据
    enumList?: IEnumItem[];
  }>();

  const emits = defineEmits(['change']);

  const isMultiple = ref(false);
  const settingEnumList = ref<IEnumItem[]>([{ label: '', value: '', hasTextError: false, hasValueError: false }]);
  const existEnumList = ref<IEnumItem[]>([]);
  const isShow = ref(false);

  watch(
    () => isShow.value,
    () => {
      isMultiple.value = props.isMultiple;
      if (props.enumList?.length) {
        settingEnumList.value = props.enumList.map((item, index) => {
          return {
            ...item,
            id: index,
          };
        });
        existEnumList.value = props.enumList.map((item, index) => {
          return {
            ...item,
            id: index,
          };
        });
      }
    },
    { immediate: true },
  );

  const handleAddEnumItem = (index: number) => {
    settingEnumList.value.splice(index + 1, 0, { label: '', value: '', hasTextError: false, hasValueError: false });
  };

  const handleDelEnumItem = (enumItem: IEnumItem, index: number) => {
    if (existEnumList.value.find((item) => item.id === enumItem.id)) return;
    if (settingEnumList.value.length > 1) {
      settingEnumList.value.splice(index, 1);
    }
  };

  const validateEnumSetting = () => {
    settingEnumList.value.forEach((enumItem) => {
      enumItem.hasTextError = !enumItem.label;
      enumItem.hasValueError = !enumItem.value;
    });
    return !settingEnumList.value.some((item) => item.hasTextError || item.hasValueError);
  };

  const handleConfirmSettingEnum = () => {
    const isValid = validateEnumSetting();
    if (!isValid) return;
    const enumList = settingEnumList.value.map((item) => {
      return { label: item.label, value: item.value };
    });
    emits('change', [enumList, isMultiple.value]);
    isShow.value = false;
  };

  const closeSettingEnumPopover = () => {
    isShow.value = false;
    settingEnumList.value = [{ label: '', value: '', hasTextError: false, hasValueError: false }];
    isMultiple.value = false;
  };
</script>

<style scoped lang="scss">
  .setting-icon {
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-left: 1px solid #dcdee5;
    width: 30px;
    font-size: 16px;
    color: #979ba5;
    cursor: pointer;
    &:hover {
      color: #3a84ff;
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
          color: #c4c6cc;
          cursor: pointer;
          i {
            &:hover {
              color: #3a84ff;
            }
            &.disabled {
              color: #eaebf0;
            }
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
</style>

<style lang="scss">
  .bk-popover.bk-pop2-content.setting-enum-popover {
    padding-bottom: 54px;
  }
</style>
