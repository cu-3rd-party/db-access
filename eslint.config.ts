import js from '@eslint/js';
import globals from 'globals';
import playwright from 'eslint-plugin-playwright';
import tseslint from 'typescript-eslint';

export default tseslint.config(
  {ignores: ['node_modules/', 'public/', 'tests/e2e/reports/', 'tests/e2e/test-artifacts/']},
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    languageOptions: {globals: {...globals.browser, ...globals.node}},
    rules: {
      'object-curly-spacing': 'off',
      'no-restricted-syntax': ['error', {selector: 'CallExpression[callee.property.name="innerHTML"]', message: 'avoid innerHTML'}],
      'prefer-const': 'error',
      'eqeqeq': 'error',
    },
  },
  {
    files: ['web_src/**/*.test.ts'],
    languageOptions: {globals: globals.vitest},
  },
  {
    files: ['tests/e2e/**/*.ts'],
    ...playwright.configs['flat/recommended'],
  },
);
