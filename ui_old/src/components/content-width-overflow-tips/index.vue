<template>
  <bk-popover placement="top" :disabled="disabled" reference-cls="content-overflow-popover-reference">
    <div ref="containerRef" class="overflow-popover-container">
      <slot></slot>
    </div>
    <template #content>
      <slot></slot>
    </template>
  </bk-popover>
</template>
<script lang="ts" setup>
  import { ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue';

  const props = defineProps<{
    // 内容变化（如弹窗打开后名称才赋值）时触发重新计算，避免仅监听容器尺寸导致不生效
    watchKey?: unknown;
  }>();

  const containerRef = ref();
  const disabled = ref(true);

  onMounted(() => {
    calcPopover();
    // 监听容器宽度变化，重新设置popover激活态
    const observer = new ResizeObserver(calcPopover);
    observer.observe(containerRef.value);
    onBeforeUnmount(() => {
      containerRef.value && observer?.unobserve(containerRef.value);
      observer?.disconnect();
    });
  });

  // 内容变化后等待 DOM 更新再重新计算
  watch(
    () => props.watchKey,
    () => nextTick(calcPopover),
  );

  // 计算内容宽度是否超出容器宽度，超出则激活popover
  const calcPopover = () => {
    const contentEl = containerRef.value.firstElementChild;

    if (contentEl && contentEl.scrollWidth > containerRef.value.clientWidth) {
      disabled.value = false;
    } else {
      disabled.value = true;
    }
  };
</script>
<!-- 非 scoped：bk-popover 升级后把 slot 包进一个 span(reference 节点，内联 display:inline-block)。
     该 span 由子组件内部渲染、不携带本组件 data-v，且其 class 由 reference-cls 注入(本组件唯一使用)，-->
<style lang="scss">
  span.content-overflow-popover-reference {
    display: block !important;
    overflow: hidden;
  }
</style>
