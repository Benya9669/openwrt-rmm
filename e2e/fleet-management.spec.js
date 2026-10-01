const { test, expect } = require("@playwright/test");

const tabs = ["Массовые операции", "Профили UCI", "Политики доступа", "Сравнение копий", "Расписания", "История состояния", "Диагностика", "Автоматические реакции", "Оборудование", "Роли и права", "Центр инцидентов", "Обновления волнами", "Карта сети"];

test.beforeEach(async ({ page }) => {
  await page.goto("/login");
  await page.locator("#loginUsername").fill("e2e-admin");
  await page.locator("#loginPassword").fill("e2e-password-long-enough");
  await page.locator("#loginForm").press("Enter");
  await expect(page.locator("#appShell")).toBeVisible();
  await page.locator("[data-fleet-management-open]:visible").first().click();
  await expect(page.locator("#fleetManagementView")).toBeVisible();
});

for (const width of [320, 390, 1440]) {
  test(`all fleet features stay usable at ${width}px`, async ({ page }) => {
    const errors = []; page.on("pageerror", (error) => errors.push(error.message));
    await page.setViewportSize({ width, height: 900 });
    const view = page.locator("#fleetManagementView");
    for (const name of tabs) {
      await view.getByRole("button", { name, exact: true }).click();
      await expect(view.locator("[data-fleet-message]")).not.toHaveText("Выполняется…");
      await expect(view.locator("[data-fleet-content] h3").first()).toBeVisible();
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)).toBeLessThanOrEqual(1);
    }
    expect(errors).toEqual([]);
    await page.screenshot({ path: `test-results/fleet-management-${width}.png`, fullPage: true });
    await view.getByRole("button", { name: "К объектам", exact: true }).click();
    await expect(view).toBeHidden();
    await expect(page.locator("#fleetView")).toBeVisible();
  });
}

test("operation refresh preserves expanded targets and command output", async ({ page }) => {
  let refreshes = 0;
  await page.route("**/api/fleet/operations", async (route) => {
    refreshes++;
    await route.fulfill({ json: { operations: [{ id: "synthetic-operation", title: "Refresh test", type: "ping", status: "completed", parallelism: 1, created_at: "2026-09-30T12:00:00Z", items: [{ device_id: "synthetic-device", command_id: "synthetic-command", status: "completed" }] }] } });
  });
  await page.route("**/api/devices/synthetic-device/commands/synthetic-command", (route) => route.fulfill({ json: { output: "Synthetic diagnostic result" } }));
  const view = page.locator("#fleetManagementView");
  await view.getByRole("button", { name: "Массовые операции", exact: true }).click();
  await expect(view.locator("[data-fleet-message]")).toHaveText("Готово");
  const record = view.locator('[data-operation-record="synthetic-operation"]');
  await record.locator("summary").click();
  await record.getByRole("button", { name: "Результат", exact: true }).click();
  await expect(record.locator("[data-operation-output]")).toHaveText("Synthetic diagnostic result");
  // Move focus outside the record so the background refresh can run.
  await view.getByRole("button", { name: "Массовые операции", exact: true }).focus();
  await expect.poll(() => refreshes, { timeout: 15000 }).toBeGreaterThanOrEqual(2);
  await expect(record.locator("details")).toHaveAttribute("open", "");
  await expect(record.locator("[data-operation-output]")).toBeVisible();
  await expect(record.locator("[data-operation-output]")).toHaveText("Synthetic diagnostic result");
});

test("fleet forms persist assets, disabled schedules, profiles and rules", async ({ page }) => {
  const view = page.locator("#fleetManagementView");
  await view.getByRole("button", { name: "Оборудование", exact: true }).click();
  await expect(view.locator("[data-fleet-message]")).toHaveText("Готово");
  await view.locator('[name="model"]').fill("Fleet Test Model");
  await view.locator('[name="site"]').fill("Лаборатория");
  await view.locator('[name="serial_number"]').fill("SYNTHETIC-FLEET-001");
  await view.getByRole("button", { name: "Сохранить карточку" }).click();
  await expect(view.locator("[data-fleet-assets]")).toContainText("Fleet Test Model");
  await view.getByRole("button", { name: "Расписания", exact: true }).click();
  await expect(view.locator("[data-fleet-message]")).toHaveText("Готово");
  await view.locator('[data-schedule-form] [name="title"]').fill("Fleet test backup");
  await view.getByRole("button", { name: "Сохранить расписание" }).click();
  await expect(view.locator("[data-schedules]")).toContainText("Fleet test backup");
  await expect(view.locator("[data-schedules]")).toContainText("Отключено");
  await view.getByRole("button", { name: "Профили UCI", exact: true }).click();
  await expect(view.locator("[data-fleet-message]")).toHaveText("Готово");
  await view.locator('[data-profile-form] [name="title"]').fill("Fleet test system");
  await view.locator('[data-profile-form] [name="options"]').fill("core.hostname=fleet-lab");
  await view.getByRole("button", { name: "Сохранить профиль", exact: true }).click();
  await expect(view.locator("[data-profiles]")).toContainText("core.hostname=fleet-lab");
  await view.getByRole("button", { name: "Автоматические реакции", exact: true }).click();
  await expect(view.locator("[data-fleet-message]")).toHaveText("Готово");
  await view.locator('[data-rule-form] [name="title"]').fill("Fleet test notify");
  await view.getByRole("button", { name: "Сохранить правило", exact: true }).click();
  await expect(view.locator("[data-rules]")).toContainText("Fleet test notify");
  await expect(view.locator("[data-rules]")).toContainText("Отключено");
});

test("permission presets save through the administrator confirmation", async ({ page }) => {
  const username = `fleet-role-${Date.now()}`;
  const created = await page.request.post("/api/users", { data: { username, display_name: "Synthetic role operator", password: "synthetic-e2e-password-long", role: "user" } });
  expect(created.ok()).toBeTruthy();
  const user = await created.json();
  const view = page.locator("#fleetManagementView");
  await view.getByRole("button", { name: "Роли и права", exact: true }).click();
  await expect(view.locator("[data-fleet-message]")).not.toHaveText("Выполняется…");
  const form = view.locator("[data-permission-form]");
  await form.locator('[name="user"]').selectOption(user.user?.id || user.id);
  await expect(view.locator("[data-fleet-message]")).not.toHaveText("Выполняется…");
  await form.locator('[name="preset"]').selectOption("operator");
  await form.locator('[name="groups"]').fill("Лаборатория");
  await form.getByRole("button", { name: "Сохранить права", exact: true }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Сохранить права", exact: true }).click();
  await expect(form.locator("[data-permission-saved]")).toContainText(username);
  const saved = await (await page.request.get(`/api/fleet/permissions/${user.user?.id || user.id}`)).json();
  expect(saved.permissions).toEqual(["view", "diagnostics", "backups", "incidents"]);
  expect(saved.groups).toEqual(["Лаборатория"]);
});

test("incident comments and explicit canary waves are submitted", async ({ page }) => {
  const deviceID = await page.locator("[data-device-id]").first().getAttribute("data-device-id");
  const incident = { id: "synthetic-incident", device_id: deviceID, category: "network", title: "Synthetic uplink incident", status: "open", occurrences: 1, assignee_id: "", opened_at: "2026-09-30T12:00:00Z", events: [] };
  await page.route("**/api/fleet/incidents", (route) => route.fulfill({ json: { incidents: [incident] } }));
  await page.route("**/api/fleet/incidents/synthetic-incident", async (route) => {
    const input = route.request().postDataJSON();
    expect(input.action).toBe("comment");
    incident.events.push({ actor: "Synthetic operator", action: "comment", body: input.body, created_at: "2026-09-30T12:01:00Z" });
    await route.fulfill({ json: { status: "updated" } });
  });
  const view = page.locator("#fleetManagementView");
  await view.getByRole("button", { name: "Центр инцидентов", exact: true }).click();
  await expect(view.locator("[data-incidents]")).toContainText(incident.title);
  await view.locator('[data-incident-comment] [name="body"]').fill("Synthetic comment from UI");
  await view.getByRole("button", { name: "Добавить комментарий", exact: true }).click();
  await view.locator("[data-incidents] summary").click();
  await expect(view.locator("[data-incidents]")).toContainText("Synthetic comment from UI");
  let submission = null;
  await page.route("**/api/agent-rollouts", async (route) => {
    if (route.request().method() === "POST") { submission = route.request().postDataJSON(); await route.fulfill({ json: { id: "synthetic-rollout" } }); }
    else await route.fulfill({ json: { rollouts: [], candidate_available: true } });
  });
  await view.getByRole("button", { name: "Обновления волнами", exact: true }).click();
  await expect(view.locator("[data-fleet-message]")).not.toHaveText("Выполняется…");
  const form = view.locator("[data-wave-form]");
  await form.locator('[name="devices"]').selectOption(deviceID);
  await form.locator('[name="canaries"]').selectOption(deviceID);
  await form.getByRole("button", { name: "Создать обновление волнами" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Начать обновление", exact: true }).click();
  await expect.poll(() => submission).not.toBeNull();
  expect(submission.guard).toEqual({ canary_ids: [deviceID], wave_sizes: [3, 10, 25], observation_seconds: 120 });
  expect(submission.request_key).toBeTruthy();
});
