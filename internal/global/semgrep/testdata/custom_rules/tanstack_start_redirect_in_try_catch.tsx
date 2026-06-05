import { redirect, notFound } from '@tanstack/start';

export function testViolations() {
  // Violation 1: redirect in try-catch
  try {
    redirect({ to: '/dashboard' });
  } catch (error) {
    console.error(error);
  }

  // Violation 2: notFound in try-catch
  try {
    notFound();
  } catch (err) {
    console.log(err);
  }
}

export function testSafe() {
  // Safe 1: redirect outside try-catch
  redirect({ to: '/profile' });

  // Safe 2: redirect in try-finally
  try {
    redirect({ to: '/settings' });
  } finally {
    console.log('done');
  }
}
