<template>
  <DetailLayout :name="$t('配置模板详情')" :show-footer="false" @close="handleClose">
    <template #header-suffix>
      <div class="header-suffix">
        <div class="suffix-left">
          <span class="line"></span>
          <span class="name">{{ templateDetail.name }}</span>
          <bk-tag>{{ $t('当前版本') }}: {{ templateDetail.revision_name }}</bk-tag>
        </div>
        <div class="suffix-right">
          <bk-button @click="handleOpTemplate('edit')">{{ $t('编辑') }}</bk-button>
          <bk-button theme="primary" @click="handleOpTemplate('issue')">{{ $t('配置下发') }}</bk-button>
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
              <span class="value">{{ templateDetail[item.value as keyof typeof templateDetail] }}</span>
            </div>
          </div>
          <div class="editor-wrap">
            <ConfigContent :bk-biz-id="bkBizId" :content="editorContent" :editable="false" />
          </div>
        </div>
      </section>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { getConfigTemplateDetail } from '../../../../api/config-template';
  import { downloadTemplateContent } from '../../../../api/template';
  import { Ellipsis } from 'bkui-vue/lib/icon';
  import DetailLayout from '../../scripts/components/detail-layout.vue';
  import ConfigContent from '../components/config-content.vue';

  const { t } = useI18n();

  const emits = defineEmits(['close', 'operate']);
  const props = defineProps<{
    bkBizId: string;
    templateId: number;
    templateSpaceId: number;
  }>();
  const templateDetail = ref({
    name: '',
    file_name: '',
    memo: '',
    privilege: '',
    user: '',
    user_group: '',
    revision_name: '',
    sign: '',
    attribution: '',
    highlight_style: '',
  });
  const editorContent = ref('');

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
      value: 'attribution',
    },
    {
      label: t('模板名称'),
      value: 'name',
    },
    {
      label: t('配置文件名'),
      value: 'file_name',
    },
    {
      label: t('配置文件描述'),
      value: 'memo',
    },
    {
      label: t('文件权限'),
      value: 'privilege',
    },
    {
      label: t('用户'),
      value: 'user',
    },
    {
      label: t('用户组'),
      value: 'user_group',
    },
  ];

  onMounted(() => {
    getDetail();
  });

  const handleOpTemplate = (op: string) => {
    emits('operate', op);
    emits('close');
  };

  const getDetail = async () => {
    const res = await getConfigTemplateDetail(props.bkBizId, props.templateId);
    const detail = res.bind_template;
    templateDetail.value = {
      name: detail.name || '',
      file_name: detail.file_path + detail.file_name,
      memo: detail.memo,
      privilege: detail.privilege,
      user: detail.user,
      user_group: detail.user_group,
      revision_name: detail.revision_name,
      sign: detail.sign,
      highlight_style: detail.highlight_style,
      attribution: `${detail.template_space_name}/${detail.template_set_name}`,
    };
    editorContent.value = await downloadTemplateContent(props.bkBizId, props.templateSpaceId, detail.sign);
  };

  const handleClose = () => {
    emits('close');
  };
</script>

<style scoped lang="scss">
  .header-suffix {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex: 1;
    .suffix-left {
      display: flex;
      align-items: center;
      .line {
        width: 1px;
        height: 16px;
        background: #dcdee5;
        margin-right: 12px;
      }
      .name {
        font-size: 14px;
        color: #4d4f56;
        margin-right: 16px;
      }
    }
    .suffix-right {
      display: flex;
      align-items: center;
      .bk-button {
        width: 88px;
        margin-left: 8px;
      }
    }
  }
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
            margin-bottom: 4px;
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
      &.disabled {
        color: #c4c6cc;
        cursor: not-allowed;
      }
      &:hover {
        background: #f5f7fa;
      }
    }
  }
</style>
