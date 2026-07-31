const { request } = require("@playwright/test");

module.exports = async () => {
  const api = await request.newContext({
    baseURL: "http://127.0.0.1:18081",
    extraHTTPHeaders: { Authorization: "Bearer e2e-operator-token" },
  });
  const grantResponse = await api.post("/api/enrollment-grants", { data: { dns_label: "e2e-router" } });
  if (!grantResponse.ok()) throw new Error(`failed to create enrollment grant: ${grantResponse.status()}`);
  const { enrollment_token: enrollmentToken } = await grantResponse.json();
  const enrollmentResponse = await api.post("/api/agent/enroll", {
    data: {
      enrollment_token: enrollmentToken,
      hostname: "E2E OpenWrt",
      openwrt_version: "OpenWrt 25.12",
    },
  });
  if (!enrollmentResponse.ok()) throw new Error(`failed to enroll e2e router: ${enrollmentResponse.status()}`);
  const enrolled = await enrollmentResponse.json();
  const agent = await request.newContext({
    baseURL: "http://127.0.0.1:18081",
    extraHTTPHeaders: { Authorization: `Bearer ${enrolled.device_token}` },
  });
  const heartbeatResponse = await agent.post("/api/agent/heartbeat", {
    data: {
      device_id: enrolled.device_id,
      inventory: { hostname: "E2E OpenWrt", model: "Test Router" },
      metrics: { loadavg: "0.00 0.01 0.02", memory_percent: 32 },
    },
  });
  if (!heartbeatResponse.ok()) throw new Error(`failed to seed e2e heartbeat: ${heartbeatResponse.status()}`);
  await Promise.all([api.dispose(), agent.dispose()]);
};
