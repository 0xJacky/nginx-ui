import type { TaskReport } from './tasks'
import { debounce } from 'lodash'
import { taskManager } from './tasks'

export const useSelfCheckStore = defineStore('selfCheck', () => {
  const data = ref<TaskReport[]>([])

  const requestError = ref(false)
  const loading = ref(false)

  async function __check() {
    if (loading.value)
      return

    loading.value = true
    try {
      data.value = await taskManager.runAllChecks()
    }
    catch (error) {
      console.error(error)
      requestError.value = true
    }
    finally {
      loading.value = false
    }
  }

  const check = debounce(__check, 1000, {
    leading: true,
    trailing: false,
  })

  const fixing = reactive({})

  async function fix(taskName: string) {
    if (fixing[taskName])
      return

    fixing[taskName] = true
    try {
      await taskManager.fixTask(taskName)
      check()
    }
    finally {
      fixing[taskName] = false
    }
  }

  const hasError = computed(() => {
    return requestError.value || data.value?.some(item => item.status === 'error')
  })

  return { data, loading, fixing, hasError, check, fix }
})
