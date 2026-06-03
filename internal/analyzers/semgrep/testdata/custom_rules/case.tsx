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
