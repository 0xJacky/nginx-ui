export const CA_SERVER_OPTIONS = [
  {
    label: 'Let\'s Encrypt Production',
    value: 'https://acme-v02.api.letsencrypt.org/directory',
  },
  {
    label: 'Let\'s Encrypt Staging',
    value: 'https://acme-staging-v02.api.letsencrypt.org/directory',
  },
  {
    label: 'ZeroSSL',
    value: 'https://acme.zerossl.com/v2/DV90',
  },
  {
    label: 'Google Trust Services Production',
    value: 'https://dv.acme-v02.api.pki.goog/directory',
  },
  {
    label: 'Google Trust Services Test',
    value: 'https://dv.acme-v02.test-api.pki.goog/directory',
  },
  {
    label: 'Buypass Production',
    value: 'https://api.buypass.com/acme/directory',
  },
  {
    label: 'Buypass Test',
    value: 'https://api.test4.buypass.no/acme/directory',
  },
  {
    label: 'Pebble Local Test',
    value: 'https://localhost:14000/dir',
  },
]
