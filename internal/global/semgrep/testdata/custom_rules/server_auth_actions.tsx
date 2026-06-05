// Violation 1: server action function declaration without auth check
export async function updateProfile(data: any) {
  "use server";
  db.users.update(data);
}

// Safe 1: server action function declaration calling auth()
export async function updateEmail(email: string) {
  "use server";
  const session = await auth();
  if (!session) throw new Error("Unauthorized");
  db.users.updateEmail(session.user.id, email);
}

// Safe 2: server action function declaration calling checkAuth()
export async function updatePassword(password: string) {
  "use server";
  await checkAuth();
  db.users.updatePassword(password);
}
