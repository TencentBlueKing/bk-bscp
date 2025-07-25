<template>
  <section class="version-container">
    <div class="service-selector-wrapper">
      <ServiceSelector ref="serviceSelectorRef" :value="props.appId" @change="handleAppChange">
        <template #trigger>
          <div class="selector-trigger">
            <input readonly :value="editingService.spec.name" />
            <AngleUpFill class="arrow-icon arrow-fill" />
          </div>
        </template>
      </ServiceSelector>
      <div class="details-btn" v-bk-tooltips="{ content: t('查看服务属性') }" @click="isEditServicePopShow = true">
        <span class="bk-bscp-icon icon-view-detail"></span>
      </div>
    </div>
    <bk-loading :loading="versionListLoading">
      <div class="version-search-wrapper">
        <SearchInput v-model="searchStr" class="config-search-input" :placeholder="t('版本名称')" />
      </div>
      <section class="versions-wrapper">
        <section v-if="!searchStr" class="unnamed-version">
          <section
            :class="['version-item', { active: versionData.id === 0 }]"
            @click="handleSelectVersion(unNamedVersion)">
            <i class="bk-bscp-icon icon-edit-small edit-icon" />
            <div class="version-name">{{ t('未命名版本') }}</div>
          </section>
          <div class="divider"></div>
        </section>
        <!-- 待审批/待上线置顶软链 -->
        <section
          v-if="pendingApprovalVersion"
          :class="['approval-version version-item', { active: versionData.id === pendingApprovalVersion.id }]"
          @click="handleSelectVersion(pendingApprovalVersion)">
          <div class="status">
            {{ pendingApprovalVersion.status.strategy_status === 'pending_approval' ? $t('待审批') : $t('待上线') }}
          </div>
          <bk-overflow-title class="version-name" type="tips">
            {{ pendingApprovalVersion.spec.name }}
          </bk-overflow-title>
        </section>
        <section
          v-for="version in versionsInView"
          :key="version.id"
          :class="['version-item', { active: versionData.id === version.id }]"
          @click="handleSelectVersion(version)">
          <div :class="['dot', version.status.publish_status]"></div>
          <bk-overflow-title class="version-name" type="tips">
            {{ version.spec.name }}
          </bk-overflow-title>
          <div
            v-if="version.status.fully_released"
            :class="['all-tag', { 'full-release': version.status.fully_release }]"
            v-bk-tooltips="{
              content: version.status.fully_release ? t('当前线上全量版本') : t('历史全量上线过的版本'),
            }">
            ALL
          </div>
          <Ellipsis class="action-more-icon" @mouseenter="handlePopShow(version, $event)" @mouseleave="handlePopHide" />
        </section>
        <TableEmpty v-if="searchStr && versionsInView.length === 0" :is-search-empty="true" @clear="searchStr = ''" />
      </section>
    </bk-loading>
    <VersionDiff v-model:show="showDiffPanel" :current-version="currentOperatingVersion" />
    <VersionOperateConfirmDialog
      v-model:show="showOperateConfirmDialog"
      :title="t('确认废弃该版本')"
      :tips="t('此操作不会删除版本，如需找回或彻底删除请去版本详情的废弃版本列表操作')"
      :confirm-fn="handleDeprecateVersion"
      :version="currentOperatingVersion" />
    <div
      class="action-list"
      ref="popover"
      v-show="popShow"
      @mouseenter="handlePopContentMouseEnter"
      @mouseleave="handlePopContentMouseLeave">
      <div class="action-item" @click="handleDiffDialogShow(selectedVersion!)">{{ t('版本对比') }}</div>
      <div
        v-bk-tooltips="{
          disabled: !isDeprecateDisabled,
          placement: 'bottom',
          content: t('只支持未上线和未待审批版本'),
        }"
        :class="['action-item', { disabled: isDeprecateDisabled }]"
        @click="handleDeprecateDialogShow(selectedVersion!)">
        {{ t('版本废弃') }}
      </div>
    </div>
    <EditService
      v-model:show="isEditServicePopShow"
      :service="editingService"
      @reload="handleReloadService"></EditService>
  </section>
</template>
<script setup lang="ts">
  import { ref, onMounted, computed, watch } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { storeToRefs } from 'pinia';
  import { useI18n } from 'vue-i18n';
  import { Message } from 'bkui-vue';
  import { Ellipsis, AngleUpFill } from 'bkui-vue/lib/icon';
  import useConfigStore from '../../../../../../store/config';
  import { getConfigVersionList, deprecateVersion } from '../../../../../../api/config';
  import { GET_UNNAMED_VERSION_DATA } from '../../../../../../constants/config';
  import { IConfigVersion } from '../../../../../../../types/config';
  import { IAppItem } from '../../../../../../../types/app';
  import ServiceSelector from '../../../../../../components/service-selector.vue';
  import SearchInput from '../../../../../../components/search-input.vue';
  import TableEmpty from '../../../../../../components/table/table-empty.vue';
  import VersionDiff from '../../config/components/version-diff/index.vue';
  import VersionOperateConfirmDialog from './version-operate-confirm-dialog.vue';
  import EditService from '../../../list/components/edit-service.vue';

  const configStore = useConfigStore();
  const { versionData, refreshVersionListFlag, publishedVersionId } = storeToRefs(configStore);

  const route = useRoute();
  const router = useRouter();
  const { t } = useI18n();

  const props = defineProps<{
    bkBizId: string;
    appId: number;
  }>();

  const unNamedVersion: IConfigVersion = GET_UNNAMED_VERSION_DATA();
  const versionListLoading = ref(false);
  const versionList = ref<IConfigVersion[]>([]);
  const searchStr = ref('');
  const showDiffPanel = ref(false);
  const currentOperatingVersion = ref();
  const showOperateConfirmDialog = ref(false);
  const selectedVersion = ref<IConfigVersion>();
  const popShow = ref(false);
  const popover = ref<HTMLInputElement | null>(null);
  const popHideTimerId = ref(0);
  const isMouseenter = ref(false);
  const isEditServicePopShow = ref(false);
  const serviceSelectorRef = ref();
  const editingService = ref<IAppItem>({
    id: 0,
    biz_id: 0,
    space_id: '',
    spec: {
      name: '',
      config_type: '',
      memo: '',
      alias: '',
      data_type: '',
      is_approve: true,
      approver: '',
      approve_type: 'or_sign',
    },
    revision: {
      creator: '',
      reviser: '',
      create_at: '',
      update_at: '',
    },
    permissions: {},
  });

  const versionsInView = computed(() => {
    if (searchStr.value === '') {
      return versionList.value.slice(1);
    }
    return versionList.value.filter((item) => {
      const isNameMatched = item.spec.name.toLowerCase().includes(searchStr.value.toLocaleLowerCase());
      return item.id > 0 && isNameMatched;
    });
  });

  // 选择版本是否可废弃
  const isDeprecateDisabled = computed(() => {
    const { strategy_status, publish_status } = selectedVersion.value?.status || {};
    return (
      strategy_status === 'pending_approval' ||
      strategy_status === 'pending_publish' ||
      publish_status !== 'not_released'
    );
  });

  // 待审批状态版本
  const pendingApprovalVersion = computed(() => {
    return versionList.value.find(
      (item) => item.status.strategy_status === 'pending_publish' || item.status.strategy_status === 'pending_approval',
    );
  });

  // 监听刷新版本列表标识，处理新增版本场景，默认选中新增的版本
  watch(refreshVersionListFlag, async (val) => {
    if (val) {
      await getVersionList();
      let versionDetail;
      // 判断当前是生成版本还是上线版本
      if (publishedVersionId.value) {
        versionDetail = versionList.value.find((item) => item.id === publishedVersionId.value);
        publishedVersionId.value = 0;
      } else {
        versionDetail = versionList.value[1];
      }
      if (versionDetail) {
        versionData.value = versionDetail;
        refreshVersionListFlag.value = false;
        // 默认选中新增的版本时，路由参数versionId需要更新
        router.push({ name: route.name as string, params: { versionId: versionDetail.id } });
      }
    }
  });

  watch(
    () => props.appId,
    () => {
      init();
    },
  );

  onMounted(async () => {
    init();
  });

  const init = async () => {
    await getVersionList();
    if (pendingApprovalVersion.value) {
      versionData.value = pendingApprovalVersion.value;
      router.push({ name: route.name as string, params: { versionId: versionData.value.id } });
    }
    if (route.params.versionId) {
      const version = versionList.value.find((item) => item.id === Number(route.params.versionId));
      if (version) {
        versionData.value = version;
      }
    }
  };

  const getVersionList = async () => {
    try {
      versionListLoading.value = true;
      const params = {
        // 未命名版本不在实际的版本列表里，需要特殊处理
        start: 0,
        all: true,
      };
      const res = await getConfigVersionList(props.bkBizId, props.appId, params);
      versionList.value = [unNamedVersion, ...res.data.details];
      const index = versionList.value.findIndex((version: IConfigVersion) =>
        version.status.released_groups.some((group) => group.id === 0),
      );
      if (index > -1) versionList.value[index].status.fully_release = true;
    } catch (e) {
      console.error(e);
    } finally {
      versionListLoading.value = false;
    }
  };

  const refreshVersionApprovalStatus = async () => {
    try {
      const params = {
        // 未命名版本不在实际的版本列表里，需要特殊处理
        start: 0,
        all: true,
      };
      const res = await getConfigVersionList(props.bkBizId, props.appId, params);
      versionList.value.forEach((version: IConfigVersion) => {
        const newVersion = res.data.details.find((item: IConfigVersion) => item.id === version.id);
        if (newVersion) {
          version.status.strategy_status = newVersion.status.strategy_status;
        }
      });
    } catch (error) {}
  };

  const handleSelectVersion = (version: IConfigVersion) => {
    if (version.id === versionData.value.id) return;
    configStore.$patch((state) => {
      state.allExistConfigCount = 0;
      state.conflictFileCount = 0;
    });
    versionData.value = version;
    const params: { spaceId: string; appId: number; versionId?: number } = {
      spaceId: props.bkBizId,
      appId: props.appId,
    };
    if (version.id !== 0) {
      params.versionId = version.id;
    }
    refreshVersionApprovalStatus();
    router.push({ name: route.name as string, params });
    // 更新版本审批状态
  };

  const handleDiffDialogShow = (version: IConfigVersion) => {
    currentOperatingVersion.value = version;
    showDiffPanel.value = true;
  };

  const handleDeprecateDialogShow = (version: IConfigVersion) => {
    if (isDeprecateDisabled.value) {
      return;
    }
    currentOperatingVersion.value = version;
    showOperateConfirmDialog.value = true;
  };

  const handleDeprecateVersion = () =>
    new Promise(() => {
      const id = currentOperatingVersion.value.id;
      deprecateVersion(props.bkBizId, props.appId, id).then(() => {
        showOperateConfirmDialog.value = false;
        Message({
          theme: 'success',
          message: '版本废弃成功',
        });
        if (id !== versionData.value.id) {
          return;
        }

        const versions = versionsInView.value.filter((item) => item.id > 0);
        const index = versions.findIndex((item) => item.id === id);

        if (versions.length === 1) {
          handleSelectVersion(unNamedVersion);
        } else if (index === versions.length - 1) {
          handleSelectVersion(versions[index - 1]);
        } else {
          handleSelectVersion(versions[index + 1]);
        }

        versionList.value = versionList.value.filter((item) => item.id !== id);
      });
    });

  const handlePopShow = (version: IConfigVersion, event: any) => {
    selectedVersion.value = version;
    const element = event.target;
    const rect = element.getBoundingClientRect();
    const distanceToBottom = window.innerHeight - rect.bottom;
    if (distanceToBottom < 70) {
      popover.value!.style.top = `${rect.top - 140}px`;
    } else {
      popover.value!.style.top = `${rect.top - 20}px`;
    }
    popHideTimerId.value && clearTimeout(popHideTimerId.value);
    popShow.value = true;
  };

  const handlePopHide = () => {
    popHideTimerId.value = window.setTimeout(() => {
      popShow.value = false;
    }, 300);
  };

  const handlePopContentMouseEnter = () => {
    if (popHideTimerId.value) {
      isMouseenter.value = true;
      clearTimeout(popHideTimerId.value);
      popHideTimerId.value = 0;
    }
  };
  const handlePopContentMouseLeave = () => {
    if (isMouseenter.value) {
      handlePopHide();
      isMouseenter.value = false;
    }
  };

  const handleReloadService = () => {
    serviceSelectorRef.value.reloadService();
  };

  const handleAppChange = (service: IAppItem) => {
    editingService.value = service;
    configStore.$patch((state) => {
      state.conflictFileCount = 0;
      state.allConfigCount = 0;
      state.allExistConfigCount = 0;
    });
    let name = route.name as string;
    if (route.name === 'init-script' && service.spec.config_type === 'kv') {
      name = 'service-config';
    }

    router.push({ name, params: { spaceId: service.space_id, appId: service.id } });
  };
</script>

<style lang="scss" scoped>
  .version-container {
    height: 100%;
  }
  .service-selector-wrapper {
    display: flex;
    padding: 10px 8px 9px;
    width: 280px;
    border-bottom: 1px solid #eaebf0;
    :deep(.service-selector) {
      flex: 1;
    }
    .details-btn {
      width: 32px;
      height: 32px;
      background: #f0f1f5;
      border-radius: 2px;
      font-size: 14px;
      margin: 0 8px;
      text-align: center;
      line-height: 32px;
      color: #979ba5;
      cursor: pointer;
      &:hover {
        color: #3a84ff;
        background: #e1ecff;
      }
    }
    .selector-trigger {
      display: inline-flex;
      align-items: stretch;
      width: 100%;
      height: 32px;
      font-size: 12px;
      border-radius: 2px;
      transition: all 0.3s;
      & > input {
        flex: 1;
        width: 100%;
        padding: 0 24px 0 10px;
        line-height: 1;
        font-size: 14px;
        color: #313238;
        background: #f0f1f5;
        border-radius: 2px;
        border: none;
        outline: none;
        transition: all 0.3s;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        cursor: pointer;
      }
      .arrow-icon {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        position: absolute;
        right: 4px;
        top: 0;
        width: 20px;
        height: 100%;
        transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
        color: #979ba5;
        &.arrow-line {
          font-size: 20px;
        }
      }
    }
  }
  .bk-nested-loading {
    height: calc(100% - 52px);
  }
  .version-search-wrapper {
    padding: 8px 16px;
  }
  .versions-wrapper {
    position: relative;
    height: calc(100% - 48px);
    overflow: auto;
  }
  .version-steps {
    padding: 16px 0;
    overflow: auto;
  }

  .divider {
    margin: 8px 24px;
    border-bottom: 1px solid #dcdee5;
  }
  .version-item {
    position: relative;
    padding: 0 40px 0 48px;
    cursor: pointer;
    &.active {
      background: #e1ecff;
      .version-name {
        color: #3a84ff;
      }
    }
    &:hover {
      background: #e1ecff;
    }
    .edit-icon {
      position: absolute;
      top: 10px;
      left: 24px;
      font-size: 22px;
      color: #979ba5;
    }
    .dot {
      position: absolute;
      left: 28px;
      top: 16px;
      width: 8px;
      height: 8px;
      border-radius: 50%;
      border: 1px solid #c4c6cc;
      background: #f0f1f5;
      &.not_released {
        border: 1px solid #ff9c01;
        background: #ffe8c3;
      }
      &.full_released,
      &.partial_released {
        border: 1px solid #3fc06d;
        background: #e5f6ea;
      }
    }
    .all-tag {
      position: absolute;
      right: 53px;
      top: 0;
      transform: translateY(50%);
      width: 31px;
      height: 22px;
      font-size: 12px;
      background: #fafbfd;
      border: 1px solid #dcdee5;
      border-radius: 2px;
      color: #63656e;
      text-align: center;
      line-height: 22px;
      &.full-release {
        background: #e4faf0;
        border: 1px solid #14a5684d;
        border-radius: 2px;
        color: #14a568;
      }
    }
  }
  .approval-version {
    display: flex;
    align-items: center;
    padding-left: 16px;
    gap: 8px;
    background: #fdf4e8 !important;
    font-size: 12px;
    cursor: pointer;
    .status {
      width: 52px;
      height: 22px;
      background: #f59500;
      border-radius: 2px;
      color: #ffffff;
      line-height: 22px;
      text-align: center;
    }
  }
  .version-name {
    height: 42px;
    width: 140px;
    line-height: 42px;
    font-size: 12px;
    color: #313238;
    text-align: left;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  .action-more-icon {
    position: absolute;
    top: 10px;
    right: 10px;
    transform: rotate(90deg);
    width: 22px;
    height: 22px;
    color: #979ba5;
    border-radius: 50%;
    cursor: pointer;
    &:hover {
      background: rgba(99, 101, 110, 0.1);
      color: #3a84ff;
    }
  }
  .list-pagination {
    margin-top: 16px;
  }
  .action-list {
    position: absolute;
    right: 25px;
    padding: 4px 0;
    border: 1px solid #dcdee5;
    box-shadow: 0 2px 6px 0 #0000001a;
    background-color: #fff;
    border-radius: 4px;
    .action-item {
      padding: 0 12px;
      height: 32px;
      line-height: 32px;
      color: #63656e;
      font-size: 12px;
      cursor: pointer;
      &:hover {
        background: #f5f7fa;
      }
      &.disabled {
        color: #dcdee5;
        cursor: not-allowed;
      }
    }
  }
</style>
