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
    <bk-form-item :label="t('服务可见范围')" property="public" required>
      <bk-radio-group v-model="localVal.public" @change="change">
        <bk-radio :label="true">{{ t('公开') }}</bk-radio>
        <bk-radio :label="false">{{ t('指定服务') }}</bk-radio>
      </bk-radio-group>
      <div v-if="!localVal.public" class="env-service-row">
        <bk-form-item :label="t('环境')" required property="env_id" class="env-selector">
          <env-selector
            v-model="localVal.env_id"
            :placeholder="t('请选择环境')"
            :use-default-trigger="true"
            @change="handleEnvInfoChange" />
        </bk-form-item>
        <bk-form-item :label="t('绑定服务')" required property="bound_apps" class="service-selector">
          <bk-select
            v-model="localVal.bound_apps"
            multiple
            filterable
            :placeholder="t('请选择服务')"
            :input-search="false"
            :loading="serviceLoading"
            @change="handleServiceChange">
            <bk-option v-for="service in serviceList" :key="service.id" :label="service.spec.name" :value="service.id">
            </bk-option>
          </bk-select>
        </bk-form-item>
      </div>
      <p v-if="!localVal.public && deletedApps.length > 0" class="tips">
        {{ t('提醒：修改可见范围后，服务') }}
        <span v-for="item in deletedApps" :key="item.id">【{{ item.spec.name }}】</span>
        {{ t('将不再引用此套餐') }}
      </p>
    </bk-form-item>
  </bk-form>
</template>
<script lang="ts" setup>
  import { ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { cloneDeep } from 'lodash';
  import { ITemplatePackageEditParams } from '../../../../../../types/template';
  import { getAppList } from '../../../../../api/index';
  import { IAppItem } from '../../../../../../types/app';
  import EnvSelector from '../../../../../components/env-selector.vue';
  import { IEnvItem } from '../../../../../../types/env';

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
  const serviceLoading = ref(false);
  const serviceList = ref<IAppItem[]>([]);
  const deletedApps = ref<IAppItem[]>([]);
  const rules = {
    memo: [
      {
        validator: (value: string) => value.length <= 200,
        message: t('最大长度200个字符'),
      },
    ],
    bound_apps: [
      {
        validator: (val: number[]) => {
          if (!localVal.value.public && val.length === 0) {
            return false;
          }
          return true;
        },
        message: t('绑定服务不能为空'),
        trigger: 'blur',
      },
    ],
  };

  watch(
    () => props.data,
    (val) => {
      localVal.value = cloneDeep(val);
    },
  );

  const getServiceList = async () => {
    serviceLoading.value = true;
    try {
      const bizId = props.spaceId;
      const query = {
        all: true,
      };
      const resp = await getAppList(bizId, props.projectId, localVal.value.env_id, query);
      serviceList.value = resp.details.filter((service: IAppItem) => service.spec.config_type === 'file');
    } catch (e) {
      console.error(e);
    } finally {
      serviceLoading.value = false;
    }
  };

  // 环境选择变化（带环境信息）
  const handleEnvInfoChange = (_env: IEnvItem, isManual?: boolean) => {
    if (isManual) {
      localVal.value.bound_apps = [];
    };
    getServiceList();
  };

  const handleServiceChange = () => {
    const changed: IAppItem[] = [];
    if (!localVal.value.public && props.apps) {
      props.apps.forEach((id) => {
        if (!localVal.value.bound_apps.includes(id)) {
          const app = serviceList.value.find((item) => item.id === id);
          if (app) {
            changed.push(app);
          }
        }
      });
    }
    deletedApps.value = changed;
    change();
  };

  const change = () => {
    emits('change', localVal.value);
  };

  const validate = () => formRef.value.validate();

  defineExpose({
    validate,
  });
</script>
<style lang="scss" scoped>
  .tips {
    margin: 8px 0;
    line-height: 16px;
    font-size: 12px;
    color: #ff9c01;
  }
  .env-service-row {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 8px;
    padding: 16px 16px 24px;
    border-radius: 2px;
    background-color: #F5F7FA;
    :deep(.bk-form-item) {
      margin-bottom: 0;
    }
    .env-selector {
      flex: 1;
    }
    .service-selector {
      width: 392px;
    }
  }
</style>
