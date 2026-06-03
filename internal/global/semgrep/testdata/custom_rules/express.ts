// @ts-nocheck — semgrep express golden fixture
// Tests express-missing-helmet, express-missing-rate-limit, and express-async-error-handling.

import express from 'express';

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
