import express from 'express';

const app = express();
const router = express.Router();

const handler = (req: any, res: any) => res.send('ok');
const auth = (req: any, res: any, next: any) => next();

// Violation 1: sensitive route on app without middleware
app.get('/admin/users', handler);

// Violation 2: sensitive route on router without middleware
router.post('/delete-item', handler);

// Safe 1: non-sensitive route without middleware
app.get('/public-info', handler);

// Safe 2: sensitive route with auth middleware
app.get('/admin/settings', auth, handler);

// Safe 3: sensitive route on router with auth middleware
router.post('/secure/update', auth, handler);
