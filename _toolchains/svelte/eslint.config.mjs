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
  },
];
