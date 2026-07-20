"use strict";

const concepts = {
  evolution: {
    number: "1",
    title: "Эволюция текущего интерфейса",
    description: "Сохраняет тёмный стиль и знакомую структуру, но переносит навигацию вниз, сокращает шапку и делает состояния понятнее.",
    risk: "Минимальный риск миграции",
    audience: "Для дома и небольших парков",
  },
  friendly: {
    number: "2",
    title: "Friendly mobile",
    description: "Светлый, спокойный интерфейс с крупными зонами нажатия и человеческими формулировками. Ближе к приложениям Keenetic и домашним облачным сервисам.",
    risk: "Средний объём изменений",
    audience: "Для частных пользователей и малого бизнеса",
  },
  console: {
    number: "3",
    title: "Professional console",
    description: "Более плотная операторская консоль: высокая информационная ёмкость, быстрые статусы и акцент на управление большим количеством устройств.",
    risk: "Полная переработка оболочки",
    audience: "Для MSP и больших парков",
  },
};

const errorStates = {
  timeout: {
    code: "504 · GATEWAY TIMEOUT",
    symbol: "!",
    title: "LuCI не отвечает",
    description: "Роутер на связи, но веб-интерфейс не ответил за 15 секунд. Настройки роутера не изменялись.",
    session: "Ещё 14:32",
    primary: "Повторить подключение",
    secondary: "Открыть диагностику",
  },
  expired: {
    code: "401 · SESSION EXPIRED",
    symbol: "⌛",
    title: "Доступ к LuCI истёк",
    description: "Временная сессия завершилась. Создайте новый безопасный доступ, когда он снова понадобится.",
    session: "Завершена",
    primary: "Открыть новый доступ",
    secondary: "Вернуться к роутеру",
  },
  offline: {
    code: "503 · ROUTER OFFLINE",
    symbol: "×",
    title: "Роутер не на связи",
    description: "RMM-агент давно не выходил на связь. LuCI станет доступен после восстановления подключения роутера.",
    session: "Не создана",
    primary: "Открыть диагностику",
    secondary: "Посмотреть события",
  },
  forbidden: {
    code: "403 · ACCESS DENIED",
    symbol: "⊘",
    title: "Нет доступа к роутеру",
    description: "Этот роутер принадлежит другому аккаунту или ваши права были изменены. Обратитесь к администратору.",
    session: "Недоступна",
    primary: "Вернуться к объектам",
    secondary: "Связаться с администратором",
  },
  starting: {
    code: "409 · TUNNEL STARTING",
    symbol: "…",
    title: "Туннель запускается",
    description: "Агент получил команду и поднимает защищённое соединение. Обычно это занимает до 20 секунд.",
    session: "Подключение…",
    primary: "Проверить снова",
    secondary: "Отменить запуск",
  },
};

function setConcept(name) {
  const concept = concepts[name] || concepts.evolution;
  document.body.dataset.concept = name in concepts ? name : "evolution";
  document.querySelector("#conceptNumber").textContent = concept.number;
  document.querySelector("#conceptTitle").textContent = concept.title;
  document.querySelector("#conceptDescription").textContent = concept.description;
  document.querySelector("#conceptRisk").textContent = concept.risk;
  document.querySelector("#conceptAudience").textContent = concept.audience;
  for (const button of document.querySelectorAll(".concept-button")) {
    button.classList.toggle("is-active", button.dataset.concept === document.body.dataset.concept);
  }
}

function setErrorState(name) {
  const state = errorStates[name] || errorStates.timeout;
  document.querySelector("#errorCode").textContent = state.code;
  document.querySelector("#errorSymbol").textContent = state.symbol;
  document.querySelector("#errorTitle").textContent = state.title;
  document.querySelector("#errorDescription").textContent = state.description;
  document.querySelector("#errorSession").textContent = state.session;
  document.querySelector("#errorPrimaryAction").textContent = state.primary;
  document.querySelector("#errorSecondaryAction").textContent = state.secondary;
  for (const button of document.querySelectorAll(".error-demo")) {
    button.classList.toggle("is-active", button.dataset.error === name);
  }
}

for (const button of document.querySelectorAll(".concept-button")) {
  button.addEventListener("click", () => setConcept(button.dataset.concept));
}

for (const button of document.querySelectorAll(".error-demo")) {
  button.addEventListener("click", () => setErrorState(button.dataset.error));
}

const initialConcept = new URLSearchParams(window.location.search).get("concept") || "evolution";
setConcept(initialConcept);
setErrorState("timeout");
