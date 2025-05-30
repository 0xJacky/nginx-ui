import type { TaskReport } from './tasks'
import { debounce } from 'lodash'
import selfCheck, { ReportStatus } from '@/api/self_check'
import frontendTasks from './tasks/frontend'

export const useSelfCheckStore = defineStore('selfCheck', () => {
  const data = ref<TaskReport[]>([])

  const loading = ref(false)

  async function __check() {
    if (loading.value)
      return

    loading.value = true
    try {
      const backendReports = (await selfCheck.run()).map(r => {
        return {
          key: r.key,
          name: () => $gettext(r.name.message, r.name.args),
          description: () => $gettext(r.description.message, r.description.args),
          type: 'backend' as const,
          status: r.status,
          fixable: r.fixable,
          err: r.err,
        }
      })
      const frontendReports = await Promise.all(
        Object.entries(frontendTasks).map(async ([key, task]) => {
          return {
            key,
            name: task.name,
            description: task.description,
            type: 'frontend' as const,
            status: await task.check(),
            fixable: false,
          }
        }),
      )
      data.value = [...backendReports, ...frontendReports]
    }
    catch (error) {
      console.error(error)
    }
    finally {
      loading.value = false
    }
  }

  const check = debounce(__check, 1000, {
    leading: true,
    trailing: false,
  })

  const fixing = ref<Record<string, boolean>>({})

  async function fix(taskName: string) {
    if (fixing.value[taskName])
      return

    fixing.value[taskName] = true
    try {
      await selfCheck.fix(taskName)
    }
    finally {
      setTimeout(() => {
        check()
        fixing.value[taskName] = false
      }, 1000)
    }
  }

  const hasError = computed(() => {
    return data.value?.some(item => item.status === ReportStatus.Error)
  })

  return { data, loading, fixing, hasError, check, fix }
})
