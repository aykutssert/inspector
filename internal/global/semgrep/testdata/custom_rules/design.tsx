// @ts-nocheck

// ─── design-no-redundant-padding-axes ─────────────────────────────────────────

// Violation: px-4 py-4 → should be p-4
function RedundantPadding() {
  return <div className="px-4 py-4 bg-white">Padded</div>;
}

// Violation: px-8 py-8 with other classes
function RedundantPaddingMixed() {
  return <button className="flex items-center px-8 py-8 rounded">Button</button>;
}

// Safe: different values
function DifferentPadding() {
  return <div className="px-4 py-2">Asymmetric</div>;
}

// ─── design-no-redundant-size-axes ────────────────────────────────────────────

// Violation: w-8 h-8 → should be size-8 (Tailwind v3.4+)
function RedundantSize() {
  return <div className="w-8 h-8 bg-blue-500 rounded" />;
}

// Violation: w-12 h-12 with other classes
function IconBox() {
  return <span className="inline-flex w-12 h-12 items-center justify-center">Icon</span>;
}

// Safe: different values
function DifferentSize() {
  return <div className="w-8 h-4">Rect</div>;
}

// ─── design-no-three-period-ellipsis ──────────────────────────────────────────

// Violation: ASCII three periods in JSX text
function LoadingText() {
  return <p>Loading...</p>;
}

// Violation: three periods in button label
function MoreButton() {
  return <button>Read more...</button>;
}

// Safe: typographic ellipsis character
function SafeLoading() {
  return <p>Loading…</p>;
}
