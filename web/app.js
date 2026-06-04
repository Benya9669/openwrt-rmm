const state = {
  username: "",
  devices: [],
  selectedDeviceId: null,
  deviceTab: "overview",
  filter: "all",
  clientFilter: "all",
  commandFilter: "all",
  commands: [],
  alerts: [],
  remoteSessions: [],
  selectedCommand: null,
};

const els = {
  loginView: document.querySelector("#loginView"),
  loginForm: document.querySelector("#loginForm"),
  loginUsername: document.querySelector("#loginUsername"),
  loginPassword: document.querySelector("#loginPassword"),
  loginError: document.querySelector("#loginError"),
  appShell: document.querySelector("#appShell"),
  operatorName: document.querySelector("#operatorName"),
  logoutBtn: document.querySelector("#logoutBtn"),
  apiState: document.querySelector("#apiState"),
  refreshBtn: document.querySelector("#refreshBtn"),
  deviceList: document.querySelector("#deviceList"),
  fleetView: document.querySelector("#fleetView"),
  fleetTotalCount: document.querySelector("#fleetTotalCount"),
  fleetOnlineCount: document.querySelector("#fleetOnlineCount"),
  fleetOfflineCount: document.querySelector("#fleetOfflineCount"),
  fleetAlertCount: document.querySelector("#fleetAlertCount"),
  navAlertCount: document.querySelector("#navAlertCount"),
  statusLine: document.querySelector("#statusLine"),
  pageTitle: document.querySelector("#pageTitle"),
  emptyState: document.querySelector("#emptyState"),
  deviceView: document.querySelector("#deviceView"),
  backToFleetBtn: document.querySelector("#backToFleetBtn"),
  quickDiagnosticBtn: document.querySelector("#quickDiagnosticBtn"),
  openLuciBtn: document.querySelector("#openLuciBtn"),
  remoteAccessPanel: document.querySelector("#remoteAccessPanel"),
  runFullDiagnosticBtn: document.querySelector("#runFullDiagnosticBtn"),
  diagnosticStatus: document.querySelector("#diagnosticStatus"),
  deviceName: document.querySelector("#deviceName"),
  deviceMeta: document.querySelector("#deviceMeta"),
  deviceBadge: document.querySelector("#deviceBadge"),
  healthSummary: document.querySelector("#healthSummary"),
  lastSeen: document.querySelector("#lastSeen"),
  loadAvg: document.querySelector("#loadAvg"),
  uptime: document.querySelector("#uptime"),
  defaultRoute: document.querySelector("#defaultRoute"),
  wanIp: document.querySelector("#wanIp"),
  memoryUsage: document.querySelector("#memoryUsage"),
  diskUsage: document.querySelector("#diskUsage"),
  clientCounts: document.querySelector("#clientCounts"),
  connectivityStatus: document.querySelector("#connectivityStatus"),
  inventoryJson: document.querySelector("#inventoryJson"),
  clientList: document.querySelector("#clientList"),
  clientSummary: document.querySelector("#clientSummary"),
  clientSearch: document.querySelector("#clientSearch"),
  interfaceCounters: document.querySelector("#interfaceCounters"),
  networkSummary: document.querySelector("#networkSummary"),
  networkHealth: document.querySelector("#networkHealth"),
  fleetSearch: document.querySelector("#fleetSearch"),
  fleetGroupFilter: document.querySelector("#fleetGroupFilter"),
  fleetTagFilter: document.querySelector("#fleetTagFilter"),
  bulkCommandType: document.querySelector("#bulkCommandType"),
  bulkCommandTarget: document.querySelector("#bulkCommandTarget"),
  sendBulkCommandBtn: document.querySelector("#sendBulkCommandBtn"),
  fleetGroup: document.querySelector("#fleetGroup"),
  fleetTags: document.querySelector("#fleetTags"),
  saveFleetBtn: document.querySelector("#saveFleetBtn"),
  reloadAlertsBtn: document.querySelector("#reloadAlertsBtn"),
  alertSummary: document.querySelector("#alertSummary"),
  alertList: document.querySelector("#alertList"),
  reloadMetricsHistoryBtn: document.querySelector("#reloadMetricsHistoryBtn"),
  metricsHistory: document.querySelector("#metricsHistory"),
  commandType: document.querySelector("#commandType"),
  commandTarget: document.querySelector("#commandTarget"),
  sendCommandBtn: document.querySelector("#sendCommandBtn"),
  packageCommand: document.querySelector("#packageCommand"),
  packageName: document.querySelector("#packageName"),
  sendPackageCommandBtn: document.querySelector("#sendPackageCommandBtn"),
  remoteServerHost: document.querySelector("#remoteServerHost"),
  remoteServerPort: document.querySelector("#remoteServerPort"),
  remotePort: document.querySelector("#remotePort"),
  remoteLocalPort: document.querySelector("#remoteLocalPort"),
  remoteLuCIScheme: document.querySelector("#remoteLuCIScheme"),
  remoteDuration: document.querySelector("#remoteDuration"),
  createRemoteSessionBtn: document.querySelector("#createRemoteSessionBtn"),
  reloadRemoteSessionsBtn: document.querySelector("#reloadRemoteSessionsBtn"),
  remoteSummary: document.querySelector("#remoteSummary"),
  remoteSessionList: document.querySelector("#remoteSessionList"),
  uciConfig: document.querySelector("#uciConfig"),
  uciSection: document.querySelector("#uciSection"),
  uciOption: document.querySelector("#uciOption"),
  uciValue: document.querySelector("#uciValue"),
  uciCommitNow: document.querySelector("#uciCommitNow"),
  uciBackupBtn: document.querySelector("#uciBackupBtn"),
  uciPreviewBtn: document.querySelector("#uciPreviewBtn"),
  uciShowBtn: document.querySelector("#uciShowBtn"),
  uciSetBtn: document.querySelector("#uciSetBtn"),
  uciCommitBtn: document.querySelector("#uciCommitBtn"),
  uciCommitConfirmedBtn: document.querySelector("#uciCommitConfirmedBtn"),
  uciRevertBtn: document.querySelector("#uciRevertBtn"),
  uciRestoreBtn: document.querySelector("#uciRestoreBtn"),
  presetLanIp: document.querySelector("#presetLanIp"),
  presetHostname: document.querySelector("#presetHostname"),
  presetWifiSsid: document.querySelector("#presetWifiSsid"),
  presetWifiKey: document.querySelector("#presetWifiKey"),
  presetDhcpLan: document.querySelector("#presetDhcpLan"),
  reloadCommandsBtn: document.querySelector("#reloadCommandsBtn"),
  reloadAuditBtn: document.querySelector("#reloadAuditBtn"),
  commandSummary: document.querySelector("#commandSummary"),
  auditSummary: document.querySelector("#auditSummary"),
  commandList: document.querySelector("#commandList"),
  commandDetailPanel: document.querySelector("#commandDetailPanel"),
  commandDetailMeta: document.querySelector("#commandDetailMeta"),
  commandDetailOutput: document.querySelector("#commandDetailOutput"),
  copyCommandOutputBtn: document.querySelector("#copyCommandOutputBtn"),
  auditList: document.querySelector("#auditList"),
};

if (els.remoteServerHost) {
  els.remoteServerHost.value = window.location.hostname || "10.10.10.2";
}

function setStatus(message) {
  els.statusLine.textContent = message;
}

async function api(path, options = {}) {
  const headers = {
    ...(options.body ? { "Content-Type": "application/json" } : {}),
    ...(options.headers || {}),
  };
  const response = await fetch(path, { ...options, credentials: "same-origin", headers });
  const requestId = response.headers.get("X-Request-ID");
  const text = await response.text();
  let data = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = text;
    }
  }
  if (!response.ok) {
    if (response.status === 401 && !path.startsWith("/api/auth/")) {
      showLogin();
    }
    const message = data && data.error ? data.error : `HTTP ${response.status}`;
    const suffix = requestId ? ` (${requestId})` : "";
    throw new Error(`${message}${suffix}`);
  }
  return data;
}

function showLogin(message = "") {
  els.appShell.classList.add("is-hidden");
  els.loginView.classList.remove("is-hidden");
  els.loginError.textContent = message;
  els.loginPassword.value = "";
}

function showApp(username) {
  state.username = username || "operator";
  els.operatorName.textContent = state.username;
  els.loginView.classList.add("is-hidden");
  els.appShell.classList.remove("is-hidden");
  els.loginError.textContent = "";
}

async function checkSession() {
  try {
    const me = await api("/api/auth/me");
    showApp(me.username);
    await loadDevices();
  } catch {
    showLogin();
  }
}

async function login() {
  els.loginError.textContent = "";
  const response = await api("/api/auth/login", {
    method: "POST",
    body: JSON.stringify({
      username: els.loginUsername.value.trim(),
      password: els.loginPassword.value,
    }),
  });
  showApp(response.username);
  await loadDevices();
}

async function logout() {
  await api("/api/auth/logout", { method: "POST" });
  state.devices = [];
  state.selectedDeviceId = null;
  showLogin();
}

function formatDate(value) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return date.toLocaleString();
}

function formatShortDate(value) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function statusLabel(status) {
  return {
    requested: "Requested",
    queued: "Queued",
    active: "Active",
    closed: "Closed",
    claimed: "Running",
    completed: "Done",
    failed: "Failed",
    cancelled: "Cancelled",
    expired: "Expired",
  }[status] || status || "-";
}

function remoteStatusLabel(status) {
  return {
    requested: "Запрашивается",
    queued: "Открывается",
    active: "Активен",
    closed: "Закрыт",
    failed: "Ошибка",
    expired: "Завершен",
  }[status] || statusLabel(status);
}

function alertTypeLabel(type) {
  return {
    offline: "Роутер не выходит на связь",
    memory_high: "Мало свободной памяти",
    disk_high: "Мало места на диске",
    wan_ip_changed: "Изменился WAN IP",
    command_attention: "Есть проблемные команды",
    wan_down: "Нет связи с WAN",
    packet_loss_high: "Потери пакетов",
    latency_high: "Высокая задержка",
  }[type] || type || "-";
}

function dangerLabel(type) {
  return {
    reboot: "перезагрузить роутер",
    pkg_remove: "удалить пакет",
    opkg_remove: "удалить пакет",
    uci_set: "изменить конфигурацию",
    uci_commit: "применить конфигурацию",
    uci_commit_confirmed: "применить конфигурацию с проверкой связи",
    uci_restore: "восстановить конфигурацию из backup",
  }[type] || "";
}

function confirmDanger(type) {
  const label = dangerLabel(type);
  if (!label) return true;
  return window.confirm(`Подтвердите действие: ${label}. Продолжить?`);
}

function commandArgs() {
  const type = els.commandType.value;
  if (type === "pkg_list_installed" || type === "route_show" || type === "interfaces_show" || type === "reboot") return {};
  const target = els.commandTarget.value.trim() || "1.1.1.1";
  return { target };
}

function bulkCommandArgs() {
  const type = els.bulkCommandType.value;
  if (type === "pkg_list_installed") return {};
  const target = els.bulkCommandTarget.value.trim() || "1.1.1.1";
  return { target };
}

function currentDevice() {
  return state.devices.find((device) => device.id === state.selectedDeviceId) || null;
}

function deviceDisplayName(device) {
  if (!device) return "-";
  const inventoryHostname = device.inventory && device.inventory.hostname;
  const boardHostname = device.inventory && device.inventory.board && device.inventory.board.hostname;
  for (const value of [inventoryHostname, boardHostname, device.hostname]) {
    if (value && value !== "unknown") return value;
  }
  return device.id || "unknown";
}

function deviceModel(device) {
  return device && device.inventory && device.inventory.board && device.inventory.board.model
    ? device.inventory.board.model
    : "-";
}

function deviceClientCount(device) {
  const leases = Array.isArray(device.inventory && device.inventory.dhcp_leases) ? device.inventory.dhcp_leases.length : 0;
  const wifi = Array.isArray(device.inventory && device.inventory.wifi_clients) ? device.inventory.wifi_clients.length : 0;
  return { leases, wifi, total: Math.max(leases, wifi) };
}

function deviceWanSummary(device) {
  const wanIP = device.inventory && device.inventory.wan_ip;
  const checks = Array.isArray(device.metrics && device.metrics.connectivity_checks) ? device.metrics.connectivity_checks : [];
  if (!device.online) return { label: "Нет данных", className: "offline" };
  if (checks.some((check) => !check.reachable)) return { label: wanIP || "Есть потери связи", className: "warning" };
  return { label: wanIP || "Подключено", className: "online" };
}

function filteredDevices() {
  const search = els.fleetSearch.value.trim().toLowerCase();
  const group = els.fleetGroupFilter.value.trim().toLowerCase();
  const tag = els.fleetTagFilter.value.trim().toLowerCase();
  return state.devices.filter((device) => {
    if (state.filter === "online" && !device.online) return false;
    if (state.filter === "offline" && device.online) return false;
    if (state.filter === "alerts" && !device.active_alerts) return false;
    if (group && String(device.group || "").toLowerCase() !== group) return false;
    if (tag && !((device.tags || []).map((item) => String(item).toLowerCase()).includes(tag))) return false;
    if (search) {
      const haystack = [
        deviceDisplayName(device),
        device.id,
        device.openwrt_version,
        deviceModel(device),
        device.inventory && device.inventory.wan_ip,
        device.group,
        ...(device.tags || []),
      ].join(" ").toLowerCase();
      if (!haystack.includes(search)) return false;
    }
    return true;
  });
}

function renderDevices() {
  const devices = filteredDevices();
  els.deviceList.innerHTML = "";
  const online = state.devices.filter((device) => device.online).length;
  const alerts = state.devices.filter((device) => device.active_alerts).length;
  els.fleetTotalCount.textContent = state.devices.length;
  els.fleetOnlineCount.textContent = online;
  els.fleetOfflineCount.textContent = state.devices.length - online;
  els.fleetAlertCount.textContent = alerts;
  els.navAlertCount.textContent = alerts;

  if (devices.length === 0) {
    els.emptyState.classList.remove("is-hidden");
    return;
  }
  els.emptyState.classList.add("is-hidden");

  for (const device of devices) {
    const displayName = deviceDisplayName(device);
    const clients = deviceClientCount(device);
    const wan = deviceWanSummary(device);
    const item = document.createElement("button");
    item.type = "button";
    item.className = `device-item ${device.id === state.selectedDeviceId ? "is-selected" : ""}`;
    item.innerHTML = `
      <span class="fleet-status ${device.online ? "online" : "offline"}">
        <i></i>${device.online ? "На связи" : "Не на связи"}
      </span>
      <div class="device-identity">
        <strong>${escapeHtml(displayName)}</strong>
        <small>${escapeHtml(device.openwrt_version || device.id)}</small>
      </div>
      <div class="device-context">
        <span>${escapeHtml(deviceModel(device))}</span>
        <small>${escapeHtml([device.group || "", ...(device.tags || [])].filter(Boolean).join(" / ") || "Без группы")}</small>
      </div>
      <span class="wan-state ${wan.className}">${escapeHtml(wan.label)}</span>
      <span class="client-count">${clients.total}</span>
      <span class="${device.active_alerts ? "problem-count has-problems" : "problem-count"}">
        ${device.active_alerts ? `${device.active_alerts} активн.` : "Нет"}
      </span>
      <span class="last-contact">${escapeHtml(formatDate(device.last_seen_at))}</span>
      <span class="row-chevron">›</span>
    `;
    item.addEventListener("click", () => selectDevice(device.id));
    els.deviceList.appendChild(item);
  }
}

function renderDeviceDetail(device) {
  if (!device) {
    els.fleetView.classList.remove("is-hidden");
    els.deviceView.classList.add("is-hidden");
    els.pageTitle.textContent = "Объекты";
    return;
  }

  els.fleetView.classList.add("is-hidden");
  els.deviceView.classList.remove("is-hidden");
  renderDeviceTab();
  const displayName = deviceDisplayName(device);
  els.pageTitle.textContent = displayName;
  els.deviceName.textContent = displayName;
  els.deviceMeta.textContent = `${device.id} - ${device.openwrt_version || "unknown"}`;
  els.deviceBadge.textContent = device.online ? "online" : "offline";
  els.deviceBadge.className = `badge ${device.online ? "online" : "offline"}`;
  els.lastSeen.textContent = formatDate(device.last_seen_at);
  els.loadAvg.textContent = device.metrics && device.metrics.loadavg ? device.metrics.loadavg : "-";
  els.uptime.textContent = device.metrics && device.metrics.uptime ? device.metrics.uptime : "-";
  els.defaultRoute.textContent = device.inventory && device.inventory.default_route ? device.inventory.default_route : "-";
  els.wanIp.textContent = device.inventory && device.inventory.wan_ip ? device.inventory.wan_ip : "-";
  els.memoryUsage.textContent = formatMemory(device.metrics && device.metrics.memory);
  els.diskUsage.textContent = formatDisk(device.metrics && device.metrics.disk);
  const leaseCount = Array.isArray(device.inventory && device.inventory.dhcp_leases) ? device.inventory.dhcp_leases.length : 0;
  const wifiCount = Array.isArray(device.inventory && device.inventory.wifi_clients) ? device.inventory.wifi_clients.length : 0;
  els.clientCounts.textContent = `${leaseCount} DHCP / ${wifiCount} Wi-Fi`;
  els.connectivityStatus.textContent = formatConnectivity(device.metrics && device.metrics.connectivity_checks);
  els.fleetGroup.value = device.group || "";
  els.fleetTags.value = Array.isArray(device.tags) ? device.tags.join(", ") : "";
  els.inventoryJson.textContent = JSON.stringify(device.inventory || {}, null, 2);
  renderHealthSummary(device);
  renderClients(device);
  renderInterfaceCounters(device);
}

function formatMemory(memory) {
  if (!memory || !memory.total_kb) return "-";
  return `${kbToMb(memory.used_kb)} / ${kbToMb(memory.total_kb)} MB`;
}

function formatDisk(disk) {
  if (!disk || !disk.total_kb) return "-";
  return `${disk.used_percent || "-"} (${kbToMb(disk.used_kb)} / ${kbToMb(disk.total_kb)} MB)`;
}

function formatConnectivity(checks) {
  if (!Array.isArray(checks) || checks.length === 0) return "-";
  const reachable = checks.filter((check) => check.reachable).length;
  const reachableChecks = checks.filter((check) => check.reachable);
  const worstReachableLatency = Math.max(0, ...reachableChecks.map((check) => Number(check.latency_ms || 0)));
  const downTargets = checks.filter((check) => !check.reachable).map((check) => check.target).filter(Boolean);
  const lossyChecks = reachableChecks.filter((check) => Number(check.packet_loss_percent || 0) > 0);
  const worstLossy = lossyChecks.sort((a, b) => Number(b.packet_loss_percent || 0) - Number(a.packet_loss_percent || 0))[0];
  if (downTargets.length > 0) {
    return `${reachable}/${checks.length} up / ${worstReachableLatency} ms / down: ${downTargets.join(", ")}`;
  }
  if (worstLossy) {
    return `${reachable}/${checks.length} up / ${worstReachableLatency} ms / loss: ${worstLossy.packet_loss_percent}% on ${worstLossy.target}`;
  }
  return `${reachable}/${checks.length} up / ${worstReachableLatency} ms / no loss`;
}

function renderHealthSummary(device) {
  const checks = Array.isArray(device.metrics && device.metrics.connectivity_checks) ? device.metrics.connectivity_checks : [];
  const serverHost = window.location.hostname;
  const serverCheck = checks.find((check) => check.target === serverHost);
  const wanChecks = checks.filter((check) => check.target !== serverHost);
  const wanReachable = wanChecks.some((check) => check.reachable);
  const rows = [
    ["Статус", device.online ? "На связи" : "Нет связи", device.online ? "ok" : "bad"],
    ["Связь с сервером", serverCheck ? (serverCheck.reachable ? `Есть, ${serverCheck.latency_ms} ms` : "Нет") : "Нет данных", serverCheck && serverCheck.reachable ? "ok" : "warn"],
    ["Интернет/WAN", wanChecks.length ? (wanReachable ? "Есть" : "Нет") : "Нет данных", wanChecks.length && !wanReachable ? "bad" : "ok"],
    ["Активные проблемы", String(device.active_alerts || 0), device.active_alerts ? "warn" : "ok"],
    ["WAN IP", device.inventory && device.inventory.wan_ip ? device.inventory.wan_ip : "-", "neutral"],
  ];
  els.healthSummary.innerHTML = "";
  for (const [label, value, state] of rows) {
    const row = document.createElement("div");
    row.className = `health-item ${state}`;
    row.innerHTML = `<span>${escapeHtml(label)}</span><strong>${escapeHtml(value)}</strong>`;
    els.healthSummary.appendChild(row);
  }
}

function kbToMb(value) {
  return Math.round((Number(value || 0) / 1024) * 10) / 10;
}

function formatBytes(value) {
  let amount = Number(value || 0);
  const units = ["Б", "КБ", "МБ", "ГБ", "ТБ"];
  let unit = 0;
  while (amount >= 1024 && unit < units.length - 1) {
    amount /= 1024;
    unit += 1;
  }
  return `${amount >= 10 || unit === 0 ? Math.round(amount) : amount.toFixed(1)} ${units[unit]}`;
}

function normalizedClients(device) {
  const leases = Array.isArray(device.inventory && device.inventory.dhcp_leases) ? device.inventory.dhcp_leases : [];
  const wifi = Array.isArray(device.inventory && device.inventory.wifi_clients) ? device.inventory.wifi_clients : [];
  const byMac = new Map();
  for (const lease of leases) {
    const mac = String(lease.mac || "").toLowerCase();
    const key = mac || `ip:${lease.ip || Math.random()}`;
    byMac.set(key, {
      name: lease.hostname && lease.hostname !== "*" ? lease.hostname : "",
      ip: lease.ip || "-",
      mac: lease.mac || "-",
      connection: "Проводное / DHCP",
      type: "wired",
      online: true,
    });
  }
  for (const station of wifi) {
    const mac = String(station.mac || "").toLowerCase();
    const existing = byMac.get(mac) || {};
    byMac.set(mac || `wifi:${station.interface || Math.random()}`, {
      ...existing,
      name: existing.name || `Wi-Fi клиент ${station.mac || ""}`,
      ip: existing.ip || "-",
      mac: station.mac || existing.mac || "-",
      connection: `Wi-Fi ${station.interface || ""}`.trim(),
      type: "wifi",
      online: true,
    });
  }
  return [...byMac.values()];
}

function renderClients(device) {
  const clients = normalizedClients(device);
  const search = els.clientSearch.value.trim().toLowerCase();
  const filtered = clients.filter((client) => {
    if (state.clientFilter !== "all" && client.type !== state.clientFilter) return false;
    return !search || [client.name, client.ip, client.mac, client.connection].join(" ").toLowerCase().includes(search);
  });
  els.clientList.innerHTML = "";
  const wifiCount = clients.filter((client) => client.type === "wifi").length;
  els.clientSummary.textContent = `${clients.length} всего / ${wifiCount} Wi-Fi`;
  if (filtered.length === 0) {
    els.clientList.innerHTML = '<div class="inline-empty">Клиенты не найдены</div>';
    return;
  }
  for (const client of filtered) {
    const row = document.createElement("div");
    row.className = "client-row";
    row.innerHTML = `
      <div class="client-name"><span class="client-icon">${client.type === "wifi" ? "⌁" : "▣"}</span><strong>${escapeHtml(client.name || client.ip || "Неизвестное устройство")}</strong></div>
      <span>${escapeHtml(client.ip)}</span>
      <code>${escapeHtml(client.mac)}</code>
      <span>${escapeHtml(client.connection)}</span>
      <span class="client-online"><i></i>В сети</span>
    `;
    els.clientList.appendChild(row);
  }
}

function renderInterfaceCounters(device) {
  const counters = Array.isArray(device.metrics && device.metrics.interface_counters) ? device.metrics.interface_counters : [];
  const addresses = Array.isArray(device.inventory && device.inventory.interfaces) ? device.inventory.interfaces : [];
  const byName = new Map();
  for (const address of addresses) {
    if (!byName.has(address.name)) byName.set(address.name, []);
    byName.get(address.name).push(address.address);
  }
  els.interfaceCounters.innerHTML = "";
  els.networkSummary.textContent = `${counters.length} интерфейсов`;
  els.networkHealth.innerHTML = `
    <div><span>WAN-адрес</span><strong>${escapeHtml(device.inventory && device.inventory.wan_ip ? device.inventory.wan_ip : "Нет данных")}</strong></div>
    <div><span>Маршрут по умолчанию</span><strong>${escapeHtml(device.inventory && device.inventory.default_route ? device.inventory.default_route : "Нет данных")}</strong></div>
    <div><span>Проверки связи</span><strong>${escapeHtml(formatConnectivity(device.metrics && device.metrics.connectivity_checks))}</strong></div>
  `;
  if (counters.length === 0) {
    els.interfaceCounters.innerHTML = '<div class="inline-empty">Нет данных об интерфейсах</div>';
    return;
  }
  for (const item of counters) {
    const row = document.createElement("div");
    const errors = Number(item.rx_errors || 0) + Number(item.tx_errors || 0);
    row.className = "network-row";
    row.innerHTML = `
      <strong>${escapeHtml(item.name || "-")}</strong>
      <span>${escapeHtml((byName.get(item.name) || []).join(", ") || "-")}</span>
      <span>${escapeHtml(formatBytes(item.rx_bytes))}<small>${escapeHtml(item.rx_packets || 0)} пакетов</small></span>
      <span>${escapeHtml(formatBytes(item.tx_bytes))}<small>${escapeHtml(item.tx_packets || 0)} пакетов</small></span>
      <span class="${errors ? "interface-errors" : ""}">${escapeHtml(errors)}</span>
    `;
    els.interfaceCounters.appendChild(row);
  }
}

async function loadMetricsHistory() {
  if (!state.selectedDeviceId) return;
  const data = await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/metrics-history?limit=12`);
  renderMetricsHistory(data.samples || []);
}

function renderMetricsHistory(samples) {
  els.metricsHistory.innerHTML = "";
  if (samples.length === 0) {
    els.metricsHistory.textContent = "No history yet";
    return;
  }
  for (const sample of samples) {
    const row = document.createElement("div");
    row.className = "mini-row";
    row.innerHTML = `<strong>${escapeHtml(formatDate(sample.created_at))}</strong><span>${escapeHtml(formatMemory(sample.metrics && sample.metrics.memory))}</span><small>${escapeHtml(formatDisk(sample.metrics && sample.metrics.disk))}</small>`;
    els.metricsHistory.appendChild(row);
  }
}

async function loadAlerts() {
  if (!state.selectedDeviceId) return;
  const data = await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/alerts`);
  state.alerts = data.alerts || [];
  renderAlerts(state.alerts);
}

function renderAlerts(alerts) {
  els.alertList.innerHTML = "";
  const activeCount = alerts.filter((alert) => alert.status === "active").length;
  const acknowledgedCount = alerts.filter((alert) => alert.status === "acknowledged").length;
  els.alertSummary.textContent = `${activeCount} active / ${acknowledgedCount} ack`;
  if (alerts.length === 0) {
    els.alertList.textContent = "No active alerts";
    return;
  }
  for (const alert of alerts) {
    const details = alert.details ? Object.entries(alert.details).map(([key, value]) => `${key}: ${value}`).join(" / ") : "";
    const row = document.createElement("div");
    row.className = `mini-row alert-row ${alert.severity || "warning"} ${alert.status || "active"}`;
    row.innerHTML = `
      <strong>${escapeHtml(alertTypeLabel(alert.type))}</strong>
      <span>${escapeHtml(`${alert.severity || "-"} / ${alert.status || "-"}`)}</span>
      <details class="alert-detail">
        <summary>${escapeHtml(details || "Подробности")}</summary>
        <div>Первый раз: ${escapeHtml(formatDate(alert.first_seen_at || alert.created_at))}</div>
        <div>Последний раз: ${escapeHtml(formatDate(alert.last_seen_at || alert.created_at))}</div>
        <div>${escapeHtml(alert.message || "")}</div>
      </details>
      <button type="button" data-action="diagnose">Диагностика</button>
      <button type="button" data-action="commands">Команды</button>
      <button type="button" data-action="ack" ${alert.status === "active" ? "" : "disabled"}>Ack</button>
    `;
    row.querySelector('[data-action="diagnose"]').addEventListener("click", () => runAlertDiagnostics(alert));
    row.querySelector('[data-action="commands"]').addEventListener("click", scrollToCommands);
    row.querySelector('[data-action="ack"]').addEventListener("click", () => acknowledgeAlert(alert.id));
    els.alertList.appendChild(row);
  }
}

async function loadDevices() {
  setStatus("Обновление");
  const data = await api("/api/devices");
  state.devices = data.devices || [];
  if (state.selectedDeviceId && !currentDevice()) {
    state.selectedDeviceId = null;
  }
  renderDevices();
  renderDeviceDetail(currentDevice());
  if (state.selectedDeviceId) {
    await Promise.all([loadCommands(), loadAudit(), loadMetricsHistory(), loadAlerts(), loadRemoteSessions()]);
  }
  setStatus(`Обновлено ${new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`);
}

async function selectDevice(id) {
  state.selectedDeviceId = id;
  state.deviceTab = "overview";
  renderDevices();
  renderDeviceDetail(currentDevice());
  await Promise.all([loadCommands(), loadAudit(), loadMetricsHistory(), loadAlerts(), loadRemoteSessions()]);
}

function showFleet() {
  state.selectedDeviceId = null;
  state.selectedCommand = null;
  renderDevices();
  renderDeviceDetail(null);
}

function selectDeviceTab(tab) {
  state.deviceTab = tab;
  renderDeviceTab();
}

function renderDeviceTab() {
  for (const button of document.querySelectorAll(".device-tab")) {
    button.classList.toggle("is-active", button.dataset.deviceTabTarget === state.deviceTab);
  }
  for (const section of document.querySelectorAll("[data-device-tab]")) {
    section.classList.toggle("is-hidden", section.dataset.deviceTab !== state.deviceTab);
  }
}

function openLuciOrRemoteAccess() {
  const activeSession = state.remoteSessions.find((session) => session.status === "active" && session.luci_port);
  if (activeSession) {
    window.open(`/luci/${encodeURIComponent(activeSession.device_id)}/${encodeURIComponent(activeSession.id)}/`, "_blank", "noopener");
    return;
  }
  selectDeviceTab("operations");
  els.remoteAccessPanel.scrollIntoView({ behavior: "smooth", block: "start" });
  setStatus("Создайте удаленную сессию, затем откройте LuCI");
}

async function loadCommands() {
  if (!state.selectedDeviceId) return;
  const data = await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/commands?limit=50`);
  state.commands = data.commands || [];
  renderCommands(state.commands);
  if (state.selectedCommand) {
    const refreshed = state.commands.find((command) => command.id === state.selectedCommand.id);
    state.selectedCommand = refreshed || null;
    renderCommandDetail(state.selectedCommand);
  }
}

function renderCommands(commands) {
  els.commandList.innerHTML = "";
  renderCommandSummary(commands);
  const filtered = state.commandFilter === "all" ? commands : commands.filter((command) => command.status === state.commandFilter);
  if (filtered.length === 0) {
    els.commandList.textContent = "No commands";
    return;
  }

  for (const command of filtered) {
    const row = document.createElement("div");
    row.className = "row";
    const canCancel = command.status === "queued" || command.status === "claimed";
    const output = command.output || JSON.stringify(command.args || {});
    const finishedAt = command.completed_at || command.cancelled_at || command.expired_at;
    row.innerHTML = `
      <div>
        <strong>${escapeHtml(command.type)}</strong><br>
        <small>${escapeHtml(command.id)}</small>
      </div>
      <span class="status ${escapeHtml(command.status)}">${escapeHtml(statusLabel(command.status))}</span>
      <span>try ${command.attempt_count || 0}/${command.max_attempts || 3}</span>
      <span class="lifecycle-time">${escapeHtml(formatShortDate(finishedAt || command.claimed_at || command.created_at))}</span>
      <details class="command-output">
        <summary>${escapeHtml(outputSummary(output))}</summary>
        <pre>${escapeHtml(output)}</pre>
      </details>
      <div class="row-actions">
        <button type="button" data-action="detail">Open</button>
        <button type="button" data-action="cancel" ${canCancel ? "" : "disabled"}>Cancel</button>
      </div>
    `;
    row.querySelector('[data-action="detail"]').addEventListener("click", () => {
      state.selectedCommand = command;
      renderCommandDetail(command);
    });
    row.querySelector('[data-action="cancel"]').addEventListener("click", () => cancelCommand(command.id));
    els.commandList.appendChild(row);
  }
}

function renderCommandSummary(commands) {
  const counts = commands.reduce((acc, command) => {
    acc[command.status] = (acc[command.status] || 0) + 1;
    return acc;
  }, {});
  const total = commands.length;
  const active = (counts.queued || 0) + (counts.claimed || 0);
  const failed = (counts.failed || 0) + (counts.expired || 0);
  els.commandSummary.textContent = `${total} total / ${active} active / ${failed} attention`;
}

function renderCommandDetail(command) {
  if (!command) {
    els.commandDetailPanel.classList.add("is-hidden");
    return;
  }
  const output = command.output || JSON.stringify(command.args || {}, null, 2);
  els.commandDetailPanel.classList.remove("is-hidden");
  els.commandDetailMeta.innerHTML = `
    <span>${escapeHtml(command.type)}</span>
    <span>${escapeHtml(statusLabel(command.status))}</span>
    <span>try ${command.attempt_count || 0}/${command.max_attempts || 3}</span>
    <span>created ${escapeHtml(formatShortDate(command.created_at))}</span>
    <span>expires ${escapeHtml(formatShortDate(command.expires_at))}</span>
    <span>claimed ${escapeHtml(formatShortDate(command.claimed_at))}</span>
    <span>finished ${escapeHtml(formatShortDate(command.completed_at || command.cancelled_at || command.expired_at))}</span>
    <span>${escapeHtml(command.id)}</span>
  `;
  els.commandDetailOutput.innerHTML = highlightOutput(output);
}

function highlightOutput(output) {
  return escapeHtml(output).replace(/^(BACKUP|BEFORE|AFTER|CHANGE|DIFF|PREVIEW)(.*)$/gm, '<span class="output-section">$1$2</span>');
}

function outputSummary(output) {
  const firstLine = String(output || "").split("\n")[0] || "-";
  return firstLine.length > 90 ? `${firstLine.slice(0, 90)}...` : firstLine;
}

async function loadAudit() {
  if (!state.selectedDeviceId) return;
  const data = await api(`/api/audit-events?device_id=${encodeURIComponent(state.selectedDeviceId)}&limit=50`);
  renderAudit(data.audit_events || []);
}

function renderAudit(events) {
  els.auditList.innerHTML = "";
  els.auditSummary.textContent = `${events.length} events`;
  if (events.length === 0) {
    els.auditList.textContent = "No audit events";
    return;
  }

  for (const event of events) {
    const row = document.createElement("div");
    row.className = "row audit";
    row.innerHTML = `
      <small>${formatDate(event.created_at)}</small>
      <strong>${escapeHtml(event.action)}</strong>
      <small>${escapeHtml(event.command_id || event.device_id || "-")}</small>
    `;
    els.auditList.appendChild(row);
  }
}

async function loadRemoteSessions() {
  if (!state.selectedDeviceId) return;
  const data = await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/remote-sessions?limit=25`);
  state.remoteSessions = data.remote_sessions || [];
  renderRemoteSessions(state.remoteSessions);
}

function renderRemoteSessions(sessions) {
  els.remoteSessionList.innerHTML = "";
  const active = sessions.filter((session) => ["requested", "queued", "active"].includes(session.status)).length;
  els.remoteSummary.textContent = active ? `${active} активн.` : "Нет активных сессий";
  if (sessions.length === 0) {
    els.remoteSessionList.innerHTML = '<div class="inline-empty">Удаленный доступ еще не открывался</div>';
    return;
  }
  for (const session of sessions) {
    const canClose = ["requested", "queued", "active"].includes(session.status);
    const endpoint = `${session.server_host || "-"}:${session.remote_port || "-"}`;
    const connectCommand = session.remote_port ? `ssh -p ${session.remote_port} root@${session.server_host || "server"}` : "-";
    const canOpenLuCI = session.status === "active" && session.luci_port;
    const row = document.createElement("div");
    row.className = "remote-session-row";
    row.innerHTML = `
      <div class="remote-session-main">
        <span class="remote-session-status ${escapeHtml(session.status || "")}"><i></i>${escapeHtml(remoteStatusLabel(session.status))}</span>
        <div>
          <strong>Доступ к роутеру</strong>
          <small>${escapeHtml(endpoint)} · до ${escapeHtml(formatShortDate(session.expires_at))}</small>
        </div>
      </div>
      <code>${escapeHtml(connectCommand)}</code>
      <div class="row-actions">
        <button type="button" data-action="copy" ${session.remote_port ? "" : "disabled"}>Копировать SSH</button>
        <button class="primary" type="button" data-action="luci" ${canOpenLuCI ? "" : "disabled"}>Открыть LuCI</button>
        <button type="button" data-action="close" ${canClose ? "" : "disabled"}>Закрыть</button>
      </div>
    `;
    row.querySelector('[data-action="luci"]').addEventListener("click", () => {
      window.open(`/luci/${encodeURIComponent(session.device_id)}/${encodeURIComponent(session.id)}/`, "_blank", "noopener");
    });
    row.querySelector('[data-action="copy"]').addEventListener("click", async () => {
      await navigator.clipboard.writeText(connectCommand);
      setStatus("SSH-команда скопирована");
    });
    row.querySelector('[data-action="close"]').addEventListener("click", () => closeRemoteSession(session.id));
    els.remoteSessionList.appendChild(row);
  }
}

async function sendCommand() {
  if (!state.selectedDeviceId) return;
  if (!confirmDanger(els.commandType.value)) return;
  setStatus("Creating command");
  await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/commands`, {
    method: "POST",
    body: JSON.stringify({
      type: els.commandType.value,
      args: commandArgs(),
    }),
  });
  await Promise.all([loadCommands(), loadAudit()]);
  setStatus("Command queued");
}

function uciConfigArg() {
  return { config: els.uciConfig.value };
}

function uciSetArgs() {
  return {
    config: els.uciConfig.value,
    section: els.uciSection.value.trim(),
    option: els.uciOption.value.trim(),
    value: els.uciValue.value,
    commit: els.uciCommitNow.checked ? "true" : "false",
  };
}

async function createDeviceCommand(type, args, options = {}) {
  if (!state.selectedDeviceId) return;
  if (!options.skipConfirm && !confirmDanger(type)) return;
  await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/commands`, {
    method: "POST",
    body: JSON.stringify({ type, args }),
  });
  if (!options.skipRefresh) {
    await Promise.all([loadCommands(), loadAudit(), loadAlerts()]);
  }
}

async function sendBulkCommand() {
  const devices = filteredDevices();
  if (devices.length === 0) {
    setStatus("No filtered devices");
    return;
  }
  const type = els.bulkCommandType.value;
  if (!confirmDanger(type)) return;
  setStatus(`Queueing ${type} for ${devices.length} devices`);
  await api("/api/devices/bulk-commands", {
    method: "POST",
    body: JSON.stringify({
      device_ids: devices.map((device) => device.id),
      type,
      args: bulkCommandArgs(),
    }),
  });
  await Promise.all([loadCommands(), loadAudit()]);
  setStatus(`${type} queued for ${devices.length} devices`);
}

async function saveFleetMetadata() {
  if (!state.selectedDeviceId) return;
  setStatus("Saving fleet metadata");
  const tags = els.fleetTags.value.split(",").map((tag) => tag.trim()).filter(Boolean);
  const device = await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/fleet`, {
    method: "PATCH",
    body: JSON.stringify({ group: els.fleetGroup.value.trim(), tags }),
  });
  state.devices = state.devices.map((item) => (item.id === device.id ? device : item));
  renderDevices();
  renderDeviceDetail(device);
  await loadAudit();
  setStatus("Fleet metadata saved");
}

async function sendPackageCommand() {
  if (!state.selectedDeviceId) return;
  const type = els.packageCommand.value;
  const packageName = els.packageName.value.trim();
  if ((type === "pkg_install" || type === "pkg_remove") && !packageName) {
    setStatus("Package name is required");
    return;
  }
  if (!confirmDanger(type)) return;
  setStatus(`Queueing ${type}`);
  await createDeviceCommand(type, packageName ? { package: packageName } : {}, { skipConfirm: true });
  setStatus(`${type} queued`);
}

async function sendUciCommand(type) {
  if (!state.selectedDeviceId) return;
  if ((type === "uci_set" || type === "uci_preview") && (!els.uciSection.value.trim() || !els.uciOption.value.trim())) {
    setStatus("UCI section and option are required");
    return;
  }
  if (!confirmDanger(type)) return;
  setStatus(`Queueing ${type}`);
  const args = type === "uci_set" || type === "uci_preview" ? uciSetArgs() : uciConfigArg();
  await createDeviceCommand(type, args, { skipConfirm: true });
  setStatus(`${type} queued`);
}

async function createRemoteSession() {
  if (!state.selectedDeviceId) return;
  const serverHost = els.remoteServerHost.value.trim() || window.location.hostname;
  const serverPort = Number(els.remoteServerPort.value || 2222);
  const remotePort = Number(els.remotePort.value || 0);
  const localPort = Number(els.remoteLocalPort.value || 22);
  const luciScheme = els.remoteLuCIScheme.value || "http";
  const durationSeconds = Number(els.remoteDuration.value || 900);
  if (!serverHost) {
    setStatus("Tunnel server is required");
    return;
  }
  if (!window.confirm(`Открыть временный доступ к роутеру на ${Math.round(durationSeconds / 60)} минут?`)) return;
  setStatus("Открытие удаленного доступа");
  await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/remote-sessions`, {
    method: "POST",
    body: JSON.stringify({
      target: "ssh",
      server_host: serverHost,
      server_port: serverPort,
      remote_port: remotePort,
      local_port: localPort,
      luci_scheme: luciScheme,
      duration_seconds: durationSeconds,
    }),
  });
  await Promise.all([loadRemoteSessions(), loadCommands(), loadAudit()]);
  setStatus("Команда открытия доступа отправлена");
}

async function closeRemoteSession(sessionId) {
  if (!state.selectedDeviceId) return;
  if (!window.confirm("Закрыть удаленный доступ к роутеру?")) return;
  setStatus("Закрытие удаленного доступа");
  await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/remote-sessions/${encodeURIComponent(sessionId)}/close`, {
    method: "POST",
  });
  await Promise.all([loadRemoteSessions(), loadAudit()]);
  setStatus("Удаленный доступ закрыт");
}

function presetCommand(preset) {
  switch (preset) {
    case "lan_ip":
      return {
        config: "network",
        section: "lan",
        option: "ipaddr",
        value: els.presetLanIp.value.trim(),
        commit: "false",
      };
    case "hostname":
      return {
        config: "system",
        section: "@system[0]",
        option: "hostname",
        value: els.presetHostname.value.trim(),
        commit: "false",
      };
    case "wifi_ssid":
      return {
        config: "wireless",
        section: "@wifi-iface[0]",
        option: "ssid",
        value: els.presetWifiSsid.value.trim(),
        commit: "false",
      };
    case "wifi_key":
      return {
        config: "wireless",
        section: "@wifi-iface[0]",
        option: "key",
        value: els.presetWifiKey.value,
        commit: "false",
      };
    case "dhcp_lan":
      return {
        config: "dhcp",
        section: "lan",
        option: "ignore",
        value: els.presetDhcpLan.value,
        commit: "false",
      };
    default:
      return null;
  }
}

async function sendPresetCommand(preset, action) {
  const args = presetCommand(preset);
  if (!args) return;
  if (!args.value) {
    setStatus("Preset value is required");
    return;
  }
  const type = action === "preview" ? "uci_preview" : "uci_set";
  if (!confirmDanger(type)) return;
  setStatus(`Queueing ${preset} ${action}`);
  await createDeviceCommand(type, args, { skipConfirm: true });
  setStatus(`${preset} ${action} queued`);
}

async function cancelCommand(commandId) {
  if (!state.selectedDeviceId) return;
  setStatus("Cancelling command");
  await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/commands/${encodeURIComponent(commandId)}/cancel`, {
    method: "POST",
  });
  await Promise.all([loadCommands(), loadAudit()]);
  setStatus("Command cancelled");
}

async function acknowledgeAlert(alertId) {
  if (!state.selectedDeviceId) return;
  setStatus("Acknowledging alert");
  await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/alerts/${encodeURIComponent(alertId)}/acknowledge`, {
    method: "POST",
  });
  await Promise.all([loadAlerts(), loadAudit(), loadDevices()]);
  setStatus("Alert acknowledged");
}

function diagnosticCommand(name) {
  const serverHost = window.location.hostname || "10.10.10.2";
  switch (name) {
    case "ping_server":
      return { type: "ping", args: { target: serverHost } };
    case "ping_internet":
      return { type: "ping", args: { target: "1.1.1.1" } };
    case "traceroute_internet":
      return { type: "traceroute", args: { target: "1.1.1.1" } };
    case "show_routes":
      return { type: "route_show", args: {} };
    case "show_interfaces":
      return { type: "interfaces_show", args: {} };
    default:
      return null;
  }
}

async function sendDiagnostic(name) {
  const command = diagnosticCommand(name);
  if (!command) return;
  setStatus("Запуск проверки");
  await createDeviceCommand(command.type, command.args);
  setStatus("Проверка поставлена в очередь");
  scrollToCommands();
}

async function runFullDiagnostic() {
  const checks = ["ping_server", "ping_internet", "show_routes", "show_interfaces"];
  els.runFullDiagnosticBtn.disabled = true;
  els.diagnosticStatus.classList.add("is-running");
  els.diagnosticStatus.innerHTML = `
    <span class="operation-icon">↻</span>
    <div><strong>Диагностика запускается</strong><small>Отправляем проверки на роутер</small></div>
  `;
  try {
    for (const name of checks) {
      const command = diagnosticCommand(name);
      await createDeviceCommand(command.type, command.args, { skipRefresh: true });
    }
    await Promise.all([loadCommands(), loadAudit(), loadAlerts()]);
    els.diagnosticStatus.classList.remove("is-running");
    els.diagnosticStatus.classList.add("is-complete");
    els.diagnosticStatus.innerHTML = `
      <span class="operation-icon">✓</span>
      <div><strong>Диагностика запущена</strong><small>Результаты появятся в истории команд</small></div>
      <button id="openDiagnosticResultsBtn" type="button">Открыть результаты</button>
    `;
    els.diagnosticStatus.querySelector("#openDiagnosticResultsBtn").addEventListener("click", scrollToCommands);
    setStatus("Полная диагностика поставлена в очередь");
  } finally {
    els.runFullDiagnosticBtn.disabled = false;
  }
}

async function runAlertDiagnostics(alert) {
  const target = alert.details && alert.details.target ? alert.details.target : "1.1.1.1";
  if (alert.type === "latency_high" || alert.type === "packet_loss_high" || alert.type === "wan_down") {
    await createDeviceCommand("ping", { target });
  } else if (alert.type === "offline") {
    await createDeviceCommand("ping", { target: window.location.hostname || "10.10.10.2" });
  } else {
    scrollToCommands();
    return;
  }
  setStatus("Diagnostic queued");
  scrollToCommands();
}

function scrollToCommands() {
  selectDeviceTab("expert");
  document.querySelector("#commandsPanel")?.scrollIntoView({ behavior: "smooth", block: "start" });
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

async function checkHealth() {
  try {
    const response = await fetch("/healthz");
    els.apiState.textContent = response.ok ? "API: online" : "API: degraded";
  } catch {
    els.apiState.textContent = "API: offline";
  }
}

els.loginForm.addEventListener("submit", (event) => {
  event.preventDefault();
  login().catch((error) => showLogin(error.message));
});
els.logoutBtn.addEventListener("click", () => logout().catch((error) => setStatus(error.message)));

els.refreshBtn.addEventListener("click", () => loadDevices().catch((error) => setStatus(error.message)));
els.backToFleetBtn.addEventListener("click", showFleet);
els.quickDiagnosticBtn.addEventListener("click", () => selectDeviceTab("operations"));
els.openLuciBtn.addEventListener("click", openLuciOrRemoteAccess);
els.runFullDiagnosticBtn.addEventListener("click", () => runFullDiagnostic().catch((error) => setStatus(error.message)));
els.reloadCommandsBtn.addEventListener("click", () => loadCommands().catch((error) => setStatus(error.message)));
els.reloadAuditBtn.addEventListener("click", () => loadAudit().catch((error) => setStatus(error.message)));
els.reloadAlertsBtn.addEventListener("click", () => loadAlerts().catch((error) => setStatus(error.message)));
els.reloadMetricsHistoryBtn.addEventListener("click", () => loadMetricsHistory().catch((error) => setStatus(error.message)));
els.reloadRemoteSessionsBtn.addEventListener("click", () => loadRemoteSessions().catch((error) => setStatus(error.message)));
els.sendCommandBtn.addEventListener("click", () => sendCommand().catch((error) => setStatus(error.message)));
els.sendBulkCommandBtn.addEventListener("click", () => sendBulkCommand().catch((error) => setStatus(error.message)));
els.saveFleetBtn.addEventListener("click", () => saveFleetMetadata().catch((error) => setStatus(error.message)));
els.sendPackageCommandBtn.addEventListener("click", () => sendPackageCommand().catch((error) => setStatus(error.message)));
els.createRemoteSessionBtn.addEventListener("click", () => createRemoteSession().catch((error) => setStatus(error.message)));
els.uciBackupBtn.addEventListener("click", () => sendUciCommand("uci_backup").catch((error) => setStatus(error.message)));
els.uciPreviewBtn.addEventListener("click", () => sendUciCommand("uci_preview").catch((error) => setStatus(error.message)));
els.uciShowBtn.addEventListener("click", () => sendUciCommand("uci_show").catch((error) => setStatus(error.message)));
els.uciSetBtn.addEventListener("click", () => sendUciCommand("uci_set").catch((error) => setStatus(error.message)));
els.uciCommitBtn.addEventListener("click", () => sendUciCommand("uci_commit").catch((error) => setStatus(error.message)));
els.uciCommitConfirmedBtn.addEventListener("click", () => sendUciCommand("uci_commit_confirmed").catch((error) => setStatus(error.message)));
els.uciRevertBtn.addEventListener("click", () => sendUciCommand("uci_revert").catch((error) => setStatus(error.message)));
els.uciRestoreBtn.addEventListener("click", () => sendUciCommand("uci_restore").catch((error) => setStatus(error.message)));
els.copyCommandOutputBtn.addEventListener("click", async () => {
  if (!state.selectedCommand) return;
  const output = state.selectedCommand.output || JSON.stringify(state.selectedCommand.args || {}, null, 2);
  await navigator.clipboard.writeText(output);
  setStatus("Output copied");
});

for (const button of document.querySelectorAll(".preset-btn")) {
  button.addEventListener("click", () => {
    sendPresetCommand(button.dataset.preset, button.dataset.action).catch((error) => setStatus(error.message));
  });
}

for (const button of document.querySelectorAll(".diagnostic-btn")) {
  button.addEventListener("click", () => {
    sendDiagnostic(button.dataset.diagnostic).catch((error) => setStatus(error.message));
  });
}

for (const button of document.querySelectorAll(".device-tab")) {
  button.addEventListener("click", () => selectDeviceTab(button.dataset.deviceTabTarget));
}

for (const button of document.querySelectorAll(".client-filter")) {
  button.addEventListener("click", () => {
    state.clientFilter = button.dataset.clientFilter;
    document.querySelectorAll(".client-filter").forEach((item) => item.classList.toggle("is-active", item === button));
    renderClients(currentDevice());
  });
}

els.clientSearch.addEventListener("input", () => renderClients(currentDevice()));

for (const button of document.querySelectorAll(".filter")) {
  button.addEventListener("click", () => {
    state.filter = button.dataset.filter;
    document.querySelectorAll(".filter").forEach((item) => item.classList.toggle("is-active", item === button));
    renderDevices();
  });
}

for (const input of [els.fleetSearch, els.fleetGroupFilter, els.fleetTagFilter]) {
  input.addEventListener("input", renderDevices);
}

for (const button of document.querySelectorAll(".command-filter")) {
  button.addEventListener("click", () => {
    state.commandFilter = button.dataset.commandFilter;
    document.querySelectorAll(".command-filter").forEach((item) => item.classList.toggle("is-active", item === button));
    renderCommands(state.commands);
  });
}

els.commandType.addEventListener("change", () => {
  els.commandTarget.disabled = ["pkg_list_installed", "route_show", "interfaces_show", "reboot"].includes(els.commandType.value);
});

els.bulkCommandType.addEventListener("change", () => {
  els.bulkCommandTarget.disabled = els.bulkCommandType.value === "pkg_list_installed";
});

checkHealth();
checkSession();
setInterval(() => {
  checkHealth();
  if (!els.appShell.classList.contains("is-hidden")) {
    loadDevices().catch((error) => setStatus(error.message));
  }
}, 30000);
