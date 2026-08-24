const { test, expect } = require("@playwright/test");

test.describe("authenticated operator flows", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/login");
    await page.locator("#loginUsername").fill("e2e-admin");
    await page.locator("#loginPassword").fill("e2e-password-long-enough");
    await page.locator("#loginForm").press("Enter");
    await expect(page.locator("#appShell")).toBeVisible();
  });

  test("login, fleet, router details, and LuCI unavailable state", async ({ page }) => {
  await expect(page.locator("#operatorName")).toHaveText("e2e-admin");
  await expect(page.locator("#serverVersion")).toHaveText(" · сервер dev");
  await expect(page.locator("#fleetTotalCount")).toHaveText("1");
  await page.getByRole("button", { name: "Открыть роутер E2E OpenWrt" }).click();
  await expect(page.locator("#deviceView")).toBeVisible();
  await expect(page.locator("#deviceName")).toHaveText("E2E OpenWrt");
  await expect(page.locator("#infoHostname")).toHaveText("E2E OpenWrt");
  await page.locator("#openLuciBtn").click();
  await expect(page.locator("#luciStateDialog")).toBeVisible();
  await expect(page.locator("#luciStateTitle")).toContainText("доступ");
  });

  test("profile tabs expose notification diagnostics without sending messages", async ({ page }) => {
  await page.locator("#profileBtn").click();
  await expect(page.locator("#profileDialog")).toBeVisible();
  await page.getByRole("tab", { name: "Уведомления" }).click();
  await expect(page.locator("#notificationSettingsForm")).toBeVisible();
  await expect(page.locator("#notificationMetrics")).toBeVisible();
  await expect(page.locator("#notificationChannelDiagnostics")).toContainText("SMTP");
    await expect(page.locator("#notificationHistory")).toContainText("Отправок пока нет");
  });

  test("profile tabs are keyboard navigable", async ({ page }) => {
  await page.locator("#profileBtn").click();
  const accountTab = page.getByRole("tab", { name: "Профиль" });
  await accountTab.focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("tab", { name: "Безопасность" })).toBeFocused();
  await expect(page.locator("#passwordForm")).toBeVisible();
  });

  test("notification center keeps long entries separated and scrollable", async ({ page }) => {
    await page.setViewportSize({ width: 528, height: 760 });
    const notifications = Array.from({ length: 50 }, (_, index) => ({
      id: `notification-${index}`,
      incident_id: `incident-${index}`,
      device_id: null,
      severity: index % 4 === 0 ? "critical" : "warning",
      title: `Восстановлено: E2E OpenWrt ${index + 1}`,
      body: "Проверка доступности завершена. Роутер снова отвечает, связанные события сгруппированы без наложения текста.",
      created_at: new Date(Date.now() - index * 60_000).toISOString(),
      read_at: null,
    }));
    await page.route("**/api/notification-center?limit=50", (route) => route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ notifications, unread: 360 }),
    }));

    await page.locator('[data-mobile-route="notifications"]').click();
    await expect(page.locator("#notificationCenterDialog")).toBeVisible();
    await expect(page.locator("#notificationCenterSummary")).toHaveText("360 непрочитанных");
    await expect(page.locator(".notification-center-item")).toHaveCount(50);
    await expect.poll(() => page.locator("#notificationCenterDialog").evaluate(
      (dialog) => dialog.scrollWidth <= dialog.clientWidth,
    )).toBe(true);
    await expect.poll(() => page.locator(".notification-center-item").evaluateAll((items) => items.every((item, index) => {
      if (index === items.length - 1) return true;
      return item.getBoundingClientRect().bottom <= items[index + 1].getBoundingClientRect().top;
    }))).toBe(true);
    await expect.poll(() => page.locator("#notificationCenterList").evaluate(
      (list) => list.scrollHeight > list.clientHeight,
    )).toBe(true);
  });

  test("admin can review a compatible agent rollback", async ({ page }) => {
    await page.getByRole("button", { name: "Открыть роутер E2E OpenWrt" }).click();
    await expect(page.locator("#rollbackAgentBtn")).toBeVisible();
    await page.locator("#rollbackAgentBtn").click();
    await expect(page.locator("#agentRollbackDialog")).toBeVisible();
    await expect(page.locator("#agentRollbackPreview")).toContainText("0.8.0");

    await page.locator("#agentRollbackVersion").fill("0.8.0");
    await page.locator("#agentRollbackForm").press("Enter");
    await expect(page.locator("#agentRollbackMessage")).toContainText("ниже установленной");

    await page.locator("#agentRollbackVersion").fill("0.7.9");
    await expect.poll(() => page.locator("#agentRollbackVersion").evaluate((input) => input.checkValidity())).toBe(true);
    await expect.poll(() => page.locator("#agentRollbackDialog").evaluate((dialog) => dialog.scrollWidth <= dialog.clientWidth)).toBe(true);
  });

  test("agent update waits for a reconnect confirmation", async ({ page }) => {
    await page.route("**/api/devices/*/commands?*", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          commands: [{
            id: "cmd_waiting_reconnect",
            type: "agent_update",
            status: "completed",
            args: { target_version: "0.6.10" },
            result: { health_status: "waiting_reconnect", installed_package_version: "0.6.10-r1" },
            attempt_count: 1,
            max_attempts: 3,
            created_at: new Date().toISOString(),
          }],
        }),
      });
    });
    await page.getByRole("button", { name: "Открыть роутер E2E OpenWrt" }).click();
    await expect(page.locator("#agentUpdateStatus")).toBeVisible();
    await expect(page.locator("#agentUpdateStatus")).toContainText("ожидание повторного подключения");
    await expect(page.locator("#agentUpdateStatus")).toContainText("сервер ждёт heartbeat");
  });

  for (const [name, viewport] of Object.entries({
    fullHD: { width: 1920, height: 1080 },
    laptop16x9: { width: 1366, height: 768 },
  })) {
    test(`network clients keep a compact aligned layout at ${name}`, async ({ page }) => {
      await page.setViewportSize(viewport);
      await page.getByRole("button", { name: "Открыть роутер E2E OpenWrt" }).click();
      await page.getByRole("tab", { name: "Клиенты" }).click();
      await expect(page.locator("#clientList .client-row")).toHaveCount(16);
      await expect(page.locator("#clientList .client-online-label")).toHaveCount(16);
      await expect.poll(() => page.locator("#clientList").evaluate(
        (list) => list.scrollWidth - list.clientWidth,
      )).toBeLessThanOrEqual(0);
      await expect.poll(() => page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      )).toBe(true);
      const statusLayout = await page.locator("#clientList .client-online").evaluateAll((statuses) => statuses.map((status) => {
        const dot = status.querySelector("i").getBoundingClientRect();
        const bounds = status.getBoundingClientRect();
        const lineHeight = parseFloat(getComputedStyle(status).lineHeight);
        return {
          delta: Math.abs((dot.top + dot.height / 2) - (bounds.top + bounds.height / 2)),
          wraps: bounds.height > lineHeight * 1.5,
        };
      }));
      expect(statusLayout.every(({ delta }) => delta <= 2)).toBe(true);
      expect(statusLayout.every(({ wraps }) => !wraps)).toBe(true);
      if (viewport.width >= 1280) {
        await expect(page.locator(".client-table-head")).toBeVisible();
        const rowHeight = await page.locator("#clientList .client-row").first().evaluate((row) => row.getBoundingClientRect().height);
        expect(rowHeight).toBeLessThan(90);
      }
    });
  }
});

for (const [name, viewport] of Object.entries({
  desktop: { width: 1920, height: 1080 },
  laptop: { width: 1366, height: 768 },
  tablet: { width: 1024, height: 768 },
  portrait: { width: 768, height: 1024 },
  mobile: { width: 390, height: 844 },
  compactMobile: { width: 360, height: 800 },
})) {
  test(`landing page remains usable at ${name}`, async ({ page }) => {
    await page.setViewportSize(viewport);
    await page.goto("/");
    await expect(page.getByRole("heading", { name: "Единый центр управления OpenWrt" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Открыть кабинет" })).toBeVisible();
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  });
}

test("landing remains operable at 200% zoom", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto("/");
  await page.locator("body").evaluate((body) => { body.style.zoom = "200%"; });
  await expect(page.getByRole("link", { name: "Открыть кабинет" })).toBeVisible();
  await page.getByRole("link", { name: "Открыть кабинет" }).focus();
  await expect(page.getByRole("link", { name: "Открыть кабинет" })).toBeFocused();
});
