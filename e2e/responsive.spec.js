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

async function login(page) {
  await page.goto("/login");
  await page.locator("#loginUsername").fill("e2e-admin");
  await page.locator("#loginPassword").fill("e2e-password-long-enough");
  await page.locator("#loginForm").press("Enter");
  await expect(page.locator("#appShell")).toBeVisible();
}

async function expectNoPageOverflow(page, viewSelector) {
  await expect.poll(() => page.evaluate((selector) => {
    const root = document.documentElement;
    const view = selector ? document.querySelector(selector) : null;
    return root.scrollWidth <= window.innerWidth && (!view || view.scrollWidth <= view.clientWidth);
  }, viewSelector)).toBe(true);
}

test("Fleet and Device Overview fit the complete responsive matrix", async ({ page }) => {
  await page.route("**/api/devices", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    const device = data.devices && data.devices[0];
    if (device) {
      device.online = false;
      device.active_alerts = 4;
      device.last_seen_at = new Date(Date.now() - 36 * 60 * 60 * 1000).toISOString();
      device.group = "Северо-западный производственный кластер";
      device.tags = ["резервный-канал-с-длинным-названием", "edge", "stale-data"];
      device.inventory = {
        ...(device.inventory || {}),
        hostname: "Маршрутизатор центрального офиса с очень длинным эксплуатационным именем",
        wan_ip: "2001:db8:85a3:0000:0000:8a2e:0370:7334",
      };
    }
    await route.fulfill({ response, json: data });
  });

  await login(page);
  const deviceButton = page.getByRole("button", { name: /Открыть роутер Маршрутизатор центрального офиса/ });
  await expect(deviceButton).toBeVisible();

  for (const viewport of viewports) {
    await page.setViewportSize(viewport);
    await expectNoPageOverflow(page, "#fleetView");
  }

  await page.setViewportSize({ width: 1440, height: 900 });
  await deviceButton.click();
  await expect(page.locator("#deviceView")).toBeVisible();
  await expect(page.locator("#deviceBadge")).toContainText("Не на связи");

  for (const viewport of viewports) {
    await page.setViewportSize(viewport);
    // Device tabs are intentionally a local horizontal scroll container.
    await expectNoPageOverflow(page, null);
    await expect(page.locator(".device-tabs")).toBeVisible();
  }

  await page.setViewportSize({ width: 320, height: 568 });
  await page.locator("#openLuciBtn").click();
  await expect(page.locator("#luciStateDialog")).toBeVisible();
  await expect.poll(() => page.locator("#luciStateDialog").evaluate(
    (dialog) => dialog.scrollWidth <= dialog.clientWidth,
  )).toBe(true);
});

test("reusable system states cover 404, access, backend, and generic errors", async ({ page, request }) => {
  const missing = await request.get("/route-that-does-not-exist");
  expect(missing.status()).toBe(404);
  expect(await missing.text()).toContain("Страница не найдена");

  await login(page);
  const cases = [
    { kind: "access", status: 403, title: "Недостаточно прав" },
    { kind: "backend", status: 0, title: "Сервер недоступен" },
    { kind: "generic", status: 500, title: "Не удалось выполнить запрос" },
  ];
  for (const item of cases) {
    await page.evaluate(({ kind, status }) => {
      const error = new Error("E2E system state");
      error.status = status;
      error.requestId = "req_e2e_system_state";
      showSystemState(error, kind);
    }, item);
    await expect(page.locator("#systemStateDialog")).toBeVisible();
    await expect(page.locator("#systemStateTitle")).toHaveText(item.title);
    await expect(page.locator("#systemStateContext")).toContainText("req_e2e_system_state");
    await page.locator("#closeSystemStateBtn").click();
  }
});
