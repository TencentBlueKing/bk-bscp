<template>
  <bk-popover ref="opPopRef" theme="light" placement="bottom-start" :arrow="false">
    <div class="more-actions">
      <Ellipsis class="ellipsis-icon" />
    </div>
    <template #content>
      <ul class="dropdown-ul">
        <li class="dropdown-li" v-for="item in operationList" :key="item.id" @click="handleClick(item)">
          <span>{{ item.name }}</span>
        </li>
      </ul>
    </template>
  </bk-popover>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { Ellipsis } from 'bkui-vue/lib/icon';

  interface operationType {
    name: string;
    id: string;
  }

  defineProps<{
    operationList: operationType[];
  }>();
  const emits = defineEmits(['operation']);

  const opPopRef = ref();

  const handleClick = (operation: operationType) => {
    emits('operation', operation.id);
    opPopRef.value.hide();
  };
</script>

<style scoped lang="scss">
  .more-actions {
    display: flex;
    align-items: center;
    justify-content: center;
    margin-left: 8px;
    width: 16px;
    height: 16px;
    border-radius: 50%;
    cursor: pointer;
    &:hover {
      background: #dcdee5;
      color: #3a84ff;
    }
    .ellipsis-icon {
      font-size: 16px;
      transform: rotate(90deg);
      cursor: pointer;
    }
  }

  .dropdown-ul {
    margin: -12px;
    font-size: 12px;
    .dropdown-li {
      padding: 0 12px;
      min-width: 68px;
      font-size: 12px;
      line-height: 32px;
      color: #4d4f56;
      cursor: pointer;
      &:hover {
        background: #f5f7fa;
      }
    }
  }
</style>
