const storageKeys = {
  adminKey: "deployctl.adminKey",
};

const state = {
  links: [],
  uploads: [],
  latestLink: null,
};

const els = {
  serverOrigin: document.getElementById("server-origin"),
  grantForm: document.getElementById("grant-form"),
  adminKey: document.getElementById("admin-key"),
  folder: document.getElementById("folder"),
  expiresIn: document.getElementById("expires-in"),
  maxFiles: document.getElementById("max-files"),
  status: document.getElementById("status"),
  clearAdminKey: document.getElementById("clear-admin-key"),
  refreshData: document.getElementById("refresh-data"),
  latestLink: document.getElementById("latest-link"),
  linksList: document.getElementById("links-list"),
  uploadsList: document.getElementById("uploads-list"),
};

function setStatus(text, tone = "") {
  els.status.textContent = text;
  els.status.className = "status";
  if (tone) {
    els.status.classList.add(`is-${tone}`);
  }
}

function currentAdminKey() {
  return els.adminKey.value.trim();
}

async function apiRequest(path, { method = "GET", body } = {}) {
  const response = await fetch(new URL(path, window.location.origin), {
    method,
    headers: {
      "Content-Type": "application/json",
      "X-Admin-Secret": currentAdminKey(),
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

function formatExpiry(expiresAt) {
  if (!expiresAt) return "24h 默认";
  const date = new Date(expiresAt);
  if (Number.isNaN(date.getTime())) return expiresAt;
  return date.toLocaleString("zh-CN", { hour12: false });
}

function renderLatestLink() {
  if (!state.latestLink) {
    els.latestLink.className = "empty";
    els.latestLink.textContent = "还没有生成上传链接。";
    return;
  }

  const link = state.latestLink;
  els.latestLink.className = "card";
  els.latestLink.innerHTML = `
    <div class="card-header">
      <div>
        <p class="card-title">刚生成的上传链接</p>
        <p class="card-meta">folder: ${link.folder} · max_files: ${link.max_files} · expires: ${formatExpiry(link.expires_at)}</p>
      </div>
    </div>
    <a class="card-link" href="${link.upload_url}" target="_blank" rel="noreferrer">${link.upload_url}</a>
    <div class="card-actions">
      <button class="copy-button" type="button" data-copy="${link.upload_url}">复制链接</button>
    </div>
  `;
}

function renderLinks() {
  if (!state.links.length) {
    els.linksList.className = "list empty";
    els.linksList.textContent = "还没有可用上传链接。";
    return;
  }
  els.linksList.className = "list";
  els.linksList.innerHTML = state.links
    .slice()
    .reverse()
    .map(
      (item) => `
        <article class="card">
          <div class="card-header">
            <div>
              <p class="card-title">${item.folder}</p>
              <p class="card-meta">grant_id: ${item.grant_id} · 剩余 ${item.remaining_files} / ${item.max_files} · expires: ${formatExpiry(item.expires_at)}</p>
            </div>
          </div>
          <div class="card-actions">
            <button class="copy-button" type="button" data-copy="${window.location.origin}/u/${item.grant_code || ""}" data-grant-id="${item.grant_id}">复制上传链接</button>
            <button class="delete-button" type="button" data-delete="${item.grant_id}">删除</button>
          </div>
        </article>
      `
    )
    .join("");
}

function renderUploads() {
  if (!state.uploads.length) {
    els.uploadsList.className = "list empty";
    els.uploadsList.textContent = "还没有上传记录。";
    return;
  }
  els.uploadsList.className = "list";
  els.uploadsList.innerHTML = state.uploads
    .slice()
    .reverse()
    .map(
      (item) => `
        <article class="card">
          <div class="card-header">
            <div>
              <p class="card-title">${item.original_file_name}</p>
              <p class="card-meta">${item.saved_path} · ${item.size_bytes} bytes · ${new Date(item.uploaded_at).toLocaleString("zh-CN", { hour12: false })}</p>
            </div>
          </div>
          <a class="card-link" href="${item.file_url}" target="_blank" rel="noreferrer">${item.file_url}</a>
          <div class="card-actions">
            <button class="copy-button" type="button" data-copy="${item.file_url}">复制文件地址</button>
          </div>
        </article>
      `
    )
    .join("");
}

async function refreshDashboard() {
  const adminKey = currentAdminKey();
  if (!adminKey) {
    state.links = [];
    state.uploads = [];
    state.latestLink = null;
    renderLatestLink();
    renderLinks();
    renderUploads();
    setStatus("输入 adminKey 后即可生成上传链接。");
    return;
  }

  localStorage.setItem(storageKeys.adminKey, adminKey);
  const bootstrap = await apiRequest("/v1/admin/bootstrap");
  state.links = bootstrap.upload_links || [];
  state.uploads = bootstrap.uploads || [];
  renderLatestLink();
  renderLinks();
  renderUploads();
  setStatus("已连接服务，可继续生成上传链接。", "success");
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
    await copyText(result.upload_url);
    setStatus("上传链接已生成并复制。", "success");
    els.folder.value = "";
    await refreshDashboard();
  } catch (error) {
    setStatus(error.message, "error");
  }
}

async function deleteGrant(grantID) {
  try {
    await apiRequest(`/v1/admin/upload-links/${grantID}`, { method: "DELETE" });
    if (state.latestLink?.grant_id === grantID) {
      state.latestLink = null;
    }
    await refreshDashboard();
    setStatus("上传链接已删除。", "success");
  } catch (error) {
    setStatus(error.message, "error");
  }
}

async function handleListClick(event) {
  const copyTarget = event.target.closest("[data-copy]");
  if (copyTarget) {
    let value = copyTarget.getAttribute("data-copy");
    const grantID = copyTarget.getAttribute("data-grant-id");
    if (grantID) {
      const link = state.links.find((item) => item.grant_id === grantID);
      if (link?.upload_url) {
        value = link.upload_url;
      }
    }
    if (value) {
      try {
        await copyText(value);
        setStatus("已复制。", "success");
      } catch (error) {
        setStatus(error.message, "error");
      }
    }
    return;
  }

  const deleteTarget = event.target.closest("[data-delete]");
  if (deleteTarget) {
    await deleteGrant(deleteTarget.getAttribute("data-delete"));
  }
}

function restoreSession() {
  els.serverOrigin.textContent = window.location.origin;
  els.adminKey.value = localStorage.getItem(storageKeys.adminKey) || "";
}

function clearAdminKey() {
  localStorage.removeItem(storageKeys.adminKey);
  els.adminKey.value = "";
  state.latestLink = null;
  refreshDashboard().catch((error) => setStatus(error.message, "error"));
}

restoreSession();
renderLatestLink();
renderLinks();
renderUploads();
refreshDashboard().catch((error) => setStatus(error.message, "error"));

els.grantForm.addEventListener("submit", createGrant);
els.refreshData.addEventListener("click", () => {
  refreshDashboard().catch((error) => setStatus(error.message, "error"));
});
els.clearAdminKey.addEventListener("click", clearAdminKey);
els.linksList.addEventListener("click", handleListClick);
els.latestLink.addEventListener("click", handleListClick);
els.uploadsList.addEventListener("click", handleListClick);
