import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { JSX } from 'vue/jsx-runtime'
import type { Node } from '@/api/node'
import { ExclamationCircleOutlined, InfoCircleOutlined } from '@ant-design/icons-vue'
import { datetimeRender } from '@uozi-admin/curd'
import { Badge, InputPassword, Popover, Tag } from 'ant-design-vue'
import { h } from 'vue'
import nodeApi from '@/api/node'
import { SensitiveInput } from '@/components/SensitiveString'
import NodeAuthStatus from './NodeAuthStatus.vue'

function renderConnectionErrorContent(record: Node) {
  if (record.connection_error_code !== 'clock_skew') {
    return (
      <div class="max-w-[400px] text-sm leading-6">
        <p class="m-0">
          {$gettext('Check the node URL, network, TLS certificate, and authentication settings.')}
        </p>
        <details class="mt-3">
          <summary class="cursor-pointer select-none font-medium">
            {$gettext('Technical details')}
          </summary>
          <pre class="mb-0 mt-2 max-h-48 overflow-auto whitespace-pre-wrap break-all rounded bg-gray-100 p-3 text-xs dark:bg-gray-800">
            {record.connection_error}
          </pre>
        </details>
      </div>
    )
  }

  return (
    <div class="w-[400px] max-w-[calc(100vw-48px)] text-sm leading-6">
      <p class="m-0">
        {$gettext('The controller time is earlier than the node certificate validity start time. TLS stops the connection before node authentication.')}
      </p>
      <ol class="my-3 pl-5">
        <li>{$gettext('Synchronize the host system time on both the controller and the node.')}</li>
        <li>{$gettext('If Nginx UI runs in Docker, correct the Docker host clock instead of the container clock.')}</li>
      </ol>
      <p class="mb-1 mt-0 font-medium">
        {$gettext('On Linux hosts, check and enable network time synchronization:')}
      </p>
      <pre class="my-0 overflow-auto rounded bg-gray-100 px-3 py-2 text-xs dark:bg-gray-800">
        {'timedatectl status\nsudo timedatectl set-ntp true'}
      </pre>
      <p class="mb-0 mt-3 text-gray-600 dark:text-gray-300">
        {$gettext('The node will reconnect automatically after the clocks are synchronized.')}
      </p>
      <details class="mt-3">
        <summary class="cursor-pointer select-none font-medium">
          {$gettext('Technical details')}
        </summary>
        <pre class="mb-0 mt-2 max-h-48 overflow-auto whitespace-pre-wrap break-all rounded bg-gray-100 p-3 text-xs dark:bg-gray-800">
          {record.connection_error}
        </pre>
      </details>
    </div>
  )
}

function renderConnectionError(record: Node) {
  const isClockSkew = record.connection_error_code === 'clock_skew'
  const title = isClockSkew
    ? $gettext('System clocks are out of sync')
    : $gettext('Node connection failed')

  return h(Popover, {
    content: renderConnectionErrorContent(record),
    placement: 'rightTop',
    title: h('div', { class: 'flex items-center gap-2' }, [
      h(ExclamationCircleOutlined, { class: isClockSkew ? 'text-orange-500' : 'text-red-500' }),
      h('span', title),
    ]),
    trigger: ['hover', 'focus', 'click'],
  }, () => h('button', {
    'type': 'button',
    'aria-label': title,
    'class': 'ml-1 inline-flex cursor-help items-center border-0 bg-transparent p-0 text-red-500',
  }, h(InfoCircleOutlined)))
}

const columns: StdTableColumn[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pure: true,
  edit: {
    type: 'input',
  },
  search: true,
  width: 200,
}, {
  title: () => $gettext('URL'),
  dataIndex: 'url',
  sorter: true,
  pure: true,
  edit: {
    type: 'input',
    input: {
      placeholder: 'https://10.0.0.1:9000',
    },
  },
  width: 260,
}, {
  // The node secret is what a relationship starts from: the controller signs
  // its requests with it, then swaps itself onto a dedicated key pair in the
  // background. Copy it from the target node's Node Settings page.
  title: () => $gettext('Node Secret'),
  dataIndex: 'legacy_secret',
  edit: {
    // An existing node stores a secret worth reading back, so it gets the
    // reveal-after-2FA input. A node being added has nothing to reveal yet.
    type: (context: { formData: Node }) => {
      if (context.formData.legacy_secret === undefined)
        context.formData.legacy_secret = ''

      return context.formData.id
        ? (
            <SensitiveInput
              v-model={context.formData.legacy_secret}
              placeholder={$gettext('Leave blank for no change')}
              resolve={() => nodeApi.getSecret(context.formData.id).then(({ value }) => value)}
            />
          )
        : <InputPassword v-model:value={context.formData.legacy_secret} />
    },
  },
  hiddenInTable: true,
  hiddenInDetail: true,
}, {
  title: () => $gettext('Version'),
  dataIndex: 'version',
  pure: true,
  width: 120,
}, {
  title: () => $gettext('Authentication'),
  dataIndex: 'auth_method',
  customRender: ({ record }: CustomRenderArgs) => <NodeAuthStatus node={record as Node} />,
  pure: true,
  width: 160,
}, {
  title: () => $gettext('Status'),
  dataIndex: 'status',
  customRender: (args: CustomRenderArgs) => {
    const template: JSX.Element[] = []
    const { text } = args
    if (args.record.enabled) {
      if (text === true || text > 0) {
        template.push(<Badge status="success" />)
        template.push(<span>{$gettext('Online')}</span>)
      }
      else {
        template.push(<Badge status="error" />)
        template.push(<span>{$gettext('Offline')}</span>)
      }
    }
    else {
      template.push(<Badge status="default" />)
      template.push(<span>{$gettext('Disabled')}</span>)
    }

    if (args.record.connection_error)
      template.push(renderConnectionError(args.record as Node))

    return h('div', { class: 'flex items-center' }, template)
  },
  sorter: true,
  pure: true,
  width: 120,
}, {
  title: () => $gettext('Enabled'),
  dataIndex: 'enabled',
  customRender: (args: CustomRenderArgs) => {
    const template: JSX.Element[] = []
    const { text } = args
    if (text === true || text > 0)
      template.push(<Tag color="green">{$gettext('Enabled')}</Tag>)

    else
      template.push(<Tag color="orange">{$gettext('Disabled')}</Tag>)

    return h('div', template)
  },
  edit: {
    type: 'switch',
  },
  sorter: true,
  pure: true,
  width: 120,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetimeRender,
  sorter: true,
  pure: true,
  width: 150,
}, {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  fixed: 'right',
  width: 200,
}]

export default columns
