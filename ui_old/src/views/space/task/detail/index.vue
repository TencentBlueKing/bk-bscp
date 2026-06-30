<template>
  <DetailLayout :name="$t('任务详情')" :show-footer="false" @close="handleClose">
    <template #header-suffix>
      <bk-tag type="filled" :theme="suffix.theme">
        {{ suffix.text }}
      </bk-tag>
    </template>
    <template #content>
      <div class="detail-content">
        <Info :task-detail="taskDetail" />
        <DetailList />
      </div>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import { ref, watch } from 'vue';
  import { TASK_STATUS_MAP } from '../../../../constants/task';
  import { useRouter } from 'vue-router';
  import { storeToRefs } from 'pinia';
  import DetailLayout from '../../scripts/components/detail-layout.vue';
  import Info from './info.vue';
  import DetailList from './detail-list.vue';
  import useTaskStore from '../../../../store/task';
  const { taskDetail } = storeToRefs(useTaskStore());

  const router = useRouter();
  const suffix = ref({
    text: '',
    theme: '',
  });

  watch(
    () => taskDetail.value.status,
    () => {
      suffix.value.text = TASK_STATUS_MAP[taskDetail.value.status as keyof typeof TASK_STATUS_MAP];
      if (taskDetail.value.status === 'succeed') {
        suffix.value.theme = 'success';
      } else if (taskDetail.value.status === 'failed') {
        suffix.value.theme = 'danger';
      } else {
        suffix.value.theme = 'info';
      }
    },
    { immediate: true },
  );

  const handleClose = () => {
    router.push({ name: 'task-list' });
  };
</script>

<style scoped lang="scss">
  .detail-content {
    background: #f5f7fa;
    height: 100%;
  }
</style>
