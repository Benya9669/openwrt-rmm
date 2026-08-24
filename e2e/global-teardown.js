const fs = require("node:fs");
const path = require("node:path");

const pidPath = path.join(process.cwd(), "test-results", "rmm-e2e-server.pid");
const stopPath = path.join(process.cwd(), "test-results", "rmm-e2e-server.stop");

function delay(milliseconds) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}

module.exports = async () => {
  if (process.env.RMM_E2E_REUSE_SERVER === "true" || !fs.existsSync(pidPath)) return;

  fs.writeFileSync(stopPath, "stop\n", { encoding: "utf8", mode: 0o600 });
  const deadline = Date.now() + 7_000;
  while (fs.existsSync(pidPath) && Date.now() < deadline) await delay(100);
  if (fs.existsSync(pidPath)) throw new Error("timed out while stopping the E2E server");
};
