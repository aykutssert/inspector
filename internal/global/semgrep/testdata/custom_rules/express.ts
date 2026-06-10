// @ts-nocheck — semgrep express golden fixture
// Tests express-missing-helmet, express-missing-rate-limit, express-async-error-handling,
// and jwt-decode-without-verify.

import express from 'express';
import jwt from 'jsonwebtoken';

// ---- UNSAFE section ----
const appUnsafe = express(); // triggers express-missing-helmet

appUnsafe.post('/login', (req, res) => { // triggers express-missing-rate-limit
  const user = req.body.user;
  res.json({ user });
});

appUnsafe.get('/data', async (req, res) => { // triggers express-async-error-handling
  const data = await fetchData();
  res.json(data);
});

appUnsafe.get('/me', (req, res) => {
  // triggers jwt-decode-without-verify: trusts an unverified token payload
  const claims = jwt.decode(req.headers.authorization);
  res.json({ userId: claims.sub });
});

// ---- SAFE section ----
import rateLimit from 'express-rate-limit';
import helmet from 'helmet';

const appSafe = express();
appSafe.use(helmet());
const limiter = rateLimit({ windowMs: 60_000, max: 100 });
appSafe.use(limiter);

appSafe.get('/safe', async (req, res) => { // ok — wrapped in try/catch
  try {
    const data = await fetchData();
    res.json(data);
  } catch (err) {
    res.status(500).json({ error: String(err) });
  }
});

appUnsafe.use(cors({ origin: "*", credentials: true })); // triggers express-cors-wildcard-with-credentials
appUnsafe.use(cors({ credentials: true, origin: '*' })); // triggers express-cors-wildcard-with-credentials
appSafe.use(cors({ origin: "https://example.com", credentials: true }));
appSafe.use(cors({ origin: "*", credentials: false }));

appSafe.get('/sync-config', (req, res) => {
  const config = nodeFs.readFileSync('/etc/service/config.json', 'utf8'); // triggers sync-fs-in-server
  res.send(config);
});

appSafe.post('/sync-audit', function (req, res) {
  writeFileSync('/tmp/audit.log', JSON.stringify(req.body)); // triggers sync-fs-in-server
  res.sendStatus(204);
});

appSafe.post('/parse-profile', (req, res) => {
  const profile = JSON.parse(req.body.profile); // triggers json-parse-without-guard
  res.json(profile);
});

appSafe.post('/parse-profile-safe', (req, res) => {
  try {
    const profile = JSON.parse(req.body.profile);
    res.json(profile);
  } catch (err) {
    res.status(400).json({ error: String(err) });
  }
});

appSafe.post('/parse-local-json', (req, res) => {
  const profile = JSON.parse('{"safe":true}');
  res.json(profile);
});

appUnsafe.post('/hash-password', (req, res) => {
  const hash = bcrypt.hashSync(req.body.password, 10); // triggers crypto-sync-in-request-path
  res.json({ hash });
});

appSafe.post('/hash-password-safe', async (req, res) => {
  const hash = await bcrypt.hash(req.body.password, 10); // ok
  res.json({ hash });
});


appSafe.get('/download', (req, res) => {
  const filePath = path.join('/srv/files', req.query.file); // triggers path-traversal-unchecked-join
  res.sendFile(filePath);
});

appSafe.get('/download-safe', (req, res) => {
  const filePath = path.join('/srv/files', path.basename(req.query.file));
  res.sendFile(filePath);
});

appSafe.get('/download-static', (req, res) => {
  const filePath = path.join('/srv/files', 'manual.pdf');
  res.sendFile(filePath);
});

function preloadConfigAtStartup(nodeFs: any) {
  return nodeFs.readFileSync('/etc/service/config.json', 'utf8');
}

appSafe.use(express.json()); // triggers express-body-limit-missing
appSafe.use(express.urlencoded({ extended: true })); // triggers express-body-limit-missing
appSafe.use(express.json({ limit: "1mb" }));
appSafe.use(express.urlencoded({ extended: true, limit: "500kb" }));

appSafe.use(session({ secret: "test", cookie: { httpOnly: false, secure: true } })); // triggers express-insecure-session-cookie
appSafe.use(session({ secret: "test", cookie: { httpOnly: true, secure: false } })); // triggers express-insecure-session-cookie
appSafe.use(session({ secret: "test", cookie: { httpOnly: true, secure: true } }));

// ─── jwt-verify-without-algorithms ───────────────────────────────────────────

// Violation: jwt.verify without algorithms option (triggers jwt-verify-without-algorithms)
export function UnsafeTokenVerify() {
  const claims = jwt.verify(token, secret);
  return { userId: claims.sub };
}

// Violation: jwt.verify with options but no algorithms (triggers jwt-verify-without-algorithms)
export function UnsafeTokenVerifyWithOpts() {
  const claims = jwt.verify(token, secret, { expiresIn: "1h" });
  return { userId: claims.sub };
}

// Safe: jwt.verify with algorithms
export function SafeTokenVerify() {
  const claims = jwt.verify(token, secret, { algorithms: ["HS256"] });
  return { userId: claims.sub };
}

// ─── express-trust-proxy-misconfig ───────────────────────────────────────────

// Violation: trust proxy disabled (triggers express-trust-proxy-misconfig)
appUnsafe.set("trust proxy", false);

// Violation: trust proxy set to "none" (triggers express-trust-proxy-misconfig)
appUnsafe.set("trust proxy", "none");

// ─── js-prototype-pollution-merge ────────────────────────────────────────────

// Violation: Object.assign with req.body (triggers js-prototype-pollution-merge)
export function UnsafeMerge(req: any, res: any) {
  const config = Object.assign({}, req.body);
  res.json(config);
}

// Violation: Object.assign with req.query (triggers js-prototype-pollution-merge)
export function UnsafeQueryMerge(req: any, res: any) {
  const filters = Object.assign({}, req.query);
  res.json(filters);
}

// Safe: Object.assign with trusted source
export function SafeMerge(req: any, res: any) {
  const config = Object.assign({}, { key: "trusted" });
  res.json(config);
}

// ─── js-hoist-regexp ─────────────────────────────────────────────────────────

// Violation: new RegExp inside function (triggers js-hoist-regexp)
export function MatchUserAgent(req: any, res: any) {
  const pattern = new RegExp("^Mozilla/");
  res.json({ matched: pattern.test(req.headers["user-agent"]) });
}

// Safe: new RegExp at module scope
const modulePattern = new RegExp("^Mozilla/");

// ─── js-hoist-intl ───────────────────────────────────────────────────────────

// Violation: new Intl.NumberFormat inside function (triggers js-hoist-intl)
export function FormatPrice(price: number) {
  const fmt = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD" });
  return fmt.format(price);
}

// Safe: new Intl.NumberFormat at module scope
const currencyFormatter = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD" });
