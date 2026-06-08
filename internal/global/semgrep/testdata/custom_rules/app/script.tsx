import Script from 'next/script';

export default function Page() {
  return (
    <div>
      {/* trigger nextjs-inline-script-missing-id */}
      <Script>
        {`console.log('inline code missing id');`}
      </Script>

      {/* trigger nextjs-inline-script-missing-id */}
      <Script dangerouslySetInnerHTML={{ __html: "console.log('dangerous missing id')" }} />

      {/* safe: has id */}
      <Script id="safe-script">
        {`console.log('safe inline');`}
      </Script>

      {/* safe: has id and dangerouslySetInnerHTML */}
      <Script id="safe-dangerous" dangerouslySetInnerHTML={{ __html: "console.log('safe dangerous')" }} />

      {/* safe: has src */}
      <Script src="https://example.com/analytics.js" />
    </div>
  );
}
