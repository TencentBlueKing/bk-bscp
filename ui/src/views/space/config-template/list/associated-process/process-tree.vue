<template>
  <bk-tree :data="tree" node-key="id" :show-node-type-icon="false">
    <template #node="node">
      <div class="node-item-wrapper">
        <div v-if="node.children" class="prefix">集</div>
        <bk-checkbox
          v-else
          size="small"
          :model-value="node.checked"
          :disabled="node.disabled"
          :indeterminate="node.indeterminate" />
        <div class="node-name-text">
          {{ node.name }}
        </div>
      </div>
    </template>
  </bk-tree>
</template>

<script lang="ts" setup>
  const tree = [
    {
      id: 1,
      name: '根节点',
      children: [
        {
          id: 2,
          name: '子节点 A',
        },
        {
          id: 3,
          name: '子节点 B',
          children: [
            { id: 4, name: '子节点 B-1' },
            { id: 5, name: '子节点 B-2' },
          ],
        },
      ],
    },
  ];
</script>

<style scoped lang="scss">
  .node-item-wrapper {
    display: flex;
    align-items: center;
    gap: 8px;
    .prefix {
      width: 20px;
      height: 20px;
      background: #c4c6cc;
      border-radius: 50%;
      color: #ffffff;
      font-size: 12px;
      line-height: 20px;
      text-align: center;
    }
  }
</style>
