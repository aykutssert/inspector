// @ts-nocheck
import React from "react";
import axios from "axios";

// Violation: Server Component fetching internal relative API route
export async function ServerComponentOne() {
  const res = await fetch("/api/users");
  const users = await res.json();
  return <div>{users.length} users</div>;
}

// Violation: Server Component fetching internal absolute localhost API route using axios
export async function ServerComponentTwo() {
  const res = await axios.get("http://localhost:3000/api/posts");
  const posts = res.data;
  return <div>{posts.length} posts</div>;
}

// Violation: Server Component fetching internal localhost API route using axios default method call
export async function ServerComponentThree() {
  const res = await axios("http://127.0.0.1:3000/api/comments");
  const comments = res.data;
  return <div>{comments.length} comments</div>;
}

// Safe Case: Client Component fetching internal API route (perfectly fine for Client Components)
// Note the "use client" directive at the very top of the block or file.
// Wait, Client Component must be marked with "use client" at file top level.
// Let's create another file or just a sub-module. Since "pattern-not-inside" checks the whole file,
// we'll put the client component tests in a separate component in client.tsx, but here we can just test
// third-party APIs. Let's make sure we test a safe third-party API fetch in a Server Component:
export async function ServerComponentSafeThirdParty() {
  const res = await fetch("https://api.github.com/repos/vercel/next.js");
  const repo = await res.json();
  return <div>{repo.name}</div>;
}

// Safe Case: Server Component fetching external API that happens to have /api/ but not local
export async function ServerComponentSafeExternalApi() {
  const res = await fetch("https://some-external-service.com/api/v1/data");
  const data = await res.json();
  return <div>Data loaded</div>;
}

// Safe Case: Calling direct DB layer or service function instead of fetch
import { getUsers } from "../services/db";
export async function ServerComponentDirect() {
  const users = await getUsers();
  return <div>{users.length} users</div>;
}
