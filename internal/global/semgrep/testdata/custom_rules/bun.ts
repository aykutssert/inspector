// @ts-nocheck — semgrep Bun golden fixture
import { Database } from "bun:sqlite";

export function bunUnsafeSQLite(userId: string) {
  const db = new Database("app.sqlite");
  db.prepare(`SELECT * FROM users WHERE id = ${userId}`); // should trigger bun-sqlite-injection
  db.prepare("SELECT * FROM users WHERE id = " + userId); // should trigger bun-sqlite-injection
  db.query("SELECT * FROM users WHERE id = ?");
}

export function bunUnsafeShell(userInput: string) {
  Bun.$`cat ${userInput}`; // should trigger bun-shell-injection
  Bun.spawn(["sh", "-c", "cat " + userInput]); // should trigger bun-shell-injection
  Bun.spawn(["cat", "--", userInput]);
}
