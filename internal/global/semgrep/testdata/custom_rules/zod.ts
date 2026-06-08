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
