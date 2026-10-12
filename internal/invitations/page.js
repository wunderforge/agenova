'use strict';
const form = document.querySelector('#invite');
const notice = document.querySelector('#notice');
const list = document.querySelector('#invitations');
const loading = document.querySelector('#loading');
const send = document.querySelector('#send');
const roles = {member:'Members can run Work and read their own evidence in this project.',viewer:'Viewers can read project evidence. They cannot submit Work.',admin:'Admins can run Work and read project evidence, and manage Member and Viewer invitations. Only the Owner can invite an Admin.'};
document.querySelector('#role').addEventListener('change', e => { document.querySelector('#role-help').textContent = roles[e.target.value]; });
async function refresh() {
 loading.textContent = 'Loading invitations…';
 try {
  const response = await fetch('/api/invitations', {credentials:'same-origin',redirect:'error',cache:'no-store'});
  if (!response.ok) throw new Error(response.status === 403 ? 'Only an Owner or account Admin can manage invitations.' : 'Sign in with an active administrator account to view invitations.');
  const items = await response.json();
  if (!Array.isArray(items)) throw new Error('Invitations are unavailable.');
  list.replaceChildren();
  for (const invite of items) {
   const row = document.createElement('li');
   const detail = document.createElement('div');
   const email = document.createElement('strong'); email.textContent = invite.email;
   const project = document.createElement('small'); project.textContent = `${invite.project} · ${invite.role}`;
   const status = document.createElement('span'); status.className = 'badge'; status.textContent = invite.status;
   detail.append(email,project); row.append(detail,status); list.append(row);
  }
  loading.textContent = items.length ? 'Sent invitations activate after the recipient signs in. Uncertain delivery requires administrator review.' : 'No invitations yet.';
 } catch (error) { list.replaceChildren(); loading.textContent = error.message || 'Invitations are unavailable.'; }
}
form.addEventListener('submit', async e => {
 e.preventDefault(); send.disabled = true; notice.textContent = 'Sending invitation…';
 try {
  const response = await fetch('/api/invitations', {method:'POST',credentials:'same-origin',redirect:'error',headers:{'Content-Type':'application/json'},body:JSON.stringify({email:form.elements.namedItem('email').value.trim().toLowerCase(),project:form.elements.namedItem('project').value.trim(),role:form.elements.namedItem('role').value})});
  if (!response.ok) throw new Error(response.status === 403 ? 'Your role cannot send this invitation. Only the Owner can invite an Admin.' : 'The invitation was not confirmed. Refresh the list and review its status before trying again.');
  notice.textContent = 'Invitation sent. The recipient can follow the email to set up their account.';
  form.elements.namedItem('email').value = '';
  await refresh();
 } catch (error) { notice.textContent = error.message || 'The invitation could not be confirmed.'; }
 finally { send.disabled = false; }
});
document.querySelector('#refresh').addEventListener('click',refresh);
refresh();
