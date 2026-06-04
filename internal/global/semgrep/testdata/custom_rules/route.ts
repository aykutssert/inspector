// @ts-nocheck

export async function FETCH(request: Request) {
  return Response.json({ ok: true });
}

export function LOAD() {
  return Response.json({ ok: true });
}

export const PURGE = async () => {
  return Response.json({ ok: true });
};

export async function GET(request: Request) {
  return Response.json({ ok: true });
}

export const POST = async (request: Request) => {
  return Response.json({ ok: true });
};

export const runtime = "edge";

const topLevelCookies = cookies();
const topLevelHeaders = headers();

export async function safeRequestApiUsage() {
  const requestCookies = cookies();
  const requestHeaders = headers();
  return Response.json({ requestCookies, requestHeaders });
}
