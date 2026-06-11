import express from "express";

const router = express.Router();
declare const User: any;
declare const globalThis: any;
declare const window: any;

// --- nosql-injection-tainted-query: positive ---
router.post("/login", (req, res) => {
  User.findOne(req.body); // FIRE
});
router.get("/users", (req, res) => {
  User.find(req.query); // FIRE
  User.deleteMany(req.body); // FIRE
});

// --- nosql-injection-tainted-query: negative ---
router.post("/safe", (req, res) => {
  const { email } = req.body;
  User.findOne({ email }); // NO FIRE
  User.find({ active: true }); // NO FIRE
});

// --- dynamic-global-invocation: positive ---
function dispatch(name: string) {
  globalThis[name](); // FIRE
  window[name](1, 2); // FIRE
}

// --- dynamic-global-invocation: negative ---
function staticCall() {
  globalThis["console"]; // NO FIRE (no call)
  window.fetch("/api"); // NO FIRE (static property)
}

// --- dynamic-object-key-assignment: positive ---
function assignFromReq(req: any) {
  const target: any = {};
  target[req.body.key] = "x"; // FIRE
  target[req.query.field] = 1; // FIRE
  return target;
}

// --- dynamic-object-key-assignment: negative ---
function safeAssign(req: any) {
  const target: any = {};
  const { name } = req.body;
  target.name = name; // NO FIRE (static key)
  target["fixed"] = 1; // NO FIRE (literal key)
  return target;
}

// --- error-detail-leak-to-response: positive ---
router.use((err: any, req: any, res: any, next: any) => {
  res.status(500).send(err.stack); // FIRE
  res.json({ error: err.stack }); // FIRE
});

// --- error-detail-leak-to-response: negative ---
router.use((err: any, req: any, res: any, next: any) => {
  console.error(err.stack); // NO FIRE (server-side log)
  res.status(500).json({ error: "Internal error" }); // NO FIRE
});

// --- express-no-code-after-response: positive ---
function handlerBug(req: any, res: any) {
  if (!req.user) {
    res.status(401).send("unauthorized"); // FIRE (no return, falls through)
  }
  deleteAccount(req.user); // runs even when unauthorized
}

// --- express-no-code-after-response: negative ---
function handlerSafe(req: any, res: any) {
  if (!req.user) {
    return res.status(401).send("unauthorized"); // NO FIRE (returned)
  }
  deleteAccount(req.user);
}
declare function deleteAccount(u: any): void;

export { router };

// --- cors-origin-reflection: positive ---
function corsBad(req: any, res: any, next: any) {
  res.setHeader("Access-Control-Allow-Origin", req.headers.origin); // FIRE
  next();
}
const corsAll = cors({ origin: true }); // FIRE

// --- cors-origin-reflection: negative ---
function corsSafe(req: any, res: any) {
  res.setHeader("Access-Control-Allow-Origin", "https://app.example.com"); // NO FIRE
}
declare function cors(opts: any): any;
