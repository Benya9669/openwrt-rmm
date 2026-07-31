const { spawn } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const databasePath = path.join(process.cwd(), "test-results", "rmm-e2e.db");
fs.mkdirSync(path.dirname(databasePath), { recursive: true });
for (const suffix of ["", "-shm", "-wal"]) {
  fs.rmSync(`${databasePath}${suffix}`, { force: true });
}

const server = spawn("go", ["run", "./server/cmd/rmm-server"], {
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

for (const signal of ["SIGINT", "SIGTERM"]) {
  process.on(signal, () => server.kill(signal));
}
server.on("exit", (code) => process.exit(code ?? 1));
