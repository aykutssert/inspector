declare const prisma: any;
declare const axios: any;

// --- positive: network I/O inside a transaction ---
async function chargeAndSave(userId: string) {
  await prisma.$transaction(async (tx: any) => {
    const user = await tx.user.findUnique({ where: { id: userId } });
    const res = await fetch("https://payments.example/charge"); // FIRE
    await tx.payment.create({ data: { userId, ok: res.ok } });
  });
}

async function withAxios(userId: string) {
  await prisma.$transaction(async (tx: any) => {
    await axios.post("https://hook.example", { userId }); // FIRE
    await tx.user.update({ where: { id: userId }, data: {} });
  });
}

// --- negative: network I/O outside the transaction ---
async function chargeFirst(userId: string) {
  const res = await fetch("https://payments.example/charge"); // NO FIRE (outside)
  await prisma.$transaction(async (tx: any) => {
    await tx.payment.create({ data: { userId, ok: res.ok } });
  });
}
