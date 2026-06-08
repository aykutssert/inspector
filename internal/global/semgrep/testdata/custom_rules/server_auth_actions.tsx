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

import { cache } from "react";

// Safe: cache defined at module scope
const getCachedUser = cache(async (id: string) => {
  return db.users.find(id);
});
const getCachedSettings = React.cache(async () => {
  return db.settings.find();
});

export function UserProfile({ id }: any) {
  // Violations: cache defined inside component render body (triggers server.server-cache-with-object-literal)
  const getTodo = cache(async (todoId: string) => {
    return db.todos.find(todoId);
  });
  const getTheme = React.cache(async () => {
    return db.theme.find();
  });

  return <div>Profile</div>;
}

export const useSettings = () => {
  // Violation: cache defined inside hook body (triggers server.server-cache-with-object-literal)
  const getCachedDetails = cache(async (key: string) => {
    return db.details.find(key);
  });
};

export class CacheContainer {
  loadData() {
    // Violation: cache defined inside class method (triggers server.server-cache-with-object-literal)
    const getCachedItems = cache(async () => {
      return db.items.find();
    });
  }
}

