import type { TaskReport } from './tasks'
import type { SelfCheckAccessOptions } from '@/api/self_check'
import { debounce } from 'lodash'
import selfCheck, { ReportStatus } from '@/api/self_check'
import frontendTasks from './tasks/frontend'

export const useSelfCheckStore = defineStore('selfCheck', () => {
  const data = ref<TaskReport[]>([])

  const loading = ref(false)
  const checked = ref(false)
  const accessError = ref('')

  function getFrontendDebugReports(): TaskReport[] {
    return [
      {
        key: 'Frontend-Debug-Secret',
        name: () => $gettext('Install Secret'),
        description: () => $gettext('Frontend debug mode is active. Secret verification is mocked locally and no backend request is sent.'),
        status: ReportStatus.Success,
      },
      {
        key: 'Frontend-Debug-Preview',
        name: () => $gettext('Install Flow Preview'),
        description: () => $gettext('This mode only previews the installation UI flow and does not change any server state.'),
        status: ReportStatus.Warning,
      },
    ]
  }

  async function __check(options?: SelfCheckAccessOptions) {
    if (loading.value)
      return

    if (options?.setupAuth && !options.installSecret?.trim()) {
      data.value = []
      checked.value = false
      accessError.value = ''
      return
    }

    if (options?.debugMode === 'frontend') {
      data.value = getFrontendDebugReports()
      checked.value = true
      accessError.value = ''
      return
    }

    loading.value = true
    accessError.value = ''
    try {
      const backendReports = (await selfCheck.run(options)).map(r => {
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
            status: await task.check(options),
            fixable: false,
          }
        }),
      )
      data.value = [...backendReports, ...frontendReports]
      checked.value = true
    }
    catch (error) {
      console.error(error)
      data.value = []
      checked.value = false
      accessError.value = error instanceof Error
        ? error.message
        : ((error as { message?: string })?.message || String(error))
    }
    finally {
      loading.value = false
    }
  }

  async function runCheck(options?: SelfCheckAccessOptions) {
    await __check(options)
  }

  const check = debounce(__check, 1000, {
    leading: true,
    trailing: false,
  })

  const fixing = ref<Record<string, boolean>>({})

  async function fix(taskName: string, options?: SelfCheckAccessOptions) {
    if (fixing.value[taskName])
      return

    fixing.value[taskName] = true
    try {
      await selfCheck.fix(taskName, options)
    }
    finally {
      setTimeout(() => {
        check(options)
        fixing.value[taskName] = false
      }, 1000)
    }
  }

  const hasError = computed(() => {
    return data.value?.some(item => item.status === ReportStatus.Error)
  })

  return { data, loading, fixing, checked, accessError, hasError, check, runCheck, fix }
})
