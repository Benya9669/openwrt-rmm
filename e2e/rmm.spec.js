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
