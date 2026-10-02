// Native anchors and forms still work without this page's enhancements.
function revealChannelNote() {
  let id;
  try { id = decodeURIComponent(window.location.hash.slice(1)); } catch { return; }
  const field = document.getElementById(id);
  const editor = field?.closest('.detail-note-editor');
  if (!editor) return;
  editor.open = true;
  requestAnimationFrame(() => {
    field.scrollIntoView({block: 'center'});
    field.focus({preventScroll: true});
  });
}
window.addEventListener('hashchange', revealChannelNote);
revealChannelNote();

const sectionLinks = [...document.querySelectorAll('.detail-nav a')];
if ('IntersectionObserver' in window) {
  const observer = new IntersectionObserver(entries => {
    const visible = entries.filter(entry => entry.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
    if (!visible.length) return;
    for (const link of sectionLinks) {
      if (link.hash === '#' + visible[0].target.id) link.setAttribute('aria-current', 'location');
      else link.removeAttribute('aria-current');
    }
  }, {rootMargin: '-48px 0px -55% 0px'});
  function observeSections() {
    observer.disconnect();
    for (const link of sectionLinks) {
      const section = document.querySelector(link.hash);
      if (section) observer.observe(section);
    }
  }
  observeSections();
  document.addEventListener('htmx:afterSettle', observeSections);
}

document.addEventListener('htmx:afterSettle', () => {
  const heading = document.getElementById('device-heading');
  if (heading) initializeLabelColors(heading);
});

// Disable only after native validation, preserving hidden CSRF fields.
document.addEventListener('submit', event => {
  if (event.defaultPrevented) return;
  if (!event.target.matches('[data-device-action]')) return;
  const button = event.submitter;
  if (!button) return;
  button.disabled = true;
  button.setAttribute('aria-busy', 'true');
});
window.addEventListener('pageshow', () => {
  for (const button of document.querySelectorAll('[data-device-action] button')) {
    button.disabled = false;
    button.removeAttribute('aria-busy');
  }
});
