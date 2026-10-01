/* Fleet controls reuse the application's API and native confirmation dialog. */
window.RMMFleet = {
  mount({ api, notify, confirm, chart, getDevices, onOpen, onClose }) {
    const root = document.querySelector("#fleetManagementView");
    const selected = new Set();
	const pendingCreates = new Map();
	async function create(path,payload) {
		const body=JSON.stringify(payload);let request=pendingCreates.get(path);
		if (!request || request.body!==body) {request={body,key:crypto.randomUUID()};pendingCreates.set(path,request);}
		const response=await api(path,{method:"POST",body:JSON.stringify({...payload,request_key:request.key})});
		pendingCreates.delete(path);return response;
	}
    const escape = (value) => String(value ?? "").replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[char]));
    const management = window.RMMManagement.mount({root,api,action,escape,confirm,getDevices,create});
    let tab = "operations";
    let timer = null;
    let busy = false;
    let operations = [];
    let backups = [];
    const states = { running: "RUNNING", queued: "QUEUED", pending: "PENDING", claimed: "RUNNING", paused: "PAUSED", completed: "COMPLETED", failed: "FAILED", cancelled: "CANCELLED", cancelling: "CANCELLING" };
    const status = (value) => `<span class="status ${value === "completed" ? "success" : value === "failed" ? "danger" : "warning"}">${escape(states[value] || value)}</span>`;
    function message(text) { root.querySelector("[data-fleet-message]").textContent = text; }
    async function action(work) {
      if (busy) return;
      busy = true;
      const buttons = [...root.querySelectorAll("button[type=submit], [data-fleet-tab]")];
      buttons.forEach((button) => { button.disabled = true; });
      message("Выполняется…");
      try { const result = await work(); message(typeof result === "string" ? result : "Готово"); }
      catch (error) { message(error.message); }
      finally { busy = false; buttons.forEach((button) => { button.disabled = false; }); }
    }
    const deviceOptions = () => getDevices().map((device) => `<option value="${escape(device.id)}">${escape(device.hostname)} · ${escape(device.id)}</option>`).join("");
    function show(nextTab = tab) {
      const navScroll=root.querySelector(".fleet-management-tabs")?.scrollLeft || 0;
      tab = nextTab;
      root.innerHTML = `<header class="fleet-management-header"><div><p class="eyebrow">FLEET / MANAGEMENT</p><h2 id="fleetManagementTitle">Управление парком</h2></div><button type="button" data-fleet-close>К объектам</button></header>
        <nav class="fleet-management-tabs" aria-label="Управление парком">${[["operations", "Массовые операции"], ["profiles", "Профили UCI"], ["schedules", "Расписания"], ["access", "Политики доступа"], ["diagnostics", "Диагностика"], ["backups", "Сравнение копий"], ["history", "История состояния"], ["rules", "Автоматические реакции"], ["assets", "Оборудование"], ["permissions", "Роли и права"], ["incidents", "Центр инцидентов"], ["waves", "Обновления волнами"], ["topology", "Карта сети"]].map(([key, label]) => `<button type="button" data-fleet-tab="${key}" ${key === tab ? 'aria-current="page"' : ""}>${label}</button>`).join("")}</nav>
        <p class="fleet-management-message" data-fleet-message role="status" aria-live="polite"></p><section class="panel" data-fleet-content></section>`;
      root.querySelector("[data-fleet-close]").addEventListener("click", () => { close(); onClose(); });
      root.querySelectorAll("[data-fleet-tab]").forEach((button) => button.addEventListener("click", () => show(button.dataset.fleetTab)));
      if (tab === "operations" || tab === "diagnostics") renderOperationForm();
      if (tab === "assets") renderAssets();
      if (tab === "backups") renderBackups();
      if (tab === "schedules") renderSchedules();
      if (tab === "access") renderAccess();
      if (tab === "profiles") renderProfiles();
      if (tab === "history") renderHistory();
      if (tab === "rules") renderRules();
      if (management[tab]) management[tab]();
      const nav=root.querySelector(".fleet-management-tabs"),active=nav.querySelector('[aria-current="page"]');nav.scrollLeft=navScroll;
      const bounds=nav.getBoundingClientRect(),selectedBounds=active.getBoundingClientRect();
      if (selectedBounds.left<bounds.left) nav.scrollLeft+=selectedBounds.left-bounds.left;
      else if(selectedBounds.right>bounds.right) nav.scrollLeft+=selectedBounds.right-bounds.right;

    }
    function renderOperationForm() {
      const diagnostic = tab === "diagnostics";
      root.querySelector("[data-fleet-content]").innerHTML = `<form class="fleet-management-form" data-fleet-operation-form>
        <h3>${diagnostic ? "Диагностический отчёт" : "Новая операция"}</h3><p>${diagnostic ? "DNS, ICMP, маршрут, интерфейсы, время и состояние сервисов. Требуется агент с поддержкой диагностического отчёта." : "Выберите устройства. Очередь сохраняется на сервере; лимит учитывает ожидающие и выполняющиеся команды."}</p>
        <div class="fleet-management-fields"><label>Название<input name="title" maxlength="160" value="${diagnostic ? "Диагностика сети" : "Операция парка"}" required></label>
          <label>Действие<select name="type">${(diagnostic ? [["diagnostic_report", "Полная диагностика"]] : [["system_backup_create", "Резервная копия"], ["ping", "Ping"], ["route_show", "Маршруты"], ["interfaces_show", "Интерфейсы"], ["pkg_list_installed", "Список пакетов"]]).map(([value, label]) => `<option value="${value}">${label}</option>`).join("")}</select></label>
          <label>Цель проверки<input name="target" value="1.1.1.1" maxlength="253"></label><label>Параллельность<input name="parallelism" type="number" min="1" max="20" value="3" required></label>
          <label>Группа<input data-fleet-group aria-label="Фильтр устройств по группе"></label><label>Тег<input data-fleet-tag aria-label="Фильтр устройств по тегу"></label></div>
        <label><input name="stop" type="checkbox" checked> Приостановить оставшуюся очередь при ошибке</label>
        <div><button type="button" data-fleet-select-visible>Выбрать показанные</button> <button type="button" data-fleet-clear-selection>Очистить выбор</button></div>
        <div class="fleet-management-device-list" data-fleet-devices></div><p data-fleet-selection></p><button class="primary" type="submit">${diagnostic ? "Начать диагностику" : "Запустить операцию"}</button></form>
        <h3>История операций</h3><div class="fleet-management-records" data-fleet-operations></div>`;
      const form = root.querySelector("[data-fleet-operation-form]");
      const visible = () => getDevices().filter((device) => (!form.querySelector("[data-fleet-group]").value || device.group === form.querySelector("[data-fleet-group]").value) && (!form.querySelector("[data-fleet-tag]").value || (device.tags || []).includes(form.querySelector("[data-fleet-tag]").value)));
      const refreshSelection = () => { root.querySelector("[data-fleet-selection]").textContent = `Выбрано ${selected.size} из ${getDevices().length} · максимум 100`; };
      const renderDevices = () => {
        root.querySelector("[data-fleet-devices]").innerHTML = visible().map((device) => `<label class="fleet-management-device-choice"><input type="checkbox" data-device-id="${escape(device.id)}" ${selected.has(device.id) ? "checked" : ""}><span>${escape(device.hostname)} · ${escape(device.group || "Без группы")} · ${device.online ? "На связи" : "Не на связи"}</span></label>`).join("") || "Нет подходящих устройств";
        root.querySelectorAll("[data-device-id]").forEach((input) => input.addEventListener("change", () => { if (input.checked) selected.add(input.dataset.deviceId); else selected.delete(input.dataset.deviceId); refreshSelection(); }));
        refreshSelection();
      };
      form.querySelectorAll("[data-fleet-group], [data-fleet-tag]").forEach((input) => input.addEventListener("input", renderDevices));
      form.querySelector("[data-fleet-select-visible]").addEventListener("click", () => { visible().forEach((device) => selected.add(device.id)); renderDevices(); });
      form.querySelector("[data-fleet-clear-selection]").addEventListener("click", () => { selected.clear(); renderDevices(); });
      form.addEventListener("submit", (event) => { event.preventDefault(); action(async () => {
        if (!selected.size || selected.size > 100) throw new Error("Выберите от 1 до 100 устройств");
        const values = new FormData(form);
        const type = String(values.get("type"));
        const args = ["ping", "diagnostic_report"].includes(type) ? { target: String(values.get("target")).trim() } : {};
        await create("/api/fleet/operations", { title: String(values.get("title")), device_ids: [...selected], type, args, parallelism: Number(values.get("parallelism")), stop_on_failure: values.has("stop") });
        notify("Операция добавлена в очередь", "success"); await loadOperations();
      }); });
      renderDevices(); action(loadOperations);
    }
    async function loadOperations() {
      const data = await api("/api/fleet/operations"); operations = data.operations || [];
      const list = root.querySelector("[data-fleet-operations]"); if (!list) return;
      const retained = new Map([...list.querySelectorAll("[data-operation-record]")].map((record) => [record.dataset.operationRecord, { open: record.querySelector("details").open, output: record.querySelector("[data-operation-output]").textContent, visible: !record.querySelector("[data-operation-output]").classList.contains("is-hidden") }]));
      list.innerHTML = operations.map((operation) => `<article class="fleet-management-record" data-operation-record="${escape(operation.id)}"><header><strong>${escape(operation.title || operation.type)}</strong>${status(operation.status)}</header><p>${escape(operation.created_at)} · Параллельность ${operation.parallelism}</p>
        ${["running", "paused", "cancelling"].includes(operation.status) ? `<button type="button" data-operation-action="cancel" data-operation-id="${escape(operation.id)}">Отменить оставшуюся очередь</button>` : ""}
        ${operation.status === "paused" ? `<button type="button" data-operation-action="resume" data-operation-id="${escape(operation.id)}">Продолжить очередь</button>` : ""}
        <details><summary>Устройства · ${operation.items.length}</summary>${operation.items.map((item) => `<div class="fleet-management-target"><span>${escape(getDevices().find((device) => device.id === item.device_id)?.hostname || item.device_id)}${item.error ? ` · ${escape(item.error)}` : ""}</span>${status(item.status)}${item.command_id ? `<button type="button" data-fleet-result="${escape(item.command_id)}" data-result-device="${escape(item.device_id)}">Результат</button>` : ""}${operation.type === "uci_profile_apply" && item.status === "completed" ? `<button type="button" data-profile-rollback="${escape(item.command_id)}" data-rollback-device="${escape(item.device_id)}">Откатить профиль</button>` : ""}</div>`).join("")}</details><pre class="fleet-management-output is-hidden" data-operation-output></pre></article>`).join("") || "Операций пока нет";
      list.querySelectorAll("[data-operation-record]").forEach((record) => {
        const saved = retained.get(record.dataset.operationRecord); if (!saved) return;
        record.querySelector("details").open = saved.open;
        const output = record.querySelector("[data-operation-output]"); output.textContent = saved.output; output.classList.toggle("is-hidden", !saved.visible);
      });
      list.querySelectorAll("[data-operation-action]").forEach((button) => button.addEventListener("click", () => action(async () => {
        const cancelling = button.dataset.operationAction === "cancel";
        if (!await confirm({ title: cancelling ? "Отменить оставшуюся очередь" : "Продолжить очередь", message: cancelling ? "Не начатые команды будут отменены. Выполняющиеся команды завершатся на роутерах." : "Оставшиеся устройства продолжат выполнение. Проверьте результаты первого этапа перед продолжением.", confirmLabel: cancelling ? "Отменить очередь" : "Продолжить", variant: "warning" })) return;
        await api(`/api/fleet/operations/${encodeURIComponent(button.dataset.operationId)}/${button.dataset.operationAction}`, { method: "POST" }); await loadOperations();
      })));
      list.querySelectorAll("[data-profile-rollback]").forEach((button) => button.addEventListener("click", () => action(async () => {
        if (!await confirm({ title: "Откатить профиль UCI?", message: "Агент восстановит сохранённую копию, если конфигурация не изменялась после применения профиля.", confirmLabel: "Откатить", variant: "warning" })) return;
        await create("/api/fleet/operations", { title: "Откат профиля", device_ids: [button.dataset.rollbackDevice], type: "uci_profile_rollback", args: { apply_id: button.dataset.profileRollback }, parallelism: 1, stop_on_failure: true }); await loadOperations();
      })));
      list.querySelectorAll("[data-fleet-result]").forEach((button) => button.addEventListener("click", () => action(async () => {
        const result = await api(`/api/devices/${encodeURIComponent(button.dataset.resultDevice)}/commands/${encodeURIComponent(button.dataset.fleetResult)}`);
        const output = button.closest("article").querySelector("[data-operation-output]"); output.textContent = result.result?.diagnostics ? JSON.stringify(result.result.diagnostics, null, 2) : result.output || "Результат ещё не получен"; output.classList.remove("is-hidden");
      })));
    }
    function renderAssets() {
      root.querySelector("[data-fleet-content]").innerHTML = `<h3>Учёт оборудования</h3><form class="fleet-management-form" data-fleet-asset-form><div class="fleet-management-fields"><label>Устройство<select name="device_id" required>${deviceOptions()}</select></label>${[["model", "Модель"], ["serial_number", "Серийный номер"], ["site", "Площадка"], ["responsible", "Ответственный"]].map(([name, label]) => `<label>${label}<input name="${name}" maxlength="255"></label>`).join("")}<label>Гарантия до<input name="warranty_until" type="date"></label><label>Заметки<textarea name="notes" maxlength="4096"></textarea></label></div><button type="submit">Сохранить карточку</button></form><label>Поиск<input data-fleet-asset-search type="search"></label><div class="fleet-management-records" data-fleet-assets></div>`;
      let assets = [];
      const form = root.querySelector("[data-fleet-asset-form]");
      const fill = () => { const asset = assets.find((item) => item.device_id === form.elements.device_id.value) || {}; ["model", "serial_number", "site", "responsible", "warranty_until", "notes"].forEach((key) => { form.elements[key].value = asset[key] || ""; }); };
      const render = () => {
        const query = root.querySelector("[data-fleet-asset-search]").value.toLowerCase();
        root.querySelector("[data-fleet-assets]").innerHTML = assets.filter((item) => Object.values(item).some((value) => String(value).toLowerCase().includes(query))).map((item) => `<article class="fleet-management-record"><strong>${escape(item.hostname)}</strong><p>Модель: ${escape(item.model || "Не указана")} · Серийный номер: ${escape(item.serial_number || "Не указан")}</p><p>Площадка: ${escape(item.site || "Не указана")} · Ответственный: ${escape(item.responsible || "Не указан")}</p><p>Гарантия: ${escape(item.warranty_until || "Не указана")}</p><p>${escape(item.notes)}</p></article>`).join("") || "Оборудование не найдено";
      };
      const load = async () => { assets = (await api("/api/fleet/assets")).assets || []; render(); fill(); };
      form.elements.device_id.addEventListener("change", fill);
      form.addEventListener("submit", (event) => { event.preventDefault(); action(async () => { const asset = Object.fromEntries(new FormData(form)); await api(`/api/fleet/assets/${encodeURIComponent(asset.device_id)}`, { method: "PUT", body: JSON.stringify(asset) }); await load(); }); });
      root.querySelector("[data-fleet-asset-search]").addEventListener("input", render); action(load);
    }
    function renderBackups() {
      root.querySelector("[data-fleet-content]").innerHTML = `<h3>Сравнение резервных копий</h3><p>Сравниваются пути, типы, размеры и содержимое файлов по хешам. Секретные значения не выводятся. Для проверки текущей конфигурации сначала снимите свежую копию.</p><form class="fleet-management-form" data-fleet-backup-form><div class="fleet-management-fields"><label>Устройство<select name="device_id">${deviceOptions()}</select></label><label>Исходная копия<select name="before" required></select></label><label>Копия для сравнения<select name="after" required></select></label></div><div><button type="submit">Сравнить</button> <button type="button" data-fleet-current-backup>Снять текущую копию</button> <button type="button" data-fleet-reload-backups>Обновить список</button></div></form><pre class="fleet-management-output" data-fleet-backup-diff>Выберите две готовые копии.</pre>`;
      const form = root.querySelector("[data-fleet-backup-form]");
      const load = async () => { backups = (await api(`/api/devices/${encodeURIComponent(form.elements.device_id.value)}/backups`)).backups || []; const options = backups.filter((backup) => backup.status === "ready").map((backup) => `<option value="${escape(backup.id)}">${escape(backup.created_at)} · ${escape(backup.id)}</option>`).join(""); form.elements.before.innerHTML = options; form.elements.after.innerHTML = options; if (form.elements.after.options.length > 1) form.elements.after.selectedIndex = 1; };
      form.elements.device_id.addEventListener("change", () => action(load));
      root.querySelector("[data-fleet-reload-backups]").addEventListener("click", () => action(load));
      root.querySelector("[data-fleet-current-backup]").addEventListener("click", () => action(async () => { await api(`/api/devices/${encodeURIComponent(form.elements.device_id.value)}/backups`, { method: "POST" }); return "Создание текущей копии поставлено в очередь. Обновите список после завершения."; }));
      form.addEventListener("submit", (event) => { event.preventDefault(); action(async () => { const result = await api(`/api/devices/${encodeURIComponent(form.elements.device_id.value)}/backups/${encodeURIComponent(form.elements.before.value)}/compare?against=${encodeURIComponent(form.elements.after.value)}`); root.querySelector("[data-fleet-backup-diff]").textContent = result.changes.length ? result.changes.map((item) => `${item.change.toUpperCase()}  ${item.path}  ${item.before_size ?? "—"} → ${item.after_size ?? "—"} bytes`).join("\n") : "Изменений нет"; }); });
      if (getDevices().length) action(load); else message("Сначала добавьте устройство");
    }
    function renderSchedules() {
      root.querySelector("[data-fleet-content]").innerHTML = `<h3>Расписания обслуживания</h3><p>Запуск выполняется только в заданном окне. Пропущенные окна не воспроизводятся. Версия обновления фиксируется из совместимого подписанного feed при сохранении; установленная версия пропускается. Очередь и журнал сохраняются после перезапуска сервера.</p><form class="fleet-management-form" data-schedule-form><div class="fleet-management-fields"><label>Название<input name="title" maxlength="160" required></label><label>Устройство<select name="device">${deviceOptions()}</select></label><label>Действие<select name="type"><option value="system_backup_create">Резервная копия</option><option value="diagnostic_report">Диагностика</option><option value="ping">Ping</option><option value="agent_update">Обновление агента из подписанного feed</option></select></label><label>Часовой пояс<input name="timezone" value="Europe/Moscow" required></label><label>Время начала<input type="time" name="time" value="03:00" required></label><label>Окно, минут<input type="number" name="window" min="1" max="1440" value="60" required></label><label>Цель проверки<input name="target" value="1.1.1.1" maxlength="253"></label></div><fieldset><legend>Дни недели</legend>${["Вс", "Пн", "Вт", "Ср", "Чт", "Пт", "Сб"].map((day, index) => `<label><input type="checkbox" name="days" value="${index}" checked> ${day}</label>`).join(" ")}</fieldset><label><input type="checkbox" name="enabled"> Включить автоматический запуск</label><button type="submit">Сохранить расписание</button></form><h3>Сохранённые расписания</h3><div class="fleet-management-records" data-schedules></div><h3>Журнал запусков</h3><div class="fleet-management-records" data-schedule-runs></div>`;
      const form = root.querySelector("[data-schedule-form]");
      let schedules = [];
      const load = async () => {
        const data = await api("/api/fleet/schedules"); if (tab !== "schedules") return;
        schedules = data.schedules || [];
        root.querySelector("[data-schedules]").innerHTML = schedules.map((item) => `<article class="fleet-management-record"><strong>${escape(item.title)}</strong><p>${item.enabled ? "Включено" : "Отключено"} · ${escape(item.timezone)} · Следующий запуск: ${escape(item.next_run_at)} · Окно: ${item.window_minutes} мин.</p><button type="button" data-schedule-toggle="${escape(item.id)}">${item.enabled ? "Отключить" : "Включить"}</button></article>`).join("") || "Расписаний пока нет";
        root.querySelector("[data-schedule-runs]").innerHTML = (data.runs || []).map((item) => `<article class="fleet-management-record"><header><span>${escape(item.scheduled_at)}</span>${status(item.status)}</header><p>${escape(item.error || (item.status === "missed" ? "Окно обслуживания пропущено" : item.operation_id))}</p></article>`).join("") || "Запусков пока нет";
        root.querySelectorAll("[data-schedule-toggle]").forEach((button) => button.addEventListener("click", () => action(async () => {
          const item = schedules.find((schedule) => schedule.id === button.dataset.scheduleToggle);
          if (!await confirm({ title: item.enabled ? "Отключить расписание?" : "Включить расписание?", message: item.enabled ? "Уже созданные операции продолжат выполнение. Их можно отменить в массовых операциях." : "Сервер будет автоматически выполнять действие в выбранные дни и время.", confirmLabel: item.enabled ? "Отключить" : "Включить" })) return;
          await api(`/api/fleet/schedules/${encodeURIComponent(item.id)}`, { method: "PUT", body: JSON.stringify({ ...item, enabled: !item.enabled }) }); await load();
        })));
      };
      form.addEventListener("submit", (event) => { event.preventDefault(); action(async () => {
        const values = new FormData(form); const [hour, minute] = String(values.get("time")).split(":").map(Number);
        const enabled = values.has("enabled");
        if (enabled && !await confirm({ title: "Включить автоматический запуск?", message: "Действие будет выполняться в выбранные дни в пределах окна обслуживания.", confirmLabel: "Создать и включить" })) return;
        const type = String(values.get("type"));
        await create("/api/fleet/schedules", { title: String(values.get("title")), timezone: String(values.get("timezone")), weekdays: values.getAll("days").map(Number), minute_of_day: hour * 60 + minute, window_minutes: Number(values.get("window")), enabled, operation: { device_ids: [String(values.get("device"))], type, args: type === "system_backup_create" ? {} : { target: String(values.get("target")) }, parallelism: 1, stop_on_failure: true } }); await load();
      }); }); action(load);
    }
    function renderAccess() {
      root.querySelector("[data-fleet-content]").innerHTML = `<h3>Политика удалённого доступа</h3><p>Политику изменяет администратор. Сохранение закрывает действующие сессии и отзывает LuCI-гранты. Для ограничения SSH/LuCI требуется агент с поддержкой отдельных режимов.</p><form class="fleet-management-form" data-access-form><div class="fleet-management-fields"><label>Устройство<select name="device">${deviceOptions()}</select></label><label>Максимальный TTL, секунд<input name="ttl" type="number" min="60" max="7200" required></label></div><label><input type="checkbox" name="ssh"> Разрешить SSH</label><label><input type="checkbox" name="luci"> Разрешить LuCI</label><label><input type="checkbox" name="restrict"> Ограничить список пользователей</label><div class="fleet-management-device-list" data-access-users></div><button type="submit">Сохранить политику</button></form>`;
      const form = root.querySelector("[data-access-form]");
      let users = [];
      const load = async () => {
        const policy = await api(`/api/fleet/access-policies/${encodeURIComponent(form.elements.device.value)}`);
        if (tab !== "access") return;
        form.elements.ttl.value = policy.max_ttl_seconds;
        form.elements.ssh.checked = policy.ssh_allowed; form.elements.luci.checked = policy.luci_allowed; form.elements.restrict.checked = policy.restrict_users;
        root.querySelector("[data-access-users]").innerHTML = users.map((user) => `<label><input type="checkbox" name="users" value="${escape(user.id)}" ${(policy.user_ids || []).includes(user.id) ? "checked" : ""}> ${escape(user.display_name || user.username)}</label>`).join("") || "Список доступен администратору";
      };
      form.elements.device.addEventListener("change", () => action(load));
      form.addEventListener("submit", (event) => { event.preventDefault(); action(async () => {
        if (!await confirm({ title: "Применить политику доступа?", message: "Действующие сессии устройства будут закрыты, а LuCI-гранты отозваны. Новые сессии будут проверяться по этой политике.", confirmLabel: "Применить" })) return;
        const values = new FormData(form);
        await api(`/api/fleet/access-policies/${encodeURIComponent(values.get("device"))}`, { method: "PUT", body: JSON.stringify({ ssh_allowed: values.has("ssh"), luci_allowed: values.has("luci"), max_ttl_seconds: Number(values.get("ttl")), restrict_users: values.has("restrict"), user_ids: values.getAll("users") }) }); await load();
      }); });
      action(async () => { try { users = (await api("/api/users")).users || []; } catch (error) { if (!/403|administrator|admin|forbidden/i.test(error.message)) throw error; form.querySelector("button[type=submit]").hidden = true; } if (getDevices().length) await load(); else return "Сначала добавьте устройство"; });
    }
    function renderProfiles() {
      root.querySelector("[data-fleet-content]").innerHTML = `<h3>Профили UCI</h3><p>Поддерживаются именованные секции и @type[index]. Preview читает текущие значения без staging. Пароли и ключи задаются через существующий процесс управления учётными данными.</p><form class="fleet-management-form" data-profile-form><div class="fleet-management-fields"><label>Название<input name="title" maxlength="160" required></label><label>Конфигурация<select name="config">${["system", "network", "wireless", "dhcp", "firewall"].map((name) => `<option>${name}</option>`).join("")}</select></label></div><label>Параметры: одна строка section.option=value<textarea name="options" rows="5" maxlength="12000" placeholder="core.hostname=branch-router" required></textarea></label><button type="submit">Сохранить профиль</button></form><h3>Проверка и применение</h3><form class="fleet-management-form" data-profile-operation><div class="fleet-management-fields"><label>Профиль<select name="profile" required></select></label><label>Устройства<select name="devices" multiple size="5" required>${deviceOptions()}</select></label><label>Успешный preview<select name="preview"></select></label><label>Параллельность после canary<input name="parallelism" type="number" min="1" max="20" value="3"></label></div><p>Применение сначала выполнится на одном устройстве и остановится для проверки. Остальную очередь можно продолжить в массовых операциях. Перед изменением сохраняется backup; потеря связи вызывает откат. Preview действует 30 минут.</p><button type="submit" value="preview">Проверить отклонения</button> <button type="submit" value="apply">Применить с canary</button></form><div class="fleet-management-records" data-profiles></div>`;
      const form = root.querySelector("[data-profile-form]"); const operationForm = root.querySelector("[data-profile-operation]");
      const load = async () => {
        const [data, history] = await Promise.all([api("/api/fleet/profiles"), api("/api/fleet/operations")]); if (tab !== "profiles") return;
        operationForm.elements.profile.innerHTML = (data.profiles || []).map((profile) => `<option value="${escape(profile.id)}">${escape(profile.title)}</option>`).join("");
        operationForm.elements.preview.innerHTML = `<option value="">Выберите preview для применения</option>` + (history.operations || []).filter((operation) => operation.type === "uci_profile_preview" && operation.status === "completed").map((operation) => `<option value="${escape(operation.id)}">${escape(operation.title)} · ${escape(operation.created_at)}</option>`).join("");
        root.querySelector("[data-profiles]").innerHTML = (data.profiles || []).map((profile) => `<article class="fleet-management-record"><strong>${escape(profile.title)}</strong><p>${escape(profile.definition.config)}</p><pre class="fleet-management-output">${escape(profile.definition.options.map((option) => `${option.section}.${option.option}=${option.value}`).join("\n"))}</pre></article>`).join("") || "Профилей пока нет";
      };
      form.addEventListener("submit", (event) => { event.preventDefault(); action(async () => {
        const values = new FormData(form);
        const options = String(values.get("options")).split(/\r?\n/).filter((line) => line.trim()).map((line) => { const match = /^(@[\w-]+\[\d{1,3}\]|[\w-]+)\.([\w-]+)=(.*)$/.exec(line); if (!match) throw new Error("Формат параметра: section.option=value"); return { section: match[1], option: match[2], value: match[3] }; });
        await api("/api/fleet/profiles", { method: "POST", body: JSON.stringify({ title: String(values.get("title")), definition: { config: String(values.get("config")), options } }) }); await load();
      }); });
      operationForm.addEventListener("submit", (event) => { event.preventDefault(); const requestedAction = event.submitter.value; action(async () => {
        const values = new FormData(operationForm);
        if (requestedAction === "apply" && !await confirm({ title: "Применить профиль с canary?", message: "Будут изменены выбранные настройки UCI. Сначала сервер запустит одно устройство. Проверьте результат перед продолжением остальных.", confirmLabel: "Начать canary" })) return;
        const operation = await create("/api/fleet/profile-operations", { profile_id: String(values.get("profile")), device_ids: values.getAll("devices"), action: requestedAction, preview_operation_id: String(values.get("preview")), parallelism: Number(values.get("parallelism")) });
        notify("Профиль добавлен в очередь", "success"); return `Операция ${operation.id}. Результаты и продолжение canary доступны в массовых операциях.`;
      }); }); action(load);
    }
    function renderHistory() {
      root.querySelector("[data-fleet-content]").innerHTML = `<h3>История состояния</h3><form class="fleet-management-form" data-history-form><div class="fleet-management-fields"><label>Устройство<select name="device">${deviceOptions()}</select></label></div><button type="submit">Обновить историю</button></form><p data-history-availability></p><p>Доступность оценена по heartbeat: интервалы без данных дольше 120 секунд считаются недоступными. Это оценка связи с агентом. CPU показан как load average за минуту.</p><div class="fleet-management-fields" data-history-charts></div><h3>Временная шкала</h3><div class="fleet-management-records" data-history-events></div>`;
      const form = root.querySelector("[data-history-form]");
      const load = async () => {
        const history = await api(`/api/fleet/history/${encodeURIComponent(form.elements.device.value)}`); if (tab !== "history") return;
        root.querySelector("[data-history-availability]").textContent = history.observed_availability_percent == null ? "Недостаточно данных для оценки доступности" : `Наблюдаемая доступность: ${history.observed_availability_percent.toFixed(1)}% · samples: ${history.points.length}`;
        const charts = root.querySelector("[data-history-charts]"); charts.replaceChildren();
        [["Нагрузка CPU", "LA1", "load", 1, 0], ["Память", "%", "memory_percent", 1, 100], ["WAN RX", "Mbps", "wan_rx_bps", 1000000, 0], ["WAN TX", "Mbps", "wan_tx_bps", 1000000, 0]].forEach(([label, unit, key, divisor, max]) => {
          const points = history.points.filter((point) => point[key] != null).map((point) => ({ time: point.at, value: point[key] / divisor }));
          if (points.length) charts.appendChild(chart(label, unit, points, "accent", max));
          else { const empty = document.createElement("p"); empty.className = "fleet-management-muted"; empty.textContent = `${label}: нет измерений`; charts.appendChild(empty); }
        });
        root.querySelector("[data-history-events]").innerHTML = history.events.slice().reverse().map((event) => `<article class="fleet-management-record"><header><time>${escape(event.at)}</time><span>${escape(event.type)}</span></header><p>${escape(event.message)}</p></article>`).join("") || "Событий пока нет";
      };
      form.addEventListener("submit", (event) => { event.preventDefault(); action(load); });
      form.elements.device.addEventListener("change", () => action(load));
      if (getDevices().length) action(load); else message("Сначала добавьте устройство");
    }
    function renderRules() {
      root.querySelector("[data-fleet-content]").innerHTML = `<h3>Автоматические реакции</h3><p>Счётчик учитывает новые наблюдения. Cooldown — минимум 5 минут, лимит — до 5 действий в сутки UTC. При восстановлении условия ожидающий перезапуск отменяется. Недоступность сервиса без подтверждённого состояния не запускает реакцию.</p><form class="fleet-management-form" data-rule-form><div class="fleet-management-fields"><label>Название<input name="title" maxlength="160" required></label><label>Устройство<select name="device">${deviceOptions()}</select></label><label>Условие<select name="kind"><option value="memory_high">Память выше порога</option><option value="load_high">CPU load выше порога</option><option value="wan_unreachable">Все проверки связи неуспешны</option><option value="offline">Нет heartbeat</option><option value="service_down">Сервис остановлен по диагностике</option></select></label><label>Порог, % RAM или LA1<input name="threshold" type="number" min="0" max="100" step="0.1" value="85"></label><label>Последовательные наблюдения<input name="consecutive" type="number" min="1" max="20" value="3"></label><label>Cooldown, секунд<input name="cooldown" type="number" min="300" max="86400" value="1800"></label><label>Действий в сутки<input name="maximum" type="number" min="1" max="5" value="2"></label><label>Действие<select name="action"><option value="notify">Уведомление в центре событий</option><option value="service_restart">Перезапуск сервиса</option></select></label><label>Сервис<select name="service"><option>dnsmasq</option><option>uhttpd</option></select></label></div><label><input type="checkbox" name="enabled"> Включить правило</label><button type="submit">Сохранить правило</button></form><h3>Правила</h3><div class="fleet-management-records" data-rules></div><h3>Журнал реакций</h3><div class="fleet-management-records" data-rule-events></div>`;
      const form = root.querySelector("[data-rule-form]"); let rules = [];
      const load = async () => {
        const data = await api("/api/fleet/rules"); if (tab !== "rules") return; rules = data.rules || [];
        root.querySelector("[data-rules]").innerHTML = rules.map((rule) => `<article class="fleet-management-record"><strong>${escape(rule.title)}</strong><p>${rule.enabled ? "Включено" : "Отключено"} · ${escape(rule.kind)} · ${escape(rule.action)} · Последовательных: ${rule.consecutive} · Cooldown: ${rule.cooldown_seconds} s · Лимит: ${rule.max_actions_per_day}/сутки</p><button type="button" data-rule-toggle="${escape(rule.id)}">${rule.enabled ? "Отключить" : "Включить"}</button></article>`).join("") || "Правил пока нет";
        root.querySelector("[data-rule-events]").innerHTML = (data.events || []).map((event) => `<article class="fleet-management-record"><header><time>${escape(event.created_at)}</time>${status(event.status)}</header><p>${escape(event.device_id)} · ${escape(event.action)} · ${escape(event.message)}</p></article>`).join("") || "Реакций пока нет";
        root.querySelectorAll("[data-rule-toggle]").forEach((button) => button.addEventListener("click", () => action(async () => {
          const rule = rules.find((item) => item.id === button.dataset.ruleToggle);
          if (!rule.enabled && !await confirm({ title: "Включить автоматическую реакцию?", message: rule.action === "service_restart" ? `Правило сможет перезапускать ${rule.service} в пределах cooldown и дневного лимита.` : "При подтверждённом условии правило создаст уведомление.", confirmLabel: "Включить" })) return;
          await api(`/api/fleet/rules/${encodeURIComponent(rule.id)}`, { method: "PUT", body: JSON.stringify({ ...rule, enabled: !rule.enabled }) }); await load();
        })));
      };
      form.addEventListener("submit", (event) => { event.preventDefault(); action(async () => {
        const values = new FormData(form); const enabled = values.has("enabled");
        if (enabled && !await confirm({ title: "Создать включённое правило?", message: values.get("action") === "service_restart" ? `Правило сможет автоматически перезапускать ${values.get("service")}.` : "Правило создаст уведомления при подтверждении условия.", confirmLabel: "Создать и включить" })) return;
        await create("/api/fleet/rules", { title: String(values.get("title")), device_ids: [String(values.get("device"))], kind: String(values.get("kind")), threshold: Number(values.get("threshold")), consecutive: Number(values.get("consecutive")), cooldown_seconds: Number(values.get("cooldown")), max_actions_per_day: Number(values.get("maximum")), action: String(values.get("action")), service: String(values.get("service")), enabled }); await load();
      }); }); action(load);
    }
    function close() { root.classList.add("is-hidden"); clearInterval(timer); timer = null; }
    function open() {
      onOpen(); root.classList.remove("is-hidden"); if (!busy) show(); clearInterval(timer);
      timer = setInterval(() => { if (!busy && !root.classList.contains("is-hidden") && !document.querySelector("#appShell")?.classList.contains("is-hidden") && document.visibilityState === "visible" && root.querySelector("[data-fleet-operations]") && !root.querySelector("[data-fleet-operations]").contains(document.activeElement)) loadOperations().catch((error) => message(error.message)); }, 10000);
    }
    document.querySelectorAll("[data-fleet-management-open]").forEach((button) => button.addEventListener("click", open));
    return { close };
  },
};
