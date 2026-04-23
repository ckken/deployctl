const storageKeys = {
  serverUrl: "deployctl.serverUrl",
  adminKey: "deployctl.adminKey",
};

const state = {
  tokens: [],
  shares: [],
  selectedTokenId: "",
};

const quickShareDefaults = {
  expiresIn: "7d",
  maxClaims: 1,
};

const els = {
  unlockSection: document.getElementById("unlock-section"),
  connectForm: document.getElementById("connect-form"),
  serverUrl: document.getElementById("server-url"),
  adminKey: document.getElementById("admin-key"),
  connectStatus: document.getElementById("connect-status"),
  clearSession: document.getElementById("clear-session"),
  manageSession: document.getElementById("manage-session"),
  dashboard: document.getElementById("dashboard"),
  shareClaim: document.getElementById("share-claim"),
  tokenForm: document.getElementById("token-form"),
  tokenOutput: document.getElementById("token-output"),
  shareOutput: document.getElementById("share-output"),
  createDefaultToken: document.getElementById("create-default-token"),
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
  const serverUrl = window.location.origin;
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
  els.unlockSection.classList.remove("hidden");
  els.manageSession.classList.add("hidden");
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

function buildQuickShareName(token) {
  return `${token.token_name} 分享`;
}

function buildDefaultTokenName() {
  return `quick-share-${Date.now().toString(36).slice(-6)}`;
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
            <button class="button button-primary quick-share-button" data-id="${item.token_id}">复制分享链接</button>
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

async function createToken(payload) {
  return apiRequest("/v1/admin/tokens", {
    method: "POST",
    serverUrl: els.serverUrl.value.trim(),
    adminKey: els.adminKey.value.trim(),
    body: payload,
  });
}

async function createQuickShare(token, shouldCopy = true) {
  const payload = {
    share_name: buildQuickShareName(token),
    token_name: token.token_name,
    scope: token.scope,
    project_scope: token.project_scope || "",
    share_expires_in: quickShareDefaults.expiresIn,
    max_claims: quickShareDefaults.maxClaims,
  };

  const result = await apiRequest("/v1/admin/share-links", {
    method: "POST",
    serverUrl: els.serverUrl.value.trim(),
    adminKey: els.adminKey.value.trim(),
    body: payload,
  });

  const link = agentLinkForShare(els.serverUrl.value.trim(), result);
  let copied = false;
  if (shouldCopy) {
    try {
      await copyText(link);
      copied = true;
    } catch (error) {
      setStatus(error.message, true);
    }
  }
  renderShareOutput(result, copied);
  await refreshDashboard();
  return result;
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

function renderShareOutput(result, copied = false) {
  const link = agentLinkForShare(els.serverUrl.value.trim(), result);
  els.shareOutput.className = "code-card link-output-card";
  els.shareOutput.innerHTML = `
    <div class="link-output-title">${copied ? "分享链接已复制" : "分享链接已生成"}</div>
    <a class="link-output-anchor" href="${link}" target="_blank" rel="noreferrer">${link}</a>
    <div class="link-output-meta">默认策略：${quickShareDefaults.expiresIn} · ${quickShareDefaults.maxClaims} 次领取 · ${result.scope}</div>
  `;
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
  }

  renderTokens();
  renderShares();
  els.unlockSection.classList.add("hidden");
  els.dashboard.classList.remove("hidden");
  els.manageSession.classList.remove("hidden");
  setStatus(`已连接 ${bootstrap.server_url}`);
}

els.connectForm?.addEventListener("submit", async (event) => {
  event.preventDefault();
  const serverUrl = window.location.origin;
  const adminKey = els.adminKey.value.trim();
  try {
    els.serverUrl.value = serverUrl;
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

els.manageSession?.addEventListener("click", () => {
  els.unlockSection.classList.remove("hidden");
  els.dashboard.classList.add("hidden");
  els.manageSession.classList.add("hidden");
  els.adminKey.focus();
});

els.tokenForm?.elements?.scope?.addEventListener("change", syncTokenProjectScopeField);

els.createDefaultToken?.addEventListener("click", async () => {
  try {
    const result = await createToken({
      name: buildDefaultTokenName(),
      scope: "read-only",
      project_scope: "",
      expires_in: "",
    });
    els.tokenOutput.className = "code-card";
    els.tokenOutput.textContent = formatJSON(result);
    state.selectedTokenId = result.token_id;
    await refreshDashboard();
    setStatus("默认 token 已创建。");
  } catch (error) {
    els.tokenOutput.className = "code-card";
    els.tokenOutput.textContent = error.message;
  }
});

document.addEventListener("click", async (event) => {
  const target = event.target;
  if (!(target instanceof HTMLElement)) {
    return;
  }

  if (target.classList.contains("quick-share-button")) {
    const token = activeTokens().find((item) => item.token_id === target.dataset.id);
    if (!token) {
      setStatus("token 不存在或已失效", true);
      return;
    }
    state.selectedTokenId = token.token_id;
    renderTokens();
    try {
      await createQuickShare(token, true);
      setStatus("分享链接已复制。");
    } catch (error) {
      els.shareOutput.className = "code-card";
      els.shareOutput.textContent = error.message;
    }
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
    renderTokens();
  } catch (error) {
    els.tokenOutput.className = "code-card";
    els.tokenOutput.textContent = error.message;
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
  els.serverUrl.value = window.location.origin;
  if (session.adminKey) {
    try {
      await refreshDashboard();
    } catch (error) {
      els.unlockSection.classList.remove("hidden");
      setStatus(error.message, true);
    }
    return;
  }
  els.unlockSection.classList.remove("hidden");
}

init();
