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

export { router };
