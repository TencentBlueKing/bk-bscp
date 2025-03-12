<template>
  <bk-form form-type="vertical" ref="formRef" :model="formData" :rules="rules">
    <bk-form-item :label="t('分组名称')" required property="name">
      <bk-input v-model="formData.name" :placeholder="t('请输入分组名称')" @blur="change"></bk-input>
    </bk-form-item>
    <bk-form-item class="radio-group-form" :label="t('服务可见范围')" required property="public">
      <bk-radio-group v-model="formData.public" @change="change">
        <bk-radio :label="true">{{ t('公开') }}</bk-radio>
        <bk-radio :label="false">{{ t('指定服务') }}</bk-radio>
      </bk-radio-group>
      <bk-select
        v-if="!formData.public"
        v-model="formData.bind_apps"
        class="service-selector"
        multiple
        filterable
        :placeholder="t('请选择服务')"
        :input-search="false"
        @change="change">
        <bk-option
          v-for="service in serviceList"
          :key="service.id"
          :label="service.spec.name"
          :value="service.id"></bk-option>
      </bk-select>
    </bk-form-item>
    <bk-form-item class="radio-group-form" :label="t('标签选择器')" required property="rules">
      <template #label>
        <span class="label-text">{{ t('标签选择器') }}</span>
        <span
          ref="nodeRef"
          v-bk-tooltips="{
            content: t(
              '标签选择器由key、操作符、value组成，筛选符合条件的客户端拉取服务配置，一般用于灰度发布服务配置',
            ),
          }"
          class="bk-tooltips-base">
          <Info />
        </span>
      </template>
      <div v-for="(rule, index) in formData.rules" class="rule-config" :key="index">
        <TagSelector
          ref="tagSelectorRef"
          :rule="rule"
          :length="index"
          :bk-biz-id="route.params.spaceId as string"
          @change="handleRuleChange(index, $event)"
          @add="handleAddRule(index)"
          @delete="handleDeleteRule(index)" />
      </div>
      <!-- <div v-if="!rulesValid" class="bk-form-error">{{ t('分组规则表单不能为空') }}</div> -->
    </bk-form-item>
  </bk-form>
</template>
<script setup lang="ts">
  import { ref, watch, onMounted } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRoute } from 'vue-router';
  import { cloneDeep } from 'lodash';
  import { IGroupEditing, IGroupRuleItem } from '../../../../../types/group';
  import { getAppList } from '../../../../api/index';
  import { IAppItem } from '../../../../../types/app';
  import { Info } from 'bkui-vue/lib/icon';
  import TagSelector from './tag-selector.vue';

  const getDefaultRuleConfig = (): IGroupRuleItem => ({ key: '', op: 'eq', value: '' });

  const route = useRoute();
  const { t } = useI18n();

  const props = defineProps<{
    group: IGroupEditing;
  }>();

  const emits = defineEmits(['change']);

  const serviceLoading = ref(false);
  const serviceList = ref<IAppItem[]>([]);
  const formData = ref(cloneDeep(props.group));
  const formRef = ref();
  const tagSelectorRef = ref();

  const rules = {
    name: [
      {
        validator: (value: string) => value.length <= 128,
        message: t('最大长度128个字符'),
      },
      {
        validator: (value: string) => {
          if (value.length > 0) {
            return /^[\u4e00-\u9fa5a-zA-Z0-9][\u4e00-\u9fa5a-zA-Z0-9_-]*[\u4e00-\u9fa5a-zA-Z0-9]?$/.test(value);
          }
          return true;
        },
        message: t('仅允许使用中文、英文、数字、下划线、中划线，且必须以中文、英文、数字开头和结尾'),
      },
    ],
    public: [
      {
        validator: (val: boolean) => {
          if (!val && formData.value.bind_apps.length === 0) {
            return false;
          }
          return true;
        },
        message: t('指定服务不能为空'),
      },
    ],
  };

  watch(
    () => props.group,
    (val) => {
      formData.value = cloneDeep(val);
    },
  );

  onMounted(() => {
    getServiceList();
  });

  const getServiceList = async () => {
    serviceLoading.value = true;
    try {
      const bizId = route.params.spaceId as string;
      const query = {
        all: true,
      };
      const resp = await getAppList(bizId, query);
      serviceList.value = resp.details;
    } catch (e) {
      console.error(e);
    } finally {
      serviceLoading.value = false;
    }
  };

  // 增加规则
  const handleAddRule = (index: number) => {
    if (formData.value.rules.length === 5) {
      return;
    }
    const rule = getDefaultRuleConfig();
    formData.value.rules.splice(index + 1, 0, rule);
  };

  // 删除规则
  const handleDeleteRule = (index: number) => {
    formData.value.rules.splice(index, 1);
    change();
  };

  const handleRuleChange = (index: number, rule: IGroupRuleItem) => {
    formData.value.rules.splice(index, 1, rule);
    change();
  };

  const change = () => {
    emits('change', formData.value);
  };

  const validate = () => {
    const validate = tagSelectorRef.value.every((item: any) => item.validate());
    return formRef.value.validate() && validate;
  };

  defineExpose({
    validate,
  });
</script>
<style lang="scss" scoped>
  .bk-form {
    :deep(.bk-form-label) {
      font-size: 12px;
    }
    :deep(.radio-group-form .bk-form-content) {
      line-height: 1;
    }
  }
  .service-selector {
    margin-top: 10px;
  }
  .published-version {
    line-height: 16px;
    font-size: 12px;
    color: #313238;
  }
  .label-text {
    margin-right: 5px;
  }
  .bk-tooltips-base {
    font-size: 14px;
    color: #3a84ff;
    line-height: 19px;
    vertical-align: middle;
  }
</style>

<style>
  .bk-popover.bk-pop2-content.group-selector-popover {
    padding: 0;
  }
</style>
