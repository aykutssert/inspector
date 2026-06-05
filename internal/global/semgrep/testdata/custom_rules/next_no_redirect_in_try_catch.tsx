import { redirect, permanentRedirect } from 'next/navigation';

export function TestViolations() {
  // Violation 1: redirect in try-catch
  try {
    redirect('/dashboard');
  } catch (error) {
    console.error(error);
  }

  // Violation 2: permanentRedirect in try-catch
  try {
    permanentRedirect('/login');
  } catch (err) {
    console.log(err);
  }
}

export function TestSafe() {
  // Safe 1: redirect outside try-catch
  redirect('/profile');

  // Safe 2: redirect in try-finally (no catch to suppress error)
  try {
    redirect('/settings');
  } finally {
    console.log('cleanup');
  }
}
