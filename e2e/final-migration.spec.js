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

async function expectNoPageOverflow(page, selector = null) {
  await expect.poll(() => page.evaluate((targetSelector) => {
    const target = targetSelector ? document.querySelector(targetSelector) : null;
    return document.documentElement.scrollWidth <= window.innerWidth
      && (!target || target.scrollWidth <= target.clientWidth);
  }, selector)).toBe(true);
}

async function login(page) {
  await page.goto("/login");
  await page.locator("#loginUsername").fill("e2e-admin");
  await page.locator("#loginPassword").fill("e2e-password-long-enough");
  await page.locator("#loginForm").press("Enter");
  await expect(page.locator("#appShell")).toBeVisible();
}

test("Landing, Login, Legal and the real 404 state fit the supported viewport matrix", async ({ page }) => {
  for (const path of ["/", "/login", "/legal.html", "/route-that-does-not-exist"]) {
    await page.goto(path);
    for (const viewport of viewports) {
      await page.setViewportSize(viewport);
      await expectNoPageOverflow(page);
    }
  }
});

test("Login reports invalid credentials and network failure inline", async ({ page }) => {
  await page.goto("/login");
  await page.locator("#loginUsername").fill("missing-user");
  await page.locator("#loginPassword").fill("definitely-invalid-password");
  await page.locator("#loginForm").press("Enter");
  await expect(page.locator("#loginError")).toContainText("Неверный логин или пароль");
  await expect(page.locator("#loginUsername")).toHaveAttribute("aria-invalid", "true");

  await page.route("**/api/auth/login", (route) => route.abort("failed"));
  await page.locator("#loginPassword").fill("definitely-invalid-password");
  await page.locator("#loginForm").press("Enter");
  await expect(page.locator("#loginError")).toContainText("Нет соединения с OpenWrt RMM");
  await expect(page.locator("#loginSubmitBtn")).toBeEnabled();
});

test("Profile, Security, Notifications and Admin use compact responsive settings patterns", async ({ page }) => {
  const longValue = "production-router-accounting-floor-three-with-a-deliberately-long-technical-context";
  await page.route("**/api/auth/profile", (route) => route.fulfill({ json: { user: {
    id: "e2e-admin",
    username: "e2e-admin",
    display_name: `Operator ${longValue}`,
    email: `${longValue}@example.infrastructure`,
    role: "admin",
  } } }));
  let passwordAttempt = 0;
  await page.route("**/api/auth/change-password", (route) => {
    passwordAttempt += 1;
    if (passwordAttempt === 1) {
      route.fulfill({ status: 400, json: { error: "invalid current password" } });
      return;
    }
    route.fulfill({ json: { ok: true } });
  });
  await page.route("**/api/notifications/settings", async (route) => {
    if (route.request().method() === "PUT") {
      await route.fulfill({ json: { settings: { configured: true }, channels: {} } });
      return;
    }
    await route.fulfill({ json: {
      settings: {
        configured: true,
        email_enabled: true,
        telegram_enabled: true,
        telegram_chat_id: "123456789",
        notify_warning: true,
        notify_critical: true,
        notify_resolved: true,
        memory_threshold_percent: 85,
        disk_threshold_percent: 90,
        packet_loss_percent: 20,
        latency_threshold_ms: 500,
        timezone: "Europe/Moscow-with-a-deliberately-long-but-wrappable-name",
      },
      channels: {
        email: { available: true, profile_email_configured: true, verified: false, destination: "very.long.operator.email.address@example.infrastructure" },
        telegram: { available: true, verified: true, status: "verified", message: "Канал готов" },
        webhook: { available: true, status: "available", message: "Endpoint configured" },
      },
    } });
  });
  await page.route("**/api/notifications?*", (route) => route.fulfill({ json: {
    notifications: [{
      id: "delivery-1",
      device_id: "missing-device",
      channel: "webhook",
      title: `Long notification ${longValue}`,
      destination: `https://example.invalid/hooks/${longValue}`,
      status: "dead_letter",
      severity: "critical",
      event: "active",
      error: `Delivery failed: ${longValue.repeat(3)}`,
      created_at: new Date().toISOString(),
    }],
    metrics: { queued: 4, sent: 21, failed: 1, dead_letter: 1, channels: [] },
  } }));
  await page.route("**/api/users", async (route) => {
    if (route.request().method() === "POST") {
      await route.fulfill({ status: 409, json: { error: "username already exists" } });
      return;
    }
    await route.fulfill({ json: { users: [{
      id: "managed-user-long",
      username: `operator-${longValue}`,
      display_name: `Infrastructure operator ${longValue}`,
      email: `${longValue}@example.infrastructure`,
      role: "user",
      disabled: false,
    }] } });
  });

  await login(page);
  await page.locator("#profileBtn").click();
  await expect(page.locator("#profileDialog")).toBeVisible();
  await page.locator("#profileDisplayName").fill(`Operator ${longValue}`);
  await page.locator("#profileEmail").fill(`${longValue}@example.infrastructure`);
  await page.locator("#profileSubmitBtn").click();
  await expect(page.locator("#profileMessage")).toHaveText("Профиль сохранён");
  await page.locator('[data-profile-tab="notifications"]').click();
  await expect(page.locator("#notificationEmailState")).toHaveText("PENDING VERIFICATION");
  await expect(page.locator("#notificationTelegramState")).toHaveText("VERIFIED");
  await expect(page.locator(".notification-history-row")).toHaveCount(1);
  await page.locator("#notificationQuietEnabled").check();
  await page.locator("#notificationSubmitBtn").click();
  await expect(page.locator("#notificationSettingsMessage")).toHaveText("Настройки уведомлений сохранены");

  for (const viewport of viewports) {
    await page.setViewportSize(viewport);
    await expectNoPageOverflow(page);
    await expectNoPageOverflow(page, "#profileDialog");
  }

  await page.setViewportSize({ width: 390, height: 844 });
  await page.locator('[data-profile-tab="security"]').click();
  await expect(page.locator("#passwordForm")).toBeVisible();
  await page.locator("#currentPassword").fill("wrong-current-password");
  await page.locator("#newPassword").fill("replacement-password-long-enough");
  await page.locator("#confirmPassword").fill("replacement-password-long-enough");
  await page.locator("#passwordSubmitBtn").click();
  await expect(page.locator("#passwordMessage")).toContainText("Текущий пароль указан неверно");
  await page.locator("#currentPassword").fill("correct-current-password");
  await page.locator("#passwordSubmitBtn").click();
  await expect(page.locator("#passwordMessage")).toContainText("Пароль изменён");
  await page.locator('[data-profile-tab="admin"]').click();
  await expect(page.locator(".user-row")).toHaveCount(1);
  await expectNoPageOverflow(page, "#profileDialog");
  await page.locator("[data-user-role]").selectOption("admin");
  await expect(page.locator("#confirmationDialog")).toBeVisible();
  await expect(page.locator("#confirmationTitle")).toContainText("Изменить роль пользователя");
  await page.locator("#confirmationCancelBtn").click();

  await page.locator("#openCreateUserBtn").click();
  await page.locator("#newUsername").fill("existing-user");
  await page.locator("#newUserEmail").fill("existing@example.com");
  await page.locator("#newUserPassword").fill("temporary-password-long-enough");
  await page.locator("#createUserForm").press("Enter");
  await expect(page.locator("#createUserMessage")).toContainText("username already exists");
  await expect(page.locator("#createUserDialog")).toBeVisible();
  await expectNoPageOverflow(page, "#createUserDialog");
});

test("Enrollment copy feedback and icon accessibility use the shared local registry", async ({ page, context, request }) => {
  const sprite = await request.get("/assets/icons/tabler-sprite.svg");
  expect(sprite.ok()).toBeTruthy();
  const spriteText = await sprite.text();
  expect((spriteText.match(/<symbol id="icon-/g) || []).length).toBe(33);
  expect(spriteText).toContain("Tabler Icons v3.46.0 (8ac7d81)");
  expect((await request.get("/licenses/Tabler-Icons-MIT.txt")).ok()).toBeTruthy();

  await context.grantPermissions(["clipboard-read", "clipboard-write"], { origin: "http://127.0.0.1:18081" });
  await login(page);
  await page.evaluate(() => {
    document.querySelector("#enrollmentTokenOutput").value = "enroll_0123456789abcdefghijklmnopqrstuvwxyz";
    document.querySelector("#enrollmentGrantDialog").showModal();
  });
  await page.locator("#copyEnrollmentTokenBtn").click();
  await expect(page.locator("#enrollmentCopyState")).toHaveText("COPIED");
  await expect(page.evaluate(() => navigator.clipboard.readText())).resolves.toContain("enroll_0123456789");

  const accessibilityIssues = await page.evaluate(() => {
    const iconOnlyWithoutName = [...document.querySelectorAll("button")].filter((button) => {
      if (!button.querySelector("svg")) return false;
      if (button.textContent.trim()) return false;
      return !(button.getAttribute("aria-label") || button.getAttribute("title"));
    }).map((button) => button.id || button.outerHTML.slice(0, 80));
    const unmanagedSVG = [...document.querySelectorAll("svg")].filter((svg) => {
      const decorative = svg.getAttribute("aria-hidden") === "true" || svg.closest('[aria-hidden="true"]');
      return !decorative && svg.getAttribute("role") !== "img";
    }).map((svg) => svg.outerHTML.slice(0, 80));
    return { iconOnlyWithoutName, unmanagedSVG };
  });
  expect(accessibilityIssues).toEqual({ iconOnlyWithoutName: [], unmanagedSVG: [] });
});

test("production source has no native dialogs, emoji UI icons or Unicode pseudo-icons", async ({ request }) => {
  const source = (await Promise.all([
    "/index.html", "/landing.html", "/legal.html", "/error.html", "/app.js", "/styles.css",
  ].map(async (path) => (await request.get(path)).text()))).join("\n");
  expect(source.match(/(?:window\.)?(?:alert|confirm|prompt)\s*\(/g) || []).toHaveLength(0);
  expect(source.match(/[✓×🔔☁◫⌁○↻↪]/g) || []).toHaveLength(0);
  expect(source.match(/[\u{1F300}-\u{1FAFF}]/gu) || []).toHaveLength(0);
});
