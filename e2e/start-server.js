const { spawn, spawnSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const databasePath = path.join(process.cwd(), "test-results", "rmm-e2e.db");
const serverPath = path.join(process.cwd(), "test-results", process.platform === "win32" ? "rmm-e2e-server.exe" : "rmm-e2e-server");
const commandKeyPath = path.join(process.cwd(), "test-results", "rmm-e2e-command-key.pem");
const dataKeyPath = path.join(process.cwd(), "test-results", "rmm-e2e-data-encryption.key");
const pidPath = path.join(process.cwd(), "test-results", "rmm-e2e-server.pid");
const stopPath = path.join(process.cwd(), "test-results", "rmm-e2e-server.stop");
fs.mkdirSync(path.dirname(databasePath), { recursive: true });
for (const suffix of ["", "-shm", "-wal"]) {
  fs.rmSync(`${databasePath}${suffix}`, { force: true });
}
for (const keyPath of [commandKeyPath, dataKeyPath]) fs.rmSync(keyPath, { force: true });
for (const controlPath of [pidPath, stopPath]) fs.rmSync(controlPath, { force: true });

const build = spawnSync("go", ["build", "-o", serverPath, "./server/cmd/rmm-server"], {
  cwd: process.cwd(),
  env: process.env,
  stdio: "inherit",
});
if (build.error) throw build.error;
if (build.status !== 0) process.exit(build.status ?? 1);

const server = spawn(serverPath, [], {
  cwd: process.cwd(),
  env: {
    ...process.env,
    RMM_ADDR: ":18081",
    RMM_DB_PATH: databasePath,
    RMM_COMMAND_SIGNING_KEY_PATH: commandKeyPath,
    RMM_DATA_ENCRYPTION_KEY_PATH: dataKeyPath,
    RMM_INSECURE_DEV_MODE: "true",
    RMM_COOKIE_SECURE: "false",
    RMM_WEB_DIR: "web",
    RMM_OPERATOR_USERNAME: "e2e-admin",
    RMM_OPERATOR_PASSWORD: "e2e-password-long-enough",
    RMM_OPERATOR_TOKEN: "e2e-operator-token",
  },
  stdio: "inherit",
});
fs.writeFileSync(pidPath, String(server.pid), { encoding: "utf8", mode: 0o600 });

let stopping = false;
let forceStopTimer;
let stopPollTimer;

function cleanupControlFiles() {
  if (stopPollTimer) clearInterval(stopPollTimer);
  for (const controlPath of [pidPath, stopPath]) fs.rmSync(controlPath, { force: true });
}

function finishStop() {
  if (forceStopTimer) clearTimeout(forceStopTimer);
  cleanupControlFiles();
  process.exit(0);
}

function stopServer(signal) {
  if (stopping) return;
  stopping = true;

  if (!server.kill(signal)) {
    finishStop();
    return;
  }

  // Playwright terminates the wrapper after a run. On Windows the child Go
  // process can outlive that signal and keep the runner open indefinitely.
  forceStopTimer = setTimeout(() => {
    if (process.platform === "win32") {
      spawnSync("taskkill", ["/PID", String(server.pid), "/T", "/F"], { stdio: "ignore" });
    } else {
      server.kill("SIGKILL");
    }
    finishStop();
  }, 5_000);
}

stopPollTimer = setInterval(() => {
  if (fs.existsSync(stopPath)) stopServer("SIGTERM");
}, 100);

for (const signal of ["SIGINT", "SIGTERM", "SIGHUP"]) {
  process.on(signal, () => {
    stopServer(signal);
  });
}
server.on("exit", (code) => {
  if (stopping) finishStop();
  else {
    cleanupControlFiles();
    process.exit(code ?? 1);
  }
});
