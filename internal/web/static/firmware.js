(() => {
  const dialog = document.getElementById('firmware-dialog');
  const content = document.getElementById('firmware-dialog-content');
  let openerID, dialogFocus, wantsDialog = false, failures = 0;

  function updateSelection(form) {
    const selected = [...form.querySelectorAll('input[name="device"]:checked:not(:disabled)')];
    const beta = selected.some(input => form.querySelector(`[name="stage_${input.value}"]`)?.value === 'beta');
    const count = selected.length;
    form.querySelector('[data-firmware-selection]').textContent = `${count} selected${beta ? ' · includes beta firmware' : ''}`;
    const submit = form.querySelector('[data-firmware-submit]');
    submit.textContent = `Update ${count} ${count === 1 ? 'device' : 'devices'}`;
    submit.disabled = count === 0 || form.dataset.firmwareRunning === 'true';
  }

  function closeDialog(event) {
    if (!dialog?.open) return;
    event?.preventDefault();
    if (content.querySelector('.htmx-request')) return;
    dialog.close();
  }

  document.addEventListener('click', event => {
    const opener = event.target.closest('[data-firmware-open]');
    if (opener) {
      wantsDialog = true;
      openerID = opener.id;
    }
    if (event.target.closest('[data-firmware-close]')) {
      if (dialog?.open) closeDialog(event);
      else if (event.target.closest('button')) window.location.assign(basePath + '/');
    }
    if (event.target.closest('[data-firmware-select-stable]')) {
      const form = event.target.closest('form');
      for (const input of form.querySelectorAll('input[name="device"]:not(:disabled)')) {
        input.checked = input.dataset.hasStable === 'true';
        const stage = form.querySelector(`[name="stage_${input.value}"]`);
        if (input.checked && stage) stage.value = 'stable';
      }
      updateSelection(form);
    }
  });

  document.addEventListener('change', event => {
    const form = event.target.closest('[data-firmware-form]');
    if (form) updateSelection(form);
  });

  document.addEventListener('htmx:beforeSwap', event => {
    if (event.detail.target.id === 'firmware-dialog-content' && dialog?.open) dialogFocus = document.activeElement?.id;
  });

  document.addEventListener('htmx:afterSwap', event => {
    if (event.detail.target.id !== 'firmware-dialog-content') return;
    const form = content.querySelector('[data-firmware-form]');
    if (!form) return;
    updateSelection(form);
    if (!dialog.open) {
      if (!wantsDialog) return;
      hideTooltip();
      dialog.showModal();
      content.querySelector('#firmware-dialog-close')?.focus();
    } else {
      (document.getElementById(dialogFocus) || content.querySelector('#firmware-dialog-close'))?.focus({preventScroll: true});
    }
  });

  document.addEventListener('htmx:beforeRequest', event => {
    if (event.detail.elt.matches('[data-firmware-form]')) {
      event.detail.elt.querySelector('[data-firmware-error]').hidden = true;
    }
  });

  document.addEventListener('htmx:afterRequest', event => {
    const form = event.detail.elt;
    if (form.matches('[data-firmware-watch]')) {
      if (event.detail.successful) failures = 0;
      else if (++failures === 3) showToast('Could not read update progress. Updates may still be running; refresh when the connection returns.', 'warning');
    } else if (form.matches('[data-firmware-form]')) {
      if (event.detail.successful) {
        dialog.close();
        htmx.trigger(document.body, 'firmware-progress');
      } else {
        const error = form.querySelector('[data-firmware-error]');
        let message = 'Could not start firmware updates. Review and try again.';
        try { message = JSON.parse(event.detail.xhr.responseText).message || message; } catch { /* Network failure. */ }
        error.textContent = message;
        error.hidden = false;
        error.scrollIntoView({block: 'nearest'});
      }
    }
  });

  if (dialog) {
    dialog.addEventListener('cancel', event => {
      if (content.querySelector('.htmx-request')) event.preventDefault();
    });
    dialog.addEventListener('close', () => {
      wantsDialog = false;
      content.replaceChildren();
      const opener = document.getElementById(openerID) || document.getElementById('firmware-open-all');
      if (opener) { dismissTooltip(opener); opener.focus({preventScroll: true}); }
    });
  }

  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) htmx.trigger(document.body, 'firmware-progress');
  });
  for (const form of document.querySelectorAll('[data-firmware-form]')) updateSelection(form);
})();
