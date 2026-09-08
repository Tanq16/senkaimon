const el = (id) => document.getElementById(id);

const state = {
  account: null,
  users: [],
  tokens: [],
  policies: [],
  audit: [],
  view: 'users',
};

const VIEWS = [
  { id: 'users', label: 'Users', icon: 'users', admin: true },
  { id: 'tokens', label: 'Tokens', icon: 'key-round', admin: true },
  { id: 'policies', label: 'Policies', icon: 'shield', admin: true },
  { id: 'sessions', label: 'Sessions', icon: 'monitor-smartphone', admin: true },
  { id: 'audit', label: 'Audit', icon: 'scroll-text', admin: true },
  { id: 'account', label: 'Account', icon: 'user-round', admin: false },
];

const EVENT_TONE = {
  'login.success': 'green',
  'totp.enrolled': 'green',
  'token.minted': 'blue',
  'user.created': 'blue',
  'user.updated': 'blue',
  'policy.updated': 'blue',
  'session.revoked': 'peach',
  'token.revoked': 'peach',
  'recovery.used': 'yellow',
  'login.failure': 'red',
  'login.locked': 'red',
  'totp.failure': 'red',
  'token.denied': 'red',
  'access.denied': 'red',
  'user.deleted': 'red',
};

function esc(value) {
  return String(value ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

function refreshIcons() {
  lucide.createIcons();
}

async function api(method, path, body) {
  const options = { method, headers: {} };
  if (body !== undefined) {
    options.headers['Content-Type'] = 'application/json';
    options.body = JSON.stringify(body);
  }
  const res = await fetch(path, options);
  if (res.status === 401 && path !== '/api/account') {
    window.location.href = '/login';
    return null;
  }
  if (res.status === 204) return null;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `Request failed with ${res.status}`);
  return data;
}

function toast(message, tone = 'green') {
  const node = document.createElement('div');
  node.className = `flex items-center gap-2 rounded-full bg-mantle px-4 py-2.5 text-sm text-${tone} shadow-2xl shadow-crust/60 transition duration-300`;
  node.innerHTML = `<i data-lucide="${tone === 'red' ? 'circle-alert' : 'circle-check'}" class="w-4 h-4"></i><span>${esc(message)}</span>`;
  el('toast-root').appendChild(node);
  refreshIcons();
  setTimeout(() => {
    node.classList.add('opacity-0', 'translate-y-1');
    setTimeout(() => node.remove(), 300);
  }, 3200);
}

function closeModal() {
  el('modal-root').classList.add('hidden');
  el('modal-root').classList.remove('flex');
  el('modal-root').innerHTML = '';
}

function modal(title, bodyHTML, { confirmLabel = 'Save', tone = 'mauve', onConfirm = null } = {}) {
  const root = el('modal-root');
  root.innerHTML = `
    <div class="w-full max-w-md rounded-2xl bg-mantle p-5 shadow-2xl shadow-crust/70 flex flex-col gap-4">
      <div class="flex items-start justify-between gap-4">
        <h2 class="font-display text-lg text-text tracking-tight">${esc(title)}</h2>
        <button data-modal-close class="flex items-center justify-center w-8 h-8 rounded-full text-overlay1 hover:bg-surface0 hover:text-text transition">
          <i data-lucide="x" class="w-4 h-4"></i>
        </button>
      </div>
      <div class="flex flex-col gap-3">${bodyHTML}</div>
      <div class="flex items-center justify-end gap-2 pt-1">
        <button data-modal-close class="rounded-full px-4 py-2 text-sm text-overlay1 hover:text-text transition">Cancel</button>
        ${onConfirm ? `<button data-modal-confirm class="rounded-full bg-${tone} px-4 py-2 text-sm font-medium text-crust hover:opacity-90 transition">${esc(confirmLabel)}</button>` : ''}
      </div>
    </div>`;
  root.classList.remove('hidden');
  root.classList.add('flex');
  root.querySelectorAll('[data-modal-close]').forEach((b) => b.addEventListener('click', closeModal));
  const confirm = root.querySelector('[data-modal-confirm]');
  if (confirm) {
    confirm.addEventListener('click', async () => {
      confirm.disabled = true;
      try {
        await onConfirm(root);
        closeModal();
      } catch (err) {
        toast(err.message, 'red');
        confirm.disabled = false;
      }
    });
  }
  refreshIcons();
  root.querySelector('input, select, textarea')?.focus();
}

function field(label, inputHTML) {
  return `<label class="flex flex-col gap-1.5">
    <span class="text-xs font-medium uppercase tracking-wider text-overlay1">${esc(label)}</span>
    ${inputHTML}
  </label>`;
}

const INPUT = 'bg-surface0 rounded-xl px-3.5 py-2.5 text-text placeholder:text-overlay0 outline-none focus:ring-2 focus:ring-mauve/60 transition';
const SMALL = 'bg-surface0 rounded-lg px-3 py-2 text-sm text-text placeholder:text-overlay0 outline-none focus:ring-2 focus:ring-mauve/60 transition';
const CHEVRON = 'appearance-none pr-9';
const CHEVRON_ICON = '<i data-lucide="chevron-down" class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 text-overlay1"></i>';

function badge(text, tone) {
  return `<span class="inline-flex items-center rounded-full bg-${tone}/15 px-2.5 py-0.5 text-xs font-medium text-${tone}">${esc(text)}</span>`;
}

function iconButton(action, value, icon, tone, title) {
  return `<button data-action="${action}" data-value="${esc(value)}" title="${esc(title)}"
    class="flex items-center justify-center w-8 h-8 rounded-full text-overlay1 hover:bg-surface0 hover:text-${tone} transition">
    <i data-lucide="${icon}" class="w-4 h-4 pointer-events-none"></i></button>`;
}

function relative(value) {
  if (!value || value.startsWith('0001-')) return 'never';
  const seconds = (Date.now() - new Date(value).getTime()) / 1000;
  const steps = [[60, 'm'], [60, 'h'], [24, 'd'], [30, 'mo'], [12, 'y']];
  let n = Math.max(seconds, 0);
  let unit = 's';
  for (const [size, next] of steps) {
    if (n < size) break;
    n /= size;
    unit = next;
  }
  return `${Math.floor(n)}${unit} ago`;
}

function absolute(value) {
  if (!value) return '';
  return new Date(value).toLocaleString();
}

function table(headers, rows, empty) {
  if (!rows.length) {
    return `<div class="flex flex-col items-center gap-2 px-5 py-14 text-center">
      <i data-lucide="inbox" class="w-6 h-6 text-overlay0"></i>
      <p class="text-sm text-overlay1">${esc(empty)}</p>
    </div>`;
  }
  const head = headers.map((h) => `<th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-overlay1 ${h.right ? 'text-right' : ''}">${esc(h.label)}</th>`).join('');
  const body = rows.map((cells) => `<tr class="hover:bg-surface0/40 transition">${cells.join('')}</tr>`).join('');
  return `<div class="overflow-x-auto"><table class="w-full text-sm">
    <thead class="bg-crust/40"><tr>${head}</tr></thead>
    <tbody class="divide-y divide-surface1">${body}</tbody>
  </table></div>`;
}

function cell(content, extra = '') {
  return `<td class="px-5 py-3.5 ${extra}">${content}</td>`;
}

function renderNav() {
  const admin = state.account?.admin;
  el('nav').innerHTML = VIEWS.filter((v) => admin || !v.admin).map((v) => `
    <button data-view="${v.id}" class="shrink-0 flex items-center gap-2 rounded-full px-4 py-2 text-sm transition ${
      state.view === v.id ? 'bg-surface0 text-text' : 'text-overlay1 hover:text-subtext0'
    }">
      <i data-lucide="${v.icon}" class="w-4 h-4 pointer-events-none"></i>
      <span class="pointer-events-none">${v.label}</span>
    </button>`).join('');
  refreshIcons();
}

async function switchTo(view) {
  state.view = view;
  renderNav();
  for (const v of VIEWS) {
    const node = el('view-' + v.id);
    node.classList.toggle('hidden', v.id !== view);
    node.classList.toggle('flex', v.id === view);
  }
  await RENDERERS[view]();
  refreshIcons();
}

async function renderUsers() {
  state.users = await api('GET', '/api/admin/users');
  const rows = state.users.map((u) => [
    cell(`<div class="flex items-center gap-2.5">
      <span class="font-medium text-text">${esc(u.username)}</span>
      ${u.admin ? badge('admin', 'mauve') : ''}
    </div>`),
    cell(u.state === 'active' ? badge('active', 'green') : badge('pending enrolment', 'yellow')),
    cell(u.totp_required ? `<span class="text-subtext0">required</span>` : `<span class="text-overlay0">off</span>`),
    cell(`<span class="text-overlay1">${u.recovery_codes}</span>`),
    cell(`<span class="text-overlay1" title="${esc(absolute(u.created_at))}">${relative(u.created_at)}</span>`),
    cell(`<div class="flex items-center justify-end gap-0.5">
      ${iconButton('user-edit', u.username, 'pencil', 'text', 'Edit')}
      ${iconButton('user-reset-totp', u.username, 'rotate-ccw', 'yellow', 'Reset TOTP')}
      ${iconButton('user-delete', u.username, 'trash-2', 'red', 'Delete')}
    </div>`, 'text-right'),
  ]);
  el('users-table').innerHTML = table(
    [{ label: 'User' }, { label: 'State' }, { label: 'TOTP' }, { label: 'Recovery' }, { label: 'Created' }, { label: '', right: true }],
    rows,
    'No users yet.',
  );
}

async function renderTokens() {
  state.tokens = await api('GET', '/api/admin/tokens');
  const rows = state.tokens.map((t) => [
    cell(`<span class="font-medium text-text">${esc(t.name)}</span>`),
    cell(`<code class="font-mono text-xs text-mauve">${esc(t.id)}</code>`),
    cell(`<span class="text-subtext0">${esc(t.owner)}</span>`),
    cell(`<span class="text-overlay1">${t.expires_at ? absolute(t.expires_at) : 'no expiry'}</span>`),
    cell(`<span class="text-overlay1" title="${esc(absolute(t.last_used))}">${relative(t.last_used)}</span>`),
    cell(`<div class="flex items-center justify-end gap-0.5">${iconButton('token-revoke', t.id, 'trash-2', 'red', 'Revoke')}</div>`, 'text-right'),
  ]);
  el('tokens-table').innerHTML = table(
    [{ label: 'Name' }, { label: 'ID' }, { label: 'Owner' }, { label: 'Expires' }, { label: 'Last used' }, { label: '', right: true }],
    rows,
    'No tokens minted.',
  );
}

function subjectOptions(selected) {
  const users = state.users.map((u) => u.username);
  const tokens = state.tokens.map((t) => t.id);
  return [...users, ...tokens].filter((s) => !selected.includes(s));
}

function ruleRow(rule = { effect: 'allow', host: '' }) {
  return `<div class="flex items-center gap-2" data-rule>
    <div class="relative w-24 shrink-0">
      <select data-rule-effect class="w-full ${SMALL} ${CHEVRON}">
        <option value="allow" ${rule.effect === 'allow' ? 'selected' : ''}>allow</option>
        <option value="deny" ${rule.effect === 'deny' ? 'selected' : ''}>deny</option>
      </select>${CHEVRON_ICON}
    </div>
    <input data-rule-host value="${esc(rule.host)}" placeholder="*.example.com" class="flex-1 font-mono ${SMALL}">
    <button data-action="rule-remove" class="flex items-center justify-center w-8 h-8 shrink-0 rounded-full text-overlay1 hover:bg-surface0 hover:text-red transition">
      <i data-lucide="x" class="w-4 h-4 pointer-events-none"></i>
    </button>
  </div>`;
}

function policyCard(policy) {
  const subjects = policy.subjects || [];
  const orphaned = subjects.length === 0;
  return `<div class="bg-mantle rounded-2xl p-5 flex flex-col gap-4" data-policy="${esc(policy.name)}">
    <div class="flex items-start justify-between gap-3">
      <div class="flex items-center gap-2.5">
        <i data-lucide="shield" class="w-4 h-4 text-mauve"></i>
        <h2 class="font-medium text-text">${esc(policy.name)}</h2>
        ${orphaned ? badge('applies to nobody', 'yellow') : ''}
      </div>
      <div class="flex items-center gap-0.5">
        ${iconButton('policy-save', policy.name, 'check', 'green', 'Save')}
        ${iconButton('policy-delete', policy.name, 'trash-2', 'red', 'Delete')}
      </div>
    </div>

    <div class="flex flex-col gap-2">
      <span class="text-xs font-medium uppercase tracking-wider text-overlay1">Subjects</span>
      <div class="flex flex-wrap items-center gap-1.5" data-subjects>
        ${subjects.map((s) => `<span data-subject="${esc(s)}" class="inline-flex items-center gap-1.5 rounded-full bg-surface0 py-1 pl-3 pr-1.5 text-sm text-subtext0">
          ${esc(s)}
          <button data-action="subject-remove" class="flex items-center justify-center w-5 h-5 rounded-full text-overlay1 hover:text-red transition">
            <i data-lucide="x" class="w-3 h-3 pointer-events-none"></i>
          </button>
        </span>`).join('')}
        <div class="relative">
          <select data-action="subject-add" class="rounded-full bg-surface0 pl-3 pr-9 py-1 text-sm text-overlay1 outline-none focus:ring-2 focus:ring-mauve/60 transition appearance-none">
            <option value="">Add subject</option>
            ${subjectOptions(subjects).map((s) => `<option value="${esc(s)}">${esc(s)}</option>`).join('')}
          </select>${CHEVRON_ICON}
        </div>
      </div>
    </div>

    <div class="flex flex-col gap-2">
      <span class="text-xs font-medium uppercase tracking-wider text-overlay1">Rules</span>
      <div class="flex flex-col gap-2" data-rules>${(policy.rules || []).map(ruleRow).join('')}</div>
      <button data-action="rule-add" class="self-start flex items-center gap-1.5 rounded-full px-3 py-1.5 text-sm text-overlay1 hover:bg-surface0 hover:text-text transition">
        <i data-lucide="plus" class="w-3.5 h-3.5 pointer-events-none"></i>
        <span class="pointer-events-none">Add rule</span>
      </button>
    </div>
  </div>`;
}

async function renderPolicies() {
  [state.users, state.tokens, state.policies] = await Promise.all([
    api('GET', '/api/admin/users'),
    api('GET', '/api/admin/tokens'),
    api('GET', '/api/admin/policies'),
  ]);
  el('policies-list').innerHTML = state.policies.length
    ? state.policies.map(policyCard).join('')
    : `<div class="bg-mantle rounded-2xl flex flex-col items-center gap-2 px-5 py-14 text-center">
        <i data-lucide="shield-off" class="w-6 h-6 text-overlay0"></i>
        <p class="text-sm text-overlay1">No policies. Nobody can reach anything.</p>
      </div>`;
}

function readPolicy(card) {
  return {
    name: card.dataset.policy,
    subjects: [...card.querySelectorAll('[data-subject]')].map((n) => n.dataset.subject),
    rules: [...card.querySelectorAll('[data-rule]')].map((n) => ({
      effect: n.querySelector('[data-rule-effect]').value,
      host: n.querySelector('[data-rule-host]').value.trim(),
    })).filter((r) => r.host !== ''),
  };
}

function sessionRows(sessions, action) {
  return sessions.map((s) => [
    cell(`<span class="font-medium text-text">${esc(s.subject)}</span>`),
    cell(`<code class="font-mono text-xs text-overlay1">${esc(s.hash.slice(0, 12))}</code>`),
    cell(`<span class="text-overlay1" title="${esc(absolute(s.issued_at))}">${relative(s.issued_at)}</span>`),
    cell(`<span class="text-overlay1" title="${esc(absolute(s.last_seen))}">${relative(s.last_seen)}</span>`),
    cell(`<span class="text-overlay1">${absolute(s.expires_at)}</span>`),
    cell(`<div class="flex items-center justify-end gap-0.5">${iconButton(action, s.hash, 'log-out', 'red', 'Revoke')}</div>`, 'text-right'),
  ]);
}

const SESSION_HEADERS = [{ label: 'Subject' }, { label: 'Session' }, { label: 'Signed in' }, { label: 'Last seen' }, { label: 'Expires' }, { label: '', right: true }];

async function renderSessions() {
  const sessions = await api('GET', '/api/admin/sessions');
  el('sessions-table').innerHTML = table(SESSION_HEADERS, sessionRows(sessions, 'session-revoke'), 'Nobody is signed in.');
}

function renderAuditTable() {
  const wanted = el('audit-event').value;
  const subject = el('audit-subject').value.trim().toLowerCase();
  const rows = state.audit
    .filter((e) => (!wanted || e.event === wanted) && (!subject || (e.subject || '').toLowerCase().includes(subject)))
    .map((e) => [
      cell(badge(e.event, EVENT_TONE[e.event] || 'overlay1')),
      cell(`<span class="text-subtext0">${esc(e.subject || '')}</span>`),
      cell(`<span class="font-mono text-xs text-overlay1">${esc(e.ip || '')}</span>`),
      cell(`<span class="text-overlay1">${esc(e.host || e.detail || '')}</span>`),
      cell(`<span class="text-overlay1" title="${esc(absolute(e.ts))}">${relative(e.ts)}</span>`, 'text-right'),
    ]);
  el('audit-table').innerHTML = table(
    [{ label: 'Event' }, { label: 'Subject' }, { label: 'Address' }, { label: 'Detail' }, { label: 'When', right: true }],
    rows,
    'Nothing recorded yet.',
  );
  refreshIcons();
}

async function renderAudit() {
  state.audit = await api('GET', '/api/admin/audit?limit=500');
  const seen = [...new Set(state.audit.map((e) => e.event))].sort();
  const current = el('audit-event').value;
  el('audit-event').innerHTML = `<option value="">All events</option>` + seen.map((e) => `<option value="${esc(e)}" ${e === current ? 'selected' : ''}>${esc(e)}</option>`).join('');
  renderAuditTable();
}

async function renderAccount() {
  state.account = await api('GET', '/api/account');
  const labels = {
    enrolled: [badge('enrolled', 'green'), 'Your authenticator is registered. An admin can reset it if you lose the device.'],
    pending: [badge('pending enrolment', 'yellow'), 'You enrol at your next sign in.'],
    off: [badge('off', 'overlay1'), 'This account signs in with a password alone.'],
  }[state.account.totp];
  el('account-totp').innerHTML = `
    <div>${labels[0]}</div>
    <p class="text-overlay1">${labels[1]}</p>
    ${state.account.totp === 'enrolled' ? `<p class="text-overlay1">${state.account.recovery_codes} recovery code${state.account.recovery_codes === 1 ? '' : 's'} left.</p>` : ''}`;

  const sessions = await api('GET', '/api/account/sessions');
  el('account-sessions').innerHTML = table(SESSION_HEADERS, sessionRows(sessions, 'account-session-revoke'), 'No active sessions.');
}

const RENDERERS = {
  users: renderUsers,
  tokens: renderTokens,
  policies: renderPolicies,
  sessions: renderSessions,
  audit: renderAudit,
  account: renderAccount,
};

const ACTIONS = {
  'user-create': () => modal('New user', [
    field('Username', `<input data-f="username" placeholder="asha" class="${INPUT}">`),
    field('Password', `<input data-f="password" type="password" placeholder="at least 12 characters" class="${INPUT}">`),
    `<label class="flex items-center gap-2.5 text-sm text-subtext0 cursor-pointer select-none">
      <input data-f="totp" type="checkbox" checked class="w-4 h-4 accent-mauve"><span>Require a second factor</span></label>`,
    `<label class="flex items-center gap-2.5 text-sm text-subtext0 cursor-pointer select-none">
      <input data-f="admin" type="checkbox" class="w-4 h-4 accent-mauve"><span>Grant admin</span></label>`,
  ].join(''), {
    confirmLabel: 'Create',
    onConfirm: async (root) => {
      const f = (name) => root.querySelector(`[data-f="${name}"]`);
      await api('POST', '/api/admin/users', {
        username: f('username').value.trim(),
        password: f('password').value,
        admin: f('admin').checked,
        totp_required: f('totp').checked,
      });
      toast('User created');
      await renderUsers();
      refreshIcons();
    },
  }),

  'user-edit': (username) => {
    const user = state.users.find((u) => u.username === username);
    modal(`Edit ${username}`, [
      field('New password', `<input data-f="password" type="password" placeholder="leave blank to keep it" class="${INPUT}">`),
      `<label class="flex items-center gap-2.5 text-sm text-subtext0 cursor-pointer select-none">
        <input data-f="totp" type="checkbox" ${user.totp_required ? 'checked' : ''} class="w-4 h-4 accent-mauve"><span>Require a second factor</span></label>`,
      `<label class="flex items-center gap-2.5 text-sm text-subtext0 cursor-pointer select-none">
        <input data-f="admin" type="checkbox" ${user.admin ? 'checked' : ''} class="w-4 h-4 accent-mauve"><span>Grant admin</span></label>`,
      `<p class="text-xs text-overlay1">Turning the second factor on clears the existing secret and sends the user back through enrolment.</p>`,
    ].join(''), {
      onConfirm: async (root) => {
        const f = (name) => root.querySelector(`[data-f="${name}"]`);
        const body = { admin: f('admin').checked, totp_required: f('totp').checked };
        if (f('password').value) body.password = f('password').value;
        await api('PATCH', `/api/admin/users/${encodeURIComponent(username)}`, body);
        toast('User updated');
        await renderUsers();
        refreshIcons();
      },
    });
  },

  'user-reset-totp': (username) => modal(`Reset TOTP for ${username}`,
    `<p class="text-sm text-subtext0">The current secret and every recovery code are discarded. ${esc(username)} enrols a new authenticator at the next sign in.</p>`, {
      confirmLabel: 'Reset', tone: 'yellow',
      onConfirm: async () => {
        await api('POST', `/api/admin/users/${encodeURIComponent(username)}/reset-totp`);
        toast('TOTP reset');
        await renderUsers();
        refreshIcons();
      },
    }),

  'user-delete': (username) => modal(`Delete ${username}`,
    `<p class="text-sm text-subtext0">This also deletes their sessions, the tokens they own, and removes them from every policy.</p>`, {
      confirmLabel: 'Delete', tone: 'red',
      onConfirm: async () => {
        await api('DELETE', `/api/admin/users/${encodeURIComponent(username)}`);
        toast('User deleted');
        await renderUsers();
        refreshIcons();
      },
    }),

  'token-mint': () => modal('Mint token', [
    field('Name', `<input data-f="name" placeholder="backhub nightly" class="${INPUT}">`),
    field('Owner', `<div class="relative"><select data-f="owner" class="w-full ${INPUT} ${CHEVRON}">${state.users.map((u) => `<option value="${esc(u.username)}">${esc(u.username)}</option>`).join('')}</select>${CHEVRON_ICON}</div>`),
    field('Expires', `<input data-f="expires" type="date" class="${INPUT}">`),
    `<p class="text-xs text-overlay1">Leave the date empty for a token that never expires. A token is never an admin.</p>`,
  ].join(''), {
    confirmLabel: 'Mint',
    onConfirm: async (root) => {
      const f = (name) => root.querySelector(`[data-f="${name}"]`);
      const expires = f('expires').value;
      const result = await api('POST', '/api/admin/tokens', {
        name: f('name').value.trim(),
        owner: f('owner').value,
        expires_at: expires ? new Date(expires + 'T23:59:59Z').toISOString() : null,
      });
      await renderTokens();
      showToken(result.token);
    },
  }),

  'token-revoke': (id) => modal(`Revoke ${id}`,
    `<p class="text-sm text-subtext0">Any caller still holding this token is refused immediately, and the id is removed from every policy.</p>`, {
      confirmLabel: 'Revoke', tone: 'red',
      onConfirm: async () => {
        await api('DELETE', `/api/admin/tokens/${encodeURIComponent(id)}`);
        toast('Token revoked');
        await renderTokens();
        refreshIcons();
      },
    }),

  'policy-create': () => modal('New policy', field('Name', `<input data-f="name" placeholder="family" class="${INPUT}">`), {
    confirmLabel: 'Create',
    onConfirm: async (root) => {
      const name = root.querySelector('[data-f="name"]').value.trim();
      if (!name) throw new Error('A policy needs a name');
      await api('PUT', `/api/admin/policies/${encodeURIComponent(name)}`, {
        name, subjects: [], rules: [{ effect: 'allow', host: '*.example.com' }],
      });
      toast('Policy created');
      await renderPolicies();
      refreshIcons();
    },
  }),

  'policy-save': async (name, target) => {
    const policy = readPolicy(target.closest('[data-policy]'));
    await api('PUT', `/api/admin/policies/${encodeURIComponent(name)}`, policy);
    toast('Policy saved');
    await renderPolicies();
    refreshIcons();
  },

  'policy-delete': (name) => modal(`Delete ${name}`,
    `<p class="text-sm text-subtext0">Any subject this was the only policy for is denied everything from now on.</p>`, {
      confirmLabel: 'Delete', tone: 'red',
      onConfirm: async () => {
        await api('DELETE', `/api/admin/policies/${encodeURIComponent(name)}`);
        toast('Policy deleted');
        await renderPolicies();
        refreshIcons();
      },
    }),

  'rule-add': (_, target) => {
    target.closest('[data-policy]').querySelector('[data-rules]').insertAdjacentHTML('beforeend', ruleRow());
    refreshIcons();
  },

  'rule-remove': (_, target) => target.closest('[data-rule]').remove(),

  'subject-remove': (_, target) => target.closest('[data-subject]').remove(),

  'session-revoke': async (hash) => {
    await api('DELETE', `/api/admin/sessions/${encodeURIComponent(hash)}`);
    toast('Session revoked');
    await renderSessions();
    refreshIcons();
  },

  'account-session-revoke': async (hash) => {
    await api('DELETE', `/api/account/sessions/${encodeURIComponent(hash)}`);
    toast('Session revoked');
    await renderAccount();
    refreshIcons();
  },
};

function showToken(token) {
  modal('Token minted', `
    <p class="text-sm text-subtext0">Copy it now. Only its hash is stored, so this is the last time it is readable.</p>
    <code id="token-value" class="block rounded-xl bg-base p-3 font-mono text-xs text-text break-all leading-relaxed">${esc(token)}</code>
    <button id="token-copy" class="self-start flex items-center gap-2 rounded-full bg-surface0 px-4 py-2 text-sm text-subtext0 hover:bg-surface1 hover:text-text transition">
      <i data-lucide="copy" class="w-4 h-4 pointer-events-none"></i><span class="pointer-events-none">Copy</span>
    </button>`);
  el('token-copy').addEventListener('click', async () => {
    await navigator.clipboard.writeText(token);
    el('token-copy').querySelector('span').textContent = 'Copied';
  });
  refreshIcons();
}

document.addEventListener('click', async (event) => {
  const viewButton = event.target.closest('[data-view]');
  if (viewButton) return switchTo(viewButton.dataset.view);

  const actionButton = event.target.closest('[data-action]');
  if (!actionButton || actionButton.tagName === 'SELECT') return;
  const handler = ACTIONS[actionButton.dataset.action];
  if (!handler) return;
  try {
    await handler(actionButton.dataset.value, actionButton);
  } catch (err) {
    toast(err.message, 'red');
  }
});

document.addEventListener('change', (event) => {
  const picker = event.target.closest('[data-action="subject-add"]');
  if (!picker || !picker.value) return;
  const subject = picker.value;
  picker.insertAdjacentHTML('beforebegin', `<span data-subject="${esc(subject)}" class="inline-flex items-center gap-1.5 rounded-full bg-surface0 py-1 pl-3 pr-1.5 text-sm text-subtext0">
    ${esc(subject)}
    <button data-action="subject-remove" class="flex items-center justify-center w-5 h-5 rounded-full text-overlay1 hover:text-red transition">
      <i data-lucide="x" class="w-3 h-3 pointer-events-none"></i>
    </button>
  </span>`);
  picker.querySelector(`option[value="${CSS.escape(subject)}"]`)?.remove();
  picker.value = '';
  refreshIcons();
});

document.addEventListener('keydown', (event) => {
  if (event.key === 'Escape' && !el('modal-root').classList.contains('hidden')) closeModal();
});

el('audit-event').addEventListener('change', renderAuditTable);
el('audit-subject').addEventListener('input', renderAuditTable);

el('password-form').addEventListener('submit', async (event) => {
  event.preventDefault();
  try {
    await api('POST', '/api/account/password', {
      current_password: el('current-password').value,
      new_password: el('new-password').value,
    });
    el('current-password').value = '';
    el('new-password').value = '';
    toast('Password updated');
  } catch (err) {
    toast(err.message, 'red');
  }
});

el('logout').addEventListener('click', async () => {
  await api('POST', '/api/logout');
  window.location.href = '/login';
});

(async () => {
  try {
    state.account = await api('GET', '/api/account');
  } catch {
    window.location.href = '/login';
    return;
  }
  el('whoami').innerHTML = `<i data-lucide="user-round" class="w-3.5 h-3.5 text-mauve"></i>${esc(state.account.username)}`;
  await switchTo(state.account.admin ? 'users' : 'account');
})();
