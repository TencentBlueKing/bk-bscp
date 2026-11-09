<template>
  <EnvAlertBar
    :model-value="envId"
    @change="handleEnvChange" />
  <div class="service-wrap">
    <ServiceListContent
      :space-id="spaceId"
      :env-id="envId"
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
  import { IEnvItem } from '../../../../../types/env';

  import ServiceListContent from './components/service-list-content.vue';
  import AppFooter from '../../../../components/footer.vue';
  import EnvAlertBar from '../../../../components/env-alert-bar.vue';
  import { useRoute, useRouter } from 'vue-router';

  const { spaceId, projectId } = storeToRefs(useGlobalStore());

  const hasCreateServicePerm = ref(false);
  const permCheckLoading = ref(false);

  const router = useRouter();
  const route = useRoute();
  const envId = ref(String(route.query?.envId || ''));
  // 环境切换
  const handleEnvChange = (env: IEnvItem) => {
    envId.value = String(env.id);
    router.replace({
      query: {
        envId: String(env.id)
      }
    });
  };


  // 项目、环境切换或环境异步就绪后需重新判权，保证请求携带当前有效维度
  watch(
    () => [spaceId.value, projectId.value, envId.value],
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
    // 项目或环境 ID 未就绪时不发起请求，避免携带无效维度（+id > 0 同时排除空串、NaN、0、负数）
    if (!(+projectId.value > 0) || !(+envId.value > 0)) {
      hasCreateServicePerm.value = false;
      return;
    }
    permCheckLoading.value = true;
    const res = await permissionCheck({
      resources: [
        {
          biz_id: spaceId.value,
          basic: {
            type: 'app',
            action: 'create',
          },
          ...(+projectId.value > 0 ? { project_id: +projectId.value } : {}),
          ...(+envId.value > 0 ? { environment_id: +envId.value } : {}),
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
    height: calc(100% - 36px);
    width: 100%;
  }
</style>
