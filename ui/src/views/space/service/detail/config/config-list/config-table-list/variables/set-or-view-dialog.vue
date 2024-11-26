<template>
  <bk-dialog :is-show="isShow" :title="$t('变量值')" :width="768">
    <div class="editor">
      <CodeEditor :model-value="value" @update:model-value="localVal = $event"></CodeEditor>
    </div>
    <template #footer>
      <bk-button v-if="props.isSet" theme="primary" style="margin-right: 8px" @click="handleConfirm">
        {{ $t('确认') }}
      </bk-button>
      <bk-button @click="emits('update:isShow', false)">{{ $t('取消') }}</bk-button>
    </template>
  </bk-dialog>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import CodeEditor from '../../../../../../../../components/code-editor/index.vue';
  const props = defineProps<{
    value: string;
    isSet: boolean;
    isShow: boolean;
  }>();

  const localVal = ref(props.value);

  const emits = defineEmits(['change', 'update:isShow']);

  const handleConfirm = () => {
    emits('change', localVal.value);
    emits('update:isShow', false);
  };
</script>

<style scoped lang="scss">
  .editor {
    height: 400px;
  }
</style>
