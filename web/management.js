/* Extended management workflows share the fleet view and application dialogs. */
window.RMMManagement = {
  mount({ root, api, action, escape, confirm, getDevices, create }) {
    const content = () => root.querySelector("[data-fleet-content]");
    const devices = () => getDevices().map((d) => `<option value="${escape(d.id)}">${escape(d.hostname)} · ${escape(d.id)}</option>`).join("");
    const deviceName = (id) => getDevices().find((d) => d.id === id)?.hostname || id;
    const permissionNames = { view: "Просмотр", diagnostics: "Диагностика", updates: "Обновления", uci: "Конфигурация UCI", remote: "Удалённый доступ", backups: "Резервные копии", maintenance: "Обслуживание", incidents: "Работа с инцидентами" };

    function permissions() {
      content().innerHTML = `<h3>Роли и права</h3><p>Права ограничивают существующее владение устройствами. Группы и площадки задают дополнительную область; пустой список означает все принадлежащие пользователю устройства.</p><div data-permission-current></div><form class="fleet-management-form is-hidden" data-permission-form><div class="fleet-management-fields"><label>Пользователь<select name="user" required></select></label><label>Шаблон роли<select name="preset"><option value="viewer">Наблюдатель</option><option value="operator">Оператор</option><option value="maintainer">Инженер</option><option value="custom">Свои права</option></select></label></div><fieldset><legend>Разрешения</legend>${Object.entries(permissionNames).map(([key, label]) => `<label class="fleet-management-device-choice"><input name="permissions" type="checkbox" value="${key}"><span>${label}</span></label>`).join("")}</fieldset><div class="fleet-management-fields"><label>Группы: одна на строку<textarea name="groups" rows="3" maxlength="12000"></textarea></label><label>Площадки: одна на строку<textarea name="sites" rows="3" maxlength="12000"></textarea></label></div><button type="submit">Сохранить права</button><div data-permission-saved></div></form>`;
      action(async () => {
        const own = await api("/api/fleet/permissions");
        root.querySelector("[data-permission-current]").textContent = own.configured ? `Ваши разрешения: ${own.permissions.map((p) => permissionNames[p] || p).join(", ") || "Нет доступа к устройствам"}` : "Для вашей учётной записи действуют стандартные права роли.";
        let users;
        try { users = (await api("/api/users")).users || []; }
        catch (error) { if (error.status === 403 || error.status === 401) return "Изменение прав доступно администратору"; throw error; }
        const form = root.querySelector("[data-permission-form]");
        form.classList.remove("is-hidden");
        const targets = users.filter((u) => u.role !== "admin" && !u.disabled);
        form.elements.user.innerHTML = targets.map((u) => `<option value="${escape(u.id)}">${escape(u.display_name || u.username)}</option>`).join("");
        const presets = { viewer: ["view"], operator: ["view", "diagnostics", "backups", "incidents"], maintainer: Object.keys(permissionNames) };
        const fill = (policy) => {
          form.elements.preset.value = "custom";
          form.querySelectorAll('[name="permissions"]').forEach((c) => { c.checked = policy.permissions.includes(c.value); });
          form.elements.groups.value = (policy.groups || []).join("\n"); form.elements.sites.value = (policy.sites || []).join("\n");
        };
        const load = async () => { if (form.elements.user.value) fill(await api(`/api/fleet/permissions/${encodeURIComponent(form.elements.user.value)}`)); };
        form.elements.user.addEventListener("change", () => action(load));
        form.elements.preset.addEventListener("change", () => { const preset = presets[form.elements.preset.value]; if (preset) form.querySelectorAll('[name="permissions"]').forEach((c) => { c.checked = preset.includes(c.value); }); });
        form.addEventListener("submit", (event) => { event.preventDefault(); action(async () => {
          const user = targets.find((u) => u.id === form.elements.user.value); if (!user) throw new Error("Выберите пользователя");
          const split = (value) => value.split("\n").map((s) => s.trim()).filter(Boolean);
          const policy = { permissions: [...form.querySelectorAll('[name="permissions"]:checked')].map((c) => c.value), groups: split(form.elements.groups.value), sites: split(form.elements.sites.value) };
          if (!await confirm({ title: "Изменить права пользователя?", message: `Учётная запись: ${user.display_name || user.username}. Изменения действуют сразу; ожидающие команды проверяются перед выдачей. Связанные с пользователем сессии SSH/LuCI RMM будут закрыты. Администраторы сохраняют полный доступ.`, confirmLabel: "Сохранить права", variant: "warning" })) return;
          await api(`/api/fleet/permissions/${encodeURIComponent(user.id)}`, { method: "PUT", body: JSON.stringify(policy) });
          root.querySelector("[data-permission-saved]").textContent = `Права сохранены: ${user.username}`; await load();
        }); });
        if (!targets.length) { form.querySelector("button[type=submit]").disabled = true; return "Нет обычных пользователей. Создайте пользователя в разделе администрирования."; }
        await load();
      });
    }

    function incidents() {
      content().innerHTML = `<h3>Центр инцидентов</h3><p>Подтверждённые условия объединяются по устройству и категории. Неизвестное состояние не считается восстановлением. Можно назначить ответственного, подтвердить проблему и добавить комментарий.</p><div class="fleet-management-fields"><label>Статус<select data-incident-filter><option value="open">Открытые</option><option value="resolved">Закрытые</option><option value="all">Все</option></select></label><label>Поиск<input type="search" data-incident-search></label></div><button type="button" data-incident-refresh>Обновить инциденты</button><div class="fleet-management-records" data-incidents></div>`;
      let entries = []; let users = [];
      const render = () => {
        const state = root.querySelector("[data-incident-filter]").value, query = root.querySelector("[data-incident-search]").value.toLowerCase();
        root.querySelector("[data-incidents]").innerHTML = entries.filter((i) => (state === "all" || (state === "open" ? i.status !== "resolved" : i.status === "resolved")) && `${i.title} ${deviceName(i.device_id)} ${i.category}`.toLowerCase().includes(query)).map((i) => `<article class="fleet-management-record" data-incident="${escape(i.id)}"><header><strong>${escape(i.title)}</strong><span class="status ${i.status === "resolved" ? "success" : "warning"}">${escape({ open: "OPEN", acknowledged: "ACKNOWLEDGED", resolved: "RESOLVED" }[i.status])}</span></header><p>${escape(deviceName(i.device_id))} · ${escape(i.category)} · Повторений: ${i.occurrences}</p><p>Открыт: ${escape(i.opened_at)} · Ответственный: ${escape(users.find((u) => u.id === i.assignee_id)?.display_name || i.assignee_id || "Не назначен")}</p>${i.status !== "resolved" ? `<button type="button" data-incident-action="acknowledge">Подтвердить</button> <button type="button" data-incident-action="resolve">Закрыть вручную</button>` : ""}<button type="button" data-incident-diagnostic>Диагностика</button> <button type="button" data-incident-history>История состояния</button><form class="fleet-management-form" data-incident-assign><label>Ответственный<select name="assignee"><option value="">Не назначен</option>${users.map((u) => `<option value="${escape(u.id)}" ${i.assignee_id === u.id ? "selected" : ""}>${escape(u.display_name || u.username)}</option>`).join("")}</select></label><button type="submit">Назначить</button></form><form class="fleet-management-form" data-incident-comment><label>Комментарий<textarea name="body" maxlength="2048" rows="2" required></textarea></label><button type="submit">Добавить комментарий</button></form><details><summary>История инцидента · ${i.events.length}</summary>${i.events.map((e) => `<p><span class="fleet-management-muted">${escape(e.created_at)} · ${escape(e.actor)} · ${escape(e.action)}</span><br>${escape(e.body)}</p>`).join("")}</details><pre class="fleet-management-output is-hidden" data-incident-output></pre></article>`).join("") || "Инцидентов по выбранному фильтру нет";
        root.querySelectorAll("[data-incident]").forEach((record) => {
          const i = entries.find((item) => item.id === record.dataset.incident);
          const update = async (kind, body = "") => { await api(`/api/fleet/incidents/${encodeURIComponent(i.id)}`, { method: "POST", body: JSON.stringify({ action: kind, body }) }); await load(); };
          record.querySelectorAll("[data-incident-action]").forEach((b) => b.addEventListener("click", () => action(async () => {
            if (b.dataset.incidentAction === "resolve" && !await confirm({ title: "Закрыть инцидент вручную?", message: `${i.title} · ${deviceName(i.device_id)}. Закрытие не изменяет роутер. После восстановления и нового нарушения инцидент откроется снова.`, confirmLabel: "Закрыть инцидент", variant: "warning" })) return;
            await update(b.dataset.incidentAction);
          })));
          record.querySelector("[data-incident-comment]").addEventListener("submit", (e) => { e.preventDefault(); action(() => update("comment", e.target.elements.body.value)); });
          record.querySelector("[data-incident-assign]").addEventListener("submit", (e) => { e.preventDefault(); action(() => update("assign", e.target.elements.assignee.value)); });
          record.querySelector("[data-incident-diagnostic]").addEventListener("click", () => action(async () => { await create("/api/fleet/operations", { title: `Инцидент: ${i.title}`.slice(0, 70), device_ids: [i.device_id], type: "diagnostic_report", args: { target: "1.1.1.1" }, parallelism: 1, stop_on_failure: true }); return "Диагностика добавлена в массовые операции"; }));
          record.querySelector("[data-incident-history]").addEventListener("click", () => action(async () => { const history = await api(`/api/fleet/history/${encodeURIComponent(i.device_id)}`); const output = record.querySelector("[data-incident-output]"); output.textContent = JSON.stringify(history.events || [], null, 2); output.classList.remove("is-hidden"); }));
        });
      };
      const load = async () => { entries = (await api("/api/fleet/incidents")).incidents || []; render(); };
      root.querySelector("[data-incident-refresh]").addEventListener("click", () => action(load));
      root.querySelector("[data-incident-filter]").addEventListener("change", render); root.querySelector("[data-incident-search]").addEventListener("input", render);
      action(async () => {
        const me = await api("/api/auth/me"); users = me.user ? [me.user] : [];
        if (me.user?.role === "admin") users = ((await api("/api/users")).users || []).filter((u) => !u.disabled);
        await load();
      });
    }

    function waves() {
      content().innerHTML = `<h3>Обновления волнами</h3><p>Создание и управление доступны администратору. Сначала выбранные тестовые устройства, затем заданные волны. Каждая волна должна вернуться с целевой версией и оставаться на связи весь период наблюдения. Ошибка или потеря связи приостанавливает распространение.</p><form class="fleet-management-form" data-wave-form><div class="fleet-management-fields"><label>Канал<select name="channel"><option value="stable">Stable</option><option value="candidate">Candidate</option></select></label><label>Размеры волн через запятую<input name="sizes" value="3,10,25" required></label><label>Наблюдение после волны, секунд<input name="observation" type="number" min="60" max="3600" value="120" required></label><label>Все целевые устройства<select name="devices" multiple size="6" required>${devices()}</select></label><label>Тестовые устройства: от 1 до 10<select name="canaries" multiple size="6" required>${devices()}</select></label></div><p>Тестовые устройства должны входить в общий выбор. Последний размер волны повторяется до завершения. Требуются совместимые агенты с проверкой подписанного манифеста.</p><button type="submit">Создать обновление волнами</button></form><button type="button" data-wave-refresh>Обновить состояние</button><div class="fleet-management-records" data-waves></div>`;
      const load = async () => {
        let data;
        try { data = await api("/api/agent-rollouts"); }
        catch (error) { if (error.status === 403) { form.classList.add("is-hidden"); root.querySelector("[data-waves]").textContent = "Управление обновлениями доступно администратору"; return; } throw error; }
        const candidate = form.querySelector('[name="channel"] option[value="candidate"]');
        candidate.disabled = !data.candidate_available; candidate.textContent = data.candidate_available ? "Candidate" : "Candidate — недоступен";
        if (candidate.disabled && form.elements.channel.value === "candidate") form.elements.channel.value = "stable";

        root.querySelector("[data-waves]").innerHTML = (data.rollouts || []).filter((r) => r.guard).map((r) => `<article class="fleet-management-record"><header><strong>${escape(r.target_version)} · ${escape(r.channel)}</strong><span class="status ${r.status === "completed" ? "success" : "warning"}">${escape(r.status.toUpperCase())}</span></header><p>Волны: ${escape(r.guard.wave_sizes.join(", "))} · Наблюдение: ${r.guard.observation_seconds} с</p><p>${escape(r.guard.pause_reason || (r.guard.healthy_since ? `Наблюдение с ${r.guard.healthy_since}` : "Ожидание результатов текущей волны"))}</p>${r.devices.map((d) => `<p>${escape(deviceName(d.device_id))} · Волна ${d.batch} · ${escape(d.status)}${d.last_error ? ` · ${escape(d.last_error)}` : ""}</p>`).join("")}${!["completed", "cancelled"].includes(r.status) ? `<button type="button" data-wave-action="${r.status === "paused" ? "resume" : "pause"}" data-wave-id="${escape(r.id)}">${r.status === "paused" ? "Продолжить" : "Приостановить"}</button> <button type="button" data-wave-action="cancel" data-wave-id="${escape(r.id)}">Отменить остаток</button>` : ""}</article>`).join("") || "Обновлений волнами пока нет";
        root.querySelectorAll("[data-wave-action]").forEach((b) => b.addEventListener("click", () => action(async () => {
          if (!await confirm({ title: "Изменить состояние обновления?", message: `Действие: ${b.dataset.waveAction}. Уже выданные команды завершатся на устройствах. Продолжение проходит повторную проверку состояния волны.`, confirmLabel: "Подтвердить", variant: "warning" })) return;
          await api(`/api/agent-rollouts/${encodeURIComponent(b.dataset.waveId)}/${b.dataset.waveAction}`, { method: "POST" }); await load();
        })));
      };
      const form = root.querySelector("[data-wave-form]");
      form.addEventListener("submit", (e) => { e.preventDefault(); action(async () => {
        const data = new FormData(form), targets = data.getAll("devices"), canaries = data.getAll("canaries");
        const sizes = String(data.get("sizes")).split(",").map((s) => Number(s.trim()));
        if (!targets.length || canaries.length < 1 || canaries.length > 10 || canaries.some((id) => !targets.includes(id)) || sizes.some((n) => !Number.isInteger(n) || n < 1 || n > 500)) throw new Error("Проверьте цели, тестовые устройства и размеры волн");
        if (!await confirm({ title: "Начать обновление волнами?", message: `Устройств: ${targets.length}. Сначала обновятся тестовые устройства: ${canaries.map(deviceName).join(", ")}. Обновление перезапускает агента; дальнейшие волны требуют подтверждённой связи.`, confirmLabel: "Начать обновление", variant: "warning" })) return;
        await create("/api/agent-rollouts", { device_ids: targets, channel: String(data.get("channel")), batch_size: sizes[0], failure_threshold: 1, guard: { canary_ids: canaries, wave_sizes: sizes, observation_seconds: Number(data.get("observation")) } }); await load();
      }); });
      root.querySelector("[data-wave-refresh]").addEventListener("click", () => action(load)); action(load);
    }

    function topology() {
      content().innerHTML = `<h3>Карта сети</h3><p>Наблюдаемые соседи из ARP/NDP и интерфейсы. Совпадение IP позволяет предположить связь с управляемым роутером; это не подтверждение физического кабеля. Повторяющиеся адреса остаются неоднозначными.</p><label>Роутер<select data-topology-device>${devices()}</select></label><button type="button" data-topology-refresh>Обновить карту</button><div data-topology-map></div><div class="fleet-management-records" data-topology-links></div>`;
      let data = { nodes: [], links: [] };
      const render = () => {
        const id = root.querySelector("[data-topology-device]").value, source = data.nodes.find((n) => n.id === id), links = data.links.filter((l) => l.from === id);
        const shown = links.slice(0, 20), height = Math.max(120, shown.length * 60 + 30);
        root.querySelector("[data-topology-map]").innerHTML = source ? `<p>${source.stale ? "STALE · Данные устарели" : "Наблюдение актуально"} · ${escape(source.observed_at || "Нет heartbeat")}${data.truncated ? " · Набор ограничен для отображения" : ""}</p><svg class="fleet-topology-graph" viewBox="0 0 720 ${height}" role="img" aria-label="Наблюдаемые соседи выбранного роутера"><rect x="10" y="${height / 2 - 20}" width="230" height="40" rx="4"></rect><text x="22" y="${height / 2 + 5}">${escape(source.label.slice(0, 28))}</text>${shown.map((l, index) => { const target = data.nodes.find((n) => n.id === l.to), y = index * 60 + 30; return `<path d="M240 ${height / 2} L470 ${y}" class="${l.stale ? "is-stale" : ""}"></path><rect x="470" y="${y - 20}" width="240" height="40" rx="4"></rect><text x="480" y="${y + 5}">${escape((target?.label || l.address).slice(0, 28))}</text>`; }).join("")}</svg>` : "Нет данных об устройстве";
        root.querySelector("[data-topology-links]").innerHTML = `${shown.length < links.length ? "На схеме показаны первые 20 соседей. Полный список ниже." : ""}${links.map((l) => `<article class="fleet-management-record"><header><strong>${escape(l.address)}</strong><span class="status ${l.stale || ["FAILED", "INCOMPLETE"].includes(l.state) ? "warning" : "neutral"}">${escape(l.stale ? "STALE" : l.state)}</span></header><p>Интерфейс: ${escape(l.interface)} · MAC: ${escape(l.mac || "Не сообщён")}</p><p>${l.ambiguous ? "Неоднозначный адрес: несколько управляемых роутеров" : l.to.startsWith("neighbor:") ? "Внешний или неопознанный сосед" : `Предполагаемый управляемый сосед: ${escape(deviceName(l.to))}`}</p></article>`).join("") || "Соседи не сообщены"}<details><summary>Интерфейсы</summary><pre class="fleet-management-output">${escape(JSON.stringify(source?.interfaces || [], null, 2))}</pre></details>`;
      };
      const load = async () => { data = await api("/api/fleet/topology"); render(); };
      root.querySelector("[data-topology-device]").addEventListener("change", render); root.querySelector("[data-topology-refresh]").addEventListener("click", () => action(load)); action(load);
    }
    return { permissions, incidents, waves, topology };
  },
};
