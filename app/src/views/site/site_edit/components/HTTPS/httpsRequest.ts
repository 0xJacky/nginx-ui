import type { AutoCertOptions } from '@/api/auto_cert'
import type { HTTPSChallengeMethod, HTTPSCheckRequest, HTTPSRequest } from '@/api/https'
import { PrivateKeyTypeEnum } from '@/constants'

// Pure request builders for the HTTPS card, kept free of Vue so they can be
// unit-tested in isolation.

/** The methods offered by the HTTPS card. */
export type HTTPSCardMethod = HTTPSChallengeMethod | 'existing' | 'skip'

export interface HTTPSRequestForm {
  method: HTTPSCardMethod
  domains: readonly string[]
  redirectHTTPToHTTPS: boolean
  options: AutoCertOptions
  // The certificate record picked for the `existing` method.
  certificateId?: number
}

export function isChallengeMethod(method: HTTPSCardMethod): method is HTTPSChallengeMethod {
  return method === 'http01' || method === 'dns01'
}

export function buildHTTPSRequest(form: HTTPSRequestForm): HTTPSRequest {
  const o = form.options
  const challengeMethod: HTTPSChallengeMethod = form.method === 'dns01' ? 'dns01' : 'http01'
  const request: HTTPSRequest = {
    domains: [...form.domains],
    challenge_method: challengeMethod,
    dns_credential_id: challengeMethod === 'dns01' ? (o.dns_credential_id ?? 0) : 0,
    redirect_http_to_https: form.redirectHTTPToHTTPS,
    key_type: o.key_type || PrivateKeyTypeEnum.P256,
    acme_user_id: o.acme_user_id ?? 0,
    profile: o.profile ?? '',
    must_staple: !!o.must_staple,
    lego_disable_cname_support: !!o.lego_disable_cname_support,
    disable_authoritative_ns_propagation: challengeMethod === 'dns01' && !!o.disable_authoritative_ns_propagation,
    enable_common_name: !!o.enable_common_name,
    revoke_old: !!o.revoke_old,
  }

  // The backend ignores the challenge for an existing certificate; the id is
  // only sent for that method so an issuing run can never pick one up.
  if (form.method === 'existing' && form.certificateId)
    request.certificate_id = form.certificateId

  return request
}

export function buildHTTPSCheckRequest(request: HTTPSRequest): HTTPSCheckRequest {
  const check: HTTPSCheckRequest = {
    domains: [...request.domains],
    challenge_method: request.challenge_method,
  }
  if (request.certificate_id)
    check.certificate_id = request.certificate_id

  return check
}
