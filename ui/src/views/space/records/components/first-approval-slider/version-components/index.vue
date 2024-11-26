<template>
  <bk-sideslider
    :is-show="props.show"
    :title="t('审批版本详情')"
    ext-cls="config-version-diff-slider"
    :width="960"
    @closed="handleClose">
    <bk-loading class="loading-wrapper" :loading="loading">
      <div v-if="!loading" class="version-diff-content">
        <AsideMenu
          :app-id="props.appId"
          :current-version-id="currentVersion.id"
          :un-named-version-variables="props.unNamedVersionVariables"
          :selected-config="props.selectedConfig"
          :selected-kv-config-id="selectedKV"
          @selected="handleSelectDiffItem"
          @render="publishBtnLoading = $event" />
        <div class="content-area">
          <info :diff="diffDetailData" :id="props.appId" :selected-kv-config-id="selectedKV" :loading="false">
            <template #headContent>
              <slot name="currentHead"> </slot>
            </template>
          </info>
        </div>
      </div>
    </bk-loading>
    <template #footer>
      <div class="actions-btns">
        <slot name="footerActions">
          <bk-button
            :loading="publishBtnLoading || props.btnLoading"
            :disabled="publishBtnLoading || props.btnLoading"
            class="publish-btn"
            theme="primary"
            @click="emits('publish')">
            {{ t('通过') }}
          </bk-button>
          <bk-button :loading="publishBtnLoading || props.btnLoading" @click="emits('reject')">
            {{ t('驳回') }}
          </bk-button>
        </slot>
      </div>
    </template>
  </bk-sideslider>
</template>
<script setup lang="ts">
  import { ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  // import { IConfigVersion, IConfigDiffSelected } from '../../../../../../../../types/config';
  import { IConfigVersion, IConfigDiffSelected } from '../../../../../../../types/config';
  import { IDiffDetail } from '../../../../../../../types/service';
  import { IVariableEditParams } from '../../../../../../../types/variable';
  // import { getConfigVersionList } from '../../../../../../api/config';
  import AsideMenu from './aside-menu/index.vue';
  // import Diff from '../../../../../../components/diff/index.vue';
  import Info from '../info/index.vue';

  const getDefaultDiffData = (): IDiffDetail => ({
    // 差异详情数据
    id: 0,
    contentType: 'text',
    is_secret: false,
    secret_hidden: false,
    current: {
      language: '',
      content: '',
    },
    base: {
      language: '',
      content: '',
    },
  });

  const { t } = useI18n();
  const props = defineProps<{
    show: boolean;
    currentVersion: IConfigVersion; // 当前版本详情
    unNamedVersionVariables?: IVariableEditParams[];
    baseVersionId?: number; // 默认选中的基准版本id
    selectedConfig?: IConfigDiffSelected; // 默认选中的配置文件
    versionDiffList?: IConfigVersion[];
    selectedKvConfigId?: number; // 选中的kv类型配置id
    isApprovalMode?: boolean; // 是否审批模式(操作记录-去审批-拒绝)
    btnLoading?: boolean;
    appId: number;
  }>();

  const emits = defineEmits(['update:show', 'publish', 'reject']);

  // const bkBizId = ref(String(route.params.spaceId));
  const versionList = ref<IConfigVersion[]>([]);
  const selectedKV = ref(props.selectedKvConfigId);
  const diffDetailData = ref<IDiffDetail>(getDefaultDiffData());

  const loading = ref(false);
  const publishBtnLoading = ref(true);

  // watch(
  //   () => props.show,
  //   async (val) => {
  //     publishBtnLoading.value = true;
  //     if (val) {
  //       await getVersionList();
  //       if (props.baseVersionId) {
  //         selectedBaseVersion.value = props.baseVersionId;
  //       } else if (versionList.value.length > 0) {
  //         selectedBaseVersion.value = versionList.value[0].id;
  //       }
  //     }
  //   },
  // );

  watch(
    () => props.selectedKvConfigId,
    (val) => {
      selectedKV.value = val;
    },
  );

  // 获取所有对比基准版本
  // const getVersionList = async () => {
  //   try {
  //     if (props.versionDiffList) {
  //       versionList.value = props.versionDiffList;
  //       return;
  //     }
  //     const res = await getConfigVersionList(bkBizId.value, appId.value, { start: 0, all: true });
  //     versionList.value = res.data.details.filter((item: IConfigVersion) => item.id !== props.currentVersion.id);
  //   } catch (e) {
  //     console.error(e);
  //   }
  // };

  // 选中对比对象，配置或者脚本
  const handleSelectDiffItem = (data: IDiffDetail) => {
    diffDetailData.value = data;
    if (data.contentType === 'singleLineKV') {
      selectedKV.value = data.id as number;
    }
  };

  const handleClose = () => {
    versionList.value = [];
    selectedKV.value = 0;
    diffDetailData.value = getDefaultDiffData();
    emits('update:show', false);
  };
</script>
<style lang="scss" scoped>
  .loading-wrapper {
    height: calc(100vh - 106px);
  }
  .version-diff-content {
    display: flex;
    align-items: center;
    height: 100%;
  }
  .configs-wrapper {
    height: calc(100% - 49px);
    overflow: auto;
    & > li {
      display: flex;
      align-items: center;
      justify-content: space-between;
      position: relative;
      padding: 0 24px;
      height: 41px;
      color: #313238;
      border-bottom: 1px solid #dcdee5;
      cursor: pointer;
      &:hover {
        background: #e1ecff;
        color: #3a84ff;
      }
      &.active {
        background: #e1ecff;
        color: #3a84ff;
      }
      .name {
        width: calc(100% - 24px);
        line-height: 16px;
        font-size: 12px;
        white-space: nowrap;
        text-overflow: ellipsis;
        overflow: hidden;
      }
      .arrow-icon {
        position: absolute;
        top: 50%;
        right: 5px;
        transform: translateY(-60%);
        font-size: 12px;
        color: #3a84ff;
      }
    }
  }
  .content-area {
    width: calc(100% - 264px);
    height: 100%;
  }
  .diff-panel-head {
    display: flex;
    align-items: center;
    padding: 0 16px;
    width: 100%;
    height: 100%;
    font-size: 12px;
    color: #b6b6b6;
    .version-tag {
      margin-right: 8px;
      padding: 0 10px;
      height: 22px;
      line-height: 22px;
      font-size: 12px;
      color: #14a568;
      background: #e4faf0;
      border-radius: 2px;
      &.base-version {
        color: #3a84ff;
        background: #edf4ff;
      }
    }
    .version-name {
      max-width: 300px;
    }
  }
  .actions-btns {
    padding: 0 24px;
    .bk-button {
      min-width: 88px;
    }
    .publish-btn {
      margin-right: 8px;
    }
  }
</style>
<style lang="scss">
  .config-version-diff-slider {
    .bk-modal-body {
      transform: none;
    }
  }
</style>
