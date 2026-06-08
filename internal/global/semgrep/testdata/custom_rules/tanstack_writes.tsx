// @ts-nocheck
"use client";

import { QueryClient, useQuery, useSuspenseQuery, useInfiniteQuery } from "@tanstack/react-query";
import { useEffect } from "react";

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

// ─── query-no-query-in-effect ──────────────────────────────────────────────────

// Violation: refetch() inside useEffect
export function ComponentWithEffectRefetch({ userId }: { userId: string }) {
  const query = useQuery({
    queryKey: ["user", userId],
    queryFn: () => fetch(`/api/users/${userId}`).then((r) => r.json()),
  });

  useEffect(() => {
    query.refetch();
  }, [userId]);

  return <div>{query.data?.name}</div>;
}

// Safe: refetch in event handler, not useEffect
export function ComponentWithHandlerRefetch() {
  const query = useQuery({ queryKey: queryKeys.list(), queryFn: () => fetch("/api/users") });

  function handleRefresh() {
    query.refetch();
  }

  return <button onClick={handleRefresh}>Refresh</button>;
}

// ─── query-no-rest-destructuring ──────────────────────────────────────────────

// Violation: rest spread captures all query state fields
export function ComponentWithRestDestructure() {
  const { data, ...queryRest } = useQuery({
    queryKey: queryKeys.list(),
    queryFn: () => fetch("/api/users").then((r) => r.json()),
  });
  return <pre>{JSON.stringify(queryRest)}</pre>;
}

// Safe: explicit destructuring
export function ComponentWithExplicitDestructure() {
  const { data, isLoading, error } = useQuery({
    queryKey: queryKeys.list(),
    queryFn: () => fetch("/api/users").then((r) => r.json()),
  });
  if (isLoading) return <span>Loading…</span>;
  if (error) return <span>Error</span>;
  return <div>{data}</div>;
}
