// One room, multiple tags; both reuse the inventory's existing labels.
const labelPickers = new WeakMap();
const tagFilters = new WeakMap();
const cleanLabel = value => value.trim().replace(/\s+/g, ' ');
const tagParts = value => value.split(/[,\r\n]+/).map(cleanLabel).filter(Boolean);

// Name-based colors stay consistent across pages, filtering, and new labels.
function labelColor(value) {
  const name = cleanLabel(value).toLowerCase();
  if (!name) return '';
  let hash = 2166136261;
  for (const character of name) hash = Math.imul(hash ^ character.codePointAt(0), 16777619) >>> 0;
  return `hsl(${(hash % 36000) / 100} 68% 76%)`;
}

function colorLabel(element, value) {
  const color = labelColor(value);
  element.toggleAttribute('data-colored-label', Boolean(color));
  if (color) element.style.setProperty('--label-color', color);
  else element.style.removeProperty('--label-color');
}

function colorRoomFilter() {
  const filter = document.getElementById('room');
  if (!filter) return;
  colorLabel(filter, filter.value);
  for (const option of filter.options) colorLabel(option, option.value);
}

function initializeLabelColors(container) {
  for (const element of container.querySelectorAll('[data-room-label]')) colorLabel(element, element.dataset.roomLabel);
  colorRoomFilter();
}

function initializeLabelPickers(container) {
  for (const root of container.querySelectorAll('[data-label-picker]')) {
    if (labelPickers.has(root)) continue;
    const multiple = root.dataset.labelPicker === 'tags';
    const input = root.querySelector('[data-picker-input]');
    const field = root.querySelector('.picker-field');
    const menu = root.querySelector('[data-picker-menu]');
    const toggle = root.querySelector('[data-picker-toggle]');
    const clear = root.querySelector('[data-picker-clear]');
    const chips = root.querySelector('[data-tag-chips]');
    const status = root.querySelector('[data-picker-status]');
    let known = [...root.querySelectorAll('[data-picker-source] option')].map(option => option.textContent);
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
      hideTooltip();
      closeLabelMenus(input);
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
        colorLabel(option, choice.value);
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
        colorLabel(chip, value);
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
      } else {
        input.value = canonical(value);
        clear.hidden = !input.value;
        colorLabel(field, input.value);
      }
      input.focus({preventScroll: true});
      close();
    }

    toggle.addEventListener('click', () => {
      const wasOpen = !menu.hidden;
      input.focus();
      if (wasOpen) close();
      else open(true);
    });
    input.addEventListener('click', () => { if (menu.hidden) open(true); });
    input.addEventListener('input', event => {
      input.setCustomValidity('');
      if (clear && !multiple) clear.hidden = !input.value;
      if (!multiple) colorLabel(field, input.value);
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
      colorLabel(field, input.value);
      clear.addEventListener('click', () => {
        input.value = '';
        clear.hidden = true;
        colorLabel(field, '');
        input.focus();
        open(true);
      });
    }
    root.addEventListener('focusout', event => { if (!root.contains(event.relatedTarget)) close(); });
    labelPickers.set(root, {
      commit() {
        if (multiple) {
          if (!addTags(tagParts(input.value))) return false;
          input.value = '';
        }
        close();
        return true;
      },
      refreshOptions() {
        known = [...root.querySelectorAll('[data-picker-source] option')].map(option => option.textContent);
        if (!menu.hidden) open();
      },
      close
    });
  }
}

// Filtering uses a stable summary and checkboxes; editing uses removable chips.
function initializeTagFilter() {
  const root = document.querySelector('[data-tag-filter]');
  if (!root || tagFilters.has(root)) return;
  const toggle = root.querySelector('.tag-filter-toggle');
  const summary = root.querySelector('[data-filter-summary]');
  const count = root.querySelector('[data-filter-count]');
  const clear = root.querySelector('[data-filter-clear]');
  const clearAll = root.querySelector('[data-filter-clear-all]');
  const menu = root.querySelector('.tag-filter-menu');
  const search = root.querySelector('input[type="search"]');
  const options = root.querySelector('[data-filter-options]');
  const serialized = root.querySelector('input[name="tags"]');
  const status = root.querySelector('[data-filter-status]');
  const readOptions = () => [...root.querySelectorAll('[data-picker-source] option')].map(option => option.textContent);
  let known = readOptions();
  let selected = tagParts(serialized.value);
  const isSelected = value => selected.some(tag => tag.toLowerCase() === value.toLowerCase());

  function updateSummary() {
    serialized.value = selected.join(', ');
    summary.textContent = selected.length === 1 ? selected[0] : selected.length ? 'Selected tags' : 'All tags';
    colorLabel(summary, selected.length === 1 ? selected[0] : '');
    count.textContent = selected.length;
    count.hidden = clear.hidden = !selected.length;
    clearAll.disabled = !selected.length;
    root.toggleAttribute('data-active', Boolean(selected.length));
    toggle.setAttribute('aria-label', selected.length ? `Filter by tags, ${selected.length} selected: ${selected.join(', ')}` : 'Filter by tags, all tags');
    status.textContent = selected.length ? 'Selected tags: ' + selected.join(', ') : 'All tags shown.';
    for (const checkbox of options.querySelectorAll('input')) checkbox.checked = isSelected(checkbox.value);
  }

  function close(restoreFocus = false) {
    menu.hidden = true;
    toggle.setAttribute('aria-expanded', 'false');
    if (restoreFocus) toggle.focus({preventScroll: true});
  }

  function renderOptions() {
    const focused = options.contains(document.activeElement) ? document.activeElement.value : null;
    const query = cleanLabel(search.value).toLowerCase();
    const values = [...new Set([...known, ...selected])].filter(value => value.toLowerCase().includes(query))
      .sort((a, b) => Number(isSelected(b)) - Number(isSelected(a)) || a.localeCompare(b));
    options.replaceChildren();
    for (const value of values) {
      const label = document.createElement('label');
      label.className = 'tag-filter-option';
      colorLabel(label, value);
      const checkbox = document.createElement('input');
      checkbox.type = 'checkbox';
      checkbox.value = value;
      checkbox.checked = isSelected(value);
      label.addEventListener('pointerdown', event => {
        // Keep focus inside the control until the label activates its checkbox.
        if (event.button === 0 && event.target !== checkbox) event.preventDefault();
      });
      checkbox.addEventListener('change', event => {
        // Emit one filter change, independent of the native checkbox event.
        event.stopPropagation();
        selected = checkbox.checked ? [...selected, value] : selected.filter(tag => tag.toLowerCase() !== value.toLowerCase());
        updateSummary();
        root.dispatchEvent(new Event('change', {bubbles: true}));
      });
      const text = document.createElement('span');
      text.textContent = value;
      label.append(checkbox, text);
      options.append(label);
      if (value === focused) checkbox.focus({preventScroll: true});
    }
    if (!values.length) {
      const empty = document.createElement('p');
      empty.className = 'tag-filter-empty';
      empty.textContent = query ? 'No matching tags.' : 'No tags available.';
      options.append(empty);
    }
    if (!menu.hidden) positionMenu();
  }

  function positionMenu() {
    if (menu.hidden) return;
    const rect = toggle.closest('.tag-filter-control').getBoundingClientRect();
    const viewportHeight = window.visualViewport?.height || window.innerHeight;
    const viewportTop = window.visualViewport?.offsetTop || 0;
    const below = viewportTop + viewportHeight - rect.bottom - 14;
    const above = rect.top - viewportTop - 14;
    const upwards = below < 240 && above > below;
    menu.style.width = Math.min(Math.max(rect.width, 280), window.innerWidth - 24) + 'px';
    menu.style.left = Math.max(12, Math.min(rect.left, window.innerWidth - menu.offsetWidth - 12)) + 'px';
    menu.style.maxHeight = Math.max(120, Math.min(360, upwards ? above : below)) + 'px';
    const top = upwards ? rect.top - menu.offsetHeight - 6 : rect.bottom + 6;
    menu.style.top = Math.max(viewportTop + 12, Math.min(top, viewportTop + viewportHeight - menu.offsetHeight - 12)) + 'px';
  }

  function open(focusSearch = false) {
    hideTooltip();
    closeLabelMenus(toggle);
    search.value = '';
    renderOptions();
    menu.hidden = false;
    toggle.setAttribute('aria-expanded', 'true');
    positionMenu();
    options.scrollTop = 0;
    // Pointer opening keeps focus on the trigger; keyboard navigation enters search.
    if (focusSearch) search.focus({preventScroll: true});
  }

  function clearSelection() {
    if (!selected.length) return;
    selected = [];
    updateSummary();
    root.dispatchEvent(new Event('change', {bubbles: true}));
  }

  toggle.addEventListener('click', () => { if (menu.hidden) open(); else close(); });
  toggle.addEventListener('keydown', event => {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      open(true);
    }
  });
  search.addEventListener('input', renderOptions);
  menu.addEventListener('keydown', event => {
    if (event.isComposing) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      close(true);
    } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      const inputs = [...options.querySelectorAll('input')];
      const index = inputs.indexOf(document.activeElement);
      const next = event.key === 'ArrowDown' ? inputs[index + 1] : inputs[index - 1];
      if (next) {
        next.focus({preventScroll: true});
        next.closest('label').scrollIntoView({block: 'nearest'});
      } else if (event.key === 'ArrowUp') search.focus({preventScroll: true});
    } else if (event.key === 'Enter') {
      event.preventDefault();
      if (event.target.matches('input[type="checkbox"]')) event.target.click();
      else if (event.target === search) options.querySelector('input')?.focus({preventScroll: true});
      else event.target.click();
    }
  });
  clear.addEventListener('click', () => { clearSelection(); close(true); });
  clearAll.addEventListener('click', () => { clearSelection(); search.focus({preventScroll: true}); });
  root.querySelector('[data-filter-done]').addEventListener('click', () => close(true));
  root.addEventListener('focusout', event => {
    if (root.contains(event.relatedTarget)) return;
    // Option updates can briefly remove the focused checkbox before restoring it.
    setTimeout(() => { if (!root.contains(document.activeElement)) close(); }, 0);
  });
  window.visualViewport?.addEventListener('resize', positionMenu);
  updateSummary();
  tagFilters.set(root, {
    close,
    positionMenu,
    refreshOptions() {
      const next = readOptions();
      if (JSON.stringify(next) === JSON.stringify(known)) return;
      known = next;
      if (!menu.hidden) renderOptions();
    }
  });
}

initializeLabelPickers(document);
initializeTagFilter();
initializeLabelColors(document);
document.addEventListener('change', event => {
  if (event.target.id === 'room') colorRoomFilter();
});
// HTMX restores the response's inline styles while settling matched elements.
for (const eventName of ['htmx:afterSwap', 'htmx:afterSettle']) {
  document.addEventListener(eventName, event => initializeLabelColors(event.detail.target));
}
function closeLabelMenus(target) {
  for (const root of document.querySelectorAll('[data-label-picker]')) {
    if (!(target instanceof Node) || !root.contains(target)) labelPickers.get(root)?.close();
  }
  for (const root of document.querySelectorAll('[data-tag-filter]')) {
    if (!(target instanceof Node) || !root.contains(target)) tagFilters.get(root)?.close();
  }
}
document.addEventListener('pointerdown', event => closeLabelMenus(event.target));
document.addEventListener('scroll', event => closeLabelMenus(event.target), true);
window.addEventListener('resize', () => {
  for (const root of document.querySelectorAll('[data-label-picker]')) labelPickers.get(root)?.close();
  for (const root of document.querySelectorAll('[data-tag-filter]')) tagFilters.get(root)?.positionMenu();
});
document.addEventListener('keydown', event => {
  if (event.key !== 'Escape' || event.isComposing || event.defaultPrevented) return;
  for (const root of document.querySelectorAll('[data-label-picker]')) {
    if (root.querySelector('[data-picker-menu]').hidden) continue;
    event.preventDefault();
    event.stopPropagation();
    closeLabelMenus();
    return;
  }
  for (const root of document.querySelectorAll('[data-tag-filter]')) {
    if (root.querySelector('.tag-filter-menu').hidden) continue;
    event.preventDefault();
    tagFilters.get(root)?.close(true);
  }
});
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
function closeOrganizationDialog() {
  if (!organizationDialog || organizationDialog.querySelector('.htmx-request')) return;
  dismissTooltip(document.getElementById(organizationOpener));
  closeLabelMenus();
  organizationDialog.close();
}
document.addEventListener('click', event => {
  const opener = event.target.closest('[data-organization-open]');
  if (opener) {
    organizationOpener = opener.id;
    organizationField = opener.id.startsWith('tags-edit-') ? 'device-tags' : 'device-room';
  }
  if (event.target.closest('[data-organization-close]')) closeOrganizationDialog();
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
  event.detail.parameters.filter_tags = document.querySelector('[data-tag-filter] input[name="tags"]').value;
  event.detail.elt.querySelector('[data-organization-error]').hidden = true;
});
document.addEventListener('htmx:oobAfterSwap', event => {
  const filter = event.detail.target;
  if (filter.id === 'tag-filter-options') {
    tagFilters.get(filter.closest('[data-tag-filter]'))?.refreshOptions();
    return;
  }
  if (filter.id !== 'room') return;
  // Inserting options before removing the old ones can reset native selection.
  filter.value = [...filter.options].find(option => option.defaultSelected)?.value || '';
  colorRoomFilter();
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
  let startedOnBackdrop = false;
  function onBackdrop(event) {
    if (event.target !== organizationDialog) return false;
    const rect = organizationDialog.getBoundingClientRect();
    return event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom;
  }
  organizationDialog.addEventListener('pointerdown', event => {
    startedOnBackdrop = event.button === 0 && onBackdrop(event);
  });
  organizationDialog.addEventListener('click', event => {
    if (startedOnBackdrop && onBackdrop(event)) closeOrganizationDialog();
    startedOnBackdrop = false;
  });
  organizationDialog.addEventListener('cancel', event => {
    if (organizationDialog.querySelector('.htmx-request')) event.preventDefault();
  });
  organizationDialog.addEventListener('close', () => {
    closeLabelMenus();
    document.getElementById('organization-content').replaceChildren();
    const opener = document.getElementById(organizationOpener) || document.getElementById('search');
    dismissTooltip(opener);
    opener.focus({preventScroll: true});
  });
}
