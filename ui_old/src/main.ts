import { createApp } from 'vue';
import pinia from './store/index';
import bkui, { bkTooltips, bkEllipsis, overflowTitle } from 'bkui-vue';
import 'bkui-vue/dist/style.css';
import './css/style.scss';
import App from './App.vue';
import router from './router';
import './utils/login';
import i18n from './i18n/index';
import cursor from './components/permission/cursor';
import './components/permission/cursor.css';
import auth from './common/auth';
import zhCn from 'bkui-vue/dist/locale/zh-cn.esm';
import en from 'bkui-vue/dist/locale/en.esm';
import { getCookie } from './utils';
import 'vue-virtual-scroller/dist/vue-virtual-scroller.css';
import VirtualScroller from 'vue-virtual-scroller';
import VxeUIAll from 'vxe-pc-ui';
import 'vxe-pc-ui/lib/style.css';
import VxeUITable from 'vxe-table';
import 'vxe-table/lib/style.css';
import useGlobalStore from './store/global';
import { PrimaryTable, TableColumn } from '@blueking/tdesign-ui';
import '@blueking/tdesign-ui/vue3/index.css';

import BkUserDisplayName from '@blueking/bk-user-display-name';

auth().then((userInfo) => {
  BkUserDisplayName.configure({
    // 必填，租户 ID
    tenantId: userInfo.tenant_id,
    // 必填，网关地址
    apiBaseUrl: `${(window as any).USER_MAN_HOST}`,
    // 可选，缓存时间，单位为毫秒, 默认 5 分钟, 只对单一用户查询有效
    cacheDuration: 1000 * 60 * 5,
    // 可选，当输入为空时，显示的文本，默认为 '--'
    emptyText: '--',
  });
  const app = createApp(App);
  app.directive('bkTooltips', bkTooltips);
  app.directive('bkEllipsis', bkEllipsis);
  app.directive('overflowTitle', overflowTitle);
  app.directive('cursor', cursor);
  app.directive('clickOutside', {
    mounted(el, binding) {
      const handleClickOutside = (event: any) => {
        if (!el.contains(event.target) && el !== event.target) {
          binding.value(event);
        }
      };
      setTimeout(() => {
        document.addEventListener('click', handleClickOutside);
        el.clickOutsideHandler = handleClickOutside;
      }, 0);
    },
    unmounted(el) {
      document.removeEventListener('click', el.clickOutsideHandler);
      delete el.clickOutsideHandler;
    },
  });

  app
    .use(pinia)
    .use(i18n)
    .use(router)
    .use(VirtualScroller)
    .use(VxeUIAll)
    .use(VxeUITable)
    .use(bkui, {
      locale: getCookie('blueking_language') === 'en' ? en : zhCn,
    })
    .component('PrimaryTable', PrimaryTable)
    .component('TableColumn', TableColumn)
    .mount('#app');
});

// 监听登录成功页通过postMessage发送的消息，刷新当前页面
window.addEventListener('message', (event) => {
  const globalStore = useGlobalStore(pinia);
  if (event.origin === globalStore.loginOriginUrl) {
    if (event.data === 'login') {
      console.log('login');
      window.location.reload();
    }
  }
});
