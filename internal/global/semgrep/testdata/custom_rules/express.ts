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
