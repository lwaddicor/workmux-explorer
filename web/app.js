"use strict";

// ---------- helpers ----------
const $ = (id) => document.getElementById(id);
const enc = encodeURIComponent;

function esc(s) {
  return String(s == null ? "" : s).replace(/[&<>"']/g, (c) => (
    { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]
  ));
}

async function api(path, opts = {}) {
  const res = await fetch(path, opts);
  let data = null;
  try { data = await res.json(); } catch (e) { /* not json */ }
  if (!res.ok) {
    const msg = (data && data.error) || res.statusText || ("HTTP " + res.status);
    throw new Error(msg);
  }
  return data;
}

function fmtElapsed(secs) {
  if (secs == null || isNaN(secs)) return "";
  secs = Math.max(0, Math.floor(secs));
  if (secs < 60) return secs + "s";
  const m = Math.floor(secs / 60);
  if (m < 60) return m + "m " + (secs % 60) + "s";
  const h = Math.floor(m / 60);
  if (h < 24) return h + "h " + (m % 60) + "m";
  const d = Math.floor(h / 24);
  return d + "d " + (h % 24) + "h";
}

function fmtCreated(unix) {
  if (!unix) return "";
  const diff = Math.floor(Date.now() / 1000 - unix);
  if (diff < 60) return "just now";
  if (diff < 3600) return Math.floor(diff / 60) + "m ago";
  if (diff < 86400) return Math.floor(diff / 3600) + "h ago";
  const days = Math.floor(diff / 86400);
  return days + "d ago (" + new Date(unix * 1000).toLocaleDateString() + ")";
}

// ---------- render ----------
let lastInv = null;
let selectedProject = null;

function render(inv) {
  lastInv = inv;
  const grid = $("worktree-grid");
  const empty = $("empty");
  const banner = $("banner");
  const badge = $("status-badge");

  if (inv.degraded) {
    banner.textContent = inv.degraded;
    banner.className = "banner";
    banner.classList.remove("hidden");
  } else {
    banner.classList.add("hidden");
  }

  const projects = inv.projects || [];
  const totalW = projects.reduce((n, p) => n + (p.worktrees ? p.worktrees.length : 0), 0);
  const totalA = projects.reduce((n, p) => n + (p.worktrees ? p.worktrees.filter((w) => w.agent).length : 0), 0);
  badge.textContent = projects.length + " project(s) · " + totalW + " worktree(s) · " + totalA + " active";

  if (projects.length === 0) {
    $("sidebar").innerHTML = "";
    grid.innerHTML = "";
    empty.classList.remove("hidden");
    $("empty-text").textContent = inv.degraded || "No worktrees found.";
    return;
  }
  empty.classList.add("hidden");
  renderSidebar(projects);
  renderGrid(projects);
}

function renderSidebar(projects) {
  $("sidebar").innerHTML = projects.map((p) => {
    const wts = p.worktrees || [];
    const active = wts.filter((w) => w.agent).length;
    const sel = p.name === selectedProject ? ' aria-current="true"' : "";
    const err = p.error ? `<span class="repo-err">⚠ ${esc(p.error)}</span>` : "";
    return `<button class="repo-item" data-project="${esc(p.name)}"${sel}>
      <span class="repo-name">${esc(p.name)}</span>
      <span class="repo-count">${wts.length} worktree(s) · ${active} active</span>
      ${err}
    </button>`;
  }).join("");
}

function renderGrid(projects) {
  const p = projects.find((x) => x.name === selectedProject) || projects[0];
  selectedProject = p.name;
  localStorage.setItem("gt_selected_project", p.name);
  const wts = p.worktrees || [];
  if (!wts.length) {
    $("worktree-grid").innerHTML = '<div class="grid-empty">No worktrees</div>';
    return;
  }
  $("worktree-grid").innerHTML = wts.map((w) => renderWorktree(w, p.name)).join("");
}

function renderWorktree(w, projectName) {
  const active = !!w.agent;
  const statusCls = active ? "badge-" + (w.agent.status || "working") : "badge-closed";
  const statusTxt = active ? (w.agent.status || "working") : "no agent";

  const badges = [
    `<span class="badge ${statusCls}">${esc(statusTxt)}</span>`,
    w.is_main ? `<span class="badge badge-main">main</span>` : "",
    w.is_open ? `<span class="badge badge-open">window open</span>` : `<span class="badge badge-closed">window closed</span>`,
    w.has_uncommitted_changes ? `<span class="badge badge-dirty">uncommitted</span>` : "",
  ].join("");

  let git = "";
  if (w.agent && w.agent.git) {
    const g = w.agent.git;
    git = `<div class="wt-git">
      <span class="${g.has_staged ? "on" : ""}">staged</span>
      <span class="${g.has_unstaged ? "on" : ""}">unstaged</span>
      <span class="${g.has_unmerged_commits ? "on" : ""}">unmerged commits</span>
    </div>`;
  }

  const kind = (w.agent && w.agent.agent_kind) ? w.agent.agent_kind : (active ? "unknown" : "");
  const meta = [
    active && kind ? `<span>agent: ${esc(kind)}</span>` : "",
    active && w.agent.elapsed_secs != null ? `<span>elapsed: ${esc(fmtElapsed(w.agent.elapsed_secs))}</span>` : "",
    w.created_at ? `<span>created: ${esc(fmtCreated(w.created_at))}</span>` : "",
  ].filter(Boolean).join("");

  const title = (w.agent && w.agent.title) ? `<div class="wt-title">${esc(w.agent.title)}</div>` : "";
  const sendDisabled = w.agent ? "" : "disabled";
  const outDisabled = w.agent ? "" : "disabled";
  const cls = active ? "active" : "closed";

  return `<article class="worktree ${cls}" data-handle="${esc(w.handle)}">
    <div class="wt-top">
      <span class="handle">${esc(w.handle)}</span>
      ${badges}
    </div>
    <div class="wt-branch">${esc(w.branch)}</div>
    ${title}
    <div class="wt-meta">${meta}</div>
    ${git}
    <div class="wt-actions">
      <button data-act="open" data-project="${esc(projectName)}" data-handle="${esc(w.handle)}">Open window</button>
      <button data-act="close" data-project="${esc(projectName)}" data-handle="${esc(w.handle)}">Close window</button>
      <button data-act="send" data-project="${esc(projectName)}" data-handle="${esc(w.handle)}" ${sendDisabled}>Send prompt</button>
      <button data-act="output" data-project="${esc(projectName)}" data-handle="${esc(w.handle)}" ${outDisabled}>Output</button>
      <button data-act="remove" class="danger" data-project="${esc(projectName)}" data-handle="${esc(w.handle)}">Remove</button>
    </div>
  </article>`;
}

// ---------- polling ----------
let pollTimer = null;
let paused = false;

function getPollMs() {
  const v = parseInt($("poll-interval").value, 10);
  return v >= 1000 ? v : 4000;
}

async function load() {
  try {
    const inv = await api("/api/projects");
    render(inv);
    $("last-updated").textContent = "updated " + new Date().toLocaleTimeString();
  } catch (e) {
    const banner = $("banner");
    banner.textContent = "Failed to load inventory: " + e.message;
    banner.className = "banner";
    banner.classList.remove("hidden");
  }
}

function startPolling() {
  stopPolling();
  if (paused) return;
  pollTimer = setInterval(load, getPollMs());
}
function stopPolling() { if (pollTimer) { clearInterval(pollTimer); pollTimer = null; } }

function togglePause() {
  paused = !paused;
  $("pause-btn").textContent = paused ? "Resume" : "Pause";
  if (paused) stopPolling();
  else { load(); startPolling(); }
}

// ---------- actions ----------
async function doAction(project, handle, action) {
  try {
    await api(`/api/projects/${enc(project)}/worktrees/${enc(handle)}/${action}`, { method: "POST" });
    await load();
  } catch (e) {
    alert(action + " failed: " + e.message);
  }
}

function doRemove(project, handle, dirty) {
  let msg = 'Remove worktree "' + handle + '"?\n\nThis deletes its worktree directory, its tmux window, and its local branch. This cannot be undone.';
  if (dirty) msg += "\n\n⚠ WARNING: this worktree has UNCOMMITTED CHANGES that will be permanently lost.";
  if (!confirm(msg)) return;
  removeSubmit(project, handle);
}

async function removeSubmit(project, handle) {
  try {
    await api(`/api/projects/${enc(project)}/worktrees/${enc(handle)}/remove`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ confirmed: true }),
    });
    await load();
  } catch (e) {
    alert("remove failed: " + e.message);
  }
}

// ---------- send prompt ----------
let sendTarget = null;
function openSend(project, handle) {
  sendTarget = { project, handle };
  $("send-handle").textContent = handle;
  $("send-text").value = "";
  $("send-error").classList.add("hidden");
  $("send-modal").classList.remove("hidden");
  $("send-text").focus();
}
function closeSend() { $("send-modal").classList.add("hidden"); sendTarget = null; }

async function submitSend() {
  const text = $("send-text").value;
  const errEl = $("send-error");
  if (!text.trim()) { errEl.textContent = "Please enter a prompt."; errEl.classList.remove("hidden"); return; }
  const { project, handle } = sendTarget;
  try {
    await api(`/api/projects/${enc(project)}/worktrees/${enc(handle)}/send`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text }),
    });
    closeSend();
    await load();
  } catch (e) {
    errEl.textContent = e.message;
    errEl.classList.remove("hidden");
  }
}

// ---------- detail / output panel ----------
let detail = null;
let detailTimer = null;

async function openDetail(project, handle) {
  detail = { project, handle };
  $("detail").classList.remove("hidden");
  $("detail-title").textContent = handle;
  $("detail-output").textContent = "Loading…";
  $("detail-status").textContent = "";
  await refreshDetail();
  startDetailTimer();
}
function closeDetail() {
  stopDetailTimer();
  detail = null;
  $("detail").classList.add("hidden");
}
function startDetailTimer() { stopDetailTimer(); detailTimer = setInterval(refreshDetail, Math.min(getPollMs(), 3000)); }
function stopDetailTimer() { if (detailTimer) { clearInterval(detailTimer); detailTimer = null; } }

async function refreshDetail() {
  if (!detail) return;
  try {
    const data = await api(`/api/projects/${enc(detail.project)}/worktrees/${enc(detail.handle)}/output`);
    $("detail-output").textContent = (data.output && data.output.length) ? data.output : "(no output yet)";
    $("detail-status").textContent = "updated " + new Date().toLocaleTimeString();
  } catch (e) {
    const out = $("detail-output");
    if (out.textContent === "Loading…") out.textContent = e.message;
    else $("detail-status").textContent = "refresh error: " + e.message;
  }
}

// ---------- wiring ----------
function onWorktreeClick(e) {
  const btn = e.target.closest("button[data-act]");
  if (!btn) return;
  const act = btn.dataset.act;
  const project = btn.dataset.project;
  const handle = btn.dataset.handle;
  if (act === "open") doAction(project, handle, "open");
  else if (act === "close") doAction(project, handle, "close");
  else if (act === "send") openSend(project, handle);
  else if (act === "output") openDetail(project, handle);
  else if (act === "remove") {
    const card = btn.closest(".worktree");
    const dirty = card && card.querySelector(".badge-dirty") !== null;
    doRemove(project, handle, dirty);
  }
}

function init() {
  const saved = localStorage.getItem("gt_poll");
  if (saved) $("poll-interval").value = saved;

  selectedProject = localStorage.getItem("gt_selected_project") || null;

  $("poll-interval").addEventListener("change", (e) => { localStorage.setItem("gt_poll", e.target.value); startPolling(); });
  $("pause-btn").addEventListener("click", togglePause);
  $("sidebar").addEventListener("click", (e) => {
    const btn = e.target.closest("button[data-project]");
    if (!btn || !lastInv) return;
    selectedProject = btn.dataset.project;
    localStorage.setItem("gt_selected_project", selectedProject);
    render(lastInv);
  });
  $("worktree-grid").addEventListener("click", onWorktreeClick);
  $("detail-close").addEventListener("click", closeDetail);
  $("send-cancel").addEventListener("click", closeSend);
  $("send-submit").addEventListener("click", submitSend);
  $("send-text").addEventListener("keydown", (e) => { if ((e.metaKey || e.ctrlKey) && e.key === "Enter") submitSend(); });
  $("send-modal").addEventListener("click", (e) => { if (e.target.id === "send-modal") closeSend(); });

  load();
  startPolling();
}

document.addEventListener("DOMContentLoaded", init);
