// Invitation proof stays in the fragment until an explicit same-origin login POST.
// It is never sent to Google or retained in browser storage.
(() => {
  const form = document.querySelector('form[action="/login"]');
  function readInvitation() {
    const fragment = window.location.hash;
    window.history.replaceState(null, '', window.location.pathname);
    form.querySelector('input[name="invitation"]')?.remove();
    form.querySelector('[role="alert"]')?.remove();
    form.querySelector('button').disabled = false;
    if (!fragment) return;
    if (!/^#invite=[A-Za-z0-9_-]{43}$/.test(fragment)) {
      form.querySelector('button').disabled = true;
      const error = document.createElement('p');
      error.setAttribute('role', 'alert');
      error.textContent = 'Invalid invitation. Ask your administrator for a new invitation.';
      form.append(error);
      return;
    }
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = 'invitation';
    input.value = fragment.slice(8);
    form.append(input);
  }
  readInvitation();
  window.addEventListener('hashchange', readInvitation);
})();
