// eslint-disable-next-line @typescript-eslint/no-var-requires
const i18n = require('./i18n.json')

module.exports = {
  output: {
    locales: Object.keys(i18n),
  },
}
