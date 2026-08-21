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
const iso = (offset = 0) => new Date(now + offset).toISOString();
const longIPv6 = "2001:db8:1234:5678:90ab:cdef:1234:5678";
const longPackage = "luci-app-some-very-long-package-name-testing";

function command(id, type, status, args = {}, output = "") {
  return { id, type, status, args, output, attempt_count: 1, max_attempts: 3, created_at: iso(-60_000), completed_at: status === "completed" ? iso(-30_000) : null, expires_at: iso(15 * 60_000) };
}

async function installExpertFixtures(page) {
  const fixture = {
    commands: [
      command("cmd-preview", "uci_preview", "completed", { config: "wireless", section: "@wifi-iface[12]", option: "ssid", value: "production-router-accounting-floor-three" }, `PREVIEW\n- old-value\n+ production-router-accounting-floor-three\n${longIPv6}`),
      command("cmd-package", "pkg_list_installed", "completed", {}, `${longPackage} - 1.0.0-r1\n`.repeat(160)),
      command("cmd-ping", "ping", "completed", { target: longIPv6 }, `PING ${longIPv6}\n64 bytes response`),
      command("cmd-failed", "uci_commit", "failed", { config: "network" }, "network connection failed after a deliberately long backend response"),
    ],
    audit: Array.from({ length: 30 }, (_, index) => ({
      id: `audit-${index}`,
      actor: index % 3 === 0 ? "agent" : "e2e-admin-with-a-long-operational-identity",
      action: index % 2 === 0 ? "command.result" : "configuration.preview_with_a_deliberately_long_action_name",
      device_id: "fixture-device-with-a-long-identifier",
      command_id: `cmd-${index}-${"x".repeat(30)}`,
      details: { status: index % 7 === 0 ? "failed" : "completed", package: longPackage, address: longIPv6, request_id: `req_${"a".repeat(48)}` },
      created_at: iso(-index * 60_000),
    })),
    nextCommand: 1,
    failCommandType: "",
    failDelete: "",
  };

  await page.route("**/api/devices", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    const device = data.devices && data.devices[0];
    if (device) {
      device.online = true;
      device.last_seen_at = iso(-20_000);
      device.inventory = {
        ...(device.inventory || {}),
        hostname: "production-router-accounting-floor-three",
        wan_ip: longIPv6,
        wan_ipv6: longIPv6,
        agent_version: "0.6.10-production-build-with-long-version-metadata",
        package_manager: "apk",
        packages: Array.from({ length: 180 }, (_, index) => `${longPackage}-${index}`),
        board: { model: "OpenWrt infrastructure board with a deliberately long model designation", release: { version: "OpenWrt 25.12.4 r12345-long-release", target: "qualcommax/ipq807x-production-target" } },
        diagnostic_blob: { output: `${longIPv6} ${"technical-value-".repeat(24)}` },
      };
    }
    await route.fulfill({ response, json: data });
  });

  await page.route("**/api/devices/*/commands?*", (route) => {
    const url = new URL(route.request().url());
    const offset = Number(url.searchParams.get("offset") || 0);
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ commands: fixture.commands.slice(offset, offset + 25) }) });
  });
  await page.route("**/api/devices/*/commands", async (route) => {
    if (route.request().method() !== "POST") return route.fallback();
    const body = route.request().postDataJSON();
    if (fixture.failCommandType === body.type) {
      return route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: `backend rejected ${body.type}: ${"long technical failure ".repeat(10)}`, request_id: "req_expert_failure" }) });
    }
    const created = command(`cmd-created-${fixture.nextCommand++}`, body.type, "queued", body.args || {}, "");
    fixture.commands.unshift(created);
    return route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify(created) });
  });
  await page.route("**/api/audit-events?*", (route) => {
    const url = new URL(route.request().url());
    const offset = Number(url.searchParams.get("offset") || 0);
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ audit_events: fixture.audit.slice(offset, offset + 25) }) });
  });
  await page.route("**/api/devices/*/alerts", (route) => {
    if (route.request().method() !== "DELETE") return route.fallback();
    if (fixture.failDelete === "alerts") return route.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({ error: "database cleanup failed with a deliberately long technical response", request_id: "req_cleanup_failure" }) });
    return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
  });
  await page.route("**/api/devices/*/commands", (route) => route.fallback());
  return fixture;
}

async function loginAndOpenExpert(page) {
  await page.goto("/login");
  await page.locator("#loginUsername").fill("e2e-admin");
  await page.locator("#loginPassword").fill("e2e-password-long-enough");
  await page.locator("#loginForm").press("Enter");
  await expect(page.locator("#appShell")).toBeVisible();
  await page.getByRole("button", { name: /Открыть роутер production-router-accounting-floor-three/ }).click();
  await page.getByRole("tab", { name: "Эксперт" }).click();
  await expect(page.locator(".expert-notice")).toBeVisible();
}

async function expectNoOverflow(page, selector) {
  const result = await page.evaluate((target) => {
    const element = document.querySelector(target);
    const bounds = element?.getBoundingClientRect();
    const offenders = element && bounds
      ? [...element.querySelectorAll("*")]
        .filter((child) => {
          const childBounds = child.getBoundingClientRect();
          return childBounds.right > bounds.right + 1 || childBounds.left < bounds.left - 1;
        })
        .slice(0, 8)
        .map((child) => {
          const id = child.id ? `#${child.id}` : "";
          const classes = child.className && typeof child.className === "string"
            ? `.${child.className.trim().replace(/\s+/g, ".")}`
            : "";
          return `${child.tagName.toLowerCase()}${id}${classes}`;
        })
      : [];
    return {
      page: document.documentElement.scrollWidth - window.innerWidth,
      local: element ? element.scrollWidth - element.clientWidth : 9999,
      offenders,
    };
  }, selector);
  expect(result.page, `${selector} creates page overflow`).toBeLessThanOrEqual(0);
  expect(result.local, `${selector} creates local overflow: ${result.offenders.join(", ")}`).toBeLessThanOrEqual(0);
}

test("Expert infrastructure layer fits the complete responsive matrix", async ({ page }) => {
  await installExpertFixtures(page);
  await loginAndOpenExpert(page);
  await expect(page.locator("#inventorySummary")).toContainText("production-router-accounting-floor-three");
  await expect(page.locator("#auditList .audit-record")).toHaveCount(25);

  for (const viewport of viewports) {
    await page.setViewportSize(viewport);
    for (const selector of [".expert-notice", ".inventory-panel", ".manual-command-panel", ".uci-panel", ".package-panel", ".audit-panel", ".maintenance-panel", ".transfer-panel"]) {
      await expectNoOverflow(page, selector);
    }
    await page.evaluate(() => {
      void confirmAction({
        variant: "danger",
        context: "DANGER / STRESS TEST",
        title: "Удалить infrastructure object with a deliberately long confirmation title that must wrap safely?",
        message: "A deliberately long consequence message must remain inside the dialog at every supported viewport without creating page-level horizontal overflow.",
        description: "The operation is irreversible and can interrupt the connection.",
        values: [["IPv6", "2001:db8:1234:5678:90ab:cdef:1234:5678"], ["Package", "luci-app-some-very-long-package-name-testing"]],
        confirmLabel: "Delete infrastructure object",
      });
    });
    await expect(page.locator("#confirmationDialog")).toBeVisible();
    await expectNoOverflow(page, "#confirmationDialog");
    await page.locator("#confirmationCancelBtn").click();
  }
});

test("Expert workflows and confirmation lifecycle remain functional", async ({ page }) => {
  const fixture = await installExpertFixtures(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await loginAndOpenExpert(page);

  await page.locator("#commandType").selectOption("reboot");
  await page.locator("#sendCommandBtn").click();
  await expect(page.locator("#confirmationDialog")).toHaveAttribute("data-variant", "danger");
  await expect(page.locator("#confirmationTitle")).toContainText("Перезагрузить");
  await page.keyboard.press("Escape");
  await expect(page.locator("#confirmationDialog")).toBeHidden();
  await expect(page.locator("#sendCommandBtn")).toBeFocused();

  await page.locator("#sendCommandBtn").click();
  await page.locator("#confirmationCancelBtn").click();
  await expect(page.locator("#sendCommandBtn")).toBeFocused();

  await page.locator("#uciConfig").selectOption("wireless");
  await page.locator("#uciSection").fill("@wifi-iface[12]");
  await page.locator("#uciOption").fill("ssid");
  await page.locator("#uciValue").fill("production-router-accounting-floor-three");
  const previewRequest = page.waitForRequest((request) => request.method() === "POST" && request.url().endsWith("/commands") && request.postDataJSON().type === "uci_preview");
  await page.locator("#uciPreviewBtn").click();
  await previewRequest;
  await expect(page.locator("#uciDiff")).toContainText("@wifi-iface[12]");
  await expect(page.locator("#uciDiff")).toContainText("production-router-accounting-floor-three");

  fixture.failCommandType = "uci_commit";
  await page.locator(".recovery-workflow > summary").click();
  await page.locator("#uciCommitBtn").click();
  await expect(page.locator("#confirmationContext")).toHaveText("CONFIGURATION / UCI");
  await page.locator("#confirmationConfirmBtn").click();
  await expect(page.locator("#confirmationError")).toContainText("Сервер временно");
  await expect(page.locator("#confirmationTechnicalOutput")).toContainText("backend rejected uci_commit");
  await expect(page.locator("#confirmationDialog")).toBeVisible();
  fixture.failCommandType = "";
  await page.locator("#confirmationConfirmBtn").click();
  await expect(page.locator("#confirmationDialog")).toBeHidden();

  await page.locator("#packageCommand").selectOption("pkg_remove");
  await page.locator("#packageName").fill(longPackage);
  await page.locator("#sendPackageCommandBtn").click();
  await expect(page.locator("#confirmationDialog")).toHaveAttribute("data-variant", "danger");
  await expect(page.locator("#confirmationValues")).toContainText(longPackage);
  await page.locator("#confirmationConfirmBtn").click();
  await expect(page.locator("#packageFormMessage")).toContainText("очередь");

  await page.locator("#loadMoreAuditBtn").click();
  await expect(page.locator("#auditList .audit-record")).toHaveCount(30);
  await expect(page.locator("#auditSummary")).toHaveText("30 events");

  fixture.failDelete = "alerts";
  await page.locator("#clearAlertsBtn").click();
  await page.locator("#confirmationInput").fill("WRONG");
  await page.locator("#confirmationConfirmBtn").click();
  await expect(page.locator("#confirmationError")).toContainText("CLEAR");
  await page.locator("#confirmationInput").fill("CLEAR");
  await page.locator("#confirmationConfirmBtn").click();
  await expect(page.locator("#confirmationError")).toContainText("Сервер временно");
  await expect(page.locator("#confirmationConfirmBtn")).toBeEnabled();
  fixture.failDelete = "";
  await page.locator("#confirmationCancelBtn").click();

  await page.locator("#transferUsername").fill("new-owner-account");
  await page.locator("#transferPassword").fill("e2e-password-long-enough");
  await page.locator("#deviceTransferForm").getByRole("button", { name: "Проверить и передать" }).click();
  await expect(page.locator("#confirmationTitle")).toContainText("Передать");
  await expect(page.locator("#confirmationValues")).toContainText("new-owner-account");
  await page.locator("#confirmationCancelBtn").click();

  await page.locator("#deleteDeviceBtn").click();
  await expect(page.locator("#confirmationContext")).toHaveText("DANGER / DEVICE");
  await expect(page.locator("#confirmationInputGroup")).toBeVisible();
  await page.locator("#confirmationCancelBtn").click();
  for (const selector of [".manual-command-panel", ".uci-panel", ".package-panel", ".audit-panel", ".maintenance-panel", ".transfer-panel"]) {
    await expectNoOverflow(page, selector);
  }
});

test("native browser dialogs are absent from frontend source", async ({ page }) => {
  const response = await page.request.get("/app.js");
  const source = await response.text();
  expect(source).not.toMatch(/\bwindow\.(?:confirm|alert|prompt)\s*\(/);
  expect(source).not.toMatch(/(^|[^\w.])(?:confirm|alert|prompt)\s*\(/m);
});
