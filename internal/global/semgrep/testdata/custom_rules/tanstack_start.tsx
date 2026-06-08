// @ts-nocheck
import { createFileRoute, useLoaderData } from "@tanstack/react-router";
import { createServerFn } from "@tanstack/start";
import { useEffect, useState } from "react";

// ─── tanstack-start-no-direct-fetch-in-loader ─────────────────────────────────

// Violation: raw fetch inside loader
export const Route = createFileRoute("/products")({
  loader: async ({ context }) => {
    const data = await fetch("https://api.example.com/products");
    return data.json();
  },
});

// Safe: using createServerFn
const getProducts = createServerFn().handler(async () => {
  const data = await fetch("https://api.example.com/products");
  return data.json();
});

export const SafeRoute = createFileRoute("/products-safe")({
  loader: async () => getProducts(),
});

// ─── tanstack-start-no-secrets-in-loader ─────────────────────────────────────

// Violation: secret env var inside loader
export const SecretRoute = createFileRoute("/admin")({
  loader: async () => {
    const key = process.env.API_KEY;
    return { key };
  },
});

// Violation 2: database URL in loader
export const DbRoute = createFileRoute("/data")({
  loader: async () => {
    const url = process.env.DATABASE_URL;
    return fetchData(url);
  },
});

// Safe: public env var in loader
export const PublicRoute = createFileRoute("/public")({
  loader: async () => {
    const url = process.env.NEXT_PUBLIC_API_URL;
    return fetch(url).then((r) => r.json());
  },
});

// ─── tanstack-start-no-useeffect-fetch ───────────────────────────────────────

// Violation: useEffect + fetch for initial data
function ProductList() {
  const [products, setProducts] = useState([]);

  useEffect(() => {
    fetch("/api/products")
      .then((r) => r.json())
      .then(setProducts);
  }, []);

  return <ul>{products.map((p) => <li key={p.id}>{p.name}</li>)}</ul>;
}

// Violation 2: useEffect + await fetch
function UserDashboard() {
  const [user, setUser] = useState(null);

  useEffect(async () => {
    const res = await fetch("/api/me");
    const data = await res.json();
    setUser(data);
  }, []);

  return <div>{user?.name}</div>;
}

// Safe: using loader data
function SafeProductList() {
  const products = useLoaderData({ from: "/products-safe" });
  return <ul>{products.map((p) => <li key={p.id}>{p.name}</li>)}</ul>;
}

// ─── tanstack-start-no-use-server-in-handler ──────────────────────────────────

// Violation: redundant "use server" inside createServerFn handler
const badServerFn = createServerFn().handler(async () => {
  "use server";
  return { ok: true };
});

// Violation: with validator
const badValidatedFn = createServerFn()
  .validator((data: unknown) => data)
  .handler(async () => {
    "use server";
    return { processed: true };
  });

// Safe: no "use server" in handler (boundary is implicit)
const goodServerFn = createServerFn().handler(async () => {
  return { ok: true };
});

// ─── tanstack-start-server-fn-method-order ────────────────────────────────────

// Violation: handler before validator — validator won't run
const wrongOrderFn = createServerFn().handler(async () => {
  return "data";
}).validator((input: unknown) => input);

// Safe: correct order — validator first, then handler
const correctOrderFn = createServerFn()
  .validator((input: unknown) => input)
  .handler(async () => "data");

// ─── no-document-start-view-transition ───────────────────────────────────────

// Violation: direct DOM API bypasses React reconciler
function NavigateWithTransition() {
  function handleClick() {
    document.startViewTransition(() => {
      navigate("/next-page");
    });
  }
  return <button onClick={handleClick}>Go</button>;
}

// Safe: React 19 approach via startTransition
// import { startTransition } from "react";
// startTransition(() => navigate("/next-page"));
