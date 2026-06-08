// trigger rn-detox-missing-await
element(by.id('login_button')).tap();

// trigger rn-detox-missing-await
expect(element(by.id('welcome'))).toBeVisible();

// safe: awaited
await element(by.id('login_button')).tap();
await expect(element(by.id('welcome'))).toBeVisible();
