<template>
  <bk-form ref="formRef" form-type="vertical" :model="localVal" :rules="rules">
    <bk-form-item :label="t('模板套餐名称')" property="name" required>
      <bk-input v-model="localVal.name" :placeholder="t('请输入')" @input="change" />
    </bk-form-item>
    <bk-form-item :label="t('模板套餐描述')" property="memo">
      <bk-input
        v-model="localVal.memo"
        type="textarea"
        :placeholder="t('请输入')"
        :rows="6"
        :maxlength="200"
        :resize="true"
        @input="change" />
    </bk-form-item>
    <service-scope-selector
      ref="envAppRef"
      :form-data="localVal"
      :space-id="spaceId"
      :project-id="projectId"
      config-type="file"
      :show-removed-tip="true"
      :apps="apps"
      @update:form-data="(val) => (localVal = val)"
      @change="change" />
  </bk-form>
</template>
<script lang="ts" setup>
  import { ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { cloneDeep } from 'lodash';
  import { ITemplatePackageEditParams } from '../../../../../../types/template';
  import ServiceScopeSelector from '../../../../../components/service-scope-selector.vue';

  const { t } = useI18n();

  const props = defineProps<{
    spaceId: string;
    projectId: string;
    data: ITemplatePackageEditParams;
    apps?: number[]; // 套餐绑定的服务，编辑时需要区分哪些服务被去掉
  }>();

  const emits = defineEmits(['change']);

  const localVal = ref<ITemplatePackageEditParams>(cloneDeep(props.data));
  const formRef = ref();
  const rules = {
    memo: [
      {
        validator: (value: string) => value.length <= 200,
        message: t('最大长度 200 个字符'),
      },
    ],
  };
  const envAppRef = ref();

  watch(
    () => props.data,
    (val) => {
      localVal.value = cloneDeep(val);
    },
  );

  const change = () => {
    emits('change', localVal.value);
  };

  const validate = async () => {
    console.log(formRef.value.validate() && envAppRef.value.validate());
    return await formRef.value.validate() && envAppRef.value.validate();
  };

  defineExpose({
    validate,
  });
</script>
<style lang="scss" scoped>
</style>
