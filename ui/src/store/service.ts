// 服务实例的pinia数据
import { ref, computed } from 'vue';
import { defineStore, storeToRefs } from 'pinia';
import useGlobalStore from './global';

interface IAppData {
  id: number | string;
  env_id: string;
  spec: {
    name: string;
    config_type: string;
    data_type?: string;
  };
}
const { spaceId, projectId, permissionQuery, showApplyPermDialog } = storeToRefs(useGlobalStore());

export default defineStore('service', () => {
  // 服务详情数据
  const appData = ref<IAppData>({
    id: '',
    env_id: '',
    spec: {
      name: '',
      config_type: 'file',
    },
  });
  const permCheckLoading = ref(false);
  const hasEditServicePerm = ref(false);

  const isFileType = computed(() => appData.value.spec.config_type === 'file');

  const checkPermBeforeOperate = (perm: string) => {
    if (perm === 'update' && !hasEditServicePerm.value) {
      permissionQuery.value = {
        resources: [
          {
            biz_id: spaceId.value,
            basic: {
              type: 'app',
              action: 'update',
              resource_id: appData.value.id,
            },
            ...(+projectId.value > 0 ? { project_id: +projectId.value } : {}),
            ...(+appData.value.env_id > 0 ? { environment_id: +appData.value.env_id } : {}),
          },
        ],
      };
      showApplyPermDialog.value = true;
      return false;
    }
    return true;
  };

  // 保留新建文件用户输入文件权限
  const lastCreatePermission = ref({
    privilege: '644',
    user: 'root',
    user_group: 'root',
  });

  // 置顶的ids
  const topIds = ref<number[]>([]);

  return {
    appData,
    permCheckLoading,
    hasEditServicePerm,
    checkPermBeforeOperate,
    lastCreatePermission,
    topIds,
    isFileType,
  };
});
