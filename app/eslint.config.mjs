import createConfig from '@antfu/eslint-config'
import sonarjs from 'eslint-plugin-sonarjs'
import autoImport from './.eslint-auto-import.mjs'

export default createConfig(
  {
    stylistic: true,
    ignores: ['**/version.json', 'tsconfig.json', 'tsconfig.node.json', '.eslint-auto-import.mjs', '**/china.json', '**/world.json'],
    languageOptions: {
      globals: {
        ...autoImport.globals,
        ApexCharts: 'readonly',
        NodeJS: 'readonly',
        JSX: 'readonly',
      },
    },
  },
  sonarjs.configs.recommended,
  {
    name: '@nginx-ui/eslint-config',
    rules: {
      'no-console': 'warn',
      'no-alert': 'warn',
      'ts/no-explicit-any': 'warn',
      'vue/no-unused-refs': 'warn',
      'vue/prop-name-casing': 'warn',
      'node/prefer-global/process': 'off',
      'unused-imports/no-unused-vars': 'warn',

      // Prevent redundant manual imports for auto-imported globals
      'import/no-duplicates': 'error',
      'no-undef': 'error',

      // https://eslint.org/docs/latest/rules/dot-notation
      'style/dot-notation': 'off',

      // https://eslint.org/docs/latest/rules/arrow-parens
      'style/arrow-parens': ['error', 'as-needed'],

      // https://eslint.org/docs/latest/rules/prefer-template
      'prefer-template': 'error',

      // https://eslint.style/rules/js/arrow-spacing
      'style/arrow-spacing': ['error', { before: true, after: true }],

      // https://github.com/un-ts/eslint-plugin-import-x/blob/master/docs/rules/prefer-default-export.md
      'import/prefer-default-export': 'off',

      // https://eslint.vuejs.org/rules/require-typed-ref
      'vue/require-typed-ref': 'warn',

      // https://eslint.vuejs.org/rules/require-prop-types
      'vue/require-prop-types': 'warn',

      // https://eslint.vuejs.org/rules/no-ref-as-operand.html
      'vue/no-ref-as-operand': 'error',

      // https://eslint.vuejs.org/rules/component-name-in-template-casing
      'vue/component-name-in-template-casing': ['error', 'PascalCase', {
        registeredComponentsOnly: false,
      }],

      // -- Sonarlint
      'sonarjs/no-duplicate-string': 'off',
      'sonarjs/no-nested-template-literals': 'off',
      'sonarjs/pseudo-random': 'warn',
      'sonarjs/no-nested-functions': 'off',

      'eslint-comments/no-unlimited-disable': 'off',
    },
  },
)
