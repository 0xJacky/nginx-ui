<script setup lang="ts">
const modelValue = defineModel<string>({ default: '' })

interface CronConfig {
  type: 'daily' | 'weekly' | 'monthly' | 'custom'
  hour: number
  minute: number
  dayOfWeek?: number // 0-6, 0 = Sunday
  dayOfMonth?: number // 1-31
}

const cronConfig = ref<CronConfig>({
  type: 'daily',
  hour: 0,
  minute: 0,
})

const cronTypes = [
  { label: $gettext('Daily'), value: 'daily' },
  { label: $gettext('Weekly'), value: 'weekly' },
  { label: $gettext('Monthly'), value: 'monthly' },
//  { label: $gettext('Custom'), value: 'custom' },
]

const weekDays = [
  { label: $gettext('Sunday'), value: 0 },
  { label: $gettext('Monday'), value: 1 },
  { label: $gettext('Tuesday'), value: 2 },
  { label: $gettext('Wednesday'), value: 3 },
  { label: $gettext('Thursday'), value: 4 },
  { label: $gettext('Friday'), value: 5 },
  { label: $gettext('Saturday'), value: 6 },
]

const customCronExpression = ref('')

// Parse cron expression to config
function parseCronExpression(cron: string) {
  if (!cron)
    return

  const parts = cron.trim().split(/\s+/)
  if (parts.length !== 5) {
    cronConfig.value.type = 'custom'
    customCronExpression.value = cron
    return
  }

  const [minute, hour, dayOfMonth, month, dayOfWeek] = parts

  cronConfig.value.minute = Number.parseInt(minute) || 0
  cronConfig.value.hour = Number.parseInt(hour) || 0

  // Check if it's a daily pattern (every day)
  if (dayOfMonth === '*' && month === '*' && dayOfWeek === '*') {
    cronConfig.value.type = 'daily'
    return
  }

  // Check if it's a weekly pattern (specific day of week)
  if (dayOfMonth === '*' && month === '*' && dayOfWeek !== '*') {
    cronConfig.value.type = 'weekly'
    cronConfig.value.dayOfWeek = Number.parseInt(dayOfWeek) || 0
    return
  }

  // Check if it's a monthly pattern (specific day of month)
  if (dayOfMonth !== '*' && month === '*' && dayOfWeek === '*') {
    cronConfig.value.type = 'monthly'
    cronConfig.value.dayOfMonth = Number.parseInt(dayOfMonth) || 1
    return
  }

  // Otherwise, it's custom
  cronConfig.value.type = 'custom'
  customCronExpression.value = cron
}

// Generate cron expression from config
function generateCronExpression() {
  const { type, minute, hour, dayOfWeek, dayOfMonth } = cronConfig.value

  switch (type) {
    case 'daily':
      return `${minute} ${hour} * * *`
    case 'weekly':
      return `${minute} ${hour} * * ${dayOfWeek ?? 0}`
    case 'monthly':
      return `${minute} ${hour} ${dayOfMonth ?? 1} * *`
    case 'custom':
      return customCronExpression.value
    default:
      return `${minute} ${hour} * * *`
  }
}

// Watch for changes and update model value
watch(cronConfig, () => {
  if (cronConfig.value.type !== 'custom') {
    modelValue.value = generateCronExpression()
  }
}, { deep: true })

watch(customCronExpression, newValue => {
  if (cronConfig.value.type === 'custom') {
    modelValue.value = newValue
  }
})

// Initialize from model value
watch(modelValue, newValue => {
  if (newValue) {
    parseCronExpression(newValue)
  }
}, { immediate: true })

// Human readable description
const cronDescription = computed(() => {
  const { type, hour, minute, dayOfWeek, dayOfMonth } = cronConfig.value
  const timeStr = `${hour.toString().padStart(2, '0')}:${minute.toString().padStart(2, '0')}`
  const dayName = weekDays.find(d => d.value === dayOfWeek)?.label || $gettext('Sunday')

  switch (type) {
    case 'daily':
      return $gettext('Execute on every day at %{time}', { time: timeStr })
    case 'weekly':
      return $gettext('Execute on every %{day} at %{time}', { day: dayName, time: timeStr })
    case 'monthly':
      return $gettext('Execute on every month on day %{day} at %{time}', { day: dayOfMonth?.toString() || '1', time: timeStr })
    case 'custom':
      return customCronExpression.value || $gettext('Custom cron expression')
    default:
      return ''
  }
})
</script>

<template>
  <div>
    <div class="font-500 mb-4">
      {{ $gettext('Backup Schedule') }}
    </div>

    <AFormItem :label="$gettext('Schedule Type')">
      <ASelect v-model:value="cronConfig.type" :options="cronTypes" />
    </AFormItem>

    <AAlert
      v-if="cronDescription"
      :message="cronDescription"
      type="info"
      show-icon
      class="mb-4"
    />

    <template v-if="cronConfig.type !== 'custom'">
      <ARow :gutter="16">
        <ACol :span="12">
          <AFormItem :label="$gettext('Hour')">
            <AInputNumber
              v-model:value="cronConfig.hour"
              :min="0"
              :max="23"
              style="width: 100%"
            />
          </AFormItem>
        </ACol>
        <ACol :span="12">
          <AFormItem :label="$gettext('Minute')">
            <AInputNumber
              v-model:value="cronConfig.minute"
              :min="0"
              :max="59"
              style="width: 100%"
            />
          </AFormItem>
        </ACol>
      </ARow>

      <AFormItem v-if="cronConfig.type === 'weekly'" :label="$gettext('Day of Week')">
        <ASelect v-model:value="cronConfig.dayOfWeek" :options="weekDays" />
      </AFormItem>

      <AFormItem v-if="cronConfig.type === 'monthly'" :label="$gettext('Day of Month')">
        <AInputNumber
          v-model:value="cronConfig.dayOfMonth"
          :min="1"
          :max="31"
          style="width: 100%"
        />
      </AFormItem>
    </template>

    <AFormItem v-if="cronConfig.type === 'custom'" :label="$gettext('Cron Expression')">
      <AInput
        v-model:value="customCronExpression"
        :placeholder="$gettext('e.g., 0 0 * * * (daily at midnight)')"
      />
      <div class="mt-2 text-gray-500 text-sm">
        {{ $gettext('Format: minute hour day month weekday') }}
      </div>
    </AFormItem>
  </div>
</template>

<style scoped lang="less">
</style>
