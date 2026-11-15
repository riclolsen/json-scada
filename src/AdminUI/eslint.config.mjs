import js from '@eslint/js'
import pluginVue from 'eslint-plugin-vue'
import globals from 'globals'

export default [
  { ignores: ['dist/**', 'node_modules/**', 'public/lib/**'] },
  js.configs.recommended,
  ...pluginVue.configs['flat/essential'],
  {
    languageOptions: {
      ecmaVersion: 'latest',
      sourceType: 'module',
      globals: { ...globals.browser, ...globals.node },
    },
    // keep the ESLint 8 defaults this project was linted with
    linterOptions: { reportUnusedDisableDirectives: 'off' },
    rules: { 'no-unused-vars': ['error', { caughtErrors: 'none' }] },
  },
]
