// Fixture: each construct below trips exactly one oxlint rule the inspector
// explicitly enables for React projects. The coverage test asserts every one of
// these rule ids still fires — a guard against an oxlint upgrade or a config
// drift silently dropping a rule we rely on.
import { useEffect, useState } from "react";

export function Widget({ items, cond }: { items: string[]; cond: boolean }) {
  // react-hooks/rules-of-hooks: a hook called inside a conditional.
  if (cond) {
    useState(0);
  }

  const [count, setCount] = useState(0);

  // react-hooks/exhaustive-deps: effect reads `count` but omits it from deps.
  useEffect(() => {
    console.log(count);
  }, []);

  return (
    <div>
      {/* react/no-array-index-key: array index used as the React key. */}
      {items.map((it, index) => (
        <span key={index}>{it}</span>
      ))}

      {/* react/button-has-type: a <button> with no explicit type submits forms. */}
      <button onClick={() => setCount(count + 1)}>inc</button>
    </div>
  );
}
