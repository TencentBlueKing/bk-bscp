<template>
  <div :class="['sheet-list-wrap', { close: !isShowSheetList }]">
    <div class="sheet-list">
      <div
        v-for="sheet in list"
        :key="sheet.name"
        :class="['sheet-item', { active: viewSheet === sheet.name }]"
        @click="handleChangeSheet(sheet.name)">
        {{ sheet.name }}
      </div>
    </div>
    <div :class="['toggle-btn', { close: !isShowSheetList }]" @click="isShowSheetList = !isShowSheetList">
      <AngleDownLine />
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { AngleDownLine } from 'bkui-vue/lib/icon';

  defineProps<{
    list: { name: string }[];
    viewSheet: string;
  }>();

  const emits = defineEmits(['change']);

  const isShowSheetList = ref(true);

  const handleChangeSheet = (sheetName: string) => {
    emits('change', sheetName);
  };
</script>

<style scoped lang="scss">
  .sheet-list-wrap {
    position: relative;
    height: 100%;
    width: 220px;
    transition: 0.5s;
    &.close {
      width: 0;
    }
    .sheet-list {
      height: 100%;
      overflow: auto;
      border-right: 1px solid #dcdee5;
      .sheet-item {
        height: 40px;
        line-height: 40px;
        padding-left: 24px;
        color: #63656e;
        cursor: pointer;
        &.active {
          color: #3a84ff;
          background: #e1ecff;
          border-right: 2px solid #3a84ff;
        }
      }
    }
    .toggle-btn {
      display: flex;
      align-items: center;
      justify-content: space-between;
      position: absolute;
      right: -16px;
      top: 50%;
      transform: translateY(-50%);
      width: 16px;
      height: 64px;
      background: #dcdee5;
      border-radius: 0 4px 4px 0;
      color: #fff;
      cursor: pointer;
      span {
        font-size: 16px;
        transform: rotate(90deg);
        transition: 0.5s;
      }
      &.close {
        span {
          transform: rotate(-90deg);
        }
      }
    }
  }
</style>
