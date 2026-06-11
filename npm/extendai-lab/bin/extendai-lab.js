#!/usr/bin/env node
const { spawnSync } = require("node:child_process");

const pkg = `@extendai-lab/cli-${process.platform}-${process.arch}`;
const exe = `extendai-lab${process.platform === "win32" ? ".exe" : ""}`;

let binary;
try {
  binary = require.resolve(`${pkg}/bin/${exe}`);
} catch {
  console.error(
    `extendai-lab: no prebuilt binary for ${process.platform}-${process.arch}.\n` +
      `Install the matching optional package (${pkg}), or build from source:\n` +
      `  https://github.com/Linxira-OS/extendai-lab-cli`,
  );
  process.exit(1);
}

const res = spawnSync(binary, process.argv.slice(2), { stdio: "inherit" });
if (res.error) throw res.error;
process.exit(res.status === null ? 1 : res.status);
