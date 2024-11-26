<template>
  <div class="example-wrap">
    <div class="header">
      <span>{{ $t('格式示例') }}</span>
      <copy-shape
        class="icon"
        v-bk-tooltips="{
          content: $t('复制示例内容'),
          placement: 'top',
          extCls: 'copy-example-content',
        }"
        @click="handleCopyText" />
    </div>
    <div class="content">
      <template v-if="format === 'text'">
        <template v-for="item in textFormat" :key="item.formatTitle">
          <div class="format">
            <div>{{ item.formatTitle }}</div>
            <div>{{ item.formatContent }}</div>
          </div>
          <div class="example">
            <div v-for="exampleList in item.example" :key="exampleList.title">
              <div>{{ exampleList.title }}</div>
              <div v-for="(example, index) in exampleList.list" :key="index" class="text-example">
                <span>{{ example.key }}</span>
                <span class="type">{{ example.type }}</span>
                <span>{{ example.value }}</span>
              </div>
            </div>
          </div>
        </template>
      </template>
      <div v-else class="format">
        <div>{{ props.format === 'json' ? $t('字段说明') : $t('格式说明') }}:</div>
        <div class="description">{{ fieldsDescription }}</div>
      </div>
      <div v-if="format !== 'text'" class="example">
        <div>{{ $t('示例') }}:</div>
        <div class="description">{{ copyContent }}</div>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { computed } from 'vue';
  import { CopyShape } from 'bkui-vue/lib/icon';
  import { copyToClipBoard } from '../../../../utils';
  import { Message } from 'bkui-vue';
  import { useI18n } from 'vue-i18n';

  const { t } = useI18n();
  const props = defineProps<{
    format: string;
  }>();

  const textFormat = [
    {
      formatTitle: t('简单文本格式：'),
      formatContent: t('变量名称 变量类型 默认值 描述（可选）'),
      example: [
        {
          title: t('示例：'),
          list: [
            {
              key: 'nginx_listen',
              type: 'string',
              value: '0.0.0.0',
              memo: '',
            },
            {
              key: 'number_key',
              type: 'number',
              value: 8080,
              memo: 'Nginx监听端口',
            },
          ],
        },
      ],
    },
  ];

  const fieldsDescription = computed(() => {
    if (props.format === 'json') {
      return ` {
      "变量名称": {
          "variable_type": "变量类型，暂时只支持string、number、text三种变量类型",
          "value": "变量默认值",
          "memo": "变量描述"
      }
  }`;
    }
    return `    变量名称:
        variable_type: 变量类型，暂时只支持string、number、text三种变量类型
        value: 变量默认值
        memo: 变量描述`;
  });

  /* eslint-disable */
  const copyContent = computed(() => {
    if (props.format === 'text') {
      return `bk_bscp_nginx_listen string 0.0.0.0
bk_bscp_number_key number 8080`;
    }
    if (props.format === 'json') {
      return `{
    "bk_bscp_nginx_listen": {
        "variable_type": "string",
        "value": "0.0.0.0",
        "memo": "Nginx监听地址"
    },
    "bk_bscp_nginx_port": {
        "variable_type": "number",
        "value": "8080",
        "memo": "Nginx监听端口"
    },
    "bk_bscp_nginx_location": {
        "variable_type": "text",
        "value": "location /images/ {\\n       root /data;\\n       autoindex on;\\n   }",
        "memo": "Nginx location"
    }
}`;
    }
    return `bk_bscp_nginx_listen:
    variable_type: string
    value: 0.0.0.0
    memo: Nginx监听地址
bk_bscp_nginx_port:
    variable_type: number
    value: '8080'
    memo: Nginx监听端口
bk_bscp_nginx_location:
    variable_type: text
    value: |-
      location /images/ {
            root /data;
            autoindex on;
        }
    memo: Nginx location`;
  });
  /* eslint-enable */

  // 复制
  const handleCopyText = () => {
    copyToClipBoard(copyContent.value);
    Message({
      theme: 'success',
      message: t('示例内容已复制'),
    });
  };
</script>

<style scoped lang="scss">
  .example-wrap {
    width: 520px;
    background: #2e2e2e;
    padding: 0 16px;
    border-top: 1px solid #000;
    .header {
      display: flex;
      align-items: center;
      gap: 16px;
      padding: 8px 0 12px 0;
      font-weight: 700;
      font-size: 14px;
      color: #979ba5;
      border-bottom: 1px solid #000;
      .icon {
        cursor: pointer;
      }
    }
    .content {
      color: #c4c6cc;
      font-size: 12px;
      overflow: auto;
      max-height: calc(100% - 42px);
      .text-example {
        white-space: nowrap;
        span {
          margin-right: 4px;
        }
      }
      .example {
        margin-top: 13px;
        .type {
          color: #ff9c01;
        }
      }
    }
    .format {
      margin-top: 16px;
    }
  }

  .description {
    white-space: pre-wrap;
  }
</style>

<style>
  .copy-example-content {
    background-color: #000000 !important;
  }
</style>
