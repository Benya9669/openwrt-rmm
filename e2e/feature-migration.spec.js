const { test, expect } = require("@playwright/test");

const viewports = [
  { width: 320, height: 568 },
  { width: 360, height: 800 },
  { width: 390, height: 844 },
  { width: 430, height: 932 },
  { width: 600, height: 800 },
  { width: 768, height: 1024 },
  { width: 900, height: 700 },
  { width: 1024, height: 768 },
  { width: 1280, height: 800 },
  { width: 1366, height: 768 },
  { width: 1440, height: 900 },
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
];

const now = Date.now();
const iso = (offsetMs = 0) => new Date(now + offsetMs).toISOString();

const clients = [
  { key: "wifi", hostname: "workstation-accounting-department-floor-three", ip: "2001:db8:85a3::8a2e:370:7334", mac: "8C:17:59:42:7A:11", interface: "wlan0", connection: "wifi", status: "online", confirmation: "wifi_station", last_seen_at: iso(-20_000), last_checked_at: iso(-20_000) },
  { key: "wired", hostname: "core-switch-laboratory", ip: "192.168.10.18", mac: "2C:F0:5D:A1:3B:82", interface: "br-lan", connection: "wired", status: "online", confirmation: "active_probe", last_seen_at: iso(-30_000), last_checked_at: iso(-20_000) },
  { key: "offline", hostname: "meeting-room-display", ip: "192.168.10.67", mac: "70:3A:CB:08:14:E0", interface: "wlan1", connection: "wifi", status: "offline", confirmation: "lease", last_seen_at: iso(-2 * 60 * 60 * 1000), last_checked_at: iso(-10 * 60 * 1000) },
  { key: "unknown", hostname: "", ip: "192.168.10.91", mac: "AA:BB:CC:DD:90:AB", interface: "", connection: "dhcp", status: "unconfirmed", confirmation: "lease", last_checked_at: iso(-10 * 60 * 1000) },
];

const commands = [
  { id: "cmd_running", type: "ping", status: "claimed", args: { target: "1.1.1.1" }, output: "PING 1.1.1.1\n64 bytes from 1.1.1.1: seq=1 ttl=56 time=18.2 ms", attempt_count: 1, max_attempts: 3, created_at: iso(-90_000), claimed_at: iso(-80_000), expires_at: iso(10 * 60_000) },
  { id: "cmd_completed", type: "interfaces_show", status: "completed", output: `INTERFACES\n${"2001:db8:85a3::8a2e:370:7334 dev br-lan scope global dynamic\n".repeat(80)}`, attempt_count: 1, max_attempts: 3, created_at: iso(-20 * 60_000), completed_at: iso(-19 * 60_000), expires_at: iso(10 * 60_000) },
  { id: "cmd_failed", type: "traceroute", status: "failed", output: "network unreachable: a deliberately long technical failure message that must remain inside the command console and never widen the page", attempt_count: 3, max_attempts: 3, created_at: iso(-40 * 60_000), completed_at: iso(-35 * 60_000), expires_at: iso(-30 * 60_000) },
  { id: "cmd_expired", type: "route_show", status: "expired", output: "command claim timeout", attempt_count: 0, max_attempts: 3, created_at: iso(-60 * 60_000), expired_at: iso(-50 * 60_000), expires_at: iso(-50 * 60_000) },
];

const alerts = [
  { id: "alert-critical", type: "wan_down", severity: "critical", status: "active", message: "WAN interface did not answer the last connectivity checks", details: { interface: "wan.2001:db8::100", target: "2001:4860:4860::8888" }, first_seen_at: iso(-2 * 60 * 60_000), last_seen_at: iso(-30_000), created_at: iso(-2 * 60 * 60_000) },
  { id: "alert-warning", type: "latency_high", severity: "warning", status: "acknowledged", message: "Latency remains above the configured operational threshold", details: { target: "1.1.1.1", latency_ms: 282 }, first_seen_at: iso(-30 * 60_000), last_seen_at: iso(-60_000), created_at: iso(-30 * 60_000) },
  { id: "alert-resolved", type: "memory_high", severity: "warning", status: "resolved", message: "Memory pressure returned to normal", details: { used_percent: 91 }, first_seen_at: iso(-6 * 60 * 60_000), last_seen_at: iso(-4 * 60 * 60_000), resolved_at: iso(-4 * 60 * 60_000), created_at: iso(-6 * 60 * 60_000) },
];

const remoteSessions = [
  { id: "remote-active", device_id: "fixture-device", status: "active", access_state: "ready", server_host: "tunnel.infrastructure.example.internal", remote_port: 22001, luci_port: 28001, created_at: iso(-10 * 60_000), expires_at: iso(3 * 60_000) },
  { id: "remote-creating", device_id: "fixture-device", status: "queued", server_host: "tunnel.infrastructure.example.internal", created_at: iso(-30_000), expires_at: iso(15 * 60_000) },
  { id: "remote-expired", device_id: "fixture-device", status: "expired", server_host: "tunnel.infrastructure.example.internal", remote_port: 22002, created_at: iso(-2 * 60 * 60_000), expires_at: iso(-60 * 60_000) },
  { id: "remote-failed", device_id: "fixture-device", status: "failed", server_host: "tunnel.infrastructure.example.internal", created_at: iso(-30 * 60_000), expires_at: iso(-20 * 60_000) },
];

async function installFeatureFixtures(page) {
  await page.route("**/api/devices", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    const device = data.devices && data.devices[0];
    if (device) {
      device.inventory = {
        ...(device.inventory || {}),
        hostname: "edge-router-production-site-with-a-long-name",
        wan_ip: "2001:db8:85a3::8a2e:370:7334",
        default_route: "default via fe80::1 dev wan.4094 metric 1024",
        wifi_clients: [{ mac: clients[0].mac, interface: "wlan0", signal_dbm: -54, rx_rate: "866 Mbps", tx_rate: "721 Mbps" }],
        interfaces: [
          { name: "br-lan", family: "inet", address: "192.168.10.1/24" },
          { name: "br-lan", family: "inet6", address: "2001:db8:85a3::8a2e:370:7334/64" },
          { name: "br-lan", family: "inet6", address: "fe80::f816:3eff:fe21:67cf/64" },
          { name: "wan.4094", family: "inet6", address: "2001:db8:ffff:ffff:ffff:ffff:ffff:ffff/128" },
        ],
      };
      device.metrics = {
        ...(device.metrics || {}),
        interface_counters: [
          { name: "br-lan", rx_bytes: 9876543210, rx_packets: 400002, rx_errors: 0, tx_bytes: 4567890123, tx_packets: 380001, tx_errors: 0 },
          { name: "wan.4094", rx_bytes: 123456789, rx_packets: 9002, rx_errors: 7, tx_bytes: 98765432, tx_packets: 8011, tx_errors: 2 },
        ],
      };
    }
    await route.fulfill({ response, json: data });
  });

  await page.route("**/api/devices/*/clients", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ clients }) }));
  await page.route("**/api/devices/*/alerts?*", (route) => {
    const filter = new URL(route.request().url()).searchParams.get("status");
    const filtered = filter === "resolved" ? alerts.filter((alert) => alert.status === "resolved") : filter === "all" ? alerts : alerts.filter((alert) => alert.status !== "resolved");
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ alerts: filtered }) });
  });
  await page.route("**/api/devices/*/commands?*", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ commands }) }));
  await page.route("**/api/devices/*/commands", (route) => route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify(commands[0]) }));
  await page.route("**/api/devices/*/remote-sessions?*", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ remote_sessions: remoteSessions }) }));
  await page.route("**/api/devices/*/remote-sessions", (route) => route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify(remoteSessions[1]) }));
  await page.route("**/api/devices/*/remote-sessions/*/close", (route) => route.fulfill({ status: 200, contentType: "application/json", body: "{}" }));
}

async function loginAndOpenDevice(page) {
  await page.goto("/login");
  await page.locator("#loginUsername").fill("e2e-admin");
  await page.locator("#loginPassword").fill("e2e-password-long-enough");
  await page.locator("#loginForm").press("Enter");
  await expect(page.locator("#appShell")).toBeVisible();
  await page.getByRole("button", { name: /Открыть роутер edge-router-production-site/ }).click();
  await expect(page.locator("#deviceView")).toBeVisible();
}

async function expectNoOverflow(page, selector) {
  const metrics = await page.evaluate((target) => {
    const element = document.querySelector(target);
    const offenders = element ? [...element.querySelectorAll("*")]
      .map((item) => ({ selector: `${item.tagName.toLowerCase()}${item.id ? `#${item.id}` : ""}${item.classList.length ? `.${[...item.classList].join(".")}` : ""}`, overflow: item.scrollWidth - item.clientWidth }))
      .filter((item) => item.overflow > 0)
      .sort((left, right) => right.overflow - left.overflow)
      .slice(0, 5) : [];
    return { page: document.documentElement.scrollWidth - window.innerWidth, local: element ? element.scrollWidth - element.clientWidth : 9999, offenders };
  }, selector);
  expect(metrics.page, `${selector} creates page overflow: ${JSON.stringify(metrics.offenders)}`).toBeLessThanOrEqual(0);
  expect(metrics.local, `${selector} creates local overflow: ${JSON.stringify(metrics.offenders)}`).toBeLessThanOrEqual(0);
}

test("Clients, Network, Problems, Operations, Diagnostics and Remote Access fit the responsive matrix", async ({ page }) => {
  await installFeatureFixtures(page);
  await loginAndOpenDevice(page);

  for (const viewport of viewports) {
    await page.setViewportSize(viewport);

    await page.getByRole("tab", { name: "Клиенты" }).click();
    await expect(page.locator("#clientList .client-row")).toHaveCount(4);
    await expectNoOverflow(page, ".clients-panel");

    await page.getByRole("tab", { name: "Сеть" }).click();
    await expect(page.locator("#interfaceCounters .network-row")).toHaveCount(2);
    await expectNoOverflow(page, ".network-panel");

    await page.getByRole("tab", { name: "Обзор" }).click();
    await expect(page.locator("#alertList .problem-record")).toHaveCount(2);
    await expectNoOverflow(page, ".problems-panel");

    await page.getByRole("tab", { name: "Операции" }).click();
    await expect(page.locator("#commandList .command-row")).toHaveCount(4);
    await expect(page.locator("#remoteSessionList .remote-session-row")).toHaveCount(4);
    await expectNoOverflow(page, ".commands-panel");
    await expectNoOverflow(page, "#remoteAccessPanel");
  }

  await page.setViewportSize({ width: 768, height: 1024 });
  await page.getByRole("tab", { name: "Клиенты" }).click();
  await expect(page.locator(".client-table-head")).toBeHidden();
  await page.setViewportSize({ width: 900, height: 700 });
  await expect(page.locator(".client-table-head")).toBeVisible();
});

test("migrated feature interactions preserve filters, diagnostics, output and remote lifecycle", async ({ page }) => {
  await installFeatureFixtures(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await loginAndOpenDevice(page);

  await page.getByRole("tab", { name: "Клиенты" }).click();
  await page.locator("#clientSearch").fill("no-such-client-with-a-long-name");
  await expect(page.locator("#clientList")).toContainText("Клиенты не найдены");
  await page.locator("#clientSearch").fill("");
  await page.locator('[data-client-filter="unconfirmed"]').click();
  await expect(page.locator("#clientList .client-row")).toHaveCount(1);

  await page.getByRole("tab", { name: "Обзор" }).click();
  await page.locator("#alertStatusFilter").selectOption("resolved");
  await expect(page.locator("#alertList .problem-record")).toHaveCount(1);
  await expect(page.locator("#alertList")).toContainText("RESOLVED");

  await page.getByRole("tab", { name: "Операции" }).click();
  const diagnosticRequest = page.waitForRequest((request) => request.method() === "POST" && request.url().endsWith("/commands"));
  await page.locator('[data-diagnostic="ping_server"]').click();
  await diagnosticRequest;
  await expect(page.locator("#diagnosticStatus")).toContainText("Проверка поставлена в очередь");

  await page.locator("#commandList .command-row").nth(1).getByRole("button", { name: "Детали" }).click();
  await expect(page.locator("#commandDetailPanel")).toBeVisible();
  await expect(page.locator("#commandDetailOutput")).toContainText("2001:db8");
  await expectNoOverflow(page, ".command-detail-panel");

  await page.locator("#remoteAccessPanel details").click();
  const createRequest = page.waitForRequest((request) => request.method() === "POST" && request.url().endsWith("/remote-sessions"));
  await page.locator("#createRemoteSessionBtn").click();
  await expect(page.locator("#confirmationDialog")).toBeVisible();
  await page.locator("#confirmationConfirmBtn").click();
  await createRequest;
  await expect(page.locator("#remoteSessionList .remote-session-row")).toHaveCount(4);

  const closeRequest = page.waitForRequest((request) => request.method() === "POST" && request.url().endsWith("/remote-active/close"));
  await page.locator("#remoteSessionList .remote-session-row").filter({ hasText: "EXPIRING" }).getByRole("button", { name: "Закрыть" }).click();
  await expect(page.locator("#confirmationDialog")).toBeVisible();
  await page.locator("#confirmationConfirmBtn").click();
  await closeRequest;

  await page.route("**/api/devices/*/remote-sessions", (route) => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: "failed to create a deliberately long remote tunnel request because the backend provider is unavailable" }) }));
  await page.locator("#createRemoteSessionBtn").click();
  await expect(page.locator("#confirmationDialog")).toBeVisible();
  await page.locator("#confirmationConfirmBtn").click();
  await expect(page.locator("#cloudAccessState")).toHaveText("FAILED");
  await expectNoOverflow(page, "#remoteAccessPanel");
  if (await page.locator("#systemStateDialog").isVisible()) await page.locator("#closeSystemStateBtn").click();

  await expect(page.locator(".mobile-nav")).toBeVisible();
  await page.locator('[data-mobile-route="operations"]').click();
  await expect(page.getByRole("tab", { name: "Операции" })).toHaveAttribute("aria-selected", "true");
});
