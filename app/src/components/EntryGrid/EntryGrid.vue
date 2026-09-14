<script setup lang="ts">
interface EntryItem {
  title: string
  description: string
  path: string
}

const props = defineProps<{
  entries: EntryItem[]
}>()

const router = useRouter()

function goTo(path: string) {
  router.push(path)
}
</script>

<template>
  <ARow :gutter="[16, 16]">
    <ACol
      v-for="item in props.entries"
      :key="item.path"
      :xs="24"
      :sm="12"
      :xl="8"
    >
      <ACard
        hoverable
        class="entry-card h-full"
        :bordered="false"
        @click="goTo(item.path)"
      >
        <h3 class="entry-title">
          {{ item.title }}
        </h3>
        <p class="entry-description">
          {{ item.description }}
        </p>
      </ACard>
    </ACol>
  </ARow>
</template>

<style lang="less" scoped>
.entry-card {
  cursor: pointer;
  background: #f7f9fc;
  border-top: 4px solid var(--ant-color-primary);
}

.entry-title {
  font-size: 28px;
  line-height: 1.15;
  font-weight: 700;
  margin: 0 0 12px;
}

.entry-description {
  margin: 0;
  color: rgba(0, 0, 0, 0.65);
  font-size: 16px;
  line-height: 1.7;
}

.dark {
  .entry-card {
    background: #14181f;
  }

  .entry-description {
    color: rgba(255, 255, 255, 0.72);
  }
}
</style>
