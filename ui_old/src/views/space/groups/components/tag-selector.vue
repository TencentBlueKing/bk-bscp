<template>
  <div class="rule-config">
    <div class="key-input">
      <bk-popover
        :is-show="isShowKeyPopover"
        ref="popoverRef"
        theme="light"
        trigger="manual"
        ext-cls="group-selector-popover"
        placement="bottom">
        <bk-input
          v-model="rule.key"
          ref="keyInputRef"
          :class="[{ 'is-error': showKeyError }, 'key-input']"
          :placeholder="$t('请输入或选择key')"
          @change="handleKeyChange"
          @click="isShowKeyPopover = true">
          <template #suffix>
            <angle-down :class="['suffix-icon', { 'show-popover': isShowKeyPopover }]" />
          </template>
        </bk-input>
        <template #content>
          <div class="selector-list" v-click-outside="() => (isShowKeyPopover = false)">
            <div v-for="item in BuiltInTag" :key="item" class="selector-item" @click="handleSelectKey(item)">
              {{ item }}
            </div>
          </div>
        </template>
      </bk-popover>
      <div v-show="showKeyError" class="error-msg is--key">
        {{ $t("仅支持字母，数字，'-'，'_'，'.' 及 '/' 且需以字母数字开头和结尾") }}
      </div>
    </div>
    <bk-select :model-value="rule.op" style="width: 82px" :clearable="false" @change="handleLogicChange">
      <bk-option v-for="op in GROUP_RULE_OPS" :key="op.id" :value="op.id" :label="op.name"></bk-option>
    </bk-select>
    <div class="value-input">
      <bk-popover
        :is-show="isShowValuePopover && valueList.length > 0"
        theme="light"
        trigger="manual"
        ext-cls="group-selector-popover"
        placement="bottom-start">
        <bk-tag-input
          v-if="['in', 'nin'].includes(rule.op)"
          v-model="rule.value"
          :class="{ 'is-error': showValueError }"
          :allow-create="true"
          :collapse-tags="true"
          :has-delete-icon="true"
          :show-clear-only-hover="true"
          :allow-auto-match="true"
          :list="[]"
          placeholder="value"
          @click="isShowValuePopover = true"
          @change="handleValueChange">
          <template #suffix>
            <angle-down :class="['suffix-icon', { 'show-popover': isShowValuePopover && valueList.length > 0 }]" />
          </template>
        </bk-tag-input>
        <bk-input
          v-else
          v-model="rule.value"
          :loading="true"
          placeholder="value"
          :class="{ 'is-error': showValueError }"
          :type="['gt', 'ge', 'lt', 'le'].includes(rule.op) ? 'number' : 'text'"
          @click="isShowValuePopover = true"
          @change="handleValueChange">
          <template #suffix>
            <angle-down :class="['suffix-icon', { 'show-popover': isShowValuePopover && valueList.length > 0 }]" />
          </template>
        </bk-input>
        <template #content>
          <div class="value-selector-list" v-click-outside="() => (isShowValuePopover = false)">
            <div v-for="item in valueList" :key="item" class="selector-item" @click="handleSelectValue(item)">
              {{ item }}
            </div>
          </div>
        </template>
      </bk-popover>
      <div v-show="showValueError" class="error-msg is--value">
        {{ $t("需以字母、数字开头和结尾，可包含 '-'，'_'，'.' 和字母数字及负数") }}
      </div>
    </div>
    <div class="action-btns">
      <i v-if="length > 0" class="bk-bscp-icon icon-reduce" @click="emits('delete')"></i>
      <i
        class="bk-bscp-icon icon-add"
        v-bk-tooltips="{ content: $t('分组最多支持 5 个标签选择器'), disabled: length < 4 }"
        @click="emits('add')"></i>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { AngleDown } from 'bkui-vue/lib/icon';
  import GROUP_RULE_OPS from '../../../../constants/group';
  import { EGroupRuleType, IGroupRuleItem } from '../../../../../types/group';
  import { getGroupSelector } from '../../../../api/group';

  const props = defineProps<{
    rule: IGroupRuleItem;
    length: number;
    bkBizId: string;
  }>();
  const emits = defineEmits(['change', 'add', 'delete']);
  const showKeyError = ref(false);
  const showValueError = ref(false);
  const rule = ref({ ...props.rule });

  const valueList = ref<string[]>([]); // value联想输入列表

  // 内置标签
  const BuiltInTag = ['ip', 'pod_name', 'pod_id', 'gray_percent'];
  const keyValidateReg = new RegExp(
    '^[a-z0-9A-Z]([-_a-z0-9A-Z]*[a-z0-9A-Z])?((\\.|\\/)[a-z0-9A-Z]([-_a-z0-9A-Z]*[a-z0-9A-Z])?)*$',
  );
  const valueValidateReg = new RegExp(/^(?:-?\d+(\.\d+)?|[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?)$/);

  const isShowKeyPopover = ref(false);
  const isShowValuePopover = ref(false);
  const keyInputRef = ref();

  onMounted(async () => {
    if (rule.value.key) {
      await getValueList();
    }
  });

  const handleKeyChange = async () => {
    keyInputRef.value.blur();
    isShowKeyPopover.value = false;
    validateKey();
    await getValueList();
    handleRuleChange();
  };

  const handleValueChange = () => {
    validateValue();
    handleRuleChange();
  };

  const getValueList = async () => {
    try {
      const res = await getGroupSelector(props.bkBizId, rule.value.key);
      valueList.value = res.values;
    } catch (error) {
      console.error(error);
    }
  };

  const handleSelectKey = (item: string) => {
    rule.value.key = item;
    handleKeyChange();
  };

  const handleSelectValue = (item: string) => {
    if (['in', 'nin'].includes(rule.value.op)) {
      rule.value.value = rule.value.value ? [...(rule.value.value as string[]), item] : [item];
    } else {
      rule.value.value = item;
      isShowValuePopover.value = false;
    }
    validateValue();
    handleRuleChange();
  };

  // 获取操作符对应操作值的数据类型
  const getOpValType = (op: string) => {
    if (['in', 'nin'].includes(op)) {
      return 'array';
    }
    if (['gt', 'ge', 'lt', 'le'].includes(op)) {
      return 'number';
    }
    return 'string';
  };

  // 操作符修改后，string和number类型之间操作值可直接转换时自动转换，不能转换则设置为默认空值
  const handleLogicChange = (val: EGroupRuleType) => {
    const newValType = getOpValType(val);
    const oldValType = getOpValType(rule.value.op);
    if (newValType !== oldValType) {
      if (newValType === 'array' && ['string', 'number'].includes(oldValType)) {
        rule.value.value = [];
      } else if (newValType === 'string' && oldValType === 'number') {
        rule.value.value = String(rule.value.value);
      } else if (newValType === 'number' && oldValType === 'string' && /\d+/.test(rule.value.value as string)) {
        rule.value.value = Number(rule.value.value);
      } else {
        rule.value.value = '';
      }
    }
    rule.value.op = val;
    handleRuleChange();
  };

  // 验证key
  const validateKey = () => {
    showKeyError.value = !keyValidateReg.test(rule.value.key);
    if (showValueError.value) {
      showValueError.value = false;
    }
  };

  // 验证value
  const validateValue = () => {
    if (Array.isArray(rule.value.value)) {
      const valueArrValidation = (rule.value.value as string[]).every((item: string) => {
        return valueValidateReg.test(item);
      });
      showValueError.value = !valueArrValidation || !((rule.value.value as string[]).length > 0);
    } else {
      showValueError.value = !valueValidateReg.test(`${rule.value.value}`);
    }
    if (showKeyError.value) {
      showKeyError.value = false;
    }
  };

  const handleRuleChange = () => {
    validateValue();
    validateKey();
    emits('change', rule.value);
  };

  defineExpose({
    validate: () => {
      validateKey();
      validateValue();
      return !showKeyError.value && !showValueError.value;
    },
  });
</script>

<style scoped lang="scss">
  .rule-config {
    position: relative;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    position: relative;
    margin-top: 15px;
    .rule-logic {
      position: absolute;
      top: 3px;
      left: -48px;
      height: 26px;
      line-height: 26px;
      width: 40px;
      background: #e1ecff;
      color: #3a84ff;
      font-size: 12px;
      text-align: center;
      cursor: pointer;
    }
    .value-input {
      width: 280px;
    }
  }
  .action-btns {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 38px;
    height: 32px;
    font-size: 14px;
    color: #979ba5;
    .bk-bscp-icon {
      cursor: pointer;
    }
    i:hover {
      color: #3a84ff;
    }
  }
  .key-input {
    width: 174px;
  }

  .suffix-icon {
    width: 20px;
    font-size: 14px;
    &.show-popover {
      transform: rotate(180deg);
    }
  }
  .selector-list {
    width: 174px;
    padding: 4px 0;
    .selector-item {
      height: 32px;
      line-height: 32px;
      padding: 0 12px;
      cursor: pointer;
      align-items: center;
      &:hover {
        background-color: #f5f7fa;
        color: #63656e;
      }
    }
  }

  .value-selector-list {
    @extend .selector-list;
    width: 280px;
    max-height: 300px;
    overflow: auto;
  }

  .is-error {
    border-color: #ea3636;
    &:focus-within {
      border-color: #3a84ff;
    }
    &:hover:not(.is-disabled) {
      border-color: #ea3636;
    }
    :deep(.bk-tag-input-trigger) {
      border-color: #ea3636;
    }
  }
  .error-msg {
    font-size: 12px;
    line-height: 14px;
    white-space: normal;
    word-wrap: break-word;
    color: #ea3636;
    animation: form-error-appear-animation 0.15s;
    margin-top: 8px;
    &.is--key {
      white-space: nowrap;
    }
  }
  @keyframes form-error-appear-animation {
    0% {
      opacity: 0;
      transform: translateY(-30%);
    }
    100% {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>
