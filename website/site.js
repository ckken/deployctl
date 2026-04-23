const storageKeys = {
  serverUrl: "deployctl.serverUrl",
  adminKey: "deployctl.adminKey",
};

const state = {
  tokens: [],
  shares: [],
  selectedTokenId: "",
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
  selectedTokenCard: document.getElementById("selected-token-card"),
  clearSelectedToken: document.getElementById("clear-selected-token"),
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
  state.tokens = [];
  state.shares = [];
  state.selectedTokenId = "";
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

function describeScope(item) {
  return item.project_scope ? `${item.scope} · ${item.project_scope}` : item.scope;
}

function activeTokens() {
  return state.tokens.filter((item) => !item.revoked_at);
}

function activeShares() {
  return state.shares.filter((item) => !item.revoked_at);
}

function selectedToken() {
  return activeTokens().find((item) => item.token_id === state.selectedTokenId) || null;
}

function formatExpiry(expiresAt) {
  if (!expiresAt) return "不过期";
  const date = new Date(expiresAt);
  if (Number.isNaN(date.getTime())) return expiresAt;
  return date.toLocaleString("zh-CN", { hour12: false });
}

function scopeTone(scope) {
  if (scope === "admin") return "scope-admin";
  if (scope.startsWith("project:")) return "scope-project";
  return "scope-read-only";
}

function scopeLabel(item) {
  if (item.scope === "admin") return "管理员";
  if (item.scope.startsWith("project:")) return item.project_scope ? `项目 ${item.project_scope}` : "项目访问";
  return "只读";
}

function agentLinkForShare(serverUrl, share) {
  return share.share_url || new URL(`/s/${share.share_code}`, serverUrl).toString();
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
curl "${new URL(`/v1/share-links/claim?code=${extractShareCode()}`, serverUrl)}"
`;
}

function extractShareCode() {
  const params = new URLSearchParams(window.location.search);
  const fromQuery = params.get("code");
  if (fromQuery) return fromQuery;
  const match = window.location.pathname.match(/^\/s\/([^/]+)$/);
  return match ? decodeURIComponent(match[1]) : "";
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

function syncTokenProjectScopeField() {
  const field = els.tokenForm?.querySelector("[data-project-scope-field]");
  const scope = els.tokenForm?.elements?.scope?.value;
  if (!(field instanceof HTMLElement)) return;
  field.classList.toggle("hidden", scope !== "project:demo");
}

function populateShareDefaults(token) {
  if (!(els.shareForm instanceof HTMLFormElement)) return;
  els.shareForm.elements.share_name.value = `${token.token_name}-share`;
  els.shareForm.elements.share_expires_in.value = "24h";
  els.shareForm.elements.max_claims.value = "1";
}

function renderSelectedToken() {
  const token = selectedToken();
  if (!token) {
    els.selectedTokenCard.className = "selected-token empty-state";
    els.selectedTokenCard.textContent = "先在左侧选择一个 token。选中后，这里会自动带出默认分享配置。";
    els.shareForm.classList.add("hidden");
    return;
  }

  els.selectedTokenCard.className = "selected-token";
  els.selectedTokenCard.innerHTML = `
    <div class="selected-token-name">${token.token_name}</div>
    <div class="selected-token-meta">权限：${describeScope(token)}</div>
    <div class="selected-token-meta">到期：${formatExpiry(token.expires_at)}</div>
    <div class="selected-token-meta">接下来只需要改分享名称、有效期和 claim 次数。</div>
  `;
  els.shareForm.classList.remove("hidden");
}

function renderTokens() {
  const tokens = [...activeTokens()].reverse();
  renderRecordList(els.tokensList, tokens, (item) => `
    <div class="record ${state.selectedTokenId === item.token_id ? "is-selected" : ""}">
      <div class="record-main">
        <div class="record-icon">↗</div>
        <div class="record-text">
          <div class="record-top">
            <div>
              <div class="record-title">${item.token_name}</div>
              <div class="record-pills">
                <span class="scope-pill ${scopeTone(item.scope)}">${scopeLabel(item)}</span>
              </div>
            </div>
          </div>
          <div class="record-meta">范围：${describeScope(item)}</div>
          <div class="record-meta">到期：${formatExpiry(item.expires_at)}</div>
          <div class="record-meta">id: ${item.token_id}</div>
          <div class="record-actions">
            <button class="button button-secondary select-token-button ${state.selectedTokenId === item.token_id ? "is-selected" : ""}" data-id="${item.token_id}">
              ${state.selectedTokenId === item.token_id ? "已选中" : "用它生成分享"}
            </button>
            <button class="button button-secondary revoke-button" data-kind="token" data-id="${item.token_id}">撤销 Token</button>
          </div>
        </div>
      </div>
    </div>
  `);
}

function renderShares() {
  const shares = [...activeShares()].reverse();
  renderRecordList(els.sharesList, shares, (item) => `
    <div class="record">
      <div class="record-main">
        <div class="record-icon">⛓</div>
        <div class="record-text">
          <div class="record-title">${item.share_name}</div>
          <div class="record-meta">来源 token：${item.token_name}</div>
          <div class="record-meta">范围：${describeScope(item)}</div>
          <div class="record-meta">剩余次数：${Math.max(item.max_claims - item.claim_count, 0)} / ${item.max_claims}</div>
          <div class="record-meta">到期：${formatExpiry(item.expires_at)}</div>
          <div class="record-meta">链接只在创建成功时展示一次，删除后不会保留。</div>
          <div class="record-actions">
            <button class="button button-secondary revoke-button" data-kind="share" data-id="${item.share_id}">删除分享</button>
          </div>
        </div>
      </div>
    </div>
  `);
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

async function copyText(value) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value);
    return;
  }
  throw new Error("当前浏览器不支持复制，请手动复制");
}

function renderShareOutput(serverUrl, result) {
  const handoff = {
    share_name: result.share_name,
    agent_link: agentLinkForShare(serverUrl, result),
    claim_url: result.claim_url,
    scope: result.scope,
    project_scope: result.project_scope || "",
    max_claims: result.max_claims,
    expires_at: result.expires_at || null,
  };
  els.shareOutput.className = "code-card";
  els.shareOutput.textContent = formatJSON(handoff);
}

async function refreshDashboard() {
  const serverUrl = els.serverUrl.value.trim();
  const adminKey = els.adminKey.value.trim();
  if (!serverUrl || !adminKey) return;

  const bootstrap = await apiRequest("/v1/admin/bootstrap", {
    serverUrl,
    adminKey,
  });

  state.tokens = bootstrap.tokens;
  state.shares = bootstrap.share_links;

  if (!selectedToken()) {
    const firstToken = [...activeTokens()].reverse()[0];
    state.selectedTokenId = firstToken ? firstToken.token_id : "";
    if (firstToken) {
      populateShareDefaults(firstToken);
    }
  }

  renderSelectedToken();
  renderTokens();
  renderShares();
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

els.clearSelectedToken?.addEventListener("click", () => {
  state.selectedTokenId = "";
  renderSelectedToken();
  renderTokens();
});

els.tokenForm?.elements?.scope?.addEventListener("change", syncTokenProjectScopeField);

document.addEventListener("click", async (event) => {
  const target = event.target;
  if (!(target instanceof HTMLElement)) {
    return;
  }

  if (target.classList.contains("select-token-button")) {
    const token = activeTokens().find((item) => item.token_id === target.dataset.id);
    if (!token) {
      setStatus("token 不存在或已失效", true);
      return;
    }
    state.selectedTokenId = token.token_id;
    populateShareDefaults(token);
    renderSelectedToken();
    renderTokens();
    els.shareOutput.className = "code-card empty-state";
    els.shareOutput.textContent = "生成后会在这里给出可直接发给 agent 的链接。";
    return;
  }

  if (target.classList.contains("revoke-button")) {
    try {
      await revokeResource(target.dataset.kind, target.dataset.id);
      setStatus(target.dataset.kind === "share" ? "分享链接已删除。" : "token 已吊销。");
    } catch (error) {
      setStatus(error.message, true);
    }
  }
});

els.tokenForm?.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = event.currentTarget instanceof HTMLFormElement ? event.currentTarget : null;
  if (!form) return;

  const formData = new FormData(form);
  const scopeValue = String(formData.get("scope") || "");
  const rawProjectScope = String(formData.get("project_scope") || "").trim();
  const projectScope = scopeValue === "project:demo" ? rawProjectScope || "demo" : "";
  const scope = scopeValue === "project:demo" ? `project:${projectScope}` : scopeValue;

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
    form.reset();
    syncTokenProjectScopeField();
    state.selectedTokenId = result.token_id;
    await refreshDashboard();
    const token = selectedToken();
    if (token) {
      populateShareDefaults(token);
      renderSelectedToken();
      renderTokens();
    }
  } catch (error) {
    els.tokenOutput.className = "code-card";
    els.tokenOutput.textContent = error.message;
  }
});

els.shareForm?.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = event.currentTarget instanceof HTMLFormElement ? event.currentTarget : null;
  const token = selectedToken();
  if (!form || !token) {
    els.shareOutput.className = "code-card";
    els.shareOutput.textContent = "请先选择一个 token。";
    return;
  }

  const formData = new FormData(form);
  try {
    const payload = {
      share_name: String(formData.get("share_name") || "").trim(),
      token_name: token.token_name,
      scope: token.scope,
      project_scope: token.project_scope || "",
      share_expires_in: String(formData.get("share_expires_in") || "").trim(),
      max_claims: Number(formData.get("max_claims") || 1),
    };
    const result = await apiRequest("/v1/admin/share-links", {
      method: "POST",
      serverUrl: els.serverUrl.value.trim(),
      adminKey: els.adminKey.value.trim(),
      body: payload,
    });
    renderShareOutput(els.serverUrl.value.trim(), result);
    await refreshDashboard();
    populateShareDefaults(token);
  } catch (error) {
    els.shareOutput.className = "code-card";
    els.shareOutput.textContent = error.message;
  }
});

async function bootShareClaimView() {
  const params = new URLSearchParams(window.location.search);
  const code = extractShareCode();
  const serverUrl = params.get("server") || window.location.origin;

  if (!code) {
    return false;
  }

  els.shareClaim.classList.remove("hidden");
  els.dashboard.classList.add("hidden");
  els.connectForm.closest(".section").classList.add("hidden");

  try {
    const resolveUrl = new URL("/v1/share-links/resolve", serverUrl);
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
        const claimResult = await apiRequest(`/v1/share-links/claim?code=${encodeURIComponent(code)}`, {
          method: "GET",
          serverUrl,
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

  syncTokenProjectScopeField();

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
