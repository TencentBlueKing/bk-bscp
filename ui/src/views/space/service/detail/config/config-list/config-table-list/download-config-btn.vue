<template>
  <bk-button text theme="primary" :loading="pending" :disabled="props.disabled" @click="handleDownloadConfig">
    {{ $t('下载') }}
  </bk-button>
</template>
<script lang="ts" setup>
  import { storeToRefs } from 'pinia';
  import { ref } from 'vue';
  import {
    getConfigItemDetail,
    getReleasedConfigItemDetail,
    downloadConfigContent,
  } from '../../../../../../../api/config';
  import {
    getTemplateVersionsDetailByIds,
    getTemplateVersionDetail,
    downloadTemplateContent,
  } from '../../../../../../../api/template';
  import useConfigStore from '../../../../../../../store/config';
  import useGlobalStore from '../../../../../../../store/global';
  import { fileDownload } from '../../../../../../../utils/file';

  const { versionData } = storeToRefs(useConfigStore());
  const { projectId } = storeToRefs(useGlobalStore());

  const props = defineProps<{
    bkBizId: string;
    envId: string;
    appId: number;
    id: number;
    type: string; // 取值为config/template，分别表示非模板套餐下配置文件和模板套餐下配置文件
    disabled?: boolean;
  }>();

  const pending = ref(false);

  const handleDownloadConfig = async () => {
    let signature;
    let content;
    let fileName;
    pending.value = true;
    const { bkBizId, appId, envId, id} = props;
    if (props.type === 'config') {
      let res;
      if (versionData.value.id) {
        res = await getReleasedConfigItemDetail(bkBizId, appId, projectId.value, envId, versionData.value.id, id);
        signature = res.config_item.commit_spec.content.signature;
      } else {
        res = await getConfigItemDetail(bkBizId, id, appId, projectId.value, envId);
        signature = res.content.signature;
      }
      fileName = res.config_item.spec.name;
      content = await downloadConfigContent(bkBizId, appId, signature, true);
    } else {
      let templateSpaceId;
      if (versionData.value.id) {
        const res = await getTemplateVersionDetail(
          bkBizId,
          projectId.value,
          appId,
          versionData.value.id,
          id);
        signature = res.detail.signature;
        fileName = res.detail.name;
        templateSpaceId = res.detail.template_space_id;
      } else {
        const res = await getTemplateVersionsDetailByIds(bkBizId, projectId.value, [id]);
        signature = res.details[0].spec.content_spec.signature;
        fileName = res.details[0].spec.name;
        templateSpaceId = res.details[0].attachment.template_space_id;
      }
      content = await downloadTemplateContent(bkBizId, templateSpaceId, signature, true);
    }
    fileDownload(content, fileName);

    pending.value = false;
  };
</script>
