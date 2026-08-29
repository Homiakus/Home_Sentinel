const LOG_CARD_ID = 'settings-runtime-logs';
const DISPLAY_LIMIT = 250;
const REFRESH_MS = 5000;

let refreshTimer = null;
let renderQueued = false;

function isSettingsRoute() {
  return (location.hash || '').split('/')[0] === '#settings';
}

function installStyles() {
  if (document.getElementById('settings-runtime-log-styles')) return;
  const style = document.createElement('style');
  style.id = 'settings-runtime-log-styles';
  style.textContent = `
    #${LOG_CARD_ID} { margin-top: 24px; }
    #${LOG_CARD_ID} .runtime-log-toolbar { display:flex; gap:8px; align-items:center; flex-wrap:wrap; }
    #${LOG_CARD_ID} .runtime-log-meta { margin-top:8px; font-size:.82rem; opacity:.72; }
    #${LOG_CARD_ID} .runtime-log-view {
      margin:14px 0 0;
      min-height:180px;
      max-height:420px;
      overflow:auto;
      padding:14px;
      border-radius:10px;
      border:1px solid rgba(148,163,184,.18);
      background:rgba(3,7,18,.62);
      color:#d8e2f0;
      font:12px/1.55 'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      white-space:pre-wrap;
      overflow-wrap:anywhere;
      user-select:text;
    }
    #${LOG_CARD_ID} .runtime-log-view[data-empty='true'] { color:rgba(216,226,240,.55); }
    #${LOG_CARD_ID} .runtime-log-live { display:inline-flex; align-items:center; gap:6px; font-size:.8rem; opacity:.8; }
    #${LOG_CARD_ID} .runtime-log-live input { accent-color:currentColor; }
    @media (max-width: 640px) {
      #${LOG_CARD_ID} .runtime-log-toolbar .btn { flex:1 1 auto; }
      #${LOG_CARD_ID} .runtime-log-view { max-height:55vh; font-size:11px; }
    }
  `;
  document.head.append(style);
}

async function fetchRuntimeLogs(limit = DISPLAY_LIMIT) {
  const response = await fetch(`/api/v1/system/logs?limit=${encodeURIComponent(limit)}`, {
    credentials: 'same-origin',
    headers: { Accept: 'application/json' }
  });
  let payload = null;
  try { payload = await response.json(); } catch {}
  if (!response.ok) {
    const error = new Error(payload?.message || `HTTP ${response.status}`);
    error.status = response.status;
    throw error;
  }
  return payload || {};
}

function cardTemplate() {
  const section = document.createElement('section');
  section.id = LOG_CARD_ID;
  section.className = 'card';
  section.innerHTML = `
    <div class="card-header" style="align-items:flex-start; gap:12px; flex-wrap:wrap;">
      <div>
        <h2>Логи программы</h2>
        <p class="muted text-sm" style="margin-top:4px;">Логи текущего запуска Home Sentinel и его модулей. Чувствительные поля редактируются до отображения.</p>
      </div>
      <div class="runtime-log-toolbar">
        <label class="runtime-log-live" title="Автоматически обновлять логи">
          <input id="runtime-log-auto" type="checkbox" checked>
          Автообновление
        </label>
        <button class="btn btn-ghost btn-sm" type="button" id="runtime-log-refresh">Обновить</button>
        <button class="btn btn-primary btn-sm" type="button" id="runtime-log-copy">Скопировать все логи</button>
      </div>
    </div>
    <div class="runtime-log-meta" id="runtime-log-meta">Загрузка логов…</div>
    <pre class="runtime-log-view" id="runtime-log-view" data-empty="true">Загрузка…</pre>
  `;
  return section;
}

async function refreshLogs({ preserveScroll = false } = {}) {
  const card = document.getElementById(LOG_CARD_ID);
  if (!card || !isSettingsRoute()) return;
  const view = card.querySelector('#runtime-log-view');
  const meta = card.querySelector('#runtime-log-meta');
  const refresh = card.querySelector('#runtime-log-refresh');
  if (!view || !meta) return;

  const nearBottom = view.scrollHeight - view.scrollTop - view.clientHeight < 36;
  if (refresh) refresh.disabled = true;
  try {
    const data = await fetchRuntimeLogs(DISPLAY_LIMIT);
    const lines = Array.isArray(data.lines) ? data.lines : [];
    view.textContent = lines.length ? lines.join('\n') : 'Логи текущего запуска пока пусты.';
    view.dataset.empty = lines.length ? 'false' : 'true';
    meta.textContent = `Показано ${lines.length} из ${Number(data.retained || 0)} сохранённых строк · буфер ${Number(data.capacity || 0)} · ${new Date(data.captured_at || Date.now()).toLocaleTimeString('ru-RU')}`;
    if (!preserveScroll || nearBottom) view.scrollTop = view.scrollHeight;
  } catch (error) {
    view.textContent = error.status === 403
      ? 'Просмотр логов доступен только администратору.'
      : `Не удалось загрузить логи: ${error.message}`;
    view.dataset.empty = 'true';
    meta.textContent = 'Логи недоступны';
  } finally {
    if (refresh) refresh.disabled = false;
  }
}

async function copyAllLogs(button) {
  const original = button.textContent;
  button.disabled = true;
  button.textContent = 'Копирование…';
  try {
    const data = await fetchRuntimeLogs(4096);
    const lines = Array.isArray(data.lines) ? data.lines : [];
    const text = lines.join('\n');
    await copyText(text);
    button.textContent = `Скопировано: ${lines.length}`;
    setTimeout(() => { button.textContent = original; }, 1800);
  } catch (error) {
    button.textContent = 'Ошибка копирования';
    setTimeout(() => { button.textContent = original; }, 1800);
  } finally {
    button.disabled = false;
  }
}

async function copyText(text) {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.setAttribute('readonly', '');
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  document.body.append(textarea);
  textarea.select();
  const ok = document.execCommand('copy');
  textarea.remove();
  if (!ok) throw new Error('Clipboard API unavailable');
}

function stopAutoRefresh() {
  if (refreshTimer !== null) {
    clearInterval(refreshTimer);
    refreshTimer = null;
  }
}

function startAutoRefresh() {
  stopAutoRefresh();
  const checkbox = document.querySelector(`#${LOG_CARD_ID} #runtime-log-auto`);
  if (!isSettingsRoute() || !checkbox?.checked) return;
  refreshTimer = setInterval(() => refreshLogs({ preserveScroll: true }), REFRESH_MS);
}

function enhanceSettings() {
  renderQueued = false;
  if (!isSettingsRoute()) {
    stopAutoRefresh();
    return;
  }
  installStyles();
  const content = document.getElementById('content');
  if (!content || document.getElementById(LOG_CARD_ID)) return;

  const card = cardTemplate();
  content.append(card);
  card.querySelector('#runtime-log-refresh').addEventListener('click', () => refreshLogs());
  card.querySelector('#runtime-log-copy').addEventListener('click', event => copyAllLogs(event.currentTarget));
  card.querySelector('#runtime-log-auto').addEventListener('change', startAutoRefresh);
  refreshLogs();
  startAutoRefresh();
}

function scheduleEnhance() {
  if (renderQueued) return;
  renderQueued = true;
  requestAnimationFrame(enhanceSettings);
}

const content = document.getElementById('content');
if (content) {
  new MutationObserver(scheduleEnhance).observe(content, { childList: true });
}
window.addEventListener('hashchange', scheduleEnhance);
window.addEventListener('beforeunload', stopAutoRefresh);
scheduleEnhance();
