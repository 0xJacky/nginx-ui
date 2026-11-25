import type { TableColumnsType } from 'antdv-next'
import type { Cert } from '@/api/cert'
import { InfoCircleOutlined } from '@antdv-next/icons'
import { Tag, Tooltip } from 'antdv-next'
import dayjs from 'dayjs'
import { certRenewalMethodLabel, certStateLabel, certStateTone, splitVisible } from '../certState'
import { renderCertUsage } from '../certUsage'

// The columns of the certificate list. They live outside the page so a test
// can check which ones it shows. Build them inside a computed, so the titles
// follow a language switch. The desktop widths add up to 1150, which fits a
// 1440px window next to the open sidebar without scrolling sideways.

type EditCert = (record: Cert) => void

function toneColor(record: Cert) {
  return `var(--ant-color-${{
    success: 'success',
    warning: 'warning',
    error: 'error',
    processing: 'primary',
    default: 'text-quaternary',
  }[certStateTone(record.state)]})`
}

function renderState(record: Cert) {
  const label = (
    <span
      class="cert-state"
      style={{
        color: toneColor(record),
      }}
    >
      <span class="cert-state-dot" />
      {certStateLabel(record)}
      {record.state === 'failed' && record.last_renewal_error && <InfoCircleOutlined />}
    </span>
  )
  if (record.state === 'failed' && record.last_renewal_error)
    return <Tooltip title={record.last_renewal_error}>{label}</Tooltip>
  return label
}

function expiryDate(record: Cert) {
  return record.certificate_info?.not_after
    ? dayjs(record.certificate_info.not_after).format('YYYY-MM-DD')
    : '-'
}

function actionsColumn(): TableColumnsType<Cert>[number] {
  return {
    title: $gettext('Actions'),
    key: 'actions',
    fixed: 'right',
    align: 'right',
    width: 130,
  }
}

// A certificate issued for a node names that node, since it does not serve
// any configuration of this instance.
function renderName(record: Cert, onEdit: EditCert) {
  const link = <a onClick={() => onEdit(record)}>{record.name || record.domains?.[0] || '-'}</a>
  if (!record.delegated_node_id)
    return link
  const node = record.delegated_node_name || `#${record.delegated_node_id}`
  return (
    <div class="cert-name">
      {link}
      <Tooltip title={$gettext('Issued by this instance for %{node} and renewed here', { node })}>
        <Tag class="m-0" color="geekblue" variant="filled">{$gettext('For %{node}', { node })}</Tag>
      </Tooltip>
    </div>
  )
}

export function certListColumns(onEdit: EditCert): TableColumnsType<Cert> {
  return [
    {
      title: $gettext('Name'),
      dataIndex: 'name',
      ellipsis: true,
      width: 200,
      render: (_: unknown, record: Cert) => renderName(record, onEdit),
    },
    {
      title: $gettext('Domains'),
      dataIndex: 'domains',
      width: 250,
      render: (_: unknown, record: Cert) => {
        const { visible, hidden } = splitVisible(record.domains ?? [], 2)
        if (!visible.length)
          return '-'
        return (
          <div class="cert-domains">
            {visible.map(domain => <Tag class="m-0 font-mono">{domain}</Tag>)}
            {hidden > 0 && (
              <Tooltip title={(record.domains ?? []).slice(2).join(', ')}>
                <Tag class="m-0">{`+${hidden}`}</Tag>
              </Tooltip>
            )}
          </div>
        )
      },
    },
    {
      title: $gettext('Status'),
      dataIndex: 'state',
      width: 155,
      render: (_: unknown, record: Cert) => renderState(record),
    },
    {
      title: $gettext('Renewal method'),
      dataIndex: 'renewal_method',
      width: 145,
      ellipsis: true,
      render: (_: unknown, record: Cert) => certRenewalMethodLabel(record.renewal_method, record.dns_provider),
    },
    {
      title: $gettext('Expiry date'),
      dataIndex: ['certificate_info', 'not_after'],
      width: 110,
      render: (_: unknown, record: Cert) => expiryDate(record),
    },
    {
      title: $gettext('Used By'),
      dataIndex: 'used_by',
      width: 160,
      render: (_: unknown, record: Cert) => renderCertUsage(record.used_by),
    },
    actionsColumn(),
  ]
}

// Phones get one column with the status and expiry under the name, so the
// status stays visible without scrolling sideways.
export function certListNarrowColumns(onEdit: EditCert): TableColumnsType<Cert> {
  return [
    {
      title: $gettext('Name'),
      dataIndex: 'name',
      render: (_: unknown, record: Cert) => (
        <div class="min-w-0">
          {renderName(record, onEdit)}
          <div class="cert-narrow-meta">
            {renderState(record)}
            {record.certificate_info?.not_after && <span class="cert-narrow-expiry">{expiryDate(record)}</span>}
          </div>
        </div>
      ),
    },
    {
      ...actionsColumn(),
      fixed: undefined,
      width: 110,
    },
  ]
}
