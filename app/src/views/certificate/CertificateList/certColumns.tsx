import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { CertificateUsage } from '@/api/cert'
import type { ConfigUsageItem } from '@/components/ConfigUsage'
import type { JSXElements } from '@/types'
import { SyncOutlined } from '@antdv-next/icons'
import { datetimeRender } from '@uozi-admin/curd'
import { Tag, Tooltip } from 'antdv-next'
import dayjs from 'dayjs'
import ConfigUsage from '@/components/ConfigUsage'
import { AutoCertState, formatPrivateKeyType } from '@/constants'
import { certStateLabel, certStateTone } from '../certState'

function toConfigUsages(usedBy: CertificateUsage[] = []): ConfigUsageItem[] {
  return usedBy.map(usage => ({
    key: `${usage.kind}:${usage.name}`,
    label: usage.name,
    kind: usage.kind,
    to: `/${usage.kind}s/${encodeURIComponent(usage.name)}`,
    status: usage.status,
  }))
}

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
    else if (text === AutoCertState.Paused) {
      template.push(
        <Tag variant="filled" color="processing">
          {managed}
        </Tag>,
        <Tag variant="filled" color="default">
          {$gettext('Renewal paused')}
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
      return h(Tag, { color: 'processing', icon: h(SyncOutlined, { spin: true }) }, () => $gettext('Issuing...'))
    }
    if (record.status === 'failure') {
      const errorMsg = record.last_error || $gettext('Issuance failed')
      return h(Tooltip, { title: errorMsg }, () =>
        h(Tag, { color: 'error' }, () => $gettext('Failed')))
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
        h(Tag, { color: 'warning' }, () => label))
    }
    if (deployment?.state === 'unreadable' && deployment.error) {
      return h(Tooltip, { title: deployment.error }, () =>
        h(Tag, { color: 'warning' }, () => $gettext('Unable to verify deployment')))
    }
    // Newer backends compute the state, which tells a certificate that was
    // never issued apart from an expired one.
    if (record.state) {
      const tone = certStateTone(record.state)
      return h(Tag, { color: tone === 'default' ? undefined : tone }, () => certStateLabel(record))
    }

    const info = record.certificate_info
    if (!info)
      return h(Tag, {}, () => $gettext('Not issued yet'))
    const valid = info.not_before
      && info.not_after
      && !dayjs().isBefore(info.not_before)
      && !dayjs().isAfter(info.not_after)
    if (valid)
      return h(Tag, { color: 'success' }, () => $gettext('Valid'))

    return h(Tag, { color: 'error' }, () => $gettext('Expired'))
  },
}, {
  title: () => $gettext('Not After'),
  dataIndex: ['certificate_info', 'not_after'],
  customRender: datetimeRender,
  sorter: true,
  pure: true,
}, {
  title: () => $gettext('Used By'),
  dataIndex: 'used_by',
  pure: true,
  customRender: ({ record }: CustomRenderArgs) => {
    const usages = toConfigUsages(record.used_by)
    return h(ConfigUsage, {
      usages,
      summary: $ngettext('Used by %{count} configuration', 'Used by %{count} configurations', usages.length, { count: String(usages.length) }),
    })
  },
}, {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  fixed: 'right',
}]

export default columns
