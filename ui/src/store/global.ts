// 应用全部的pinia数据
import { ref } from 'vue';
import { defineStore } from 'pinia';
import { getPlatformConfig, setShortcutIcon, setDocumentTitle } from '@blueking/platform-config';
import { localT } from '../i18n';
import { ISpaceDetail, IPermissionResource, IPermissionQueryResourceItem } from '../../types/index';

export default defineStore('global', () => {
  const bscpVersion = ref(''); // 产品版本号
  const spaceId = ref(''); // 空间id
  const spaceFeatureFlags = ref<{ [key: string]: any }>({}); // 空间的配置开关
  const spaceList = ref<ISpaceDetail[]>([]);
  // @ts-ignore
  const showNotice = ref(false); // 是否展示消息通知
  const showApplyPermDialog = ref(false); // 资源无权限申请弹窗
  const showPermApplyPage = ref(false); // 无业务查看权限时，申请页面
  const applyPermUrl = ref(''); // 跳转到权限中心的申请链接
  const applyPermResource = ref<IPermissionResource[]>([]); // 无权限提示页的action
  const loginOriginUrl = ref(''); // 登录页的origin url
  const permissionQuery = ref<{ resources: IPermissionQueryResourceItem[] }>({
    resources: [],
  });
  const appGlobalConfig = ref({
    name: '',
    nameEn: '',
    appLogo: '',
    favicon: '',
    brandName: '',
    brandNameEn: '',
    footerCopyrightContent: '',
    i18n: {
      name: '',
      brandName: '',
      footerInfoHTML: '',
    },
  });

  const getAppGlobalConfig = async () => {
    const defaultConfig = {
      name: '服务配置中心',
      nameEn: 'BSCP',
      appLogo: '',
      favicon: `${(window as any).BK_STATIC_URL}/favicon.ico`,
      brandName: '蓝鲸智云',
      brandNameEn: 'Tencent BlueKing',
      footerCopyrightContent: '',
      i18n: {
        name: localT('服务配置中心'),
        brandName: localT('蓝鲸智云'),
        footerInfoHTML: `<a href="https://wpa1.qq.com/KziXGWJs?_type=wpa&qidian=true" target="_blank">${localT('技术支持')}</a> |
        <a href="https://bk.tencent.com/s-mart/community/" target="_blank">${localT('社区论坛')}</a> |
        <a href="https://bk.tencent.com/index/" target="_blank">${localT('产品官网')}</a>`,
      },
      version: bscpVersion.value,
    };
    if ((window as any).BK_SHARED_RES_BASE_JS_URL) {
      const config = await getPlatformConfig((window as any).BK_SHARED_RES_BASE_JS_URL, defaultConfig);
      appGlobalConfig.value = config;
    } else {
      appGlobalConfig.value = defaultConfig;
    }
    setShortcutIcon(appGlobalConfig.value.favicon);
    setDocumentTitle(appGlobalConfig.value.i18n);
  };

  return {
    bscpVersion,
    spaceId,
    spaceFeatureFlags,
    spaceList,
    showNotice,
    showApplyPermDialog,
    showPermApplyPage,
    applyPermUrl,
    applyPermResource,
    permissionQuery,
    appGlobalConfig,
    getAppGlobalConfig,
    loginOriginUrl,
  };
});
