// Violation 1: nested quantifier in literal
const re1 = /([a-zA-Z]+)*/;

// Violation 2: nested quantifier in RegExp constructor
const re2 = new RegExp("([a-z]+)*");

// Violation 3: nested quantifier inside nested quantifier
const re3 = /(a+)+/;

// Safe 1: simple quantifier
const safeRe1 = /[a-z]+/;

// Safe 2: non-nested quantifier in constructor
const safeRe2 = new RegExp("^[a-z]+$");
