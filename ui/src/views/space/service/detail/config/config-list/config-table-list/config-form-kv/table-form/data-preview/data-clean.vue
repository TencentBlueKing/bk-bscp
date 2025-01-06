<template>
  <div class="data-clean-wrap">
    <div class="title">{{ $t('数据清洗') }}</div>
    <div class="rule-wrap">
      <div v-for="(rule, index) in ruleList" :key="index" class="rule-item">
        <bk-select
          v-model="rule.key"
          :placeholder="$t('请选择字段')"
          :class="['field-select', { 'is-error': showErrorKeyValidation[index] }]"
          @change="handleSelectField(rule)"
          @blur="validateKey(index)">
          <bk-option
            v-for="field in props.fields"
            :key="field.name"
            :value="field.name"
            :label="field.alias"></bk-option>
        </bk-select>
        <bk-select v-model="rule.op" style="width: 82px" :clearable="false">
          <bk-option v-for="op in KV_TABLE_CLEAN_RULE" :key="op.id" :value="op.id" :label="op.name"></bk-option>
        </bk-select>
        <div class="value-input">
          <bk-select
            v-if="rule.field?.column_type === 'enum'"
            v-model="rule.value"
            :class="{ 'is-error': showErrorValueValidation[index] }"
            :multiple="rule.field.selected"
            @change="validateValue(index)"
            @blur="validateValue(index)">
            <bk-option v-for="item in rule.enum_list" :key="item.value" :value="item.value" :label="item.text" />
          </bk-select>
          <template v-else>
            <bk-tag-input
              v-if="['in', 'nin'].includes(rule.op)"
              v-model="rule.value"
              :class="{ 'is-error': showErrorValueValidation[index] }"
              :allow-create="true"
              :collapse-tags="true"
              :has-delete-icon="true"
              :show-clear-only-hover="true"
              :allow-auto-match="true"
              :list="[]"
              :placeholder="$t('请输入条件值')"
              @change="validateValue(index)"
              @blur="validateValue(index)">
            </bk-tag-input>
            <bk-input
              v-else
              v-model="rule.value"
              :class="{ 'is-error': showErrorValueValidation[index] }"
              :placeholder="$t('请输入条件值')"
              @change="validateValue(index)"
              @blur="validateValue(index)">
            </bk-input>
          </template>
        </div>
        <div class="action-btns">
          <i v-if="index === ruleList.length - 1" class="bk-bscp-icon icon-add" @click="handleAddRule(index)"></i>
          <i
            v-if="index > 0 || ruleList.length > 1"
            class="bk-bscp-icon icon-reduce"
            @click="handleDeleteRule(index)"></i>
        </div>
      </div>
    </div>
    <div class="operations-btn">
      <bk-button theme="primary" @click="handleConfirm">{{ $t('确定') }}</bk-button>
      <bk-button @click="emits('close')">{{ $t('取消') }}</bk-button>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { IDataCleanItem, IFieldItem } from '../../../../../../../../../../../types/kv-table';
  import { KV_TABLE_CLEAN_RULE } from '../../../../../../../../../../constants/config';
  import { cloneDeep } from 'lodash';

  interface IRuleItem extends IDataCleanItem {
    field?: IFieldItem;
    enum_list?: { text: string; value: string }[];
  }

  const props = defineProps<{
    fields: IFieldItem[];
    ruleList: IDataCleanItem[];
  }>();
  const emits = defineEmits(['change', 'close']);

  const getDefaultRuleConfig = (): IDataCleanItem => ({ key: '', op: 'eq', value: '' });
  const ruleList = ref<IRuleItem[]>(cloneDeep(props.ruleList));
  const showErrorKeyValidation = ref<boolean[]>([]);
  const showErrorValueValidation = ref<boolean[]>([]);

  onMounted(() => {
    if (ruleList.value.length > 0) {
      ruleList.value.forEach((rule) => {
        rule.field = props.fields.find((item) => item.name === rule.key);
        if (rule.field?.column_type === 'enum') {
          rule.enum_list = JSON.parse(rule.field!.enum_value);
          if (rule.field.selected) {
            rule.value = JSON.parse(rule.value as string);
          }
        }
      });
    } else {
      handleAddRule(0);
    }
  });

  const handleSelectField = (rule: IRuleItem) => {
    rule.field = props.fields.find((item) => item.name === rule.key);
    rule.value = rule.field?.column_type === 'enum' ? [] : '';
    if (rule.field?.column_type === 'enum') {
      rule.enum_list = JSON.parse(rule.field!.enum_value);
    }
  };

  const handleAddRule = (index: number) => {
    const rule = getDefaultRuleConfig();
    ruleList.value.splice(index + 1, 0, rule);
    showErrorKeyValidation.value.push(false);
    showErrorValueValidation.value.push(false);
  };

  // 删除规则
  const handleDeleteRule = (index: number) => {
    ruleList.value.splice(index, 1);
    showErrorKeyValidation.value.splice(index, 1);
    showErrorValueValidation.value.splice(index, 1);
  };

  // 校验规则是否有表单项为空
  const validateRules = () => {
    let allValid = true;
    ruleList.value.forEach((item, index) => {
      const { op } = item;
      if (op === '') return (allValid = false);
      item.key ? validateValue(index) : validateKey(index);
    });
    allValid = !showErrorKeyValidation.value.includes(true) && !showErrorValueValidation.value.includes(true);
    if (ruleList.value.length === 1 && ruleList.value[0].key === '') {
      allValid = true;
    }
    return allValid;
  };

  // 验证key
  const validateKey = (index: number) => {
    showErrorKeyValidation.value[index] = ruleList.value[index].key === '';
    if (showErrorValueValidation.value[index]) {
      showErrorValueValidation.value[index] = false;
    }
  };

  // 验证value
  const validateValue = (index: number) => {
    if (Array.isArray(ruleList.value[index].value)) {
      showErrorValueValidation.value[index] = ruleList.value[index].value.length === 0;
    } else {
      showErrorValueValidation.value[index] = ruleList.value[index].value === '';
    }
    if (showErrorKeyValidation.value[index]) {
      showErrorKeyValidation.value[index] = false;
    }
  };

  const handleConfirm = () => {
    if (!validateRules()) return;
    let list = ruleList.value.map((rule) => {
      return {
        key: rule.key,
        op: rule.op,
        value: Array.isArray(rule.value) ? JSON.stringify(rule.value) : rule.value,
      };
    });
    if (list.length === 1 && list[0].key === '') {
      list = [];
    }
    emits('change', list);
    emits('close');
  };
</script>

<style scoped lang="scss">
  .data-clean-wrap {
    padding: 12px 16px;
    background: #f5f7fa;
    border-radius: 2px;
    margin-bottom: 24px;
    .title {
      color: #313238;
      font-size: 14px;
    }
    .operations-btn {
      display: flex;
      gap: 8px;
    }
  }
  .rule-wrap {
    margin: 8px 0 16px;
    .field-select {
      width: 358px;
    }
    .rule-item {
      position: relative;
      display: flex;
      align-items: flex-start;
      gap: 4px;
      position: relative;
      margin-top: 15px;
      .rule-logic {
        position: absolute;
        top: 3px;
        left: -48px;
        height: 26px;
        line-height: 26px;
        width: 68px;
        background: #e1ecff;
        color: #3a84ff;
        font-size: 12px;
        text-align: center;
        cursor: pointer;
      }
      .value-input {
        width: 240px;
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
    :deep(.bk-input--default) {
      @extend .is-error;
    }
  }
</style>
