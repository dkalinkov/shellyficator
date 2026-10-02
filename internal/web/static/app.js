document.addEventListener('htmx:responseError', event => {
  if (event.detail.elt.matches('[data-organization-form], [data-organization-open]')) return;
  const alert = document.getElementById('request-error');
  if (alert) alert.hidden = false;
});
document.addEventListener('change', event => {
  if (event.target.id !== 'auto-refresh') return;
  const table = document.getElementById('inventory');
  table.dataset.paused = String(!event.target.checked);
});
document.addEventListener('htmx:beforeRequest', event => {
  if (event.detail.elt.id !== 'inventory') return;
  const tooltip = document.getElementById('app-tooltip');
  const editor = document.getElementById('organization-dialog');
  if (document.hidden || event.detail.elt.dataset.paused === 'true' || (tooltip && !tooltip.hidden) || (editor && editor.open) || document.querySelector('[data-tag-filter] [aria-expanded="true"]')) event.preventDefault();
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

function setInputsOpen(button, open) {
  const group = button.closest('.device-group');
  group.dataset.open = String(open);
  button.setAttribute('aria-expanded', String(open));
  const row = group.querySelector('.input-detail-row');
  row.setAttribute('aria-hidden', String(!open));
  row.inert = !open;
}

function applyInputsState(root) {
  for (const button of root.querySelectorAll('[data-input-toggle]')) {
    setInputsOpen(button, expandedInputs.has(button.getAttribute('aria-controls')));
  }
}

function restoreInputs(root) {
  root.classList.add('inventory-restoring');
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
document.addEventListener('htmx:beforeSwap', event => {
  if (event.detail.target.id !== 'inventory' || !event.detail.shouldSwap) return;
  // HTMX restores focus during the swap. Insert panels already open so that
  // focus restoration cannot collapse the page and clamp its scroll position.
  const content = document.createElement('template');
  content.innerHTML = event.detail.serverResponse;
  applyInputsState(content.content);
  event.detail.serverResponse = content.innerHTML;
});
const inventory = document.getElementById('inventory');
if (inventory) restoreInputs(inventory);

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
