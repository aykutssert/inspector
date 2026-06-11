import crypto from "crypto";
import https from "https";

// --- weak-cipher-algorithm: positive ---
export function badCiphers(key: Buffer, iv: Buffer) {
  crypto.createCipher("aes-256-cbc", "pw"); // FIRE (legacy createCipher)
  crypto.createCipheriv("des-ede3", key, iv); // FIRE (3DES)
  crypto.createCipheriv("rc4", key, iv); // FIRE (RC4)
  crypto.createCipheriv("aes-128-ecb", key, iv); // FIRE (ECB)
}

// --- weak-cipher-algorithm: negative ---
export function goodCipher(key: Buffer, iv: Buffer) {
  crypto.createCipheriv("aes-256-gcm", key, iv); // NO FIRE
}

// --- insecure-tls-disabled-verification: positive ---
export function badTls() {
  const agent = new https.Agent({ rejectUnauthorized: false }); // FIRE
  process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0"; // FIRE
  return agent;
}

// --- insecure-tls-disabled-verification: negative ---
export function goodTls(ca: Buffer) {
  return new https.Agent({ rejectUnauthorized: true, ca }); // NO FIRE
}

// --- weak-crypto-hash: positive ---
export function badHash(data: string) {
  crypto.createHash("md5").update(data).digest("hex"); // FIRE
  crypto.createHash("sha1").update(data).digest("hex"); // FIRE
}

// --- weak-crypto-hash: negative ---
export function goodHash(data: string) {
  crypto.createHash("sha256").update(data).digest("hex"); // NO FIRE
}
