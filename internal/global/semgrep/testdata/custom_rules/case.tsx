// @ts-nocheck — semgrep golden fixture; React/JSX types are not installed in
// this repo, so type-checking is intentionally disabled. semgrep ignores this.
type Item = { id: string; label: string };

type Props = {
  html: string;
  token: string;
  items: Item[];
};

export function Example({ html, token, items }: Props) {
  localStorage.setItem("authToken", token);
  sessionStorage.setItem("theme", "dark");
  eval(html);
  console.log("debug", html);
  console.debug(token);
  try {
    JSON.parse(html);
  } catch (err) {
  }

  return (
    <>
      <a href="javascript:alert(1)">bad</a>
      <a href="/safe">safe</a>
      <div dangerouslySetInnerHTML={{ __html: html }} />
      <div dangerouslySetInnerHTML={{ __html: "<p>static</p>" }} />
      {items.map((item) => (
        <span key={Math.random()}>{item.label}</span>
      ))}
      {items.map((item) => (
        <span key={item.id}>{item.label}</span>
      ))}
    </>
  );
}

export async function loadAll(ids: string[]) {
  for (const id of ids) {
    await fetch(id);
  }
}

export function stub() {
  // TODO: implement this handler
  // ... rest of the implementation
}

export function renderLegacy(node: HTMLElement, html: string) {
  node.innerHTML = html;
  node.innerHTML = "<p>static</p>";
  node.innerHTML = DOMPurify.sanitize(html);
  node.textContent = html;
}

export function cloneSlow<T>(value: T) {
  return JSON.parse(JSON.stringify(value));
}

export function collectSlow(items: Item[]) {
  return items.reduce((acc, item) => [...acc, item.label], [] as string[]);
}

export function collectOk(items: Item[]) {
  return items.reduce((acc, item) => {
    acc.push(item.label);
    return acc;
  }, [] as string[]);
}

export function lodashSlow(values: string[]) {
  const _ = require("lodash");
  return _.chunk(values, 2);
}

export function lodashOk(values: string[]) {
  const chunk = require("lodash/chunk");
  return chunk(values, 2);
}

export function insecureRandoms() {
  const resetToken = Math.random();
  const randomSort = Math.random();
  const sessionId = crypto.randomUUID();
  return { resetToken, randomSort, sessionId };
}

export function setCookies(token: string) {
  document.cookie = `session=${token}; path=/`;
  document.cookie = "theme=dark; Secure; SameSite=Lax";
  cookieStore.set("session", token);
}

export function redirectUser(next: string) {
  location.href = next;
  location.href = "/dashboard";
  window.location.assign(`/users/${encodeURIComponent(next)}`);
}

export function receiveMessages(allowedOrigins: string[]) {
  window.addEventListener("message", (event) => {
    handleMessage(event.data);
  });
  window.addEventListener("message", (event) => {
    if (!allowedOrigins.includes(event.origin)) return;
    handleMessage(event.data);
  });
  addEventListener("message", function (event) {
    if (event.origin !== "https://example.com") return;
    handleMessage(event.data);
  });
}

export async function loadAllInParallel(ids: string[]) {
  await Promise.all(ids.map((id) => fetch(id)));
}

export function navigateWithRouter(router: { push(url: string): void; replace(url: string): void }, next: string) {
  router.push(next);
  router.replace("/dashboard");
  router.replace(next);
}

export function navigateWithRouterSingleton(Router: { push(url: string): void }, next: string) {
  Router.push(next);
}

export function readPublicEnv() {
  const apiSecret = process.env.NEXT_PUBLIC_API_SECRET;
  const authToken = process.env["NEXT_PUBLIC_AUTH_TOKEN"];
  const siteUrl = process.env.NEXT_PUBLIC_SITE_URL;
  const serverSecret = process.env.STRIPE_SECRET_KEY;
  return { apiSecret, authToken, siteUrl, serverSecret };
}

export function hardcodedSecrets() {
  const awsKey = "AKIAIOSFODNN7EXAMPLE";
  const githubToken = "ghp_abcdefghijklmnopqrstuvwxyz0123456789";
  const stripeKey = "sk_live_0123456789abcdefABCDEF";
  const slackToken = "xoxb-EXAMPLE-NOT-A-REAL-TOKEN-000";
  const googleKey = "AIzaSyA0123456789abcdefghijklmnopqrstuv";
  const pemHeader = "-----BEGIN PRIVATE KEY-----";
  const publishable = "pk_live_0123456789abcdef";
  const notAws = "akiaiosfodnn7example";
  const testKey = "sk_test_0123456789abcdef";
  return { awsKey, githubToken, stripeKey, slackToken, googleKey, pemHeader, publishable, notAws, testKey };
}
