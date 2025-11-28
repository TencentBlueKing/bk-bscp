<template>
  <DetailLayout :name="$t('配置模板详情')" :show-footer="false" @close="handleClose">
    <template #suffix>
      <div class="header-suffix">
        <div class="suffix-left">
          <span class="line"></span>
          <span>模板文件1</span>
          <bk-tag>当前版本: 105</bk-tag>
        </div>
        <div class="suffix-right">
          <bk-button theme="primary" text>{{ $t('编辑') }}</bk-button>
          <bk-button theme="primary" text>{{ $t('配置下发') }}</bk-button>
          <bk-popover ref="opPopRef" theme="light" placement="bottom-end" :arrow="false">
            <div class="more-actions">
              <Ellipsis class="ellipsis-icon" />
            </div>
            <template #content>
              <ul class="dropdown-ul">
                <li
                  class="dropdown-li"
                  v-for="item in operationList"
                  :key="item.name"
                  @click="handleOpTemplate(item.id)">
                  <span>{{ item.name }}</span>
                </li>
              </ul>
            </template>
          </bk-popover>
        </div>
      </div>
    </template>
    <template #content>
      <section class="content-wrap">
        <div class="content">
          <div class="form-wrap">
            <div class="title">{{ $t('模板信息') }}</div>
            <div class="info-item" v-for="item in infoList" :key="item.label">
              <span class="label">{{ item.label }}</span>
              <span class="value">{{ item.value }}</span>
            </div>
          </div>
          <div class="editor-wrap">
            <ConfigContent :content="formData.content" />
          </div>
        </div>
      </section>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import DetailLayout from '../../scripts/components/detail-layout.vue';
  import ConfigContent from '../components/config-content.vue';

  const { t } = useI18n();

  const emits = defineEmits(['close', 'created']);

  const formData = ref({
    privilege: '644',
    user: 'root',
    user_group: 'root',
    template_name: '',
    file_name: '',
    memo: '',
    content: '',
  });
  const operationList = [
    {
      name: t('版本管理'),
      id: 'version-manage',
    },
    {
      name: t('删除'),
      id: 'delete',
    },
  ];
  const infoList = [
    {
      label: t('模板归属'),
      value: 'config_delivery/默认模板',
    },
    {
      label: t('模板名称'),
      value: '模板文件1',
    },
    {
      label: t('配置文件名'),
      value: 'nginx.conf',
    },
    {
      label: t('配置文件描述'),
      value: '这是一个用于演示的配置文件描述',
    },
    {
      label: t('文件权限'),
      value: '644',
    },
    {
      label: t('用户'),
      value: 'root',
    },
    {
      label: t('用户组'),
      value: 'root',
    },
  ];

  const handleOpTemplate = (id: string) => {
    console.log('op template', id);
  };

  const handleClose = () => {
    emits('close', false);
  };
</script>

<style scoped lang="scss">
  .content-wrap {
    padding: 24px;
    height: 100%;
    background: #f5f7fa;
    .content {
      display: flex;
      height: 100%;
      background: #ffffff;
      .form-wrap {
        padding: 12px 24px;
        width: 368px;
        .title {
          font-weight: 700;
          font-size: 14px;
          color: #4d4f56;
          line-height: 22px;
          margin-bottom: 16px;
        }
        .info-item {
          display: flex;
          flex-direction: column;
          font-size: 12px;
          line-height: 20px;
          margin-bottom: 24px;
          .label {
            color: #979ba5;
          }
          .value {
            color: #313238;
          }
        }
      }
      .editor-wrap {
        flex: 1;
        min-width: 0;
      }
    }
  }
</style>
