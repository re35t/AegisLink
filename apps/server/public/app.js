const agentForm = document.querySelector("#agent-form");
const capabilityForm = document.querySelector("#capability-form");
const chatForm = document.querySelector("#chat-form");
const chatLog = document.querySelector("#chat-log");
const chatSources = document.querySelector("#chat-sources");
const refreshButton = document.querySelector("#refresh");
const expiresAt = capabilityForm.elements.expiresAt;
const routes = [...document.querySelectorAll("[data-route]")];
const navLinks = [...document.querySelectorAll("[data-link]")];

expiresAt.value = new Date(Date.now() + 60 * 60 * 1000).toISOString().slice(0, 16);

navLinks.forEach((link) => {
  link.addEventListener("click", (event) => {
    event.preventDefault();
    navigate(link.getAttribute("href") || "/");
  });
});

agentForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = new FormData(agentForm);
  await postJson("/api/agents", {
    id: valueOrUndefined(form.get("id")),
    ownerUserId: form.get("ownerUserId"),
    displayName: form.get("displayName"),
    publicKeyPem: form.get("publicKeyPem"),
    metadata: { source: "web-console" },
  });
  agentForm.reset();
  await refresh();
});

capabilityForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = new FormData(capabilityForm);
  await postJson("/api/capabilities", {
    subjectAgentId: form.get("subjectAgentId"),
    audienceAgentId: valueOrUndefined(form.get("audienceAgentId")),
    actions: csv(form.get("actions")),
    resources: csv(form.get("resources")),
    scope: form.get("scope"),
    expiresAt: new Date(form.get("expiresAt")).toISOString(),
  });
  await refresh();
});

chatForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = new FormData(chatForm);
  const question = String(form.get("question") || "").trim();
  if (!question) {
    return;
  }

  appendMessage("user", question);
  chatForm.elements.question.value = "";
  chatForm.querySelector("button").disabled = true;

  try {
    const result = await postJson("/api/chat", {
      question,
      agentId: form.get("agentId"),
      conversationId: valueOrUndefined(form.get("conversationId")),
      useMemory: form.get("useMemory") === "on",
    });
    appendMessage("assistant", result.answer);
    renderChatSources(result.sources || []);
  } finally {
    chatForm.querySelector("button").disabled = false;
    chatForm.elements.question.focus();
  }
});

refreshButton.addEventListener("click", refresh);

function navigate(pathname) {
  if (window.location.pathname !== pathname) {
    window.history.pushState({}, "", pathname);
  }
  renderRoute();
}

function renderRoute() {
  const pathname = window.location.pathname === "/chat" ? "/chat" : "/";
  document.body.dataset.page = pathname.slice(1) || "console";
  for (const route of routes) {
    route.hidden = route.dataset.route !== pathname;
  }
  for (const link of navLinks) {
    link.classList.toggle("active", link.getAttribute("href") === pathname);
  }
  refreshButton.hidden = pathname === "/chat";
}

window.addEventListener("popstate", renderRoute);

async function refresh() {
  const snapshot = await fetchJson("/api/snapshot");
  render("agents", snapshot.agents);
  render("capabilities", snapshot.capabilities);
  render("messages", snapshot.messages);
  render("audit", snapshot.auditLogs);
}

async function postJson(url, body) {
  const response = await fetch(url, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    const error = await response.json();
    alert(error.error || response.statusText);
    throw new Error(error.error || response.statusText);
  }
  return response.json();
}

async function fetchJson(url) {
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(response.statusText);
  }
  return response.json();
}

function render(id, value) {
  document.querySelector(`#${id}`).textContent = JSON.stringify(value, null, 2);
}

function appendMessage(role, content) {
  const message = document.createElement("div");
  message.className = `chat-message ${role}`;
  const label = document.createElement("strong");
  label.textContent = role === "user" ? "You" : "Agent";
  const text = document.createElement("p");
  text.textContent = content;
  message.append(label, text);
  chatLog.append(message);
  chatLog.scrollTop = chatLog.scrollHeight;
}

function renderChatSources(sources) {
  chatSources.textContent = JSON.stringify(sources, null, 2);
}

function csv(value) {
  return String(value)
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function valueOrUndefined(value) {
  const text = String(value ?? "").trim();
  return text.length === 0 ? undefined : text;
}

refresh().catch((error) => {
  console.error(error);
});
renderRoute();
