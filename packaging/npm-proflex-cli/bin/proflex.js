#!/usr/bin/env node
"use strict";

const fs = require("node:fs");
const path = require("node:path");
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

function binaryPath() {
  const os = mapPlatform();
  const arch = mapArch();
  const execName = os === "windows" ? "proflex.exe" : "proflex";
  return path.join(__dirname, "..", "runtime", `${os}-${arch}`, execName);
}

function main() {
  let bin;
  try {
    bin = binaryPath();
  } catch (err) {
    console.error(`[proflex-npm] ${err.message}`);
    process.exit(1);
  }

  if (!fs.existsSync(bin)) {
    console.error("[proflex-npm] Proflex binary is missing for this platform.");
    console.error("[proflex-npm] Reinstall with: npm i -g proflex-cli");
    process.exit(1);
  }

  const child = cp.spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
  if (child.error) {
    console.error(`[proflex-npm] Failed to launch binary: ${child.error.message}`);
    process.exit(1);
  }
  process.exit(child.status ?? 1);
}

main();
