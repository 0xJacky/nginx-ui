import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { JSXElements } from '@/types'
import { datetimeRender } from '@uozi-admin/curd'
import { Badge, Tag, Tooltip } from 'antdv-next'
import dayjs from 'dayjs'
import { AutoCertState, formatPrivateKeyType } from '@/constants'

const columns: StdTableColumn[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pure: true,
  customRender: (args: CustomRenderArgs) => {
    const { text, record } = args
    if (!text)
      return h('div', record.domain)

    return h('div', text)
  },
  search: {
    type: 'input',
  },
}, {
  title: () => $gettext('Type'),
  dataIndex: 'auto_cert',
  customRender: ({ text }: CustomRenderArgs) => {
    const template: JSXElements = []
    const sync = $gettext('Sync Certificate')
    const managed = $gettext('Managed Certificate')
    const general = $gettext('General Certificate')
    const selfSigned = $gettext('Self-signed Certificate')
    if (text === true || text === AutoCertState.Enable) {
      template.push(
        <Tag variant="filled" color="processing">
          {managed}
        </Tag>,
      )
    }
    else if (text === AutoCertState.Sync) {
      template.push(
        <Tag variant="filled" color="success">
          {sync}
        </Tag>,
      )
    }
    else if (text === AutoCertState.SelfSigned) {
      template.push(
        <Tag variant="filled" color="cyan">
          {selfSigned}
        </Tag>,
      )
    }
    else {
      template.push(
        <Tag variant="filled" color="purple">
          {general}
        </Tag>,
      )
    }
    return h('div', template)
  },
  sorter: true,
  pure: true,
}, {
  title: () => $gettext('Key Type'),
  dataIndex: 'key_type',
  customRender: ({ text }: CustomRenderArgs) => formatPrivateKeyType(text),
  sorter: true,
  pure: true,
}, {
  title: () => $gettext('Status'),
  dataIndex: 'status',
  pure: true,
  customRender: (args: CustomRenderArgs) => {
    const { record } = args
    if (record.status === 'pending') {
      return h('div', [
        h(Badge, { status: 'processing' }),
        h('span', $gettext('Issuing...')),
      ])
    }
    if (record.status === 'failure') {
      const errorMsg = record.last_error || $gettext('Issuance failed')
      return h(Tooltip, { title: errorMsg }, () =>
        h('div', [
          h(Badge, { status: 'error' }),
          h('span', $gettext('Failed')),
        ]))
    }
    const deployment = record.deployment_status
    if (deployment?.state === 'legacy_drift' || deployment?.state === 'mismatch') {
      const label = deployment.state === 'legacy_drift'
        ? $gettext('Automatic migration pending')
        : $gettext('Configuration mismatch')
      const configuredPaths = deployment.configured_certificate_paths?.join(', ') || '-'
      const managedPath = deployment.managed_certificate_path || '-'
      const title = $gettext('Configured path: %{configured}; managed path: %{managed}', {
        configured: configuredPaths,
        managed: managedPath,
      })
      return h(Tooltip, { title }, () =>
        h('div', [
          h(Badge, { status: 'warning' }),
          h('span', label),
        ]))
    }
    if (deployment?.state === 'unreadable' && deployment.error) {
      return h(Tooltip, { title: deployment.error }, () =>
        h('div', [
          h(Badge, { status: 'warning' }),
          h('span', $gettext('Unable to verify deployment')),
        ]))
    }
    const info = record.certificate_info
    const valid = info?.not_before
      && info?.not_after
      && !dayjs().isBefore(info.not_before)
      && !dayjs().isAfter(info.not_after)
    if (valid) {
      return h('div', [
        h(Badge, { status: 'success' }),
        h('span', $gettext('Valid')),
      ])
    }
    return h('div', [
      h(Badge, { status: 'error' }),
      h('span', $gettext('Expired')),
    ])
  },
}, {
  title: () => $gettext('Not After'),
  dataIndex: ['certificate_info', 'not_after'],
  customRender: datetimeRender,
  sorter: true,
  pure: true,
}, {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  fixed: 'right',
}]

export default columns
