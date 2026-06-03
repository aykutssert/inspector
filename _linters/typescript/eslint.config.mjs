import tseslint from 'typescript-eslint';

// Type-aware lint config consumed by inspector's ts-eslint analyzer. Only the
// sharp, low-noise type-checked rules are enabled; the no-unsafe-* family is
// left off because it floods any-heavy code. projectService auto-discovers the
// repo's tsconfig (run with cwd at the scan root).
export default tseslint.config({
  files: ['**/*.{ts,tsx,mts,cts}'],
  languageOptions: {
    parser: tseslint.parser,
    parserOptions: { projectService: true },
  },
  plugins: { '@typescript-eslint': tseslint.plugin },
  rules: {
    '@typescript-eslint/no-floating-promises': 'error',
    '@typescript-eslint/no-misused-promises': 'error',
    '@typescript-eslint/await-thenable': 'error',
    '@typescript-eslint/no-for-in-array': 'error',
    '@typescript-eslint/require-await': 'warn',
  },
});
