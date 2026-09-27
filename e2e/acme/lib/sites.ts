import { randomBytes } from 'node:crypto'

/** A domain no other test uses, e.g. `plain-3f9a1c.e2e.test`. */
export function uniqueDomain(label: string) {
  return `${label}-${randomBytes(3).toString('hex')}.e2e.test`
}

// nginx-ui's HTTP-01 listener (settings.CertSettings.HTTPChallengePort).
export const challengePort = 9180

/** The location the letsencrypt.conf block template adds. */
export function challengeLocation(upstream = `http://127.0.0.1:${challengePort}`) {
  return `    location ~ /.well-known/acme-challenge {
        proxy_set_header Host $host;
        proxy_set_header X-Real_IP $remote_addr;
        proxy_set_header X-Forwarded-For $remote_addr:$remote_port;
        proxy_pass ${upstream};
    }`
}

/** Body marker so a probe can tell which server block answered. */
export function okLocation(marker: string) {
  return `    location / {
        default_type text/plain;
        return 200 "${marker}\\n";
    }`
}

export function httpServer(domain: string, ...body: string[]) {
  return `server {
    listen 80;
    server_name ${domain};

${body.join('\n\n')}
}
`
}

export function tlsServer(domain: string, certificate: string, key: string, ...body: string[]) {
  return `server {
    listen 443 ssl;
    server_name ${domain};
    ssl_certificate ${certificate};
    ssl_certificate_key ${key};

${body.join('\n\n')}
}
`
}
