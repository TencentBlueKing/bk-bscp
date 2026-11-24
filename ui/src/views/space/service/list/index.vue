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
  import { getCreateServicePermResources } from '../../../../utils/permission';
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

  // 同模块内切换业务/项目时组件实例被复用，setup 不会重跑，envId 仍是上一个项目的环境 ID。
  // 先清空：新项目无可用环境时选择器不会触发 change，若不清空，服务列表会一直展示上一个项目的数据；
  // 新项目有可用环境时，选择器会自动选中并 change 回写新的 envId。
  watch(
    () => `${String(route.params.spaceId || '')}/${String(route.params.projectId || '')}`,
    () => {
      envId.value = '';
    },
  );

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
    // 项目 ID 未就绪时不发起请求，避免携带无效维度（+id > 0 同时排除空串、NaN、0、负数）
    // 没有可见环境时 envId 为空，仍按项目判权，由后端补齐该项目的默认环境
    if (!(+projectId.value > 0)) {
      hasCreateServicePerm.value = false;
      return;
    }
    permCheckLoading.value = true;
    try {
      const res = await permissionCheck({
        resources: getCreateServicePermResources(spaceId.value, projectId.value, envId.value),
      });
      // 没有可选环境时即使有权限也无法创建，按无权限处理并引导申请
      hasCreateServicePerm.value = res.is_allowed && +envId.value > 0;
    } catch (e) {
      console.error(e);
      hasCreateServicePerm.value = false;
    } finally {
      permCheckLoading.value = false;
    }
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
