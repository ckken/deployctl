const storageKeys = {
  adminKey: "deployctl.adminKey",
};

const state = {
  adminKey: "",
  links: [],
  uploads: [],
  latestLink: null,
};

const els = {
  authView: document.getElementById("auth-view"),
  dashboardView: document.getElementById("dashboard-view"),
  loginForm: document.getElementById("login-form"),
  loginAdminKey: document.getElementById("login-admin-key"),
  loginStatus: document.getElementById("login-status"),
  loginServerOrigin: document.getElementById("login-server-origin"),
  serverOrigin: document.getElementById("server-origin"),
  grantForm: document.getElementById("grant-form"),
  folder: document.getElementById("folder"),
  expiresIn: document.getElementById("expires-in"),
  maxFiles: document.getElementById("max-files"),
  status: document.getElementById("status"),
  refreshData: document.getElementById("refresh-data"),
  logoutButton: document.getElementById("logout-button"),
  topLogoutButton: document.getElementById("top-logout-button"),
  latestLink: document.getElementById("latest-link"),
  linksList: document.getElementById("links-list"),
  uploadsList: document.getElementById("uploads-list"),
  metricLinks: document.getElementById("metric-links"),
  metricRemaining: document.getElementById("metric-remaining"),
  metricUploads: document.getElementById("metric-uploads"),
  metricLatest: document.getElementById("metric-latest"),
};

function setStatus(target, text, tone = "") {
  target.textContent = text;
  target.className = "status";
  if (tone) {
    target.classList.add(`is-${tone}`);
  }
}

function setAppStatus(text, tone = "") {
  setStatus(els.status, text, tone);
}

function setLoginStatus(text, tone = "") {
  setStatus(els.loginStatus, text, tone);
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function formatDate(value, fallback = "无") {
  if (!value) return fallback;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  return date.toLocaleString("zh-CN", { hour12: false });
}

function formatBytes(bytes) {
  const size = Number(bytes);
  if (!Number.isFinite(size) || size < 0) return "0 B";
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}

async function apiRequest(path, { method = "GET", body } = {}) {
  const response = await fetch(new URL(path, window.location.origin), {
    method,
    headers: {
      "Content-Type": "application/json",
      "X-Admin-Secret": state.adminKey,
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const json = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(json.message || `request failed: ${response.status}`);
  }
  return json;
}

async function copyText(text) {
  await navigator.clipboard.writeText(text);
}

function showAuth() {
  els.authView.classList.remove("is-hidden");
  els.dashboardView.classList.add("is-hidden");
  els.loginAdminKey.value = state.adminKey;
}

function showDashboard() {
  els.authView.classList.add("is-hidden");
  els.dashboardView.classList.remove("is-hidden");
}

function resetDashboardData() {
  state.links = [];
  state.uploads = [];
  state.latestLink = null;
  renderAll();
}

function renderMetrics() {
  const remaining = state.links.reduce((total, item) => total + Math.max(Number(item.remaining_files) || 0, 0), 0);
  const latestUpload = state.uploads
    .slice()
    .sort((a, b) => new Date(b.uploaded_at).getTime() - new Date(a.uploaded_at).getTime())[0];
  const latestLink = state.links
    .slice()
    .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())[0];
  const latestAt = latestUpload?.uploaded_at || latestLink?.created_at || "";

  els.metricLinks.textContent = state.links.length;
  els.metricRemaining.textContent = remaining;
  els.metricUploads.textContent = state.uploads.length;
  els.metricLatest.textContent = latestAt ? formatDate(latestAt) : "无";
}

function renderLatestLink() {
  if (!state.latestLink) {
    els.latestLink.className = "empty";
    els.latestLink.textContent = "还没有生成上传链接。";
    return;
  }

  const link = state.latestLink;
  els.latestLink.className = "record-card latest-card";
  els.latestLink.innerHTML = `
    <div>
      <p class="record-title">刚生成的上传链接</p>
      <p class="record-meta">folder: ${escapeHTML(link.folder)} · max_files: ${escapeHTML(link.max_files)} · expires: ${escapeHTML(formatDate(link.expires_at, "24h 默认"))}</p>
    </div>
    <a class="record-link" href="${escapeHTML(link.upload_url)}" target="_blank" rel="noreferrer">${escapeHTML(link.upload_url)}</a>
    <div class="record-actions">
      <button class="copy-button" type="button" data-copy="${escapeHTML(link.upload_url)}">复制链接</button>
    </div>
  `;
}

function renderLinks() {
  if (!state.links.length) {
    els.linksList.className = "records empty";
    els.linksList.textContent = "还没有可用上传链接。";
    return;
  }

  els.linksList.className = "records";
  els.linksList.innerHTML = state.links
    .slice()
    .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
    .map(
      (item) => `
        <article class="record-card">
          <div class="record-main">
            <p class="record-title">${escapeHTML(item.folder)}</p>
            <p class="record-meta">grant_id: ${escapeHTML(item.grant_id)} · 剩余 ${escapeHTML(item.remaining_files)} / ${escapeHTML(item.max_files)} · expires: ${escapeHTML(formatDate(item.expires_at, "24h 默认"))}</p>
            <a class="record-link" href="${escapeHTML(item.upload_url)}" target="_blank" rel="noreferrer">${escapeHTML(item.upload_url)}</a>
          </div>
          <div class="record-actions">
            <button class="copy-button" type="button" data-copy="${escapeHTML(item.upload_url)}">复制</button>
            <button class="delete-button" type="button" data-delete="${escapeHTML(item.grant_id)}">删除</button>
          </div>
        </article>
      `
    )
    .join("");
}

function renderUploads() {
  if (!state.uploads.length) {
    els.uploadsList.className = "records empty";
    els.uploadsList.textContent = "还没有上传记录。";
    return;
  }

  els.uploadsList.className = "records";
  els.uploadsList.innerHTML = state.uploads
    .slice()
    .sort((a, b) => new Date(b.uploaded_at).getTime() - new Date(a.uploaded_at).getTime())
    .map(
      (item) => `
        <article class="record-card">
          <div class="record-main">
            <p class="record-title">${escapeHTML(item.original_file_name)}</p>
            <p class="record-meta">${escapeHTML(item.saved_path)} · ${escapeHTML(formatBytes(item.size_bytes))} · ${escapeHTML(formatDate(item.uploaded_at))}</p>
            <a class="record-link" href="${escapeHTML(item.file_url)}" target="_blank" rel="noreferrer">${escapeHTML(item.file_url)}</a>
          </div>
          <div class="record-actions">
            <button class="copy-button" type="button" data-copy="${escapeHTML(item.file_url)}">复制地址</button>
          </div>
        </article>
      `
    )
    .join("");
}

function renderAll() {
  renderMetrics();
  renderLatestLink();
  renderLinks();
  renderUploads();
}

async function refreshDashboard({ quiet = false } = {}) {
  if (!state.adminKey) {
    resetDashboardData();
    showAuth();
    setLoginStatus(`当前服务：${window.location.origin}`);
    return;
  }

  const bootstrap = await apiRequest("/v1/admin/bootstrap");
  state.links = bootstrap.upload_links || [];
  state.uploads = bootstrap.uploads || [];
  localStorage.setItem(storageKeys.adminKey, state.adminKey);
  els.serverOrigin.textContent = bootstrap.server_url || window.location.origin;
  renderAll();
  showDashboard();
  if (!quiet) {
    setAppStatus("数据已刷新。", "success");
  }
}

async function login(event) {
  event.preventDefault();
  state.adminKey = els.loginAdminKey.value.trim();
  if (!state.adminKey) {
    setLoginStatus("请输入 adminKey。", "error");
    return;
  }
  setLoginStatus("正在连接...");
  try {
    await refreshDashboard({ quiet: true });
    setAppStatus("已进入 dashboard。", "success");
  } catch (error) {
    localStorage.removeItem(storageKeys.adminKey);
    setLoginStatus(error.message, "error");
    resetDashboardData();
    showAuth();
  }
}

function logout() {
  localStorage.removeItem(storageKeys.adminKey);
  state.adminKey = "";
  els.loginAdminKey.value = "";
  resetDashboardData();
  showAuth();
  setLoginStatus(`已退出。当前服务：${window.location.origin}`, "success");
}

async function createGrant(event) {
  event.preventDefault();
  const payload = {
    folder: els.folder.value.trim(),
    expires_in: els.expiresIn.value.trim() || "24h",
    max_files: Number.parseInt(els.maxFiles.value, 10) || 1,
  };

  try {
    const result = await apiRequest("/v1/admin/upload-links", {
      method: "POST",
      body: payload,
    });
    state.latestLink = result;
    els.folder.value = "";
    let copyError = null;
    try {
      await copyText(result.upload_url);
    } catch (error) {
      copyError = error;
    }
    await refreshDashboard({ quiet: true });
    if (copyError) {
      setAppStatus(`上传链接已生成，但复制失败：${copyError.message}`, "error");
      return;
    }
    setAppStatus("上传链接已生成并复制。", "success");
  } catch (error) {
    setAppStatus(error.message, "error");
  }
}

async function deleteGrant(grantID) {
  try {
    await apiRequest(`/v1/admin/upload-links/${encodeURIComponent(grantID)}`, { method: "DELETE" });
    if (state.latestLink?.grant_id === grantID) {
      state.latestLink = null;
    }
    await refreshDashboard({ quiet: true });
    setAppStatus("上传链接已删除。", "success");
  } catch (error) {
    setAppStatus(error.message, "error");
  }
}

async function handleRecordClick(event) {
  const copyTarget = event.target.closest("[data-copy]");
  if (copyTarget) {
    const value = copyTarget.getAttribute("data-copy");
    if (value) {
      try {
        await copyText(value);
        setAppStatus("已复制。", "success");
      } catch (error) {
        setAppStatus(error.message, "error");
      }
    }
    return;
  }

  const deleteTarget = event.target.closest("[data-delete]");
  if (deleteTarget) {
    await deleteGrant(deleteTarget.getAttribute("data-delete"));
  }
}

function init() {
  els.loginServerOrigin.textContent = window.location.origin;
  els.serverOrigin.textContent = window.location.origin;
  state.adminKey = localStorage.getItem(storageKeys.adminKey) || "";
  if (state.adminKey) {
    setLoginStatus("正在恢复会话...");
    refreshDashboard({ quiet: true }).catch((error) => {
      localStorage.removeItem(storageKeys.adminKey);
      state.adminKey = "";
      setLoginStatus(error.message, "error");
      resetDashboardData();
      showAuth();
    });
  } else {
    resetDashboardData();
    showAuth();
    setLoginStatus(`当前服务：${window.location.origin}`);
  }
}

els.loginForm.addEventListener("submit", login);
els.grantForm.addEventListener("submit", createGrant);
els.refreshData.addEventListener("click", () => {
  refreshDashboard().catch((error) => setAppStatus(error.message, "error"));
});
els.logoutButton.addEventListener("click", logout);
els.topLogoutButton.addEventListener("click", logout);
els.linksList.addEventListener("click", handleRecordClick);
els.latestLink.addEventListener("click", handleRecordClick);
els.uploadsList.addEventListener("click", handleRecordClick);

init();
