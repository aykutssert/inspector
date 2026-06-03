// @ts-nocheck
"use client";

import { useState } from "react";
import fs from "fs"; // should trigger next-server-only-in-client
import { Example } from "./case";

export function ClientComponent() {
  const [state, setState] = useState(0);
  const data = fs.readFileSync("test.txt", "utf8"); // should trigger next-server-only-in-client (via import)
  return <div>{state}</div>;
}
