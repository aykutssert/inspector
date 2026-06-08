// @ts-nocheck
import Script from "next/script";

export function ScriptsComponent() {
  const url = "https://example.com/analytic.js";

  return (
    <div>
      {/* Violation: Standard script missing defer/async */}
      <script src="https://example.com/sdk.js"></script>

      {/* Violation: Standard script missing defer/async (expression src) */}
      <script src={url}></script>

      {/* Violation: Next.js Script with beforeInteractive strategy */}
      <Script src="https://example.com/sdk.js" strategy="beforeInteractive" />


      {/* Safe: defer attribute */}
      <script src="https://example.com/sdk.js" defer></script>

      {/* Safe: defer={true} */}
      <script src="https://example.com/sdk.js" defer={true}></script>

      {/* Safe: async attribute */}
      <script src="https://example.com/sdk.js" async></script>

      {/* Safe: async={true} */}
      <script src="https://example.com/sdk.js" async={true}></script>

      {/* Safe: Next.js Script with default strategy (afterInteractive) */}
      <Script src="https://example.com/sdk.js" />

      {/* Safe: Next.js Script with non-blocking strategy */}
      <Script src="https://example.com/sdk.js" strategy="lazyOnload" />
    </div>
  );
}
