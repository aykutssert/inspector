import tseslint from 'typescript-eslint';

// Type-aware lint config consumed by inspector's ts-eslint analyzer. Only the
// sharp, low-noise type-checked rules are enabled; the no-unsafe-* family is
// left off because it floods any-heavy code. projectService auto-discovers the
// repo's tsconfig (run with cwd at the scan root).
//
// require-await is intentionally NOT enabled: it fired ~59x on real repos for
// trivially-async wrappers, drowning the real bugs. The high-value async-misuse
// rules (no-floating-promises, no-misused-promises, await-thenable) stay.
export default tseslint.config({
  files: ['**/*.{ts,tsx,mts,cts}'],
  languageOptions: {
    parser: tseslint.parser,
    parserOptions: { projectService: true },
  },
  plugins: { '@typescript-eslint': tseslint.plugin },
  rules: {
    // Async misuse — real crash/silent-failure bugs.
    '@typescript-eslint/no-floating-promises': 'error',
    '@typescript-eslint/no-misused-promises': 'error',
    '@typescript-eslint/await-thenable': 'error',
    '@typescript-eslint/no-for-in-array': 'error',
    // Type-safety smells — sharp, low false-positive.
    // no-explicit-any is deliberately NOT enabled: it fired 103x on a real
    // generics-heavy TS lib (zustand), where `any` in type machinery is
    // idiomatic — it drowns the real bugs. The `!` non-null bypass is bounded.
    '@typescript-eslint/no-non-null-assertion': 'warn',
    // Type-checked, near-zero FP: a redundant `as` the compiler proves useless.
    '@typescript-eslint/no-unnecessary-type-assertion': 'error',
  },
});
