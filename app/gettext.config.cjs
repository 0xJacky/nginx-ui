const i18n = require('./i18n.json')

module.exports = {
  input: {
    include: ['**/*.js', '**/*.ts', '**/*.vue', '**/*.jsx', '**/*.tsx'],
  },
  output: {
    locales: Object.keys(i18n),
  },
}
