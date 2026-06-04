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
  const salt = crypto.pbkdf2Sync('secret', 'salt', 100000, 64, 'sha512'); // triggers crypto-sync-in-request-path
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
