import js from '@eslint/js'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import prettier from 'eslint-config-prettier'
import globals from 'globals'
import tseslint from 'typescript-eslint'

export default tseslint.config(
  // schema.d.ts is generated from api/openapi.yaml on every `make generate`.
  // It is still type-checked by tsc; it is just not ours to hold to style
  // rules, and reformatting it would make the codegen-drift gate fail.
  { ignores: ['dist', 'coverage', 'node_modules', 'src/api/schema.d.ts'] },

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

  // M0B-009: one module knows what an API URL looks like.
  //
  // A hand-written URL is a route the spec cannot check and the generator
  // cannot rename — the drift v1 accumulated for months. Everything goes
  // through `api/client.ts`, whose paths come from the generated schema, so a
  // path that is not in api/openapi.yaml fails to compile.
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/api/client.ts'],
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          selector: 'Literal[value=/^\\/api(\\/|$)/]',
          message:
            'Do not write API URLs. Call the generated client (src/api/client.ts) or apiUrl(path).',
        },
        {
          selector: 'TemplateElement[value.raw=/\\/api\\//]',
          message:
            'Do not write API URLs. Call the generated client (src/api/client.ts) or apiUrl(path).',
        },
      ],
      'no-restricted-globals': [
        'error',
        {
          name: 'fetch',
          message:
            'Use the generated client (src/api/client.ts): it types the request and the response, and turns a problem document into an ApiError.',
        },
      ],
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
    files: ['**/*.test.{ts,tsx}', 'src/test/**/*.{ts,tsx}'],
    rules: {
      // Test doubles are allowed to be partial: a fake that satisfies the whole
      // of a DOM interface is not a clearer test.
      '@typescript-eslint/unbound-method': 'off',
      // Fast refresh has nothing to do with test files, and the render helpers
      // in src/test export a wrapper component beside the factory that builds
      // its client.
      'react-refresh/only-export-components': 'off',
    },
  },

  // Must stay last: it turns off every rule Prettier already decides.
  prettier,
)
