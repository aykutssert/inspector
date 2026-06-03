import tailwind from 'eslint-plugin-tailwindcss';
import tsParser from '@typescript-eslint/parser';

// Flat config consumed by inspector's tailwind-lint analyzer. Wraps the proven
// eslint-plugin-tailwindcss across the JS/TS family. The TypeScript parser is
// used (with JSX enabled) so .ts/.tsx parse too; no type information is needed
// for these rules, so projectService is intentionally off (fast, no tsconfig).
//
// Rule choice (see architecture.md rule-selection):
//   - no-contradicting-classname: ERROR. Conflicting utilities (e.g. `block
//     inline`, `p-2 p-4`) are a real visual bug — the outcome is ambiguous.
//     Works without a tailwind config.
//   - enforces-shorthand: WARN. `w-4 h-4` -> `size-4`, `mt-2 mb-2` -> `my-2`.
//     A consistency hint; the plugin reads the project's tailwind config and
//     stays inert when it can't resolve one, so it never emits wrong advice.
// no-custom-classname is deliberately left off: without a fully-resolved
// tailwind config it floods every utility class as "unknown".
export default [
  {
    files: ['**/*.{js,jsx,mjs,cjs,ts,tsx,mts,cts}'],
    languageOptions: {
      parser: tsParser,
      ecmaVersion: 'latest',
      sourceType: 'module',
      parserOptions: { ecmaFeatures: { jsx: true } },
    },
    plugins: { tailwindcss: tailwind },
    rules: {
      'tailwindcss/no-contradicting-classname': 'error',
      'tailwindcss/enforces-shorthand': 'warn',
    },
  },
];
