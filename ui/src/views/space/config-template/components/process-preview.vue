<template>
  <div class="preview-wrap">
    <div class="head">
      <div class="head-left">
        <div class="close-btn" @click="emits('close')">
          <angle-down-line class="close-icon" />
        </div>
        <span class="title">{{ $t('预览') }}</span>
      </div>
      <bk-select
        class="process-select"
        v-model="inst"
        :filterable="false"
        :clearable="false"
        @select="handleSelectProcess">
        <template #prefix>
          <span class="select-prefix">{{ $t('进程实例') }}</span>
        </template>
        <bk-option v-for="(item, index) in instOptions" :id="item.id" :key="index" :name="item.name" />
      </bk-select>
    </div>
    <div class="preview-content">
      <CodeEditor
        v-if="inst"
        :model-value="previewContent"
        :editable="false"
        line-numbers="off"
        :minimap="false"
        :vertical-scrollbar-size="0"
        :horizon-scrollbar-size="0"
        render-line-highlight="none"
        :render-indent-guides="false"
        :folding="false"
        language="python" />
      <bk-exception
        v-else
        class="exception-wrap-item exception-gray"
        :description="$t('请先选择进程实例')"
        scene="part"
        type="empty" />
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onBeforeUnmount, onMounted } from 'vue';
  import { AngleDownLine } from 'bkui-vue/lib/icon';
  import { getProcessList } from '../../../../api/process';
  import { previewConfig } from '../../../../api/config-template';
  import CodeEditor from '../../../../components/code-editor/index.vue';
  import type { IProcessItem } from '../../../../../types/process';

  const emits = defineEmits(['close']);
  const props = defineProps<{
    bkBizId: string;
    configContent: string;
  }>();

  const codeEditorRef = ref();
  const inst = ref('');
  const instOptions = ref<{ id: number; name: string }[]>([]);
  const previewContent = ref('');

  const loading = ref(false);

  onBeforeUnmount(() => {
    if (codeEditorRef.value) {
      codeEditorRef.value.destroy();
    }
  });

  onMounted(() => {
    loadBindProcess();
  });

  const loadBindProcess = async () => {
    try {
      loading.value = true;
      const res = await getProcessList(props.bkBizId, { start: 0, all: true });
      instOptions.value = res.process.map((item: IProcessItem) => ({
        id: item.attachment.cc_process_id,
        name: item.spec.alias,
      }));
    } catch (error) {
      console.error(error);
    } finally {
      loading.value = false;
    }
  };

  const handleSelectProcess = async (id: number) => {
    try {
      const data = {
        templateContent: props.configContent,
        ccProcessId: id,
      };
      const res = await previewConfig(props.bkBizId, data);
      console.log('res', res);
      previewContent.value = res.content;
    } catch (error) {
      console.error(error);
    }
  };
</script>

<style scoped lang="scss">
  .preview-wrap {
    flex-shrink: 0;
    width: 417px;
    height: 100%;
    border-radius: 4px;
    background: #f5f7fa;
    .head {
      display: flex;
      justify-content: space-between;
      align-items: center;
      height: 40px;
      line-height: 40px;
      background: #2e2e2e;
      .head-left {
        display: flex;
        align-items: center;
      }
      .close-btn {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 30px;
        height: 40px;
        background: #478efd;
        cursor: pointer;
        .close-icon {
          color: #ffffff;
          font-size: 14px;
          transform: rotate(-90deg);
        }
      }
      .title {
        margin-left: 8px;
        font-size: 14px;
        color: #e6e6e6;
      }
      .process-select {
        width: 260px;
        margin-right: 16px;
        :deep(.bk-input) {
          height: 26px;
          line-height: 26px;
          border: 1px solid #575757;
          input {
            background: #2e2e2e;
            color: #b3b3b3;
          }
        }
        .select-prefix {
          padding: 0 8px;
          background: #3d3d3d;
          color: #b3b3b3;
        }
      }
    }
    .preview-content {
      height: calc(100% - 40px);
      background: #242424;
      .exception-wrap-item {
        padding-top: 100px;
        :deep(.bk-exception-img) {
          height: 150px;
        }
        :deep(.bk-exception-description) {
          font-size: 14px;
        }
      }
    }
  }
</style>
