<script setup lang="ts">
defineProps<{
  placeholder?: string
  addButtonText?: string
}>()

const items = defineModel<string[]>({ required: true })

function addItem() {
  items.value = [...items.value, '']
}

function removeItem(index: number) {
  if (items.value.length <= 1)
    return
  const next = [...items.value]
  next.splice(index, 1)
  items.value = next
}

function updateItem(index: number, value: string) {
  const next = [...items.value]
  next[index] = value
  items.value = next
}
</script>

<template>
  <div class="space-y-2">
    <div
      v-for="(item, index) in items"
      :key="index"
      class="flex items-center gap-2"
    >
      <AInput
        :value="item"
        :placeholder="placeholder"
        class="flex-1"
        @update:value="(value: string) => updateItem(index, value)"
      />
      <AButton
        v-if="items.length > 1"
        type="link"
        danger
        @click="removeItem(index)"
      >
        {{ $gettext('Remove') }}
      </AButton>
    </div>
    <AButton
      block
      @click="addItem"
    >
      {{ addButtonText ?? $gettext('Add Item') }}
    </AButton>
  </div>
</template>
