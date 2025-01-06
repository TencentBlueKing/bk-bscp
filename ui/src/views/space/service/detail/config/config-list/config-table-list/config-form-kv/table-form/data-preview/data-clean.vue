<template>
  <div class="data-clean-wrap">
    <div class="title">{{ $t('数据清洗') }}</div>
    <div class="rule-wrap">
      <div v-for="(rule, index) in ruleList" :key="index" class="rule-item">
        <bk-select v-model="rule.key" :placeholder="$t('请选择字段')" class="field-select">
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
            :placeholder="$t('请输入条件值')">
          </bk-tag-input>
          <bk-input
            v-else
            v-model="rule.value"
            :class="{ 'is-error': showErrorValueValidation[index] }"
            :placeholder="$t('请输入条件值')">
          </bk-input>
          <div v-show="showErrorValueValidation[index]" class="error-msg is--value">
            {{ $t("需以字母、数字开头和结尾，可包含 '-'，'_'，'.' 和字母数字及负数") }}
          </div>
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
  import { IDataCleanItem } from '../../../../../../../../../../../types/kv-table';
  import { KV_TABLE_CLEAN_RULE } from '../../../../../../../../../../constants/config';
  import { cloneDeep } from 'lodash';
  interface IFieldItem {
    name: string;
    alias: string;
    column_type: string;
    primary: boolean;
  }
  const props = defineProps<{
    fields: IFieldItem[];
    ruleList: IDataCleanItem[];
  }>();
  const emits = defineEmits(['change', 'close']);

  const getDefaultRuleConfig = (): IDataCleanItem => ({ key: '', op: 'eq', value: '' });
  const ruleList = ref<IDataCleanItem[]>(cloneDeep(props.ruleList));
  const showErrorKeyValidation = ref<boolean[]>([]);
  const showErrorValueValidation = ref<boolean[]>([]);

  onMounted(() => {
    if (ruleList.value.length === 0) {
      handleAddRule(0);
    }
  });

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

  const handleConfirm = () => {
    emits('change', ruleList.value);
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
</style>
