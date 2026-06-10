// @ts-nocheck
import fs from "fs";
import { readFile } from "fs/promises";

// ─── server-hoist-static-io ───────────────────────────────────────────────────

// Safe: module-level read — runs once at startup
const moduleConfig = fs.readFileSync("./config.json", "utf8");

// Violation 1: sync read inside async route handler
export async function GET(request: Request) {
  const config = fs.readFileSync("./config.json", "utf8");
  return Response.json({ config });
}

// Violation 2: async read inside arrow handler
export const POST = async (request: Request) => {
  const template = await readFile("./email-template.html", "utf8");
  const body = await request.json();
  return Response.json({ body });
};

// ─── server-dedup-props ───────────────────────────────────────────────────────

type Item = { id: number; name: string };
declare const ClientWidget: any;

// Violation: raw + JSON.stringify of the same value as two separate props
function ServerPage({ items }: { items: Item[] }) {
  return (
    <ClientWidget
      items={items}
      itemsJson={JSON.stringify(items)}
    />
  );
}

// Safe: single format only
function SafePage({ items }: { items: Item[] }) {
  return <ClientWidget items={items} />;
}

// Safe: different variables
export function SafePage2({ items, meta }: { items: Item[]; meta: object }) {
  return <ClientWidget items={items} metaJson={JSON.stringify(meta)} />;
}

// ─── server-after-nonblocking ─────────────────────────────────────────────────

// Violation: console.log inside request handler blocks response
export async function AFTER_LOG(request: Request) {
  console.log("Fetching users");
  const data = await db.query("SELECT * FROM users");
  return Response.json(data);
}

// Violation: analytics.track inside request handler
export async function AFTER_ANALYTICS(request: Request) {
  const body = await request.json();
  analytics.track("order.created", body);
  return Response.json({ ok: true });
}

// ─── server-fetch-without-revalidate ──────────────────────────────────────────

// Violation: fetch without cache config inside request handler
export async function FETCH_DATA(request: Request) {
  const data = await fetch("https://api.example.com/data");
  return Response.json(await data.json());
}

// Safe: fetch with cache config
export async function FETCH_WITH_REVALIDATE(request: Request) {
  const data = await fetch("https://api.example.com/data", { next: { revalidate: 60 } });
  return Response.json(await data.json());
}

// Safe: fetch with no-store
export async function FETCH_NOSTORE(request: Request) {
  const data = await fetch("https://api.example.com/data", { cache: "no-store" });
  return Response.json(await data.json());
}
