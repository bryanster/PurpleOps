import js from '@eslint/js'
import prettier from 'eslint-config-prettier'
import playwright from 'eslint-plugin-playwright'
import globals from 'globals'
import tseslint from 'typescript-eslint'

export default tseslint.config(
  { ignores: ['node_modules', 'test-results', 'playwright-report'] },

  // This file. Not type-checked: it is the only JavaScript here, and adding
  // `allowJs` to tsconfig.json to include it would pull JavaScript checking
  // into a project that has none.
  { files: ['eslint.config.mjs'], extends: [js.configs.recommended] },

  {
    files: ['**/*.ts'],
    extends: [
      js.configs.recommended,
      // Type-aware, like web/'s. `no-floating-promises` is the reason: an
      // `expect(...)` or a `page.click()` nobody awaited passes whatever the
      // application did, which is the single most common way a Playwright suite
      // stops testing anything.
      ...tseslint.configs.strictTypeChecked,
      ...tseslint.configs.stylisticTypeChecked,
    ],
    languageOptions: {
      ecmaVersion: 2023,
      globals: globals.node,
      parserOptions: { projectService: true, tsconfigRootDir: import.meta.dirname },
    },
    rules: {
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/no-floating-promises': 'error',
      '@typescript-eslint/consistent-type-imports': [
        'error',
        { prefer: 'type-imports', fixStyle: 'inline-type-imports' },
      ],
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
      // Playwright's own fixture syntax: `async ({}, use) => {}` is how a
      // fixture that depends on nothing is declared.
      'no-empty-pattern': 'off',
    },
  },

  {
    files: ['specs/**/*.ts'],
    extends: [playwright.configs['flat/recommended']],
    rules: {
      // The two the ticket asks a reviewer to check, promoted from warnings so
      // the build says it instead of a person: an arbitrary sleep asserts
      // nothing and fails on a slow machine, and a conditional skip is how v1
      // ended up with a suite that reported success without running.
      'playwright/no-wait-for-timeout': 'error',
      'playwright/no-skipped-test': ['error', { allowConditional: true }],
    },
  },

  // Must stay last: it turns off every rule Prettier already decides.
  prettier,
)
