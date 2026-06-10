// @ts-nocheck — semgrep js-async-foreach fixture

// ─── UNSAFE ───────────────────────────────────────────────────────────────────

function unsafe1(ids: string[]) {
  ids.forEach(async (id) => {
    await fetch(`/api/${id}`);
  });
}

function unsafe2(items: number[]) {
  items.forEach(async function (item) {
    await processItem(item);
  });
}

function unsafe3(entries: [string, number][]) {
  entries.forEach(async ([key, val]) => {
    await save(key, val);
  });
}

// ─── SAFE ─────────────────────────────────────────────────────────────────────

function safe1(ids: string[]) {
  for (const id of ids) {
    await fetch(`/api/${id}`);
  }
}

function safe2(items: number[]) {
  await Promise.all(items.map((item) => processItem(item)));
}

function safe3(entries: [string, number][]) {
  entries.forEach(([key, val]) => {
    syncOp(key, val);
  });
}
