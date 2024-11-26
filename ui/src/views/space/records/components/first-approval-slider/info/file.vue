<template>
  <section class="file-diff">
    <div class="version-content">
      <template v-if="props.current">
        <div class="property-diff">
          <div class="property-wrap">
            <div>
              <span class="label">{{ `${t('权限')}：` }}</span>
              <span class="value">
                {{ props.currentPermission.privilege }}
              </span>
            </div>
            <div>
              <span class="label">{{ `${t('用户')}：` }}</span>
              <span class="value">
                {{ props.currentPermission.user }}
              </span>
            </div>
            <div>
              <span class="label">{{ `${t('用户组')}：` }}</span>
              <span class="value">
                {{ props.currentPermission.user_group }}
              </span>
            </div>
          </div>
        </div>
        <div class="file-content-diff">
          <div class="file-wrapper" @click="handleDownloadFile(props.current)">
            <div class="basic-info">
              <TextFill class="file-icon" />
              <div class="content">
                <div class="name">{{ props.current.name }}</div>
                <div class="time">{{ props.current.update_at }}</div>
              </div>
              <div class="size">{{ props.current.size }}</div>
            </div>
            <div class="signature">{{ props.current.signature }}</div>
          </div>
        </div>
      </template>
      <bk-exception v-else class="exception-tips" scene="part" theme="empty">{{ t('文件已被删除') }}</bk-exception>
    </div>
  </section>
</template>
<script setup lang="ts">
  import { withDefaults } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRoute } from 'vue-router';
  import { TextFill } from 'bkui-vue/lib/icon';
  import { IFileConfigContentSummary } from '../../../../../../../types/config';
  import { downloadConfigContent } from '../../../../../../api/config';
  import { downloadTemplateContent } from '../../../../../../api/template';
  import { fileDownload } from '../../../../../../utils/file';

  interface IPermissionType {
    privilege: string;
    user: string;
    user_group: string;
  }

  const { t } = useI18n();
  const route = useRoute();
  const bkBizId = String(route.params.spaceId);

  const props = withDefaults(
    defineProps<{
      current: IFileConfigContentSummary;
      currentPermission: IPermissionType;
      id: number;
      downloadable?: boolean;
      isTpl?: boolean;
    }>(),
    {
      downloadable: true,
    },
  );

  // 下载已上传文件
  const handleDownloadFile = async (config: IFileConfigContentSummary) => {
    if (!props.downloadable) {
      return;
    }
    const { signature, name } = config;
    const getConfigContent = props.isTpl ? downloadTemplateContent : downloadConfigContent;
    const res = await getConfigContent(bkBizId, props.id, signature, true);
    fileDownload(res, name);
  };
</script>
<style lang="scss" scoped>
  .file-diff {
    display: flex;
    align-items: center;
    height: 100%;
    background: #fafbfd;
  }
  .version-content {
    padding: 24px;
    width: 100%;
    height: 100%;
  }
  .version-content {
    // border-left: 1px solid #dcdee5;
    .property-diff::before,
    .file-content-diff::before {
      content: '';
      display: block;
      height: 28px;
    }
  }
  .file-wrapper {
    padding: 21px 16px;
    background: #ffffff;
    font-size: 12px;
    border: 1px solid #c4c6cc;
    border-radius: 2px;
    .basic-info {
      display: flex;
      align-items: center;
      justify-content: space-between;
    }
    .signature {
      margin-top: 14px;
      padding: 6px 8px;
      font-size: 12px;
      color: #979ba5;
      background: #f5f7fa;
      border-radius: 2px;
      word-break: break-all;
    }
  }
  .file-icon {
    margin-right: 17px;
    font-size: 28px;
    color: #63656e;
  }
  .content {
    flex: 1;
    .name {
      color: #63656e;
      line-height: 20px;
    }
    .time {
      margin-top: 2px;
      color: #979ba5;
      line-height: 16px;
    }
  }
  .size {
    color: #63656e;
    font-weight: 700;
  }
  .exception-tips {
    margin-top: 100px;
    font-size: 12px;
  }

  .label {
    font-size: 12px;
    color: #63656e;
    line-height: 20px;
    margin-bottom: 8px;
  }

  .property-diff {
    margin-bottom: 16px;
  }
  .property-wrap {
    padding: 14px 0;
    height: 112px;
    background: #ffffff;
    border: 1px solid #c4c6cc;
    border-radius: 2px;
    font-size: 12px;
    .label {
      display: inline-block;
      width: 70px;
      text-align: right;
    }
    .value {
      color: #313238;
      &.diff {
        color: #ff9c01;
      }
    }
  }
</style>
