<template>
  <div class="more-options">
    <Ellipsis class="ellipsis" />
    <ul class="more-options-ul">
      <li class="more-options-li" @click="handleLinkTo">{{ $t('服务上线记录') }}</li>
      <bk-loading :loading="loading">
        <li
          class="more-options-li"
          v-if="
            [APPROVE_STATUS.pending_approval, APPROVE_STATUS.pending_publish].includes(
              props.approveStatus as APPROVE_STATUS,
            )
          "
          @click="handleConfirm">
          {{ $t('撤销上线') }}
        </li>
      </bk-loading>
    </ul>
    <!-- 撤销弹窗 -->
    <DialogConfirm
      v-model:show="confirmShow"
      :space-id="String(route.params.spaceId)"
      :app-id="Number(route.params.appId)"
      :release-id="Number(route.params.versionId)"
      :data="confirmData"
      @refresh-list="handleUndo" />
  </div>
</template>

<script setup lang="ts">
  import { ref } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { storeToRefs } from 'pinia';
  import useConfigStore from '../../../../../store/config';
  import { Ellipsis } from 'bkui-vue/lib/icon';
  import { APPROVE_STATUS } from '../../../../../constants/record';
  import { IDialogData } from '../../../../../../types/record';
  import DialogConfirm from '../../../records/components/dialog-confirm.vue';
  import useServiceStore from '../../../../../store/service';

  const versionStore = useConfigStore();
  const { versionData, publishedVersionId } = storeToRefs(versionStore);
  const { appData } = storeToRefs(useServiceStore());

  const props = withDefaults(
    defineProps<{
      approveStatus: string;
      targetGroups: [];
    }>(),
    {
      approveStatus: '',
      targetGroups: () => [],
    },
  );

  const emits = defineEmits(['handleUndo']);

  const route = useRoute();
  const router = useRouter();

  const loading = ref(false);
  const confirmShow = ref(false);
  const confirmData = ref<IDialogData>({
    service: '',
    version: '',
    group: '',
    serviceId: 0,
    releaseId: 0,
    memo: '',
  });

  // 跳转到服务记录页面
  const handleLinkTo = () => {
    const url = router.resolve({
      name: 'records-app',
      query: {
        action: 'publish_release_config',
      },
      params: {
        appId: route.params.appId,
      },
    }).href;
    window.open(url, '_blank');
  };

  // 撤回提示框
  const handleConfirm = async () => {
    confirmShow.value = true;
    const group = props.targetGroups.map((item: any) => item.spec.name).join(',');
    confirmData.value = {
      service: appData.value.spec.name || '--',
      version: versionData.value.spec?.name || '--',
      group: group.length ? group : '--',
      serviceId: 0,
      releaseId: 0,
      memo: '',
    };
  };

  // 撤销审批
  const handleUndo = async () => {
    publishedVersionId.value = versionData.value.id;
    emits('handleUndo', Number(route.params.release_id));
  };
</script>

<style lang="scss" scoped>
  .more-options {
    box-sizing: content-box;
    position: relative;
    margin: 0 -12px 0 0;
    width: 32px;
    height: 32px;
    cursor: pointer;
    &:hover {
      .more-options-ul {
        display: block;
      }
      .ellipsis {
        color: #3a84ff;
      }
      &::after {
        background-color: #dcdee5;
      }
    }
    &::after {
      content: '';
      position: absolute;
      left: 50%;
      top: 50%;
      transform: translate(-50%, -50%);
      width: 20px;
      height: 20px;
      border-radius: 50%;
    }
    .ellipsis {
      position: absolute;
      z-index: 1;
      top: 50%;
      left: 50%;
      transform: translate(-50%, -50%) rotate(90deg);
      font-size: 16px;
      font-weight: 700;
      color: #9a9fa9;
    }
  }
  .more-options-ul {
    position: absolute;
    z-index: 1;
    right: 7px;
    top: 32px;
    display: none;
    border: 1px solid #dcdee5;
    border-radius: 2px;
    box-shadow: 0 2px 6px 0 #0000001a;
  }
  .more-options-li {
    padding: 0 12px;
    min-width: 96px;
    line-height: 32px;
    font-size: 12px;
    white-space: nowrap;
    color: #63656e;
    background-color: #fff;
    &:hover {
      background-color: #f5f7fa;
    }
  }
</style>
