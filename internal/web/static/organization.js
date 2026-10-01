// One room, multiple tags; both reuse the inventory's existing labels.
const labelPickers = new WeakMap();
const cleanLabel = value => value.trim().replace(/\s+/g, ' ');
const tagParts = value => value.split(/[,\r\n]+/).map(cleanLabel).filter(Boolean);

function initializeLabelPickers(container) {
  for (const root of container.querySelectorAll('[data-label-picker]')) {
    if (labelPickers.has(root)) continue;
    const multiple = root.dataset.labelPicker === 'tags';
    const input = root.querySelector('[data-picker-input]');
    const menu = root.querySelector('[data-picker-menu]');
    const toggle = root.querySelector('[data-picker-toggle]');
    const clear = root.querySelector('[data-picker-clear]');
    const chips = root.querySelector('[data-tag-chips]');
    const status = root.querySelector('[data-picker-status]');
    const known = [...root.querySelectorAll('[data-picker-source] option')].map(option => option.textContent);
    let selected = [];
    let choices = [];
    let active = -1;
    let serialized;
    input.setAttribute('role', 'combobox');
    input.setAttribute('aria-autocomplete', 'list');
    input.setAttribute('aria-controls', menu.id);
    input.setAttribute('aria-expanded', 'false');

    const canonical = value => known.find(tag => tag.toLowerCase() === value.toLowerCase()) || value;
    if (multiple) {
      for (const value of tagParts(input.value)) {
        const tag = canonical(value);
        if (!selected.some(value => value.toLowerCase() === tag.toLowerCase())) selected.push(tag);
      }
      serialized = document.createElement('input');
      serialized.type = 'hidden';
      serialized.name = input.name;
      input.removeAttribute('name');
      root.append(serialized);
      input.value = '';
    }

    function close() {
      menu.hidden = true;
      input.setAttribute('aria-expanded', 'false');
      toggle.setAttribute('aria-expanded', 'false');
      input.removeAttribute('aria-activedescendant');
      active = -1;
    }

    function highlight(index) {
      active = index;
      for (const [i, option] of [...menu.querySelectorAll('[role=option]')].entries()) {
        option.setAttribute('aria-selected', String(i === active));
        if (i === active) {
          input.setAttribute('aria-activedescendant', option.id);
          option.scrollIntoView({block: 'nearest'});
        }
      }
    }

    function open(showAll = false) {
      const query = showAll ? '' : cleanLabel(input.value);
      const key = query.toLowerCase();
      choices = known.filter(value => value.toLowerCase().includes(key) &&
        (!multiple || !selected.some(tag => tag.toLowerCase() === value.toLowerCase())))
        .sort((a, b) => a.localeCompare(b)).map(value => ({value, label: value}));
      if (!multiple && !query) choices.unshift({value: '', label: 'No room'});
      if (query && !known.some(value => value.toLowerCase() === key) &&
        !selected.some(value => value.toLowerCase() === key)) {
        choices.push({value: query, label: (multiple ? 'Add “' : 'Create “') + query + '”', create: true});
      }
      menu.replaceChildren();
      active = -1;
      input.removeAttribute('aria-activedescendant');
      for (const [index, choice] of choices.entries()) {
        const option = document.createElement('div');
        option.id = menu.id + '-' + index;
        option.className = 'picker-option' + (choice.create ? ' picker-create' : '');
        option.setAttribute('role', 'option');
        option.setAttribute('aria-selected', 'false');
        option.textContent = choice.label;
        option.addEventListener('pointerdown', event => event.preventDefault());
        option.addEventListener('click', () => choose(choice.value));
        menu.append(option);
      }
      if (!choices.length) {
        close();
        return;
      }
      menu.hidden = false;
      input.setAttribute('aria-expanded', 'true');
      toggle.setAttribute('aria-expanded', 'true');
      const rect = input.getBoundingClientRect();
      const below = window.innerHeight - rect.bottom - 18;
      const above = rect.top - 18;
      const upwards = (multiple && root.closest('dialog')) || (below < 160 && above > below);
      menu.style.width = rect.width + 'px';
      menu.style.left = rect.left + 'px';
      menu.style.maxHeight = Math.max(48, Math.min(208, upwards ? above : below)) + 'px';
      menu.style.top = (upwards ? rect.top - menu.offsetHeight - 6 : rect.bottom + 6) + 'px';
      menu.scrollTop = 0;
    }

    function renderTags() {
      serialized.value = selected.join(', ');
      chips.replaceChildren();
      chips.hidden = !selected.length;
      for (const value of selected) {
        const chip = document.createElement('span');
        chip.className = 'tag-chip';
        const label = document.createElement('span');
        label.textContent = value;
        const remove = document.createElement('button');
        remove.type = 'button';
        remove.setAttribute('aria-label', 'Remove tag ' + value);
        remove.textContent = '×';
        remove.addEventListener('click', () => {
          selected = selected.filter(tag => tag !== value);
          renderTags();
          input.setCustomValidity('');
          status.textContent = 'Removed tag ' + value;
          input.focus();
          close();
        });
        chip.append(label, remove);
        chips.append(chip);
      }
    }

    function addTags(values) {
      const next = [...selected];
      for (let value of values) {
        value = canonical(cleanLabel(value));
        if (value && !next.some(tag => tag.toLowerCase() === value.toLowerCase())) next.push(value);
      }
      if ([...next.join(', ')].length > 500) {
        input.setCustomValidity('Tags can contain at most 500 characters in total.');
        input.reportValidity();
        return false;
      }
      selected = next;
      for (const value of selected) if (!known.includes(value)) known.push(value);
      input.setCustomValidity('');
      renderTags();
      status.textContent = 'Selected tags: ' + selected.join(', ');
      return true;
    }

    function choose(value) {
      if (multiple) {
        if (!addTags(tagParts(value))) return;
        input.value = '';
        input.focus();
        open();
      } else {
        input.value = canonical(value);
        clear.hidden = !input.value;
        close();
      }
    }

    input.addEventListener('focus', () => open(true));
    toggle.addEventListener('click', () => {
      const wasOpen = !menu.hidden;
      input.focus();
      if (wasOpen) close();
      else open(true);
    });
    input.addEventListener('click', () => { if (menu.hidden) open(true); });
    input.addEventListener('input', event => {
      input.setCustomValidity('');
      if (clear) clear.hidden = !input.value;
      if (multiple && !event.isComposing && input.value.includes(',')) {
        const pieces = input.value.split(',');
        const pending = pieces.pop();
        if (addTags(pieces)) input.value = pending;
      }
      open();
    });
    input.addEventListener('keydown', event => {
      if (event.isComposing) return;
      if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        event.preventDefault();
        if (menu.hidden) open(true);
        if (choices.length) highlight(event.key === 'ArrowDown' ? Math.min(active + 1, choices.length - 1) : (active < 0 ? choices.length - 1 : Math.max(0, active - 1)));
      } else if (event.key === 'Enter' && (!menu.hidden || (multiple && cleanLabel(input.value)))) {
        event.preventDefault();
        if (active >= 0) choose(choices[active].value);
        else if (cleanLabel(input.value)) choose(cleanLabel(input.value));
        else close();
      } else if (event.key === 'Escape' && !menu.hidden) {
        event.preventDefault();
        event.stopPropagation();
        close();
      } else if (event.key === 'Tab') close();
    });
    if (multiple) {
      input.addEventListener('paste', event => {
        const text = event.clipboardData.getData('text');
        if (!/[,\r\n]/.test(text)) return;
        event.preventDefault();
        const combined = input.value.slice(0, input.selectionStart) + text + input.value.slice(input.selectionEnd);
        if (addTags(tagParts(combined))) {
          input.value = '';
          open();
        }
      });
      renderTags();
    } else {
      clear.hidden = !input.value;
      clear.addEventListener('click', () => { input.value = ''; clear.hidden = true; input.focus(); open(true); });
    }
    root.addEventListener('focusout', event => { if (!root.contains(event.relatedTarget)) close(); });
    labelPickers.set(root, {
      commit() {
        if (!multiple) return true;
        if (!addTags(tagParts(input.value))) return false;
        input.value = '';
        close();
        return true;
      },
      close
    });
  }
}

initializeLabelPickers(document);
function closeLabelMenus(event) {
  for (const root of document.querySelectorAll('[data-label-picker]')) {
    if (!(event.target instanceof Node) || !root.contains(event.target)) labelPickers.get(root)?.close();
  }
}
document.addEventListener('pointerdown', closeLabelMenus);
document.addEventListener('scroll', closeLabelMenus, true);
window.addEventListener('resize', closeLabelMenus);
document.addEventListener('submit', event => {
  const form = event.target;
  if (!form.matches('[data-organization-form], [data-metadata-form]')) return;
  for (const root of form.querySelectorAll('[data-label-picker]')) {
    if (!labelPickers.get(root)?.commit()) {
      event.preventDefault();
      event.stopImmediatePropagation();
      return;
    }
  }
}, true);

const organizationDialog = document.getElementById('organization-dialog');
let organizationOpener;
let organizationField;
document.addEventListener('click', event => {
  const opener = event.target.closest('[data-organization-open]');
  if (opener) {
    organizationOpener = opener.id;
    organizationField = opener.id.startsWith('tags-edit-') ? 'device-tags' : 'device-room';
  }
  if (event.target.closest('[data-organization-close]')) organizationDialog?.close();
});
document.addEventListener('htmx:afterSwap', event => {
  if (event.detail.target.id !== 'organization-content') return;
  initializeLabelPickers(event.detail.target);
  if (!event.detail.target.querySelector('[data-organization-form]')) return;
  hideTooltip();
  organizationDialog.showModal();
  const input = document.getElementById(organizationField || 'device-room');
  input.focus();
  input.select();
});
document.addEventListener('htmx:configRequest', event => {
  if (!event.detail.elt.matches('[data-organization-form]')) return;
  event.detail.parameters.filter_q = document.getElementById('search').value;
  event.detail.parameters.filter_room = document.getElementById('room').value;
  event.detail.elt.querySelector('[data-organization-error]').hidden = true;
});
document.addEventListener('htmx:oobAfterSwap', event => {
  const filter = event.detail.target;
  if (filter.id !== 'room') return;
  // Inserting options before removing the old ones can reset native selection.
  filter.value = [...filter.options].find(option => option.defaultSelected)?.value || '';
});
document.addEventListener('htmx:afterRequest', event => {
  const source = event.detail.elt;
  if (source.matches('[data-organization-form]')) {
    if (event.detail.successful) {
      organizationDialog.close();
      document.getElementById('organization-status').textContent = 'Room & tags saved.';
    } else {
      const error = source.querySelector('[data-organization-error]');
      let message = 'Could not save. Try again.';
      try { message = JSON.parse(event.detail.xhr.responseText).message || message; } catch { /* Network failure. */ }
      error.textContent = message;
      error.hidden = false;
    }
  } else if (source.matches('[data-organization-open]') && !event.detail.successful) {
    const error = document.getElementById('request-error');
    error.textContent = 'Could not open room & tags.';
    error.hidden = false;
  }
});
if (organizationDialog) {
  organizationDialog.addEventListener('cancel', event => {
    if (organizationDialog.querySelector('.htmx-request')) event.preventDefault();
  });
  organizationDialog.addEventListener('close', () => {
    document.getElementById('organization-content').replaceChildren();
    (document.getElementById(organizationOpener) || document.getElementById('search')).focus({preventScroll: true});
  });
}
