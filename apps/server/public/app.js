const agentForm = document.querySelector("#agent-form");
const capabilityForm = document.querySelector("#capability-form");
const refreshButton = document.querySelector("#refresh");
const expiresAt = capabilityForm.elements.expiresAt;

expiresAt.value = new Date(Date.now() + 60 * 60 * 1000).toISOString().slice(0, 16);

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

refreshButton.addEventListener("click", refresh);

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
    return;
  }
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
