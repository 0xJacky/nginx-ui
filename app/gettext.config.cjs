const i18n = require('./i18n.json')

module.exports = {
  input: {
    include: ['**/*.js', '**/*.ts', '**/*.vue', '**/*.jsx', '**/*.tsx'],
  },
  output: {
    locales: Object.keys(i18n),
    // Source locations change with every edit and made the catalogs conflict
    // on merge. Search the code for a string instead.
    locations: false,
    // A fuzzy match pairs a new string with an unrelated old translation. It
    // is never shown, so it only adds noise to the catalogs.
    fuzzyMatching: false,
  },
}
