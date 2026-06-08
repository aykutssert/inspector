// @ts-nocheck
"use client";

import { useSelector } from "react-redux";
import { createSelector } from "@reduxjs/toolkit";

interface RootState {
  items: { id: number; name: string; active: boolean }[];
  user: { name: string; email: string };
}

// ─── redux-useselector-inline-derivation ─────────────────────────────────────

// Violation: filter() inside useSelector — new array every render
function ActiveItemList() {
  const activeItems = useSelector((s: RootState) => s.items.filter((i) => i.active));
  return <ul>{activeItems.map((i) => <li key={i.id}>{i.name}</li>)}</ul>;
}

// Violation: map() inside useSelector
function ItemNames() {
  const names = useSelector((s: RootState) => s.items.map((i) => i.name));
  return <ul>{names.map((n) => <li key={n}>{n}</li>)}</ul>;
}

// Safe: stable slice selector
function UserEmail() {
  const email = useSelector((s: RootState) => s.user.email);
  return <span>{email}</span>;
}

// Safe: memoized selector with createSelector
const selectActiveItems = createSelector(
  (s: RootState) => s.items,
  (items) => items.filter((i) => i.active)
);

function SafeActiveItemList() {
  const activeItems = useSelector(selectActiveItems);
  return <ul>{activeItems.map((i) => <li key={i.id}>{i.name}</li>)}</ul>;
}

// ─── redux-useselector-returns-new-collection ─────────────────────────────────

// Violation: object literal — new ref every render
function UserSummary() {
  const summary = useSelector((s: RootState) => ({ name: s.user.name, email: s.user.email }));
  return <div>{summary.name}</div>;
}

// Violation: array literal — new ref every render
function FixedItems() {
  const pair = useSelector((s: RootState) => [s.items[0], s.items[1]]);
  return <div>{pair[0]?.name}</div>;
}

// Safe: select stable primitive
function UserName() {
  const name = useSelector((s: RootState) => s.user.name);
  return <span>{name}</span>;
}
