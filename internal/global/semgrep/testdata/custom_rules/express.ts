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
