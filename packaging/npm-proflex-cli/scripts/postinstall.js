"use strict";

const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const https = require("node:https");
const crypto = require("node:crypto");
const cp = require("node:child_process");

function mapPlatform() {
  switch (process.platform) {
    case "win32":
      return "windows";
    case "darwin":
      return "darwin";
    case "linux":
      return "linux";
    default:
      throw new Error(`Unsupported platform: ${process.platform}`);
  }
}

function mapArch() {
  switch (process.arch) {
    case "x64":
      return "amd64";
    case "arm64":
      return "arm64";
    default:
      throw new Error(`Unsupported architecture: ${process.arch}`);
  }
}

function readPackageVersion() {
  const packageJsonPath = path.join(__dirname, "..", "package.json");
  const pkg = JSON.parse(fs.readFileSync(packageJsonPath, "utf8"));
  return pkg.version;
}

function isTruthy(value, defaultValue = true) {
  if (value == null || value === "") return defaultValue;
  const lowered = String(value).toLowerCase();
  return lowered === "1" || lowered === "true" || lowered === "yes" || lowered === "on";
}

function fetchToFile(url, outPath) {
  return new Promise((resolve, reject) => {
    const req = https.get(url, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        fetchToFile(res.headers.location, outPath).then(resolve).catch(reject);
        return;
      }
      if (res.statusCode !== 200) {
        reject(new Error(`HTTP ${res.statusCode} from ${url}`));
        return;
      }
      const out = fs.createWriteStream(outPath);
      res.pipe(out);
      out.on("finish", () => {
        out.close(resolve);
      });
      out.on("error", reject);
    });
    req.on("error", reject);
  });
}

function sha256File(filePath) {
  return new Promise((resolve, reject) => {
    const hash = crypto.createHash("sha256");
    const rs = fs.createReadStream(filePath);
    rs.on("data", (d) => hash.update(d));
    rs.on("error", reject);
    rs.on("end", () => resolve(hash.digest("hex")));
  });
}

function expectedChecksum(checksumsFile, assetName) {
  const lines = fs.readFileSync(checksumsFile, "utf8").split(/\r?\n/);
  for (const line of lines) {
    const m = line.match(/^([A-Fa-f0-9]{64})\s+(.+)$/);
    if (!m) continue;
    if (m[2].trim() === assetName) return m[1].toLowerCase();
  }
  return null;
}

function run(cmd, args) {
  const res = cp.spawnSync(cmd, args, { stdio: "inherit" });
  if (res.error) throw res.error;
  if (res.status !== 0) throw new Error(`${cmd} exited with code ${res.status}`);
}

async function ensureCosign(tempDir, platform, arch) {
  const existing = cp.spawnSync("cosign", ["version"], { stdio: "ignore" });
  if (!existing.error && existing.status === 0) {
    return "cosign";
  }

  const cosignVersion = process.env.PROFLEX_COSIGN_VERSION || "v2.5.3";
  const suffix = platform === "windows" ? ".exe" : "";
  const asset = `cosign-${platform}-${arch}${suffix}`;
  const outFile = path.join(tempDir, asset);
  const url = `https://github.com/sigstore/cosign/releases/download/${cosignVersion}/${asset}`;
  console.log(`[proflex-npm] cosign not found; downloading ${cosignVersion}`);
  await fetchToFile(url, outFile);
  if (platform !== "windows") {
    fs.chmodSync(outFile, 0o755);
  }
  return outFile;
}

async function verifyChecksumsSignature(tempDir, platform, arch, checksumsPath, sigPath, certPath, repo) {
  if (!isTruthy(process.env.PROFLEX_VERIFY_SIGNATURES, true)) {
    console.warn("[proflex-npm] Signature verification disabled via PROFLEX_VERIFY_SIGNATURES=0");
    return;
  }
  const identityRe =
    process.env.PROFLEX_COSIGN_IDENTITY_RE ||
    `^https://github.com/${repo}/.github/workflows/release.yml@refs/tags/.*$`;
  const oidcIssuer = process.env.PROFLEX_COSIGN_OIDC_ISSUER || "https://token.actions.githubusercontent.com";
  const cosignBin = await ensureCosign(tempDir, platform, arch);
  run(cosignBin, [
    "verify-blob",
    "--certificate",
    certPath,
    "--signature",
    sigPath,
    "--certificate-identity-regexp",
    identityRe,
    "--certificate-oidc-issuer",
    oidcIssuer,
    checksumsPath,
  ]);
}

function findBinary(rootDir, wantedName) {
  const queue = [rootDir];
  while (queue.length) {
    const current = queue.shift();
    const entries = fs.readdirSync(current, { withFileTypes: true });
    for (const entry of entries) {
      const full = path.join(current, entry.name);
      if (entry.isDirectory()) {
        queue.push(full);
        continue;
      }
      if (entry.isFile() && entry.name.toLowerCase() === wantedName.toLowerCase()) {
        return full;
      }
    }
  }
  return null;
}

async function main() {
  const version = readPackageVersion();
  if (!version || version.includes("dev")) {
    console.log("[proflex-npm] Development version detected; skipping binary download.");
    return;
  }

  const repo = process.env.PROFLEX_REPO || "derekurban/proflex-cli";
  const platform = mapPlatform();
  const arch = mapArch();
  const extension = platform === "windows" ? "zip" : "tar.gz";
  const asset = `proflex_${version}_${platform}_${arch}.${extension}`;
  const tag = `v${version}`;
  const baseUrl = `https://github.com/${repo}/releases/download/${tag}`;

  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "proflex-npm-"));
  const assetPath = path.join(tempDir, asset);
  const checksumsPath = path.join(tempDir, "checksums.txt");
  const checksumsSigPath = path.join(tempDir, "checksums.txt.sig");
  const checksumsCertPath = path.join(tempDir, "checksums.txt.pem");
  const extractDir = path.join(tempDir, "extract");
  fs.mkdirSync(extractDir, { recursive: true });

  console.log(`[proflex-npm] Downloading ${asset}`);
  await fetchToFile(`${baseUrl}/${asset}`, assetPath);
  await fetchToFile(`${baseUrl}/checksums.txt`, checksumsPath);
  await fetchToFile(`${baseUrl}/checksums.txt.sig`, checksumsSigPath);
  await fetchToFile(`${baseUrl}/checksums.txt.pem`, checksumsCertPath);

  await verifyChecksumsSignature(
    tempDir,
    platform,
    arch,
    checksumsPath,
    checksumsSigPath,
    checksumsCertPath,
    repo,
  );

  const expected = expectedChecksum(checksumsPath, asset);
  if (!expected) {
    throw new Error(`No checksum entry found for ${asset}`);
  }
  const actual = await sha256File(assetPath);
  if (expected !== actual) {
    throw new Error(`Checksum mismatch for ${asset}`);
  }

  if (platform === "windows") {
    run("powershell", [
      "-NoProfile",
      "-Command",
      `Expand-Archive -Path '${assetPath}' -DestinationPath '${extractDir}' -Force`,
    ]);
  } else {
    run("tar", ["-xzf", assetPath, "-C", extractDir]);
  }

  const binaryName = platform === "windows" ? "proflex.exe" : "proflex";
  const foundBinary = findBinary(extractDir, binaryName);
  if (!foundBinary) {
    throw new Error(`Unable to find ${binaryName} in extracted archive`);
  }

  const runtimeDir = path.join(__dirname, "..", "runtime", `${platform}-${arch}`);
  fs.mkdirSync(runtimeDir, { recursive: true });
  const destBinary = path.join(runtimeDir, binaryName);
  fs.copyFileSync(foundBinary, destBinary);
  if (platform !== "windows") {
    fs.chmodSync(destBinary, 0o755);
  }

  console.log(`[proflex-npm] Installed ${binaryName} for ${platform}/${arch}`);
}

main().catch((err) => {
  console.error(`[proflex-npm] Install failed: ${err.message}`);
  process.exit(1);
});
