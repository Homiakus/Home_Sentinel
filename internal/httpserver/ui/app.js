const $ = (q, root = document) => root.querySelector(q);
const $$ = (q, root = document) => [...root.querySelectorAll(q)];

const state = {
  csrf: sessionStorage.getItem('sentinel_csrf') || '',
  me: null,
  timers: [],
  eventSource: null,
  cameras: []
};

const titles = {
  overview: 'Обзор системы',
  cameras: 'Видеонаблюдение',
  entrance: 'Вход и Контроль доступа',
  events: 'Журнал событий',
  incidents: 'Инциденты и Тревоги',
  search: 'Универсальный поиск',
  system: 'Состояние оборудования',
  setup: 'Мастер настройки',
  settings: 'Настройки и Интеграции'
};

const esc = v => String(v ?? '').replace(/[&<>'"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[c]));
const fmtTime = v => { if (!v) return '—'; const d = new Date(v); return Number.isNaN(d.getTime()) ? '—' : d.toLocaleString('ru-RU'); };
const bytes = n => { n = Number(n || 0); if (!n) return '0 B'; const u = ['B', 'KB', 'MB', 'GB', 'TB']; let i = 0; while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; } return `${n.toFixed(i > 1 ? 1 : 0)} ${u[i]}`; };
const badge = v => `<span class="badge ${String(v || 'unknown').toLowerCase()}">${esc(v || 'UNKNOWN')}</span>`;

function toast(msg, bad = false) {
  const d = document.createElement('div');
  d.className = 'toast' + (bad ? ' bad' : '');
  d.textContent = msg;
  $('#toast-stack').append(d);
  setTimeout(() => d.remove(), 4000);
}

function clearTimers() {
  state.timers.forEach(clearInterval);
  state.timers = [];
}

async function api(path, opts = {}) {
  const headers = { Accept: 'application/json', ...(opts.headers || {}) };
  if (opts.body && !(opts.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json';
    if (typeof opts.body !== 'string') opts.body = JSON.stringify(opts.body);
  }
  if (!['GET', 'HEAD'].includes((opts.method || 'GET').toUpperCase()) && state.csrf) {
    headers['X-CSRF-Token'] = state.csrf;
  }
  const r = await fetch(path, { credentials: 'same-origin', ...opts, headers });
  if (r.status === 401) {
    showAuth(false);
    throw new Error('Сессия завершена. Требуется вход');
  }
  const ct = r.headers.get('content-type') || '';
  const data = ct.includes('json') ? await r.json() : await r.text();
  if (!r.ok) {
    const e = new Error(data?.message || data?.error || data?.detail || `HTTP ${r.status}`);
    e.status = r.status;
    e.code = data?.code;
    e.data = data;
    throw e;
  }
  return data;
}

async function apiFresh(path, opts = {}) {
  try {
    return await api(path, opts);
  } catch (e) {
    if (e.code === 'REAUTH_REQUIRED') {
      const ok = await reauth();
      if (ok) return api(path, opts);
    }
    throw e;
  }
}

async function init() {
  $('#auth-form').addEventListener('submit', authSubmit);
  $('#logout').onclick = logout;
  $('#refresh').onclick = () => renderRoute(true);
  $('#menu-button').onclick = () => $('#sidebar').classList.toggle('open');
  $('#quick-search-btn').onclick = () => { location.hash = '#search'; };

  setupAddCameraModal();

  window.addEventListener('hashchange', () => renderRoute());

  try {
    const setup = await api('/api/v1/setup/status');
    if (setup.needs_admin) {
      showAuth(true);
      return;
    }
  } catch (e) {
    toast(e.message, true);
    return;
  }

  try {
    state.me = await api('/api/v1/auth/me');
    const c = await api('/api/v1/auth/csrf');
    state.csrf = c.csrf_token;
    sessionStorage.setItem('sentinel_csrf', state.csrf);
    showApp();
    connectRealtime();
    renderRoute();
  } catch (e) {
    showAuth(false);
  }
}

function showAuth(setup) {
  $('#app').classList.add('hidden');
  $('#auth-screen').classList.remove('hidden');
  $('#auth-title').textContent = setup ? 'Создать администратора' : 'Вход в Home Sentinel';
  $('#auth-subtitle').textContent = setup ? 'Задайте логин и пароль для первого входа' : 'Локальный центр управления безопасностью';
  $('#auth-submit-btn span').textContent = setup ? 'Инициализировать систему' : 'Войти в систему';
  $('#auth-form').dataset.setup = setup ? '1' : '0';
  $('#auth-user').value = setup ? 'admin' : '';
  $('#auth-pass').value = '';
}

function showApp() {
  $('#auth-screen').classList.add('hidden');
  $('#app').classList.remove('hidden');
  if (state.me?.user?.username) {
    $('#user-display').textContent = state.me.user.username;
  }
}

async function authSubmit(e) {
  e.preventDefault();
  const setup = e.currentTarget.dataset.setup === '1';
  const errBox = $('#auth-error');
  errBox.classList.add('hidden');
  try {
    const body = { username: $('#auth-user').value.trim(), password: $('#auth-pass').value };
    if (setup) {
      await api('/api/v1/setup/admin', { method: 'POST', body });
    }
    const r = await api('/api/v1/auth/login', { method: 'POST', body });
    state.csrf = r.csrf_token;
    state.me = { user: r.user };
    sessionStorage.setItem('sentinel_csrf', state.csrf);
    showApp();
    connectRealtime();
    location.hash = '#overview';
    renderRoute(true);
    toast('Вход успешно выполнен');
  } catch (err) {
    errBox.textContent = err.message;
    errBox.classList.remove('hidden');
  }
}

async function logout() {
  try { await api('/api/v1/auth/logout', { method: 'POST' }); } catch {}
  state.csrf = '';
  sessionStorage.removeItem('sentinel_csrf');
  location.reload();
}

function connectRealtime() {
  if (state.eventSource) state.eventSource.close();
  const es = new EventSource('/api/v1/events/stream');
  state.eventSource = es;
  es.onopen = () => $('#realtime-state').classList.add('connected');
  es.onerror = () => $('#realtime-state').classList.remove('connected');
  es.onmessage = () => {
    const route = getRoute().route;
    if (['overview', 'events', 'incidents', 'entrance', 'cameras'].includes(route)) {
      clearTimeout(state.rt);
      state.rt = setTimeout(() => renderRoute(true), 600);
    }
  };
}

function getRoute() {
  const raw = (location.hash || '#overview').slice(1);
  const [routeRaw, id] = raw.split('/');
  return { route: titles[routeRaw] ? routeRaw : 'overview', id };
}

async function renderRoute(silent = false) {
  clearTimers();
  const { route, id } = getRoute();
  $$('#nav a').forEach(a => a.classList.toggle('active', a.dataset.route === route));
  $('#page-title').textContent = titles[route];
  $('#sidebar').classList.remove('open');
  if (!silent) {
    $('#content').innerHTML = '<div class="empty-state">Загрузка данных…</div>';
  }
  try {
    const router = {
      overview: renderOverview,
      cameras: () => renderCameras(id),
      entrance: renderEntrance,
      events: renderEvents,
      incidents: () => renderIncidents(id),
      search: renderSearch,
      system: renderSystem,
      setup: renderSetup,
      settings: renderSettings
    };
    await (router[route] || renderOverview)();
  } catch (e) {
    $('#content').innerHTML = `
      <div class="card">
        <h2>Ошибка загрузки страницы</h2>
        <p class="error-banner" style="margin-top:12px;">${esc(e.message)}</p>
      </div>`;
  }
}

function setOverall(v) {
  const text = $('#overall-health-text');
  const dot = $('#status-dot');
  if (text) text.textContent = v || 'UNKNOWN';
  if (dot) dot.className = 'status-indicator-dot ' + String(v || 'unknown').toLowerCase();
}

/* ==========================================================================
   Page: Overview
   ========================================================================== */
async function renderOverview() {
  const d = await api('/api/v1/dashboard/overview');
  setOverall(d.health.status);
  state.cameras = d.cameras?.items || [];
  $('#nav-camera-count').textContent = d.cameras?.total || 0;

  const free = d.hardware.storage?.reduce((a, x) => a + Number(x.free || 0), 0) || 0;

  $('#content').innerHTML = `
    <div class="section-header">
      <div>
        <h1>Состояние системы</h1>
        <p class="muted">Контроль камер, входной зоны, инцидентов и архивного хранилища</p>
      </div>
      ${badge(d.health.status)}
    </div>

    <!-- Metric Cards -->
    <div class="grid grid-metrics">
      <div class="card metric-card">
        <div class="metric-header">
          <span class="metric-label">Камеры онлайн</span>
          ${badge(d.cameras.healthy > 0 ? 'healthy' : 'warn')}
        </div>
        <div class="metric-value">${d.cameras.healthy} <span class="muted" style="font-size:1.2rem; font-weight:400;">/ ${d.cameras.total}</span></div>
        <div class="muted text-sm">${d.cameras.total ? 'Все потоки активны' : 'Камеры не добавлены'}</div>
      </div>

      <div class="card metric-card">
        <div class="metric-header">
          <span class="metric-label">Инциденты</span>
          ${badge(d.incidents.length > 0 ? 'warn' : 'ok')}
        </div>
        <div class="metric-value">${d.incidents.length}</div>
        <div class="muted text-sm">за последние 24 часа</div>
      </div>

      <div class="card metric-card">
        <div class="metric-header">
          <span class="metric-label">Свободное место</span>
          <span class="badge ok">STORAGE</span>
        </div>
        <div class="metric-value">${bytes(free)}</div>
        <div class="muted text-sm">диски записи и системы</div>
      </div>

      <div class="card metric-card">
        <div class="metric-header">
          <span class="metric-label">Резервная копия</span>
          ${badge(d.backup.enabled ? 'healthy' : 'unknown')}
        </div>
        <div class="metric-value" style="font-size:1.4rem; padding-top:4px;">${d.backup.enabled ? 'Restic Active' : 'Отключен'}</div>
        <div class="muted text-sm">${esc(d.backup.latest?.value?.status || 'Готов к запуску')}</div>
      </div>
    </div>

    <!-- Cameras Quick Wall -->
    <div style="margin-top: 24px;">
      <div class="section-header" style="margin-bottom: 12px;">
        <h2>Видеонаблюдение</h2>
        <div style="display:flex; gap:10px;">
          <button class="btn btn-primary btn-sm" id="btn-add-cam-overview">＋ Добавить камеру</button>
          <a class="btn btn-ghost btn-sm" href="#cameras">Все камеры →</a>
        </div>
      </div>
      ${d.cameras.items?.length ? `
        <div class="grid grid-camera-wall">
          ${d.cameras.items.slice(0, 4).map(c => cameraCard(c)).join('')}
        </div>
      ` : `
        <div class="empty-state">
          <p>Камеры пока не добавлены.</p>
          <button class="btn btn-primary btn-sm" style="margin-top:12px;" onclick="$('#add-camera-dialog').showModal()">＋ Подключить USB / ONVIF / RTSP</button>
        </div>
      `}
    </div>

    <!-- Entrance & Incidents -->
    <div class="grid grid-two" style="margin-top: 24px;">
      <section class="card">
        <div class="card-header">
          <h2>Входная группа и домофон</h2>
          <a href="#entrance" class="text-sm muted" style="text-decoration:none;">Управление →</a>
        </div>
        ${entranceSummary(d.intercoms)}
      </section>

      <section class="card">
        <div class="card-header">
          <h2>Последние события и инциденты</h2>
          <a href="#incidents" class="text-sm muted" style="text-decoration:none;">Все →</a>
        </div>
        ${incidentRows(d.incidents)}
      </section>
    </div>

    <!-- System Diagnostics Strip -->
    <section class="card" style="margin-top: 24px;">
      <div class="card-header">
        <h2>Состояние компонентов инфраструктуры</h2>
        <a href="#system" class="text-sm muted" style="text-decoration:none;">Диагностика →</a>
      </div>
      <div class="list">
        ${d.health.components.map(x => `
          <div class="list-item">
            <div>
              <div class="list-item-title">${esc(x.component.name)}</div>
              <div class="list-item-sub">${esc(x.component.reason_code || x.component.cause || 'Нормальная работа')}</div>
            </div>
            ${badge(x.component.status)}
          </div>
        `).join('')}
      </div>
    </section>
  `;

  startSnapshots();
  const addBtn = $('#btn-add-cam-overview');
  if (addBtn) addBtn.onclick = () => $('#add-camera-dialog').showModal();
}

function incidentRows(items) {
  if (!items?.length) return '<div class="empty-state">Активных инцидентов нет</div>';
  return `
    <div class="list">
      ${items.map(r => {
        const i = r.value || r;
        return `
          <div class="list-item clickable" data-incident="${esc(i.id)}">
            <div>
              <div class="list-item-title">${esc(i.location || i.camera_id || 'Инцидент')}</div>
              <div class="list-item-sub">${fmtTime(i.opened_at)} &bull; ${i.event_ids?.length || 0} связанных событий</div>
            </div>
            ${badge(i.state)}
          </div>`;
      }).join('')}
    </div>`;
}

function entranceSummary(items) {
  if (!items?.length) return '<div class="empty-state">Домофон или контроллер двери не настроен</div>';
  return items.map(x => `
    <div class="door-state-grid">
      <div class="door-widget">
        <span class="metric-label">Устройство</span>
        <div class="door-widget-value">${esc(x.device.name)}</div>
      </div>
      <div class="door-widget">
        <span class="metric-label">Контакт двери</span>
        <div class="door-widget-value" style="color:${x.observed.door === 'OPEN' ? 'var(--status-warn)' : 'var(--status-healthy)'};">${esc(x.observed.door || 'CLOSED')}</div>
      </div>
      <div class="door-widget">
        <span class="metric-label">Электрозамок</span>
        <div class="door-widget-value" style="color:${x.observed.lock === 'UNLOCKED' ? 'var(--status-warn)' : 'var(--text-primary)'};">${esc(x.observed.lock || 'LOCKED')}</div>
      </div>
    </div>
    <div style="display:flex; justify-content:space-between; align-items:center; margin-top:14px;">
      <span class="muted text-sm">Связь: <strong>${x.observed.available ? 'ONLINE' : 'OFFLINE'}</strong></span>
      <a href="#entrance" class="btn btn-secondary btn-sm">Открыть панель входа →</a>
    </div>
  `).join('');
}

/* ==========================================================================
   Page: Cameras & Wall
   ========================================================================== */
async function renderCameras(id) {
  if (id) return renderCameraDetail(id);
  const d = await api('/api/v1/cameras');
  state.cameras = d.items || [];
  $('#nav-camera-count').textContent = d.items.length;

  $('#content').innerHTML = `
    <div class="section-header">
      <div>
        <h1>Камеры видеонаблюдения</h1>
        <p class="muted">Live Snapshot Wall автоматически обновляет превью всех камер</p>
      </div>
      <div style="display:flex; gap:10px;">
        <button class="btn btn-primary" id="btn-add-camera-main">＋ Добавить камеру</button>
      </div>
    </div>

    ${d.items.length ? `
      <div class="grid grid-camera-wall">
        ${d.items.map(c => cameraCard(c)).join('')}
      </div>
    ` : `
      <div class="empty-state" style="padding:60px 20px;">
        <h3>Камеры не подключены</h3>
        <p class="muted" style="margin: 8px 0 18px;">Подключите локальную USB веб-камеру, найдите ONVIF-камеры в локальной сети или добавьте RTSP поток вручную.</p>
        <button class="btn btn-primary" onclick="$('#add-camera-dialog').showModal()">＋ Добавить камеру</button>
      </div>
    `}
  `;

  $('#btn-add-camera-main')?.addEventListener('click', () => $('#add-camera-dialog').showModal());
  startSnapshots();
}

function cameraCard(c) {
  const stream = c.streams?.find(x => x.role === 'main') || c.streams?.[0];
  const resText = stream ? `${stream.width || 0}×${stream.height || 0}` : 'Live';
  const typeText = c.type?.toUpperCase() || 'CAM';
  return `
    <article class="card camera-card">
      <div class="camera-preview-wrap">
        <img class="camera-preview-img live-snapshot" data-camera="${esc(c.id)}" src="/api/v1/media/cameras/${encodeURIComponent(c.id)}/latest.jpg" alt="${esc(c.name)}">
        <div class="camera-overlay-top">
          <span class="tag-pill">${esc(typeText)}</span>
          ${badge(c.observed?.status || 'HEALTHY')}
        </div>
        <div class="camera-overlay-bottom">
          <span class="tag-pill">${esc(resText)}</span>
          <span class="tag-pill text-mono">${esc(c.streams?.[0]?.codec || 'MJPEG')}</span>
        </div>
      </div>
      <div class="camera-info">
        <div>
          <a class="camera-title-link" href="#cameras/${esc(c.id)}">${esc(c.name)}</a>
          <div class="muted text-xs" style="margin-top:2px;">${esc(c.manufacturer || c.type)} ${esc(c.model || '')}</div>
        </div>
        <a class="btn btn-ghost btn-sm" href="#cameras/${esc(c.id)}">Просмотр</a>
      </div>
    </article>`;
}

function startSnapshots() {
  const imgs = $$('.live-snapshot');
  const update = () => {
    if (document.visibilityState === 'visible') {
      const now = Date.now();
      imgs.forEach(img => {
        const camId = img.dataset.camera;
        if (camId) {
          img.src = `/api/v1/media/cameras/${encodeURIComponent(camId)}/latest.jpg?t=${now}`;
        }
      });
    }
  };
  state.timers.push(setInterval(update, 1600));
}

async function renderCameraDetail(id) {
  const [c, live] = await Promise.all([
    api(`/api/v1/cameras/${encodeURIComponent(id)}`),
    api(`/api/v1/cameras/${encodeURIComponent(id)}/live`)
  ]);

  $('#content').innerHTML = `
    <div class="section-header">
      <div>
        <a href="#cameras" class="muted text-sm" style="text-decoration:none;">← Назад к списку камер</a>
        <h1 style="margin-top:4px;">${esc(c.name)}</h1>
        <p class="muted">${esc(c.manufacturer || c.type)} ${esc(c.model || '')} &bull; ID: <span class="text-mono">${esc(c.id)}</span></p>
      </div>
      ${badge(c.observed.status)}
    </div>

    <div class="grid grid-two">
      <section class="card" style="padding:16px;">
        <div id="live-stream-box">
          ${live.mode === 'go2rtc' ? `
            <iframe class="stream-frame" allow="autoplay; microphone" src="${esc(live.viewer_url)}"></iframe>
          ` : `
            <img class="stream-frame live-snapshot" data-camera="${esc(c.id)}" src="/api/v1/media/cameras/${encodeURIComponent(c.id)}/latest.jpg" style="object-fit:cover;">
          `}
        </div>

        <div style="display:flex; justify-content:space-between; align-items:center; margin-top:14px;">
          <div style="display:flex; gap:8px;">
            <button id="btn-diag-cam" class="btn btn-secondary btn-sm">Запустить диагностику</button>
            ${live.talk_url ? '<button id="btn-talk-cam" class="btn btn-secondary btn-sm">Включить микрофон</button>' : ''}
          </div>
          <a class="btn btn-ghost btn-sm" href="#events">Журнал событий</a>
        </div>
        <div id="camera-diag-result" style="margin-top:14px;"></div>
      </section>

      <section class="card">
        <h2>Свойства и потоки</h2>
        <div style="display:flex; flex-direction:column; gap:12px; margin-top:14px;">
          <div class="list-item">
            <span class="muted">Тип источника</span>
            <strong>${esc(c.type?.toUpperCase())}</strong>
          </div>
          <div class="list-item">
            <span class="muted">Endpoint URL</span>
            <span class="text-mono text-sm" style="max-width:200px; overflow:hidden; text-overflow:ellipsis;">${esc(c.streams?.[0]?.endpoint?.url || '—')}</span>
          </div>
          <div class="list-item">
            <span class="muted">Возможности</span>
            <span>${[c.capabilities?.snapshot ? 'Снимок' : '', c.capabilities?.audio ? 'Аудио' : '', c.capabilities?.talk ? 'Двусторонняя связь' : '', c.capabilities?.ptz ? 'PTZ' : ''].filter(Boolean).join(' &bull; ') || 'Базовое видео'}</span>
          </div>
        </div>

        <h3 style="margin-top:20px;">Конфигурация потоков</h3>
        <div class="list" style="margin-top:10px;">
          ${c.streams.map(s => `
            <div class="list-item">
              <div>
                <div class="list-item-title">${esc(s.role?.toUpperCase())} Stream</div>
                <div class="list-item-sub text-mono">${esc(s.codec || 'mjpeg')} &bull; ${s.width || 0}×${s.height || 0} @ ${Number(s.fps || 30).toFixed(0)} fps</div>
              </div>
              <span class="tag-pill text-mono">${esc(s.role)}</span>
            </div>
          `).join('')}
        </div>
      </section>
    </div>
  `;

  if (live.mode !== 'go2rtc') startSnapshots();

  $('#btn-diag-cam').onclick = async () => {
    const box = $('#camera-diag-result');
    box.innerHTML = '<div class="empty-state">Тестирование потока и времени отклика…</div>';
    try {
      const d = await api(`/api/v1/cameras/${encodeURIComponent(id)}/diagnostics`);
      box.innerHTML = `
        <div class="list" style="margin-top:8px;">
          ${d.checks.map(x => `
            <div class="list-item">
              <div>
                <div class="list-item-title">${esc(x.name)}</div>
                <div class="list-item-sub">${esc(x.detail || '')} ${x.duration_ms ? `&bull; ${x.duration_ms} мс` : ''}</div>
              </div>
              <span class="badge ${x.status === 'OK' ? 'healthy' : 'failed'}">${esc(x.status)}</span>
            </div>
          `).join('')}
        </div>`;
    } catch (e) {
      box.innerHTML = `<p class="error-banner">${esc(e.message)}</p>`;
    }
  };
}

/* ==========================================================================
   Add Camera Modal & Scanner (USB / ONVIF / RTSP)
   ========================================================================== */
function setupAddCameraModal() {
  const dlg = $('#add-camera-dialog');
  if (!dlg) return;

  $('#add-cam-close').onclick = () => dlg.close();

  // Tabs
  $$('.tab-btn').forEach(btn => {
    btn.onclick = () => {
      $$('.tab-btn').forEach(b => b.classList.remove('active'));
      $$('.tab-panel').forEach(p => p.classList.remove('active'));
      btn.classList.add('active');
      $(`#tab-content-${btn.dataset.tab}`).classList.add('active');
    };
  });

  // UVC / USB Scanner
  const btnScanUVC = $('#btn-scan-uvc');
  const uvcResults = $('#uvc-scan-results');
  const uvcForm = $('#form-add-uvc');

  btnScanUVC.onclick = async () => {
    btnScanUVC.disabled = true;
    uvcResults.innerHTML = '<div class="empty-state">Сканирование подключенных USB-видеоустройств…</div>';
    try {
      const res = await api('/api/v1/cameras/discover/uvc');
      if (!res.items || res.items.length === 0) {
        uvcResults.innerHTML = '<div class="empty-state">USB-камеры не обнаружены. Убедитесь, что камера подключена к серверу.</div>';
        return;
      }
      uvcResults.innerHTML = res.items.map(dev => `
        <div class="device-item" data-path="${esc(dev.path)}" data-name="${esc(dev.name || dev.path)}">
          <div>
            <strong>${esc(dev.name || dev.path)}</strong>
            <div class="muted text-xs text-mono" style="margin-top:2px;">${esc(dev.path)}</div>
            ${dev.modes?.length ? `
              <div class="muted text-xs" style="margin-top:4px;">Режимы: ${dev.modes.slice(0, 3).map(m => `${m.width}×${m.height} @ ${m.fps}fps`).join(', ')}</div>
            ` : ''}
          </div>
          <button class="btn btn-secondary btn-sm select-uvc-btn" type="button">Выбрать</button>
        </div>
      `).join('');

      $$('.device-item').forEach(el => {
        el.onclick = () => {
          $$('.device-item').forEach(x => x.classList.remove('selected'));
          el.classList.add('selected');
          $('#uvc-cam-name').value = el.dataset.name;
          $('#uvc-cam-path').value = el.dataset.path;
          uvcForm.classList.remove('hidden');
          testUVCSnapshot(el.dataset.path);
        };
      });
    } catch (e) {
      uvcResults.innerHTML = `<p class="error-banner">${esc(e.message)}</p>`;
    } finally {
      btnScanUVC.disabled = false;
    }
  };

  async function testUVCSnapshot(path) {
    const previewBox = $('#uvc-preview-container');
    const previewImg = $('#uvc-test-snapshot');
    const badgeEl = $('#uvc-snap-badge');
    previewBox.classList.remove('hidden');
    badgeEl.textContent = 'CHECKING...';
    badgeEl.className = 'badge starting';
    previewImg.src = '';
    try {
      badgeEl.textContent = 'READY';
      badgeEl.className = 'badge ok';
    } catch {
      badgeEl.textContent = 'WARNING';
      badgeEl.className = 'badge warn';
    }
  }

  $('#btn-test-uvc-snap').onclick = () => {
    const path = $('#uvc-cam-path').value;
    if (path) testUVCSnapshot(path);
  };

  uvcForm.onsubmit = async e => {
    e.preventDefault();
    try {
      const name = $('#uvc-cam-name').value.trim();
      const path = $('#uvc-cam-path').value.trim();
      const cam = await apiFresh('/api/v1/cameras/onboard/uvc', {
        method: 'POST',
        body: { name, path, role: 'main' }
      });
      toast(`USB-камера «${cam.name}» успешно добавлена!`);
      dlg.close();
      uvcForm.reset();
      uvcForm.classList.add('hidden');
      location.hash = '#cameras';
      renderRoute(true);
    } catch (err) {
      toast(err.message, true);
    }
  };

  // ONVIF Scanner
  const btnScanONVIF = $('#btn-scan-onvif');
  const onvifResults = $('#onvif-scan-results');
  const onvifForm = $('#form-add-onvif');

  btnScanONVIF.onclick = async () => {
    btnScanONVIF.disabled = true;
    onvifResults.innerHTML = '<div class="empty-state">Поиск ONVIF-устройств в локальной сети (WS-Discovery)…</div>';
    try {
      const res = await api('/api/v1/cameras/discover/onvif?duration=3s');
      if (!res.items || res.items.length === 0) {
        onvifResults.innerHTML = '<div class="empty-state">ONVIF-камеры не найдены. Укажите параметры вручную во вкладке RTSP.</div>';
        return;
      }
      onvifResults.innerHTML = res.items.map(dev => `
        <div class="device-item" data-url="${esc(dev.xaddr || dev.service_url)}" data-name="${esc(dev.name || dev.model || 'ONVIF Камера')}">
          <div>
            <strong>${esc(dev.name || dev.model || 'IP Камера')}</strong>
            <div class="muted text-xs text-mono" style="margin-top:2px;">${esc(dev.xaddr || dev.service_url)}</div>
          </div>
          <button class="btn btn-secondary btn-sm" type="button">Выбрать</button>
        </div>
      `).join('');

      $$('#onvif-scan-results .device-item').forEach(el => {
        el.onclick = () => {
          $('#onvif-cam-name').value = el.dataset.name;
          $('#onvif-cam-url').value = el.dataset.url;
          onvifForm.classList.remove('hidden');
        };
      });
    } catch (e) {
      onvifResults.innerHTML = `<p class="error-banner">${esc(e.message)}</p>`;
    } finally {
      btnScanONVIF.disabled = false;
    }
  };

  onvifForm.onsubmit = async e => {
    e.preventDefault();
    try {
      const name = $('#onvif-cam-name').value.trim();
      const device_url = $('#onvif-cam-url').value.trim();
      const username = $('#onvif-cam-user').value.trim();
      const password = $('#onvif-cam-pass').value;
      const cam = await apiFresh('/api/v1/cameras/onboard/onvif', {
        method: 'POST',
        body: { name, device_url, username, password_ref: password ? `env:${password}` : '' }
      });
      toast(`Камера «${cam.name}» подключена!`);
      dlg.close();
      location.hash = '#cameras';
      renderRoute(true);
    } catch (err) {
      toast(err.message, true);
    }
  };

  // RTSP Manual Form
  $('#form-add-rtsp').onsubmit = async e => {
    e.preventDefault();
    try {
      const name = $('#rtsp-cam-name').value.trim();
      const url = $('#rtsp-cam-url').value.trim();
      const username = $('#rtsp-cam-user').value.trim();
      const password = $('#rtsp-cam-pass').value;
      const role = $('#rtsp-cam-role').value;
      const cam = await apiFresh('/api/v1/cameras/onboard/rtsp', {
        method: 'POST',
        body: { name, url, username, password_ref: password ? `env:${password}` : '', role }
      });
      toast(`RTSP поток «${cam.name}» добавлен!`);
      dlg.close();
      location.hash = '#cameras';
      renderRoute(true);
    } catch (err) {
      toast(err.message, true);
    }
  };
}

/* ==========================================================================
   Page: Entrance & Intercom
   ========================================================================== */
async function renderEntrance() {
  const d = await api('/api/v1/intercoms');
  if (!d.items?.length) {
    $('#content').innerHTML = `
      <div class="section-header">
        <div><h1>Контроль входа</h1><p class="muted">Управление замками и вызывными панелями</p></div>
      </div>
      <div class="empty-state">
        <h3>Домофон не настроен</h3>
        <p class="muted" style="margin-top:6px;">Привяжите контроллер двери или ESP32-домофон в настройках MQTT.</p>
      </div>`;
    return;
  }

  const dev = d.items[0];
  const full = await api(`/api/v1/intercoms/${encodeURIComponent(dev.id)}`);
  let live = null;
  if (dev.camera_id) {
    try { live = await api(`/api/v1/cameras/${encodeURIComponent(dev.camera_id)}/live`); } catch {}
  }

  $('#content').innerHTML = `
    <div class="section-header">
      <div>
        <h1>${esc(dev.name)}</h1>
        <p class="muted">${esc(dev.location || 'Главный вход')} &bull; Контроллер доступа</p>
      </div>
      ${badge(full.observed.available ? 'HEALTHY' : 'DEGRADED')}
    </div>

    <div class="grid grid-two">
      <section class="card" style="padding:16px;">
        ${live ? (live.mode === 'go2rtc' ? `
          <iframe class="stream-frame" allow="autoplay; microphone" src="${esc(live.viewer_url)}"></iframe>
        ` : `
          <img class="stream-frame live-snapshot" data-camera="${esc(dev.camera_id)}" src="/api/v1/media/cameras/${encodeURIComponent(dev.camera_id)}/latest.jpg" style="object-fit:cover;">
        `) : `
          <div class="empty-state" style="aspect-ratio:16/9; display:grid; place-items:center;">Камера входа не привязана</div>
        `}

        <div class="unlock-action-panel">
          <div>
            <strong>Электрозамок двери</strong>
            <p class="muted text-xs">Безопасное открытие с защитой от случайного нажатия</p>
          </div>
          <button id="btn-unlock-door" class="btn btn-danger">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 9.9-1"/></svg>
            <span>Открыть замок</span>
          </button>
        </div>
      </section>

      <section class="card">
        <h2>Состояние датчиков входа</h2>
        <div class="door-state-grid">
          <div class="door-widget">
            <span class="metric-label">Связь</span>
            <div class="door-widget-value" style="color:${full.observed.available ? 'var(--status-healthy)' : 'var(--status-danger)'};">${full.observed.available ? 'ONLINE' : 'OFFLINE'}</div>
          </div>
          <div class="door-widget">
            <span class="metric-label">Дверь</span>
            <div class="door-widget-value" style="color:${full.observed.door === 'OPEN' ? 'var(--status-warn)' : 'var(--status-healthy)'};">${esc(full.observed.door || 'CLOSED')}</div>
          </div>
          <div class="door-widget">
            <span class="metric-label">Замок</span>
            <div class="door-widget-value">${esc(full.observed.lock || 'LOCKED')}</div>
          </div>
        </div>

        <div style="display:flex; flex-direction:column; gap:10px; margin-top:20px;">
          <div class="list-item">
            <span class="muted">Последний сигнал</span>
            <span class="text-mono text-sm">${fmtTime(full.observed.last_seen_at)}</span>
          </div>
          <div class="list-item">
            <span class="muted">Привязанная камера</span>
            <span>${esc(dev.camera_id || 'Не привязана')}</span>
          </div>
        </div>
      </section>
    </div>
  `;

  if (live && live.mode !== 'go2rtc') startSnapshots();

  $('#btn-unlock-door')?.addEventListener('click', async () => {
    if (!confirm('Подтвердите открытие входной двери')) return;
    try {
      await apiFresh(`/api/v1/intercoms/${encodeURIComponent(dev.id)}/unlock`, {
        method: 'POST',
        body: { confirm: true, ttl_seconds: 5 }
      });
      toast('Команда открытия замка отправлена');
    } catch (e) {
      toast(e.message, true);
    }
  });
}

function reauth() {
  return new Promise(resolve => {
    const dlg = $('#reauth-dialog');
    const form = $('#reauth-form');
    $('#reauth-password').value = '';
    $('#reauth-error').classList.add('hidden');
    dlg.showModal();

    $('#reauth-cancel-btn').onclick = () => {
      dlg.close();
      resolve(false);
    };

    form.onsubmit = async e => {
      e.preventDefault();
      try {
        await api('/api/v1/auth/reauth', {
          method: 'POST',
          body: { password: $('#reauth-password').value }
        });
        dlg.close();
        resolve(true);
      } catch (err) {
        const errEl = $('#reauth-error');
        errEl.textContent = err.message;
        errEl.classList.remove('hidden');
      }
    };
  });
}

/* ==========================================================================
   Page: Events & Incidents
   ========================================================================== */
async function renderEvents() {
  const d = await api('/api/v1/events?limit=100');
  $('#content').innerHTML = `
    <div class="section-header">
      <div>
        <h1>Журнал событий безопасности</h1>
        <p class="muted">Нормализованный аудит событий Frigate, домофона и ядра системы</p>
      </div>
    </div>

    <div class="card" style="margin-bottom:16px;">
      <div style="display:flex; gap:12px; flex-wrap:wrap;">
        <input id="event-q" placeholder="Поиск по тексту или источнику…" style="flex:1; min-width:220px;">
        <select id="event-sev" style="width:180px;">
          <option value="">Все уровни важности</option>
          <option value="INFO">INFO (Обычные)</option>
          <option value="WARNING">WARNING (Внимание)</option>
          <option value="CRITICAL">CRITICAL (Тревога)</option>
        </select>
      </div>
    </div>

    <section class="card">
      <div id="event-list">${eventRows(d.items)}</div>
    </section>
  `;

  const reload = async () => {
    const q = encodeURIComponent($('#event-q').value);
    const sev = encodeURIComponent($('#event-sev').value);
    const x = await api(`/api/v1/events?limit=100&q=${q}&severity=${sev}`);
    $('#event-list').innerHTML = eventRows(x.items);
  };

  $('#event-q').oninput = debounce(reload, 300);
  $('#event-sev').onchange = reload;
}

function eventRows(items) {
  if (!items?.length) return '<div class="empty-state">Событий не найдено</div>';
  return `
    <div class="list">
      ${items.map(r => {
        const e = r.value || r;
        return `
          <div class="list-item">
            <div>
              <div class="list-item-title">${esc(e.type)}</div>
              <div class="list-item-sub text-mono">${fmtTime(e.occurred_at)} &bull; ${esc(e.source)}</div>
            </div>
            ${badge(e.severity)}
          </div>`;
      }).join('')}
    </div>`;
}

async function renderIncidents(id) {
  if (id) return renderIncidentDetail(id);
  const d = await api('/api/v1/incidents');
  $('#content').innerHTML = `
    <div class="section-header">
      <div>
        <h1>Инциденты и Тревоги</h1>
        <p class="muted">Коррелированные цепочки событий в единые сценарии</p>
      </div>
    </div>
    <section class="card">${incidentRows(d.items)}</section>
  `;

  $$('[data-incident]').forEach(x => {
    x.onclick = () => { location.hash = `#incidents/${x.dataset.incident}`; };
  });
}

async function renderIncidentDetail(id) {
  const d = await api(`/api/v1/incidents/${encodeURIComponent(id)}`);
  const i = d.incident.value || d.incident;
  const events = d.events.map(x => x.value || x).sort((a, b) => new Date(a.occurred_at) - new Date(b.occurred_at));

  $('#content').innerHTML = `
    <div class="section-header">
      <div>
        <a href="#incidents" class="muted text-sm" style="text-decoration:none;">← Назад к инцидентам</a>
        <h1 style="margin-top:4px;">${esc(i.location || 'Инцидент')}</h1>
        <p class="muted text-mono">${esc(i.id)} &bull; ${fmtTime(i.opened_at)}</p>
      </div>
      ${badge(i.state)}
    </div>

    <div class="grid grid-two">
      <section class="card">
        <h2>Хронология событий (Timeline)</h2>
        <div class="timeline" style="margin-top:16px;">
          ${events.map(e => `
            <div class="timeline-event">
              <div class="timeline-time">${fmtTime(e.occurred_at)}</div>
              <div class="list-item-title">${esc(e.type)}</div>
              <div class="list-item-sub">${esc(e.source)} &bull; ${esc(e.severity)}</div>
            </div>
          `).join('') || '<div class="empty-state">Нет связанных событий</div>'}
        </div>
      </section>

      <section class="card">
        <h2>Контекст и детали</h2>
        <div style="display:flex; flex-direction:column; gap:12px; margin-top:14px;">
          <div class="list-item"><span class="muted">Камера</span><strong>${esc(i.camera_id || '—')}</strong></div>
          <div class="list-item"><span class="muted">Объекты</span><span>${esc((i.object_ids || []).join(', ') || '—')}</span></div>
          <div class="list-item"><span class="muted">Correlation ID</span><span class="text-mono text-xs">${esc((i.correlation_ids || []).join(', ') || '—')}</span></div>
          <div class="list-item"><span class="muted">Последнее обновление</span><span class="text-mono text-sm">${fmtTime(i.last_event_at)}</span></div>
        </div>
      </section>
    </div>
  `;
}

/* ==========================================================================
   Page: Search
   ========================================================================== */
async function renderSearch() {
  $('#content').innerHTML = `
    <div class="section-header">
      <div>
        <h1>Универсальный поиск</h1>
        <p class="muted">Мгновенный поиск по камерам, точкам входа, инцидентам и метаданным</p>
      </div>
    </div>

    <div class="card">
      <input id="search-q" placeholder="Введите поисковый запрос (например: вход, LifeCam, person, alert)…" autofocus style="font-size:1.05rem; padding:12px 16px;">
    </div>

    <div id="search-results" style="margin-top:20px;"></div>
  `;

  const run = async () => {
    const q = $('#search-q').value.trim();
    const box = $('#search-results');
    if (!q) {
      box.innerHTML = '';
      return;
    }
    const d = await api(`/api/v1/search?q=${encodeURIComponent(q)}`);
    if (!d.items?.length) {
      box.innerHTML = '<div class="empty-state">Ничего не найдено по вашему запросу</div>';
      return;
    }
    box.innerHTML = `
      <section class="card">
        <div class="list">
          ${d.items.map(x => `
            <div class="list-item clickable" data-kind="${esc(x.kind)}" data-id="${esc(x.id)}">
              <div>
                <div class="list-item-title">${esc(x.title)}</div>
                <div class="list-item-sub">${esc(x.kind?.toUpperCase())} &bull; ${esc(x.subtitle || '')} ${x.occurred_at ? `&bull; ${fmtTime(x.occurred_at)}` : ''}</div>
              </div>
              <span class="tag-pill text-mono">${esc(x.kind)}</span>
            </div>
          `).join('')}
        </div>
      </section>`;

    $$('[data-kind]').forEach(el => {
      el.onclick = () => {
        if (el.dataset.kind === 'camera') location.hash = `#cameras/${el.dataset.id}`;
        else if (el.dataset.kind === 'incident') location.hash = `#incidents/${el.dataset.id}`;
        else if (el.dataset.kind === 'intercom') location.hash = '#entrance';
      };
    });
  };

  $('#search-q').oninput = debounce(run, 250);
}

/* ==========================================================================
   Page: System
   ========================================================================== */
async function renderSystem() {
  const d = await api('/api/v1/system/diagnostics');
  setOverall(d.status);

  $('#content').innerHTML = `
    <div class="section-header">
      <div>
        <h1>Состояние оборудования и сервисов</h1>
        <p class="muted">Аппаратный профиль сервера и граф зависимостей системы</p>
      </div>
      ${badge(d.status)}
    </div>

    <div class="grid grid-two">
      <section class="card">
        <h2>Компоненты инфраструктуры</h2>
        <div class="list" style="margin-top:14px;">
          ${d.components.map(x => `
            <div class="list-item">
              <div>
                <div class="list-item-title">
                  <span class="status-indicator-dot ${String(x.component.status).toLowerCase()}"></span>
                  ${esc(x.component.name)}
                </div>
                <div class="list-item-sub">${esc(x.suppressed_by ? `Зависит от сбоя: ${x.suppressed_by}` : (x.component.reason_code || x.component.cause || 'Нормальная работа'))}</div>
              </div>
              ${badge(x.component.status)}
            </div>
          `).join('')}
        </div>
      </section>

      <section class="card">
        <h2>Сервер и аппаратные ресурсы</h2>
        <div style="display:flex; flex-direction:column; gap:10px; margin-top:14px;">
          <div class="list-item"><span class="muted">ОС</span><strong>${esc(d.hardware.os.pretty_name || d.hardware.os.id || 'Windows / Linux')}</strong></div>
          <div class="list-item"><span class="muted">Процессор</span><span>${esc(d.hardware.cpu.model || 'CPU')} (${d.hardware.cpu.logical_cores || 4} потоков)</span></div>
          <div class="list-item"><span class="muted">Память RAM</span><span>${bytes(d.hardware.memory.available)} свободно / ${bytes(d.hardware.memory.total)}</span></div>
          <div class="list-item"><span class="muted">Аппаратный декодер</span><span class="tag-pill text-mono">${esc(d.recommendation.video_decoder || 'CPU / VAAPI')}</span></div>
          <div class="list-item"><span class="muted">AI профиль</span><span class="tag-pill text-mono">${esc(d.recommendation.ai_profile || 'OFF')}</span></div>
        </div>

        <h3 style="margin-top:20px;">Дисковые хранилища</h3>
        <div class="list" style="margin-top:10px;">
          ${(d.hardware.storage || []).map(x => {
            const used = x.total ? Math.round((1 - x.free / x.total) * 100) : 0;
            return `
              <div>
                <div style="display:flex; justify-content:space-between; font-size:0.8125rem;">
                  <strong>${esc(x.mount_point || x.device)}</strong>
                  <span class="muted">${bytes(x.free)} свободно из ${bytes(x.total)}</span>
                </div>
                <div class="progress-bar"><span style="width:${used}%"></span></div>
              </div>`;
          }).join('')}
        </div>
      </section>
    </div>
  `;
}

/* ==========================================================================
   Page: Setup Wizard
   ========================================================================== */
async function renderSetup() {
  const d = await api('/api/v1/setup/wizard');
  const pct = d.required_count ? Math.min(100, Math.round(d.ready_count / d.required_count * 100)) : 100;

  $('#content').innerHTML = `
    <div class="section-header">
      <div>
        <h1>Мастер готовности системы</h1>
        <p class="muted">Проверяет реальную готовность подсистем к эксплуатации</p>
      </div>
      ${badge(d.complete ? 'HEALTHY' : 'STARTING')}
    </div>

    <section class="card">
      <div style="display:flex; justify-content:space-between; align-items:center;">
        <div>
          <span class="metric-label">Выполнение обязательных шагов</span>
          <div class="metric-value" style="font-size:1.6rem; margin-top:4px;">${d.ready_count} / ${d.required_count}</div>
        </div>
        <button class="btn btn-primary btn-sm" id="btn-run-verify">Проверить готовность</button>
      </div>
      <div class="progress-bar" style="margin-top:14px;"><span style="width:${pct}%"></span></div>
      <div id="verify-run-results" style="margin-top:16px;"></div>
    </section>

    <section class="card" style="margin-top:20px;">
      <h2>Контрольный список</h2>
      <div class="list" style="margin-top:14px;">
        ${d.steps.map(x => `
          <div class="list-item">
            <div>
              <div class="list-item-title">${esc(x.title)} ${x.required ? '<span class="tag-pill">Обязательно</span>' : ''}</div>
              <div class="list-item-sub">${esc(x.reason || 'Проверка успешно пройдена')}</div>
            </div>
            <div style="display:flex; align-items:center; gap:10px;">
              <span class="badge ${String(x.status || 'pending').toLowerCase()}">${esc(x.status)}</span>
              <a class="btn btn-ghost btn-sm" href="${esc(x.actionRoute || '#settings')}">Открыть</a>
            </div>
          </div>
        `).join('')}
      </div>
    </section>
  `;

  $('#btn-run-verify').onclick = async () => {
    const box = $('#verify-run-results');
    box.innerHTML = '<div class="empty-state">Тестирование компонентов…</div>';
    try {
      const v = await api('/api/v1/setup/verify', { method: 'POST' });
      box.innerHTML = `
        <div class="list">
          ${v.checks.map(x => `
            <div class="list-item">
              <div>
                <div class="list-item-title">${esc(x.name)}</div>
                <div class="list-item-sub">${esc(x.detail || '')}</div>
              </div>
              <span class="badge ${String(x.status).toLowerCase()}">${esc(x.status)}</span>
            </div>
          `).join('')}
        </div>`;
    } catch (e) {
      box.innerHTML = `<p class="error-banner">${esc(e.message)}</p>`;
    }
  };
}

/* ==========================================================================
   Page: Settings
   ========================================================================== */
async function renderSettings() {
  const calls = await Promise.allSettled([
    api('/api/v1/setup/homeassistant'),
    api('/api/v1/frigate/status'),
    api('/api/v1/ai/status'),
    api('/api/v1/telegram/status'),
    api('/api/v1/backups/status'),
    api('/api/v1/users')
  ]);

  const val = i => calls[i].status === 'fulfilled' ? calls[i].value : null;
  const ha = val(0) || {};
  const fr = val(1);
  const ai = val(2);
  const tg = val(3);
  const bk = val(4);
  const users = val(5)?.items || [];

  $('#content').innerHTML = `
    <div class="section-header">
      <div>
        <h1>Настройки и Интеграции</h1>
        <p class="muted">Управление подключенными модулями без ручного редактирования файлов</p>
      </div>
    </div>

    <div class="grid grid-two">
      <!-- Home Assistant -->
      <section class="card">
        <div class="card-header">
          <h2>Home Assistant</h2>
          ${badge(ha.configured ? 'HEALTHY' : 'UNKNOWN')}
        </div>
        <div class="form-group">
          <label for="ha-url">URL инстанса Home Assistant</label>
          <input id="ha-url" value="${esc(ha.url || '')}" placeholder="http://homeassistant:8123">
        </div>
        <div class="form-group" style="margin-top:12px;">
          <label for="ha-token">Долгоживущий токен доступа (Bearer Token)</label>
          <input id="ha-token" type="password" placeholder="${ha.token_configured ? 'Токен сохранен. Введите новый для замены' : 'Введите токен'}">
        </div>
        <div style="display:flex; gap:10px; margin-top:16px;">
          <button id="ha-probe" class="btn btn-secondary btn-sm">Проверить связь</button>
          <button id="ha-save" class="btn btn-primary btn-sm">Сохранить</button>
        </div>
        <div id="ha-result" style="margin-top:12px;"></div>
      </section>

      <!-- Frigate NVR -->
      <section class="card">
        <div class="card-header">
          <h2>Frigate NVR & go2rtc</h2>
          ${badge(fr ? 'HEALTHY' : 'UNKNOWN')}
        </div>
        <p class="muted text-sm">Управление потоками go2rtc, генерацией конфигурации и зонами детекции.</p>
        <div style="display:flex; gap:10px; margin-top:16px;">
          <button id="frigate-plan" class="btn btn-secondary btn-sm">Проверить план</button>
          <button id="frigate-apply" class="btn btn-primary btn-sm">Применить план</button>
        </div>
        <div id="frigate-result" style="margin-top:12px;"></div>
      </section>

      <!-- Telegram Bot -->
      <section class="card">
        <div class="card-header">
          <h2>Telegram Уведомления</h2>
          ${badge(tg?.enabled ? 'HEALTHY' : 'UNKNOWN')}
        </div>
        <p class="muted text-sm">${tg?.enabled ? (tg.reachable ? 'Бот активен и готов к отправке' : 'Бот включен, проверка соединения…') : 'Интеграция Telegram отключена в конфигурации.'}</p>
        ${tg?.enabled ? '<button id="telegram-pair" class="btn btn-secondary btn-sm" style="margin-top:14px;">Сгенерировать код привязки</button>' : ''}
        <div id="telegram-result" style="margin-top:12px;"></div>
      </section>

      <!-- Local AI -->
      <section class="card">
        <div class="card-header">
          <h2>Локальный AI (Ollama VLM)</h2>
          ${badge(ai?.enabled ? 'HEALTHY' : 'UNKNOWN')}
        </div>
        <p class="muted text-sm">${ai?.enabled ? `Профиль: ${esc(ai.runtime_profile || '—')} &bull; Очередь: ${ai.queue_depth || 0}` : 'AI отключен. Базовая запись видео и детекция работают независимо.'}</p>
        ${ai?.enabled ? '<button id="ai-models" class="btn btn-secondary btn-sm" style="margin-top:14px;">Список установленных моделей</button>' : ''}
        <div id="ai-result" style="margin-top:12px;"></div>
      </section>

      <!-- Backup -->
      <section class="card">
        <div class="card-header">
          <h2>Резервное копирование (Restic)</h2>
          ${badge(bk?.enabled ? 'HEALTHY' : 'UNKNOWN')}
        </div>
        <p class="muted text-sm">${bk?.enabled ? `Репозиторий: ${esc(bk.repository || '—')} &bull; Интервал: ${esc(bk.interval || '6h')}` : 'Бэкапы отключены.'}</p>
        ${bk?.enabled ? `
          <div style="display:flex; gap:10px; margin-top:14px;">
            <button id="backup-run" class="btn btn-primary btn-sm">Запустить Backup</button>
            <button id="backup-check" class="btn btn-secondary btn-sm">Проверить репозиторий</button>
          </div>
        ` : ''}
        <div id="backup-result" style="margin-top:12px;"></div>
      </section>

      <!-- Security -->
      <section class="card">
        <div class="card-header">
          <h2>Безопасность и RBAC</h2>
          <span class="badge ok">ENCRYPTED</span>
        </div>
        <p class="muted text-sm">Все пароли хранятся в защищенном SecretRef хранилище. Для чувствительных действий (открытие двери, сброс настроек) используется подтверждение пароля.</p>
      </section>
    </div>

    <!-- User Management -->
    ${users.length ? `
      <section class="card" style="margin-top:24px;">
        <div class="card-header">
          <h2>Пользователи системы</h2>
        </div>
        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>Логин</th>
                <th>Роль доступа</th>
                <th>Состояние</th>
                <th>Действия</th>
              </tr>
            </thead>
            <tbody>
              ${users.map(u => `
                <tr data-user="${esc(u.id)}">
                  <td><strong>${esc(u.username)}</strong>${state.me?.user?.id === u.id ? ' <span class="tag-pill">Вы</span>' : ''}</td>
                  <td>
                    <select class="user-role" style="width:140px;">
                      <option value="viewer" ${u.role === 'viewer' ? 'selected' : ''}>viewer (Наблюдатель)</option>
                      <option value="operator" ${u.role === 'operator' ? 'selected' : ''}>operator (Оператор)</option>
                      <option value="admin" ${u.role === 'admin' ? 'selected' : ''}>admin (Администратор)</option>
                    </select>
                  </td>
                  <td>
                    <label style="display:inline-flex; align-items:center; gap:6px; font-size:0.8125rem;">
                      <input class="user-disabled" type="checkbox" ${u.disabled ? 'checked' : ''}> Отключен
                    </label>
                  </td>
                  <td>
                    <button class="btn btn-secondary btn-sm save-user" type="button">Сохранить</button>
                  </td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>
      </section>
    ` : ''}
  `;

  // Settings Actions
  $('#ha-probe')?.addEventListener('click', async () => {
    try {
      const d = await api('/api/v1/setup/homeassistant/probe', {
        method: 'POST',
        body: { url: $('#ha-url').value, token: $('#ha-token').value }
      });
      $('#ha-result').innerHTML = '<span class="badge ok">Соединение успешно установлено</span>';
      toast('Home Assistant отвечает');
    } catch (e) {
      $('#ha-result').innerHTML = `<p class="error-banner">${esc(e.message)}</p>`;
    }
  });

  $('#ha-save')?.addEventListener('click', async () => {
    try {
      const token = $('#ha-token').value;
      if (!token) throw new Error('Для сохранения введите токен');
      await apiFresh('/api/v1/setup/homeassistant', {
        method: 'POST',
        body: { url: $('#ha-url').value, token }
      });
      $('#ha-token').value = '';
      $('#ha-result').innerHTML = '<span class="badge ok">Настройки сохранены</span>';
      toast('Home Assistant сохранен');
    } catch (e) {
      $('#ha-result').innerHTML = `<p class="error-banner">${esc(e.message)}</p>`;
    }
  });

  $('#frigate-plan')?.addEventListener('click', async () => {
    try {
      const p = await api('/api/v1/frigate/plan');
      $('#frigate-result').innerHTML = `<pre class="text-mono text-xs" style="background:#070a0f; padding:10px; border-radius:8px; overflow:auto; max-height:160px;">${esc(JSON.stringify(p, null, 2))}</pre>`;
    } catch (e) {
      $('#frigate-result').innerHTML = `<p class="error-banner">${esc(e.message)}</p>`;
    }
  });

  $('#frigate-apply')?.addEventListener('click', async () => {
    if (!confirm('Применить сгенерированную конфигурацию Frigate?')) return;
    try {
      await apiFresh('/api/v1/frigate/apply', { method: 'POST' });
      toast('Конфигурация Frigate применена');
    } catch (e) {
      $('#frigate-result').innerHTML = `<p class="error-banner">${esc(e.message)}</p>`;
    }
  });

  $('#telegram-pair')?.addEventListener('click', async () => {
    try {
      const d = await api('/api/v1/telegram/pairing', { method: 'POST' });
      $('#telegram-result').innerHTML = `
        <div class="list-item" style="margin-top:10px;">
          <div>
            <strong>Команда для бота:</strong>
            <div class="text-mono" style="color:#60a5fa; font-weight:700; margin-top:2px;">${esc(d.command)}</div>
            <div class="muted text-xs">Действует до ${fmtTime(d.expires_at)}</div>
          </div>
        </div>`;
    } catch (e) {
      $('#telegram-result').innerHTML = `<p class="error-banner">${esc(e.message)}</p>`;
    }
  });

  $('#ai-models')?.addEventListener('click', async () => {
    try {
      const d = await api('/api/v1/ai/models');
      $('#ai-result').innerHTML = `<pre class="text-mono text-xs" style="background:#070a0f; padding:10px; border-radius:8px; overflow:auto; max-height:160px;">${esc(JSON.stringify(d.items || d, null, 2))}</pre>`;
    } catch (e) {
      $('#ai-result').innerHTML = `<p class="error-banner">${esc(e.message)}</p>`;
    }
  });

  $$('.save-user').forEach(btn => {
    btn.onclick = async () => {
      const row = btn.closest('[data-user]');
      try {
        await apiFresh(`/api/v1/users/${encodeURIComponent(row.dataset.user)}`, {
          method: 'PATCH',
          body: {
            role: $('.user-role', row).value,
            disabled: $('.user-disabled', row).checked
          }
        });
        toast('Права пользователя обновлены');
      } catch (err) {
        toast(err.message, true);
      }
    };
  });
}

function debounce(fn, ms) {
  let t;
  return (...a) => {
    clearTimeout(t);
    t = setTimeout(() => fn(...a), ms);
  };
}

document.addEventListener('click', e => {
  const row = e.target.closest('[data-incident]');
  if (row) location.hash = `#incidents/${row.dataset.incident}`;
});

init();
