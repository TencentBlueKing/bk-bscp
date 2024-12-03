<template>
  <bk-button
    v-if="versionData.id === 0"
    v-cursor="{ active: !props.hasPerm }"
    theme="primary"
    :class="['trigger-button', { 'bk-button-with-no-perm': !props.hasPerm }]"
    :disabled="createBtnDisabled"
    :loading="createVersionBtnLoading"
    @click="handleBtnClick"
    v-bk-tooltips="{
      content: $t('在未命名版本中发现过期证书，请处理证书问题后再生成版本'),
      disabled: !hasExpiredCert,
      placement: 'bottom',
    }">
    {{ t('生成版本') }}
  </bk-button>
  <CreateVersionSlider
    v-model:show="isVersionSliderShow"
    ref="createSliderRef"
    :bk-biz-id="props.bkBizId"
    :app-id="props.appId"
    :is-diff-slider-show="isDiffSliderShow"
    @created="handleCreated" />
  <CeartExpiredDialog v-model:show="isCertDialogShow" :dialog-data="unExpiredCertList" @confirm="handleSecondConfirm" />
</template>
<script setup lang="ts">
  import { ref, computed, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import Message from 'bkui-vue/lib/message';
  import useGlobalStore from '../../../../../../store/global';
  import useConfigStore from '../../../../../../store/config';
  import useServiceStore from '../../../../../../store/service';
  import { IConfigVersion, IExpiredCert, IConfigKvType } from '../../../../../../../types/config';
  import { getUnExpiredCertList } from '../../../../../../api/config';
  import CreateVersionSlider from './create-version-slider.vue';
  import { datetimeFormat } from '../../../../../../utils';
  import dayjs from 'dayjs';
  import CeartExpiredDialog from './ceart-expired-dialog.vue';

  const props = defineProps<{
    bkBizId: string;
    appId: number;
    permCheckLoading: boolean;
    hasPerm: boolean;
  }>();

  const emits = defineEmits(['confirm']);
  const { t } = useI18n();

  const { permissionQuery, showApplyPermDialog } = storeToRefs(useGlobalStore());
  const {
    allExistConfigCount,
    versionData,
    conflictFileCount,
    createVersionBtnLoading,
    hasExpiredCert,
    refreshHasExpiredCertFlag,
  } = storeToRefs(useConfigStore());
  const { appData } = storeToRefs(useServiceStore());
  const configStore = useConfigStore();

  const isVersionSliderShow = ref(false);
  const isDiffSliderShow = ref(false);
  const createSliderRef = ref();
  const unExpiredCertList = ref<IExpiredCert[]>([]);
  const isCertDialogShow = ref(false);

  watch(
    () => refreshHasExpiredCertFlag.value,
    async (val) => {
      if (appData.value.spec.config_type === 'kv' && val) {
        await getCertList();
      }
    },
  );

  const permissionQueryResource = computed(() => [
    {
      biz_id: props.bkBizId,
      basic: {
        type: 'app',
        action: 'generate_release',
        resource_id: props.appId,
      },
    },
  ]);

  const createBtnDisabled = computed(() => {
    if (appData.value.spec.config_type === 'file') {
      return conflictFileCount.value > 0;
    }
    if (appData.value.spec.config_type === 'kv') {
      return hasExpiredCert.value;
    }
    return !props.hasPerm || allExistConfigCount.value === 0 || props.permCheckLoading;
  });

  const handleBtnClick = () => {
    if (unExpiredCertList.value.length > 0) {
      isCertDialogShow.value = true;
    } else {
      handleSecondConfirm();
    }
  };

  const handleSecondConfirm = () => {
    if (props.hasPerm) {
      isVersionSliderShow.value = true;
    } else {
      permissionQuery.value = { resources: permissionQueryResource.value };
      showApplyPermDialog.value = true;
    }
  };

  const handleCreated = (versionData: IConfigVersion, isPublish: boolean) => {
    isDiffSliderShow.value = false;
    isVersionSliderShow.value = false;
    emits('confirm', versionData, isPublish);
    Message({ theme: 'success', message: t('新版本已生成') });
  };

  const getCertList = async () => {
    try {
      const { bkBizId, appId } = props;
      const queryParams = { days: 30, all: true };
      const res = await getUnExpiredCertList(bkBizId, appId, queryParams);
      unExpiredCertList.value = res.data.details.map((item: IConfigKvType) => {
        const {
          spec: { key: name, certificate_expiration_date },
          id,
        } = item;

        const expirationTime = datetimeFormat(certificate_expiration_date as string);
        const remainingDays = Math.ceil(
          dayjs(expirationTime, 'YYYY-MM-DD HH:mm:ss').diff(dayjs(), 'second') / (60 * 60 * 24),
        );
        return {
          name,
          id,
          remainingDays,
          expirationTime,
        };
      });
      configStore.$patch((state) => {
        state.refreshHasExpiredCertFlag = false;
      });
    } catch (error) {
      console.error('Error fetching unexpired certificates:', error);
    }
  };
</script>
<style lang="scss" scoped>
  .trigger-button {
    margin-left: 8px;
  }
  .create-version-btn {
    margin-right: 8px;
  }
</style>
