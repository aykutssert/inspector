// @ts-nocheck
import { atom } from "jotai";
import { selectAtom } from "jotai/utils";
import { atomWithQuery } from "jotai-tanstack-query";
import { useAtom, useAtomValue } from "jotai";

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

// ─── jotai-select-atom-in-render-body ────────────────────────────────────────

function QueryView() {
  const userAtom = selectAtom(userStoreAtom, (state) => state.user);
  return userAtom;
}

const InlineQueryView = () => {
  const localAtom = selectAtom(profileStoreAtom, (state) => state.profile);
  return localAtom;
};

const safeSelector = selectAtom(profileStoreAtom, (state) => state.profile);

// ─── jotai-tq-use-raw-query-atom ─────────────────────────────────────────────

const todoAtom = atomWithQuery(() => ({
  queryKey: ["todos"],
  queryFn: fetchTodos,
}));

function TodoList() {
  const todoQuery = useAtomValue(todoAtom);
  return todoQuery;
}

const usedWithSlice = atomWithQuery(() => ({
  queryKey: ["users"],
  queryFn: fetchUsers,
}));

const slicedUserAtom = selectAtom(usedWithSlice, (query) => query.data);

function SafeTodoList() {
  const sliced = useAtomValue(slicedUserAtom);
  const raw = useAtom(slicedUserAtom);
  return sliced ?? raw;
}

// ─── jotai-selectatom-without-stable-equality ─────────────────────────────────

// Violation: selectAtom with object literal return, no equality fn
const badObjectSelector = selectAtom(userStoreAtom, (state) => ({ name: state.name, age: state.age }));

// Violation: selectAtom with array literal return, no equality fn
const badArraySelector = selectAtom(itemsAtom, (state) => [state.a, state.b]);

// Safe: selectAtom returning existing reference (no new object)
const safeExistingRef = selectAtom(profileStoreAtom, (state) => state.profile);
