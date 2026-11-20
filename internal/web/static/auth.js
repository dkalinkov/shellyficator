const passwordToggle = document.querySelector('[data-password-toggle]');
if (passwordToggle) {
  const password = document.getElementById(passwordToggle.getAttribute('aria-controls'));
  if (password) {
    passwordToggle.hidden = false;
    passwordToggle.addEventListener('click', () => {
      const visible = password.type === 'password';
      password.type = visible ? 'text' : 'password';
      passwordToggle.dataset.visible = String(visible);
      passwordToggle.setAttribute('aria-label', visible ? 'Hide password' : 'Show password');
    });
  }
}
