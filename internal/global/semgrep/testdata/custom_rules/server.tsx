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
function SafePage2({ items, meta }: { items: Item[]; meta: object }) {
  return <ClientWidget items={items} metaJson={JSON.stringify(meta)} />;
}
