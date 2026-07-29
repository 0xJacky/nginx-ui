import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { JSX } from 'vue/jsx-runtime'
import type { Node } from '@/api/node'
import { datetimeRender } from '@uozi-admin/curd'
import { Badge, InputPassword, Tag } from 'ant-design-vue'
import { h } from 'vue'
import nodeApi from '@/api/node'
import { SensitiveInput } from '@/components/SensitiveString'

// Only a credential that is not simply healthy is worth its own word: the
// remaining states collapse into naming the authentication method itself.
const unhealthyCredentialMap: Record<string, { color: string, text: () => string }> = {
  rotating: { color: 'blue', text: () => $gettext('Rotating') },
  unpaired: { color: 'default', text: () => $gettext('Unpaired') },
  revoked: { color: 'red', text: () => $gettext('Revoked') },
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
  customRender: ({ record }: CustomRenderArgs) => {
    if (record.auth_method !== 'paired_ed25519')
      return <Tag color="orange" class="m-0">{$gettext('Legacy secret')}</Tag>

    const unhealthy = unhealthyCredentialMap[record.credential_status as string]
    if (unhealthy)
      return <Tag color={unhealthy.color} class="m-0">{unhealthy.text()}</Tag>

    return <Tag color="green" class="m-0">{$gettext('Paired signature')}</Tag>
  },
  pure: true,
  width: 140,
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

    return h('div', template)
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
