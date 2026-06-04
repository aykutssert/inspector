// @ts-nocheck — semgrep golden fixture for incomplete-code / mock-leakage rules.
import { faker } from "@faker-js/faker"; // should trigger mock-data-in-production

export function chargeCard(): void {
  throw new Error("Not implemented"); // should trigger incomplete-not-implemented
}

export function syncInventory(): void {
  throw new Error("TODO: implement me"); // should trigger incomplete-not-implemented
}

export function legacyHook(): void {
  throw new NotImplementedError(); // should trigger incomplete-not-implemented
}

export function seedUser() {
  return { id: faker.string.uuid(), name: faker.person.fullName() };
}

export function realCharge(amount: number): number {
  if (amount < 0) {
    throw new Error("amount must be positive"); // should be ok
  }
  return amount * 100;
}

export async function weakBcryptCosts(bcryptModule: any, password: string) {
  await bcryptModule.hash(password, 4); // should trigger bcrypt-weak-cost
  bcryptModule.hashSync(password, 8); // should trigger bcrypt-weak-cost
  await bcryptModule.genSalt(5); // should trigger bcrypt-weak-cost
  bcryptModule.genSaltSync(9); // should trigger bcrypt-weak-cost
}

export async function acceptableBcryptCosts(bcryptModule: any, password: string) {
  await bcryptModule.hash(password, 10);
  bcryptModule.hashSync(password, 12);
  await bcryptModule.genSalt(10);
  bcryptModule.genSaltSync(14);
}

export function ignoredTypeErrorWithoutReason(value: unknown) {
  // @ts-ignore
  return value.missing;
}

export function ignoredTypeErrorWithReason(value: unknown) {
  // @ts-ignore: third-party type definition is stale in this fixture
  return value.missing;
}

export function expectedTypeErrorWithReason(value: unknown) {
  // @ts-expect-error: fixture intentionally accesses an unknown property
  return value.missing;
}

export function mixedModuleSystems() {
  const path = require("path"); // should trigger mixed-esm-cjs
  return path.join("a", "b");
}
