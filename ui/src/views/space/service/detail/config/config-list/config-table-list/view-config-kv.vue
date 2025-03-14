<template>
  <bk-sideslider
    ref="sideSliderRef"
    width="640"
    quick-close
    :title="t('查看配置项')"
    :is-show="props.show"
    @closed="close"
    @shown="setEditorHeight">
    <div class="view-wrap">
      <bk-tab v-model:active="activeTab" type="card-grid" ext-cls="view-config-tab">
        <bk-tab-panel name="content" :label="t('配置项信息')">
          <bk-form label-width="100" form-type="vertical">
            <bk-form-item :label="t('配置项名称')">{{ props.config.spec.key }}</bk-form-item>
            <bk-form-item :label="t('配置项描述')">
              <div class="memo">{{ props.config.spec.memo || '--' }}</div>
            </bk-form-item>
            <bk-form-item :label="t('配置项类型')">
              {{ props.config.spec.kv_type === 'secret' ? t('敏感信息') : props.config.spec.kv_type }}
            </bk-form-item>
            <bk-form-item :label="t('配置项值')">
              <div v-if="props.config.spec.kv_type === 'secret'" class="secret-value">
                <div class="secret-list disabled">
                  <div
                    :class="['secret-item', { active: config.spec.secret_type === item.value }]"
                    v-for="item in secretType"
                    :key="item.value">
                    {{ item.label }}
                  </div>
                </div>
                <span v-if="props.config.spec.secret_hidden" class="un-view-value">
                  {{ t('敏感数据不可见，无法查看实际内容') }}
                </span>
                <template v-else>
                  <SecretEditor
                    v-if="props.config.spec.secret_type === 'custom' || props.config.spec.secret_type === 'certificate'"
                    :is-edit="false"
                    :content="props.config.spec.value" />
                  <span v-else class="secret-single-line-value">
                    <span>{{ isCipherShowSecret ? '******' : props.config.spec.value }}</span>
                    <Unvisible v-if="isCipherShowSecret" class="view-icon" @click="isCipherShowSecret = false" />
                    <Eye v-else class="view-icon" @click="isCipherShowSecret = true" />
                  </span>
                </template>
              </div>
              <span v-else-if="props.config.spec.kv_type === 'string' || props.config.spec.kv_type === 'number'">
                {{ props.config.spec.value }}
              </span>
              <div v-else class="editor-wrap">
                <kvConfigContentEditor
                  :content="props.config.spec.value"
                  :editable="false"
                  :height="editorHeight"
                  :languages="props.config.spec.kv_type" />
              </div>
            </bk-form-item>
          </bk-form>
        </bk-tab-panel>
        <bk-tab-panel name="meta" :label="t('元数据')">
          <div class="meta-config-wrapper">
            <ConfigContentEditor
              language="json"
              :content="JSON.stringify(metaData, null, 2)"
              :editable="false"
              :show-tips="false" />
          </div>
        </bk-tab-panel>
      </bk-tab>
    </div>
    <section class="action-btns">
      <bk-button v-if="showEditBtn" theme="primary" @click="emits('openEdit')">
        {{ t('编辑') }}
      </bk-button>
      <bk-button @click="close">{{ t('关闭') }}</bk-button>
    </section>
  </bk-sideslider>
</template>
<script setup lang="ts">
  import { ref, computed, watch, nextTick } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { IConfigKvType } from '../../../../../../../../types/config';
  import kvConfigContentEditor from '../../components/kv-config-content-editor.vue';
  import ConfigContentEditor from '../../components/config-content-editor.vue';
  import { sortObjectKeysByAscii, datetimeFormat } from '../../../../../../../utils';
  import { Unvisible, Eye } from 'bkui-vue/lib/icon';
  import SecretEditor from './config-form-kv/secret-form/secret-content-editor.vue';

  const { t } = useI18n();
  const props = defineProps<{
    config: IConfigKvType;
    show: boolean;
    showEditBtn?: boolean;
  }>();

  const emits = defineEmits(['update:show', 'confirm', 'openEdit']);

  const secretType = [
    {
      label: t('密码'),
      value: 'password',
    },
    {
      label: t('证书'),
      value: 'certificate',
    },
    {
      label: t('API密钥'),
      value: 'secret_key',
    },
    {
      label: t('访问令牌'),
      value: 'token',
    },
    {
      label: t('自定义'),
      value: 'custom',
    },
  ];

  const activeTab = ref('content');
  const isFormChange = ref(false);
  const sideSliderRef = ref();
  const editorHeight = ref(0);
  const isCipherShowSecret = ref(true);

  const metaData = computed(() => {
    const { content_spec, revision, spec } = props.config;
    const { create_at, creator, update_at, reviser } = revision;
    const { byte_size, signature, md5 } = content_spec;
    const { key, kv_type, memo, certificate_expiration_date } = spec;
    return sortObjectKeysByAscii({
      key,
      kv_type,
      byte_size,
      signature,
      create_at: datetimeFormat(create_at),
      creator,
      reviser,
      update_at: datetimeFormat(update_at),
      md5,
      memo,
      ...(certificate_expiration_date && { expiration_date: datetimeFormat(certificate_expiration_date) }),
    });
  });

  watch(
    () => props.show,
    (val) => {
      if (val) {
        isFormChange.value = false;
        activeTab.value = 'content';
      }
    },
  );

  const setEditorHeight = () => {
    nextTick(() => {
      const el = sideSliderRef.value.$el.querySelector('.view-wrap');
      const editorMinHeight = 300; // 编辑器最小高度
      const remainingHeight = el.offsetHeight - 354; // 容器其他元素已占用高度
      editorHeight.value = remainingHeight > editorMinHeight ? remainingHeight : editorMinHeight;
    });
  };

  const close = () => {
    emits('update:show', false);
  };
</script>
<style lang="scss" scoped>
  .view-wrap {
    height: calc(100vh - 101px);
    font-size: 12px;
    overflow: hidden;
    .view-config-tab {
      height: 100%;
      :deep(.bk-tab-header) {
        padding: 8px 24px 0;
        font-size: 14px;
        background: #eaebf0;
      }
      :deep(.bk-tab-content) {
        padding: 24px 0;
        height: calc(100% - 48px);
        box-shadow: none;
      }
    }
    .bk-form {
      padding: 0 40px;
      height: 100%;
      overflow: auto;
    }
    :deep(.bk-form-item) {
      margin-bottom: 24px;
      &:last-child {
        margin-bottom: 0;
      }
      .bk-form-label,
      .bk-form-content {
        font-size: 12px;
      }
      .bk-form-label {
        line-height: 26px;
        color: #979ba5;
      }
      .bk-form-content {
        line-height: normal;
        color: #323339;
      }
    }
  }
  .memo {
    line-height: 20px;
    white-space: pre-wrap;
    word-break: break-word;
  }
  .meta-config-wrapper {
    padding: 0 40px;
    height: 100%;
    overflow: auto;
  }
  .action-btns {
    border-top: 1px solid #dcdee5;
    padding: 8px 24px;
    .bk-button {
      margin-right: 8px;
      min-width: 88px;
    }
  }
  .secret-value {
    .secret-single-line-value {
      display: flex;
      align-items: center;
      .view-icon {
        cursor: pointer;
        margin: 0 8px;
        font-size: 14px;
        color: #979ba5;
        &:hover {
          color: #3a84ff;
        }
      }
    }
    .un-view-value {
      color: #c4c6cc;
    }
  }
  .secret-list {
    display: flex;
    align-items: center;
    margin-bottom: 12px;
    &.disabled {
      .secret-item {
        cursor: not-allowed;
        background: #f0f1f5;
        color: #979ba5;
      }
    }
    .secret-item {
      padding: 0 10px;
      height: 26px;
      min-width: 80px;
      line-height: 26px;
      background: #f8f8f8;
      text-align: center;
      background: #ffffff;
      border: 1px solid #c4c6cc;
      color: #63656e;
      cursor: pointer;
      &:not(:last-child) {
        border-right: none;
      }
      &.active {
        background: #e1ecff;
        border: 1px solid #3a84ff;
        color: #3a84ff;
      }
    }
  }
</style>
