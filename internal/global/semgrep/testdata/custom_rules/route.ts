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
  
  // triggers nextjs-no-side-effect-in-get-handler
  await prisma.user.create({ data: { name: "Alice" } });
  
  // triggers nextjs-no-side-effect-in-get-handler
  await Todo.save();

  // triggers nextjs-no-side-effect-in-get-handler
  await db.insert(users).values({ name: "Bob" });

  return Response.json({ ok: true });
}


export const POST = async (request: Request) => {
  // Should NOT trigger nextjs-no-side-effect-in-get-handler (since it's a POST)
  await prisma.user.create({ data: { name: "Alice" } });
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
