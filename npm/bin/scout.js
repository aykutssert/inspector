#!/usr/bin/env node

const path = require('path');
const os = require('os');
const spawn = require('child_process').spawn;

const args = process.argv.slice(2);

// Handle manual skill/plugin installation command
if (args[0] === 'install') {
  const installScript = path.join(__dirname, '..', 'scripts', 'install.js');
  const child = spawn(process.execPath, [installScript], { stdio: 'inherit' });
  child.on('close', (code) => {
    process.exit(code !== null ? code : 0);
  });
  return;
}

// Determine binary name based on platform
const isWin = os.platform() === 'win32';
const binName = isWin ? 'scout-bin.exe' : 'scout-bin';
const binPath = path.join(__dirname, binName);

// Spawn the native binary and forward all arguments
const child = spawn(binPath, args, { stdio: 'inherit' });

child.on('close', (code) => {
  process.exit(code !== null ? code : 0);
});

child.on('error', (err) => {
  console.error(`Error executing scout binary: ${err.message}`);
  console.error("Please run 'scout install' to repair or download the binary.");
  process.exit(1);
});
