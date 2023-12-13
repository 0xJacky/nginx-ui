// eslint-disable-next-line @typescript-eslint/no-var-requires
const i18n = require('./i18n.json')

module.export = {
  input: {
    include: ['**/*.js', '**/*.ts', '**/*.vue', '**/*.jsx', '**/*.tsx'],
  },
  output: {
    locales: Object.keys(i18n),
  },
}
