<template>
  <div class="operate-area">
    <div class="operate-btns">
      <bk-button theme="primary" @click="handleOpenCreate">{{ $t('新建表格') }}</bk-button>
    </div>
    <SearchInput
      v-model="searchStr"
      class="config-search-input"
      :width="400"
      :placeholder="$t('表格名称/表格描述/最近更新人')" />
  </div>
  <Table :bk-biz-id="bkBizId" />
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import SearchInput from '../../../../../components/search-input.vue';
  import Table from './table.vue';

  const route = useRoute();
  const router = useRouter();
  const bkBizId = String(route.params.spaceId);

  const searchStr = ref('');

  const handleOpenCreate = () => {
    router.push({
      name: 'create-table-structure',
      params: { spaceId: bkBizId },
    });
  };
</script>

<style scoped lang="scss">
  .operate-area {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
  }
</style>
