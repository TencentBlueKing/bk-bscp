<template>
  <div class="popover-wrap">
    <bk-popover
      ext-cls="table-filter-popover"
      trigger="manual"
      :is-show="isShowSelect"
      theme="light"
      placement="bottom-start"
      @after-hidden="isShowSelect = false">
      <bk-button text @click="isShowSelect = !isShowSelect">
        <Funnel :fill="fillColor" />
      </bk-button>
      <template #content>
        <div v-click-outside="handleReset">
          <div class="popover-content">
            <bk-checkbox-group v-model="selectedValues">
              <bk-checkbox v-for="item in filterList" :key="item.value" :label="item.value">{{
                item.text
              }}</bk-checkbox>
            </bk-checkbox-group>
          </div>
          <div class="popover-footer">
            <bk-button theme="primary" size="small" style="margin-right: 6px" @click="handleConfirm">
              {{ $t('确定') }}
            </bk-button>
            <bk-button size="small" @click="handleReset">{{ $t('重置') }}</bk-button>
          </div>
        </div>
      </template>
    </bk-popover>
  </div>
</template>

<script lang="ts" setup>
  import { Funnel } from 'bkui-vue/lib/icon';
  import { ref, computed } from 'vue';

  defineProps<{
    filterList: {
      value: string;
      text: string;
    }[];
  }>();

  const emits = defineEmits(['selected']);

  const isShowSelect = ref(false);
  const selectedValues = ref<string[]>([]);

  const fillColor = computed(() => {
    if (isShowSelect.value) {
      return '#313238';
    }
    if (selectedValues.value.length > 0) {
      return '#3a84ff';
    }
    return '#c4c6cc';
  });

  const handleConfirm = () => {
    emits('selected', selectedValues.value);
    isShowSelect.value = false;
  };

  const handleReset = () => {
    selectedValues.value = [];
    emits('selected', selectedValues.value);
    isShowSelect.value = false;
  };
</script>

<style scoped lang="scss">
  .popover-wrap {
    display: inline-block;
    width: 20px;
  }
  .popover-content {
    padding: 0 10px;
    min-width: 200px;
    max-width: 300px;
    max-height: 200px;
    min-height: 40px;
    overflow-y: auto;
    .bk-checkbox-group {
      display: flex;
      flex-direction: column;
      .bk-checkbox {
        margin: 0;
        height: 32px;
      }
    }
  }
  .popover-footer {
    display: flex;
    padding: 12px;
    border-top: solid 1px #dcdee5;
  }
</style>

<style lang="scss">
  .table-filter-popover {
    padding: 5px 0 0 0 !important;
  }
</style>
