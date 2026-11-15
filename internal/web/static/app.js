const toastStack = document.getElementById('toast-stack');
const toastTemplate = document.getElementById('toast-template');
const toasts = new Set();
const toastStyles = {
  success: ['alert-success', 'check-circle'],
  info: ['alert-info', 'note'],
  warning: ['alert-warning', 'warning'],
  error: ['alert-error', 'close-circle'],
};

function showToast(message, level = 'info') {
  if (!toastStack || !toastTemplate || typeof message !== 'string' || !message.trim()) return;
  if (!Object.hasOwn(toastStyles, level)) level = 'info';
  const duration = level === 'error' || level === 'warning' ? 8000 : 5000;
  for (const toast of toasts) {
    if (toast.message === message && toast.level === level) {
      toast.pause();
      toast.remaining = duration;
      toast.resume();
      return;
    }
  }

  const element = toastTemplate.content.firstElementChild.cloneNode(true);
  const [color, icon] = toastStyles[level];
  element.classList.add(color);
  element.setAttribute('role', level === 'error' ? 'alert' : 'status');
  element.querySelector('[data-toast-text]').textContent = message;
  element.querySelector('[data-toast-icon]').setAttribute('href', '#icon-' + icon);
  const opener = document.activeElement;
  const toast = {element, message, level, remaining: duration, timer: null, started: 0};
  toast.pause = () => {
    if (toast.timer === null) return;
    clearTimeout(toast.timer);
    toast.timer = null;
    toast.remaining = Math.max(0, toast.remaining - (performance.now() - toast.started));
  };
  toast.resume = () => {
    if (!toasts.has(toast) || toast.timer !== null || document.hidden || element.matches(':hover, :focus-within')) return;
    toast.started = performance.now();
    toast.timer = setTimeout(() => toast.dismiss(), toast.remaining);
  };
  toast.dismiss = (immediate = false) => {
    if (!toasts.delete(toast)) return;
    toast.pause();
    if (element.contains(document.activeElement)) {
      const target = opener?.isConnected && opener !== document.body && !opener.disabled
        ? opener : document.querySelector('#search, .detail-nav a');
      target?.focus({preventScroll: true});
    }
    element.inert = true;
    element.classList.remove('is-open');
    const delay = immediate || window.matchMedia('(prefers-reduced-motion: reduce)').matches
      ? 0 : parseFloat(getComputedStyle(element).getPropertyValue('--toast-close'));
    if (delay) setTimeout(() => element.remove(), delay);
    else element.remove();
  };
  element.querySelector('button').addEventListener('click', () => toast.dismiss());
  element.addEventListener('pointerenter', toast.pause);
  element.addEventListener('pointerleave', toast.resume);
  element.addEventListener('focusin', toast.pause);
  element.addEventListener('focusout', () => requestAnimationFrame(toast.resume));

  // Repeated polling failures share one toast; keep the corner stack compact.
  if (toasts.size >= 3) {
    [...toasts].find(item => !item.element.matches(':hover, :focus-within'))?.dismiss(true);
  }
  toasts.add(toast);
  toastStack.append(element);
  requestAnimationFrame(() => requestAnimationFrame(() => {
    if (toasts.has(toast)) element.classList.add('is-open');
  }));
  toast.resume();
}

function clearNoticeURL() {
  const url = new URL(window.location.href);
  if (!url.searchParams.has('notice')) return;
  url.searchParams.delete('notice');
  url.searchParams.delete('notice_level');
  window.history.replaceState(window.history.state, '', url);
}

const initialNotice = toastStack?.querySelector('[data-initial-notice]');
if (initialNotice) {
  showToast(initialNotice.textContent.trim(), initialNotice.dataset.level);
  initialNotice.remove();
  clearNoticeURL();
}
document.addEventListener('notify', event => showToast(event.detail.message, event.detail.level));
document.addEventListener('visibilitychange', () => {
  for (const toast of toasts) {
    if (document.hidden) toast.pause();
    else toast.resume();
  }
});
document.addEventListener('htmx:afterSettle', clearNoticeURL);
for (const name of ['htmx:responseError', 'htmx:sendError', 'htmx:timeout']) {
  document.addEventListener(name, event => {
    const source = event.detail.elt;
    if (source.matches('[data-organization-form], [data-organization-open], [data-firmware-form], [data-firmware-watch]') || event.detail.xhr?.getResponseHeader('HX-Redirect')) return;
    let message = source.id === 'inventory' ? 'Refresh failed. Try again.' : 'Request failed. Try again.';
    try { message = JSON.parse(event.detail.xhr.responseText).message || message; } catch { /* Network failure. */ }
    showToast(message, 'error');
  });
}
document.addEventListener('change', event => {
  if (event.target.id !== 'auto-refresh') return;
  const table = document.getElementById('inventory');
  table.dataset.paused = String(!event.target.checked);
});
document.addEventListener('htmx:beforeRequest', event => {
  if (event.detail.elt.id !== 'inventory') return;
  const tooltip = document.getElementById('app-tooltip');
  const editor = document.getElementById('organization-dialog');
  const firmware = document.getElementById('firmware-dialog');
  if (document.hidden || event.detail.elt.dataset.paused === 'true' || (tooltip && !tooltip.hidden) || (editor && editor.open) || (firmware && firmware.open) || document.querySelector('[data-tag-filter] [aria-expanded="true"]')) event.preventDefault();
});

// Keep ordinary links and buttons native; the remaining row area opens details.
function openRow(event) {
  if (event.defaultPrevented || (event.type === 'click' ? event.button !== 0 : event.button !== 1)) return;
  const row = event.target.closest('[data-row-href]');
  if (!row || event.target.closest('a, button, input, select, textarea, label, summary, [contenteditable]')) return;
  const selection = window.getSelection();
  if (selection && !selection.isCollapsed && row.contains(selection.anchorNode)) return;
  event.preventDefault();
  if (event.button === 1 || event.ctrlKey || event.metaKey || event.shiftKey) {
    window.open(row.dataset.rowHref, '_blank', 'noopener');
  } else {
    window.location.assign(row.dataset.rowHref);
  }
}
document.addEventListener('click', openRow);
document.addEventListener('auxclick', openRow);

const expandedInputs = new Set();
try {
  const saved = JSON.parse(sessionStorage.getItem('shelly-expanded-inputs') || '[]');
  if (Array.isArray(saved)) saved.forEach(id => expandedInputs.add(id));
} catch { /* Disclosure state is optional when storage is unavailable. */ }

const collapsedRooms = new Set();
try {
  const saved = JSON.parse(sessionStorage.getItem('shelly-collapsed-rooms') || '[]');
  if (Array.isArray(saved)) saved.filter(room => typeof room === 'string').forEach(room => collapsedRooms.add(room));
} catch { /* Room disclosure state is optional when storage is unavailable. */ }

function setDisclosureOpen(owner, button, panel, open) {
  owner.dataset.open = String(open);
  button.setAttribute('aria-expanded', String(open));
  panel.setAttribute('aria-hidden', String(!open));
  panel.inert = !open;
}

function setRoomOpen(group, open) {
  setDisclosureOpen(group, group.querySelector('[data-room-toggle]'), group.querySelector('[data-room-panel]'), open);
}

function applyRoomState(root) {
  for (const group of root.querySelectorAll('[data-room-group]')) {
    const button = group.querySelector('[data-room-toggle]');
    const panel = group.querySelector('[data-room-panel]');
    // Room-based IDs stay stable when filtering changes section order.
    const key = encodeURIComponent(group.dataset.roomGroup);
    button.id = 'room-toggle-' + key;
    panel.id = 'room-panel-' + key;
    button.setAttribute('aria-controls', panel.id);
    setRoomOpen(group, !collapsedRooms.has(group.dataset.roomGroup));
  }
}

function setInputsOpen(button, open) {
  const group = button.closest('.device-group');
  setDisclosureOpen(group, button, group.querySelector('.input-detail-row'), open);
}

function applyInputsState(root) {
  for (const button of root.querySelectorAll('[data-input-toggle]')) {
    setInputsOpen(button, expandedInputs.has(button.getAttribute('aria-controls')));
  }
}

function restoreInventoryDisclosures(root) {
  root.classList.add('inventory-restoring');
  applyRoomState(root);
  applyInputsState(root);
  requestAnimationFrame(() => requestAnimationFrame(() => root.classList.remove('inventory-restoring')));
}

document.addEventListener('click', event => {
  const button = event.target.closest('[data-input-toggle]');
  if (!button) return;
  const open = button.getAttribute('aria-expanded') !== 'true';
  const id = button.getAttribute('aria-controls');
  setInputsOpen(button, open);
  if (open) expandedInputs.add(id);
  else expandedInputs.delete(id);
  try { sessionStorage.setItem('shelly-expanded-inputs', JSON.stringify([...expandedInputs])); } catch { /* Optional. */ }
});
document.addEventListener('click', event => {
  const button = event.target.closest('[data-room-toggle]');
  if (!button) return;
  const group = button.closest('[data-room-group]');
  const open = button.getAttribute('aria-expanded') !== 'true';
  setRoomOpen(group, open);
  if (open) collapsedRooms.delete(group.dataset.roomGroup);
  else collapsedRooms.add(group.dataset.roomGroup);
  try { sessionStorage.setItem('shelly-collapsed-rooms', JSON.stringify([...collapsedRooms])); } catch { /* Optional. */ }
});
document.addEventListener('htmx:beforeSwap', event => {
  if (event.detail.target.id !== 'inventory' || !event.detail.shouldSwap) return;
  // HTMX restores focus during the swap. Insert panels already open so that
  // focus restoration cannot collapse the page and clamp its scroll position.
  const content = document.createElement('template');
  content.innerHTML = event.detail.serverResponse;
  applyRoomState(content.content);
  applyInputsState(content.content);
  event.detail.serverResponse = content.innerHTML;
});
const inventory = document.getElementById('inventory');
if (inventory) restoreInventoryDisclosures(inventory);

// One shared tooltip also works for rows replaced by HTMX.
const tooltip = document.getElementById('app-tooltip');
let trigger = null;
let suppressed = null;
let description = null;
let pinned = false;
let showTimer;
let hideTimer;

function hideTooltip() {
  clearTimeout(showTimer);
  clearTimeout(hideTimer);
  if (trigger) {
    if (description === null) trigger.removeAttribute('aria-describedby');
    else trigger.setAttribute('aria-describedby', description);
  }
  trigger = null;
  pinned = false;
  tooltip.hidden = true;
}

function dismissTooltip(element = trigger) {
  suppressed = element;
  hideTooltip();
}

function showTooltip(element) {
  clearTimeout(showTimer);
  clearTimeout(hideTimer);
  if (element === suppressed || !element.isConnected) return;
  if (trigger !== element) {
    hideTooltip();
    trigger = element;
    description = element.getAttribute('aria-describedby');
    element.setAttribute('aria-describedby', [description, 'app-tooltip'].filter(Boolean).join(' '));
  }
  tooltip.textContent = element.dataset.tooltip;
  tooltip.hidden = false;
  tooltip.style.left = '12px';
  tooltip.style.top = '12px';
  const rect = element.getBoundingClientRect();
  const width = tooltip.offsetWidth;
  const height = tooltip.offsetHeight;
  tooltip.style.left = `${Math.max(12, Math.min(rect.left + rect.width / 2 - width / 2, window.innerWidth - width - 12))}px`;
  let top = rect.bottom + 8;
  if (top + height > window.innerHeight - 12) top = rect.top - height - 8;
  tooltip.style.top = `${Math.max(12, top)}px`;
}

function scheduleHide() {
  clearTimeout(showTimer);
  clearTimeout(hideTimer);
  hideTimer = setTimeout(() => {
    if (pinned || tooltip.matches(':hover') || (trigger && (trigger.matches(':hover') || trigger.matches(':focus-visible')))) return;
    hideTooltip();
  }, 180);
}

document.addEventListener('pointerover', event => {
  if (event.pointerType === 'touch') return;
  const element = event.target.closest('[data-tooltip]');
  if (!element || element.contains(event.relatedTarget)) return;
  clearTimeout(showTimer);
  clearTimeout(hideTimer);
  showTimer = setTimeout(() => showTooltip(element), 250);
});
document.addEventListener('pointerout', event => {
  if (event.pointerType === 'touch') return;
  const element = event.target.closest('[data-tooltip]');
  if (!element || element.contains(event.relatedTarget)) return;
  if (suppressed === element) suppressed = null;
  if (trigger === element) pinned = false;
  scheduleHide();
});
tooltip.addEventListener('pointerenter', () => clearTimeout(hideTimer));
tooltip.addEventListener('pointerleave', scheduleHide);
document.addEventListener('focusin', event => {
  const element = event.target.closest('[data-tooltip]');
  if (element && element.matches(':focus-visible')) showTooltip(element);
});
document.addEventListener('focusout', event => {
  if (suppressed === event.target) suppressed = null;
  if (trigger === event.target) {
    pinned = false;
    scheduleHide();
  }
});
document.addEventListener('pointerdown', event => {
  const element = event.target.closest('[data-tooltip]');
  if (element && !element.matches('.tooltip-trigger')) dismissTooltip(element);
  else if (trigger && !trigger.contains(event.target) && !tooltip.contains(event.target)) dismissTooltip();
});
document.addEventListener('click', event => {
  const element = event.target.closest('.tooltip-trigger');
  if (!element) return;
  if (trigger === element && pinned) {
    dismissTooltip(element);
  } else {
    suppressed = null;
    showTooltip(element);
    pinned = true;
  }
});
document.addEventListener('keydown', event => {
  if (event.key === 'Escape' && trigger) {
    event.preventDefault();
    event.stopPropagation();
    dismissTooltip();
  }
});
document.addEventListener('htmx:beforeSwap', event => {
  if (event.detail.target.id === 'inventory') hideTooltip();
});
window.addEventListener('resize', hideTooltip);
document.addEventListener('scroll', hideTooltip, true);
