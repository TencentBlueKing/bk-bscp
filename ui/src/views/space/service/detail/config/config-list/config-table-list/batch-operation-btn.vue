<template>
  <bk-popover
    ref="buttonRef"
    theme="light batch-operation-button-popover"
    placement="bottom-end"
    trigger="click"
    width="108"
    :arrow="false"
    @after-show="isPopoverOpen = true"
    @after-hidden="isPopoverOpen = false">
    <bk-button
      :disabled="props.selectedIds.length === 0 && !isAcrossChecked"
      :class="['batch-set-btn', { 'popover-open': isPopoverOpen }]">
      {{ t('批量操作') }}
      <AngleDown class="angle-icon" />
    </bk-button>
    <template #content>
      <div
        v-if="isFileType"
        :class="['operation-item', { disabled: selectedExistCount === 0 }]"
        @click="handleBatchOperation('edit')">
        {{ t('批量修改权限') }}
      </div>
      <div
        :class="['operation-item', { disabled: selectedExistCount === 0 }]"
        @click="handleBatchOperation('delete')">
        {{ t('批量删除') }}
      </div>
      <div
        :class="['operation-item', { disabled: selectedDeleteCount === 0 }]"
        @click="handleBatchOperation('undelete')">
        {{ t('批量恢复') }}
      </div>
    </template>
  </bk-popover>
  <DeleteConfirmDialog
    v-model:is-show="isBatchDeleteDialogShow"
    :title="
      t('确认删除所选的 {n} 项配置项？', {
        n: selectedExistCount,
      })
    "
    :pending="loading"
    @confirm="handleBatchDeleteConfirm">
    <div>
      {{
        t('已生成版本中存在的配置项，可以通过恢复按钮撤销删除，新增且未生成版本的配置项，将无法撤销删除，请谨慎操作。')
      }}
    </div>
  </DeleteConfirmDialog>
  <DeleteConfirmDialog
    v-model:is-show="isBatchUndeleteDialogShow"
    :title="
      t(isFileType ? '确认恢复所选的 {n} 个配置文件？' : '确认恢复所选的 {n} 个配置项？', {
        n: selectedDeleteCount,
      })
    "
    :pending="loading"
    :confirm-text="t('恢复')"
    @confirm="handleBatchUnDeleteConfirm">
    <div>
      {{
        isFileType
          ? t('如果所选待恢复的配置文件已存在，那么现有配置文件将被覆盖。')
          : t('如果所选待恢复的配置项已存在，那么现有配置项将被覆盖。')
      }}
    </div>
  </DeleteConfirmDialog>
  <EditPermissionDialog
    v-model:show="isBatchEditPermDialogShow"
    :loading="loading"
    :configs-length="props.selectedItems!.length"
    @confirm="handleConfimEditPermission" />
</template>
<script lang="ts" setup>
  import { ref } from 'vue';
  import { AngleDown } from 'bkui-vue/lib/icon';
  import { useI18n } from 'vue-i18n';
  import Message from 'bkui-vue/lib/message';
  import {
    batchDeleteServiceConfigs,
    batchDeleteKv,
    batchAddConfigList,
    batchUndeleteKv,
    batchUndeleteFile,
  } from '../../../../../../../api/config';
  import DeleteConfirmDialog from '../../../../../../../components/delete-confirm-dialog.vue';
  import EditPermissionDialog from '../../../../../templates/list/package-detail/operations/edit-permission/edit-permission-dialog.vue';
  import { IConfigItem } from '../../../../../../../../types/config';

  const { t } = useI18n();

  interface IPermissionType {
    privilege: string;
    user: string;
    user_group: string;
  }

  const props = defineProps<{
    bkBizId: string;
    appId: number;
    selectedIds: number[];
    selectedKeys: string[];
    isFileType: boolean; // 是否为文件型配置
    selectedItems?: IConfigItem[];
    isAcrossChecked: boolean;
    dataCount: number;
    selectedDeleteCount: number; // 选中的删除项
    selectedExistCount: number; // 选中的存在项
  }>();

  const emits = defineEmits(['deleted']);

  const loading = ref(false);
  const isBatchDeleteDialogShow = ref(false);
  const isBatchEditPermDialogShow = ref(false);
  const isBatchUndeleteDialogShow = ref(false);
  const isPopoverOpen = ref(false);
  const buttonRef = ref();

  const handleBatchDeleteConfirm = async () => {
    loading.value = true;
    try {
      if (props.isFileType) {
        await batchDeleteServiceConfigs(props.bkBizId, props.appId, props.selectedIds);
      } else {
        await batchDeleteKv(props.bkBizId, props.appId, props.selectedIds, props.isAcrossChecked);
      }
      Message({
        theme: 'success',
        message: props.isFileType ? t('批量删除配置文件成功') : t('批量删除配置项成功'),
      });
      isBatchDeleteDialogShow.value = false;
      setTimeout(() => {
        emits('deleted');
      }, 300);
    } catch (error) {
      console.error(error);
    } finally {
      loading.value = false;
    }
  };

  const handleBatchUnDeleteConfirm = async () => {
    loading.value = true;
    try {
      if (props.isFileType) {
        await batchUndeleteFile(props.bkBizId, props.appId, props.selectedIds);
      } else {
        await batchUndeleteKv(props.bkBizId, props.appId, props.selectedKeys, props.isAcrossChecked);
      }
      Message({
        theme: 'success',
        message: props.isFileType ? t('批量恢复配置文件成功') : t('批量恢复配置项成功'),
      });
      isBatchUndeleteDialogShow.value = false;
      setTimeout(() => {
        emits('deleted');
      }, 300);
    } catch (error) {
      console.error(error);
    } finally {
      loading.value = false;
    }
  };

  const handleBatchOperation = (type: string) => {
    if (type === 'delete') {
      if (props.selectedExistCount === 0) return;
      isBatchDeleteDialogShow.value = true;
    } else if (type === 'edit') {
      if (props.selectedExistCount === 0) return;
      isBatchEditPermDialogShow.value = true;
    } else if (type === 'undelete') {
      if (props.selectedDeleteCount === 0) return;
      isBatchUndeleteDialogShow.value = true;
    }
    buttonRef.value.hide();
  };

  const handleConfimEditPermission = async ({ permission }: { permission: IPermissionType }) => {
    try {
      loading.value = true;
      const { privilege, user, user_group } = permission;
      const editConfigList = props.selectedItems!.map((item) => {
        const { id, spec, commit_spec } = item;
        return {
          id,
          ...spec,
          privilege: privilege || spec.permission.privilege,
          user: user || spec.permission.user,
          user_group: user_group || spec.permission.user_group,
          byte_size: commit_spec.content.byte_size,
          sign: commit_spec.content.signature,
        };
      });
      await batchAddConfigList(props.bkBizId, props.appId, { items: editConfigList });
      Message({
        theme: 'success',
        message: t('配置文件权限批量修改成功'),
      });
      isBatchEditPermDialogShow.value = false;
    } catch (error) {
      console.error(error);
    } finally {
      loading.value = false;
    }
    emits('deleted');
  };
</script>

<style lang="scss" scoped>
  .batch-set-btn {
    min-width: 108px;
    height: 32px;
    &.popover-open {
      .angle-icon {
        transform: rotate(-180deg);
      }
    }
    .angle-icon {
      font-size: 20px;
      transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    }
  }
  .user-settings {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
  }
  .perm-input {
    display: flex;
    align-items: center;
    width: 172px;
    :deep(.bk-input) {
      width: 140px;
      border-right: none;
      border-top-right-radius: 0;
      border-bottom-right-radius: 0;
      .bk-input--number-control {
        display: none;
      }
    }
    .perm-panel-trigger {
      width: 32px;
      height: 32px;
      text-align: center;
      background: #fafcfe;
      color: #3a84ff;
      border: 1px solid #3a84ff;
      cursor: pointer;
      &.disabled {
        color: #dcdee5;
        border-color: #dcdee5;
        cursor: not-allowed;
      }
    }
  }
  .privilege-select-panel {
    display: flex;
    align-items: top;
    border: 1px solid #dcdee5;
    .group-item {
      .header {
        padding: 0 16px;
        height: 42px;
        line-height: 42px;
        color: #313238;
        font-size: 12px;
        background: #fafbfd;
        border-bottom: 1px solid #dcdee5;
      }
      &:not(:last-of-type) {
        .header,
        .checkbox-area {
          border-right: 1px solid #dcdee5;
        }
      }
    }
    .checkbox-area {
      padding: 10px 16px 12px;
      background: #ffffff;
      &:not(:last-child) {
        border-right: 1px solid #dcdee5;
      }
    }
    .group-checkboxs {
      font-size: 12px;
      .bk-checkbox ~ .bk-checkbox {
        margin-left: 16px;
      }
      :deep(.bk-checkbox-label) {
        font-size: 12px;
      }
    }
  }
  .selected-tag {
    display: inline-block;
    height: 32px;
    background: #f0f1f5;
    line-height: 32px;
    border-radius: 16px;
    padding: 0 12px;
    margin: 8px 0px 16px;
    .count {
      color: #3a84ff;
    }
  }
</style>

<style lang="scss">
  .batch-operation-button-popover.bk-popover.bk-pop2-content {
    padding: 4px 0;
    border: 1px solid #dcdee5;
    box-shadow: 0 2px 6px 0 #0000001a;
    .operation-item {
      padding: 0 12px;
      min-width: 58px;
      height: 32px;
      line-height: 32px;
      color: #63656e;
      font-size: 12px;
      cursor: pointer;
      &:hover {
        background: #f5f7fa;
      }
      &.disabled {
        color: #c4c6cc;
        cursor: not-allowed;
      }
    }
  }
</style>
