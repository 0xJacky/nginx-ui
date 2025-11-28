<script setup lang="ts">
import type { EnhancedHealthCheckConfig, HeaderItem, SiteInfo } from '@/api/site_navigation'
import { CloseOutlined, PlusOutlined } from '@ant-design/icons-vue'
import { siteNavigationApi } from '@/api/site_navigation'

interface Props {
  site?: SiteInfo
}

interface Emits {
  (e: 'save', config: EnhancedHealthCheckConfig): void
  (e: 'refresh'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()
const { message } = useGlobalApp()

const visible = defineModel<boolean>('open', { required: true })
const testing = ref(false)

const formData = ref<EnhancedHealthCheckConfig>({
  // Basic settings (health check is always enabled)
  enabled: true,
  interval: 300,
  timeout: 10,
  userAgent: 'Nginx-UI Enhanced Checker/2.0',
  maxRedirects: 3,
  followRedirects: true,
  checkFavicon: true,

  // Protocol settings
  protocol: 'http',
  method: 'GET',
  path: '/',
  headers: [],
  body: '',

  // Response validation
  expectedStatus: [200],
  expectedText: '',
  notExpectedText: '',
  validateSSL: false,
  verifyHostname: false,

  // gRPC settings
  grpcService: '',
  grpcMethod: 'Check',

  // Advanced settings
  dnsResolver: '',
  sourceIP: '',
  clientCert: '',
  clientKey: '',
})

interface StatusCodeOption {
  value: number
  label: string
}

interface StatusCodeGroup {
  title: string
  options: StatusCodeOption[]
}

function createStatusOption(code: number, description: string): StatusCodeOption {
  return {
    value: code,
    label: `${code} ${$gettext(description)}`,
  }
}

const statusCodeGroups: StatusCodeGroup[] = [
  {
    title: $gettext('Informational Responses (1xx)'),
    options: [
      createStatusOption(100, 'Continue'),
      createStatusOption(101, 'Switching Protocols'),
      createStatusOption(102, 'Processing'),
      createStatusOption(103, 'Early Hints'),
    ],
  },
  {
    title: $gettext('Successful Responses (2xx)'),
    options: [
      createStatusOption(200, 'OK'),
      createStatusOption(201, 'Created'),
      createStatusOption(202, 'Accepted'),
      createStatusOption(203, 'Non-Authoritative Information'),
      createStatusOption(204, 'No Content'),
      createStatusOption(205, 'Reset Content'),
      createStatusOption(206, 'Partial Content'),
      createStatusOption(207, 'Multi-Status'),
      createStatusOption(208, 'Already Reported'),
      createStatusOption(226, 'IM Used'),
    ],
  },
  {
    title: $gettext('Redirection Messages (3xx)'),
    options: [
      createStatusOption(300, 'Multiple Choices'),
      createStatusOption(301, 'Moved Permanently'),
      createStatusOption(302, 'Found'),
      createStatusOption(303, 'See Other'),
      createStatusOption(304, 'Not Modified'),
      createStatusOption(305, 'Use Proxy'),
      createStatusOption(306, 'Switch Proxy (Unused)'),
      createStatusOption(307, 'Temporary Redirect'),
      createStatusOption(308, 'Permanent Redirect'),
    ],
  },
  {
    title: $gettext('Client Error Responses (4xx)'),
    options: [
      createStatusOption(400, 'Bad Request'),
      createStatusOption(401, 'Unauthorized'),
      createStatusOption(402, 'Payment Required'),
      createStatusOption(403, 'Forbidden'),
      createStatusOption(404, 'Not Found'),
      createStatusOption(405, 'Method Not Allowed'),
      createStatusOption(406, 'Not Acceptable'),
      createStatusOption(407, 'Proxy Authentication Required'),
      createStatusOption(408, 'Request Timeout'),
      createStatusOption(409, 'Conflict'),
      createStatusOption(410, 'Gone'),
      createStatusOption(411, 'Length Required'),
      createStatusOption(412, 'Precondition Failed'),
      createStatusOption(413, 'Payload Too Large'),
      createStatusOption(414, 'URI Too Long'),
      createStatusOption(415, 'Unsupported Media Type'),
      createStatusOption(416, 'Range Not Satisfiable'),
      createStatusOption(417, 'Expectation Failed'),
      createStatusOption(418, "I'm a teapot"),
      createStatusOption(421, 'Misdirected Request'),
      createStatusOption(422, 'Unprocessable Content'),
      createStatusOption(423, 'Locked'),
      createStatusOption(424, 'Failed Dependency'),
      createStatusOption(425, 'Too Early'),
      createStatusOption(426, 'Upgrade Required'),
      createStatusOption(428, 'Precondition Required'),
      createStatusOption(429, 'Too Many Requests'),
      createStatusOption(431, 'Request Header Fields Too Large'),
      createStatusOption(451, 'Unavailable For Legal Reasons'),
    ],
  },
  {
    title: $gettext('Server Error Responses (5xx)'),
    options: [
      createStatusOption(500, 'Internal Server Error'),
      createStatusOption(501, 'Not Implemented'),
      createStatusOption(502, 'Bad Gateway'),
      createStatusOption(503, 'Service Unavailable'),
      createStatusOption(504, 'Gateway Timeout'),
      createStatusOption(505, 'HTTP Version Not Supported'),
      createStatusOption(506, 'Variant Also Negotiates'),
      createStatusOption(507, 'Insufficient Storage'),
      createStatusOption(508, 'Loop Detected'),
      createStatusOption(510, 'Not Extended'),
      createStatusOption(511, 'Network Authentication Required'),
    ],
  },
]

// Load existing config when site changes
watchEffect(async () => {
  if (props.site) {
    await loadExistingConfig()
  }
})

async function loadExistingConfig() {
  if (!props.site)
    return

  try {
    const config = await siteNavigationApi.getHealthCheck(props.site.id)

    // Convert backend config to frontend format
    formData.value = {
      // Basic settings
      enabled: config.health_check_enabled ?? true,
      interval: config.check_interval ?? 300,
      timeout: config.timeout ?? 10,
      userAgent: config.user_agent ?? 'Nginx-UI Enhanced Checker/2.0',
      maxRedirects: config.max_redirects ?? 3,
      followRedirects: config.follow_redirects ?? true,
      checkFavicon: config.check_favicon ?? true,

      // Protocol settings
      protocol: config.health_check_config?.protocol ?? 'http',
      method: config.health_check_config?.method ?? 'GET',
      path: config.health_check_config?.path ?? '/',
      headers: convertHeadersToArray(config.health_check_config?.headers ?? {}),
      body: config.health_check_config?.body ?? '',

      // Response validation
      expectedStatus: config.health_check_config?.expected_status ?? [200],
      expectedText: config.health_check_config?.expected_text ?? '',
      notExpectedText: config.health_check_config?.not_expected_text ?? '',
      validateSSL: config.health_check_config?.validate_ssl ?? false,
      verifyHostname: config.health_check_config?.verify_hostname ?? false,

      // gRPC settings
      grpcService: config.health_check_config?.grpc_service ?? '',
      grpcMethod: config.health_check_config?.grpc_method ?? 'Check',

      // Advanced settings
      dnsResolver: config.health_check_config?.dns_resolver ?? '',
      sourceIP: config.health_check_config?.source_ip ?? '',
      clientCert: config.health_check_config?.client_cert ?? '',
      clientKey: config.health_check_config?.client_key ?? '',
    }
  }
  catch (error) {
    console.error('Failed to load health check config:', error)
    // Fallback to defaults
    resetForm()
  }
}

function resetForm() {
  formData.value = {
    // Basic settings (health check is always enabled)
    enabled: true,
    interval: 300,
    timeout: 10,
    userAgent: 'Nginx-UI Enhanced Checker/2.0',
    maxRedirects: 3,
    followRedirects: true,
    checkFavicon: true,

    // Protocol settings
    protocol: 'http',
    method: 'GET',
    path: '/',
    headers: [],
    body: '',

    // Response validation
    expectedStatus: [200],
    expectedText: '',
    notExpectedText: '',
    validateSSL: false,
    verifyHostname: false,

    // gRPC settings
    grpcService: '',
    grpcMethod: 'Check',

    // Advanced settings
    dnsResolver: '',
    sourceIP: '',
    clientCert: '',
    clientKey: '',
  }
}

function convertHeadersToArray(headers: { [key: string]: string }): HeaderItem[] {
  return Object.entries(headers || {}).map(([name, value]) => ({ name, value }))
}

function isHttpProtocol(protocol: string): boolean {
  return ['http', 'https'].includes(protocol)
}

function isGrpcProtocol(protocol: string): boolean {
  return ['grpc', 'grpcs'].includes(protocol)
}

function isDefaultHttpPort(port: string, protocol: string): boolean {
  return (port === '80' && protocol === 'http')
    || (port === '443' && protocol === 'https')
    || !port
}

function isDefaultGrpcPort(port: string, protocol: string): boolean {
  return (port === '80' && protocol === 'grpc')
    || (port === '443' && protocol === 'grpcs')
}

function getGrpcDefaultPort(urlProtocol: string, protocol: string): string {
  return (urlProtocol === 'https:' || protocol === 'grpcs') ? '443' : '80'
}

function buildUrl(protocol: string, hostname: string, port?: string): string {
  return port ? `${protocol}://${hostname}:${port}` : `${protocol}://${hostname}`
}

function getHttpTestUrl(protocol: string, siteUrl: string): string {
  try {
    const url = new URL(siteUrl)
    const hostname = url.hostname
    const port = url.port

    if (isDefaultHttpPort(port, protocol)) {
      return buildUrl(protocol, hostname)
    }
    return buildUrl(protocol, hostname, port)
  }
  catch {
    return `${protocol}://${siteUrl}`
  }
}

function getGrpcTestUrl(protocol: string, siteUrl: string): string {
  try {
    const url = new URL(siteUrl)
    const hostname = url.hostname
    let port = url.port

    if (!port) {
      port = getGrpcDefaultPort(url.protocol, protocol)
    }

    if (isDefaultGrpcPort(port, protocol)) {
      return buildUrl(protocol, hostname)
    }
    return buildUrl(protocol, hostname, port)
  }
  catch {
    return `${protocol}://${siteUrl}`
  }
}

function getTestUrl(): string {
  if (!props.site) {
    return ''
  }

  const protocol = formData.value.protocol

  if (isHttpProtocol(protocol)) {
    return getHttpTestUrl(protocol, props.site.display_url || props.site.url || '')
  }

  if (isGrpcProtocol(protocol)) {
    return getGrpcTestUrl(protocol, props.site.display_url || props.site.url || '')
  }

  return props.site.display_url || props.site.url || ''
}

function addHeader() {
  formData.value.headers.push({ name: '', value: '' })
}

function removeHeader(index: number) {
  formData.value.headers.splice(index, 1)
}

function handleCancel() {
  visible.value = false
}

async function handleSave() {
  if (!props.site)
    return

  try {
    // Convert headers array to map for backend
    const config = { ...formData.value }
    const headersMap: { [key: string]: string } = {}
    config.headers.forEach(header => {
      if (header.name && header.value) {
        headersMap[header.name] = header.value
      }
    })

    // Create the config object for the backend
    const backendConfig = {
      url: props.site.url,
      health_check_enabled: config.enabled,
      check_interval: config.interval,
      timeout: config.timeout,
      user_agent: config.userAgent,
      max_redirects: config.maxRedirects,
      follow_redirects: config.followRedirects,
      check_favicon: config.checkFavicon,

      // Enhanced health check config
      health_check_config: {
        protocol: config.protocol,
        method: config.method,
        path: config.path,
        headers: headersMap,
        body: config.body,
        expected_status: config.expectedStatus,
        expected_text: config.expectedText,
        not_expected_text: config.notExpectedText,
        validate_ssl: config.validateSSL,
        grpc_service: config.grpcService,
        grpc_method: config.grpcMethod,
        dns_resolver: config.dnsResolver,
        source_ip: config.sourceIP,
        verify_hostname: config.verifyHostname,
        client_cert: config.clientCert,
        client_key: config.clientKey,
      },
    }

    await siteNavigationApi.updateHealthCheck(props.site.id, backendConfig)
    message.success($gettext('Health check configuration saved successfully'))

    // Trigger site refresh to update display URLs
    emit('refresh')

    visible.value = false
  }
  catch (error) {
    console.error('Failed to save health check config:', error)
    message.error($gettext('Failed to save health check configuration'))
  }
}

async function handleTest() {
  if (!props.site)
    return

  try {
    testing.value = true

    // Create a test configuration
    const testConfig = {
      protocol: formData.value.protocol,
      method: formData.value.method,
      path: formData.value.path,
      headers: formData.value.headers.reduce((acc, header) => {
        if (header.name && header.value) {
          acc[header.name] = header.value
        }
        return acc
      }, {} as { [key: string]: string }),
      body: formData.value.body,
      expected_status: formData.value.expectedStatus,
      expected_text: formData.value.expectedText,
      not_expected_text: formData.value.notExpectedText,
      validate_ssl: formData.value.validateSSL,
      grpc_service: formData.value.grpcService,
      grpc_method: formData.value.grpcMethod,
      timeout: formData.value.timeout,
    }

    // Call test API endpoint (we'll need to create this)
    const result = await siteNavigationApi.testHealthCheck(props.site.id, testConfig)

    if (result.success) {
      message.success($gettext('Test successful! Response time: %{response_time}ms', { response_time: String(result.response_time || 0) }))
    }
    else {
      message.error($gettext('Test failed: %{error}', { error: result.error || 'Unknown error' }, true))
    }
  }
  catch (error) {
    console.error('Health check test failed:', error)
    message.error($gettext('Test failed: Unable to perform health check'))
  }
  finally {
    testing.value = false
  }
}
</script>

<template>
  <AModal
    v-model:open="visible" :title="`${$gettext('Health Check Configuration')} - ${site?.name || getTestUrl()}`"
    width="800px" @cancel="handleCancel"
  >
    <div>
      <AForm :model="formData" layout="vertical" :label-col="{ span: 24 }" :wrapper-col="{ span: 24 }">
        <div>
          <!-- Enable/Disable Health Check -->
          <AFormItem :label="$gettext('Enable Health Check')">
            <div class="flex items-center gap-2">
              <ASwitch v-model:checked="formData.enabled" />
              <span class="text-sm text-gray-500 dark:text-gray-400">
                {{ formData.enabled ? $gettext('Health check is enabled') : $gettext('Health check is disabled') }}
              </span>
            </div>
          </AFormItem>

          <ADivider />

          <!-- Protocol Selection -->
          <AFormItem :label="$gettext('Protocol')">
            <ARadioGroup v-model:value="formData.protocol">
              <ARadio value="http">
                HTTP
              </ARadio>
              <ARadio value="https">
                HTTPS
              </ARadio>
              <ARadio value="grpc">
                gRPC
              </ARadio>
              <ARadio value="grpcs">
                gRPCS
              </ARadio>
            </ARadioGroup>
          </AFormItem>

          <!-- HTTP/HTTPS Settings -->
          <div v-if="!['grpc', 'grpcs'].includes(formData.protocol)">
            <ARow :gutter="16">
              <ACol :span="12">
                <AFormItem :label="$gettext('HTTP Method')">
                  <ASelect v-model:value="formData.method" style="width: 100%">
                    <ASelectOption value="GET">
                      GET
                    </ASelectOption>
                    <ASelectOption value="POST">
                      POST
                    </ASelectOption>
                    <ASelectOption value="PUT">
                      PUT
                    </ASelectOption>
                    <ASelectOption value="HEAD">
                      HEAD
                    </ASelectOption>
                    <ASelectOption value="OPTIONS">
                      OPTIONS
                    </ASelectOption>
                  </ASelect>
                </AFormItem>
              </ACol>
              <ACol :span="12">
                <AFormItem :label="$gettext('Path')">
                  <AInput v-model:value="formData.path" placeholder="/" />
                </AFormItem>
              </ACol>
            </ARow>

            <AFormItem :label="$gettext('Custom Headers')" class="mb-4">
              <div class="space-y-2">
                <div v-for="(header, index) in formData.headers" :key="index" class="flex gap-2">
                  <AInput v-model:value="header.name" placeholder="Header Name" class="flex-1" />
                  <AInput v-model:value="header.value" placeholder="Header Value" class="flex-1" />
                  <AButton type="text" danger @click="removeHeader(index)">
                    <template #icon>
                      <CloseOutlined />
                    </template>
                  </AButton>
                </div>
                <AButton type="dashed" class="w-full" @click="addHeader">
                  <template #icon>
                    <PlusOutlined />
                  </template>
                  {{ $gettext('Add Header') }}
                </AButton>
              </div>
            </AFormItem>

            <AFormItem v-if="formData.method !== 'GET'" :label="$gettext('Request Body')">
              <ATextarea v-model:value="formData.body" :rows="3" />
            </AFormItem>

            <AFormItem :label="$gettext('Expected Status Codes')">
              <ASelect
                v-model:value="formData.expectedStatus" mode="multiple" style="width: 100%"
                placeholder="200, 201, 204..."
              >
                <ASelectOption :value="200">
                  200 OK
                </ASelectOption>
                <ASelectOption :value="201">
                  201 Created
                </ASelectOption>
                <ASelectOption :value="204">
                  204 No Content
                </ASelectOption>
                <ASelectOption :value="301">
                  301 Moved Permanently
                </ASelectOption>
                <ASelectOption :value="302">
                  302 Found
                </ASelectOption>
                <ASelectOption :value="304">
                  304 Not Modified
                </ASelectOption>
                <ASelectOption :value="401">
                  401 Unauthorized
                </ASelectOption>
              </ASelect>
            </AFormItem>

            <ARow :gutter="16">
              <ACol :span="12">
                <AFormItem :label="$gettext('Expected Text')">
                  <AInput v-model:value="formData.expectedText" placeholder="Success" />
                </AFormItem>
              </ACol>
              <ACol :span="12">
                <AFormItem :label="$gettext('Not Expected Text')">
                  <AInput v-model:value="formData.notExpectedText" placeholder="Error" />
                </AFormItem>
              </ACol>
            </ARow>
          </div>

          <!-- gRPC/gRPCS Settings -->
          <div v-if="['grpc', 'grpcs'].includes(formData.protocol)">
            <AAlert
              v-if="['grpc', 'grpcs'].includes(formData.protocol)"
              :message="formData.protocol === 'grpcs'
                ? $gettext('gRPCS uses TLS encryption. Server must implement gRPC Health Check service. For testing, SSL validation is disabled by default.')
                : $gettext('gRPC health check requires server to implement gRPC Health Check service (grpc.health.v1.Health).')" type="info" show-icon class="mb-4"
            />
            <AAlert
              :message="$gettext('Note: If the server does not support gRPC Reflection, health checks may fail. Please ensure your gRPC server has Reflection enabled.')"
              type="warning" show-icon class="mb-4"
            />
            <ARow :gutter="16">
              <ACol :span="12">
                <AFormItem :label="$gettext('Service Name')">
                  <AInput v-model:value="formData.grpcService" placeholder="my.service.v1.MyService" />
                </AFormItem>
              </ACol>
              <ACol :span="12">
                <AFormItem :label="$gettext('Method Name')">
                  <AInput v-model:value="formData.grpcMethod" placeholder="Check" />
                </AFormItem>
              </ACol>
            </ARow>
          </div>

          <!-- Advanced Settings -->
          <ACollapse>
            <ACollapsePanel key="advanced" :header="$gettext('Advanced Settings')">
              <ARow :gutter="16">
                <ACol :span="12">
                  <AFormItem :label="$gettext('Check Interval (seconds)')">
                    <AInputNumber v-model:value="formData.interval" :min="30" :max="3600" style="width: 100%" />
                  </AFormItem>
                </ACol>
                <ACol :span="12">
                  <AFormItem :label="$gettext('Timeout (seconds)')">
                    <AInputNumber v-model:value="formData.timeout" :min="5" :max="60" style="width: 100%" />
                  </AFormItem>
                </ACol>
              </ARow>

              <AFormItem :label="$gettext('User Agent')">
                <AInput v-model:value="formData.userAgent" />
              </AFormItem>

              <div v-if="!['grpc', 'grpcs'].includes(formData.protocol)">
                <ARow :gutter="16">
                  <ACol :span="12">
                    <AFormItem :label="$gettext('Max Redirects')">
                      <AInputNumber v-model:value="formData.maxRedirects" :min="0" :max="10" style="width: 100%" />
                    </AFormItem>
                  </ACol>
                  <ACol :span="12">
                    <AFormItem>
                      <ACheckbox v-model:checked="formData.followRedirects">
                        {{ $gettext('Follow Redirects') }}
                      </ACheckbox>
                    </AFormItem>
                  </ACol>
                </ARow>

                <AFormItem>
                  <ACheckbox v-model:checked="formData.validateSSL">
                    {{ $gettext('Validate SSL Certificate') }}
                  </ACheckbox>
                </AFormItem>

                <AFormItem>
                  <ACheckbox v-model:checked="formData.verifyHostname">
                    {{ $gettext('Verify Hostname') }}
                  </ACheckbox>
                </AFormItem>

                <AFormItem>
                  <ACheckbox v-model:checked="formData.checkFavicon">
                    {{ $gettext('Check Favicon') }}
                  </ACheckbox>
                </AFormItem>
              </div>

              <!-- DNS & Network -->
              <ARow :gutter="16">
                <ACol :span="12">
                  <AFormItem :label="$gettext('DNS Resolver')">
                    <AInput v-model:value="formData.dnsResolver" placeholder="8.8.8.8:53" />
                  </AFormItem>
                </ACol>
                <ACol :span="12">
                  <AFormItem :label="$gettext('Source IP')">
                    <AInput v-model:value="formData.sourceIP" placeholder="192.168.1.100" />
                  </AFormItem>
                </ACol>
              </ARow>

              <!-- Client Certificates -->
              <ARow :gutter="16">
                <ACol :span="12">
                  <AFormItem :label="$gettext('Client Certificate')">
                    <AInput v-model:value="formData.clientCert" placeholder="/path/to/client.crt" />
                  </AFormItem>
                </ACol>
                <ACol :span="12">
                  <AFormItem :label="$gettext('Client Key')">
                    <AInput v-model:value="formData.clientKey" placeholder="/path/to/client.key" />
                  </AFormItem>
                </ACol>
              </ARow>
            </ACollapsePanel>
          </ACollapse>
        </div>
      </AForm>
    </div>

    <template #footer>
      <AButton @click="handleCancel">
        {{ $gettext('Cancel') }}
      </AButton>
      <AButton type="primary" @click="handleSave">
        {{ $gettext('Save') }}
      </AButton>
      <AButton :loading="testing" @click="handleTest">
        {{ $gettext('Test') }}
      </AButton>
    </template>
  </AModal>
</template>

<style scoped>
  .grpc-help-content {
    font-size: 14px;
    line-height: 1.6;
  }

  .grpc-help-content h4 {
    color: #1890ff;
    margin: 16px 0 8px 0;
    font-size: 16px;
    font-weight: 600;
  }

  .grpc-help-content h5 {
    color: #595959;
    margin: 12px 0 4px 0;
    font-size: 14px;
    font-weight: 500;
  }

  .grpc-help-content p {
    margin: 8px 0;
    color: #595959;
  }

  .code-examples {
    margin: 16px 0;
  }

  .code-examples pre {
    background-color: #f6f8fa;
    border: 1px solid #e1e4e8;
    border-radius: 6px;
    padding: 12px;
    margin: 8px 0;
    overflow-x: auto;
    font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
    font-size: 13px;
    line-height: 1.4;
  }

  .code-examples code {
    color: #24292e;
    background: transparent;
    border: none;
    padding: 0;
  }

  .dark .code-examples pre {
    background-color: #161b22;
    border-color: #30363d;
  }

  .dark .code-examples code {
    color: #e6edf3;
  }

  .dark .grpc-help-content h4 {
    color: #58a6ff;
  }

  .dark .grpc-help-content h5,
  .dark .grpc-help-content p {
    color: #c9d1d9;
  }
</style>
