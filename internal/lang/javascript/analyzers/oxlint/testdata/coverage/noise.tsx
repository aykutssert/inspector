// Fixture: each construct trips a core "noise" rule the inspector suppresses
// (no-underscore-dangle, no-unused-vars, no-shadow). The coverage test asserts
// NONE of these rule ids appear — proving the suppression holds end-to-end, not
// just in the generated config string.
export function compute(_id: string): number {
  // no-underscore-dangle (underscore-prefixed local) + no-unused-vars.
  const _internal = 1;
  // no-unused-vars.
  const unused = 2;

  const dup = 3;
  {
    // no-shadow: inner `dup` shadows the outer binding.
    const dup = 4;
    return dup + Number(_id);
  }
}
