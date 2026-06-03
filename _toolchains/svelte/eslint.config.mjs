import svelte from 'eslint-plugin-svelte';
import svelteParser from 'svelte-eslint-parser';

// Flat config consumed by inspector's svelte-lint analyzer. Targets only
// .svelte files; the recommended ruleset catches real reactivity/binding bugs.
export default [
  ...svelte.configs.recommended,
  {
    files: ['**/*.svelte'],
    languageOptions: {
      parser: svelteParser,
      parserOptions: { ecmaVersion: 'latest', sourceType: 'module' },
    },
    rules: {
      // no-navigation-without-resolve is a SvelteKit base-path convention, not a
      // bug class. It fired 29x (all error) on a real app, drowning the actual
      // security finding (no-at-html-tags, XSS) and inflating exit codes. Keep it
      // visible as a warning but stop it from dominating. The real
      // security/correctness rules (no-at-html-tags, require-each-key) stay error.
      'svelte/no-navigation-without-resolve': 'warn',
    },
  },
];
