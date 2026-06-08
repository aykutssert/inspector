import Head from 'next/head'; // trigger nextjs-no-head-import

export default function Layout({ children }) {
  return (
    <html>
      <Head>
        <title>My Page</title>
      </Head>
      <body>{children}</body>
    </html>
  );
}
