<template>
  <Card :title="$t('连接信息')">
    <bk-alert theme="info" style="margin-top: 8px;">
      {{ $t('需为平台访问数据库的出口 IP 授权。') }}
      <bk-button text theme="primary" @click="handleViewDetails">
        {{ $t('查看详情') }}
      </bk-button>
    </bk-alert>
  </Card>
</template>

<script lang="ts" setup>
  import Card from '../../component/card.vue';

  const handleViewDetails = () => {
    // TODO: 跳转至详情页
  };
</script>

<style scoped lang="scss"></style>
