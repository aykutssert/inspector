// @ts-nocheck
"use client";

import React from "react";

export function UnnecessaryClientComponent() {
  return (
    <div>
      <h1>Hello Server Component</h1>
      <p>This component is static and doesn't need client-side React features.</p>
    </div>
  );
}
