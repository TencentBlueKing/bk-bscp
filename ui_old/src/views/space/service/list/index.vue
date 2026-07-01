<template>
  <div class="service-wrap">
    <ServiceListContent
      :space-id="spaceId"
      :perm-check-loading="permCheckLoading"
      :has-create-service-perm="hasCreateServicePerm" />
    <AppFooter />
  </div>
</template>
<script setup lang="ts">
  import { ref, watch, onMounted } from 'vue';
  import { storeToRefs } from 'pinia';
  import useGlobalStore from '../../../../store/global';
  import { permissionCheck } from '../../../../api/index';

  import ServiceListContent from './components/service-list-content.vue';
  import AppFooter from '../../../../components/footer.vue';

  const { spaceId } = storeToRefs(useGlobalStore());

  const hasCreateServicePerm = ref(false);
  const permCheckLoading = ref(false);

  watch(
    () => spaceId.value,
    () => {
      checkCreateServicePerm();
    },
  );

  onMounted(() => {
    checkCreateServicePerm();
    // 访问服务管理列表页时，清空上次访问服务记录
    localStorage.removeItem('lastAccessedServiceDetail');
  });

  const checkCreateServicePerm = async () => {
    permCheckLoading.value = true;
    const res = await permissionCheck({
      resources: [
        {
          biz_id: spaceId.value,
          basic: {
            type: 'app',
            action: 'create',
          },
        },
      ],
    });
    hasCreateServicePerm.value = res.is_allowed;
    permCheckLoading.value = false;
  };
</script>

<style lang="scss" scoped>
  .service-wrap {
    display: flex;
    flex-direction: column;
    background: #f5f7fa;
    height: calc(100vh - 52px);
    width: 100%;
  }
</style>
