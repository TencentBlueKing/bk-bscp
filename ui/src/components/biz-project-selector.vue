<template>
  <div class="biz-project-selector" ref="selectorRef">
    <!-- 触发器 -->
    <div class="selector-trigger" @click="togglePanel">
      <input readonly :value="displayText" :placeholder="t('请选择业务/项目')" />
      <AngleDown class="arrow-icon" :class="{ 'is-open': isOpen }" />
    </div>

    <!-- 下拉面板 -->
    <Transition name="panel-fade">
      <div v-if="isOpen" :class="['selector-panel', { 'single-column': !showProject }]">
        <!-- 主体：左右分栏 或 单栏 -->
        <div class="panel-body">
          <!-- 左侧：业务列表 -->
          <div :class="['panel-column', 'biz-column', { 'full-width': !showProject }]">
            <div class="column-search-wrapper">
              <div class="column-search">
                <Search class="search-icon" />
                <input
                  v-model="bizSearch"
                  type="text"
                  :placeholder="t('搜索业务')"
                  @click.stop />
              </div>
            </div>
            <div class="column-list">
              <RecycleScroller
                v-if="filteredBizList.length > 0"
                :items="filteredBizList"
                :item-size="36"
                key-field="space_id"
                class="virtual-scroller"
                v-slot="{ item }">
                <div
                  v-cursor="{ active: !item.permission }"
                  :class="['column-item', { active: (tempSelectedBizId || selectedBizId) === item.space_id }]"
                  @click="handleSelectBiz($event, item)">
                  <div class="name-wrapper">
                    <span v-overflow-title class="name">{{ item.space_name }}</span>
                    <span class="id">({{ item.space_id }})</span>
                  </div>
                  <!-- 只有显示项目时才显示箭头 -->
                  <AngleRight v-if="showProject" class="arrow" />
                </div>
              </RecycleScroller>
              <div v-else class="column-empty">
                {{ t('暂无数据') }}
              </div>
            </div>
          </div>

          <!-- 右侧：项目列表（仅当 showProject=true 时显示） -->
          <div v-if="showProject" class="panel-column project-column">
            <div class="column-search-wrapper">
              <div class="column-search">
                <Search class="search-icon" />
                <input
                  v-model="projectSearch"
                  type="text"
                  :placeholder="t('搜索项目')"
                  @click.stop />
              </div>
            </div>
            <div class="column-list">
              <!-- Loading 状态 -->
              <div v-if="projectLoading" class="column-loading">
                <Loading class="loading-icon" />
                <span>{{ t('加载中') }}</span>
              </div>
              <!-- 项目列表 -->
              <template v-else>
                <RecycleScroller
                  v-if="filteredProjectList.length > 0"
                  :items="filteredProjectList"
                  :item-size="36"
                  key-field="id"
                  class="virtual-scroller"
                  v-slot="{ item }">
                <div
                  :class="['column-item', { active: selectedProjectId === String(item.id) }]"
                  @click="handleSelectProject($event, item)">
                    <div v-overflow-title class="project-item-name">{{ item.spec.name }}</div>
                  </div>
                </RecycleScroller>
                <div v-else class="column-empty">
                  {{ currentBiz ? t('暂无项目') : t('请先选择业务') }}
                </div>
              </template>
            </div>
            <!-- 底部快捷入口 -->
            <div class="panel-footer">
              <div class="footer-item" @click="handleToProjectManage">
                <FolderOpen class="footer-icon" />
                <span>{{ t('项目管理') }}</span>
              </div>
              <div class="footer-divider"></div>
              <div class="footer-item" @click="handleToEnvManage">
                <CogShape class="footer-icon" />
                <span>{{ t('环境管理') }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
  import { ref, computed, watch, onMounted, onUnmounted } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRouter, useRoute } from 'vue-router';
  import { storeToRefs } from 'pinia';
  import { AngleDown, AngleRight, Search, FolderOpen, CogShape, Loading } from 'bkui-vue/lib/icon';
  import { RecycleScroller } from 'vue-virtual-scroller';
  import 'vue-virtual-scroller/dist/vue-virtual-scroller.css';
  import useTemplateStore from '../store/template';
  import useGlobalStore from '../store/global';
  import { ISpaceDetail } from '../../types/index';
  import { getProjectList } from '../api/project';
  import type { IProjectItem } from '../../types/project';
  import { hasProjectConcept, saveSpaceToProjectId } from '../utils/project';

  const { t } = useI18n();
  const route = useRoute();
  const router = useRouter();
  const globalStore = useGlobalStore();
  const {
    spaceId,
    projectId,
    spaceList,
    showApplyPermDialog,
    permissionQuery } = storeToRefs(globalStore);
  const templateStore = useTemplateStore();

  const props = withDefaults(
    defineProps<{
      separator?: string;
      showProject?: boolean;
      navList: any[];
    }>(),
    {
      separator: ' / ',
      showProject: true,
    },
  );

  const selectorRef = ref<HTMLElement>();
  const isOpen = ref(false);
  const selectedBizId = ref('');
  const selectedProjectId = ref('');
  // 临时存储用户点击的业务ID（未确认选择项目前不更新显示）
  const tempSelectedBizId = ref('');
  const bizSearch = ref('');
  const projectSearch = ref('');

  // 缓存每个业务的项目列表，避免重复请求
  const bizProjectsMap = ref<Record<string, IProjectItem[]>>({});
  // 当前正在加载项目列表的业务ID
  const projectLoading = ref(false);

  // 获取指定业务的项目列表（公共方法，消除重复逻辑）
  const fetchProjectList = async (bizId: string) => {
    // 如果已缓存且不为空，直接返回缓存数据
    if (bizProjectsMap.value[bizId]?.length > 0) {
      return bizProjectsMap.value[bizId];
    }

    projectLoading.value = true;
    try {
      const res = await getProjectList(bizId, { all: true });
      const projects = res.data?.projects || [];
      // 使用 Vue 3 的响应式 API 正确更新
      bizProjectsMap.value = {
        ...bizProjectsMap.value,
        [bizId]: projects,
      };
      return projects;
    } catch (e) {
      console.error('获取项目列表失败', e);
      bizProjectsMap.value = {
        ...bizProjectsMap.value,
        [bizId]: [],
      };
      return [];
    } finally {
      projectLoading.value = false;
    }
  };

  const optionList = ref<ISpaceDetail[]>([]);

  watch(
    spaceList,
    (val) => {
      optionList.value = val.slice();
    },
    {
      immediate: true,
    },
  );

  // 左侧过滤后的业务列表
  const filteredBizList = computed(() => {
    if (!bizSearch.value) return optionList.value;
    const keyword = bizSearch.value.toLowerCase();
    return optionList.value.filter((biz) => {
      const spaceName = biz.space_name.toLowerCase();
      return spaceName.includes(keyword) || String(biz.space_id).includes(keyword);
    });
  });

  // 当前选中的业务（已确认的）或暂选的业务
  const currentBiz = computed(() => {
    const bizId = tempSelectedBizId.value || selectedBizId.value;
    return optionList.value.find((b) => b.space_id === bizId);
  });

  // 右侧过滤后的项目列表（根据暂选的或已确认的业务ID显示）
  const filteredProjectList = computed(() => {
    const bizId = tempSelectedBizId.value || selectedBizId.value;
    const list = bizId ? (bizProjectsMap.value[String(bizId)] || []) : [];
    if (!projectSearch.value) return list;
    const keyword = projectSearch.value.toLowerCase();
    return list.filter((proj) => proj.spec.name.toLowerCase().includes(keyword));
  });

  // 显示文本（根据已确认的选择计算）
  const displayText = computed(() => {
    if (!selectedBizId.value) {
      return '';
    }
    const biz = optionList.value.find((b) => b.space_id === selectedBizId.value);
    if (!biz) {
      return '';
    }
    // 如果不显示项目列，只显示业务名称
    if (!props.showProject) {
      return biz.space_name;
    }
    const projectList = bizProjectsMap.value[selectedBizId.value] || [];
    const project = projectList.find((p) => String(p.id) === selectedProjectId.value);
    if (biz && project) {
      return `${biz.space_name}${props.separator}${project.spec.name}`;
    }
    // 已选中业务但未选项目，只显示业务名
    return biz.space_name;
  });


  // 依据全局 projectId 恢复选中的项目
  watch(
    () => projectId.value,
    (val) => {
      if (val) {
        selectedProjectId.value = val;
      } else {
        selectedProjectId.value = '';
      }
    },
    { immediate: true },
  );

  // 当 spaceId 和 projectId 都有值时，确保项目列表已加载
  watch(
    () => [spaceId.value, projectId.value],
    async ([currentSpaceId, currentProjectId]) => {
      if (currentSpaceId && currentProjectId) {
        // 使用公共方法获取项目列表（已内含缓存检查）
        await fetchProjectList(currentSpaceId);
      }
    },
    { immediate: true },
  );

  // 依据全局 spaceId 恢复选中的业务
  watch(
    () => spaceId.value,
    (val) => {
      if (val) {
        selectedBizId.value = val;
      }
    },
    { immediate: true },
  );

  const togglePanel = () => {
    isOpen.value = !isOpen.value;
    // 打开面板时，如果有已选中的业务，自动设置 tempSelectedBizId 为当前业务ID
    // 这样项目列表才能正确显示（因为项目列表依赖 tempSelectedBizId || selectedBizId）
    if (isOpen.value && selectedBizId.value) {
      tempSelectedBizId.value = selectedBizId.value;
    }
  };

  const closePanel = () => {
    isOpen.value = false;
    // 重置临时状态和搜索关键词
    tempSelectedBizId.value = '';
    bizSearch.value = '';
    projectSearch.value = '';
  };

  // 跳转到当前模块
  const navigateToModule = (spaceId: string, projId?: string) => {
    const nav = props.navList.find((item) => item.module === route.meta.navModule);
    const params: any = { spaceId };
    // 只有当当前模块有项目概念时才传递 projectId
    if (hasProjectConcept(route.meta?.navModule as string) && projId) {
      params.projectId = projId;
    }
    if (nav) {
      router.push({ name: nav.id, params });
    } else {
      router.push({ name: 'service-all', params });
    }
  };

  const handleSelectSpace = async (id: string, shouldClearState = true) => {
    const space = spaceList.value.find((item: ISpaceDetail) => item.space_id === id);
    if (space) {
      if (!space.permission) {
        permissionQuery.value = {
          resources: [
            {
              biz_id: id,
              basic: {
                type: 'biz',
                action: 'find_business_resource',
                resource_id: id,
              },
            },
          ],
        };

        showApplyPermDialog.value = true;
        return;
      }
      // 根据参数决定是否清除状态
      if (shouldClearState) {
        // 清除 projectId
        projectId.value = '';
        templateStore.$patch((state) => {
          state.templateSpaceList = [];
          state.currentTemplateSpace = 0;
          state.currentPkg = '';
        });
      }
    }
  };

  const handleSelectBiz = async (event: MouseEvent, biz: ISpaceDetail) => {
    event.stopPropagation();
    // 暂存用户点击的业务ID，不立即更新全局状态
    tempSelectedBizId.value = biz.space_id;
    // 如果不显示项目列（无项目概念），直接选中业务并跳转
    if (!props.showProject) {
      // 无项目概念时，立即更新全局 spaceId
      spaceId.value = biz.space_id;
      await handleSelectSpace(biz.space_id);
      navigateToModule(biz.space_id);
      selectedProjectId.value = '';
      closePanel();
      return;
    }
    // 只更新本地状态，等待用户选择项目后才更新全局 spaceId 并跳转
    await handleSelectSpace(biz.space_id, false);

    projectSearch.value = '';

    // 使用公共方法获取项目列表
    await fetchProjectList(biz.space_id);
  };

  const handleSelectProject = (event: MouseEvent, proj: IProjectItem) => {
    event.stopPropagation();
    // 选择项目后，才更新业务ID和项目ID
    const strProjectId = String(proj.id);
    selectedBizId.value = tempSelectedBizId.value;
    selectedProjectId.value = strProjectId;
    tempSelectedBizId.value = '';
    // 更新全局 projectId
    projectId.value = strProjectId;

    // 保存 spaceId 到 projectId 的映射
    saveSpaceToProjectId(selectedBizId.value, strProjectId);

    const bizItem = optionList.value.find((b) => String(b.space_id) === selectedBizId.value);
    if (bizItem) {
      // 更新全局 spaceId（选择项目后才更新，避免提前触发子组件重新加载）
      spaceId.value = bizItem.space_id;
      // 跳转到当前模块，并带上 projectId 参数
      navigateToModule(bizItem.space_id, strProjectId);
    }
    closePanel();
  };

  // 在新窗口打开路由
  const openInNewTab = (routeName: string, params?: Record<string, string>) => {
    closePanel();
    const routeParams: Record<string, string> = { spaceId: spaceId.value, ...params };
    const route = router.resolve({ name: routeName, params: routeParams });
    window.open(route.href, '_blank');
  };

  const handleToProjectManage = () => {
    openInNewTab('project-manage');
  };

  const handleToEnvManage = () => {
    const params: Record<string, string> = {};
    if (projectId.value) {
      params.projectId = projectId.value;
    }
    openInNewTab('env-manage', params);
  };

  // 点击外部关闭面板
  const handleClickOutside = (event: MouseEvent) => {
    if (selectorRef.value && !selectorRef.value.contains(event.target as Node)) {
      closePanel();
    }
  };

  onMounted(() => {
    document.addEventListener('click', handleClickOutside);
  });

  onUnmounted(() => {
    document.removeEventListener('click', handleClickOutside);
  });
</script>

<style lang="scss" scoped>
  .biz-project-selector {
    position: relative;
    width: 100%;
  }

  .selector-trigger {
    position: relative;
    display: flex;
    align-items: center;
    width: 100%;
    cursor: pointer;

    input {
      padding: 0 28px 0 10px;
      width: 100%;
      height: 32px;
      line-height: 32px;
      font-size: 12px;
      border: none;
      outline: none;
      background: #303d55;
      border-radius: 2px;
      color: #d3d9e4;
      cursor: pointer;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;

      &::placeholder {
        color: #979ba5;
      }
    }

    .arrow-icon {
      position: absolute;
      top: 50%;
      right: 6px;
      transform: translateY(-50%);
      font-size: 16px;
      color: #979ba5;
      transition: transform 0.2s ease;
      pointer-events: none;

      &.is-open {
        transform: translateY(-50%) rotate(-180deg);
      }
    }
  }

  .selector-panel {
    position: absolute;
    top: calc(100% + 10px);
    left: 0;
    z-index: 10000;
    display: flex;
    flex-direction: column;
    background: #182132;
    border-radius: 0 0 2px 2px;
    width: 414px;
    max-height: 260px;
    overflow: hidden;
    font-size: 12px;

    &.single-column {
      width: 240px;

      .panel-body {
        .biz-column.full-width {
          border-right: none;
        }
      }
    }
  }

  .panel-body {
    display: flex;
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }

  .panel-column {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;

    &.biz-column {
      border-right: 1px solid #404A5C;
    }

    &.project-column {
      .panel-footer {
        flex-shrink: 0;
      }
    }
  }

  .column-search-wrapper {
    flex-shrink: 0;
    padding: 8px 12px 0;
  }

  .column-search {
    position: relative;
    .search-icon {
      position: absolute;
      left: 0;
      top: 50%;
      transform: translateY(-50%);
      font-size: 14px;
      color: #63656e;
      pointer-events: none;
    }

    input {
      width: 100%;
      height: 32px;
      padding-left: 24px;
      font-size: 12px;
      color: #c4c6cc;
      background: #182132;
      border-width: 0;
      border-bottom: 1px solid #2f3746;
      border-radius: 2px;
      outline: none;

      &::placeholder {
        color: #63656e;
      }

      &:focus {
        border-bottom-color: #3a84ff;
      }
    }
  }

  .column-list {
    flex: 1;
    overflow: hidden;  // 改为 hidden，因为滚动由 virtual-scroller 管理
    padding: 4px 0;
    position: relative;

    .virtual-scroller {
      height: 100%;
      overflow-y: auto;

      &::-webkit-scrollbar {
        width: 4px;
      }

      &::-webkit-scrollbar-thumb {
        background: rgba(150, 162, 185, 0.3);
        border-radius: 2px;
      }
    }
  }

  .column-loading {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 20px;
    color: #979ba5;
    font-size: 12px;

    .loading-icon {
      font-size: 16px;
      animation: loading-rotate 1s linear infinite;
    }
  }

  @keyframes loading-rotate {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(360deg);
    }
  }

  .column-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 16px;
    height: 36px;
    font-size: 12px;
    color: #c4c6cc;
    cursor: pointer;
    transition: background 0.15s ease;

    &:hover {
      background: #2f3746;
    }

    &.active {
      color: #3a84ff;
      background: #1a2a4a;

      .arrow {
        color: #3a84ff;
      }
    }

    .name-wrapper {
      flex: 1;
      display: flex;
      align-items: center;
      min-width: 0;
      .name {
        flex: 0 1 auto;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .id {
        margin-left: 4px;
        color: #979ba5;
      }
    }

    .arrow {
      flex-shrink: 0;
      font-size: 12px;
      color: #63656e;
    }

    .project-item-name {
      overflow: hidden;
      white-space: nowrap;
      text-overflow: ellipsis;
    }
  }

  .column-empty {
    padding: 24px 16px;
    text-align: center;
    font-size: 12px;
    color: #63656e;
  }

  .panel-footer {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0;
    padding: 5px 16px 6px 16px;
    border-top: 1px solid #2F3847;
    background: #28354D;
    flex-shrink: 0;
  }

  .footer-item {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    font-size: 12px;
    color: #c4c6cc;
    cursor: pointer;
    transition: color 0.15s ease;
    line-height: 20px;
    overflow: hidden;

    &:hover {
      color: #3a84ff;

      .footer-icon {
        color: #3a84ff;
      }
    }
    & > span {
      display: inline-block;
      overflow: hidden;
      white-space: nowrap;
      text-overflow: ellipsis;
      min-width: 0;
    }
  }

  .footer-icon {
    flex-shrink: 0;
    font-size: 16px;
    color: #C4C6CC;
    transition: color 0.15s ease;
  }

  .footer-divider {
    width: 1px;
    height: 16px;
    background: #404A5C;
    margin: 0 15px;
  }

  // 面板动画
  .panel-fade-enter-active,
  .panel-fade-leave-active {
    transition: opacity 0.2s ease, transform 0.2s ease;
  }

  .panel-fade-enter-from,
  .panel-fade-leave-to {
    opacity: 0;
    transform: translateY(-4px);
  }
</style>
