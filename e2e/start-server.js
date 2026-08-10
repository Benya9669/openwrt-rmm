const { spawn, spawnSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const databasePath = path.join(process.cwd(), "test-results", "rmm-e2e.db");
const serverPath = path.join(process.cwd(), "test-results", process.platform === "win32" ? "rmm-e2e-server.exe" : "rmm-e2e-server");
fs.mkdirSync(path.dirname(databasePath), { recursive: true });
for (const suffix of ["", "-shm", "-wal"]) {
  fs.rmSync(`${databasePath}${suffix}`, { force: true });
}

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
    RMM_INSECURE_DEV_MODE: "true",
    RMM_COOKIE_SECURE: "false",
    RMM_WEB_DIR: "web",
    RMM_OPERATOR_USERNAME: "e2e-admin",
    RMM_OPERATOR_PASSWORD: "e2e-password-long-enough",
    RMM_OPERATOR_TOKEN: "e2e-operator-token",
  },
  stdio: "inherit",
});

let stopping = false;
for (const signal of ["SIGINT", "SIGTERM"]) {
  process.on(signal, () => {
    if (stopping) return;
    stopping = true;
    if (!server.kill(signal)) process.exit(0);
  });
}
server.on("exit", (code) => process.exit(stopping ? 0 : (code ?? 1)));
