<script setup lang="ts">
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'

const props = defineProps<{
  time?: string | number
}>()

dayjs.extend(relativeTime)

const text = ref('')

const time = computed(() => {
  if (!props.time)
    return ''

  if (typeof props.time === 'number')
    return props.time

  return Number.parseInt(props.time)
})

let timer: NodeJS.Timeout
let step: number = 1

async function computedText() {
  if (!time.value)
    return

  // if time is not today, return the datetime
  const thatDay = dayjs.unix(time.value).format('YYYY-MM-DD')
  if (dayjs().format('YYYY-MM-DD') !== dayjs.unix(time.value).format('YYYY-MM-DD')) {
    clearInterval(timer)
    text.value = thatDay

    return
  }

  text.value = dayjs.unix(time.value).fromNow()

  clearInterval(timer)

  timer = setInterval(computedText, step * 60 * 1000)

  step += 5

  if (step >= 60)
    step = 60
}

onMounted(computedText)
watch(() => props.time, computedText)
</script>

<template>
  <div class="reactive-time inline">
    {{ text }}
  </div>
</template>

<style scoped lang="less">

</style>
