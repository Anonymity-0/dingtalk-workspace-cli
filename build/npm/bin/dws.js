#!/usr/bin/env node

"use strict";

const fs = require("fs");
const path = require("path");
const childProcess = require("child_process");

const binaryPath = path.join(__dirname, "..", "vendor", process.platform === "win32" ? "dws.exe" : "dws");

if (!fs.existsSync(binaryPath)) {
  console.error(`dws binary not found at ${binaryPath}. Reinstall the package.`);
  process.exit(1);
}

const child = childProcess.spawn(binaryPath, process.argv.slice(2), {
  stdio: "inherit",
});

let spawnFailed = false;
let forwardedSignal = null;
const forwardedSignals = ["SIGINT", "SIGTERM"];

function forwardSignal(signal) {
  forwardedSignal = signal;
  if (child.exitCode === null && child.signalCode === null) {
    child.kill(signal);
  }
}

const signalHandlers = new Map(
  forwardedSignals.map((signal) => [signal, () => forwardSignal(signal)]),
);
for (const signal of forwardedSignals) {
  process.on(signal, signalHandlers.get(signal));
}

child.on("error", (error) => {
  spawnFailed = true;
  console.error(error.message);
});

child.on("close", (code, signal) => {
  for (const forwarded of forwardedSignals) {
    process.removeListener(forwarded, signalHandlers.get(forwarded));
  }

  const exitSignal = forwardedSignal || signal;
  if (exitSignal && process.platform !== "win32") {
    process.kill(process.pid, exitSignal);
    return;
  }
  process.exitCode = spawnFailed || code === null ? 1 : code;
});
