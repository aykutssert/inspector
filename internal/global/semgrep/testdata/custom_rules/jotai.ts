// @ts-nocheck
import { atom } from "jotai";
import { selectAtom } from "jotai/utils";

// ─── jotai-derived-atom-returns-fresh-object ──────────────────────────────────

// Violation: derived atom returns object literal — new ref every read
const userAtom = atom((get) => ({
  name: get(nameAtom),
  age: get(ageAtom),
}));

// Violation: derived atom returns array literal — new ref every read
const itemsAtom = atom((get) => [get(aAtom), get(bAtom)]);

// Violation: async derived atom returns object literal
const asyncDerived = atom(async (get) => ({
  data: await get(asyncBaseAtom),
}));

// Safe: derived atom returns primitive — no reference instability
const countAtom = atom((get) => get(baseCountAtom) * 2);

// Safe: derived atom returns existing reference
const selectedAtom = atom((get) => get(mapAtom).get("key"));
