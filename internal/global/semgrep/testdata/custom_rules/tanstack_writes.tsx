// @ts-nocheck
"use client";

import { QueryClient, useQuery, useSuspenseQuery } from "@tanstack/react-query";

const sharedClient = new QueryClient();
const queryKeys = {
  createUser: () => ["users", "create"] as const,
  deleteUser: () => ["users", "delete"] as const,
  list: () => ["users"] as const,
  voidQuery: () => ["users", "void"] as const,
};

export function UnsafeQueryUsage() {
  const unstableClient = new QueryClient();
  const writeQuery = useQuery({
    queryKey: queryKeys.createUser(),
    queryFn: () => fetch("/api/users", { method: "POST" }),
    staleTime: Infinity,
  });
  const axiosWrite = useSuspenseQuery({
    queryKey: queryKeys.deleteUser(),
    queryFn: async () => {
      return api.delete("/api/users/1");
    },
    staleTime: Infinity,
  });
  const voidQuery = useQuery({
    queryKey: queryKeys.voidQuery(),
    queryFn: async () => {
      await fetch("/api/users");
    },
    staleTime: Infinity,
  });
  return <div>{writeQuery.data}{axiosWrite.data}{voidQuery.data}{Boolean(unstableClient)}</div>;
}

export function SafeQueryUsage() {
  const readQuery = useQuery({
    queryKey: queryKeys.list(),
    queryFn: async () => {
      return fetch("/api/users").then((response) => response.json());
    },
    staleTime: 60_000,
  });
  return <div>{readQuery.data}{Boolean(sharedClient)}</div>;
}
