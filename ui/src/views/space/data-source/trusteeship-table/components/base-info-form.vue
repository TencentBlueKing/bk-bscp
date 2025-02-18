<template>
  <bk-form form-type="vertical" :model="formData" ref="formRef">
    <Card :title="$t('基本信息')">
      <div class="basic-info-form">
        <bk-form-item :label="$t('表格名称')" property="table_name" required>
          <bk-input v-model="formData.table_name" :disabled="isEdit" @change="handleFormChange" />
        </bk-form-item>
        <bk-form-item :label="$t('表格描述')" property="table_memo">
          <bk-input v-model="formData.table_memo" @change="handleFormChange" />
        </bk-form-item>
      </div>
    </Card>
    <Card :title="$t('可见范围')">
      <bk-form-item :label="$t('选择服务')" property="visible_range" required>
        <bk-select
          v-model="formData.visible_range"
          :loading="serviceLoading"
          style="width: 464px"
          multiple
          filterable
          :placeholder="$t('请选择服务')"
          @change="handleServiceChange">
          <bk-option value="*" :label="$t('全部服务')"></bk-option>
          <bk-option v-for="service in serviceList" :key="service.id" :label="service.spec.name" :value="service.id">
          </bk-option>
        </bk-select>
      </bk-form-item>
    </Card>
  </bk-form>
</template>

<script lang="ts" setup>
  import { ref, onMounted, watch } from 'vue';
  import { getAppList } from '../../../../../api/index';
  import { IAppItem } from '../../../../../../types/app';
  import { ILocalTableBase } from '../../../../../../types/kv-table';
  import Card from '../../component/card.vue';

  const props = defineProps<{
    bkBizId: string;
    isEdit: boolean;
    form: ILocalTableBase;
  }>();

  const emits = defineEmits(['change']);

  const serviceLoading = ref(false);
  const serviceList = ref<IAppItem[]>([]);
  const formRef = ref();
  const formData = ref<ILocalTableBase>({
    ...props.form,
    visible_range: props.form.visible_range.length === 0 ? ['*'] : props.form.visible_range, // 如果没有权限范围，默认为全部
  });

  onMounted(() => {
    getServiceList();
  });

  watch(
    () => props.form,
    () => {
      formData.value = {
        ...props.form,
        visible_range: props.form.visible_range.length === 0 ? ['*'] : props.form.visible_range, // 如果没有权限范围，默认为全部
      };
    },
  );

  const getServiceList = async () => {
    serviceLoading.value = true;
    try {
      const query = {
        start: 0,
        all: true, // @todo 确认拉全量列表参数
      };
      const resp = await getAppList(props.bkBizId, query);
      serviceList.value = resp.details.filter((service: IAppItem) => service.spec.config_type === 'kv');
    } catch (e) {
      console.error(e);
    } finally {
      serviceLoading.value = false;
    }
  };

  const handleServiceChange = (val: string[]) => {
    if (val.length === 0) {
      formData.value.visible_range = [];
    }
    if (formData.value.visible_range[formData.value.visible_range.length - 1] === '*') {
      formData.value.visible_range = ['*'];
    } else if (formData.value.visible_range.length > 1 && formData.value.visible_range[0] === '*') {
      formData.value.visible_range = formData.value.visible_range.slice(1);
    }
    handleFormChange();
  };

  const handleFormChange = () => {
    const form = {
      ...formData.value,
      visible_range: formData.value.visible_range[0] === '*' ? [] : formData.value.visible_range, // 如果没有权限范围，默认为全部
    };
    emits('change', form);
  };

  defineExpose({
    validate: () => formRef.value.validate(),
  });
</script>

<style scoped lang="scss">
  .basic-info-form {
    display: flex;
    gap: 24px;
    .bk-form-item {
      flex: 1;
    }
  }

  .card:not(:last-child) {
    margin-bottom: 16px;
  }
</style>
