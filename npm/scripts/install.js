const fs = require('fs');
const path = require('path');
const os = require('os');
const https = require('https');

const VERSION = '1.0.0';
const REPO = 'aykutssert/scout'; // Will be updated to scout in the future
const BIN_DIR = path.join(__dirname, '..', 'bin');

// Resolve skills directory with fallback for developer checkouts
let SKILLS_DIR = path.join(__dirname, '..', 'skills', 'scout');
if (!fs.existsSync(SKILLS_DIR)) {
  SKILLS_DIR = path.join(__dirname, '..', '..', 'skills', 'scout');
}

// Ensure bin directory exists
if (!fs.existsSync(BIN_DIR)) {
  fs.mkdirSync(BIN_DIR, { recursive: true });
}

function getPlatformArchBin() {
  const platform = os.platform();
  const arch = os.arch();

  let goos = '';
  let goarch = '';

  switch (platform) {
    case 'darwin':
      goos = 'darwin';
      break;
    case 'linux':
      goos = 'linux';
      break;
    case 'win32':
      goos = 'windows';
      break;
    default:
      throw new Error(`Unsupported platform: ${platform}`);
  }

  switch (arch) {
    case 'x64':
      goarch = 'amd64';
      break;
    case 'arm64':
      goarch = 'arm64';
      break;
    default:
      throw new Error(`Unsupported architecture: ${arch}`);
  }

  const suffix = goos === 'windows' ? '.exe' : '';
  const nativeBinName = `scout-bin${suffix}`;
  const releaseBinName = `scout-${goos}-${goarch}${suffix}`;

  return { goos, goarch, nativeBinName, releaseBinName };
}

function downloadBinary(releaseBinName, destPath) {
  const url = `https://github.com/${REPO}/releases/download/v${VERSION}/${releaseBinName}`;
  console.log(`Downloading prebuilt binary from: ${url}`);

  return new Promise((resolve, reject) => {
    https.get(url, (res) => {
      if (res.statusCode === 302 || res.statusCode === 301) {
        // Handle redirect
        downloadBinary(res.headers.location, destPath).then(resolve).catch(reject);
        return;
      }

      if (res.statusCode !== 200) {
        reject(new Error(`Failed to download binary: HTTP ${res.statusCode}`));
        return;
      }

      const fileStream = fs.createWriteStream(destPath);
      res.pipe(fileStream);

      fileStream.on('finish', () => {
        fileStream.close();
        // Set executable permissions on unix systems
        if (os.platform() !== 'win32') {
          fs.chmodSync(destPath, 0o755);
        }
        resolve();
      });
    }).on('error', (err) => {
      reject(err);
    });
  });
}

function linkOrCopy(source, dest) {
  const destDir = path.dirname(dest);
  if (!fs.existsSync(destDir)) {
    fs.mkdirSync(destDir, { recursive: true });
  }

  // Remove existing file or symlink at dest
  if (fs.existsSync(dest)) {
    try {
      fs.unlinkSync(dest);
    } catch (e) {
      fs.rmSync(dest, { recursive: true, force: true });
    }
  } else {
    // If it's a broken symlink, fs.existsSync returns false, but we still need to delete it
    try {
      fs.unlinkSync(dest);
    } catch (e) {}
  }

  // Calculate relative source if both are within the project root (so committed symlinks aren't broken)
  const projectRoot = process.env.INIT_CWD || process.cwd();
  let linkTarget = source;
  if (source.startsWith(projectRoot) && dest.startsWith(projectRoot)) {
    linkTarget = path.relative(destDir, source);
  }

  try {
    const type = os.platform() === 'win32' ? 'file' : null;
    fs.symlinkSync(linkTarget, dest, type);
    console.log(`[OK] Symlinked: ${dest} -> ${linkTarget}`);
  } catch (err) {
    // Fallback to copy
    fs.copyFileSync(source, dest);
    console.log(`[OK] Copied (fallback): ${dest}`);
  }
}

function isSymbolicLink(p) {
  try {
    return fs.lstatSync(p).isSymbolicLink();
  } catch (e) {
    return false;
  }
}

function copyDirRecursive(src, dest) {
  fs.mkdirSync(dest, { recursive: true });
  const entries = fs.readdirSync(src, { withFileTypes: true });
  for (const entry of entries) {
    const srcPath = path.join(src, entry.name);
    const destPath = path.join(dest, entry.name);
    if (entry.isDirectory()) {
      copyDirRecursive(srcPath, destPath);
    } else {
      fs.copyFileSync(srcPath, destPath);
    }
  }
}

function linkOrCopyDir(sourceDir, destDir, forceCopy = false) {
  const parentDir = path.dirname(destDir);
  if (!fs.existsSync(parentDir)) {
    fs.mkdirSync(parentDir, { recursive: true });
  }

  // Remove existing file, folder, or symlink at destDir
  if (fs.existsSync(destDir) || isSymbolicLink(destDir)) {
    try {
      fs.rmSync(destDir, { recursive: true, force: true });
    } catch (e) {
      try {
        fs.unlinkSync(destDir);
      } catch (err) {}
    }
  }

  if (forceCopy) {
    copyDirRecursive(sourceDir, destDir);
    console.log(`[OK] Copied directory: ${destDir}`);
    return;
  }

  // Calculate relative source if both are within the project root
  const projectRoot = process.env.INIT_CWD || process.cwd();
  let linkTarget = sourceDir;
  if (sourceDir.startsWith(projectRoot) && destDir.startsWith(projectRoot)) {
    linkTarget = path.relative(parentDir, sourceDir);
  }

  try {
    const type = os.platform() === 'win32' ? 'junction' : 'dir';
    fs.symlinkSync(linkTarget, destDir, type);
    console.log(`[OK] Symlinked directory: ${destDir} -> ${linkTarget}`);
  } catch (err) {
    // Fallback to copy
    copyDirRecursive(sourceDir, destDir);
    console.log(`[OK] Copied directory (fallback): ${destDir}`);
  }
}

function installRules() {
  console.log('Installing AI agent rules and skills...');
  const home = os.homedir();
  
  // Current working directory (where the user is installing the CLI)
  const projectRoot = process.env.INIT_CWD || process.cwd();

  const claudeHome = process.env.CLAUDE_CONFIG_DIR || path.join(home, '.claude');

  // Every skill directory shipped under skills/ (scout, scout-context, ...).
  const skillsRoot = path.dirname(SKILLS_DIR);
  const skillNames = fs.readdirSync(skillsRoot).filter(
    (name) => fs.existsSync(path.join(skillsRoot, name, 'SKILL.md'))
  );

  // Agent installation targets. globalSkills/localSkills are the skills/ parent
  // directory; each shipped skill is copied into <skillsDir>/<skillName>.
  const agentsList = [
    {
      name: 'Claude Code',
      detectDir: claudeHome,
      globalSkills: path.join(claudeHome, 'skills'),
      localSkills: path.join(projectRoot, '.claude', 'skills'),
      forceCopy: false
    },
    {
      name: 'Codex',
      detectDir: path.join(home, '.codex'),
      globalSkills: path.join(home, '.codex', 'skills'),
      localSkills: path.join(projectRoot, '.agents', 'skills'),
      forceCopy: true
    }
  ];

  agentsList.forEach((agent) => {
    if (!fs.existsSync(agent.detectDir)) return;
    console.log(`Detected agent setup directory for ${agent.name} at: ${agent.detectDir}`);
    skillNames.forEach((skill) => {
      const src = path.join(skillsRoot, skill);
      if (agent.globalSkills) {
        linkOrCopyDir(src, path.join(agent.globalSkills, skill), agent.forceCopy);
      }
      if (agent.localSkills) {
        linkOrCopyDir(src, path.join(agent.localSkills, skill), agent.forceCopy);
      }
    });
  });
}

async function main() {
  const { nativeBinName, releaseBinName } = getPlatformArchBin();
  const destPath = path.join(BIN_DIR, nativeBinName);

  try {
    // Try downloading prebuilt binary from GitHub Releases
    await downloadBinary(releaseBinName, destPath);
    console.log('Prebuilt binary downloaded successfully.');
  } catch (err) {
    console.warn(`Could not download prebuilt binary: ${err.message}`);
    console.log('Attempting local build fallback (development mode)...');

    // Fallback: copy local compiled binary if running in developer checkout
    const projectRootDir = path.join(__dirname, '..', '..');
    const localBinPath = path.join(projectRootDir, 'scout-bin');

    if (fs.existsSync(localBinPath)) {
      fs.copyFileSync(localBinPath, destPath);
      if (os.platform() !== 'win32') {
        fs.chmodSync(destPath, 0o755);
      }
      console.log(`[OK] Copied local developer binary to: ${destPath}`);
    } else {
      console.error('ERROR: No prebuilt binary found and no local developer binary found at root.');
      console.error("Please compile the binary first using 'go build -o scout-bin ./cmd/scout'.");
      process.exit(1);
    }
  }

  installRules();
  console.log('Scout installation finished successfully.');
}

main().catch((err) => {
  console.error(`Scout install script failed: ${err.message}`);
  process.exit(1);
});
