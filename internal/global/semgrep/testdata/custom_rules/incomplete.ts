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
