const state = {
  username: "",
  user: null,
  devices: [],
  users: [],
  passwordResetToken: "",
  selectedDeviceId: null,
  deviceTab: "overview",
  filter: "all",
  fleetSortKey: "status",
  fleetSortDir: "desc",
  clientFilter: "all",
  alertStatusFilter: "open",
  commandFilter: "all",
  commandStatusFilter: "",
  commandLimit: 25,
  commandOffset: 0,
  commandHasMore: false,
  auditLimit: 25,
  auditOffset: 0,
  auditHasMore: false,
  commands: [],
  auditEvents: [],
  alerts: [],
  notificationSettings: null,
  notificationChannels: {},
  notifications: [],
  notificationMetrics: {},
  profileTab: "account",
  inboxNotifications: [],
  notificationUnread: 0,
  lanClients: [],
  remoteSessions: [],
  backups: [],
  selectedCommand: null,
  presetReview: null,
  luciAction: "setup",
  mobileRoute: "fleet",
  previousMobileRoute: "fleet",
  liveConnected: false,
  lastUpdatedAt: null,
  releaseMetadata: null,
  releaseMetadataCheckedAt: 0,
  systemStateRetry: null,
  confirmationRequest: null,
};

let eventSource = null;
let liveRefreshTimer = null;

const els = {
  loginView: document.querySelector("#loginView"),
  loginForm: document.querySelector("#loginForm"),
  loginUsername: document.querySelector("#loginUsername"),
  loginPassword: document.querySelector("#loginPassword"),
  loginSubmitBtn: document.querySelector("#loginSubmitBtn"),
  loginError: document.querySelector("#loginError"),
  forgotPasswordBtn: document.querySelector("#forgotPasswordBtn"),
  forgotPasswordDialog: document.querySelector("#forgotPasswordDialog"),
  forgotPasswordForm: document.querySelector("#forgotPasswordForm"),
  closeForgotPasswordBtn: document.querySelector("#closeForgotPasswordBtn"),
  passwordResetIdentifier: document.querySelector("#passwordResetIdentifier"),
  forgotPasswordMessage: document.querySelector("#forgotPasswordMessage"),
  passwordResetDialog: document.querySelector("#passwordResetDialog"),
  passwordResetForm: document.querySelector("#passwordResetForm"),
  closePasswordResetBtn: document.querySelector("#closePasswordResetBtn"),
  resetNewPassword: document.querySelector("#resetNewPassword"),
  resetConfirmPassword: document.querySelector("#resetConfirmPassword"),
  passwordResetMessage: document.querySelector("#passwordResetMessage"),
  appShell: document.querySelector("#appShell"),
  operatorName: document.querySelector("#operatorName"),
  fleetNavBtn: document.querySelector("#fleetNavBtn"),
  problemsNavBtn: document.querySelector("#problemsNavBtn"),
  operationsNavBtn: document.querySelector("#operationsNavBtn"),
  profileBtn: document.querySelector("#profileBtn"),
  logoutBtn: document.querySelector("#logoutBtn"),
  apiState: document.querySelector("#apiState"),
  serverVersion: document.querySelector("#serverVersion"),
  refreshBtn: document.querySelector("#refreshBtn"),
  addRouterBtn: document.querySelector("#addRouterBtn"),
  addUserBtn: document.querySelector("#addUserBtn"),
  enrollmentGrantDialog: document.querySelector("#enrollmentGrantDialog"),
  enrollmentTokenOutput: document.querySelector("#enrollmentTokenOutput"),
  copyEnrollmentTokenBtn: document.querySelector("#copyEnrollmentTokenBtn"),
  enrollmentCopyState: document.querySelector("#enrollmentCopyState"),
  createUserDialog: document.querySelector("#createUserDialog"),
  createUserForm: document.querySelector("#createUserForm"),
  closeCreateUserBtn: document.querySelector("#closeCreateUserBtn"),
  newUsername: document.querySelector("#newUsername"),
  newUserEmail: document.querySelector("#newUserEmail"),
  newUserPassword: document.querySelector("#newUserPassword"),
  newUserRole: document.querySelector("#newUserRole"),
  cancelCreateUserBtn: document.querySelector("#cancelCreateUserBtn"),
  createUserSubmitBtn: document.querySelector("#createUserSubmitBtn"),
  createUserMessage: document.querySelector("#createUserMessage"),
  deviceList: document.querySelector("#deviceList"),
  fleetView: document.querySelector("#fleetView"),
  fleetTotalCount: document.querySelector("#fleetTotalCount"),
  fleetOnlineCount: document.querySelector("#fleetOnlineCount"),
  fleetOfflineCount: document.querySelector("#fleetOfflineCount"),
  fleetAlertCount: document.querySelector("#fleetAlertCount"),
  navAlertCount: document.querySelector("#navAlertCount"),
  statusLine: document.querySelector("#statusLine"),
  liveState: document.querySelector("#liveState"),
  pageTitle: document.querySelector("#pageTitle"),
  emptyState: document.querySelector("#emptyState"),
  emptyStateTitle: document.querySelector("#emptyState h2"),
  emptyStateDescription: document.querySelector("#emptyState p"),
  deviceView: document.querySelector("#deviceView"),
  backToFleetBtn: document.querySelector("#backToFleetBtn"),
  updateAgentBtn: document.querySelector("#updateAgentBtn"),
  rollbackAgentBtn: document.querySelector("#rollbackAgentBtn"),
  agentUpdateStatus: document.querySelector("#agentUpdateStatus"),
  agentRollbackDialog: document.querySelector("#agentRollbackDialog"),
  agentRollbackForm: document.querySelector("#agentRollbackForm"),
  closeAgentRollbackBtn: document.querySelector("#closeAgentRollbackBtn"),
  agentRollbackVersion: document.querySelector("#agentRollbackVersion"),
  agentRollbackPreview: document.querySelector("#agentRollbackPreview"),
  agentRollbackMessage: document.querySelector("#agentRollbackMessage"),
  quickDiagnosticBtn: document.querySelector("#quickDiagnosticBtn"),
  openLuciBtn: document.querySelector("#openLuciBtn"),
  remoteAccessPanel: document.querySelector("#remoteAccessPanel"),
  runFullDiagnosticBtn: document.querySelector("#runFullDiagnosticBtn"),
  diagnosticStatus: document.querySelector("#diagnosticStatus"),
  deviceName: document.querySelector("#deviceName"),
  deviceMeta: document.querySelector("#deviceMeta"),
  deviceBadge: document.querySelector("#deviceBadge"),
  informationStatus: document.querySelector("#informationStatus"),
  infoHostname: document.querySelector("#infoHostname"),
  infoModel: document.querySelector("#infoModel"),
  infoOpenWrt: document.querySelector("#infoOpenWrt"),
  infoTarget: document.querySelector("#infoTarget"),
  infoSystem: document.querySelector("#infoSystem"),
  infoKernel: document.querySelector("#infoKernel"),
  infoAgentVersion: document.querySelector("#infoAgentVersion"),
  infoCreatedAt: document.querySelector("#infoCreatedAt"),
  infoLastSeenAt: document.querySelector("#infoLastSeenAt"),
  infoDeviceId: document.querySelector("#infoDeviceId"),
  infoBoardName: document.querySelector("#infoBoardName"),
  infoRootfs: document.querySelector("#infoRootfs"),
  healthSummary: document.querySelector("#healthSummary"),
  deviceStatusHero: document.querySelector("#deviceStatusHero"),
  agentHealthSummary: document.querySelector("#agentHealthSummary"),
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
  inventorySummary: document.querySelector("#inventorySummary"),
  inventoryStatus: document.querySelector("#inventoryStatus"),
  clientList: document.querySelector("#clientList"),
  clientSummary: document.querySelector("#clientSummary"),
  clientSearch: document.querySelector("#clientSearch"),
  clientDataState: document.querySelector("#clientDataState"),
  interfaceCounters: document.querySelector("#interfaceCounters"),
  networkSummary: document.querySelector("#networkSummary"),
  networkHealth: document.querySelector("#networkHealth"),
  networkDataState: document.querySelector("#networkDataState"),
  fleetSearch: document.querySelector("#fleetSearch"),
  fleetGroupFilter: document.querySelector("#fleetGroupFilter"),
  fleetTagFilter: document.querySelector("#fleetTagFilter"),
  fleetFilterToggle: document.querySelector("#fleetFilterToggle"),
  fleetAdvancedFilters: document.querySelector("#fleetAdvancedFilters"),
  bulkCommandType: document.querySelector("#bulkCommandType"),
  bulkCommandTarget: document.querySelector("#bulkCommandTarget"),
  sendBulkCommandBtn: document.querySelector("#sendBulkCommandBtn"),
  fleetGroup: document.querySelector("#fleetGroup"),
  fleetTags: document.querySelector("#fleetTags"),
  saveFleetBtn: document.querySelector("#saveFleetBtn"),
  alertSummary: document.querySelector("#alertSummary"),
  alertList: document.querySelector("#alertList"),
  alertStatusFilter: document.querySelector("#alertStatusFilter"),
  problemDataState: document.querySelector("#problemDataState"),
  metricsHistorySummary: document.querySelector("#metricsHistorySummary"),
  metricsHistory: document.querySelector("#metricsHistory"),
  commandType: document.querySelector("#commandType"),
  commandTarget: document.querySelector("#commandTarget"),
  sendCommandBtn: document.querySelector("#sendCommandBtn"),
  manualCommandStatus: document.querySelector("#manualCommandStatus"),
  manualCommandOutputDetails: document.querySelector("#manualCommandOutputDetails"),
  manualCommandOutput: document.querySelector("#manualCommandOutput"),
  packageCommand: document.querySelector("#packageCommand"),
  packageName: document.querySelector("#packageName"),
  sendPackageCommandBtn: document.querySelector("#sendPackageCommandBtn"),
  packageOperationStatus: document.querySelector("#packageOperationStatus"),
  packageFormMessage: document.querySelector("#packageFormMessage"),
  packageOutputDetails: document.querySelector("#packageOutputDetails"),
  packageTechnicalOutput: document.querySelector("#packageTechnicalOutput"),
  remoteServerHost: document.querySelector("#remoteServerHost"),
  remoteServerPort: document.querySelector("#remoteServerPort"),
  remotePort: document.querySelector("#remotePort"),
  remoteLocalPort: document.querySelector("#remoteLocalPort"),
  remoteLuCIScheme: document.querySelector("#remoteLuCIScheme"),
  remoteDuration: document.querySelector("#remoteDuration"),
  createRemoteSessionBtn: document.querySelector("#createRemoteSessionBtn"),
  openCloudAccessBtn: document.querySelector("#openCloudAccessBtn"),
  cloudAccessCard: document.querySelector("#cloudAccessCard"),
  cloudAccessStatus: document.querySelector("#cloudAccessStatus"),
  cloudAccessState: document.querySelector("#cloudAccessState"),
  remoteSummary: document.querySelector("#remoteSummary"),
  remoteSessionList: document.querySelector("#remoteSessionList"),
  backupList: document.querySelector("#backupList"),
  createBackupBtn: document.querySelector("#createBackupBtn"),
  refreshBackupsBtn: document.querySelector("#refreshBackupsBtn"),
  downloadDatabaseSnapshotBtn: document.querySelector("#downloadDatabaseSnapshotBtn"),
  rotateDeviceCredentialBtn: document.querySelector("#rotateDeviceCredentialBtn"),
  revokeDeviceCredentialBtn: document.querySelector("#revokeDeviceCredentialBtn"),
  credentialActionMessage: document.querySelector("#credentialActionMessage"),
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
  uciOperationStatus: document.querySelector("#uciOperationStatus"),
  uciFormMessage: document.querySelector("#uciFormMessage"),
  uciDiff: document.querySelector("#uciDiff"),
  uciOutputDetails: document.querySelector("#uciOutputDetails"),
  uciTechnicalOutput: document.querySelector("#uciTechnicalOutput"),
  presetLanIp: document.querySelector("#presetLanIp"),
  presetHostname: document.querySelector("#presetHostname"),
  presetWifiSsid: document.querySelector("#presetWifiSsid"),
  presetWifiKey: document.querySelector("#presetWifiKey"),
  presetDhcpLan: document.querySelector("#presetDhcpLan"),
  presetReviewPanel: document.querySelector("#presetReviewPanel"),
  presetReviewTitle: document.querySelector("#presetReviewTitle"),
  presetReviewStatus: document.querySelector("#presetReviewStatus"),
  presetReviewChange: document.querySelector("#presetReviewChange"),
  presetReviewOutput: document.querySelector("#presetReviewOutput"),
  cancelPresetReviewBtn: document.querySelector("#cancelPresetReviewBtn"),
  applyPresetReviewBtn: document.querySelector("#applyPresetReviewBtn"),
  commandSummary: document.querySelector("#commandSummary"),
  commandStatusFilter: document.querySelector("#commandStatusFilter"),
  loadMoreCommandsBtn: document.querySelector("#loadMoreCommandsBtn"),
  loadMoreAuditBtn: document.querySelector("#loadMoreAuditBtn"),
  auditSummary: document.querySelector("#auditSummary"),
  commandList: document.querySelector("#commandList"),
  commandDetailPanel: document.querySelector("#commandDetailPanel"),
  commandDetailMeta: document.querySelector("#commandDetailMeta"),
  commandDetailOutput: document.querySelector("#commandDetailOutput"),
  copyCommandOutputBtn: document.querySelector("#copyCommandOutputBtn"),
  auditList: document.querySelector("#auditList"),
  clearAlertsBtn: document.querySelector("#clearAlertsBtn"),
  clearCommandsBtn: document.querySelector("#clearCommandsBtn"),
  clearAuditBtn: document.querySelector("#clearAuditBtn"),
  deleteDeviceBtn: document.querySelector("#deleteDeviceBtn"),
  deviceTransferForm: document.querySelector("#deviceTransferForm"),
  transferUsername: document.querySelector("#transferUsername"),
  transferPassword: document.querySelector("#transferPassword"),
  transferMessage: document.querySelector("#transferMessage"),
  toastRegion: document.querySelector("#toastRegion"),
  profileDialog: document.querySelector("#profileDialog"),
  closeProfileBtn: document.querySelector("#closeProfileBtn"),
  profileUsername: document.querySelector("#profileUsername"),
  profileIdentityName: document.querySelector("#profileIdentityName"),
  profileRole: document.querySelector("#profileRole"),
  profileEmailState: document.querySelector("#profileEmailState"),
  profileAvatar: document.querySelector("#profileAvatar"),
  profileLogoutBtn: document.querySelector("#profileLogoutBtn"),
  profileForm: document.querySelector("#profileForm"),
  profileDisplayName: document.querySelector("#profileDisplayName"),
  profileEmail: document.querySelector("#profileEmail"),
  profileMessage: document.querySelector("#profileMessage"),
  profileSubmitBtn: document.querySelector("#profileSubmitBtn"),
  profileAdminTab: document.querySelector("#profileAdminTab"),
  profileTabs: [...document.querySelectorAll("[data-profile-tab]")],
  profilePanels: [...document.querySelectorAll("[data-profile-panel]")],
	rolloutForm: document.querySelector("#rolloutForm"),
	rolloutDeviceIds: document.querySelector("#rolloutDeviceIds"),
	rolloutChannel: document.querySelector("#rolloutChannel"),
	rolloutBatchSize: document.querySelector("#rolloutBatchSize"),
	rolloutFailureThreshold: document.querySelector("#rolloutFailureThreshold"),
	rolloutMessage: document.querySelector("#rolloutMessage"),
	rolloutList: document.querySelector("#rolloutList"),
  passwordForm: document.querySelector("#passwordForm"),
  currentPassword: document.querySelector("#currentPassword"),
  newPassword: document.querySelector("#newPassword"),
  confirmPassword: document.querySelector("#confirmPassword"),
  passwordMessage: document.querySelector("#passwordMessage"),
  passwordSubmitBtn: document.querySelector("#passwordSubmitBtn"),
  notificationSettingsForm: document.querySelector("#notificationSettingsForm"),
  notificationEmailChannel: document.querySelector("#notificationEmailChannel"),
  notificationEmailState: document.querySelector("#notificationEmailState"),
  notificationEmailEnabled: document.querySelector("#notificationEmailEnabled"),
  notificationEmailHint: document.querySelector("#notificationEmailHint"),
  notificationTelegramChannel: document.querySelector("#notificationTelegramChannel"),
  notificationTelegramState: document.querySelector("#notificationTelegramState"),
  notificationTelegramEnabled: document.querySelector("#notificationTelegramEnabled"),
  notificationTelegramHint: document.querySelector("#notificationTelegramHint"),
  notificationTelegramChatRow: document.querySelector("#notificationTelegramChatRow"),
  notificationTelegramChatId: document.querySelector("#notificationTelegramChatId"),
  verifyEmailBtn: document.querySelector("#verifyEmailBtn"),
  verifyTelegramBtn: document.querySelector("#verifyTelegramBtn"),
  emailVerificationCode: document.querySelector("#emailVerificationCode"),
  telegramVerificationCode: document.querySelector("#telegramVerificationCode"),
  confirmEmailBtn: document.querySelector("#confirmEmailBtn"),
  confirmTelegramBtn: document.querySelector("#confirmTelegramBtn"),
  notificationWarningEnabled: document.querySelector("#notificationWarningEnabled"),
  notificationCriticalEnabled: document.querySelector("#notificationCriticalEnabled"),
  notificationResolvedEnabled: document.querySelector("#notificationResolvedEnabled"),
  notificationMemoryThreshold: document.querySelector("#notificationMemoryThreshold"),
  notificationDiskThreshold: document.querySelector("#notificationDiskThreshold"),
  notificationPacketLossThreshold: document.querySelector("#notificationPacketLossThreshold"),
  notificationLatencyThreshold: document.querySelector("#notificationLatencyThreshold"),
  notificationRepeatMinutes: document.querySelector("#notificationRepeatMinutes"),
  notificationTimezone: document.querySelector("#notificationTimezone"),
  notificationPausedUntil: document.querySelector("#notificationPausedUntil"),
  notificationQuietEnabled: document.querySelector("#notificationQuietEnabled"),
  notificationQuietStart: document.querySelector("#notificationQuietStart"),
  notificationQuietEnd: document.querySelector("#notificationQuietEnd"),
  notificationWebhookEnabled: document.querySelector("#notificationWebhookEnabled"),
  notificationWebhookUrl: document.querySelector("#notificationWebhookUrl"),
  notificationWebhookSecret: document.querySelector("#notificationWebhookSecret"),
  notificationDeviceSelect: document.querySelector("#notificationDeviceSelect"),
  notificationDeviceEnabled: document.querySelector("#notificationDeviceEnabled"),
  notificationDeviceCritical: document.querySelector("#notificationDeviceCritical"),
  notificationDeviceWarning: document.querySelector("#notificationDeviceWarning"),
  notificationDeviceResolved: document.querySelector("#notificationDeviceResolved"),
  notificationDevicePausedUntil: document.querySelector("#notificationDevicePausedUntil"),
  saveDeviceNotificationSettingsBtn: document.querySelector("#saveDeviceNotificationSettingsBtn"),
  notificationSettingsMessage: document.querySelector("#notificationSettingsMessage"),
  testNotificationsBtn: document.querySelector("#testNotificationsBtn"),
  notificationSubmitBtn: document.querySelector("#notificationSubmitBtn"),
  refreshNotificationsBtn: document.querySelector("#refreshNotificationsBtn"),
  notificationMetrics: document.querySelector("#notificationMetrics"),
  notificationChannelDiagnostics: document.querySelector("#notificationChannelDiagnostics"),
  notificationFilterDevice: document.querySelector("#notificationFilterDevice"),
  notificationFilterSeverity: document.querySelector("#notificationFilterSeverity"),
  notificationFilterEvent: document.querySelector("#notificationFilterEvent"),
  notificationFilterChannel: document.querySelector("#notificationFilterChannel"),
  notificationFilterStatus: document.querySelector("#notificationFilterStatus"),
  clearNotificationFiltersBtn: document.querySelector("#clearNotificationFiltersBtn"),
  notificationHistory: document.querySelector("#notificationHistory"),
  notificationCenterBtn: document.querySelector("#notificationCenterBtn"),
  notificationUnreadCount: document.querySelector("#notificationUnreadCount"),
  mobileNotificationUnreadCount: document.querySelector("#mobileNotificationUnreadCount"),
  notificationCenterDialog: document.querySelector("#notificationCenterDialog"),
  closeNotificationCenterBtn: document.querySelector("#closeNotificationCenterBtn"),
  markAllNotificationsReadBtn: document.querySelector("#markAllNotificationsReadBtn"),
  notificationCenterSummary: document.querySelector("#notificationCenterSummary"),
  notificationCenterList: document.querySelector("#notificationCenterList"),
  openCreateUserBtn: document.querySelector("#openCreateUserBtn"),
  confirmationDialog: document.querySelector("#confirmationDialog"),
  confirmationForm: document.querySelector("#confirmationForm"),
  confirmationContext: document.querySelector("#confirmationContext"),
  confirmationVariantLabel: document.querySelector("#confirmationVariantLabel"),
  confirmationCloseBtn: document.querySelector("#confirmationCloseBtn"),
  confirmationMarker: document.querySelector("#confirmationMarker"),
  confirmationTitle: document.querySelector("#confirmationTitle"),
  confirmationMessage: document.querySelector("#confirmationMessage"),
  confirmationDescription: document.querySelector("#confirmationDescription"),
  confirmationValues: document.querySelector("#confirmationValues"),
  confirmationInputGroup: document.querySelector("#confirmationInputGroup"),
  confirmationInputLabel: document.querySelector("#confirmationInputLabel"),
  confirmationInput: document.querySelector("#confirmationInput"),
  confirmationInputHint: document.querySelector("#confirmationInputHint"),
  confirmationTechnicalDetails: document.querySelector("#confirmationTechnicalDetails"),
  confirmationTechnicalOutput: document.querySelector("#confirmationTechnicalOutput"),
  confirmationError: document.querySelector("#confirmationError"),
  confirmationCancelBtn: document.querySelector("#confirmationCancelBtn"),
  confirmationConfirmBtn: document.querySelector("#confirmationConfirmBtn"),
  logoutAllBtn: document.querySelector("#logoutAllBtn"),
  userManagementSection: document.querySelector("#userManagementSection"),
  userList: document.querySelector("#userList"),
  luciStateDialog: document.querySelector("#luciStateDialog"),
  luciStateCode: document.querySelector("#luciStateCode"),
  luciStateTitle: document.querySelector("#luciStateTitle"),
  luciStateDescription: document.querySelector("#luciStateDescription"),
  luciStateContext: document.querySelector("#luciStateContext"),
  luciStatePrimaryBtn: document.querySelector("#luciStatePrimaryBtn"),
  luciStateDiagnosticBtn: document.querySelector("#luciStateDiagnosticBtn"),
  closeLuciStateBtn: document.querySelector("#closeLuciStateBtn"),
  luciStateRequestId: document.querySelector("#luciStateRequestId"),
  systemStateDialog: document.querySelector("#systemStateDialog"),
  systemStateSheet: document.querySelector("#systemStateSheet"),
  systemStateCode: document.querySelector("#systemStateCode"),
  systemStateSymbol: document.querySelector("#systemStateSymbol"),
  systemStateIconUse: document.querySelector("#systemStateIconUse"),
  systemStateTitle: document.querySelector("#systemStateTitle"),
  systemStateDescription: document.querySelector("#systemStateDescription"),
  systemStateContext: document.querySelector("#systemStateContext"),
  systemStatePrimaryBtn: document.querySelector("#systemStatePrimaryBtn"),
  systemStateBackBtn: document.querySelector("#systemStateBackBtn"),
  systemStateCopyBtn: document.querySelector("#systemStateCopyBtn"),
  closeSystemStateBtn: document.querySelector("#closeSystemStateBtn"),
};

if (els.remoteServerHost) {
  els.remoteServerHost.value = window.location.hostname || "10.10.10.2";
}

function ensureToastRegion() {
  if (els.toastRegion) return els.toastRegion;
  const region = document.createElement("div");
  region.id = "toastRegion";
  region.className = "toast-region";
  region.setAttribute("aria-live", "polite");
  region.setAttribute("aria-atomic", "true");
  document.body.appendChild(region);
  els.toastRegion = region;
  return region;
}

function showToast(message, tone = "info") {
  if (!message) return;
  const region = ensureToastRegion();
  const toast = document.createElement("div");
  toast.className = `toast ${tone}`;
  toast.textContent = message;
  region.appendChild(toast);
  window.setTimeout(() => {
    toast.classList.add("is-leaving");
    window.setTimeout(() => toast.remove(), 180);
  }, 2800);
}

function setStatus(message) {
  els.statusLine.textContent = message;
}

function notify(message, tone = "info") {
  setStatus(message);
  showToast(message, tone);
}

function reportError(error) {
  const message = error instanceof Error ? error.message : String(error || "Unexpected error");
  setStatus(message);
  showToast(message, "error");
  if (error && error.status === 403) {
    showSystemState(error, "access");
    return;
  }
  if (!error || !error.status) {
    showSystemState(error, "backend");
    return;
  }
  if (error.status >= 500) showSystemState(error, "generic");
}

function setSystemStateContext(entries) {
  els.systemStateContext.replaceChildren();
  for (const [label, value] of entries) {
    if (!value) continue;
    const row = document.createElement("div");
    const term = document.createElement("dt");
    const description = document.createElement("dd");
    term.textContent = label;
    description.textContent = value;
    row.append(term, description);
    els.systemStateContext.appendChild(row);
  }
  els.systemStateContext.classList.toggle("is-hidden", els.systemStateContext.childElementCount === 0);
}

function showSystemState(error, kind = "generic", retry = null) {
  const requestId = error && error.requestId ? error.requestId : "";
  const message = error instanceof Error ? error.message : String(error || "");
  const variants = {
    access: {
      tone: "warning",
      code: "403 · ACCESS DENIED",
      icon: "shield-lock",
      title: "Недостаточно прав",
      description: "У вашей учётной записи нет доступа к этому действию или объекту.",
      primary: "К объектам",
      retry: () => showFleet(),
    },
    backend: {
      tone: "error",
      code: "503 · BACKEND UNAVAILABLE",
      icon: "server-off",
      title: "Сервер недоступен",
      description: "Интерфейс не может связаться с OpenWrt RMM. Последние показанные данные могут быть устаревшими.",
      primary: "Повторить подключение",
      retry: retry || (() => (els.appShell.classList.contains("is-hidden") ? checkSession() : refreshDevicesIfIdle())),
    },
    generic: {
      tone: "error",
      code: "ERROR · REQUEST FAILED",
      icon: "alert-circle",
      title: "Не удалось выполнить запрос",
      description: "Повторите действие. Если ошибка сохранится, сообщите идентификатор запроса администратору.",
      primary: "Повторить",
      retry: retry || (() => refreshDevicesIfIdle()),
    },
  };
  const variant = variants[kind] || variants.generic;
  state.systemStateRetry = variant.retry;
  els.systemStateSheet.dataset.tone = variant.tone;
  els.systemStateCode.textContent = variant.code;
  els.systemStateIconUse.setAttribute("href", `${iconSpriteURL}#icon-${variant.icon}`);
  els.systemStateTitle.textContent = variant.title;
  els.systemStateDescription.textContent = variant.description;
  els.systemStatePrimaryBtn.textContent = variant.primary;
  els.systemStateCopyBtn.classList.toggle("is-hidden", !requestId);
  setSystemStateContext([
    ["Сообщение", message],
    ["ID запроса", requestId],
    ["Последнее обновление", state.lastUpdatedAt ? formatDate(state.lastUpdatedAt) : ""],
  ]);
  if (!els.systemStateDialog.open) els.systemStateDialog.showModal();
}

function setLiveState(mode, label) {
  state.liveConnected = mode === "live";
  els.liveState.className = `live-state is-${mode}`;
  els.liveState.querySelector("span").textContent = label;
}

function lastUpdatedLabel() {
  if (!state.lastUpdatedAt) return state.liveConnected ? "Живые данные" : "Ожидание данных";
  const elapsed = Math.max(0, Math.floor((Date.now() - state.lastUpdatedAt.getTime()) / 1000));
  if (elapsed < 10) return state.liveConnected ? "Живые данные · сейчас" : "Polling · сейчас";
  if (elapsed < 60) return `${state.liveConnected ? "Живые данные" : "Polling"} · ${elapsed} сек.`;
  return `${state.liveConnected ? "Живые данные" : "Polling"} · ${Math.floor(elapsed / 60)} мин.`;
}

function updateLiveStateLabel() {
  if (els.appShell.classList.contains("is-hidden")) return;
  setLiveState(state.liveConnected ? "live" : "polling", lastUpdatedLabel());
}

function disconnectLiveUpdates() {
  if (eventSource) eventSource.close();
  if (liveRefreshTimer) window.clearTimeout(liveRefreshTimer);
  eventSource = null;
  liveRefreshTimer = null;
  state.liveConnected = false;
}

function scheduleLiveRefresh() {
  if (liveRefreshTimer) window.clearTimeout(liveRefreshTimer);
  liveRefreshTimer = window.setTimeout(() => {
    liveRefreshTimer = null;
    refreshDevicesIfIdle().catch(() => setLiveState("polling", "Polling · 30 сек."));
  }, 750);
}

function connectLiveUpdates() {
  disconnectLiveUpdates();
  if (!("EventSource" in window) || els.appShell.classList.contains("is-hidden")) {
    setLiveState("polling", "Polling · 30 сек.");
    return;
  }
  setLiveState("connecting", "Подключение");
  eventSource = new EventSource("/api/events");
  eventSource.addEventListener("ready", () => {
    state.liveConnected = true;
    setLiveState("live", lastUpdatedLabel());
  });
  eventSource.addEventListener("devices", () => {
    if (document.visibilityState !== "visible") return;
    scheduleLiveRefresh();
  });
  eventSource.addEventListener("notifications", () => {
    if (document.visibilityState !== "visible") return;
    loadNotificationCenter().catch(() => {});
    if (els.profileDialog.open && state.profileTab === "notifications") {
      loadNotificationHistory().catch(() => {});
    }
  });
  eventSource.onerror = () => {
    state.liveConnected = false;
    setLiveState("reconnecting", "Переподключение · polling");
  };
}

function inlineStateMarkup(title, description = "", tone = "neutral") {
  return `
    <div class="inline-state ${tone}">
      <strong>${escapeHtml(title)}</strong>
      ${description ? `<small>${escapeHtml(description)}</small>` : ""}
    </div>
  `;
}

const iconRegistry = new Set([
  "activity-heartbeat", "alert-circle", "alert-triangle", "arrow-down", "arrow-left", "arrow-up", "arrows-exchange", "arrows-sort",
  "bell", "brand-telegram", "check", "circle-check", "clock", "cloud-lock", "copy",
  "error-404", "external-link", "key", "layout-grid", "loader-2", "logout", "mail",
  "network", "refresh", "router", "server-off", "shield-check", "shield-lock", "terminal-2",
  "user", "users", "wifi", "x",
]);
const iconSpriteURL = "/assets/icons/tabler-sprite.svg";

function icon(name, className = "") {
  const resolved = iconRegistry.has(name) ? name : "alert-circle";
  const classes = ["icon", "icon-md", className].filter(Boolean).join(" ");
  return `<svg class="${classes}" aria-hidden="true"><use href="${iconSpriteURL}#icon-${resolved}"></use></svg>`;
}

function confirmationErrorMessage(error, fallback = "Не удалось выполнить действие.") {
  if (error && error.status === 403) return "Недостаточно прав для выполнения этого действия.";
  if (error && error.status === 409) return "Состояние объекта изменилось. Проверьте данные и повторите действие.";
  if (error && error.status >= 500) return "Сервер временно не смог выполнить действие.";
  return fallback;
}

function setConfirmationValues(entries = []) {
  els.confirmationValues.replaceChildren();
  for (const entry of entries) {
    const [label, value, tone = ""] = Array.isArray(entry)
      ? entry
      : [entry.label, entry.value, entry.tone || ""];
    if (value === undefined || value === null || value === "") continue;
    const row = document.createElement("div");
    if (tone) row.dataset.tone = tone;
    const term = document.createElement("dt");
    const description = document.createElement("dd");
    term.textContent = label;
    description.textContent = String(value);
    row.append(term, description);
    els.confirmationValues.appendChild(row);
  }
  els.confirmationValues.classList.toggle("is-hidden", els.confirmationValues.childElementCount === 0);
}

function finishConfirmation(result) {
  const request = state.confirmationRequest;
  if (!request) return;
  state.confirmationRequest = null;
  els.confirmationForm.classList.remove("is-submitting");
  if (els.confirmationDialog.open) els.confirmationDialog.close();
  request.resolve(result);
  window.requestAnimationFrame(() => {
    if (request.invoker && request.invoker.isConnected) request.invoker.focus();
  });
}

function cancelConfirmation() {
  const request = state.confirmationRequest;
  if (!request || request.submitting || request.options.allowCancel === false) return;
  finishConfirmation(false);
}

function confirmationInputValue(request) {
  if (!request.options.input) return true;
  const value = els.confirmationInput.value;
  const input = request.options.input;
  if (input.required !== false && !value) return input.requiredMessage || "Заполните поле подтверждения.";
  if (input.minLength && value.length < input.minLength) return input.validationMessage || `Минимум ${input.minLength} символов.`;
  if (input.expected !== undefined && value !== String(input.expected)) return input.validationMessage || "Значение не совпадает с подтверждаемым объектом.";
  if (typeof input.validate === "function") return input.validate(value) || true;
  return true;
}

async function submitConfirmation() {
  const request = state.confirmationRequest;
  if (!request || request.submitting || request.options.disabled) return;
  const validation = confirmationInputValue(request);
  if (validation !== true) {
    els.confirmationError.textContent = validation;
    els.confirmationInput.setAttribute("aria-invalid", "true");
    els.confirmationInput.focus();
    return;
  }
  els.confirmationInput.removeAttribute("aria-invalid");
  els.confirmationError.textContent = "";
  const result = request.options.input ? els.confirmationInput.value : true;
  if (typeof request.options.onConfirm !== "function") {
    finishConfirmation(result);
    return;
  }
  request.submitting = true;
  els.confirmationForm.classList.add("is-submitting");
  els.confirmationConfirmBtn.disabled = true;
  els.confirmationCancelBtn.disabled = true;
  els.confirmationCloseBtn.disabled = true;
  els.confirmationConfirmBtn.textContent = request.options.loadingLabel || "Выполняется…";
  try {
    await request.options.onConfirm(result);
    finishConfirmation(result);
  } catch (error) {
    request.submitting = false;
    els.confirmationForm.classList.remove("is-submitting");
    els.confirmationConfirmBtn.disabled = false;
    els.confirmationCancelBtn.disabled = false;
    els.confirmationCloseBtn.disabled = false;
    els.confirmationConfirmBtn.textContent = request.options.confirmLabel || "Продолжить";
    els.confirmationError.textContent = confirmationErrorMessage(error, request.options.failureMessage);
    els.confirmationTechnicalOutput.textContent = String(error && error.message ? error.message : error || "Unknown error");
    els.confirmationTechnicalDetails.classList.remove("is-hidden");
  }
}

function confirmAction(options = {}) {
  if (!els.confirmationDialog || typeof els.confirmationDialog.showModal !== "function") {
    return Promise.resolve(false);
  }
  if (state.confirmationRequest) finishConfirmation(false);
  const variant = ["neutral", "warning", "danger"].includes(options.variant) ? options.variant : "neutral";
  const invoker = document.activeElement;
  els.confirmationDialog.dataset.variant = variant;
  els.confirmationContext.textContent = options.context || "SYSTEM / ACTION";
  els.confirmationVariantLabel.textContent = variant === "danger" ? "DESTRUCTIVE ACTION" : variant === "warning" ? "REVIEW REQUIRED" : "CONFIRMATION";
  els.confirmationMarker.innerHTML = icon(variant === "neutral" ? "terminal-2" : "alert-triangle");
  els.confirmationTitle.textContent = options.title || "Подтвердить действие";
  els.confirmationMessage.textContent = options.message || "Проверьте параметры перед продолжением.";
  els.confirmationDescription.textContent = options.description || "";
  els.confirmationDescription.classList.toggle("is-hidden", !options.description);
  setConfirmationValues(options.values || []);
  els.confirmationInputGroup.classList.toggle("is-hidden", !options.input);
  els.confirmationInput.value = options.input && options.input.value ? options.input.value : "";
  els.confirmationInput.type = options.input && options.input.type ? options.input.type : "text";
  els.confirmationInputLabel.textContent = options.input && options.input.label ? options.input.label : "Значение";
  els.confirmationInputHint.textContent = options.input && options.input.hint ? options.input.hint : "";
  els.confirmationInput.placeholder = options.input && options.input.placeholder ? options.input.placeholder : "";
  els.confirmationInput.autocomplete = options.input && options.input.autocomplete ? options.input.autocomplete : "off";
  els.confirmationInput.removeAttribute("aria-invalid");
  els.confirmationError.textContent = "";
  els.confirmationTechnicalDetails.classList.add("is-hidden");
  els.confirmationTechnicalDetails.open = false;
  els.confirmationTechnicalOutput.textContent = options.technicalDetails || "";
  if (options.technicalDetails) els.confirmationTechnicalDetails.classList.remove("is-hidden");
  els.confirmationCancelBtn.textContent = options.cancelLabel || "Отмена";
  els.confirmationConfirmBtn.textContent = options.confirmLabel || "Продолжить";
  els.confirmationConfirmBtn.className = variant === "danger" ? "danger" : variant === "warning" ? "warning" : "primary";
  els.confirmationConfirmBtn.disabled = Boolean(options.disabled);
  els.confirmationCancelBtn.disabled = false;
  els.confirmationCloseBtn.disabled = false;
  return new Promise((resolve) => {
    state.confirmationRequest = { options, resolve, invoker, submitting: false };
    els.confirmationDialog.showModal();
    window.requestAnimationFrame(() => {
      if (options.input) els.confirmationInput.focus();
      else if (options.initialFocus === "confirm") els.confirmationConfirmBtn.focus();
      else els.confirmationCancelBtn.focus();
    });
  });
}

function setDataContextState(element, tone = "", title = "", description = "") {
  if (!element) return;
  element.className = `data-context-state${tone ? ` is-${tone}` : " is-hidden"}`;
  element.innerHTML = tone
    ? `<span>${icon(tone === "offline" || tone === "error" ? "alert-triangle" : "clock")}</span><div><strong>${escapeHtml(title)}</strong>${description ? `<small>${escapeHtml(description)}</small>` : ""}</div>`
    : "";
}

function ageInMilliseconds(value) {
  if (!value) return 0;
  const timestamp = new Date(value).getTime();
  return Number.isFinite(timestamp) ? Math.max(0, Date.now() - timestamp) : 0;
}

function formatDuration(fromValue, toValue = new Date()) {
  const from = new Date(fromValue).getTime();
  const to = new Date(toValue || Date.now()).getTime();
  if (!Number.isFinite(from) || !Number.isFinite(to)) return "-";
  const seconds = Math.max(0, Math.floor((to - from) / 1000));
  if (seconds < 60) return `${seconds} сек.`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} мин.`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} ч. ${minutes % 60} мин.`;
  return `${Math.floor(hours / 24)} дн. ${hours % 24} ч.`;
}

async function api(path, options = {}) {
  const headers = {
    ...(options.body ? { "Content-Type": "application/json" } : {}),
    ...(options.headers || {}),
  };
  let response;
  try {
    response = await fetch(path, { ...options, credentials: "same-origin", headers });
  } catch {
    const error = new Error("Нет соединения с OpenWrt RMM. Проверьте сеть и повторите попытку.");
    error.status = 0;
    error.code = "network_error";
    throw error;
  }
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
      showLogin("Сессия истекла. Войдите снова.");
    }
    const rawMessage = data && data.error ? data.error : `HTTP ${response.status}`;
    const message = friendlyAPIError(response.status, data && data.code, rawMessage);
    const suffix = requestId ? ` (${requestId})` : "";
    const error = new Error(`${message}${suffix}`);
    error.status = response.status;
    error.requestId = requestId || "";
    error.code = data && data.code ? data.code : "";
    throw error;
  }
  return data;
}

function friendlyAPIError(status, code, rawMessage) {
  const byCode = {
    device_offline: "Роутер не на связи",
    not_configured: "Облачный доступ ещё не настроен",
    grant_limit: "Создано слишком много временных ссылок. Подождите минуту",
    restart_failed: "Не удалось перезапустить защищённый туннель",
    create_failed: "Не удалось создать защищённый туннель",
  };
  if (code && byCode[code]) return byCode[code];
  const known = {
    "invalid username or password": "Неверный логин или пароль",
    "invalid current password": "Текущий пароль указан неверно",
    "device not found": "Роутер не найден или у вас нет к нему доступа",
    "invalid or expired enrollment grant": "Код подключения недействителен или уже использован",
    "active LuCI access grant limit reached": "Создано слишком много временных ссылок. Подождите минуту",
    "agent update is unavailable": "Управляемое обновление агента сейчас недоступно",
    "device does not support managed agent updates": "Этот роутер не поддерживает управляемое обновление агента",
    "no compatible immutable agent feed is available": "Для этого роутера нет совместимой версии агента",
    "failed to queue agent update": "Не удалось добавить обновление агента в очередь",
  };
  if (known[rawMessage]) return known[rawMessage];
  if (String(rawMessage).startsWith("failed to ")) {
    return status >= 500 ? "Сервер временно не смог выполнить запрос" : "Не удалось выполнить запрос";
  }
  return rawMessage;
}

function showLogin(message = "") {
  disconnectLiveUpdates();
  document.title = "Вход — OpenWrt RMM";
  els.appShell.classList.add("is-hidden");
  els.appShell.hidden = true;
  els.appShell.setAttribute("aria-hidden", "true");
  els.loginView.classList.remove("is-hidden");
  els.loginView.hidden = false;
  els.loginView.removeAttribute("aria-hidden");
  els.loginError.textContent = message;
  els.loginUsername.setAttribute("aria-invalid", String(Boolean(message)));
  els.loginPassword.setAttribute("aria-invalid", String(Boolean(message)));
  els.loginPassword.value = "";
  if (location.pathname === "/app") history.replaceState(null, "", `/login${location.hash}`);
  if (message) document.querySelector("#login")?.scrollIntoView({ behavior: "smooth", block: "center" });
}

function showApp(user) {
  document.title = "Кабинет — OpenWrt RMM";
  state.user = user || null;
  state.username = user && user.username ? user.username : "operator";
  const accountName = user && user.display_name ? user.display_name : state.username;
  els.operatorName.textContent = accountName;
  els.profileUsername.textContent = state.username;
  els.profileIdentityName.textContent = accountName;
  els.profileRole.textContent = user && user.role === "admin" ? "Администратор" : "Пользователь";
  els.profileRole.className = `status ${user && user.role === "admin" ? "info" : "neutral"}`;
  els.profileEmailState.textContent = user && user.email ? user.email : "E-mail не задан";
  const initial = accountName.trim().charAt(0).toUpperCase() || "О";
  els.profileAvatar.textContent = initial;
  document.querySelector(".operator-avatar").textContent = initial;
  els.addUserBtn.classList.toggle("is-hidden", !user || user.role !== "admin");
  els.profileAdminTab.classList.toggle("is-hidden", !user || user.role !== "admin");
  els.rollbackAgentBtn.classList.toggle("is-hidden", !user || user.role !== "admin");
  for (const element of document.querySelectorAll(".admin-only")) {
    element.classList.toggle("is-hidden", !user || user.role !== "admin");
  }
  els.loginView.classList.add("is-hidden");
  els.loginView.hidden = true;
  els.loginView.setAttribute("aria-hidden", "true");
  els.appShell.classList.remove("is-hidden");
  els.appShell.hidden = false;
  els.appShell.removeAttribute("aria-hidden");
  els.loginError.textContent = "";
  els.loginUsername.setAttribute("aria-invalid", "false");
  els.loginPassword.setAttribute("aria-invalid", "false");
  if (location.pathname !== "/app") history.replaceState(null, "", `/app${location.hash}`);
  connectLiveUpdates();
  loadNotificationCenter().catch(() => {});
}

function formatLoadAverage(value) {
  const values = String(value || "").trim().split(/\s+/).slice(0, 3).map(Number);
  if (values.length !== 3 || values.some((item) => !Number.isFinite(item))) return "-";
  return values.map((item) => item.toFixed(2)).join(" · ");
}

function formatUptime(value) {
  const seconds = Number.parseFloat(String(value || "").trim().split(/\s+/)[0]);
  if (!Number.isFinite(seconds) || seconds < 0) return "-";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days} дн. ${hours} ч.`;
  if (hours > 0) return `${hours} ч. ${minutes} мин.`;
  return `${minutes} мин.`;
}

async function checkSession() {
  try {
    const me = await api("/api/auth/me");
    showApp(me.user || { username: me.username, role: "user" });
    await loadReleaseMetadata();
    await loadDevices();
  } catch (error) {
    if (error && error.status === 401) {
      showLogin();
      return;
    }
    showLogin("Сервер OpenWrt RMM временно недоступен.");
    showSystemState(error, "backend", checkSession);
  }
}

async function login() {
  els.loginError.textContent = "";
  setSubmitting(els.loginForm, els.loginSubmitBtn, true, "Вход…");
  try {
    const response = await api("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({
        username: els.loginUsername.value.trim(),
        password: els.loginPassword.value,
      }),
    });
    showApp(response.user || { username: response.username, role: "user" });
    await loadReleaseMetadata();
    await loadDevices();
  } finally {
    setSubmitting(els.loginForm, els.loginSubmitBtn, false);
  }
}

async function logout() {
  await api("/api/auth/logout", { method: "POST" });
  state.devices = [];
  state.user = null;
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
    requested: "Запрошено",
    queued: "В очереди",
    active: "Активно",
    closed: "Закрыто",
    claimed: "Выполняется",
    completed: "Готово",
    failed: "Ошибка",
    cancelled: "Отменено",
    expired: "Истекло",
    pending: "Ожидает batch",
    waiting_reconnect: "Ожидает reconnect",
    running: "Выполняется",
    paused: "Приостановлено",
  }[status] || status || "-";
}

function commandTypeLabel(type) {
  return {
    ping: "Проверка связи",
    traceroute: "Маршрут до узла",
    route_show: "Показ маршрутов",
    interfaces_show: "Показ интерфейсов",
    reboot: "Перезагрузка",
    service_restart: "Перезапуск сервиса",
    pkg_list_installed: "Список пакетов",
    pkg_update: "Обновление индекса пакетов",
    pkg_list_upgradable: "Доступные обновления пакетов",
    pkg_install: "Установка пакета",
    pkg_remove: "Удаление пакета",
    opkg_list_installed: "Список пакетов",
    opkg_update: "Обновление индекса пакетов",
    opkg_list_upgradable: "Доступные обновления пакетов",
    opkg_install: "Установка пакета",
    opkg_remove: "Удаление пакета",
    uci_show: "Просмотр UCI",
    uci_backup: "Резервная копия UCI",
    uci_preview: "Проверка изменения настроек",
    uci_set: "Подготовка изменения настроек",
    uci_commit: "Применение настроек",
    uci_commit_confirmed: "Безопасное применение настроек",
    uci_revert: "Отмена staged-настроек",
    uci_restore: "Восстановление настроек",
    remote_ssh_reverse: "Открытие удаленного доступа",
    remote_ssh_close: "Закрытие удаленного доступа",
    agent_update: "Обновление агента",
    agent_rollback: "Откат агента",
  }[type] || type || "-";
}

function commandFilterMatches(command) {
  if (state.commandStatusFilter && command.status !== state.commandStatusFilter) return false;
  if (state.commandFilter === "running") return command.status === "queued" || command.status === "claimed";
  if (state.commandFilter === "failed") return ["failed", "expired", "cancelled"].includes(command.status);
  if (state.commandFilter === "completed") return command.status === "completed";
  return true;
}

function commandStatusCode(status) {
  return {
    requested: "QUEUED",
    queued: "QUEUED",
    claimed: "RUNNING",
    running: "RUNNING",
    completed: "COMPLETED",
    failed: "FAILED",
    cancelled: "CANCELLED",
    timeout: "TIMEOUT",
    expired: "EXPIRED",
  }[status] || String(status || "UNKNOWN").toUpperCase();
}

function remoteStatusLabel(status) {
  return {
    requested: "CREATING",
    queued: "CREATING",
    creating: "CREATING",
    active: "ACTIVE",
    expiring: "EXPIRING",
    closed: "CLOSED",
    failed: "FAILED",
    expired: "EXPIRED",
  }[status] || String(status || "UNKNOWN").toUpperCase();
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
    uci_revert: "отменить staged-конфигурацию",
    uci_restore: "восстановить конфигурацию из backup",
  }[type] || "";
}

async function confirmDanger(type, args = {}, onConfirm = null) {
  const label = dangerLabel(type);
  if (!label) return true;
  const device = currentDevice();
  const isPackage = type === "pkg_remove" || type === "opkg_remove";
  const isUCI = type.startsWith("uci_");
  const isReboot = type === "reboot";
  const variant = isPackage || isReboot || type === "uci_restore" ? "danger" : "warning";
  const context = isPackage ? "SYSTEM / PACKAGE" : isUCI ? "CONFIGURATION / UCI" : "SYSTEM / DEVICE";
  const target = isPackage ? args.package : isUCI ? args.config : deviceDisplayName(device);
  const path = isUCI && args.section && args.option ? `${args.config}.${args.section}.${args.option}` : "";
  return Boolean(await confirmAction({
    variant,
    context,
    title: isPackage ? `Удалить package «${args.package || "-"}»?` : isReboot ? `Перезагрузить «${deviceDisplayName(device)}»?` : `${commandTypeLabel(type)}?`,
    message: isPackage
      ? "Package будет удалён package manager устройства. Зависимые компоненты могут перестать работать."
      : isReboot
        ? "Роутер временно пропадёт из мониторинга и станет недоступен до завершения загрузки."
        : type === "uci_restore"
          ? "Сохранённая конфигурация будет восстановлена из backup и может изменить сетевое подключение."
          : "Команда изменит low-level конфигурацию OpenWrt. Проверьте объект перед постановкой в очередь.",
    description: isUCI ? "После commit соединение с устройством может быть временно потеряно." : "Действие будет записано в Audit и Operations.",
    values: [
      [isPackage ? "Package" : isUCI ? "Package" : "Device", target],
      ["Configuration path", path],
      ["New value", isUCI && args.value !== undefined ? args.value : ""],
      ["Command", type],
    ],
    confirmLabel: label.charAt(0).toUpperCase() + label.slice(1),
    loadingLabel: "Добавление в очередь…",
    onConfirm,
    failureMessage: "Не удалось добавить операцию в очередь.",
  }));
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

function deviceAgentVersion(device) {
  return device && device.inventory && device.inventory.agent_version ? String(device.inventory.agent_version) : "";
}

function supportsManagedAgentUpdate(device) {
  const inventory = device && device.inventory;
  return inventory && inventory.agent_runtime === "go" && inventory.agent_package === "rmm-agent-go-production";
}

function hasAgentUpdateAvailable(device) {
  return supportsManagedAgentUpdate(device) && agentVersionState(deviceAgentVersion(device)).comparison === -1;
}

function stableAgentVersion() {
  return state.releaseMetadata && state.releaseMetadata.stable_agent_version
    ? String(state.releaseMetadata.stable_agent_version)
    : "";
}

function agentVersionState(version) {
  const stable = stableAgentVersion();
  if (!version || !stable || !globalThis.RMMVersions) return { comparison: null, stable };
  return { comparison: globalThis.RMMVersions.compareSemVer(version, stable), stable };
}

function agentVersionLabel(version) {
  if (!version) return "Версия не определена";
  const versionState = agentVersionState(version);
  if (versionState.comparison === -1) return `${version} · доступна ${versionState.stable}`;
  if (versionState.comparison === 0) return `${version} · актуальная`;
  if (versionState.comparison === 1) return `${version} · новее стабильной ${versionState.stable}`;
  return `${version} · стабильная версия не определена`;
}

async function loadReleaseMetadata(force = false) {
  if (!force && Date.now() - state.releaseMetadataCheckedAt < 5 * 60 * 1000) return;
  state.releaseMetadataCheckedAt = Date.now();
  try {
    state.releaseMetadata = await api("/api/meta");
  } catch {
    state.releaseMetadata = null;
  }
  const version = state.releaseMetadata && state.releaseMetadata.server_version
    ? String(state.releaseMetadata.server_version)
    : "";
  els.serverVersion.textContent = version ? ` · сервер ${version}` : "";
  els.serverVersion.title = version ? `Версия сервера: ${version}` : "";
}

function deviceClientCount(device) {
  const clients = normalizedClients(device);
  const online = clients.filter((client) => client.presence === "online").length;
  const unconfirmed = clients.length - online;
  return { online, unconfirmed, total: clients.length };
}

function deviceWanSummary(device) {
  const wanIP = device.inventory && device.inventory.wan_ip;
  const checks = Array.isArray(device.metrics && device.metrics.connectivity_checks) ? device.metrics.connectivity_checks : [];
  if (!device.online) return { label: "Нет данных", className: "offline" };
  if (checks.some((check) => !check.reachable)) return { label: wanIP || "Есть потери связи", className: "warning" };
  return { label: wanIP || "Подключено", className: "online" };
}

function getFleetSortValue(device, key) {
  switch (key) {
    case "status":
      return device.online ? 1 : 0;
    case "name":
      return deviceDisplayName(device).toLowerCase();
    case "model":
      return `${deviceModel(device)} ${device.group || ""}`.toLowerCase();
    case "wan":
      return deviceWanSummary(device).label.toLowerCase();
    case "clients":
      return deviceClientCount(device).total;
    case "alerts":
      return Number(device.active_alerts || 0);
    case "last_seen":
      return Date.parse(device.last_seen_at || "") || 0;
    default:
      return deviceDisplayName(device).toLowerCase();
  }
}

function sortDevices(devices) {
  const direction = state.fleetSortDir === "asc" ? 1 : -1;
  return [...devices].sort((left, right) => {
    const leftValue = getFleetSortValue(left, state.fleetSortKey);
    const rightValue = getFleetSortValue(right, state.fleetSortKey);
    if (leftValue === rightValue) {
      return deviceDisplayName(left).localeCompare(deviceDisplayName(right), undefined, { sensitivity: "base" }) * direction;
    }
    if (typeof leftValue === "number" && typeof rightValue === "number") {
      return (leftValue - rightValue) * direction;
    }
    return String(leftValue).localeCompare(String(rightValue), undefined, { numeric: true, sensitivity: "base" }) * direction;
  });
}

function renderFleetSortButtons() {
  for (const button of document.querySelectorAll(".table-sort")) {
    const isActive = button.dataset.sortKey === state.fleetSortKey;
    button.classList.toggle("is-active", isActive);
    button.dataset.sortDir = isActive ? state.fleetSortDir : button.dataset.sortDefaultDir || "asc";
    const iconUse = button.querySelector("use");
    if (iconUse) iconUse.setAttribute("href", `${iconSpriteURL}#icon-${isActive ? state.fleetSortDir === "asc" ? "arrow-up" : "arrow-down" : "arrows-sort"}`);
  }
}

function initFleetTableSorting() {
  const head = document.querySelector(".fleet-table-head");
  if (!head || head.querySelector(".table-sort")) return;
  const labels = [
    ["status", "desc"],
    ["name", "asc"],
    ["model", "asc"],
    ["wan", "asc"],
    ["clients", "desc"],
    ["alerts", "desc"],
    ["last_seen", "desc"],
  ];
  const cells = Array.from(head.children);
  cells.forEach((cell, index) => {
    if (index >= labels.length) return;
    const [sortKey, defaultDir] = labels[index];
    const button = document.createElement("button");
    button.type = "button";
    button.className = "table-sort";
    button.dataset.sortKey = sortKey;
    button.dataset.sortDefaultDir = defaultDir;
    button.innerHTML = `<span>${escapeHtml(cell.textContent || "")}</span>${icon("arrows-sort", "icon-xs sort-icon")}`;
    cell.replaceWith(button);
  });
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
        deviceAgentVersion(device),
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
  const devices = sortDevices(filteredDevices());
  els.deviceList.innerHTML = "";
  const online = state.devices.filter((device) => device.online).length;
  const alerts = state.devices.filter((device) => device.active_alerts).length;
  els.fleetTotalCount.textContent = state.devices.length;
  els.fleetOnlineCount.textContent = online;
  els.fleetOfflineCount.textContent = state.devices.length - online;
  els.fleetAlertCount.textContent = alerts;
  els.navAlertCount.textContent = alerts;
  renderFleetSortButtons();

  if (devices.length === 0) {
    const hasFilters = Boolean(
      els.fleetSearch.value.trim()
      || els.fleetGroupFilter.value.trim()
      || els.fleetTagFilter.value.trim()
      || state.filter !== "all"
    );
    if (els.emptyStateTitle) {
      els.emptyStateTitle.textContent = hasFilters ? "Ничего не найдено по текущим фильтрам" : "Устройства пока не подключены";
    }
    if (els.emptyStateDescription) {
      els.emptyStateDescription.textContent = hasFilters
        ? "Снимите часть фильтров или измените запрос, чтобы снова увидеть устройства."
        : "Дождитесь первого heartbeat от агента или проверьте подключение роутера к серверу.";
    }
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
    item.dataset.deviceId = device.id;
    item.setAttribute("aria-label", `Открыть роутер ${displayName}`);
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
      <div class="device-field">
        <span class="device-field-label">WAN</span>
        <span class="wan-state ${wan.className}">${escapeHtml(wan.label)}</span>
      </div>
      <div class="device-field">
        <span class="device-field-label">Онлайн / всего</span>
        <span class="client-count">${clients.online} / ${clients.total}</span>
      </div>
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

function renderInventory(device) {
  const inventory = device && device.inventory && typeof device.inventory === "object" ? device.inventory : {};
  const entries = Object.keys(inventory);
  const board = inventory.board && typeof inventory.board === "object" ? inventory.board : {};
  const release = board.release && typeof board.release === "object" ? board.release : {};
  const packageCount = Array.isArray(inventory.packages)
    ? inventory.packages.length
    : inventory.packages && typeof inventory.packages === "object"
      ? Object.keys(inventory.packages).length
      : "Not reported";
  els.inventoryJson.textContent = entries.length ? JSON.stringify(inventory, null, 2) : "{}";
  if (!entries.length) {
    els.inventorySummary.innerHTML = inlineStateMarkup("Inventory пока не получен", "Данные появятся после следующего heartbeat агента.");
    els.inventoryStatus.className = `status ${device && device.online ? "unknown" : "offline"}`;
    els.inventoryStatus.textContent = device && device.online ? "EMPTY" : "OFFLINE";
    return;
  }
  const stale = !device.online || ageInMilliseconds(device.last_seen_at) > 5 * 60 * 1000;
  els.inventoryStatus.className = `status ${!device.online ? "offline" : stale ? "stale" : "completed"}`;
  els.inventoryStatus.textContent = !device.online ? "OFFLINE" : stale ? "STALE" : "CURRENT";
  const values = [
    ["Hostname", inventory.hostname || board.hostname || deviceDisplayName(device)],
    ["Board", board.model || board.board_name || "Not reported"],
    ["OpenWrt", release.version || device.openwrt_version || "Not reported"],
    ["Target", release.target || board.target || "Not reported"],
    ["Agent", inventory.agent_version || "Not reported"],
    ["Package manager", inventory.package_manager || "Not reported"],
    ["Packages", packageCount],
    ["WAN / IPv6", inventory.wan_ip || inventory.wan_ipv6 || "Not reported"],
  ];
  els.inventorySummary.innerHTML = values.map(([label, value]) => `<div><span>${escapeHtml(label)}</span><code title="${escapeHtml(value)}">${escapeHtml(value)}</code></div>`).join("");
}

function latestCommandMatching(commands, predicate) {
  return (commands || []).find(predicate) || null;
}

function setExpertOperationState(element, command, device) {
  if (!element) return;
  if (!device || !device.online) {
    element.className = "status offline";
    element.textContent = "OFFLINE";
    return;
  }
  if (!command) {
    element.className = "status ready";
    element.textContent = "READY";
    return;
  }
  element.className = `status ${command.status || "unknown"}`;
  element.textContent = commandStatusCode(command.status);
}

function renderExpertOperationPanels(commands = state.commands) {
  const device = currentDevice();
  const packageCommand = latestCommandMatching(commands, (command) => /^(pkg|opkg)_/.test(command.type));
  const uciCommand = latestCommandMatching(commands, (command) => command.type.startsWith("uci_"));
  const manualCommand = latestCommandMatching(commands, (command) => ["ping", "traceroute", "route_show", "interfaces_show", "pkg_list_installed", "reboot"].includes(command.type));
  setExpertOperationState(els.manualCommandStatus, manualCommand, device);
  setExpertOperationState(els.packageOperationStatus, packageCommand, device);
  setExpertOperationState(els.uciOperationStatus, uciCommand, device);

  const panels = [
    [manualCommand, els.manualCommandOutputDetails, els.manualCommandOutput],
    [packageCommand, els.packageOutputDetails, els.packageTechnicalOutput],
    [uciCommand, els.uciOutputDetails, els.uciTechnicalOutput],
  ];
  for (const [command, details, output] of panels) {
    if (!details || !output) continue;
    details.classList.toggle("is-hidden", !command);
    if (command) output.textContent = command.output || JSON.stringify(command.args || {}, null, 2);
  }

  const args = uciCommand && uciCommand.args && typeof uciCommand.args === "object" ? uciCommand.args : null;
  const hasDiff = args && args.config && args.section && args.option && args.value !== undefined;
  els.uciDiff.classList.toggle("is-hidden", !hasDiff);
  if (hasDiff) {
    const path = `${args.config}.${args.section}.${args.option}`;
    els.uciDiff.innerHTML = `
      <header><span class="eyebrow">CONFIGURATION / UCI</span><code>${escapeHtml(path)}</code><span class="status ${escapeHtml(uciCommand.status || "unknown")}">${escapeHtml(commandStatusCode(uciCommand.status))}</span></header>
      <div class="technical-diff-values"><div data-tone="current"><span>CURRENT</span><code>- Not reported</code></div><div data-tone="new"><span>NEW</span><code>+ ${escapeHtml(args.value)}</code></div></div>
    `;
  }
}

function renderDeviceDetail(device) {
  els.appShell.classList.toggle("has-selected-device", Boolean(device));
  if (!device) {
    els.fleetView.classList.remove("is-hidden");
    els.fleetView.hidden = false;
    els.deviceView.classList.add("is-hidden");
    els.deviceView.hidden = true;
    els.pageTitle.textContent = "Объекты";
    return;
  }

  els.fleetView.classList.add("is-hidden");
  els.fleetView.hidden = true;
  els.deviceView.classList.remove("is-hidden");
  els.deviceView.hidden = false;
  renderDeviceTab();
  const displayName = deviceDisplayName(device);
  els.pageTitle.textContent = displayName;
  els.deviceName.textContent = displayName;
  els.deviceMeta.textContent = `${device.id} - ${device.openwrt_version || "unknown"}${device.domain_name ? ` - ${device.domain_name}` : ""}`;
  els.deviceBadge.textContent = device.online ? "На связи" : "Не на связи";
  els.deviceBadge.className = `badge ${device.online ? "online" : "offline"}`;
  els.updateAgentBtn.classList.toggle("is-hidden", !hasAgentUpdateAvailable(device));
  els.rollbackAgentBtn.classList.toggle("is-hidden", !(state.user && state.user.role === "admin" && supportsManagedAgentUpdate(device)));
  for (const element of document.querySelectorAll(".admin-only")) {
    element.classList.toggle("is-hidden", !(state.user && state.user.role === "admin"));
  }
  els.lastSeen.textContent = formatDate(device.last_seen_at);
  els.loadAvg.textContent = formatLoadAverage(device.metrics && device.metrics.loadavg);
  els.uptime.textContent = formatUptime(device.metrics && device.metrics.uptime);
  els.defaultRoute.textContent = device.inventory && device.inventory.default_route ? device.inventory.default_route : "-";
  els.wanIp.textContent = device.inventory && device.inventory.wan_ip ? device.inventory.wan_ip : "-";
  els.memoryUsage.textContent = formatMemory(device.metrics && device.metrics.memory);
  els.diskUsage.textContent = formatDisk(device.metrics && device.metrics.disk);
  const clientCounts = deviceClientCount(device);
  els.clientCounts.textContent = `${clientCounts.online} в сети / ${clientCounts.total} известно`;
  els.connectivityStatus.textContent = formatConnectivity(device.metrics && device.metrics.connectivity_checks);
  els.fleetGroup.value = device.group || "";
  els.fleetTags.value = Array.isArray(device.tags) ? device.tags.join(", ") : "";
  renderInventory(device);
  renderDeviceInformation(device);
  renderDeviceStatus(device);
  renderAgentUpdateStatus();
  renderHealthSummary(device);
  renderClients(device);
  renderInterfaceCounters(device);
  renderOperationalAvailability(device);
  renderExpertOperationPanels();
}

function renderAgentUpdateStatus() {
  const operations = state.commands.filter((item) => item.type === "agent_update" || item.type === "agent_rollback").slice(0, 5);
  const command = operations[0];
  const pending = operations.some((item) => item.status === "queued" || item.status === "claimed" || (item.result && item.result.health_status === "waiting_reconnect"));
  els.updateAgentBtn.classList.toggle("is-hidden", !hasAgentUpdateAvailable(currentDevice()) || pending);
  if (!command) {
    els.agentUpdateStatus.classList.add("is-hidden");
    els.agentUpdateStatus.innerHTML = "";
    return;
  }

  const healthStatus = command.result && command.result.health_status;
  const running = command.status === "queued" || command.status === "claimed" || healthStatus === "waiting_reconnect";
  const targetVersion = command.args && command.args.target_version;
  const output = command.output ? ` ${outputSummary(command.output)}` : "";
  const operationLabel = command.type === "agent_rollback" ? "Откат агента" : "Обновление агента";
  const title = healthStatus === "waiting_reconnect"
    ? `${operationLabel}: ожидание повторного подключения`
    : running ? `${operationLabel} выполняется` : `${operationLabel}: ${statusLabel(command.status)}`;
  const detail = running
    ? `${healthStatus === "waiting_reconnect" ? "Пакет установлен, сервер ждёт heartbeat с новой версией" : statusLabel(command.status)}${targetVersion ? ` до версии ${targetVersion}` : ""}. Попытка ${command.attempt_count || 0}/${command.max_attempts || 3}.${output}`
    : `${targetVersion ? `Целевая версия ${targetVersion}. ` : ""}${healthStatus === "healthy" ? `Подключение и версия подтверждены ${formatShortDate(command.result.reconnect_verified_at)}.` : healthStatus === "reconnect_timeout" ? "Роутер не подтвердил новую версию вовремя." : output || "Смотрите результат в истории команд."}`;
  const history = operations.map((item) => {
    const itemHealth = item.result && item.result.health_status;
    const itemState = itemHealth === "healthy" ? "Подтверждено" : itemHealth === "waiting_reconnect" ? "Ожидается reconnect" : itemHealth === "reconnect_timeout" ? "Таймаут reconnect" : statusLabel(item.status);
    return `<li><span>${escapeHtml(commandTypeLabel(item.type))} → ${escapeHtml((item.args && item.args.target_version) || "-")}</span><small>${escapeHtml(itemState)} · ${escapeHtml(formatShortDate(item.completed_at || item.created_at))}</small></li>`;
  }).join("");
  els.agentUpdateStatus.classList.remove("is-hidden");
  els.agentUpdateStatus.classList.toggle("is-running", running);
  els.agentUpdateStatus.innerHTML = `
    <span class="operation-icon" aria-hidden="true">${icon(running ? "loader-2" : healthStatus === "healthy" || command.status === "completed" ? "check" : "alert-triangle")}</span>
    <div><strong>${escapeHtml(title)}</strong><small>${escapeHtml(detail)}</small><ul class="agent-operation-history">${history}</ul></div>
  `;
}

function renderDeviceInformation(device) {
  const board = device.inventory && device.inventory.board ? device.inventory.board : {};
  const release = board.release || {};
  els.informationStatus.textContent = device.online ? "На связи" : "Не на связи";
  els.informationStatus.className = `badge ${device.online ? "online" : "offline"}`;
  els.infoHostname.textContent = deviceDisplayName(device);
  els.infoModel.textContent = board.model || "-";
  els.infoOpenWrt.textContent = release.description || device.openwrt_version || "-";
  els.infoTarget.textContent = release.target || "-";
  els.infoSystem.textContent = board.system || "-";
  els.infoKernel.textContent = board.kernel || "-";
  els.infoAgentVersion.textContent = agentVersionLabel(deviceAgentVersion(device));
  els.infoCreatedAt.textContent = formatDate(device.created_at);
  els.infoLastSeenAt.textContent = formatDate(device.last_seen_at);
  els.infoDeviceId.textContent = device.id || "-";
  els.infoBoardName.textContent = board.board_name || "-";
  els.infoRootfs.textContent = board.rootfs_type || "-";
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

function formatRelativeTime(value) {
  const timestamp = Date.parse(value || "");
  if (!Number.isFinite(timestamp)) return "время неизвестно";
  const seconds = Math.max(0, Math.round((Date.now() - timestamp) / 1000));
  if (seconds < 60) return `${seconds} сек. назад`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} мин. назад`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} ч. назад`;
  return `${Math.floor(hours / 24)} дн. назад`;
}

function humanizeAgentError(value) {
  const message = String(value || "").trim();
  if (!message) return "Ошибок обмена не зафиксировано";
  const normalized = message.toLowerCase();
  if (normalized.includes("x509") || normalized.includes("certificate")) {
    return "Ошибка TLS-сертификата: имя или цепочка сертификата не совпадает с адресом сервера";
  }
  if (normalized.includes("no such host") || normalized.includes("bad address")) {
    return "DNS не смог определить адрес RMM-сервера";
  }
  if (normalized.includes("connection refused")) return "RMM-сервер отклонил подключение";
  if (normalized.includes("timeout") || normalized.includes("deadline exceeded")) return "RMM-сервер не ответил вовремя";
  if (normalized.includes("401") || normalized.includes("unauthorized")) return "Сервер отклонил ключ устройства";
  if (normalized.includes("403") || normalized.includes("forbidden")) return "Сервер запретил запрос агента";
  return message;
}

function renderDeviceStatus(device) {
  const checks = Array.isArray(device.metrics && device.metrics.connectivity_checks) ? device.metrics.connectivity_checks : [];
  const serverHost = (device.metrics && device.metrics.server_check_target) || window.location.hostname;
  const serverCheck = checks.find((check) => check.target === serverHost);
  const wanChecks = checks.filter((check) => check.target !== serverHost);
  const version = deviceAgentVersion(device);
  const agentHealth = device.metrics && device.metrics.agent_health ? device.metrics.agent_health : null;
  const pending = Number(agentHealth && agentHealth.pending_results || 0);
  const failures = Number(agentHealth && agentHealth.consecutive_failures || 0);
  let tone = "good";
  let title = "Роутер работает нормально";
  let description = `Heartbeat получен ${formatRelativeTime(device.last_seen_at)}. Интернет и связь с сервером доступны.`;
  let iconName = "check";

  if (!device.online) {
    tone = "bad";
    title = "Агент не выходит на связь";
    description = `Последний heartbeat был ${formatRelativeTime(device.last_seen_at)}. Проверьте службу агента, DNS, время роутера и TLS-сертификат.`;
    iconName = "alert-triangle";
  } else if (serverCheck && !serverCheck.reachable) {
    tone = "bad";
    title = "Проблема связи с RMM-сервером";
    description = "Heartbeat дошёл, но проверка адреса сервера с роутера завершается ошибкой.";
    iconName = "alert-triangle";
  } else if (wanChecks.length && !wanChecks.some((check) => check.reachable)) {
    tone = "bad";
    title = "Интернет с роутера недоступен";
    description = "Агент подключён к серверу, но внешние контрольные адреса не отвечают.";
    iconName = "alert-triangle";
  } else if (failures > 0) {
    tone = "warn";
    title = "Агент восстанавливает связь";
    description = `Неудачных попыток подряд: ${failures}. Интервал повторов временно увеличен.`;
    iconName = "loader-2";
  } else if (pending > 0) {
    tone = "warn";
    title = "Есть данные, ожидающие отправки";
    description = `Агент хранит ${pending} ${resultWord(pending)} выполнения команд локально и повторит отправку автоматически.`;
    iconName = "loader-2";
  } else if (Number(device.active_alerts || 0) > 0) {
    tone = "warn";
    title = "Роутер требует внимания";
    description = `Активных проблем: ${device.active_alerts}. Подробности находятся ниже в журнале проблем.`;
    iconName = "alert-triangle";
  } else if (agentVersionState(version).comparison === -1) {
    const stable = stableAgentVersion();
    tone = "warn";
    title = "Работает, доступно обновление агента";
    description = `Установлена версия ${version}; актуальная стабильная версия — ${stable}.`;
    iconName = "loader-2";
  }

  els.deviceStatusHero.className = `device-status-hero ${tone}`;
  els.deviceStatusHero.innerHTML = `
    <span class="device-status-icon" aria-hidden="true">${icon(iconName)}</span>
    <div><strong>${escapeHtml(title)}</strong><p>${escapeHtml(description)}</p></div>
  `;

  const lastError = agentHealth && agentHealth.last_heartbeat_error;
  const errorAt = agentHealth && agentHealth.last_heartbeat_error_at;
  const errorText = agentHealth
    ? (lastError
      ? `${failures ? "Последняя ошибка" : "Последний восстановленный сбой"}: ${humanizeAgentError(lastError)} · ${formatRelativeTime(errorAt)}`
      : "Ошибок обмена не зафиксировано")
    : "Расширенная диагностика недоступна в данных последнего heartbeat.";
  els.agentHealthSummary.innerHTML = `
    <div><span>Агент</span><strong>${escapeHtml(agentVersionLabel(version))}</strong></div>
    <div><span>Runtime</span><strong>${escapeHtml(device.inventory && device.inventory.agent_runtime || "неизвестно")}</strong></div>
    <div class="${failures ? "has-agent-error" : ""}"><span>Обмен с сервером</span><strong>${failures ? `${failures} ошибок подряд` : "Стабильно"}</strong></div>
    <div class="${pending ? "has-agent-warning" : ""}"><span>Ожидают отправки</span><strong>${pending} ${resultWord(pending)}</strong></div>
    <p class="agent-health-message">${escapeHtml(errorText)}</p>
  `;
}

function resultWord(value) {
  const number = Math.abs(Number(value || 0));
  const lastTwo = number % 100;
  if (lastTwo >= 11 && lastTwo <= 14) return "результатов";
  if (number % 10 === 1) return "результат";
  if (number % 10 >= 2 && number % 10 <= 4) return "результата";
  return "результатов";
}

function renderHealthSummary(device) {
  const checks = Array.isArray(device.metrics && device.metrics.connectivity_checks) ? device.metrics.connectivity_checks : [];
  const serverHost = (device.metrics && device.metrics.server_check_target) || window.location.hostname;
  const serverCheck = checks.find((check) => check.target === serverHost);
  const wanChecks = checks.filter((check) => check.target !== serverHost);
  const wanReachable = wanChecks.some((check) => check.reachable);
  const serverStatus = serverCheck ? (serverCheck.reachable ? `Есть, ${serverCheck.latency_ms} ms` : "Нет") : (device.online ? "Есть" : "Нет данных");
  const serverState = serverCheck ? (serverCheck.reachable ? "ok" : "bad") : (device.online ? "ok" : "warn");
  const rows = [
    ["Статус", device.online ? "На связи" : "Нет связи", device.online ? "ok" : "bad"],
    ["Связь с сервером", serverStatus, serverState],
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
  const wifiStations = Array.isArray(device && device.inventory && device.inventory.wifi_clients) ? device.inventory.wifi_clients : [];
  const wifiByMAC = new Map(wifiStations.filter((station) => station.mac).map((station) => [String(station.mac).toLowerCase(), station]));
  if (device && device.id === state.selectedDeviceId && state.lanClients.length) {
    return state.lanClients.map((client) => {
      const station = wifiByMAC.get(String(client.mac || "").toLowerCase()) || {};
      const type = client.connection === "wifi" ? "wifi" : (client.connection === "wired" ? "wired" : "unknown");
      return {
        name: client.hostname || "",
        ip: client.ip || "-",
        mac: client.mac || "-",
        connection: type === "wifi" ? `Wi-Fi ${client.interface || station.interface || ""}`.trim() : type === "wired" ? `LAN ${client.interface || ""}`.trim() : "Не определено",
        type,
        online: client.status === "online",
        presence: client.status === "unconfirmed" && client.last_seen_at ? "offline" : (client.status || "unconfirmed"),
        confirmation: client.confirmation || "",
        signal: station.signal_dbm ? `${station.signal_dbm} dBm` : "",
        rate: [station.rx_rate ? `RX ${station.rx_rate}` : "", station.tx_rate ? `TX ${station.tx_rate}` : ""].filter(Boolean).join(" / "),
        lastSeenAt: client.last_seen_at || "",
        lastCheckedAt: client.last_checked_at || "",
      };
    });
  }
  const leases = Array.isArray(device.inventory && device.inventory.dhcp_leases) ? device.inventory.dhcp_leases : [];
  const wifi = Array.isArray(device.inventory && device.inventory.wifi_clients) ? device.inventory.wifi_clients : [];
  const neighbors = Array.isArray(device.inventory && device.inventory.neighbors) ? device.inventory.neighbors : [];
  const neighborsByMAC = new Map(neighbors.filter((neighbor) => neighbor.mac).map((neighbor) => [String(neighbor.mac).toLowerCase(), neighbor]));
  const neighborsByIP = new Map(neighbors.filter((neighbor) => neighbor.ip).map((neighbor) => [String(neighbor.ip), neighbor]));
  // Only states backed by recent neighbour discovery count as online.
  // STALE, PERMANENT and NOARP can survive after a client disconnects.
  const activeNeighborStates = new Set(["REACHABLE", "DELAY", "PROBE"]);
  const byMac = new Map();
  for (const lease of leases) {
    const mac = String(lease.mac || "").toLowerCase();
    const key = mac || `ip:${lease.ip || "unknown"}`;
    const neighbor = neighborsByMAC.get(mac) || neighborsByIP.get(String(lease.ip || ""));
    const neighborState = String(neighbor && neighbor.state || "").toUpperCase();
    const presence = activeNeighborStates.has(neighborState) ? "online" : (neighborState === "STALE" ? "stale" : "reserved");
    byMac.set(key, {
      name: lease.hostname && lease.hostname !== "*" ? lease.hostname : "",
      ip: lease.ip || "-",
      mac: lease.mac || "-",
      connection: presence === "online" ? `LAN ${neighbor && neighbor.interface || ""}`.trim() : "DHCP-запись",
      type: "wired",
      online: presence === "online",
      presence,
      confirmation: neighborState ? "neighbor" : "lease",
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
      signal: station.signal_dbm ? `${station.signal_dbm} dBm` : "",
      rate: [station.rx_rate ? `RX ${station.rx_rate}` : "", station.tx_rate ? `TX ${station.tx_rate}` : ""].filter(Boolean).join(" / "),
      type: "wifi",
      online: true,
      presence: "online",
      confirmation: "wifi station",
    });
  }
  const presenceRank = { online: 0, recent: 1, stale: 1, unconfirmed: 2, reserved: 2 };
  return [...byMac.values()].sort((left, right) => {
    const rank = (presenceRank[left.presence] ?? 3) - (presenceRank[right.presence] ?? 3);
    if (rank !== 0) return rank;
    return String(left.name || left.ip).localeCompare(String(right.name || right.ip), undefined, { numeric: true, sensitivity: "base" });
  });
}

function renderClients(device) {
  const clients = normalizedClients(device);
  const search = els.clientSearch.value.trim().toLowerCase();
  const filtered = clients.filter((client) => {
    if (state.clientFilter === "online" && client.presence !== "online") return false;
    if (state.clientFilter === "unconfirmed" && !["unconfirmed", "reserved", "unknown"].includes(client.presence)) return false;
    if (["wifi", "wired"].includes(state.clientFilter) && client.type !== state.clientFilter) return false;
    return !search || [client.name, client.ip, client.mac, client.connection].join(" ").toLowerCase().includes(search);
  });
  els.clientList.innerHTML = "";
  const onlineCount = clients.filter((client) => client.presence === "online").length;
  const wifiOnline = clients.filter((client) => client.type === "wifi" && client.presence === "online").length;
  const wiredOnline = clients.filter((client) => client.type === "wired" && client.presence === "online").length;
  const unconfirmedCount = clients.filter((client) => ["unconfirmed", "reserved", "unknown"].includes(client.presence)).length;
  els.clientSummary.textContent = `${onlineCount} в сети · ${wifiOnline} Wi-Fi · ${clients.length} известно`;
  const filterLabels = { all: `Все · ${clients.length}`, online: `Online · ${onlineCount}`, wifi: `Wi-Fi · ${wifiOnline}`, wired: `LAN · ${wiredOnline}`, unconfirmed: `Unknown · ${unconfirmedCount}` };
  for (const button of document.querySelectorAll(".client-filter")) button.textContent = filterLabels[button.dataset.clientFilter] || button.textContent;

  const latestClientCheck = clients.map((client) => new Date(client.lastCheckedAt || 0).getTime()).filter(Number.isFinite).sort((left, right) => right - left)[0] || 0;
  if (!device.online) {
    setDataContextState(els.clientDataState, "offline", "Роутер не на связи", `Показаны последние известные данные · последний контакт ${formatDate(device.last_seen_at)}`);
  } else if (latestClientCheck && Date.now() - latestClientCheck > 5 * 60 * 1000) {
    setDataContextState(els.clientDataState, "stale", "Данные о клиентах устарели", `Последняя проверка ${formatDate(latestClientCheck)}`);
  } else {
    setDataContextState(els.clientDataState);
  }

  if (filtered.length === 0) {
    if (clients.length === 0) {
      els.clientList.innerHTML = inlineStateMarkup("Клиентов пока нет", device.online ? "Агент не обнаружил DHCP, neighbour или Wi-Fi записей." : "Последние данные недоступны, пока роутер не восстановит соединение.", device.online ? "neutral" : "offline");
    } else {
      const query = search ? `По запросу «${els.clientSearch.value.trim()}» совпадений нет.` : "Текущий фильтр не совпадает ни с одной записью.";
      els.clientList.innerHTML = inlineStateMarkup("Клиенты не найдены", `${query} Измените поиск или фильтр.`);
    }
    return;
  }
  for (const client of filtered) {
    const sourcePresence = client.presence || (client.online ? "online" : "unknown");
    const presence = !device.online ? "offline" : sourcePresence === "reserved" ? "unknown" : sourcePresence;
    const presenceLabel = { online: "ONLINE", recent: "STALE", stale: "STALE", offline: "OFFLINE", unconfirmed: "UNCONFIRMED", unknown: "UNKNOWN" }[presence] || "UNKNOWN";
    const typeLabel = client.type === "wifi" ? "WIFI" : client.type === "wired" ? "WIRED" : "UNKNOWN";
    const connectionMeta = [typeLabel, client.connection, client.confirmation || (presence === "online" ? "confirmed" : "lease known")].filter(Boolean).join(" · ");
    const displayName = client.name || client.ip || "Неизвестное устройство";
    const row = document.createElement("div");
    row.className = "client-row client-record";
    row.setAttribute("role", "listitem");
    row.innerHTML = `
      <div class="client-name client-primary" data-label="Hostname"><span class="client-icon">${icon(client.type === "wifi" ? "wifi" : "network")}</span><span class="client-identity"><strong title="${escapeHtml(displayName)}">${escapeHtml(displayName)}</strong><small>${escapeHtml(connectionMeta)}</small></span></div>
      <div class="client-cell" data-label="IP"><code title="${escapeHtml(client.ip)}">${escapeHtml(client.ip)}</code></div>
      <div class="client-cell" data-label="MAC"><code>${escapeHtml(client.mac)}</code></div>
      <div class="client-cell" data-label="Подключение"><span>${escapeHtml(client.connection || "-")}</span></div>
      <div class="client-cell" data-label="Сигнал"><code>${escapeHtml(client.signal || "-")}</code></div>
      <div class="client-cell" data-label="Link rate"><code>${escapeHtml(client.rate || (client.lastSeenAt ? `seen ${formatRelativeTime(client.lastSeenAt)}` : "-"))}</code></div>
      <div class="client-cell client-state-cell" data-label="Состояние"><span class="client-online status ${presence}"><i aria-hidden="true"></i><span class="client-online-label">${presenceLabel}</span></span></div>
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
    byName.get(address.name).push({ family: address.family || "", address: address.address || "" });
  }
  const counterByName = new Map(counters.filter((item) => item.name).map((item) => [item.name, item]));
  const interfaceNames = [...new Set([...counterByName.keys(), ...byName.keys()])].sort((left, right) => left.localeCompare(right, undefined, { numeric: true }));
  els.interfaceCounters.innerHTML = "";
  els.networkSummary.textContent = `${interfaceNames.length} интерфейсов`;
  els.networkHealth.innerHTML = `
    <div><span>WAN-адрес</span><code>${escapeHtml(device.inventory && device.inventory.wan_ip ? device.inventory.wan_ip : "Нет данных")}</code></div>
    <div><span>Маршрут по умолчанию</span><code>${escapeHtml(device.inventory && device.inventory.default_route ? device.inventory.default_route : "Нет данных")}</code></div>
    <div><span>Проверки связи</span><strong>${escapeHtml(formatConnectivity(device.metrics && device.metrics.connectivity_checks))}</strong></div>
  `;
  if (!device.online) {
    setDataContextState(els.networkDataState, "offline", "Роутер не на связи", `Сетевые значения сохранены с последнего контакта ${formatDate(device.last_seen_at)}`);
  } else if (ageInMilliseconds(device.last_seen_at) > 5 * 60 * 1000) {
    setDataContextState(els.networkDataState, "stale", "Сетевые данные могут быть устаревшими", `Последний heartbeat ${formatDate(device.last_seen_at)}`);
  } else {
    setDataContextState(els.networkDataState);
  }
  if (interfaceNames.length === 0) {
    els.interfaceCounters.innerHTML = inlineStateMarkup("Нет данных об интерфейсах", device.online ? "Агент ещё не прислал адреса или сетевые счётчики." : "Данные станут доступны после восстановления связи.", device.online ? "neutral" : "offline");
    return;
  }
  for (const name of interfaceNames) {
    const item = counterByName.get(name) || {};
    const interfaceAddresses = byName.get(name) || [];
    const ipv4 = interfaceAddresses.filter((entry) => entry.family === "inet" || (!entry.family && !entry.address.includes(":"))).map((entry) => entry.address);
    const ipv6 = interfaceAddresses.filter((entry) => entry.family === "inet6" || entry.address.includes(":")).map((entry) => entry.address);
    const row = document.createElement("div");
    const errors = Number(item.rx_errors || 0) + Number(item.tx_errors || 0);
    row.className = "network-row";
    row.setAttribute("role", "listitem");
    row.innerHTML = `
      <div class="network-interface" data-label="Интерфейс"><strong>${escapeHtml(name || "-")}</strong><small>${interfaceAddresses.length} address</small></div>
      <div data-label="Состояние"><span class="status ${device.online && counterByName.has(name) ? "active" : device.online ? "unknown" : "offline"}">${device.online && counterByName.has(name) ? "OBSERVED" : device.online ? "UNKNOWN" : "OFFLINE"}</span></div>
      <div class="address-stack" data-label="IPv4">${ipv4.length ? ipv4.map((address) => `<code title="${escapeHtml(address)}">${escapeHtml(address)}</code>`).join("") : "<code>-</code>"}</div>
      <div class="address-stack" data-label="IPv6">${ipv6.length ? ipv6.map((address) => `<code title="${escapeHtml(address)}">${escapeHtml(address)}</code>`).join("") : "<code>-</code>"}</div>
      <div class="traffic-cell" data-label="RX"><code>${escapeHtml(formatBytes(item.rx_bytes))}</code><small>${escapeHtml(item.rx_packets || 0)} пакетов</small></div>
      <div class="traffic-cell" data-label="TX"><code>${escapeHtml(formatBytes(item.tx_bytes))}</code><small>${escapeHtml(item.tx_packets || 0)} пакетов</small></div>
      <div data-label="Ошибки"><span class="status ${errors ? "warning" : "neutral"}">${escapeHtml(errors)} ERR</span></div>
    `;
    els.interfaceCounters.appendChild(row);
  }
}

async function loadMetricsHistory() {
  if (!state.selectedDeviceId) return;
  const data = await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/metrics-history?limit=48`);
  renderMetricsHistory(data.samples || []);
}

function renderMetricsHistory(samples) {
  els.metricsHistory.innerHTML = "";
  if (samples.length === 0) {
    els.metricsHistorySummary.textContent = "Нет замеров";
    els.metricsHistory.innerHTML = inlineStateMarkup("История мониторинга пока пуста", "Графики появятся после нескольких heartbeat от агента.");
    return;
  }
  const ordered = [...samples].reverse();
  const historyRange = ordered.length > 1
    ? `${formatRelativeTime(ordered[0].created_at)} — сейчас`
    : "первый замер";
  els.metricsHistorySummary.textContent = `${samples.length} точек · ${historyRange}`;
  const point = (sample, value) => ({ value: Number.isFinite(Number(value)) ? Number(value) : 0, time: sample.created_at });
  const memoryPoints = ordered.map((sample) => {
    const memory = sample.metrics && sample.metrics.memory;
    return point(sample, memory && memory.total_kb ? Math.round((Number(memory.used_kb || 0) / Number(memory.total_kb)) * 100) : 0);
  });
  const diskPoints = ordered.map((sample) => {
    const disk = sample.metrics && sample.metrics.disk;
    return point(sample, disk && disk.used_percent ? Number(String(disk.used_percent).replace("%", "")) : 0);
  });
  const latencyPoints = ordered.map((sample) => {
    const checks = Array.isArray(sample.metrics && sample.metrics.connectivity_checks) ? sample.metrics.connectivity_checks : [];
    return point(sample, Math.max(0, ...checks.filter((check) => check.reachable).map((check) => Number(check.latency_ms || 0))));
  });
  const loadPoints = ordered.map((sample) => point(sample, Number.parseFloat(String(sample.metrics && sample.metrics.loadavg || "0")) || 0));
  els.metricsHistory.appendChild(metricChart("Память", "%", memoryPoints, "warn", 100));
  els.metricsHistory.appendChild(metricChart("Хранилище", "%", diskPoints, "good", 100));
  els.metricsHistory.appendChild(metricChart("Задержка", "ms", latencyPoints, "accent"));
  els.metricsHistory.appendChild(metricChart("Нагрузка", "", loadPoints, "violet"));
}

function metricChart(label, unit, points, tone, fixedMax = 0) {
  const values = points.map((point) => Number(point.value || 0));
  const max = fixedMax || Math.max(1, ...values);
  const min = Math.min(...values);
  const width = 320;
  const height = 92;
  const coordinates = values.map((value, index) => {
    const x = values.length <= 1 ? width / 2 : (index / (values.length - 1)) * width;
    const y = height - Math.max(2, Math.min(height - 2, (value / max) * (height - 8) + 4));
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  });
  const linePoints = coordinates.join(" ");
  const areaPoints = values.length ? `0,${height} ${linePoints} ${width},${height}` : "";
  const card = document.createElement("div");
  card.className = `metric-chart ${tone}`;
  const latest = values.length ? values[values.length - 1] : 0;
  const formatValue = (value) => Number(value).toLocaleString(undefined, { maximumFractionDigits: unit === "%" ? 0 : 2 });
  const startTime = points.length ? formatShortDate(points[0].time) : "-";
  const endTime = points.length ? formatShortDate(points[points.length - 1].time) : "-";
  card.innerHTML = `
    <div class="metric-chart-title"><span>${escapeHtml(label)}</span><strong>${escapeHtml(formatValue(latest))}${unit ? ` ${escapeHtml(unit)}` : ""}</strong></div>
    <div class="metric-chart-meta"><span>мин. ${escapeHtml(formatValue(min))}</span><span>макс. ${escapeHtml(formatValue(Math.max(...values)))}</span></div>
    <div class="metric-line-chart">
      <svg viewBox="0 0 ${width} ${height}" preserveAspectRatio="none" role="img" aria-label="${escapeHtml(label)}: последнее значение ${escapeHtml(formatValue(latest))} ${escapeHtml(unit)}">
        <line x1="0" y1="${height / 2}" x2="${width}" y2="${height / 2}"></line>
        <polygon points="${areaPoints}"></polygon>
        <polyline points="${linePoints}"></polyline>
      </svg>
    </div>
    <div class="metric-chart-axis"><span>${escapeHtml(startTime)}</span><span>${escapeHtml(endTime)}</span></div>
  `;
  return card;
}

async function loadAlerts() {
  if (!state.selectedDeviceId) return;
  const status = encodeURIComponent(state.alertStatusFilter || "open");
  els.alertList.innerHTML = inlineStateMarkup("Загрузка проблем", "Получаем актуальное состояние мониторинга.", "loading");
  try {
    const data = await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/alerts?status=${status}`);
    state.alerts = data.alerts || [];
    renderAlerts(state.alerts);
  } catch (error) {
    els.alertList.innerHTML = inlineStateMarkup("Не удалось загрузить проблемы", error.message || "Повторите обновление.", "error");
    throw error;
  }
}

function renderAlerts(alerts) {
  els.alertList.innerHTML = "";
  const device = currentDevice();
  const activeCount = alerts.filter((alert) => alert.status === "active").length;
  const acknowledgedCount = alerts.filter((alert) => alert.status === "acknowledged").length;
  const resolvedCount = alerts.filter((alert) => alert.status === "resolved").length;
  els.alertSummary.textContent = `${activeCount} active · ${acknowledgedCount} ack · ${resolvedCount} resolved`;
  if (device && !device.online) {
    setDataContextState(els.problemDataState, "stale", "Мониторинг устройства недоступен", `Показано последнее известное состояние · контакт ${formatDate(device.last_seen_at)}`);
  } else {
    setDataContextState(els.problemDataState);
  }
  if (alerts.length === 0) {
    els.alertList.innerHTML = state.alertStatusFilter === "resolved"
      ? inlineStateMarkup("Закрытых проблем нет", "Resolved incidents появятся здесь после восстановления контролируемого состояния.")
      : state.alertStatusFilter === "all"
        ? inlineStateMarkup("История проблем пуста", "Для устройства ещё не зарегистрировано incidents.")
        : inlineStateMarkup("Активных проблем нет", "Сейчас устройство не требует оперативного вмешательства.", "success");
    return;
  }
  for (const alert of alerts) {
    const details = alert.details ? Object.entries(alert.details).map(([key, value]) => `${key}: ${value}`).join(" / ") : "";
    const severity = alert.severity === "critical" ? "critical" : "warning";
    const severityLabel = severity === "critical" ? "CRITICAL" : "WARNING";
    const status = alert.status || "active";
    const objectDetail = alert.details && (alert.details.interface || alert.details.target || alert.details.client || alert.details.device);
    const objectName = objectDetail ? `${deviceDisplayName(device)} · ${objectDetail}` : deviceDisplayName(device);
    const startedAt = alert.first_seen_at || alert.created_at;
    const endedAt = alert.resolved_at || new Date();
    const actionButtons = alert.status === "resolved"
      ? `<button type="button" data-action="commands">Команды</button>`
      : `<button type="button" data-action="diagnose">Диагностика</button>
      <button type="button" data-action="commands">Команды</button>
      <button type="button" data-action="ack" ${alert.status === "active" ? "" : "disabled"}>Ack</button>`;
    const row = document.createElement("div");
    row.className = `alert-row problem-record ${severity} ${status}`;
    row.setAttribute("role", "listitem");
    row.innerHTML = `
      <div data-label="Severity"><span class="status ${severity}">${severityLabel}</span></div>
      <div class="problem-object" data-label="Объект"><strong title="${escapeHtml(objectName)}">${escapeHtml(objectName)}</strong></div>
      <div class="problem-copy" data-label="Проблема"><strong>${escapeHtml(alertTypeLabel(alert.type))}</strong><small>${escapeHtml(alert.message || details || "Нет дополнительных сведений")}</small><details class="alert-detail"><summary>Technical details</summary><div>${escapeHtml(details || "Дополнительные параметры отсутствуют")}</div><div>Последнее событие: ${escapeHtml(formatDate(alert.last_seen_at || alert.created_at))}</div></details></div>
      <div data-label="Возникла"><time datetime="${escapeHtml(startedAt || "")}">${escapeHtml(formatDate(startedAt))}</time></div>
      <div data-label="Длительность"><code>${escapeHtml(formatDuration(startedAt, endedAt))}</code></div>
      <div data-label="Статус"><span class="status ${escapeHtml(status)}">${escapeHtml(String(status).toUpperCase())}</span></div>
      <div class="row-actions problem-actions" data-label="Действие">${actionButtons}</div>
    `;
    row.querySelector('[data-action="diagnose"]')?.addEventListener("click", () => runAlertDiagnostics(alert));
    row.querySelector('[data-action="commands"]').addEventListener("click", scrollToCommands);
    row.querySelector('[data-action="ack"]')?.addEventListener("click", () => acknowledgeAlert(alert.id));
    els.alertList.appendChild(row);
  }
}

async function loadDevices() {
  setStatus("Обновление");
  await loadReleaseMetadata();
  const data = await api("/api/devices");
  state.devices = data.devices || [];
  if (state.selectedDeviceId && !currentDevice()) {
    state.selectedDeviceId = null;
  }
  renderDevices();
  renderDeviceDetail(currentDevice());
  if (state.selectedDeviceId) {
    await Promise.all([loadCommands(), loadAudit(), loadMetricsHistory(), loadAlerts(), loadRemoteSessions(), loadLANClients(), loadBackups()]);
  }
  state.lastUpdatedAt = new Date();
  updateLiveStateLabel();
  setStatus(`Обновлено ${state.lastUpdatedAt.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`);
}

async function selectDevice(id) {
  state.selectedDeviceId = id;
  state.deviceTab = "overview";
  state.presetReview = null;
  state.commandOffset = 0;
  state.auditOffset = 0;
  state.commands = [];
  state.auditEvents = [];
  state.lanClients = [];
  state.backups = [];
  renderDevices();
  renderDeviceDetail(currentDevice());
  setMobileRoute("fleet");
  window.scrollTo({ top: 0, behavior: "auto" });
  await Promise.all([loadCommands(), loadAudit(), loadMetricsHistory(), loadAlerts(), loadRemoteSessions(), loadLANClients(), loadBackups()]);
}

async function loadLANClients() {
  if (!state.selectedDeviceId) return;
  els.clientList.innerHTML = inlineStateMarkup("Загрузка клиентов", "Получаем DHCP, neighbour и Wi-Fi presence data.", "loading");
  try {
    const response = await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/clients`);
    state.lanClients = response.clients || [];
    renderClients(currentDevice());
  } catch (error) {
    els.clientList.innerHTML = inlineStateMarkup("Не удалось загрузить клиентов", error.message || "Повторите обновление.", "error");
    throw error;
  }
}

function showFleet() {
  state.selectedDeviceId = null;
  state.selectedCommand = null;
  state.presetReview = null;
  state.commandOffset = 0;
  state.auditOffset = 0;
  state.commands = [];
  state.auditEvents = [];
  renderDevices();
  renderDeviceDetail(null);
  setMobileRoute("fleet");
  window.scrollTo({ top: 0, behavior: "auto" });
}

function selectDeviceTab(tab) {
  state.deviceTab = tab;
  renderDeviceTab();
  if (tab === "operations") setMobileRoute("operations");
}

function renderDeviceTab() {
  for (const button of document.querySelectorAll(".device-tab")) {
    const active = button.dataset.deviceTabTarget === state.deviceTab;
    button.classList.toggle("is-active", active);
    button.setAttribute("aria-selected", String(active));
    button.tabIndex = active ? 0 : -1;
  }
  for (const section of document.querySelectorAll("[data-device-tab]")) {
    const active = section.dataset.deviceTab === state.deviceTab;
    section.classList.toggle("is-hidden", !active);
    section.hidden = !active;
    section.setAttribute("aria-hidden", String(!active));
  }
}

async function openLuCIAccess(session) {
  const popup = window.open("", "_blank");
  try {
    const data = await api(`/api/devices/${encodeURIComponent(session.device_id)}/remote-sessions/${encodeURIComponent(session.id)}/access`, {
      method: "POST",
    });
    if (popup) {
      popup.opener = null;
      popup.location = data.url;
    } else {
      window.location.assign(data.url);
    }
  } catch (error) {
    if (popup) popup.close();
    showLuCIState(error, "retry");
  }
}

function openLuciOrRemoteAccess() {
  openCloudAccess().catch(reportError);
}

function luciStateCopy(status, mode, error = {}) {
  if (mode === "setup") {
    return {
      code: "409 · SESSION REQUIRED",
      title: "Удалённый доступ не запущен",
      description: "Создайте временную сессию, чтобы безопасно открыть LuCI этого роутера.",
      action: "Настроить доступ",
    };
  }
  if (error.code === "not_configured") {
    return { code: "409 · CLOUD NOT CONFIGURED", title: "Облачный доступ не настроен", description: "Администратору нужно указать домен роутеров и публичный адрес tunnel-сервиса.", action: "Открыть настройки" };
  }
  if (error.code === "restart_failed" || error.code === "create_failed") {
    return { code: "500 · TUNNEL START FAILED", title: "Не удалось запустить туннель", description: "Сервер не смог создать новый защищённый канал. Повторите попытку или откройте диагностику.", action: "Повторить подключение" };
  }
  return {
    401: { code: "401 · SESSION EXPIRED", title: "Срок доступа истёк", description: "Временная ссылка больше не действует. Создайте новый безопасный доступ из RMM.", action: "Создать новый доступ" },
    403: { code: "403 · ACCESS DENIED", title: "Доступ отклонён", description: "У вашей учётной записи нет разрешения на эту сессию LuCI.", action: "Вернуться к настройке" },
    404: { code: "404 · SESSION NOT FOUND", title: "Сессия больше не активна", description: "Удалённая сессия закрыта или роутер ещё не подтвердил её запуск.", action: "Настроить доступ" },
    409: { code: "409 · SESSION STARTING", title: "Туннель ещё запускается", description: "Роутер получил команду, но защищённый канал пока не готов. Обычно это занимает несколько секунд.", action: "Повторить подключение" },
    429: { code: "429 · TOO MANY ATTEMPTS", title: "Слишком много запросов", description: "Лимит временных ссылок достигнут. Подождите минуту и повторите попытку.", action: "Повторить позже" },
    502: { code: "502 · LUCI UNREACHABLE", title: "LuCI недоступен", description: "Туннель работает, но веб-интерфейс роутера не отвечает. Настройки роутера не изменялись.", action: "Повторить подключение" },
    503: error.code === "device_offline"
      ? { code: "503 · ROUTER OFFLINE", title: "Роутер не на связи", description: "Агент не отправляет данные. Облачный доступ станет доступен после восстановления связи.", action: "Проверить снова" }
      : { code: "503 · TUNNEL UNAVAILABLE", title: "Облачный канал недоступен", description: "Роутер на связи, но туннель до LuCI не открылся. Проверьте tunnel-сервис и журнал агента.", action: "Повторить подключение" },
    504: { code: "504 · GATEWAY TIMEOUT", title: "LuCI не отвечает", description: "Роутер на связи, но веб-интерфейс не ответил вовремя. Настройки роутера не изменялись.", action: "Повторить подключение" },
  }[status] || { code: `${status || 500} · CONNECTION ERROR`, title: "Не удалось открыть LuCI", description: "Проверьте состояние роутера и удалённой сессии, затем повторите попытку.", action: "Повторить подключение" };
}

function showLuCIState(error = {}, mode = "retry") {
  const status = Number(error.status || 0);
  const copy = luciStateCopy(status, mode, error);
  const device = currentDevice();
  const agentOnline = error.code === "device_offline" ? false : Boolean(device && device.online);
  const activeSession = state.remoteSessions.find((session) => session.status === "active" && session.luci_port);
  state.luciAction = mode;
  els.luciStateCode.textContent = copy.code;
  els.luciStateTitle.textContent = copy.title;
  els.luciStateDescription.textContent = copy.description;
  els.luciStatePrimaryBtn.textContent = copy.action;
  els.luciStateContext.innerHTML = `
    <div><span>Объект</span><strong>${escapeHtml(deviceDisplayName(device))}</strong></div>
    <div><span>RMM-агент</span><strong>${agentOnline ? "На связи" : "Не на связи"}</strong></div>
    <div><span>Сессия</span><strong>${activeSession ? escapeHtml(statusLabel(activeSession.status)) : "Не запущена"}</strong></div>
  `;
  els.luciStateRequestId.textContent = error.requestId ? `ID запроса: ${error.requestId}` : "";
  if (!els.luciStateDialog.open) els.luciStateDialog.showModal();
}

function setMobileRoute(route) {
  state.mobileRoute = route;
  for (const button of document.querySelectorAll(".mobile-nav-item")) {
    button.classList.toggle("is-active", button.dataset.mobileRoute === route);
  }
  els.fleetNavBtn.classList.toggle("is-active", route === "fleet");
  els.problemsNavBtn.classList.toggle("is-active", route === "problems");
  els.operationsNavBtn.classList.toggle("is-active", route === "operations");
}

async function openDeviceArea(tab, selector, route) {
  let device = currentDevice();
  if (!device) {
    device = route === "problems"
      ? state.devices.find((item) => Number(item.active_alerts || 0) > 0) || state.devices[0]
      : state.devices[0];
  }
  if (!device) {
    notify("Сначала добавьте роутер", "info");
    return;
  }
  if (device.id !== state.selectedDeviceId) await selectDevice(device.id);
  selectDeviceTab(tab);
  setMobileRoute(route);
  window.requestAnimationFrame(() => document.querySelector(selector)?.scrollIntoView({ behavior: "smooth", block: "start" }));
}

function showProfile() {
  state.previousMobileRoute = state.mobileRoute === "profile" ? "fleet" : state.mobileRoute;
  setMobileRoute("profile");
  els.profileDisplayName.value = state.user && state.user.display_name ? state.user.display_name : "";
  els.profileEmail.value = state.user && state.user.email ? state.user.email : "";
  setFormMessage(els.profileMessage, "");
  setFormMessage(els.passwordMessage, "");
  els.passwordForm.reset();
  if (!els.profileDialog.open) els.profileDialog.showModal();
  selectProfileTab("account");
}

function selectProfileTab(requestedTab) {
  const isAdmin = state.user && state.user.role === "admin";
  const tab = requestedTab === "admin" && !isAdmin ? "account" : requestedTab;
  state.profileTab = tab;
  for (const button of els.profileTabs) {
    const selected = button.dataset.profileTab === tab;
    button.classList.toggle("is-active", selected);
    button.setAttribute("aria-selected", String(selected));
    button.tabIndex = selected ? 0 : -1;
  }
  for (const panel of els.profilePanels) {
    panel.classList.toggle("is-hidden", panel.dataset.profilePanel !== tab);
  }
  if (tab === "notifications") {
    loadNotificationPreferences().catch((error) => {
      setFormMessage(els.notificationSettingsMessage, `Не удалось загрузить уведомления: ${error.message}`, "error");
    });
  }
  if (tab === "admin" && isAdmin) {
    loadUsers().catch((error) => {
      els.userList.innerHTML = inlineStateMarkup("Не удалось загрузить пользователей", error.message);
    });
		loadRollouts().catch((error) => { els.rolloutList.innerHTML = inlineStateMarkup("Не удалось загрузить rollout", error.message); });
  }
}

async function loadRollouts() {
  const response = await api("/api/agent-rollouts");
  els.rolloutChannel.querySelector('option[value="candidate"]').disabled = !response.candidate_available;
  els.rolloutChannel.title = response.candidate_available ? "" : "Candidate manifest is unavailable";
  els.rolloutList.innerHTML = (response.rollouts || []).map((rollout) => {
    const devices = (rollout.devices || []).map((device) => `<li><span>${escapeHtml(device.device_id)} · batch ${device.batch || "-"}</span><small>${escapeHtml(statusLabel(device.status))}${device.last_error ? ` · ${escapeHtml(device.last_error)}` : ""}</small></li>`).join("");
    return `<article class="rollout-card"><div class="rollout-card-header"><div><strong>${escapeHtml(rollout.channel)} → ${escapeHtml(rollout.target_version)}</strong><small>${escapeHtml(statusLabel(rollout.status))}: ${rollout.devices.length} устройств, ошибок ${rollout.failure_count}</small></div><div class="user-actions">${rollout.status === "running" ? `<button data-rollout-action="pause" data-rollout-id="${escapeHtml(rollout.id)}" type="button">Пауза</button>` : ""}${rollout.status === "paused" ? `<button data-rollout-action="resume" data-rollout-id="${escapeHtml(rollout.id)}" type="button">Продолжить</button>` : ""}${["running", "paused"].includes(rollout.status) ? `<button class="danger" data-rollout-action="cancel" data-rollout-id="${escapeHtml(rollout.id)}" type="button">Отменить</button>` : ""}</div></div><ul class="rollout-devices">${devices}</ul></article>`;
  }).join("") || inlineStateMarkup("Нет rollout", "Создайте первый rollout для явного списка устройств.");
}

els.rolloutForm.addEventListener("submit", (event) => {
  event.preventDefault();
  const device_ids = els.rolloutDeviceIds.value.split(",").map((value) => value.trim()).filter(Boolean);
  api("/api/agent-rollouts", { method: "POST", body: JSON.stringify({ device_ids, channel: els.rolloutChannel.value, batch_size: Number(els.rolloutBatchSize.value), failure_threshold: Number(els.rolloutFailureThreshold.value) }) }).then(() => { setFormMessage(els.rolloutMessage, "Rollout создан", "success"); return loadRollouts(); }).catch((error) => setFormMessage(els.rolloutMessage, error.message, "error"));
});
els.rolloutList.addEventListener("click", async (event) => {
  const button = event.target.closest("[data-rollout-action]"); if (!button) return;
  const action = button.dataset.rolloutAction;
  const endpoint = `/api/agent-rollouts/${encodeURIComponent(button.dataset.rolloutId)}/${action}`;
  if (action === "cancel") {
    const confirmed = await confirmAction({
      variant: "warning",
      context: "ADMIN / ROLLOUT",
      title: "Отменить rollout?",
      message: "Новые batches больше не будут запущены. Уже выполняющиеся обновления завершатся согласно серверной логике.",
      values: [["Rollout", button.dataset.rolloutId]],
      confirmLabel: "Отменить rollout",
      loadingLabel: "Отмена…",
      onConfirm: () => api(endpoint, { method: "POST" }),
      failureMessage: "Не удалось отменить rollout.",
    });
    if (!confirmed) return;
    await loadRollouts();
    return;
  }
  api(endpoint, { method: "POST" }).then(loadRollouts).catch((error) => setFormMessage(els.rolloutMessage, error.message, "error"));
});

function closeProfile() {
  if (els.profileDialog.open) els.profileDialog.close();
  setMobileRoute(state.previousMobileRoute || "fleet");
}

function setFormMessage(element, message, tone = "") {
  element.textContent = message;
  element.className = `form-message${tone ? ` is-${tone}` : ""}`;
}

function setSubmitting(form, button, submitting, busyLabel = "Сохранение…") {
  if (!form || !button) return;
  if (!button.dataset.idleLabel) button.dataset.idleLabel = button.textContent.trim();
  form.setAttribute("aria-busy", String(submitting));
  button.disabled = submitting;
  button.textContent = submitting ? busyLabel : button.dataset.idleLabel;
}

async function saveProfile() {
  setFormMessage(els.profileMessage, "Сохраняем…");
  setSubmitting(els.profileForm, els.profileSubmitBtn, true);
  try {
    const response = await api("/api/auth/profile", {
      method: "PATCH",
      body: JSON.stringify({
        display_name: els.profileDisplayName.value.trim(),
        email: els.profileEmail.value.trim(),
      }),
    });
    showApp(response.user);
    await loadNotificationPreferences();
    setFormMessage(els.profileMessage, "Профиль сохранён", "success");
  } finally {
    setSubmitting(els.profileForm, els.profileSubmitBtn, false);
  }
}

async function loadNotificationPreferences() {
  setFormMessage(els.notificationSettingsMessage, "Загружаем настройки…");
  const [settingsResponse, historyResponse] = await Promise.all([
    api("/api/notifications/settings"),
    fetchNotificationHistory(),
  ]);
  state.notificationSettings = settingsResponse.settings || {};
  state.notificationChannels = settingsResponse.channels || {};
  state.notifications = historyResponse.notifications || [];
  state.notificationMetrics = historyResponse.metrics || {};
  renderNotificationSettings();
  renderNotificationDeviceFilters();
  renderNotificationMetrics();
  renderNotificationChannelDiagnostics();
  renderNotificationHistory();
  setFormMessage(els.notificationSettingsMessage, state.notificationSettings.configured ? "Настройки загружены" : "Каналы пока выключены");
}

function renderNotificationSettings() {
  const settings = state.notificationSettings || {};
  const email = state.notificationChannels.email || {};
  const telegram = state.notificationChannels.telegram || {};
  els.notificationEmailEnabled.checked = Boolean(settings.email_enabled);
  els.notificationEmailEnabled.disabled = !email.available || !email.profile_email_configured;
  els.verifyEmailBtn.disabled = !email.available || !email.profile_email_configured;
  els.confirmEmailBtn.disabled = !email.available || !email.profile_email_configured;
  els.notificationEmailHint.textContent = !email.available
    ? "SMTP не настроен на сервере"
    : !email.profile_email_configured
      ? "Сначала добавьте e-mail в профиль"
      : `${email.verified ? "Подтверждён" : "Требуется подтверждение"} · ${email.destination || "e-mail профиля"}`;
  const emailState = !email.available || !email.profile_email_configured ? "NOT CONFIGURED" : email.verified ? "VERIFIED" : "PENDING VERIFICATION";
  els.notificationEmailState.textContent = emailState;
  els.notificationEmailState.className = `status ${email.verified ? "success" : email.available && email.profile_email_configured ? "warning" : "neutral"}`;
  els.notificationEmailChannel.dataset.state = emailState.toLowerCase().replaceAll(" ", "-");
  els.notificationTelegramEnabled.checked = Boolean(settings.telegram_enabled);
  els.notificationTelegramEnabled.disabled = !telegram.available;
  els.verifyTelegramBtn.disabled = !telegram.available;
  els.confirmTelegramBtn.disabled = !telegram.available;
  els.notificationTelegramHint.textContent = telegram.available ? (telegram.verified ? "Telegram подтверждён" : "Подтвердите кодом из сообщения") : "Bot token не настроен на сервере";
  const telegramState = !telegram.available ? "NOT CONFIGURED" : telegram.verified ? "VERIFIED" : "PENDING VERIFICATION";
  els.notificationTelegramState.textContent = telegramState;
  els.notificationTelegramState.className = `status ${telegram.verified ? "success" : telegram.available ? "warning" : "neutral"}`;
  els.notificationTelegramChannel.dataset.state = telegramState.toLowerCase().replaceAll(" ", "-");
  els.notificationTelegramChatId.value = settings.telegram_chat_id || "";
  els.notificationTelegramChatId.disabled = !telegram.available;
  els.notificationTelegramChatRow.classList.toggle("is-disabled", !telegram.available);
  els.notificationWarningEnabled.checked = settings.notify_warning !== false;
  els.notificationCriticalEnabled.checked = settings.notify_critical !== false;
  els.notificationResolvedEnabled.checked = settings.notify_resolved !== false;
  els.notificationMemoryThreshold.value = settings.memory_threshold_percent || 85;
  els.notificationDiskThreshold.value = settings.disk_threshold_percent || 85;
  els.notificationPacketLossThreshold.value = settings.packet_loss_percent || 20;
  els.notificationLatencyThreshold.value = settings.latency_threshold_ms || 200;
  els.notificationRepeatMinutes.value = String(settings.repeat_minutes || 0);
  els.notificationTimezone.value = settings.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  els.notificationPausedUntil.value = dateTimeLocalValue(settings.alerts_paused_until);
  els.notificationQuietEnabled.checked = Boolean(settings.quiet_hours_enabled);
  els.notificationQuietStart.value = settings.quiet_hours_start || "22:00";
  els.notificationQuietEnd.value = settings.quiet_hours_end || "08:00";
  els.notificationWebhookEnabled.checked = Boolean(settings.webhook_enabled);
  els.notificationWebhookUrl.value = settings.webhook_url || "";
  els.notificationWebhookSecret.value = "";
  els.notificationWebhookSecret.placeholder = settings.webhook_secret_configured ? "Секрет уже настроен" : "Не менее 32 символов";
  renderNotificationDeviceOptions();
}

async function saveNotificationSettings() {
  setFormMessage(els.notificationSettingsMessage, "Сохраняем…");
  setSubmitting(els.notificationSettingsForm, els.notificationSubmitBtn, true);
  try {
    const response = await api("/api/notifications/settings", {
      method: "PUT",
      body: JSON.stringify({
        email_enabled: els.notificationEmailEnabled.checked,
        telegram_enabled: els.notificationTelegramEnabled.checked,
        telegram_chat_id: els.notificationTelegramChatId.value.trim(),
        notify_warning: els.notificationWarningEnabled.checked,
        notify_critical: els.notificationCriticalEnabled.checked,
        notify_resolved: els.notificationResolvedEnabled.checked,
        memory_threshold_percent: Number(els.notificationMemoryThreshold.value),
        disk_threshold_percent: Number(els.notificationDiskThreshold.value),
        packet_loss_percent: Number(els.notificationPacketLossThreshold.value),
        latency_threshold_ms: Number(els.notificationLatencyThreshold.value),
        repeat_minutes: Number(els.notificationRepeatMinutes.value),
        timezone: els.notificationTimezone.value.trim() || "UTC",
        alerts_paused_until: isoFromDateTimeLocal(els.notificationPausedUntil.value),
        quiet_hours_enabled: els.notificationQuietEnabled.checked,
        quiet_hours_start: els.notificationQuietStart.value || "22:00",
        quiet_hours_end: els.notificationQuietEnd.value || "08:00",
        webhook_enabled: els.notificationWebhookEnabled.checked,
        webhook_url: els.notificationWebhookUrl.value.trim(),
        webhook_secret: els.notificationWebhookSecret.value,
      }),
    });
    state.notificationSettings = response.settings || {};
    state.notificationChannels = response.channels || {};
    renderNotificationSettings();
    setFormMessage(els.notificationSettingsMessage, "Настройки уведомлений сохранены", "success");
  } finally {
    setSubmitting(els.notificationSettingsForm, els.notificationSubmitBtn, false);
  }
}

async function testNotifications() {
  els.testNotificationsBtn.disabled = true;
  setFormMessage(els.notificationSettingsMessage, "Отправляем тест…");
  try {
    const response = await api("/api/notifications/test", { method: "POST" });
    const deliveries = response.notifications || [];
    const sent = deliveries.filter((item) => item.status === "sent").length;
    const failed = deliveries.length - sent;
    await loadNotificationHistory();
    setFormMessage(
      els.notificationSettingsMessage,
      failed ? `Отправлено: ${sent}, с ошибкой: ${failed}` : `Тест отправлен по каналам: ${sent}`,
      failed ? "error" : "success",
    );
  } finally {
    els.testNotificationsBtn.disabled = false;
  }
}

async function loadNotificationHistory() {
  const response = await fetchNotificationHistory();
  state.notifications = response.notifications || [];
  state.notificationMetrics = response.metrics || {};
  renderNotificationMetrics();
  renderNotificationChannelDiagnostics();
  renderNotificationHistory();
}

function fetchNotificationHistory() {
  const query = new URLSearchParams({ limit: "100" });
  const filters = [
    ["device_id", els.notificationFilterDevice.value],
    ["severity", els.notificationFilterSeverity.value],
    ["event", els.notificationFilterEvent.value],
    ["channel", els.notificationFilterChannel.value],
    ["status", els.notificationFilterStatus.value],
  ];
  for (const [name, value] of filters) {
    if (value) query.set(name, value);
  }
  return api(`/api/notifications?${query.toString()}`);
}

function renderNotificationDeviceFilters() {
  const current = els.notificationFilterDevice.value;
  els.notificationFilterDevice.innerHTML = '<option value="">Все роутеры</option>';
  for (const device of state.devices) {
    const option = document.createElement("option");
    option.value = device.id;
    option.textContent = deviceDisplayName(device);
    els.notificationFilterDevice.appendChild(option);
  }
  if (state.devices.some((device) => device.id === current)) {
    els.notificationFilterDevice.value = current;
  }
}

function renderNotificationMetrics() {
  const metrics = state.notificationMetrics || {};
  const queueAge = Number(metrics.oldest_queue_age_seconds || 0);
  const cards = [
    ["В очереди", Number(metrics.queued || 0), queueAge > 0 ? `старейшая ${formatDurationSeconds(queueAge)}` : "очередь пуста", "queued"],
    ["Доставлено", Number(metrics.sent || 0), "за период хранения", "sent"],
    ["С ошибкой", Number(metrics.failed || 0), "ожидают повтора", "failed"],
    ["Dead letter", Number(metrics.dead_letter || 0), "требуют внимания", "dead-letter"],
  ];
  els.notificationMetrics.innerHTML = cards.map(([label, value, note, tone]) => `
    <article class="notification-metric is-${tone}">
      <span>${escapeHtml(label)}</span>
      <strong>${value}</strong>
      <small>${escapeHtml(note)}</small>
    </article>
  `).join("");
}

function formatDurationSeconds(value) {
  const seconds = Math.max(0, Number(value || 0));
  if (seconds < 60) return `${Math.round(seconds)} сек`;
  if (seconds < 3600) return `${Math.round(seconds / 60)} мин`;
  if (seconds < 86400) return `${Math.round(seconds / 3600)} ч`;
  return `${Math.round(seconds / 86400)} дн`;
}

function renderNotificationChannelDiagnostics() {
  const channelNames = { email: "E-mail / SMTP", telegram: "Telegram", webhook: "Webhook" };
  const latestMetrics = new Map((state.notificationMetrics.channels || []).map((item) => [item.channel, item]));
  els.notificationChannelDiagnostics.innerHTML = Object.entries(channelNames).map(([channel, name]) => {
    const diagnostic = state.notificationChannels[channel] || {};
    const delivery = latestMetrics.get(channel) || diagnostic.delivery || {};
    const details = [];
    if (delivery.last_success_at) details.push(`Последняя доставка: ${formatDate(delivery.last_success_at)}`);
    if (delivery.last_error_at) details.push(`Последняя ошибка: ${formatDate(delivery.last_error_at)}`);
    const technicalError = delivery.last_error || "";
    const statusCode = diagnostic.status === "available" || diagnostic.status === "verified" ? "READY" : diagnostic.status === "error" ? "ERROR" : "NOT CONFIGURED";
    const statusTone = statusCode === "READY" ? "success" : statusCode === "ERROR" ? "danger" : "neutral";
    return `
      <article class="notification-channel-diagnostic is-${escapeHtml(diagnostic.status || "disabled")}">
        <div><strong>${escapeHtml(name)}</strong><span>${escapeHtml(diagnostic.message || "Состояние неизвестно")}</span></div>
        <span class="status ${statusTone}">${statusCode}</span>
        <small>${escapeHtml(details.join(" · ") || "Истории доставки пока нет")}</small>
        ${technicalError ? `<details class="technical-details"><summary>Technical details</summary><pre>${escapeHtml(technicalError)}</pre></details>` : ""}
      </article>
    `;
  }).join("");
}

function renderNotificationHistory() {
  els.notificationHistory.innerHTML = "";
  if (!state.notifications.length) {
    els.notificationHistory.innerHTML = inlineStateMarkup("Отправок пока нет", "После первой проблемы или теста здесь появится результат доставки.");
    return;
  }
  for (const delivery of state.notifications) {
    const row = document.createElement("article");
    row.className = `notification-history-row is-${delivery.status || "queued"}`;
    const attempts = delivery.attempt_count > 0 ? ` · попытка ${delivery.attempt_count}/${delivery.max_attempts || "?"}` : "";
    const retryAt = delivery.status === "retry" && delivery.next_attempt_at
      ? ` · повтор ${formatDate(delivery.next_attempt_at)}`
      : "";
    const device = state.devices.find((item) => item.id === delivery.device_id);
    const channelLabel = { email: "E-mail", telegram: "Telegram", webhook: "Webhook" }[delivery.channel] || delivery.channel;
    const context = [
      device ? deviceDisplayName(device) : "",
      delivery.severity === "critical" ? "Критическое" : delivery.severity === "warning" ? "Предупреждение" : "",
      notificationEventLabel(delivery.event),
    ].filter(Boolean).join(" · ");
    row.innerHTML = `
      <span class="notification-channel" data-label="Канал">${escapeHtml(channelLabel)}</span>
      <div data-label="Событие / объект"><strong>${escapeHtml(delivery.title || "Уведомление")}</strong><small>${escapeHtml([context, delivery.destination || ""].filter(Boolean).join(" · "))}${escapeHtml(attempts)}${escapeHtml(retryAt)}</small>${delivery.error ? `<details class="technical-details"><summary>Ошибка доставки</summary><pre>${escapeHtml(delivery.error)}</pre></details>` : ""}</div>
      <span data-label="Статус"><span class="status ${notificationStatusTone(delivery.status)}">${escapeHtml(notificationStatusLabel(delivery.status))}</span></span>
      <time data-label="Время">${escapeHtml(formatDate(delivery.sent_at || delivery.created_at))}</time>
    `;
    els.notificationHistory.appendChild(row);
  }
}

function notificationEventLabel(event) {
  return {
    active: "Возникло",
    repeat: "Повтор",
    resolved: "Восстановлено",
    test: "Тест",
  }[event] || event || "";
}

function dateTimeLocalValue(value) {
  if (!value) return "";
  const date = new Date(value);
  const offset = date.getTimezoneOffset() * 60000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

async function requestContactVerification(channel) {
  const destination = channel === "telegram" ? els.notificationTelegramChatId.value.trim() : "";
  await api(`/api/notifications/verify/${channel}/request`, {
    method: "POST",
    body: JSON.stringify({ destination }),
  });
  setFormMessage(els.notificationSettingsMessage, `Код отправлен в ${channel === "email" ? "e-mail" : "Telegram"}`, "success");
}

async function confirmContactVerification(channel) {
  const input = channel === "email" ? els.emailVerificationCode : els.telegramVerificationCode;
  await api(`/api/notifications/verify/${channel}/confirm`, {
    method: "POST",
    body: JSON.stringify({ code: input.value.trim() }),
  });
  input.value = "";
  await loadNotificationPreferences();
  setFormMessage(els.notificationSettingsMessage, `${channel === "email" ? "E-mail" : "Telegram"} подтверждён`, "success");
}

function isoFromDateTimeLocal(value) {
  return value ? new Date(value).toISOString() : "";
}

function renderNotificationDeviceOptions() {
  const current = els.notificationDeviceSelect.value;
  els.notificationDeviceSelect.innerHTML = '<option value="">Выберите роутер</option>';
  for (const device of state.devices) {
    const option = document.createElement("option");
    option.value = device.id;
    option.textContent = deviceDisplayName(device);
    els.notificationDeviceSelect.appendChild(option);
  }
  if (state.devices.some((device) => device.id === current)) els.notificationDeviceSelect.value = current;
  setDeviceNotificationControlsDisabled(!els.notificationDeviceSelect.value);
}

function setDeviceNotificationControlsDisabled(disabled) {
  els.notificationDeviceEnabled.disabled = disabled;
  els.notificationDeviceWarning.disabled = disabled;
  els.notificationDeviceCritical.disabled = disabled;
  els.notificationDeviceResolved.disabled = disabled;
  els.notificationDevicePausedUntil.disabled = disabled;
  els.saveDeviceNotificationSettingsBtn.disabled = disabled;
}

async function loadDeviceNotificationSettings() {
  const deviceId = els.notificationDeviceSelect.value;
  setDeviceNotificationControlsDisabled(!deviceId);
  if (!deviceId) return;
  const response = await api(`/api/devices/${encodeURIComponent(deviceId)}/notification-settings`);
  const settings = response.settings || {};
  els.notificationDeviceEnabled.checked = settings.enabled !== false;
  els.notificationDeviceWarning.checked = settings.notify_warning !== false;
  els.notificationDeviceCritical.checked = settings.notify_critical !== false;
  els.notificationDeviceResolved.checked = settings.notify_resolved !== false;
  els.notificationDevicePausedUntil.value = dateTimeLocalValue(settings.paused_until);
}

async function saveDeviceNotificationSettings() {
  const deviceId = els.notificationDeviceSelect.value;
  if (!deviceId) {
    setFormMessage(els.notificationSettingsMessage, "Выберите роутер", "error");
    return;
  }
  await api(`/api/devices/${encodeURIComponent(deviceId)}/notification-settings`, {
    method: "PATCH",
    body: JSON.stringify({
      enabled: els.notificationDeviceEnabled.checked,
      notify_warning: els.notificationDeviceWarning.checked,
      notify_critical: els.notificationDeviceCritical.checked,
      notify_resolved: els.notificationDeviceResolved.checked,
      paused_until: isoFromDateTimeLocal(els.notificationDevicePausedUntil.value),
    }),
  });
  setFormMessage(els.notificationSettingsMessage, "Настройки роутера сохранены", "success");
}

async function loadNotificationCenter() {
  const response = await api("/api/notification-center?limit=50");
  state.inboxNotifications = response.notifications || [];
  state.notificationUnread = Number(response.unread || 0);
  renderNotificationCenter();
}

function renderNotificationCenter() {
  const unread = state.notificationUnread;
  els.notificationUnreadCount.textContent = unread > 99 ? "99+" : String(unread);
  els.notificationUnreadCount.classList.toggle("is-hidden", unread === 0);
  els.mobileNotificationUnreadCount.textContent = unread > 99 ? "99+" : String(unread);
  els.mobileNotificationUnreadCount.classList.toggle("is-hidden", unread === 0);
  els.notificationCenterSummary.textContent = unread ? `${unread} непрочитанных` : "Новых событий нет";
  els.notificationCenterList.innerHTML = "";
  if (!state.inboxNotifications.length) {
    els.notificationCenterList.innerHTML = inlineStateMarkup("Уведомлений пока нет", "Новые проблемы и восстановления появятся здесь.");
    return;
  }
  const groups = new Map();
  for (const item of state.inboxNotifications) {
    const key = item.incident_id || item.id;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(item);
  }
  for (const items of groups.values()) {
    const item = items[0];
    const button = document.createElement("button");
    button.type = "button";
    button.className = `notification-center-item severity-${item.severity || "warning"} ${item.read_at ? "" : "is-unread"}`;
    const severity = item.severity === "critical" ? "CRITICAL" : item.severity === "resolved" ? "RESOLVED" : "WARNING";
    const severityTone = item.severity === "critical" ? "danger" : item.severity === "resolved" ? "success" : "warning";
    const severityIcon = item.severity === "resolved" ? "circle-check" : "alert-triangle";
    const device = state.devices.find((entry) => entry.id === item.device_id);
    button.innerHTML = `
      <span class="notification-center-marker status ${severityTone}">${icon(severityIcon, "icon-sm")} ${severity}</span>
      <span><strong>${escapeHtml(item.title)}</strong><small>${escapeHtml([device ? deviceDisplayName(device) : "", items.length > 1 ? `${items.length} связанных событий · ${item.body}` : item.body].filter(Boolean).join(" · "))}</small></span>
      <time>${escapeHtml(formatDate(item.created_at))}</time>
    `;
    button.addEventListener("click", async () => {
      for (const current of items.filter((entry) => !entry.read_at)) {
        await api(`/api/notification-center/${encodeURIComponent(current.id)}/read`, { method: "POST" });
      }
      if (item.device_id) {
        els.notificationCenterDialog.close();
        await selectDevice(item.device_id);
      }
      await loadNotificationCenter();
    });
    els.notificationCenterList.appendChild(button);
  }
}

async function showNotificationCenter() {
  if (!els.notificationCenterDialog.open) els.notificationCenterDialog.showModal();
  els.notificationCenterList.innerHTML = inlineStateMarkup("Загрузка уведомлений", "Получаем последние события…", "loading");
  try {
    await loadNotificationCenter();
  } catch (error) {
    els.notificationCenterSummary.textContent = "Не удалось загрузить";
    els.notificationCenterList.innerHTML = inlineStateMarkup("Центр уведомлений недоступен", friendlyAPIError(error.status, error.code, error.message));
  }
}

async function markAllNotificationsRead() {
  await api("/api/notification-center/read-all", { method: "POST" });
  await loadNotificationCenter();
}

function notificationStatusLabel(status) {
  return {
    queued: "В очереди",
    sending: "Отправляется",
    retry: "Повтор ожидается",
    sent: "Отправлено",
    dead_letter: "Не доставлено",
    failed: "Ошибка",
  }[status] || status || "Неизвестно";
}

function notificationStatusTone(status) {
  return {
    queued: "neutral",
    sending: "running",
    retry: "warning",
    sent: "success",
    dead_letter: "danger",
    failed: "danger",
  }[status] || "neutral";
}

function notificationErrorMessage(error) {
  const raw = String(error && error.message ? error.message : error || "Неизвестная ошибка");
  const requestId = raw.match(/\(req_[^)]+\)$/)?.[0] || "";
  const message = raw.replace(/\s*\(req_[^)]+\)$/, "");
  const translations = {
    "enable and save at least one notification channel": "Включите и сохраните хотя бы один доступный канал",
    "no available notification channels": "На сервере нет доступных каналов уведомлений",
    "email notifications are not configured on the server": "SMTP для уведомлений не настроен на сервере",
    "add an email address to the profile before enabling email notifications": "Сначала добавьте e-mail в контактные данные профиля",
    "Telegram notifications are not configured on the server": "Telegram-бот не настроен на сервере",
    "Telegram chat ID is invalid": "Некорректный Telegram Chat ID",
    "memory and disk thresholds must be between 50 and 99 percent": "Пороги памяти и диска должны быть от 50 до 99%",
    "packet loss threshold must be between 1 and 100 percent": "Порог потерь должен быть от 1 до 100%",
    "latency threshold must be between 10 and 5000 milliseconds": "Порог задержки должен быть от 10 до 5000 мс",
    "repeat interval must be 0 or between 15 and 10080 minutes": "Интервал повтора должен быть выключен или находиться в пределах 15–10080 минут",
  };
  return `${translations[message] || message}${requestId ? ` ${requestId}` : ""}`;
}

async function changePassword() {
  setFormMessage(els.passwordMessage, "");
  if (els.newPassword.value !== els.confirmPassword.value) {
    setFormMessage(els.passwordMessage, "Новые пароли не совпадают", "error");
    return;
  }
  setSubmitting(els.passwordForm, els.passwordSubmitBtn, true, "Изменение…");
  try {
    await api("/api/auth/change-password", {
      method: "POST",
      body: JSON.stringify({ current_password: els.currentPassword.value, new_password: els.newPassword.value }),
    });
    els.passwordForm.reset();
    setFormMessage(els.passwordMessage, "Пароль изменён. Остальные сессии завершены.", "success");
  } finally {
    setSubmitting(els.passwordForm, els.passwordSubmitBtn, false);
  }
}

async function logoutAll() {
  const confirmed = await confirmAction({
    variant: "danger",
    context: "SECURITY / SESSIONS",
    title: "Завершить все активные сессии?",
    message: "Все браузеры и устройства, включая текущий, потеряют доступ к OpenWrt RMM.",
    description: "Чтобы продолжить работу, потребуется снова войти в аккаунт.",
    confirmLabel: "Завершить все сессии",
    loadingLabel: "Завершение…",
    onConfirm: () => api("/api/auth/logout-all", { method: "POST" }),
    failureMessage: "Не удалось завершить активные сессии.",
  });
  if (!confirmed) return;
  closeProfile();
  state.devices = [];
  state.user = null;
  showLogin("Все сессии завершены. Войдите снова.");
}

async function loadUsers() {
  const response = await api("/api/users");
  state.users = response.users || [];
  renderUsers();
}

function renderUsers() {
  els.userList.innerHTML = "";
  if (!state.users.length) {
    els.userList.innerHTML = inlineStateMarkup("Пользователей нет", "Создайте первую отдельную учётную запись.");
    return;
  }
  for (const user of state.users) {
    const ownAccount = state.user && user.id === state.user.id;
    const row = document.createElement("article");
    row.className = `user-row${user.disabled ? " is-disabled" : ""}`;
    row.innerHTML = `
      <div class="user-row-identity" data-label="Пользователь">
        <span class="profile-avatar">${escapeHtml((user.display_name || user.username || "U").charAt(0).toUpperCase())}</span>
        <div><strong>${escapeHtml(user.display_name || user.username)}</strong><small class="technical-value">${escapeHtml(user.username)}</small><small>${user.email ? escapeHtml(user.email) : "E-mail не задан"}</small></div>
      </div>
      <label class="user-role-field" data-label="Роль"><span>Роль</span><select data-user-role ${ownAccount ? "disabled" : ""}><option value="user" ${user.role === "user" ? "selected" : ""}>Пользователь</option><option value="admin" ${user.role === "admin" ? "selected" : ""}>Администратор</option></select></label>
      <div class="user-state" data-label="Состояние"><span class="status ${user.disabled ? "danger" : "success"}">${user.disabled ? "DISABLED" : "ACTIVE"}</span>${ownAccount ? '<small>Текущий аккаунт</small>' : ""}</div>
      <div class="user-row-actions" data-label="Действия">
        <button type="button" data-user-password ${ownAccount ? "disabled" : ""}>Новый пароль</button>
        <button type="button" data-user-disabled class="${user.disabled ? "" : "danger"}" ${ownAccount ? "disabled" : ""}>${user.disabled ? "Включить" : "Отключить"}</button>
      </div>
    `;
    row.querySelector("[data-user-role]").addEventListener("change", (event) => updateManagedUser(user, { role: event.target.value }));
    row.querySelector("[data-user-disabled]").addEventListener("click", () => updateManagedUser(user, { disabled: !user.disabled }));
    row.querySelector("[data-user-password]").addEventListener("click", () => resetManagedUserPassword(user).catch(reportError));
    els.userList.appendChild(row);
  }
}

async function updateManagedUser(user, changes) {
  const roleChange = Object.hasOwn(changes, "role");
  const disabling = Object.hasOwn(changes, "disabled") && changes.disabled;
  const enabling = Object.hasOwn(changes, "disabled") && !changes.disabled;
  const confirmed = await confirmAction({
    variant: disabling ? "danger" : "warning",
    context: "SECURITY / USER",
    title: roleChange
      ? `Изменить роль пользователя ${user.username}?`
      : `${disabling ? "Отключить" : "Включить"} пользователя ${user.username}?`,
    message: roleChange
      ? "Новая роль изменит доступ пользователя к административным функциям OpenWrt RMM."
      : disabling
        ? "Пользователь потеряет доступ к OpenWrt RMM до повторного включения аккаунта."
        : "Пользователь снова сможет входить в OpenWrt RMM.",
    values: [
      ["Account", user.username],
      ...(roleChange ? [["Current role", user.role], ["New role", changes.role]] : [["New state", enabling ? "ACTIVE" : "DISABLED"]]),
    ],
    confirmLabel: roleChange ? "Изменить роль" : disabling ? "Отключить пользователя" : "Включить пользователя",
    loadingLabel: "Сохранение…",
    onConfirm: () => api(`/api/users/${encodeURIComponent(user.id)}`, { method: "PATCH", body: JSON.stringify(changes) }),
    failureMessage: "Не удалось обновить учётную запись.",
  });
  if (!confirmed) {
    renderUsers();
    return;
  }
  await loadUsers();
  notify(`Аккаунт ${user.username} обновлён`, "success");
}

async function resetManagedUserPassword(user) {
  const password = await confirmAction({
    variant: "warning",
    context: "SECURITY / USER",
    title: `Новый временный пароль для ${user.username}`,
    message: "Текущий пароль пользователя перестанет работать после сохранения.",
    values: [["Account", user.username]],
    confirmLabel: "Сохранить пароль",
    loadingLabel: "Сохранение…",
    input: { label: "Временный пароль", type: "password", minLength: 12, autocomplete: "new-password", hint: "Минимум 12 символов", validationMessage: "Пароль должен содержать не менее 12 символов." },
    onConfirm: (value) => api(`/api/users/${encodeURIComponent(user.id)}`, { method: "PATCH", body: JSON.stringify({ password: value }) }),
    failureMessage: "Не удалось изменить временный пароль.",
  });
  if (!password) return;
  await loadUsers();
  notify(`Пароль аккаунта ${user.username} обновлён`, "success");
}

async function requestPasswordReset() {
  setFormMessage(els.forgotPasswordMessage, "Отправляем…");
  const submit = els.forgotPasswordForm.querySelector('[type="submit"]');
  setSubmitting(els.forgotPasswordForm, submit, true, "Отправка…");
  try {
    await api("/api/auth/password-reset/request", {
      method: "POST",
      body: JSON.stringify({ identifier: els.passwordResetIdentifier.value.trim() }),
    });
    els.forgotPasswordForm.reset();
    setFormMessage(els.forgotPasswordMessage, "Если e-mail восстановления настроен, ссылка уже отправлена.", "success");
  } finally {
    setSubmitting(els.forgotPasswordForm, submit, false);
  }
}

async function confirmPasswordReset() {
  setFormMessage(els.passwordResetMessage, "");
  if (els.resetNewPassword.value !== els.resetConfirmPassword.value) {
    setFormMessage(els.passwordResetMessage, "Пароли не совпадают", "error");
    return;
  }
  const submit = els.passwordResetForm.querySelector('[type="submit"]');
  setSubmitting(els.passwordResetForm, submit, true, "Сохранение…");
  try {
    await api("/api/auth/password-reset/confirm", {
      method: "POST",
      body: JSON.stringify({ token: state.passwordResetToken, new_password: els.resetNewPassword.value }),
    });
    state.passwordResetToken = "";
    history.replaceState(null, "", `${location.pathname}${location.search}`);
    els.passwordResetForm.reset();
    setFormMessage(els.passwordResetMessage, "Пароль изменён. Теперь можно войти.", "success");
  } finally {
    setSubmitting(els.passwordResetForm, submit, false);
  }
}

function openPasswordResetFromURL() {
  const match = location.hash.match(/^#password-reset=(.+)$/);
  if (!match) return;
  try {
    state.passwordResetToken = decodeURIComponent(match[1]);
  } catch {
    state.passwordResetToken = "";
  }
  if (state.passwordResetToken) els.passwordResetDialog.showModal();
}

async function runLuCIPrimaryAction() {
  if (els.luciStateDialog.open) els.luciStateDialog.close();
  if (state.luciAction === "retry") {
    await openCloudAccess();
    return;
  }
  await openDeviceArea("operations", "#remoteAccessPanel", "operations");
}

async function refreshDevicesIfIdle() {
  if (refreshDevicesIfIdle.inFlight) return;
  refreshDevicesIfIdle.inFlight = true;
  try {
    await loadDevices();
  } finally {
    refreshDevicesIfIdle.inFlight = false;
  }
}

refreshDevicesIfIdle.inFlight = false;

async function loadCommands(options = {}) {
  if (!state.selectedDeviceId) return;
  const append = Boolean(options.append);
  const offset = append ? state.commandOffset : 0;
  if (!append) els.commandList.innerHTML = inlineStateMarkup("Загрузка операций", "Получаем lifecycle и последние результаты команд.", "loading");
  try {
    const data = await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/commands?limit=${state.commandLimit}&offset=${offset}`);
    const nextCommands = data.commands || [];
    state.commands = append ? [...state.commands, ...nextCommands] : nextCommands;
    state.commandOffset = offset + nextCommands.length;
    state.commandHasMore = nextCommands.length === state.commandLimit;
    renderCommands(state.commands);
    renderCommandLoadMore();
    renderPresetReview();
    if (state.selectedCommand) {
      const refreshed = state.commands.find((command) => command.id === state.selectedCommand.id);
      state.selectedCommand = refreshed || null;
      renderCommandDetail(state.selectedCommand);
    }
  } catch (error) {
    els.commandList.innerHTML = inlineStateMarkup("Не удалось загрузить операции", error.message || "Повторите обновление.", "error");
    throw error;
  }
}

function renderCommandLoadMore() {
  els.loadMoreCommandsBtn.classList.toggle("is-hidden", !state.commandHasMore);
}

function renderCommands(commands) {
  renderAgentUpdateStatus();
  renderExpertOperationPanels(commands);
  els.commandList.innerHTML = "";
  renderCommandSummary(commands);
  const filtered = commands.filter(commandFilterMatches);
  if (filtered.length === 0) {
    els.commandList.innerHTML = commands.length
      ? inlineStateMarkup("Операции не найдены", "Измените lifecycle filter или выберите другой статус.")
      : inlineStateMarkup("История операций пуста", "Запущенные команды и диагностика появятся здесь.");
    return;
  }

  for (const command of filtered) {
    const row = document.createElement("div");
    row.className = "command-row";
    row.setAttribute("role", "listitem");
    const canCancel = command.status === "queued" || command.status === "claimed";
    const output = command.output || JSON.stringify(command.args || {});
    const finishedAt = command.completed_at || command.cancelled_at || command.expired_at;
    const lifecycleAt = finishedAt || command.claimed_at || command.created_at;
    row.innerHTML = `
      <div class="command-main">
        <strong>${escapeHtml(commandTypeLabel(command.type))}</strong>
        <small>${escapeHtml(command.type)} · ${escapeHtml(command.id)}</small>
      </div>
      <div data-label="Статус"><span class="status ${escapeHtml(command.status)}">${escapeHtml(commandStatusCode(command.status))}</span></div>
      <div class="command-attempt" data-label="Попытка"><code>${command.attempt_count || 0}/${command.max_attempts || 3}</code></div>
      <div class="lifecycle-time" data-label="Обновлено"><time datetime="${escapeHtml(lifecycleAt || "")}">${escapeHtml(formatDate(lifecycleAt))}</time></div>
      <details class="command-output" data-label="Результат">
        <summary>${escapeHtml(outputSummary(output))}</summary>
        <pre>${escapeHtml(output)}</pre>
      </details>
      <div class="row-actions">
        <button type="button" data-action="detail">Детали</button>
        <button type="button" data-action="cancel" ${canCancel ? "" : "disabled"}>Отменить</button>
      </div>
    `;
    row.querySelector('[data-action="detail"]').addEventListener("click", () => {
      state.selectedCommand = command;
      renderCommandDetail(command);
      els.commandDetailPanel.scrollIntoView({ behavior: "smooth", block: "start" });
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
  els.commandSummary.textContent = `${total} всего / ${active} выполняются / ${failed} ошибки`;
}

function renderCommandDetail(command) {
  if (!command) {
    els.commandDetailPanel.classList.add("is-hidden");
    return;
  }
  const output = command.output || JSON.stringify(command.args || {}, null, 2);
  els.commandDetailPanel.classList.remove("is-hidden");
  els.commandDetailMeta.innerHTML = `
    <div class="detail-field"><span>Команда</span><strong>${escapeHtml(commandTypeLabel(command.type))}</strong></div>
    <div class="detail-field"><span>Lifecycle</span><strong class="status ${escapeHtml(command.status)}">${escapeHtml(commandStatusCode(command.status))}</strong></div>
    <div class="detail-field"><span>Попытка</span><code>${command.attempt_count || 0}/${command.max_attempts || 3}</code></div>
    <div class="detail-field"><span>Создана</span><time>${escapeHtml(formatDate(command.created_at))}</time></div>
    <div class="detail-field"><span>Истекает</span><time>${escapeHtml(formatDate(command.expires_at))}</time></div>
    <div class="detail-field"><span>Начата</span><time>${escapeHtml(formatDate(command.claimed_at))}</time></div>
    <div class="detail-field"><span>Завершена</span><time>${escapeHtml(formatDate(command.completed_at || command.cancelled_at || command.expired_at))}</time></div>
    <div class="detail-field command-id-field"><span>Command ID</span><code>${escapeHtml(command.id)}</code></div>
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

async function loadAudit(options = {}) {
  if (!state.selectedDeviceId) return;
  const append = Boolean(options.append);
  const offset = append ? state.auditOffset : 0;
  if (!append) els.auditList.innerHTML = inlineStateMarkup("Загрузка Audit", "Получаем инженерный журнал устройства.", "loading");
  try {
    const data = await api(`/api/audit-events?device_id=${encodeURIComponent(state.selectedDeviceId)}&limit=${state.auditLimit}&offset=${offset}`);
    const events = data.audit_events || [];
    const allEvents = append ? [...state.auditEvents || [], ...events] : events;
    state.auditEvents = allEvents;
    state.auditOffset = offset + events.length;
    state.auditHasMore = events.length === state.auditLimit;
    renderAudit(allEvents);
    renderAuditLoadMore();
  } catch (error) {
    els.auditList.innerHTML = inlineStateMarkup("Не удалось загрузить Audit", error.message || "Повторите обновление.", "error");
    throw error;
  }
}

function renderAuditLoadMore() {
  els.loadMoreAuditBtn.classList.toggle("is-hidden", !state.auditHasMore);
}

function renderAudit(events) {
  els.auditList.innerHTML = "";
  els.auditSummary.textContent = `${events.length} events`;
  if (events.length === 0) {
    els.auditList.innerHTML = inlineStateMarkup("Событий аудита пока нет", "Здесь появятся действия оператора и системные события по устройству.");
    return;
  }

  for (const event of events) {
    const details = event.details && typeof event.details === "object" ? event.details : {};
    const status = details.status || details.result || (String(event.action).includes("rejected") || String(event.action).includes("failed") ? "failed" : "recorded");
    const statusCode = String(status).toUpperCase();
    const statusClass = statusCode === "FAILED" || statusCode === "REJECTED" ? "failed" : statusCode === "WARNING" ? "warning" : "completed";
    const context = event.command_id || event.device_id || details.package || details.target || "-";
    const detailEntries = Object.entries(details).filter(([, value]) => value !== "" && value !== null && value !== undefined);
    const row = document.createElement("div");
    row.className = "audit-record";
    row.setAttribute("role", "listitem");
    row.innerHTML = `
      <time data-label="Timestamp" datetime="${escapeHtml(event.created_at || "")}">${escapeHtml(formatDate(event.created_at))}</time>
      <div class="audit-action" data-label="Action"><strong>${escapeHtml(event.action)}</strong><small>${escapeHtml(event.id || "")}</small></div>
      <code data-label="Actor">${escapeHtml(event.actor || "system")}</code>
      <div class="audit-context" data-label="Object / context"><code>${escapeHtml(context)}</code>${detailEntries.length ? `<details class="audit-details"><summary>Context details</summary><pre class="technical-output">${escapeHtml(JSON.stringify(details, null, 2))}</pre></details>` : ""}</div>
      <div data-label="Result"><span class="status ${escapeHtml(statusClass)}">${escapeHtml(statusCode)}</span></div>
    `;
    els.auditList.appendChild(row);
  }
}

async function loadBackups() {
  if (!state.selectedDeviceId || !els.backupList) return;
  els.backupList.innerHTML = inlineStateMarkup("Загрузка резервных копий", "Проверяем зашифрованное хранилище.", "loading");
  try {
    const data = await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/backups`);
    state.backups = data.backups || [];
    renderBackups();
  } catch (error) {
    els.backupList.innerHTML = inlineStateMarkup("Не удалось загрузить копии", error.message || "Повторите обновление.", "error");
    throw error;
  }
}

function backupStatusLabel(status) {
  return { creating: "СОЗДАЁТСЯ", ready: "ГОТОВА", failed: "ОШИБКА" }[status] || String(status || "UNKNOWN").toUpperCase();
}

function renderBackups() {
  if (!els.backupList) return;
  els.backupList.replaceChildren();
  if (!state.backups.length) {
    els.backupList.innerHTML = inlineStateMarkup("Резервных копий пока нет", "Создайте первую зашифрованную копию конфигурации роутера.");
    return;
  }
  for (const backup of state.backups) {
    const files = Array.isArray(backup.manifest) ? backup.manifest : [];
    const row = document.createElement("article");
    row.className = "backup-row";
    row.setAttribute("role", "listitem");
    row.innerHTML = `
      <div class="backup-row-main"><strong>${escapeHtml(formatDate(backup.created_at))}</strong><span>${escapeHtml(backup.model || "OpenWrt router")}</span></div>
      <div class="backup-row-meta"><span><small>Target</small><code>${escapeHtml(backup.target || "-")}</code></span><span><small>Версия</small><code>${escapeHtml(backup.openwrt_version || "-")}</code></span><span><small>Размер</small><code>${escapeHtml(formatBytes(backup.size_bytes))}</code></span></div>
      <span class="status ${backup.status === "ready" ? "success" : backup.status === "failed" ? "danger" : "warning"}">${escapeHtml(backupStatusLabel(backup.status))}</span>
      <div class="row-actions backup-actions"><button type="button" data-backup-restore ${backup.status === "ready" ? "" : "disabled"}>Восстановить</button><button type="button" data-backup-delete class="danger">Удалить</button></div>
      <details class="backup-manifest"><summary>Состав копии · ${files.length} файлов</summary><pre>${escapeHtml(files.slice(0, 250).join("\n") || backup.error || "Manifest пока недоступен")}</pre></details>
    `;
    row.querySelector("[data-backup-restore]").addEventListener("click", () => restoreBackup(backup).catch(reportError));
    row.querySelector("[data-backup-delete]").addEventListener("click", () => deleteBackup(backup).catch(reportError));
    els.backupList.appendChild(row);
  }
}

async function createBackup() {
  const device = currentDevice();
  if (!device) return;
  els.createBackupBtn.disabled = true;
  try {
    await api(`/api/devices/${encodeURIComponent(device.id)}/backups`, { method: "POST" });
    notify("Создание резервной копии поставлено в очередь", "success");
    await Promise.all([loadBackups(), loadCommands()]);
  } finally {
    els.createBackupBtn.disabled = false;
  }
}

async function restoreBackup(backup) {
  const device = currentDevice();
  if (!device) return;
  const confirmed = await confirmAction({
    variant: "danger",
    context: "RECOVERY / ROUTER",
    title: "Восстановить конфигурацию роутера?",
    message: "Сетевые настройки могут измениться. Агент проверит target и SHA-256, сохранит аварийную локальную копию и автоматически откатится, если облако не подтвердит связь за 10 минут.",
    values: [["Router", deviceDisplayName(device)], ["Target", backup.target], ["Backup", formatDate(backup.created_at)], ["SHA-256", backup.sha256]],
    confirmLabel: "Восстановить",
    loadingLabel: "Постановка в очередь…",
    onConfirm: () => api(`/api/devices/${encodeURIComponent(device.id)}/backups/${encodeURIComponent(backup.id)}/restore`, { method: "POST" }),
    failureMessage: "Не удалось запустить восстановление.",
  });
  if (!confirmed) return;
  notify("Восстановление поставлено в очередь", "success");
  await loadCommands();
}

async function deleteBackup(backup) {
  const device = currentDevice();
  if (!device) return;
  const confirmed = await confirmAction({
    variant: "danger", context: "BACKUP STORAGE", title: "Удалить резервную копию?",
    message: "Зашифрованный архив и metadata будут удалены без возможности восстановления.",
    values: [["Backup", formatDate(backup.created_at)], ["Target", backup.target]], confirmLabel: "Удалить копию", loadingLabel: "Удаление…",
    onConfirm: () => api(`/api/devices/${encodeURIComponent(device.id)}/backups/${encodeURIComponent(backup.id)}`, { method: "DELETE" }),
    failureMessage: "Не удалось удалить резервную копию.",
  });
  if (!confirmed) return;
  notify("Резервная копия удалена", "success");
  await loadBackups();
}

async function rotateDeviceCredential() {
  const device = currentDevice();
  if (!device) return;
  const confirmed = await confirmAction({
    variant: "warning", context: "SECURITY / AGENT", title: "Сменить токен агента?",
    message: "Старый токен останется действителен только до подтверждения нового агентом. Мониторинг не должен прерываться.",
    values: [["Router", deviceDisplayName(device)]], confirmLabel: "Начать ротацию", loadingLabel: "Ротация…",
    onConfirm: () => api(`/api/devices/${encodeURIComponent(device.id)}/credentials/rotate`, { method: "POST" }),
    failureMessage: "Не удалось начать ротацию.",
  });
  if (!confirmed) return;
  setFormMessage(els.credentialActionMessage, "Новый токен ожидает подтверждения агентом.", "success");
  notify("Ротация токена начата", "success");
}

async function revokeDeviceCredential() {
  const device = currentDevice();
  if (!device) return;
  const confirmed = await confirmAction({
    variant: "danger", context: "SECURITY / EMERGENCY", title: "Немедленно отозвать доступ агента?",
    message: "Агент перестанет подключаться, а все активные SSH/LuCI-сессии и временные ссылки будут закрыты. Для возврата потребуется повторная регистрация роутера.",
    values: [["Router", deviceDisplayName(device)], ["Device ID", device.id]], confirmLabel: "Отозвать доступ", loadingLabel: "Отзыв…",
    input: { label: "Введите REVOKE", type: "text", expected: "REVOKE", validationMessage: "Введите REVOKE заглавными буквами." },
    onConfirm: () => api(`/api/devices/${encodeURIComponent(device.id)}/credentials/revoke`, { method: "POST" }),
    failureMessage: "Не удалось отозвать доступ.",
  });
  if (!confirmed) return;
  setFormMessage(els.credentialActionMessage, "Доступ отозван. Требуется повторная регистрация агента.", "success");
  notify("Доступ агента отозван", "success");
  await loadDevices();
}

async function downloadDatabaseSnapshot() {
  const response = await fetch("/api/admin/database-snapshot", { credentials: "same-origin" });
  if (!response.ok) throw new Error(`Не удалось создать snapshot БД: HTTP ${response.status}`);
  const blob = await response.blob();
  const disposition = response.headers.get("Content-Disposition") || "";
  const filename = disposition.match(/filename="([^"]+)"/)?.[1] || "rmm-snapshot.db";
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
  notify("Консистентный snapshot БД создан", "success");
}

async function loadRemoteSessions() {
  if (!state.selectedDeviceId) return;
  els.remoteSessionList.innerHTML = inlineStateMarkup("Загрузка remote sessions", "Проверяем состояние временных туннелей.", "loading");
  try {
    const data = await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/remote-sessions?limit=25`);
    state.remoteSessions = data.remote_sessions || [];
    renderRemoteSessions(state.remoteSessions);
  } catch (error) {
    els.remoteSessionList.innerHTML = inlineStateMarkup("Не удалось загрузить remote sessions", error.message || "Повторите обновление.", "error");
    throw error;
  }
}

function remotePresentationState(session) {
  if (!session) return "closed";
  if (["requested", "queued", "creating"].includes(session.status)) return "creating";
  if (session.status === "active") {
    const remaining = new Date(session.expires_at).getTime() - Date.now();
    return Number.isFinite(remaining) && remaining > 0 && remaining <= 5 * 60 * 1000 ? "expiring" : "active";
  }
  if (["expired", "failed", "closed"].includes(session.status)) return session.status;
  return session.status || "closed";
}

function remoteRemainingTime(session) {
  if (!session || !session.expires_at) return "-";
  const expiry = new Date(session.expires_at).getTime();
  if (!Number.isFinite(expiry)) return "-";
  if (expiry <= Date.now()) return "Завершена";
  return formatDuration(new Date(), expiry);
}

function renderRemoteSessions(sessions) {
  els.remoteSessionList.innerHTML = "";
  const active = sessions.filter((session) => ["requested", "queued", "active"].includes(session.status)).length;
  els.remoteSummary.textContent = active ? `${active} active` : "Нет активных сессий";
  renderCloudAccessState(sessions);
  if (sessions.length === 0) {
    els.remoteSessionList.innerHTML = inlineStateMarkup("Remote sessions отсутствуют", "Временный доступ создаётся только по явному запросу оператора.");
    return;
  }
  for (const session of sessions) {
    const canClose = ["requested", "queued", "active"].includes(session.status);
    const endpoint = `${session.server_host || "-"}:${session.remote_port || "-"}`;
    const connectCommand = session.remote_port ? `ssh -p ${session.remote_port} root@${session.server_host || "server"}` : "-";
    const canOpenLuCI = session.status === "active" && session.luci_port;
    const presentationState = remotePresentationState(session);
    const sessionType = session.luci_port ? "SSH + LuCI" : "SSH tunnel";
    const row = document.createElement("div");
    row.className = "remote-session-row";
    row.setAttribute("role", "listitem");
    row.innerHTML = `
      <div class="remote-session-main" data-label="Сессия">
        <strong>${escapeHtml(sessionType)}</strong>
        <small>${escapeHtml(session.id || "temporary access")}</small>
      </div>
      <div class="remote-destination" data-label="Destination"><code title="${escapeHtml(connectCommand)}">${escapeHtml(endpoint)}</code><small>${session.luci_port ? `LuCI :${escapeHtml(session.luci_port)}` : "SSH only"}</small></div>
      <div data-label="Создана"><time>${escapeHtml(formatDate(session.created_at))}</time></div>
      <div data-label="Осталось"><code>${escapeHtml(remoteRemainingTime(session))}</code></div>
      <div data-label="Статус"><span class="remote-session-status status ${escapeHtml(presentationState)}"><i aria-hidden="true"></i>${escapeHtml(remoteStatusLabel(presentationState))}</span></div>
      <div class="row-actions remote-session-actions" data-label="Действия">
        <button type="button" data-action="copy" ${session.remote_port ? "" : "disabled"}>Копировать SSH</button>
        <button class="primary" type="button" data-action="luci" ${canOpenLuCI ? "" : "disabled"}>Открыть LuCI</button>
        <button type="button" data-action="close" ${canClose ? "" : "disabled"}>Закрыть</button>
      </div>
    `;
    row.querySelector('[data-action="luci"]').addEventListener("click", () => {
      openLuCIAccess(session).catch(reportError);
    });
    row.querySelector('[data-action="copy"]').addEventListener("click", async () => {
      await navigator.clipboard.writeText(connectCommand);
      notify("SSH-команда скопирована", "success");
    });
    row.querySelector('[data-action="close"]').addEventListener("click", () => closeRemoteSession(session.id));
    els.remoteSessionList.appendChild(row);
  }
}

function renderCloudAccessState(sessions) {
  const device = currentDevice();
  const session = sessions.find((item) => ["requested", "queued", "active"].includes(item.status));
  let accessState = session && session.access_state ? session.access_state : "closed";
  if (!device || !device.online) accessState = "offline";
  const presentationState = accessState === "ready"
    ? remotePresentationState(session || { status: "active" })
    : accessState === "starting" ? "creating" : accessState === "unavailable" ? "failed" : accessState;
  const copy = {
    ready: ["Защищённый канал готов", "Открыть LuCI"],
    starting: ["Создаём защищённый канал…", "Проверить"],
    unavailable: ["Канал не открылся — нужна диагностика", "Повторить"],
    offline: ["Роутер не на связи", "Проверить"],
    closed: ["Соединение создаётся только по вашему запросу", "Открыть LuCI"],
  }[accessState] || ["Состояние соединения неизвестно", "Проверить"];
  els.cloudAccessCard.dataset.state = accessState;
  els.cloudAccessStatus.textContent = copy[0];
  els.openCloudAccessBtn.textContent = copy[1];
  els.cloudAccessState.className = `status ${presentationState}`;
  els.cloudAccessState.textContent = remoteStatusLabel(presentationState);
}

function prepareCloudAccessPopup(popup) {
  if (!popup) return;
  try {
    popup.document.open();
    popup.document.write(`<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Подключение к LuCI — OpenWrt RMM</title><link rel="stylesheet" href="/styles.css?v=29"></head><body class="cloud-wait-page"><main><span class="cloud-wait-mark">R</span><p class="eyebrow">Защищённый доступ</p><h1>Подключаемся к LuCI</h1><p>Создаём временный туннель и проверяем ответ роутера. Эта вкладка откроется автоматически.</p><div class="cloud-wait-progress" aria-label="Подключение выполняется"><i></i></div></main></body></html>`);
    popup.document.close();
    popup.opener = null;
  } catch {
    // The popup may be restricted by the browser; navigation below still works.
  }
}

async function openCloudAccess() {
  if (!state.selectedDeviceId || openCloudAccess.inFlight) return;
  openCloudAccess.inFlight = true;
  const popup = window.open("", "_blank");
  prepareCloudAccessPopup(popup);
  els.openLuciBtn.disabled = true;
  els.openCloudAccessBtn.disabled = true;
  els.openLuciBtn.classList.add("is-loading");
  els.openCloudAccessBtn.classList.add("is-loading");
  els.openLuciBtn.textContent = "Подключаемся…";
  els.openCloudAccessBtn.textContent = "Подключаемся…";
  els.cloudAccessCard.dataset.state = "starting";
  els.cloudAccessStatus.textContent = "Связываемся с агентом и проверяем LuCI…";
  els.cloudAccessState.className = "status creating";
  els.cloudAccessState.textContent = "CREATING";
  try {
    let response = await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/cloud-access`, { method: "POST" });
    for (let attempt = 0; response.status === "starting" && attempt < 10; attempt += 1) {
      await new Promise((resolve) => window.setTimeout(resolve, 1500));
      response = await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/cloud-access`);
    }
    if (response.status !== "ready") {
      const error = new Error("cloud tunnel is not ready");
      error.status = response.status === "unavailable" ? 503 : 409;
      error.code = response.status;
      throw error;
    }
    if (!response.url) {
      response = await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/cloud-access`, { method: "POST" });
    }
    if (popup) {
      popup.opener = null;
      popup.location = response.url;
    } else {
      window.location.assign(response.url);
    }
    await loadRemoteSessions();
  } catch (error) {
    if (popup) popup.close();
    showLuCIState(error, "retry");
    await loadRemoteSessions().catch(() => {});
  } finally {
    openCloudAccess.inFlight = false;
    els.openLuciBtn.classList.remove("is-loading");
    els.openCloudAccessBtn.classList.remove("is-loading");
    els.openLuciBtn.textContent = "Открыть LuCI";
    renderCloudAccessState(state.remoteSessions || []);
    els.openLuciBtn.disabled = !state.selectedDeviceId;
    els.openCloudAccessBtn.disabled = false;
  }
}

openCloudAccess.inFlight = false;

async function sendCommand() {
  if (!state.selectedDeviceId) return;
  const type = els.commandType.value;
  const args = commandArgs();
  const queue = async () => {
    setStatus("Creating command");
    await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/commands`, {
      method: "POST",
      body: JSON.stringify({ type, args }),
    });
    await Promise.all([loadCommands(), loadAudit()]);
  };
  if (dangerLabel(type)) {
    if (!await confirmDanger(type, args, queue)) return;
  } else {
    await queue();
  }
  notify("Command queued", "success");
}

async function queueAgentUpdate() {
  const device = currentDevice();
  if (!device || !hasAgentUpdateAvailable(device)) return;
  const targetVersion = stableAgentVersion();
  if (!await confirmAction({
    variant: "warning",
    context: "SYSTEM / AGENT",
    title: `Обновить агент до ${targetVersion}?`,
    message: "Во время установки связь с роутером может кратковременно прерваться.",
    values: [["Device", deviceDisplayName(device)], ["Target version", targetVersion], ["Current version", deviceAgentVersion(device) || "Not reported"]],
    confirmLabel: "Добавить обновление в очередь",
  })) return;
  els.updateAgentBtn.disabled = true;
  els.updateAgentBtn.textContent = "Добавление в очередь...";
  try {
    const command = await api(`/api/devices/${encodeURIComponent(device.id)}/agent-update`, { method: "POST" });
    state.commands = [command, ...state.commands.filter((item) => item.id !== command.id)];
    renderCommands(state.commands);
    notify("Обновление агента добавлено в очередь", "success");
    await Promise.all([loadCommands(), loadAudit()]);
  } finally {
    els.updateAgentBtn.disabled = false;
    els.updateAgentBtn.textContent = "Обновить агент";
  }
}

function historicalAgentManifestURLs(version) {
  const configured = state.releaseMetadata && state.releaseMetadata.update_manifest_url;
  if (!configured) throw new Error("Сервер не сообщил адрес подписанного manifest");
  const manifest = new URL(configured, location.origin);
  const directory = manifest.pathname.slice(0, manifest.pathname.lastIndexOf("/") + 1);
  const encodedVersion = encodeURIComponent(version);
  manifest.pathname = `${directory}manifests/${encodedVersion}/update-manifest.json`;
  manifest.search = "";
  manifest.hash = "";
  const signature = new URL(manifest.href);
  signature.pathname = signature.pathname.replace(/\.json$/, ".sig");
  return { manifest_url: manifest.href, signature_url: signature.href };
}

function openAgentRollbackDialog() {
  const device = currentDevice();
  if (!device || !state.user || state.user.role !== "admin") return;
  const currentVersion = deviceAgentVersion(device);
  els.agentRollbackVersion.value = "";
  els.agentRollbackPreview.textContent = `Сейчас установлена версия ${currentVersion || "не определена"}.`;
  setFormMessage(els.agentRollbackMessage, "", "");
  els.agentRollbackDialog.showModal();
  els.agentRollbackVersion.focus();
}

async function queueAgentRollback(event) {
  event.preventDefault();
  const device = currentDevice();
  if (!device || !state.user || state.user.role !== "admin") return;
  const targetVersion = els.agentRollbackVersion.value.trim();
  const currentVersion = deviceAgentVersion(device);
  const comparison = globalThis.RMMVersions && currentVersion ? globalThis.RMMVersions.compareSemVer(targetVersion, currentVersion) : null;
  if (comparison === null || comparison >= 0) {
    setFormMessage(els.agentRollbackMessage, "Целевая версия должна быть корректной и ниже установленной.", "error");
    return;
  }
  const urls = historicalAgentManifestURLs(targetVersion);
  els.agentRollbackPreview.textContent = urls.manifest_url;
  if (!await confirmAction({
    variant: "danger",
    context: "SYSTEM / AGENT",
    title: `Откатить агент до ${targetVersion}?`,
    message: "Устройство установит более раннюю подписанную версию агента.",
    description: "Rollout будет остановлен, если роутер не подтвердит reconnect.",
    values: [["Device", deviceDisplayName(device)], ["Current", currentVersion], ["Target", targetVersion]],
    confirmLabel: "Добавить откат в очередь",
  })) return;
  const submit = els.agentRollbackForm.querySelector('button[type="submit"]');
  submit.disabled = true;
  try {
    const command = await api(`/api/devices/${encodeURIComponent(device.id)}/agent-rollback`, { method: "POST", body: JSON.stringify(urls) });
    state.commands = [command, ...state.commands.filter((item) => item.id !== command.id)];
    renderCommands(state.commands);
    els.agentRollbackDialog.close();
    notify("Откат агента добавлен в очередь", "success");
    await Promise.all([loadCommands(), loadAudit()]);
  } catch (error) {
    setFormMessage(els.agentRollbackMessage, error.message, "error");
  } finally {
    submit.disabled = false;
  }
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
  if (!options.skipConfirm && !await confirmDanger(type, args)) return;
  const command = await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/commands`, {
    method: "POST",
    body: JSON.stringify({ type, args }),
  });
  if (!options.skipRefresh) {
    await Promise.all([loadCommands(), loadAudit(), loadAlerts()]);
  }
  return command;
}

async function sendBulkCommand() {
  const devices = filteredDevices();
  if (devices.length === 0) {
    setStatus("No filtered devices");
    return;
  }
  const type = els.bulkCommandType.value;
  const args = bulkCommandArgs();
  const queue = async () => {
    setStatus(`Queueing ${type} for ${devices.length} devices`);
    await api("/api/devices/bulk-commands", {
      method: "POST",
      body: JSON.stringify({ device_ids: devices.map((device) => device.id), type, args }),
    });
    await Promise.all([loadCommands(), loadAudit()]);
  };
  if (dangerLabel(type)) {
    if (!await confirmDanger(type, { ...args, target_count: devices.length }, queue)) return;
  } else {
    await queue();
  }
  notify(`${type} queued for ${devices.length} devices`, "success");
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
  notify("Fleet metadata saved", "success");
}

async function sendPackageCommand() {
  if (!state.selectedDeviceId) return;
  const type = els.packageCommand.value;
  const packageName = els.packageName.value.trim();
  if ((type === "pkg_install" || type === "pkg_remove") && !packageName) {
    setFormMessage(els.packageFormMessage, "Укажите package name для этой операции.", "error");
    return;
  }
  setFormMessage(els.packageFormMessage, "");
  const args = packageName ? { package: packageName } : {};
  const queue = async () => {
    setStatus(`Queueing ${type}`);
    await createDeviceCommand(type, args, { skipConfirm: true });
  };
  if (dangerLabel(type)) {
    if (!await confirmDanger(type, args, queue)) return;
  } else {
    await queue();
  }
  setFormMessage(els.packageFormMessage, `${commandTypeLabel(type)} добавлена в очередь.`, "success");
  notify(`${type} queued`, "success");
}

async function sendUciCommand(type) {
  if (!state.selectedDeviceId) return;
  if ((type === "uci_set" || type === "uci_preview") && (!els.uciSection.value.trim() || !els.uciOption.value.trim())) {
    setFormMessage(els.uciFormMessage, "Для Preview и Stage укажите Section и Option.", "error");
    return;
  }
  setFormMessage(els.uciFormMessage, "");
  const args = type === "uci_set" || type === "uci_preview" ? uciSetArgs() : uciConfigArg();
  const queue = async () => {
    setStatus(`Queueing ${type}`);
    await createDeviceCommand(type, args, { skipConfirm: true });
  };
  if (dangerLabel(type)) {
    if (!await confirmDanger(type, args, queue)) return;
  } else {
    await queue();
  }
  setFormMessage(els.uciFormMessage, `${commandTypeLabel(type)} добавлена в очередь.`, "success");
  notify(`${type} queued`, "success");
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
  if (!await confirmAction({
    variant: "warning",
    context: "REMOTE ACCESS / SESSION",
    title: "Создать временный доступ?",
    message: "RMM поставит в очередь команду на открытие SSH tunnel к выбранному роутеру.",
    description: "Сессия автоматически завершится по таймеру.",
    values: [["Device", deviceDisplayName(currentDevice())], ["Destination", `${serverHost}:${serverPort}`], ["Duration", `${Math.round(durationSeconds / 60)} мин.`]],
    confirmLabel: "Создать сессию",
  })) return;
  setStatus("Открытие удаленного доступа");
  els.cloudAccessCard.dataset.state = "starting";
  els.cloudAccessStatus.textContent = "Создаём временный tunnel request…";
  els.cloudAccessState.className = "status creating";
  els.cloudAccessState.textContent = "CREATING";
  try {
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
    notify("Команда открытия доступа отправлена", "success");
  } catch (error) {
    els.cloudAccessCard.dataset.state = "unavailable";
    els.cloudAccessStatus.textContent = error.message || "Не удалось создать remote session";
    els.cloudAccessState.className = "status failed";
    els.cloudAccessState.textContent = "FAILED";
    throw error;
  }
}

async function closeRemoteSession(sessionId) {
  if (!state.selectedDeviceId) return;
  if (!await confirmAction({
    variant: "warning",
    context: "REMOTE ACCESS / SESSION",
    title: "Закрыть удалённый доступ?",
    message: "Активный tunnel будет закрыт, а открытая через него LuCI перестанет отвечать.",
    values: [["Device", deviceDisplayName(currentDevice())], ["Session ID", sessionId]],
    confirmLabel: "Закрыть сессию",
  })) return;
  setStatus("Закрытие удаленного доступа");
  try {
    await api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/remote-sessions/${encodeURIComponent(sessionId)}/close`, {
      method: "POST",
    });
    await Promise.all([loadRemoteSessions(), loadAudit()]);
    notify("Удаленный доступ закрыт", "success");
  } catch (error) {
    els.cloudAccessStatus.textContent = error.message || "Не удалось закрыть remote session";
    els.cloudAccessState.className = "status failed";
    els.cloudAccessState.textContent = "FAILED";
    throw error;
  }
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

function presetLabel(preset) {
  return {
    lan_ip: "LAN-адрес",
    hostname: "Имя устройства",
    wifi_ssid: "Название Wi-Fi",
    wifi_key: "Пароль Wi-Fi",
    dhcp_lan: "DHCP-сервер",
  }[preset] || preset;
}

function presetDisplayValue(preset, value) {
  if (preset === "wifi_key") return "Новый пароль будет установлен";
  if (preset === "dhcp_lan") return value === "0" ? "Включен" : "Отключен";
  return value;
}

function presetSafeOutput(review, output) {
  if (review.preset !== "wifi_key") return output || "";
  return String(output || "").replaceAll(review.args.value, "********");
}

async function reviewPreset(preset) {
  const args = presetCommand(preset);
  if (!args) return;
  if (!args.value) {
    setStatus("Заполните значение настройки");
    return;
  }
  setStatus("Проверка изменения");
  const command = await createDeviceCommand("uci_preview", args, { skipConfirm: true });
  state.presetReview = { preset, args, previewCommandId: command.id, applyCommandIds: [] };
  renderPresetReview();
  els.presetReviewPanel.scrollIntoView({ behavior: "smooth", block: "center" });
  setStatus("Изменение отправлено на проверку");
}

function renderPresetReview() {
  const review = state.presetReview;
  if (!review) {
    els.presetReviewPanel.classList.add("is-hidden");
    return;
  }
  els.presetReviewPanel.classList.remove("is-hidden");
  els.presetReviewTitle.textContent = presetLabel(review.preset);
  els.presetReviewChange.innerHTML = `
    <span>${escapeHtml(`${review.args.config}.${review.args.section}.${review.args.option}`)}</span>
    <strong>${escapeHtml(presetDisplayValue(review.preset, review.args.value))}</strong>
  `;
  const preview = state.commands.find((command) => command.id === review.previewCommandId);
  const applying = review.applyCommandIds.map((id) => state.commands.find((command) => command.id === id)).filter(Boolean);
  const applyFailed = applying.some((command) => ["failed", "cancelled", "expired"].includes(command.status));
  const applyDone = applying.length === 2 && applying.every((command) => command.status === "completed");
  if (applyFailed) {
    els.presetReviewStatus.textContent = "Ошибка применения";
    els.presetReviewOutput.textContent = presetSafeOutput(review, applying.map((command) => command.output || `${command.type}: ${command.status}`).join("\n\n"));
    els.applyPresetReviewBtn.disabled = true;
    return;
  }
  if (applyDone) {
    els.presetReviewStatus.textContent = "Применено безопасно";
    els.presetReviewOutput.textContent = presetSafeOutput(review, applying.map((command) => command.output || `${command.type}: completed`).join("\n\n"));
    els.applyPresetReviewBtn.disabled = true;
    return;
  }
  if (applying.length) {
    els.presetReviewStatus.textContent = "Применяется";
    els.presetReviewOutput.textContent = "Настройка применяется. После commit confirmed роутер проверит связь с сервером.";
    els.applyPresetReviewBtn.disabled = true;
    return;
  }
  if (!preview || ["queued", "claimed"].includes(preview.status)) {
    els.presetReviewStatus.textContent = "Проверяется";
    els.presetReviewOutput.textContent = "Ожидание результата проверки от роутера...";
    els.applyPresetReviewBtn.disabled = true;
    return;
  }
  if (preview.status !== "completed") {
    els.presetReviewStatus.textContent = "Проверка не пройдена";
    els.presetReviewOutput.textContent = presetSafeOutput(review, preview.output || `Статус: ${preview.status}`);
    els.applyPresetReviewBtn.disabled = true;
    return;
  }
  els.presetReviewStatus.textContent = "Готово к применению";
  els.presetReviewOutput.textContent = presetSafeOutput(review, preview.output || "Изменение проверено");
  els.applyPresetReviewBtn.disabled = false;
}

async function applyPresetReview() {
  const review = state.presetReview;
  if (!review) return;
  const preview = state.commands.find((command) => command.id === review.previewCommandId);
  if (!preview || preview.status !== "completed") {
    setStatus("Сначала дождитесь успешной проверки");
    return;
  }
  if (!await confirmAction({
    variant: "warning",
    context: "CONFIGURATION / UCI",
    title: "Применить проверенное изменение?",
    message: "Изменение будет staged и применено через commit с проверкой reconnect.",
    description: "При потере связи роутер автоматически восстановит конфигурацию согласно существующей логике агента.",
    values: [["Configuration path", `${review.args.config}.${review.args.section}.${review.args.option}`], ["Current", "Not reported"], ["New", presetDisplayValue(review.preset, review.args.value), "success"]],
    confirmLabel: "Commit + check",
  })) return;
  const staged = await createDeviceCommand("uci_set", review.args, { skipConfirm: true, skipRefresh: true });
  const confirmed = await createDeviceCommand("uci_commit_confirmed", {
    config: review.args.config,
    confirm_seconds: "15",
  }, { skipConfirm: true, skipRefresh: true });
  review.applyCommandIds = [staged.id, confirmed.id];
  await Promise.all([loadCommands(), loadAudit(), loadAlerts()]);
  renderPresetReview();
  notify("Безопасное применение запущено", "success");
}

function cancelPresetReview() {
  state.presetReview = null;
  renderPresetReview();
  setStatus("Изменение отменено");
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
  notify("Alert acknowledged", "success");
}

async function clearDeviceAlerts() {
  if (!state.selectedDeviceId) return;
  const device = currentDevice();
  const confirmed = await confirmAction({
    variant: "warning",
    context: "MAINTENANCE / ALERTS",
    title: "Очистить алерты устройства?",
    message: "Текущая история алертов будет удалена из RMM. Настройки роутера не изменятся.",
    values: [["Device", deviceDisplayName(device)], ["Device ID", device && device.id]],
    confirmLabel: "Очистить алерты",
    loadingLabel: "Очистка…",
    input: { label: "Введите CLEAR для подтверждения", expected: "CLEAR", hint: "Это удалит историю алертов.", validationMessage: "Введите CLEAR без изменений." },
    onConfirm: () => api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/alerts`, { method: "DELETE" }),
    failureMessage: "Не удалось очистить алерты.",
  });
  if (!confirmed) return;
  setStatus("Алерты очищены");
  await Promise.all([loadAlerts(), loadAudit(), loadDevices()]);
  notify("Алерты очищены", "success");
}

async function clearDeviceCommands() {
  if (!state.selectedDeviceId) return;
  const device = currentDevice();
  const confirmed = await confirmAction({
    variant: "warning",
    context: "MAINTENANCE / COMMANDS",
    title: "Очистить command history?",
    message: "История выполненных команд и их output будет удалена из RMM.",
    values: [["Device", deviceDisplayName(device)], ["Known commands", state.commands.length]],
    confirmLabel: "Очистить историю команд",
    loadingLabel: "Очистка…",
    input: { label: "Введите CLEAR для подтверждения", expected: "CLEAR", hint: "Активные операции обрабатываются согласно серверной логике.", validationMessage: "Введите CLEAR без изменений." },
    onConfirm: () => api(`/api/devices/${encodeURIComponent(state.selectedDeviceId)}/commands`, { method: "DELETE" }),
    failureMessage: "Не удалось очистить историю команд.",
  });
  if (!confirmed) return;
  state.commands = [];
  state.commandOffset = 0;
  state.selectedCommand = null;
  await Promise.all([loadCommands(), loadAudit(), loadDevices()]);
  renderCommandDetail();
  notify("История команд очищена", "success");
}

async function clearDeviceAudit() {
  if (!state.selectedDeviceId) return;
  const device = currentDevice();
  const confirmed = await confirmAction({
    variant: "danger",
    context: "MAINTENANCE / AUDIT",
    title: "Очистить audit log устройства?",
    message: "Инженерный журнал событий выбранного устройства будет удалён.",
    description: "После операции останется только новая запись об очистке, если её создаёт сервер.",
    values: [["Device", deviceDisplayName(device)], ["Loaded events", state.auditEvents.length]],
    confirmLabel: "Очистить аудит",
    loadingLabel: "Очистка…",
    input: { label: "Введите CLEAR для подтверждения", expected: "CLEAR", validationMessage: "Введите CLEAR без изменений." },
    onConfirm: () => api(`/api/audit-events?device_id=${encodeURIComponent(state.selectedDeviceId)}`, { method: "DELETE" }),
    failureMessage: "Не удалось очистить audit log.",
  });
  if (!confirmed) return;
  state.auditEvents = [];
  state.auditOffset = 0;
  await loadAudit();
  notify("Аудит устройства очищен", "success");
}

async function deleteSelectedDevice() {
  const device = currentDevice();
  if (!device) return;
  const name = deviceDisplayName(device);
  const confirmed = await confirmAction({
    variant: "danger",
    context: "DANGER / DEVICE",
    title: `Удалить устройство «${name}»?`,
    message: "Устройство будет удалено из RMM вместе со связанными метриками, командами, алертами и remote sessions согласно существующей логике.",
    description: "Это действие нельзя отменить.",
    values: [["Device", name], ["Device ID", device.id], ["Last contact", formatDate(device.last_seen_at)]],
    confirmLabel: "Удалить устройство",
    loadingLabel: "Удаление…",
    input: { label: "Введите имя устройства для подтверждения", expected: name, hint: name, validationMessage: "Введите точное имя устройства, показанное выше." },
    onConfirm: () => api(`/api/devices/${encodeURIComponent(device.id)}`, { method: "DELETE" }),
    failureMessage: "Не удалось удалить устройство.",
  });
  if (!confirmed) return;
  state.selectedDeviceId = null;
  state.selectedCommand = null;
  state.commands = [];
  state.auditEvents = [];
  await loadDevices();
  render();
  notify("Устройство удалено", "success");
}

async function transferSelectedDevice() {
  const device = currentDevice();
  if (!device) return;
  const targetUsername = els.transferUsername.value.trim();
  const confirmed = await confirmAction({
    variant: "danger",
    context: "SECURITY / DEVICE TRANSFER",
    title: `Передать «${deviceDisplayName(device)}»?`,
    message: `Пользователь ${targetUsername} станет новым владельцем устройства. Оно исчезнет из текущего аккаунта.`,
    description: "Текущий удалённый доступ будет закрыт согласно существующей серверной логике.",
    values: [["Device", deviceDisplayName(device)], ["Device ID", device.id], ["New owner", targetUsername]],
    confirmLabel: "Передать устройство",
    loadingLabel: "Передача…",
    onConfirm: async () => {
      setFormMessage(els.transferMessage, "Передаём…");
      await api(`/api/devices/${encodeURIComponent(device.id)}/transfer`, {
        method: "POST",
        body: JSON.stringify({ target_username: targetUsername, current_password: els.transferPassword.value }),
      });
    },
    failureMessage: "Не удалось передать устройство. Проверьте пароль и нового владельца.",
  });
  if (!confirmed) return;
  els.deviceTransferForm.reset();
  state.selectedDeviceId = null;
  await loadDevices();
  render();
  notify(`Роутер передан пользователю ${targetUsername}`, "success");
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

function renderDiagnosticState(tone, title, description, withResultsAction = false) {
  const iconName = tone === "success" ? "check" : tone === "running" || tone === "info" ? "loader-2" : tone === "neutral" ? "terminal-2" : "alert-triangle";
  els.diagnosticStatus.className = `operation-status is-${tone}`;
  els.diagnosticStatus.dataset.tone = tone;
  els.diagnosticStatus.innerHTML = `
    <span class="operation-icon" aria-hidden="true">${icon(iconName)}</span>
    <div><strong>${escapeHtml(title)}</strong><small>${escapeHtml(description)}</small></div>
    ${withResultsAction ? '<button id="openDiagnosticResultsBtn" type="button">Открыть результаты</button>' : ""}
  `;
  els.diagnosticStatus.querySelector("#openDiagnosticResultsBtn")?.addEventListener("click", scrollToCommands);
}

function renderOperationalAvailability(device) {
  const offline = !device || !device.online;
  els.runFullDiagnosticBtn.disabled = offline;
  els.createRemoteSessionBtn.disabled = offline;
  for (const button of document.querySelectorAll(".diagnostic-btn")) button.disabled = offline;
  for (const button of document.querySelectorAll(".requires-online")) button.disabled = offline || button.classList.contains("is-loading");
  renderExpertOperationPanels();
  if (offline) {
    renderDiagnosticState("offline", "Диагностика недоступна", `Роутер не на связи · последний контакт ${formatDate(device && device.last_seen_at)}`);
  } else if (els.diagnosticStatus.dataset.tone === "offline") {
    renderDiagnosticState("neutral", "Готово к проверке", "Можно запустить отдельную проверку или полный diagnostic set.");
  }
}

async function sendDiagnostic(name) {
  const command = diagnosticCommand(name);
  if (!command) return;
  const device = currentDevice();
  if (!device || !device.online) {
    renderDiagnosticState("offline", "Диагностика недоступна", "Роутер должен восстановить соединение с RMM.");
    return;
  }
  setStatus("Запуск проверки");
  renderDiagnosticState("running", "Проверка запускается", `${commandTypeLabel(command.type)} добавляется в очередь агента.`);
  try {
    await createDeviceCommand(command.type, command.args);
    renderDiagnosticState("info", "Проверка поставлена в очередь", "Lifecycle и technical output доступны в истории операций.", true);
    notify("Проверка поставлена в очередь", "success");
    scrollToCommands();
  } catch (error) {
    renderDiagnosticState("error", "Не удалось запустить проверку", error.message || "Повторите действие.");
    throw error;
  }
}

async function runFullDiagnostic() {
  const checks = ["ping_server", "ping_internet", "show_routes", "show_interfaces"];
  const device = currentDevice();
  if (!device || !device.online) {
    renderDiagnosticState("offline", "Полная диагностика недоступна", "Роутер должен восстановить соединение с RMM.");
    return;
  }
  els.runFullDiagnosticBtn.disabled = true;
  renderDiagnosticState("running", "Полная диагностика запускается", `Подготовлено 0 из ${checks.length} проверок.`);
  let queued = 0;
  try {
    for (const name of checks) {
      const command = diagnosticCommand(name);
      await createDeviceCommand(command.type, command.args, { skipRefresh: true });
      queued += 1;
      renderDiagnosticState("running", "Полная диагностика запускается", `Подготовлено ${queued} из ${checks.length} проверок.`);
    }
    await Promise.all([loadCommands(), loadAudit(), loadAlerts()]);
    renderDiagnosticState("success", "Диагностика запущена", `${checks.length} проверок поставлены в очередь. Результаты появятся в Operations.`, true);
    notify("Полная диагностика поставлена в очередь", "success");
  } catch (error) {
    renderDiagnosticState(queued ? "warning" : "error", queued ? "Диагностика запущена частично" : "Диагностика не запущена", queued ? `${queued} из ${checks.length} проверок поставлены в очередь; следующая команда завершилась ошибкой.` : error.message || "Не удалось создать команды.", queued > 0);
    throw error;
  } finally {
    els.runFullDiagnosticBtn.disabled = !currentDevice() || !currentDevice().online;
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
  notify("Diagnostic queued", "success");
  scrollToCommands();
}

function scrollToCommands() {
  selectDeviceTab("operations");
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

async function createEnrollmentGrant() {
  const value = await confirmAction({
    variant: "neutral",
    context: "FLEET / ENROLLMENT",
    title: "Создать одноразовый grant",
    message: "Необязательное DNS-имя поможет узнавать роутер после подключения.",
    description: "Grant действует 15 минут и используется один раз.",
    confirmLabel: "Создать grant",
    input: {
      label: "DNS-имя роутера",
      required: false,
      placeholder: "office-1",
      hint: "Можно оставить пустым. Допустимы строчные буквы, цифры и дефис.",
      validate: (input) => !input || /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(input.trim().toLowerCase()) || "Введите корректное DNS-имя без точек.",
    },
  });
  if (value === false) return;
  const suggested = String(value).trim().toLowerCase();
  const data = await api("/api/enrollment-grants", {
    method: "POST",
    body: JSON.stringify({ dns_label: suggested, expires_seconds: 900 }),
  });
  els.enrollmentTokenOutput.value = data.enrollment_token;
  setFormMessage(els.enrollmentCopyState, "");
  els.enrollmentGrantDialog.showModal();
  notify("Grant для добавления роутера создан", "success");
}

async function createUserAccount() {
  const username = els.newUsername.value.trim();
  const password = els.newUserPassword.value;
  setFormMessage(els.createUserMessage, "Создаём пользователя…");
  setSubmitting(els.createUserForm, els.createUserSubmitBtn, true, "Создание…");
  try {
    await api("/api/users", {
      method: "POST",
      body: JSON.stringify({ username, email: els.newUserEmail.value.trim(), password, role: els.newUserRole.value }),
    });
    els.createUserDialog.close();
    els.createUserForm.reset();
    if (els.profileDialog.open) await loadUsers();
    notify(`Пользователь ${username} создан`, "success");
  } catch (error) {
    setFormMessage(els.createUserMessage, error && error.message ? error.message : "Не удалось создать пользователя", "error");
  } finally {
    setSubmitting(els.createUserForm, els.createUserSubmitBtn, false);
  }
}

async function checkHealth() {
  try {
    const response = await fetch("/healthz");
    els.apiState.textContent = response.ok ? "API: online" : "API: degraded";
  } catch {
    els.apiState.textContent = "API: offline";
  }
}

initFleetTableSorting();

els.loginForm.addEventListener("submit", (event) => {
  event.preventDefault();
  login().catch((error) => showLogin(error.message));
});
els.forgotPasswordBtn.addEventListener("click", () => {
  setFormMessage(els.forgotPasswordMessage, "");
  els.forgotPasswordDialog.showModal();
});
els.closeForgotPasswordBtn.addEventListener("click", () => els.forgotPasswordDialog.close());
els.forgotPasswordForm.addEventListener("submit", (event) => {
  event.preventDefault();
  requestPasswordReset().catch((error) => setFormMessage(els.forgotPasswordMessage, error.message, "error"));
});
els.closePasswordResetBtn.addEventListener("click", () => els.passwordResetDialog.close());
els.passwordResetForm.addEventListener("submit", (event) => {
  event.preventDefault();
  confirmPasswordReset().catch((error) => setFormMessage(els.passwordResetMessage, error.message, "error"));
});
els.confirmationForm.addEventListener("submit", (event) => {
  event.preventDefault();
  submitConfirmation();
});
els.confirmationCancelBtn.addEventListener("click", cancelConfirmation);
els.confirmationCloseBtn.addEventListener("click", cancelConfirmation);
els.confirmationDialog.addEventListener("cancel", (event) => {
  event.preventDefault();
  cancelConfirmation();
});
els.confirmationDialog.addEventListener("click", (event) => {
  if (event.target === els.confirmationDialog) cancelConfirmation();
});
els.confirmationDialog.addEventListener("keydown", (event) => {
  if (event.key !== "Tab") return;
  const focusable = [...els.confirmationDialog.querySelectorAll('button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), summary, [tabindex]:not([tabindex="-1"])')]
    .filter((element) => !element.closest(".is-hidden"));
  if (focusable.length === 0) return;
  const first = focusable[0];
  const last = focusable[focusable.length - 1];
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
});

for (const button of document.querySelectorAll("[data-dialog-cancel]")) {
  button.addEventListener("click", () => button.closest("dialog")?.close());
}
els.logoutBtn.addEventListener("click", () => logout().catch(reportError));
els.fleetNavBtn.addEventListener("click", showFleet);
els.problemsNavBtn.addEventListener("click", () => openDeviceArea("overview", "#alertList", "problems").catch(reportError));
els.operationsNavBtn.addEventListener("click", () => openDeviceArea("operations", "#remoteAccessPanel", "operations").catch(reportError));
els.profileBtn.addEventListener("click", showProfile);
els.notificationCenterBtn.addEventListener("click", () => showNotificationCenter().catch(reportError));
els.closeNotificationCenterBtn.addEventListener("click", () => els.notificationCenterDialog.close());
els.notificationCenterDialog.addEventListener("cancel", (event) => {
  event.preventDefault();
  els.notificationCenterDialog.close();
});
els.markAllNotificationsReadBtn.addEventListener("click", () => markAllNotificationsRead().catch(reportError));

for (const button of document.querySelectorAll(".mobile-nav-item")) {
  button.addEventListener("click", () => {
    const route = button.dataset.mobileRoute;
    if (route === "fleet") showFleet();
    if (route === "problems") openDeviceArea("overview", "#alertList", "problems").catch(reportError);
    if (route === "operations") openDeviceArea("operations", "#remoteAccessPanel", "operations").catch(reportError);
    if (route === "notifications") showNotificationCenter().catch(reportError);
    if (route === "profile") showProfile();
  });
}

els.closeProfileBtn.addEventListener("click", closeProfile);
for (const button of els.profileTabs) {
  button.addEventListener("click", () => selectProfileTab(button.dataset.profileTab));
  button.addEventListener("keydown", (event) => {
    if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
    event.preventDefault();
    const available = els.profileTabs.filter((item) => !item.classList.contains("is-hidden"));
    const current = available.indexOf(button);
    let next = event.key === "Home" ? 0 : event.key === "End" ? available.length - 1 : current + (event.key === "ArrowRight" ? 1 : -1);
    next = (next + available.length) % available.length;
    selectProfileTab(available[next].dataset.profileTab);
    available[next].focus();
  });
}
els.profileDialog.addEventListener("cancel", (event) => {
  event.preventDefault();
  closeProfile();
});
els.profileLogoutBtn.addEventListener("click", () => {
  closeProfile();
  logout().catch(reportError);
});
els.profileForm.addEventListener("submit", (event) => {
  event.preventDefault();
  saveProfile().catch((error) => setFormMessage(els.profileMessage, error.message, "error"));
});
els.passwordForm.addEventListener("submit", (event) => {
  event.preventDefault();
  changePassword().catch((error) => setFormMessage(els.passwordMessage, error.message, "error"));
});
els.notificationSettingsForm.addEventListener("submit", (event) => {
  event.preventDefault();
  saveNotificationSettings().catch((error) => setFormMessage(els.notificationSettingsMessage, notificationErrorMessage(error), "error"));
});
els.testNotificationsBtn.addEventListener("click", () => {
  testNotifications().catch((error) => setFormMessage(els.notificationSettingsMessage, notificationErrorMessage(error), "error"));
});
els.refreshNotificationsBtn.addEventListener("click", () => {
  loadNotificationHistory().catch((error) => setFormMessage(els.notificationSettingsMessage, notificationErrorMessage(error), "error"));
});
for (const filter of [
  els.notificationFilterDevice,
  els.notificationFilterSeverity,
  els.notificationFilterEvent,
  els.notificationFilterChannel,
  els.notificationFilterStatus,
]) {
  filter.addEventListener("change", () => {
    loadNotificationHistory().catch((error) => setFormMessage(els.notificationSettingsMessage, notificationErrorMessage(error), "error"));
  });
}
els.clearNotificationFiltersBtn.addEventListener("click", () => {
  els.notificationFilterDevice.value = "";
  els.notificationFilterSeverity.value = "";
  els.notificationFilterEvent.value = "";
  els.notificationFilterChannel.value = "";
  els.notificationFilterStatus.value = "";
  loadNotificationHistory().catch((error) => setFormMessage(els.notificationSettingsMessage, notificationErrorMessage(error), "error"));
});
els.verifyEmailBtn.addEventListener("click", () => requestContactVerification("email").catch(reportError));
els.verifyTelegramBtn.addEventListener("click", () => requestContactVerification("telegram").catch(reportError));
els.confirmEmailBtn.addEventListener("click", () => confirmContactVerification("email").catch(reportError));
els.confirmTelegramBtn.addEventListener("click", () => confirmContactVerification("telegram").catch(reportError));
els.notificationDeviceSelect.addEventListener("change", () => loadDeviceNotificationSettings().catch(reportError));
els.saveDeviceNotificationSettingsBtn.addEventListener("click", () => saveDeviceNotificationSettings().catch(reportError));
els.logoutAllBtn.addEventListener("click", () => logoutAll().catch(reportError));

els.closeLuciStateBtn.addEventListener("click", () => els.luciStateDialog.close());
els.luciStatePrimaryBtn.addEventListener("click", () => runLuCIPrimaryAction().catch(reportError));
els.luciStateDiagnosticBtn.addEventListener("click", () => {
  els.luciStateDialog.close();
  openDeviceArea("operations", "#diagnosticStatus", "operations").catch(reportError);
});
els.luciStateDialog.addEventListener("cancel", (event) => {
  event.preventDefault();
  els.luciStateDialog.close();
});

function closeSystemState() {
  state.systemStateRetry = null;
  if (els.systemStateDialog.open) els.systemStateDialog.close();
}

els.closeSystemStateBtn.addEventListener("click", closeSystemState);
els.systemStateBackBtn.addEventListener("click", closeSystemState);
els.systemStateDialog.addEventListener("cancel", (event) => {
  event.preventDefault();
  closeSystemState();
});
els.systemStatePrimaryBtn.addEventListener("click", () => {
  const retry = state.systemStateRetry;
  closeSystemState();
  if (retry) Promise.resolve(retry()).catch(reportError);
});
els.systemStateCopyBtn.addEventListener("click", async () => {
  const requestId = Array.from(els.systemStateContext.querySelectorAll("div")).find((row) => row.querySelector("dt")?.textContent === "ID запроса")?.querySelector("dd")?.textContent;
  if (!requestId) return;
  await navigator.clipboard.writeText(requestId);
  notify("ID запроса скопирован", "success");
});

els.fleetFilterToggle.addEventListener("click", () => {
  const isOpen = els.fleetAdvancedFilters.classList.toggle("is-open");
  els.fleetFilterToggle.setAttribute("aria-expanded", String(isOpen));
  els.fleetFilterToggle.textContent = isOpen ? "Скрыть" : "Фильтры";
});

els.refreshBtn.addEventListener("click", () => loadDevices().catch(reportError));
els.addRouterBtn.addEventListener("click", () => createEnrollmentGrant().catch(reportError));
function openCreateUserDialog() {
  els.createUserForm.reset();
  setFormMessage(els.createUserMessage, "");
  els.createUserDialog.showModal();
}
els.addUserBtn.addEventListener("click", openCreateUserDialog);
els.openCreateUserBtn.addEventListener("click", openCreateUserDialog);
els.createUserForm.addEventListener("submit", (event) => {
  event.preventDefault();
  createUserAccount().catch(reportError);
});
els.cancelCreateUserBtn.addEventListener("click", () => els.createUserDialog.close());
els.closeCreateUserBtn.addEventListener("click", () => els.createUserDialog.close());
els.copyEnrollmentTokenBtn.addEventListener("click", async () => {
  await navigator.clipboard.writeText(els.enrollmentTokenOutput.value);
  setFormMessage(els.enrollmentCopyState, "COPIED", "success");
  notify("Grant скопирован", "success");
});
els.backToFleetBtn.addEventListener("click", showFleet);
els.quickDiagnosticBtn.addEventListener("click", () => selectDeviceTab("operations"));
els.openLuciBtn.addEventListener("click", openLuciOrRemoteAccess);
els.updateAgentBtn.addEventListener("click", () => queueAgentUpdate().catch(reportError));
els.rollbackAgentBtn.addEventListener("click", openAgentRollbackDialog);
els.closeAgentRollbackBtn.addEventListener("click", () => els.agentRollbackDialog.close());
els.agentRollbackForm.addEventListener("submit", (event) => queueAgentRollback(event).catch(reportError));
els.runFullDiagnosticBtn.addEventListener("click", () => runFullDiagnostic().catch(reportError));
els.loadMoreCommandsBtn.addEventListener("click", () => loadCommands({ append: true }).catch(reportError));
els.loadMoreAuditBtn.addEventListener("click", () => loadAudit({ append: true }).catch(reportError));
els.alertStatusFilter.addEventListener("change", () => {
  state.alertStatusFilter = els.alertStatusFilter.value;
  loadAlerts().catch(reportError);
});
els.sendCommandBtn.addEventListener("click", () => sendCommand().catch(reportError));
els.sendBulkCommandBtn.addEventListener("click", () => sendBulkCommand().catch(reportError));
els.saveFleetBtn.addEventListener("click", () => saveFleetMetadata().catch(reportError));
els.clearAlertsBtn.addEventListener("click", () => clearDeviceAlerts().catch(reportError));
els.clearCommandsBtn.addEventListener("click", () => clearDeviceCommands().catch(reportError));
els.clearAuditBtn.addEventListener("click", () => clearDeviceAudit().catch(reportError));
els.deleteDeviceBtn.addEventListener("click", () => deleteSelectedDevice().catch(reportError));
els.deviceTransferForm.addEventListener("submit", (event) => {
  event.preventDefault();
  transferSelectedDevice().catch((error) => setFormMessage(els.transferMessage, error.message, "error"));
});
els.sendPackageCommandBtn.addEventListener("click", () => sendPackageCommand().catch(reportError));
els.createRemoteSessionBtn.addEventListener("click", () => createRemoteSession().catch(reportError));
els.openCloudAccessBtn.addEventListener("click", () => openCloudAccess().catch(reportError));
els.createBackupBtn.addEventListener("click", () => createBackup().catch(reportError));
els.refreshBackupsBtn.addEventListener("click", () => loadBackups().catch(reportError));
els.rotateDeviceCredentialBtn.addEventListener("click", () => rotateDeviceCredential().catch(reportError));
els.revokeDeviceCredentialBtn.addEventListener("click", () => revokeDeviceCredential().catch(reportError));
els.downloadDatabaseSnapshotBtn.addEventListener("click", () => downloadDatabaseSnapshot().catch(reportError));
els.uciBackupBtn.addEventListener("click", () => sendUciCommand("uci_backup").catch(reportError));
els.uciPreviewBtn.addEventListener("click", () => sendUciCommand("uci_preview").catch(reportError));
els.uciShowBtn.addEventListener("click", () => sendUciCommand("uci_show").catch(reportError));
els.uciSetBtn.addEventListener("click", () => sendUciCommand("uci_set").catch(reportError));
els.uciCommitBtn.addEventListener("click", () => sendUciCommand("uci_commit").catch(reportError));
els.uciCommitConfirmedBtn.addEventListener("click", () => sendUciCommand("uci_commit_confirmed").catch(reportError));
els.uciRevertBtn.addEventListener("click", () => sendUciCommand("uci_revert").catch(reportError));
els.uciRestoreBtn.addEventListener("click", () => sendUciCommand("uci_restore").catch(reportError));
els.copyCommandOutputBtn.addEventListener("click", async () => {
  if (!state.selectedCommand) return;
  const output = state.selectedCommand.output || JSON.stringify(state.selectedCommand.args || {}, null, 2);
  await navigator.clipboard.writeText(output);
  notify("Output copied", "success");
});

for (const button of document.querySelectorAll(".copy-info-btn")) {
  button.addEventListener("click", async () => {
    const source = document.querySelector(`#${button.dataset.copyInfo}`);
    if (!source) return;
    await navigator.clipboard.writeText(source.textContent);
    notify("Значение скопировано", "success");
  });
}

for (const button of document.querySelectorAll(".preset-review-btn")) {
  button.addEventListener("click", () => {
    reviewPreset(button.dataset.preset).catch(reportError);
  });
}
els.applyPresetReviewBtn.addEventListener("click", () => applyPresetReview().catch(reportError));
els.cancelPresetReviewBtn.addEventListener("click", cancelPresetReview);

for (const button of document.querySelectorAll(".diagnostic-btn")) {
  button.addEventListener("click", () => {
    sendDiagnostic(button.dataset.diagnostic).catch(reportError);
  });
}

for (const button of document.querySelectorAll(".device-tab")) {
  button.addEventListener("click", () => selectDeviceTab(button.dataset.deviceTabTarget));
  button.addEventListener("keydown", (event) => {
    if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
    event.preventDefault();
    const tabs = Array.from(document.querySelectorAll(".device-tab"));
    const current = tabs.indexOf(button);
    let next = event.key === "Home" ? 0 : event.key === "End" ? tabs.length - 1 : current + (event.key === "ArrowRight" ? 1 : -1);
    next = (next + tabs.length) % tabs.length;
    selectDeviceTab(tabs[next].dataset.deviceTabTarget);
    tabs[next].focus();
  });
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

for (const button of document.querySelectorAll(".table-sort")) {
  button.addEventListener("click", () => {
    const nextKey = button.dataset.sortKey;
    const defaultDir = button.dataset.sortDefaultDir || "asc";
    if (state.fleetSortKey === nextKey) {
      state.fleetSortDir = state.fleetSortDir === "asc" ? "desc" : "asc";
    } else {
      state.fleetSortKey = nextKey;
      state.fleetSortDir = defaultDir;
    }
    renderDevices();
  });
}

for (const button of document.querySelectorAll(".command-filter")) {
  button.addEventListener("click", () => {
    state.commandFilter = button.dataset.commandFilter;
    document.querySelectorAll(".command-filter").forEach((item) => item.classList.toggle("is-active", item === button));
    renderCommands(state.commands);
  });
}

els.commandStatusFilter.addEventListener("change", () => {
  state.commandStatusFilter = els.commandStatusFilter.value;
  renderCommands(state.commands);
});

els.commandType.addEventListener("change", () => {
  els.commandTarget.disabled = ["pkg_list_installed", "route_show", "interfaces_show", "reboot"].includes(els.commandType.value);
});

function syncPackageForm() {
  const operation = els.packageCommand.value;
  const needsName = operation === "pkg_install" || operation === "pkg_remove";
  els.packageName.disabled = !needsName;
  els.packageName.required = needsName;
  els.sendPackageCommandBtn.classList.toggle("danger", operation === "pkg_remove");
  els.sendPackageCommandBtn.textContent = operation === "pkg_remove" ? "Remove" : operation === "pkg_install" ? "Install" : "Queue";
  setFormMessage(els.packageFormMessage, "");
}

els.packageCommand.addEventListener("change", syncPackageForm);
syncPackageForm();

els.bulkCommandType.addEventListener("change", () => {
  els.bulkCommandTarget.disabled = els.bulkCommandType.value === "pkg_list_installed";
});

openPasswordResetFromURL();
checkHealth();
checkSession();
setInterval(() => {
  checkHealth();
  if (!els.appShell.classList.contains("is-hidden") && !state.liveConnected) {
    refreshDevicesIfIdle().catch(reportError);
  }
}, 30000);

setInterval(updateLiveStateLabel, 10000);

document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "visible" && !els.appShell.classList.contains("is-hidden")) {
    checkHealth();
    refreshDevicesIfIdle().catch(reportError);
    if (!eventSource || eventSource.readyState === EventSource.CLOSED) connectLiveUpdates();
  }
});
