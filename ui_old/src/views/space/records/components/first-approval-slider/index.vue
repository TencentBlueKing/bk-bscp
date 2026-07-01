<template>
  <VersionContent
    :btn-loading="props.btnLoading"
    :show="props.show"
    :current-version="props.currentVersion"
    :app-id="props.appId"
    @publish="emits('publish')"
    @update:show="emits('close')">
    <template #currentHead>
      <div class="panel-header">
        <div class="version-tag current">{{ $t('待上线') }}</div>
        <div class="version-title">
          <span class="text">{{ props.currentVersion.spec.name }}</span>
          <ReleasedGroupViewer
            placement="bottom-start"
            :bk-biz-id="props.bkBizId"
            :app-id="props.appId"
            :groups="props.currentVersionGroups"
            :is-pending="true">
            <i class="bk-bscp-icon icon-resources-fill"></i>
          </ReleasedGroupViewer>
        </div>
      </div>
    </template>
  </VersionContent>
</template>
<script setup lang="ts">
  import { IConfigVersion, IReleasedGroup } from '../../../../../../types/config';
  import VersionContent from './version-components/index.vue';
  import ReleasedGroupViewer from '../../../service/detail/config/components/released-group-viewer.vue';

  const props = defineProps<{
    bkBizId: string;
    appId: number;
    show: boolean;
    currentVersion: IConfigVersion; // 当前版本详情信息
    currentVersionGroups: IReleasedGroup[]; // 当前版本上线分组实例
    btnLoading?: boolean;
  }>();

  const emits = defineEmits(['publish', 'close']);
</script>
<style lang="scss" scoped>
  .panel-header {
    display: flex;
    align-items: center;
    padding: 0 16px;
    height: 100%;
    background: transparent;
  }
  .version-tag {
    flex-shrink: 0;
    margin-right: 8px;
    padding: 0 10px;
    height: 22px;
    line-height: 22px;
    font-size: 12px;
    color: #14a568;
    background: #e4faf0;
    border-radius: 2px;
    &.base {
      color: #3a84ff;
      background: #edf4ff;
    }
  }
  .version-title {
    flex: 1;
    display: flex;
    align-items: center;
    padding-right: 20px;
    color: #b6b6b6;
    font-size: 12px;
    overflow: hidden;
    .bk-bscp-icon {
      margin-left: 3px;
      font-size: 16px;
      color: #979ba5;
      &:hover {
        color: #3a84ff;
        cursor: pointer;
      }
    }
  }
</style>
