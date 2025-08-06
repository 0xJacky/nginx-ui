import type { CustomRenderArgs } from '@uozi-admin/curd'
import type { PropType } from 'vue'
import type { CosyError } from '@/lib/http/types'
import { defineComponent, ref } from 'vue'
import { NotificationTypeT } from '@/constants'
import { translateError } from '@/lib/http/error'
import notifications from './notifications'

// Helper function to parse and translate error
async function parseError(response: string): Promise<string | null> {
  try {
    const errorData = JSON.parse(response) as CosyError
    if (errorData.scope && errorData.code) {
      return await translateError(errorData)
    }
  }
  catch (error) {
    console.error('Failed to parse error response:', error)
  }
  return null
}

// Create a component for error details to properly handle async translation
const ErrorDetails = defineComponent({
  props: {
    response: {
      type: [String, Object] as PropType<string | object>,
      required: true,
    },
  },
  setup(props) {
    const translatedError = ref<string>('')
    const isLoading = ref(true)

    // Convert response to string if it's an object
    const responseString = typeof props.response === 'string'
      ? props.response
      : JSON.stringify(props.response)

    // Immediately start translation
    parseError(responseString).then(result => {
      if (result) {
        translatedError.value = result
      }
      isLoading.value = false
    })

    return () => {
      const parsedResponse = typeof props.response === 'string'
        ? JSON.parse(props.response)
        : props.response

      return (
        <div class="mt-2">
          {/* 显示翻译后的错误信息（如果有） */}
          {translatedError.value && (
            <div class="text-red-500 font-medium mb-2">
              {translatedError.value}
            </div>
          )}

          {/* 显示翻译状态 */}
          {isLoading.value && (
            <div class="text-gray-500 text-sm mb-2">
              {$gettext('Translating error...')}
            </div>
          )}

          {/* 默认显示原始错误信息 */}
          <details class="mt-2">
            <summary class="cursor-pointer text-sm text-gray-600 hover:text-gray-800">
              {$gettext('Error details')}
            </summary>
            <pre class="mt-2 p-2 bg-gray-100 rounded text-xs overflow-hidden whitespace-pre-wrap break-words max-w-full">
              {JSON.stringify(parsedResponse, null, 2)}
            </pre>
          </details>
        </div>
      )
    }
  },
})

export function detailRender(args: Pick<CustomRenderArgs, 'record' | 'text'>) {
  try {
    return (
      <div>
        <div>
          {
            notifications[args.record.title]?.content(args.record.details)
            || args.record.content || args.record.details
          }
        </div>
        {args.record.details?.response && args.record.type !== NotificationTypeT.Success && (
          <div>
            <ErrorDetails response={args.record.details.response} />
          </div>
        )}
      </div>
    )
  }
  catch {
    return args.text
  }
}
