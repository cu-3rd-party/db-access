import type {Config} from 'stylelint';

export default {
  extends: ['stylelint-config-standard'],
  rules: {
    'at-rule-no-unknown': [true, {ignoreAtRules: ['tailwind', 'apply', 'layer']}],
    'at-rule-no-deprecated': null,
    'at-rule-prelude-no-invalid': null,
    'import-notation': 'string',
  },
} satisfies Config;
