<script setup lang="ts">
import type { QuickConfig } from './useQuickConfig'

defineOptions({ name: 'QuickSetupForm' })

const props = withDefaults(defineProps<{
  quick: QuickConfig
  showName?: boolean
}>(), {
  showName: true,
})

const { state, quickDerivedName, quickNameTouched } = props.quick
</script>

<template>
  <AForm layout="vertical">
    <AFormItem
      v-if="showName !== false"
      :label="$gettext('Configuration Name')"
    >
      <AInput
        v-model:value="state.name"
        :placeholder="quickDerivedName"
        @change="quickNameTouched = true"
      />
    </AFormItem>

    <AFormItem :label="$gettext('Type')">
      <ARadioGroup
        v-model:value="state.type"
        button-style="solid"
      >
        <ARadioButton value="reverse_proxy">
          {{ $gettext('Reverse Proxy') }}
        </ARadioButton>
        <ARadioButton value="static">
          {{ $gettext('Static Site') }}
        </ARadioButton>
        <ARadioButton value="redirect">
          {{ $gettext('Redirect') }}
        </ARadioButton>
      </ARadioGroup>
    </AFormItem>

    <AFormItem
      :label="$gettext('Domains')"
      required
    >
      <AInput
        v-model:value="state.domains"
        :placeholder="$gettext('example.com www.example.com')"
      />
    </AFormItem>

    <template v-if="state.type === 'reverse_proxy'">
      <AFormItem :label="$gettext('Scheme')">
        <ASelect v-model:value="state.rpScheme">
          <ASelectOption value="http">
            {{ $gettext('HTTP') }}
          </ASelectOption>
          <ASelectOption value="https">
            {{ $gettext('HTTPS') }}
          </ASelectOption>
        </ASelect>
      </AFormItem>

      <AFormItem
        :label="$gettext('Host')"
        required
      >
        <AInput v-model:value="state.rpHost" />
      </AFormItem>

      <AFormItem
        :label="$gettext('Port')"
        required
      >
        <AInput v-model:value="state.rpPort" />
      </AFormItem>

      <AFormItem :label="$gettext('Enable WebSocket')">
        <ASwitch v-model:checked="state.rpWebSocket" />
      </AFormItem>

      <AFormItem :label="$gettext('Client Max Body Size')">
        <AInput v-model:value="state.rpMaxBodySize" />
      </AFormItem>
    </template>

    <template v-else-if="state.type === 'static'">
      <AFormItem
        :label="$gettext('Web Root')"
        required
      >
        <AInput
          v-model:value="state.stWebRoot"
          placeholder="/var/www/html"
        />
      </AFormItem>

      <AFormItem :label="$gettext('Index')">
        <AInput v-model:value="state.stIndex" />
      </AFormItem>

      <AFormItem :label="$gettext('Single Page Application Fallback')">
        <ASwitch v-model:checked="state.stSpa" />
      </AFormItem>
    </template>

    <template v-else>
      <AFormItem
        :label="$gettext('Target URL')"
        required
      >
        <AInput
          v-model:value="state.rdTarget"
          placeholder="https://new.example.com"
        />
      </AFormItem>

      <AFormItem :label="$gettext('Status Code')">
        <ASelect v-model:value="state.rdStatus">
          <ASelectOption value="301">
            301 Moved Permanently
          </ASelectOption>
          <ASelectOption value="302">
            302 Found
          </ASelectOption>
          <ASelectOption value="308">
            308 Permanent Redirect
          </ASelectOption>
        </ASelect>
      </AFormItem>
    </template>

    <template v-if="state.type !== 'redirect'">
      <AFormItem :label="$gettext('Enable TLS')">
        <ASwitch v-model:checked="state.enableTLS" />
      </AFormItem>

      <AFormItem
        v-if="state.enableTLS"
        :label="$gettext('Redirect HTTP to HTTPS')"
      >
        <ASwitch v-model:checked="state.redirectHTTPToHTTPS" />
      </AFormItem>
    </template>
  </AForm>
</template>
