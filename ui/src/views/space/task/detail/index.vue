<template>
  <DetailLayout :name="$t('任务详情')" :show-footer="false" @close="handleClose">
    <template #header-suffix>
      <bk-tag type="filled" :theme="suffix.theme">
        {{ suffix.text }}
      </bk-tag>
    </template>
    <template #content>
      <div class="detail-content">
        <Info />
        <DetailList @change="handleChange" />
      </div>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { TASK_DETAIL_STATUS_MAP } from '../../../../constants/task';
  import { useRouter } from 'vue-router';
  import DetailLayout from '../../scripts/components/detail-layout.vue';
  import Info from './info.vue';
  import DetailList from './detail-list.vue';

  const router = useRouter();
  const suffix = ref({
    text: '',
    theme: '',
  });

  const handleChange = (status: string) => {
    suffix.value.text = TASK_DETAIL_STATUS_MAP[status as keyof typeof TASK_DETAIL_STATUS_MAP];
    if (status === 'SUCCESS') {
      suffix.value.theme = 'success';
    } else if (status === 'FAILURE') {
      suffix.value.theme = 'danger';
    } else {
      suffix.value.theme = 'info';
    }
  };

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
