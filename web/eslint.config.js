import js from '@eslint/js'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import prettier from 'eslint-config-prettier'
import globals from 'globals'
import tseslint from 'typescript-eslint'

export default tseslint.config(
  { ignores: ['dist', 'coverage', 'node_modules'] },

  // Not type-checked and not our code: the file runs in the browser before any
  // module loads, which is the whole reason it is plain JS in public/.
  {
    files: ['public/**/*.js'],
    extends: [js.configs.recommended],
    languageOptions: { globals: globals.browser },
  },

  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      // Type-aware rules, not just syntactic ones. They are the reason
      // `no-floating-promises` and `no-unnecessary-condition` below can exist,
      // and they are worth the slower lint on a codebase this size.
      ...tseslint.configs.strictTypeChecked,
      ...tseslint.configs.stylisticTypeChecked,
      // `configs.flat.*`, not `configs['recommended-latest']`: the latter is
      // still eslintrc-shaped and ESLint 10 refuses it.
      reactHooks.configs.flat['recommended-latest'],
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      ecmaVersion: 2023,
      globals: globals.browser,
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      // M0B-008 forbids `any` in committed code, and forbids silencing a rule
      // or a type error without saying why. These make both a build failure
      // rather than something a reviewer has to notice.
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/ban-ts-comment': [
        'error',
        { 'ts-expect-error': 'allow-with-description', 'ts-ignore': true, 'ts-nocheck': true },
      ],

      // A promise nobody awaits is a request whose failure nobody sees.
      '@typescript-eslint/no-floating-promises': 'error',

      // Type-only imports are erased rather than kept as side-effecting
      // requires, which `verbatimModuleSyntax` in tsconfig makes mandatory.
      '@typescript-eslint/consistent-type-imports': [
        'error',
        { prefer: 'type-imports', fixStyle: 'inline-type-imports' },
      ],

      // An unused argument named `_something` is documentation, not a mistake.
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],

      // A no-op arrow is a real answer — an unsubscribe with nothing to undo,
      // a callback a caller does not care about. An empty *named function* or
      // method is still a mistake, so the rule stays on for those.
      '@typescript-eslint/no-empty-function': ['error', { allow: ['arrowFunctions'] }],
    },
  },

  // The vendored shadcn/ui components are upstream's code, kept close to
  // upstream so their diffs stay readable (M0B-008). They are still
  // type-checked by tsc; they are just not held to our stylistic rules.
  {
    files: ['src/components/ui/**/*.tsx'],
    rules: {
      '@typescript-eslint/array-type': 'off',
      '@typescript-eslint/no-unnecessary-condition': 'off',
      '@typescript-eslint/no-unsafe-assignment': 'off',
      '@typescript-eslint/consistent-type-definitions': 'off',
      'react-refresh/only-export-components': 'off',
    },
  },

  {
    files: ['**/*.test.{ts,tsx}', 'src/test/**/*.ts'],
    rules: {
      // Test doubles are allowed to be partial: a fake that satisfies the whole
      // of a DOM interface is not a clearer test.
      '@typescript-eslint/unbound-method': 'off',
    },
  },

  // Must stay last: it turns off every rule Prettier already decides.
  prettier,
)
