const storageKeys = {
  serverUrl: "deployctl.serverUrl",
  adminKey: "deployctl.adminKey",
};

const els = {
  connectForm: document.getElementById("connect-form"),
  serverUrl: document.getElementById("server-url"),
  adminKey: document.getElementById("admin-key"),
  connectStatus: document.getElementById("connect-status"),
  clearSession: document.getElementById("clear-session"),
  dashboard: document.getElementById("dashboard"),
  shareClaim: document.getElementById("share-claim"),
  tokenForm: document.getElementById("token-form"),
  shareForm: document.getElementById("share-form"),
  tokenOutput: document.getElementById("token-output"),
  shareOutput: document.getElementById("share-output"),
  refreshData: document.getElementById("refresh-data"),
  tokensList: document.getElementById("tokens-list"),
  sharesList: document.getElementById("shares-list"),
  shareResolveOutput: document.getElementById("share-resolve-output"),
  agentPrompt: document.getElementById("agent-prompt"),
  claimShare: document.getElementById("claim-share"),
  claimOutput: document.getElementById("claim-output"),
};

function setStatus(text, isError = false) {
  els.connectStatus.textContent = text;
  els.connectStatus.style.color = isError ? "#9f2400" : "";
}

function loadSession() {
  const serverUrl = localStorage.getItem(storageKeys.serverUrl) || window.location.origin;
  const adminKey = localStorage.getItem(storageKeys.adminKey) || "";
  els.serverUrl.value = serverUrl;
  els.adminKey.value = adminKey;
  return { serverUrl, adminKey };
}

function saveSession(serverUrl, adminKey) {
  localStorage.setItem(storageKeys.serverUrl, serverUrl);
  localStorage.setItem(storageKeys.adminKey, adminKey);
}

function clearSession() {
  localStorage.setItem(storageKeys.serverUrl, window.location.origin);
  localStorage.removeItem(storageKeys.adminKey);
  els.serverUrl.value = window.location.origin;
  els.adminKey.value = "";
  els.dashboard.classList.add("hidden");
  setStatus("已清空本地凭据。");
}

async function apiRequest(path, { method = "GET", body, adminKey, serverUrl } = {}) {
  const url = new URL(path, serverUrl);
  const response = await fetch(url, {
    method,
    headers: {
      "Content-Type": "application/json",
      ...(adminKey ? { "X-Admin-Secret": adminKey } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });

  const json = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(json.message || `request failed: ${response.status}`);
  }
  return json;
}

function formatJSON(value) {
  return JSON.stringify(value, null, 2);
}

function renderRecordList(container, items, render) {
  if (!items.length) {
    container.className = "list-card empty-state";
    container.textContent = "暂无数据。";
    return;
  }
  container.className = "list-card";
  container.innerHTML = items.map(render).join("");
}

function agentLinkForShare(serverUrl, share) {
  const url = new URL(window.location.href);
  url.search = "";
  url.hash = "";
  url.searchParams.set("share", share.share_id);
  url.searchParams.set("code", share.share_code);
  return url.toString();
}

function buildAgentPrompt(serverUrl, resolve) {
  return `你现在负责接管 deployctl 服务。

已知信息：
- deployd 服务地址: ${serverUrl}
- 分享名称: ${resolve.share_name}
- token 名称: ${resolve.token_name}
- scope: ${resolve.scope}${resolve.project_scope ? ` (${resolve.project_scope})` : ""}
- 领取接口: ${resolve.claim_url}

请按以下顺序执行：
1. 先确认分享链接仍然有效
2. 调用 claim 接口领取 token
3. 保存返回的 token
4. 运行 deployctl --json doctor
5. 运行 deployctl --json auth whoami
6. 返回 doctor_json、whoami_json、server_url、token_scope

claim 示例：
curl -X POST "${new URL("/v1/share-links/claim", serverUrl)}" \\
  -H "Content-Type: application/json" \\
  -d '{"share_id":"${resolve.share_id}","code":"${new URLSearchParams(window.location.search).get("code") || ""}"}'
`;
}

async function revokeResource(kind, id) {
  const path = kind === "token" ? `/v1/admin/tokens/${id}/revoke` : `/v1/admin/share-links/${id}/revoke`;
  await apiRequest(path, {
    method: "POST",
    serverUrl: els.serverUrl.value.trim(),
    adminKey: els.adminKey.value.trim(),
  });
  await refreshDashboard();
}

async function refreshDashboard() {
  const serverUrl = els.serverUrl.value.trim();
  const adminKey = els.adminKey.value.trim();
  if (!serverUrl || !adminKey) return;

  const bootstrap = await apiRequest("/v1/admin/bootstrap", {
    serverUrl,
    adminKey,
  });

  renderRecordList(els.tokensList, bootstrap.tokens, (item) => `
    <div class="record">
      <div class="record-title">${item.token_name}</div>
      <div class="record-meta">${item.scope}${item.project_scope ? ` · ${item.project_scope}` : ""}</div>
      <div class="record-meta">id: ${item.token_id}</div>
      ${item.revoked_at ? `<div class="record-meta">已吊销</div>` : `<button class="button button-secondary revoke-button" data-kind="token" data-id="${item.token_id}">revoke</button>`}
    </div>
  `);

  renderRecordList(els.sharesList, bootstrap.share_links, (item) => `
    <div class="record">
      <div class="record-title">${item.share_name}</div>
      <div class="record-meta">${item.scope}${item.project_scope ? ` · ${item.project_scope}` : ""}</div>
      <div class="record-meta">claims: ${item.claim_count}/${item.max_claims}</div>
      <div class="record-meta">token: ${item.token_name}</div>
      ${item.revoked_at ? `<div class="record-meta">已吊销</div>` : `<button class="button button-secondary revoke-button" data-kind="share" data-id="${item.share_id}">revoke</button>`}
    </div>
  `);

  els.dashboard.classList.remove("hidden");
  setStatus(`已连接 ${bootstrap.server_url}`);
}

els.connectForm?.addEventListener("submit", async (event) => {
  event.preventDefault();
  const serverUrl = els.serverUrl.value.trim().replace(/\/$/, "");
  const adminKey = els.adminKey.value.trim();
  try {
    saveSession(serverUrl, adminKey);
    await refreshDashboard();
  } catch (error) {
    setStatus(error.message, true);
  }
});

els.clearSession?.addEventListener("click", clearSession);
els.refreshData?.addEventListener("click", async () => {
  try {
    await refreshDashboard();
  } catch (error) {
    setStatus(error.message, true);
  }
});

document.addEventListener("click", async (event) => {
  const target = event.target;
  if (!(target instanceof HTMLElement) || !target.classList.contains("revoke-button")) {
    return;
  }
  try {
    await revokeResource(target.dataset.kind, target.dataset.id);
  } catch (error) {
    setStatus(error.message, true);
  }
});

els.tokenForm?.addEventListener("submit", async (event) => {
  event.preventDefault();
  const formData = new FormData(event.currentTarget);
  const scopeValue = formData.get("scope");
  const rawProjectScope = String(formData.get("project_scope") || "").trim();
  const projectScope = scopeValue === "project:demo" ? rawProjectScope || "demo" : rawProjectScope;
  const scope = scopeValue === "project:demo" ? `project:${projectScope}` : String(scopeValue);
  try {
    const payload = {
      name: String(formData.get("name") || "").trim(),
      scope,
      project_scope: projectScope,
      expires_in: String(formData.get("expires_in") || "").trim(),
    };
    const result = await apiRequest("/v1/admin/tokens", {
      method: "POST",
      serverUrl: els.serverUrl.value.trim(),
      adminKey: els.adminKey.value.trim(),
      body: payload,
    });
    els.tokenOutput.className = "code-card";
    els.tokenOutput.textContent = formatJSON(result);
    event.currentTarget.reset();
    await refreshDashboard();
  } catch (error) {
    els.tokenOutput.className = "code-card";
    els.tokenOutput.textContent = error.message;
  }
});

els.shareForm?.addEventListener("submit", async (event) => {
  event.preventDefault();
  const formData = new FormData(event.currentTarget);
  const scopeValue = formData.get("scope");
  const rawProjectScope = String(formData.get("project_scope") || "").trim();
  const projectScope = scopeValue === "project:demo" ? rawProjectScope || "demo" : rawProjectScope;
  const scope = scopeValue === "project:demo" ? `project:${projectScope}` : String(scopeValue);

  try {
    const payload = {
      share_name: String(formData.get("share_name") || "").trim(),
      token_name: String(formData.get("token_name") || "").trim(),
      scope,
      project_scope: projectScope,
      share_expires_in: String(formData.get("share_expires_in") || "").trim(),
      token_expires_in: String(formData.get("token_expires_in") || "").trim(),
      max_claims: Number(formData.get("max_claims") || 1),
    };
    const result = await apiRequest("/v1/admin/share-links", {
      method: "POST",
      serverUrl: els.serverUrl.value.trim(),
      adminKey: els.adminKey.value.trim(),
      body: payload,
    });
    const handoff = {
      ...result,
      agent_link: agentLinkForShare(els.serverUrl.value.trim(), result),
    };
    els.shareOutput.className = "code-card";
    els.shareOutput.textContent = formatJSON(handoff);
    event.currentTarget.reset();
    await refreshDashboard();
  } catch (error) {
    els.shareOutput.className = "code-card";
    els.shareOutput.textContent = error.message;
  }
});

async function bootShareClaimView() {
  const params = new URLSearchParams(window.location.search);
  const shareId = params.get("share");
  const code = params.get("code");
  const serverUrl = params.get("server") || window.location.origin;

  if (!shareId || !code) {
    return false;
  }

  els.shareClaim.classList.remove("hidden");
  els.dashboard.classList.add("hidden");
  els.connectForm.closest(".section").classList.add("hidden");

  try {
    const resolveUrl = new URL("/v1/share-links/resolve", serverUrl);
    resolveUrl.searchParams.set("share_id", shareId);
    resolveUrl.searchParams.set("code", code);
    const response = await fetch(resolveUrl);
    const result = await response.json();
    if (!response.ok) {
      throw new Error(result.message || "resolve failed");
    }
    els.shareResolveOutput.className = "code-card";
    els.shareResolveOutput.textContent = formatJSON(result);
    els.agentPrompt.value = buildAgentPrompt(serverUrl, result);

    els.claimShare.onclick = async () => {
      try {
        const claimResult = await apiRequest("/v1/share-links/claim", {
          method: "POST",
          serverUrl,
          body: { share_id: shareId, code },
        });
        els.claimOutput.className = "code-card";
        els.claimOutput.textContent = formatJSON(claimResult);
      } catch (error) {
        els.claimOutput.className = "code-card";
        els.claimOutput.textContent = error.message;
      }
    };
  } catch (error) {
    els.shareResolveOutput.className = "code-card";
    els.shareResolveOutput.textContent = error.message;
  }

  return true;
}

async function init() {
  const inShareMode = await bootShareClaimView();
  if (inShareMode) return;

  const session = loadSession();
  els.serverUrl.value = session.serverUrl || window.location.origin;
  if (session.serverUrl && session.adminKey) {
    try {
      await refreshDashboard();
    } catch (error) {
      setStatus(error.message, true);
    }
  }
}

init();
