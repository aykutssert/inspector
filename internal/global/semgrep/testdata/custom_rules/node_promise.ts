// @ts-nocheck

export function unsafePromises() {
  Promise.reject(new Error("boom"));
  loadJob().then(processJob);
  runJob().then(reportJob);
}

export async function safePromises() {
  return loadJob().then(processJob).catch(reportError);
}
