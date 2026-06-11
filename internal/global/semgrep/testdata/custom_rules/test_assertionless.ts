// --- positive: tests with no assertion ---
it("should create a user", () => {
  const user = makeUser(); // FIRE (no assertion)
  user.activate();
});

test("renders", async () => {
  await render(); // FIRE
});

// --- negative: tests with assertions ---
it("should return the user id", () => {
  const user = makeUser();
  expect(user.id).toBe(1); // NO FIRE
});

test("rejects invalid input", async () => {
  await expect(validate(null)).rejects.toThrow(); // NO FIRE
});

it("uses a custom assert helper", () => {
  assert(makeUser().id === 1); // NO FIRE (assert)
});

it("snapshot", () => {
  expect(makeUser()).toMatchSnapshot(); // NO FIRE
});

declare function makeUser(): any;
declare function render(): Promise<void>;
declare function validate(x: any): Promise<void>;
declare function expect(x: any): any;
declare function assert(x: any): void;
