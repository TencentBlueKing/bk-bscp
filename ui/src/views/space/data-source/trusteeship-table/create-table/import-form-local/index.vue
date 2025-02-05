<template>
  <div class="import-form-local-wrap">
    <Card :title="$t('文件来源')" class="file-source-card">
      <div class="label">{{ $t('选择文件') }}</div>
      <bk-upload
        class="file-uploader"
        theme="button"
        :tip="$t('支持 .xlsx / .xls / .csv / .sql 文件，后台会自动检测文件类型，配置项格式请参照')"
        :size="100000"
        :multiple="false"
        accept=".xlsx, .xls, .csv, .sql"
        :custom-request="handleFileUpload">
        <template #trigger>
          <div ref="buttonRef">
            <bk-button class="upload-button">
              <upload />
              <span class="text">{{ $t('上传文件') }}</span>
            </bk-button>
          </div>
        </template>
      </bk-upload>
    </Card>
    <TableStructureForm :is-edit="false" :is-manual-create="false" :bk-biz-id="props.bkBizId" />
  </div>
</template>

<script lang="ts" setup>
  import { Upload } from 'bkui-vue/lib/icon';
  import Card from '../../../component/card.vue';
  import TableStructureForm from '../../components/table-structure-form.vue';

  const props = defineProps<{
    bkBizId: string;
  }>();

  const handleFileUpload = () => {};
</script>

<style scoped lang="scss">
  .file-source-card {
    margin-bottom: 16px;
    .label {
      position: relative;
      font-size: 12px;
      color: #63656e;
      &::after {
        position: absolute;
        top: 0;
        width: 14px;
        color: #ea3636;
        text-align: center;
        content: '*';
      }
    }
    .file-uploader {
      margin-top: 6px;
      :deep(.bk-upload-list) {
        display: none;
      }
    }
  }
</style>
