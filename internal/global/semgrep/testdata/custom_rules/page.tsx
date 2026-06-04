// @ts-nocheck

export default function Page() {
  const href = window.location.href;
  const theme = localStorage.getItem("theme");
  const agent = navigator.userAgent;
  return <main>{href}{theme}{agent}</main>;
}
