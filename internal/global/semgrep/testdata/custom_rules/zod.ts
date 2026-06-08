// @ts-nocheck
import { z } from "zod";

// ─── zod-v4-prefer-top-level-string-formats ──────────────────────────────────

// Violations: chained format methods — use top-level z.email() etc. in Zod v4
const emailSchema = z.string().email();
const urlSchema = z.string().url();
const uuidSchema = z.string().uuid();
const nanoidSchema = z.string().nanoid();
const emojiSchema = z.string().emoji();
const ipSchema = z.string().ip();
const base64Schema = z.string().base64();

// Safe: already using top-level format helpers
const safeEmail = z.email();
const safeUrl = z.url();
const safeUuid = z.uuid();

// Safe: string without format — plain string validation
const name = z.string().min(1).max(100);
const description = z.string().optional();

// ─── zod-v4-no-deprecated-error-customization ────────────────────────────────

// Violation: errorMap option renamed to error in Zod v4
const ageSchema = z.number({ errorMap: (issue, ctx) => ({ message: "Age must be a number" }) });
const nameSchema = z.string({ required_error: "Name required", errorMap: () => ({ message: "Invalid" }) });

// Safe: Zod v4 error option
const safeAge = z.number({ error: () => "Age must be a number" });

// ─── zod-v4-no-deprecated-schema-apis ────────────────────────────────────────

// Violation: z.preprocess deprecated — use z.pipe()
const coercedNumber = z.preprocess((val) => Number(val), z.number());
const coercedDate = z.preprocess((val) => new Date(val as string), z.date());

// Safe: z.pipe with explicit transform
const safeCoerced = z.pipe(z.unknown().transform((val) => Number(val)), z.number());

// ─── zod-v4-no-deprecated-error-apis ─────────────────────────────────────────

// Violation: .errors is deprecated alias for .issues in Zod v4
function validateInput(data: unknown) {
  try {
    z.string().parse(data);
  } catch (err) {
    if (err instanceof z.ZodError) {
      console.log(err.errors); // bad — use err.issues
      return err.errors;
    }
  }
}

// Safe: .issues is the Zod v4 canonical property
function safeValidate(data: unknown) {
  try {
    z.string().parse(data);
  } catch (err) {
    if (err instanceof z.ZodError) {
      return err.issues; // correct
    }
  }
}
