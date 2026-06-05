// @ts-nocheck
"use client";

import React, { useEffect, useState } from "react";
import axios from "axios";
import { useQuery } from "@tanstack/react-query";

export function ClientDataFetchComponent() {
  const [data, setData] = useState(null);

  // Violation: fetch inside useEffect
  useEffect(() => {
    fetch("/api/users")
      .then((res) => res.json())
      .setData(setData);
  }, []);

  // Violation: axios inside useEffect
  useEffect(() => {
    axios.get("/api/posts").then((res) => setData(res.data));
  }, []);

  // Violation: React.useEffect with axios call directly
  React.useEffect(() => {
    axios("/api/comments").then((res) => setData(res.data));
  }, []);

  // Safe Case: fetch inside event handler
  const handleRefresh = () => {
    fetch("/api/refresh");
  };

  // Safe Case: data fetching using TanStack Query (no direct fetch inside useEffect)
  const { data: queryData } = useQuery({
    queryKey: ["projects"],
    queryFn: () => fetch("/api/projects").then((res) => res.json()),
  });

  return (
    <div>
      <button onClick={handleRefresh}>Refresh</button>
    </div>
  );
}

// Safe Case: No "use client" directive (RSC calling fetch, though here we just mock a server function)
export async function ServerComponent() {
  const res = await fetch("https://api.github.com/repos/vercel/next.js");
  const repo = await res.json();
  return <div>{repo.name}</div>;
}
