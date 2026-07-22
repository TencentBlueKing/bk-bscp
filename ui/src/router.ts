import { createRouter, createWebHistory } from 'vue-router';
import useGlobalStore from './store/global';
import { ISpaceDetail } from '../types/index';
import { getSpaceFeatureFlag } from './api';
import { storeToRefs } from 'pinia';
import { hasProjectConcept, getDefaultProjectId, getSpaceToProjectId } from './utils/project';

const routes = [
  {
    path: '/',
    name: 'home',
    redirect: () => {
      // 访问首页，默认跳到服务管理列表页
      // 优先取localstorage里存的上次访问的空间id 判断空间是否存在 是否有权限
      // 不存在时取空间列表中第一个有权限的空间
      // 仍不存在时取空间列表中第一个空间
      let spaceId = localStorage.getItem('lastAccessedSpace');
      const { spaceList } = useGlobalStore();
      const firstHasPermSpace = spaceList.find((item: ISpaceDetail) => item.permission);
      const hasPermSpace = spaceList.find((item: ISpaceDetail) => item.space_id === spaceId && item.permission);
      spaceId = hasPermSpace ? spaceId : (firstHasPermSpace?.space_id ?? spaceList[0]?.space_id) || '';

      // 服务管理有项目概念
      const params: any = { spaceId };
      const lastProjectId = getSpaceToProjectId(spaceId as string);
      if (lastProjectId) {
        params.projectId = lastProjectId;
      }

      return { name: 'service-all', params };
    },
  },
  {
    path: '/space/:spaceId/:projectId?',
    name: 'space',
    component: () => import('./views/space/index.vue'),
    children: [
      {
        path: 'service',
        children: [
          {
            path: 'all',
            name: 'service-all',
            meta: {
              navModule: 'service',
            },
            component: () => import('./views/space/service/list/index.vue'),
          },
          {
            path: ':appId(\\d+)/:envId',
            component: () => import('./views/space/service/detail/index.vue'),
            children: [
              {
                path: 'config/:versionId?',
                name: 'service-config',
                meta: {
                  navModule: 'service',
                },
                component: () => import('./views/space/service/detail/config/index.vue'),
              },
              {
                path: 'script/:versionId?',
                name: 'init-script',
                meta: {
                  navModule: 'service',
                },
                component: () => import('./views/space/service/detail/init-script/index.vue'),
              },
            ],
          },
        ],
      },
      {
        path: 'groups',
        name: 'groups-management',
        meta: {
          navModule: 'groups',
        },
        component: () => import('./views/space/groups/index.vue'),
      },
      {
        path: 'variables',
        name: 'variables-management',
        meta: {
          navModule: 'variables',
        },
        component: () => import('./views/space/variables/index.vue'),
      },
      {
        path: 'templates',
        meta: {
          navModule: 'templates',
        },
        children: [
          {
            path: 'list/:templateSpaceId?/:packageId?',
            name: 'templates-list',
            meta: {
              navModule: 'templates',
            },
            component: () => import('./views/space/templates/list/index.vue'),
          },
          {
            path: ':templateSpaceId/:packageId/version_manage/:templateId',
            name: 'template-version-manage',
            meta: {
              navModule: 'templates',
            },
            component: () => import('./views/space/templates/version-manage/index.vue'),
          },
        ],
      },
      {
        path: 'scripts',
        name: 'scripts-management',
        meta: {
          navModule: 'scripts',
        },
        component: () => import('./views/space/scripts/index.vue'),
        children: [
          {
            path: 'list',
            name: 'script-list',
            meta: {
              navModule: 'scripts',
            },
            component: () => import('./views/space/scripts/list/script-list.vue'),
          },
          {
            path: 'version_manage/:scriptId',
            name: 'script-version-manage',
            meta: {
              navModule: 'scripts',
            },
            component: () => import('./views/space/scripts/version-manage/index.vue'),
          },
        ],
      },
      {
        path: 'client_statistics/:envId?/:appId?',
        name: 'client-statistics',
        meta: {
          navModule: 'client-statistics',
        },
        component: () => import('./views/space/client/statistics/index.vue'),
      },
      {
        path: 'client_search/:envId?/:appId?',
        name: 'client-search',
        meta: {
          navModule: 'client-search',
        },
        component: () => import('./views/space/client/search/index.vue'),
      },
      {
        path: 'client_credentials',
        name: 'credentials-management',
        meta: {
          navModule: 'credentials',
        },
        component: () => import('./views/space/credentials/index.vue'),
      },
      {
        path: 'configuration_example/:envId?/:appId?',
        name: 'configuration-example',
        meta: {
          navModule: 'example',
        },
        component: () => import('./views/space/client/example/index.vue'),
      },
      {
        path: 'records/:envId?',
        children: [
          {
            path: 'all',
            name: 'records-all',
            component: () => import('./views/space/records/index.vue'),
            meta: {
              navModule: 'records',
            },
          },
          {
            path: ':appId(\\d+)',
            name: 'records-app',
            component: () => import('./views/space/records/index.vue'),
            meta: {
              navModule: 'records',
            },
          },
        ],
      },
      {
        path: 'env-manage',
        name: 'env-manage',
        meta: {
          navModule: 'env-manage',
        },
        component: () => import('./views/space/env-manage/index.vue'),
      },
    ],
  },
  {
    path: '/space/:spaceId',
    name: 'sapce',
    component: () => import('./views/space/index.vue'),
    children: [
      {
        path: 'task',
        name: 'task-history',
        meta: {
          navModule: 'task',
          permission: 'process_config_view',
        },
        component: () => import('./views/space/task/index.vue'),
        children: [
          {
            path: 'list',
            name: 'task-list',
            meta: {
              navModule: 'task',
              permission: 'process_config_view',
            },
            component: () => import('./views/space/task/list/tast-list.vue'),
          },
          {
            path: 'detail/:taskId',
            name: 'task-detail',
            meta: {
              navModule: 'task',
              permission: 'process_config_view',
            },
            component: () => import('./views/space/task/detail/index.vue'),
          },
        ],
      },
      {
        path: 'process',
        name: 'process-management',
        meta: {
          navModule: 'process',
          permission: 'process_config_view',
        },
        component: () => import('./views/space/process/index.vue'),
      },
      {
        path: 'config-template',
        name: 'config-template-management',
        meta: {
          navModule: 'config-template',
          permission: 'process_config_view',
        },
        component: () => import('./views/space/config-template/index.vue'),
        children: [
          {
            path: 'list',
            name: 'config-template-list',
            meta: {
              navModule: 'config-template',
              permission: 'process_config_view',
            },
            component: () => import('./views/space/config-template/list/config-template-list.vue'),
          },
          {
            path: ':configTemplateId/version-manage/:templateSpaceId/:templateId',
            name: 'config-template-version-manage',
            meta: {
              navModule: 'config-template',
              permission: 'process_config_view',
            },
            component: () => import('./views/space/config-template/version-manage/index.vue'),
          },
          {
            path: 'config-issued',
            name: 'config-issued',
            meta: {
              navModule: 'config-template',
              permission: 'process_config_view',
            },
            component: () => import('./views/space/config-template/config-issued/index.vue'),
          },
        ],
      },
      {
        path: 'project-manage',
        name: 'project-manage',
        meta: {
          navModule: 'project-manage',
        },
        component: () => import('./views/space/project-manage/index.vue'),
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('./views/404.vue'),
  },
];

const router = createRouter({
  history: createWebHistory((window as any).SITE_URL),
  routes,
});

// 路由切换时，取消无权限页面
router.afterEach(() => {
  const globalStore = useGlobalStore();
  globalStore.$patch((state) => {
    state.showPermApplyPage = false;
  });
});

router.beforeEach(async (to, _from, next) => {
  const globalStore = storeToRefs(useGlobalStore());
  const { spaceFeatureFlags } = globalStore;

  // 页面刷新后 spaceFeatureFlags会重置，重新获取权限信息
  if (!spaceFeatureFlags.value.BIZ_VIEW) {
    const res = await getSpaceFeatureFlag(to.params.spaceId as string);
    spaceFeatureFlags.value = res;
  }

  // 处理有项目概念路由的 projectId
  if (to.params.spaceId && hasProjectConcept(to.meta?.navModule as string | undefined)) {
    const currentSpaceId = to.params.spaceId as string;
    const currentProjectId = to.params.projectId as string | undefined;

    // 路由中缺少 projectId，需要补充
    if (!currentProjectId) {
      try {
        // 获取默认 projectId：优先 localStorage，没有则取项目列表第一个
        const targetProjectId = await getDefaultProjectId(currentSpaceId);
        if (targetProjectId) {
          // 重定向到带 projectId 的路由
          const params: Record<string, string> = { ...to.params, projectId: targetProjectId };
          next({
            name: to.name as string,
            params,
            query: to.query,
          });
          return;
        }
      } catch (error) {
        console.error('获取项目列表失败', error);
      }
    }
  }

  const permissions = to.matched.map((record) => record.meta?.permission).filter(Boolean);

  if (!permissions.length) {
    next();
    return;
  }

  const hasPermission = permissions.every((perm) => {
    switch (perm) {
      case 'process_config_view':
        return spaceFeatureFlags.value.PROCESS_CONFIG_VIEW;
      default:
        return true;
    }
  });

  if (!hasPermission) {
    next('/');
    return;
  }

  next();
});

export default router;
