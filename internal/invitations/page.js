'use strict';
const registrationEnabled = document.body.dataset.registration === 'true';
if(registrationEnabled){document.querySelector('#invite-section').hidden=true;document.querySelector('#invitation-list-section').hidden=true;}
const form = document.querySelector('#invite');
const notice = document.querySelector('#notice');
const list = document.querySelector('#invitations');
const loading = document.querySelector('#loading');
const send = document.querySelector('#send');
const roles = {member:'Members can run Work and read their own evidence in this project.',viewer:'Viewers can read project evidence. They cannot submit Work.',admin:'Admins can run Work and read project evidence, and manage Member and Viewer invitations. Only the Owner can invite an Admin.'};
document.querySelector('#role').addEventListener('change', e => { document.querySelector('#role-help').textContent = roles[e.target.value]; });
function accessPreset(container, account, projectRole, owner, label, advanced) {
 const wrapper=node('label','Access');const select=node('select');select.setAttribute('aria-label',label);
 for(const [value,title] of [['developer','Developer'],['readonly','Read-only'],...(owner?[['admin','Team administrator']]:[]),['custom','Custom (advanced)']]) {const option=node('option',title);option.value=value;select.append(option);}
 const help=node('small');
 function sync(){select.value=account.value==='member'&&projectRole.value==='member'?'developer':account.value==='member'&&projectRole.value==='viewer'?'readonly':account.value==='admin'&&projectRole.value==='admin'?'admin':'custom';help.textContent=`Account: ${account.value} · Selected project: ${projectRole.value}. No deployment or governance access included.`;}
 select.addEventListener('change',()=>{if(select.value==='custom'){advanced.open=true;return;}account.value=select.value==='admin'?'admin':'member';projectRole.value=select.value==='admin'?'admin':select.value==='readonly'?'viewer':'member';sync();});
 account.addEventListener('change',sync);projectRole.addEventListener('change',sync);sync();wrapper.append(select);container.append(wrapper,help);
}
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
   const project = document.createElement('small'); project.textContent = `Account: ${invite.accountRole || "member"} · ${invite.project}: ${invite.role}`;
   const status = document.createElement('span'); status.className = 'badge'; status.textContent = invite.status;
   detail.append(email,project);
   if(invite.linkManaged && ['pending','expired','reserved','cancelled'].includes(invite.status)) {
    const actions=node('div'); actions.className='invitation-actions';
    for(const action of (invite.status==='cancelled' ? ['reissue'] : ['cancel','reissue'])) {
     const button=node('button',action==='cancel'?'Cancel invitation':'Replace link'); button.className='quiet';button.type='button';button.setAttribute('aria-label',`${button.textContent} for ${invite.email}`);
     button.addEventListener('click',()=>manageInvitation(invite.id,action,button));actions.append(button);
    }
    detail.append(actions);
   }
   row.append(detail,status); list.append(row);
  }
  loading.textContent = items.length ? 'Pending invitations activate only after the invited person signs in. Uncertain delivery requires administrator review.' : 'No invitations yet.';
 } catch (error) { list.replaceChildren(); loading.textContent = error.message || 'Invitations are unavailable.'; }
}
form.addEventListener('submit', async e => {
 e.preventDefault(); send.disabled = true; notice.textContent = 'Creating invitation…';
 try {
  const response = await fetch('/api/invitations', {method:'POST',credentials:'same-origin',redirect:'error',headers:{'Content-Type':'application/json'},body:JSON.stringify({email:form.elements.namedItem('email').value.trim().toLowerCase(),project:form.elements.namedItem('project').value.trim(),role:form.elements.namedItem('role').value,accountRole:form.elements.namedItem('accountRole').value})});
  if (!response.ok) throw new Error(response.status === 403 ? 'Your role cannot send this invitation. Only the Owner can invite an Admin.' : 'The invitation was not confirmed. Refresh the list and review its status before trying again.');
  const invitation = await response.json();
  if (invitation.acceptanceUrl) {
   showInvitationLink(invitation.acceptanceUrl);
  } else {
   notice.textContent = 'Invitation sent. The recipient can follow the email to set up their account.';
  }
  form.elements.namedItem('email').value = '';
  await refresh();
 } catch (error) { notice.textContent = error.message || 'The invitation could not be confirmed.'; }
 finally { send.disabled = false; }
});
document.querySelector('#refresh').addEventListener('click',refresh);
if(!registrationEnabled) refresh();

const memberList = document.querySelector('#members');
const memberNotice = document.querySelector('#member-notice');
const memberLoading = document.querySelector('#members-loading');
function node(tag, text) { const el = document.createElement(tag); if (text) el.textContent = text; return el; }
async function changeMember(change, button) {
 button.disabled = true;
 memberNotice.textContent = 'Saving member access…';
 try {
  const response = await fetch('/api/members', {method:'POST', credentials:'same-origin', redirect:'error', headers:{'Content-Type':'application/json'}, body:JSON.stringify(change)});
  if (!response.ok) throw new Error(response.status === 403 ? 'Your current role cannot change this member.' : 'Change not confirmed. Refresh members before retrying.');
  memberNotice.textContent = change.action === 'disable' ? 'Member disabled. Existing login sessions no longer grant access.' : 'Member access updated.';
  await refreshMembers();
 } catch(error) { memberNotice.textContent = error.message || 'Member change failed.'; }
 finally { button.disabled = false; }
}
function roleForm(member, owner, project) {
 const form = node('form'); form.className = 'member-actions';
 const label = node('label', project ? `Project: ${project.project}` : 'Account role');
 const select = node('select'); select.setAttribute('aria-label', `${member.email || member.principal} ${project ? project.project + ' role' : 'account role'}`);
 const choices = project ? [['','No access'],['member','Developer'],['viewer','Read-only'],...(owner ? [['admin','Admin']] : [])] : [['member','Member'],['admin','Admin']];
 for(const [value,text] of choices) { const option = node('option',text); option.value = value; select.append(option); }
 select.value = project ? project.role : member.accountRole;
 label.append(select); const button = node('button','Save'); button.type = 'submit';
 form.append(label,button); form.addEventListener('submit',e=>{e.preventDefault();changeMember({principal:member.principal,action:project?'project-role':'account-role',role:select.value,...(project ? {project:project.project} : {})},button);});
 return form;
}
function dutyForm(member, team) {
 const form=node('form');form.className='member-actions';
 const who=member.email || member.principal;
 const role=node('select');role.setAttribute('aria-label',`Additional role for ${who}`);
 for(const [value,title] of [['governance-admin','Governance access'],['platform-operator','Deployment access']]) { const option=node('option',title);option.value=value;role.append(option); }
 const scope=node('select');scope.setAttribute('aria-label',`Role scope for ${who}`);
 const ref=node('input');ref.required=true;ref.maxLength=128;ref.setAttribute('aria-label',`Scope reference for ${who}`);
 function scopes() { scope.replaceChildren();for(const value of (role.value==='platform-operator'?['installation']:['project','account'])) {const option=node('option',value);option.value=value;scope.append(option);}reference(); }
 function reference() {ref.readOnly=scope.value==='account';ref.value=ref.readOnly?team:'';ref.placeholder=scope.value==='installation'?'Installed Platform name':'Existing project';}
 role.addEventListener('change',scopes);scope.addEventListener('change',reference);scopes();
 const button=node('button','Grant advanced permission');button.type='submit';
 form.append(role,scope,ref,button);
 form.addEventListener('submit',e=>{e.preventDefault();changeMember({principal:member.principal,action:'grant-role',role:role.value,scopeKind:scope.value,scopeRef:ref.value.trim()},button);});
 return form;
}
async function refreshMembers() {
 memberLoading.textContent = 'Loading members…';
 try {
  const [response, meResponse] = await Promise.all(['/api/members','/auth/me'].map(path=>fetch(path,{credentials:'same-origin',redirect:'error',cache:'no-store'})));
  if(!response.ok || !meResponse.ok) throw new Error('Member administration is unavailable for this account.');
  const rows = await response.json(); const me = await meResponse.json();
  if(!Array.isArray(rows)) throw new Error('Member list unavailable.');
  const isOwner = rows.some(m=>m.principal===me.subject && m.accountRole==='owner');
  const isAccountAdmin = isOwner || rows.some(m=>m.principal===me.subject && m.accountRole==='admin');
 if(registrationEnabled){document.querySelector('#access-section').hidden=!isAccountAdmin;if(isAccountAdmin){document.querySelector('#refresh-access').onclick=()=>refreshAccessRequests(isOwner);await refreshAccessRequests(isOwner);}}
 document.querySelector('#account-role').disabled=!isOwner;
 if(!isOwner) document.querySelector('#account-role').value='member';
 document.querySelector('#role option[value=admin]').disabled=!isOwner;
 const preset=document.querySelector('#invite-preset');preset.replaceChildren();accessPreset(preset,document.querySelector('#account-role'),document.querySelector('#role'),isOwner,'Access',document.querySelector('#invite-advanced'));
  memberList.replaceChildren();
  for(const member of rows) {
   const row = node('article'); row.className = 'member';
   row.append(node('h3',member.email || member.principal),node('p',`${member.accountRole} · ${member.active?'Active':'Disabled'}`));
   for(const grant of member.projects) row.append(node('small',`${grant.project} · ${grant.role==='member'?'Developer':grant.role==='viewer'?'Read-only':member.accountRole==='admin'?'Team administrator':'Project administrator'}`));
   const advanced=node('details');advanced.className='advanced-permissions';advanced.append(node('summary','Advanced permissions'));
   advanced.append(node('small','Deployment and governance duties are scoped separately. They do not grant AWS credentials or Work access.'));
   if((member.roles || []).length) row.append(node('small',`${member.roles.length} advanced permission(s) assigned`));
   for(const grant of member.roles || []) {
    const line=node('p',`${grant.role} · ${grant.scopeKind}: ${grant.scopeRef}`);
    if(isOwner && member.active) {const remove=node('button','Revoke role');remove.type='button';remove.addEventListener('click',()=>changeMember({principal:member.principal,action:'revoke-role',...grant},remove));line.append(remove);}
    advanced.append(line);
   }
   if(isOwner && member.active) advanced.append(dutyForm(member,me.team));
   const canManage = member.accountRole !== 'owner' && (isOwner || member.accountRole !== 'admin');
   if(canManage) {
    const details = node('details'); details.append(node('summary','Manage access'));
    if(isOwner) advanced.append(roleForm(member,true,null));
    if(isOwner && member.active && member.accountRole==='admin') {
     const transfer=node('form');transfer.className='member-actions';
     const label=node('label',`Transfer ownership: type ${member.email || member.principal}`);
     const confirm=node('input');confirm.required=true;confirm.autocomplete='off';confirm.setAttribute('aria-label',`Confirm ownership transfer to ${member.email || member.principal}`);label.append(confirm);
     const button=node('button','Transfer ownership');button.type='submit';transfer.append(label,button,node('small','You will become Account Admin. This member becomes the only Owner. Project grants stay unchanged.'));
     transfer.addEventListener('submit',e=>{e.preventDefault();if(confirm.value!==(member.email || member.principal)){memberNotice.textContent='Type the recipient exactly to confirm.';return;}changeMember({principal:member.principal,action:'transfer-owner',confirmation:member.principal},button);});advanced.append(transfer);
    }
    if(member.active) {
     for(const project of member.projects) if(isOwner || project.role!=='admin') details.append(roleForm(member,isOwner,project));
     const grantForm=node('form');grantForm.className='member-actions';
     const label=node('label','Add project membership');const input=node('input');input.required=true;input.maxLength=128;input.placeholder='Existing project';input.setAttribute('aria-label',`New project for ${member.email || member.principal}`);label.append(input);
     const grant=node('button','Add as Developer');grant.type='submit';grantForm.append(label,grant);
     grantForm.addEventListener('submit',e=>{e.preventDefault();changeMember({principal:member.principal,action:'project-role',project:input.value.trim(),role:'member'},grant);});details.append(grantForm);
    }
    if(isAccountAdmin) { const toggle = node('button',member.active?'Disable member':'Enable member'); toggle.type='button';toggle.className='quiet';
    toggle.addEventListener('click',()=>changeMember({principal:member.principal,action:member.active?'disable':'enable'},toggle)); details.append(toggle); } row.append(details);
   }
   row.append(advanced);memberList.append(row);
  }
  memberLoading.textContent = `${rows.length} members. Only the Owner can appoint or remove an account Admin.`;
 } catch(error) {memberList.replaceChildren();memberLoading.textContent=error.message || 'Members unavailable.';}
}
document.querySelector('#refresh-members').addEventListener('click',refreshMembers);
refreshMembers();

function showInvitationLink(value) {
 const link = new URL(value);
 if(link.origin!==window.location.origin || link.pathname!=='/login' || !/^#invite=[A-Za-z0-9_-]{43}$/.test(link.hash)) throw new Error('Invalid invitation link.');
 notice.textContent='Invitation created. Share this one-time link with the invited person. No email has been sent. ';
 const field=node('input');field.readOnly=true;field.value=link.href;field.setAttribute('aria-label','One-time invitation link');notice.append(field);
}
async function manageInvitation(id,action,button) {
 button.disabled=true;notice.textContent='Updating invitation…';
 try {
  const response=await fetch('/api/invitations/manage',{method:'POST',credentials:'same-origin',redirect:'error',headers:{'Content-Type':'application/json'},body:JSON.stringify({id,action})});
  if(!response.ok) throw new Error('Invitation change not confirmed. Refresh before retrying.');
  const result=await response.json();
  if(result.acceptanceUrl) showInvitationLink(result.acceptanceUrl);else notice.textContent='Invitation cancelled. Its previous link no longer grants access.';
  await refresh();
 }catch(error){notice.textContent=error.message || 'Invitation update failed.';}finally{button.disabled=false;}
}

async function refreshAccessRequests(isOwner) {
 const notice=document.querySelector('#access-notice');const list=document.querySelector('#access-requests');
 notice.textContent='Loading access requests…';
 try {
  const response=await fetch('/api/access-requests',{credentials:'same-origin',redirect:'error',cache:'no-store'});
  if(!response.ok) throw new Error('Account Owner or Account Admin access is required.');
  const requests=await response.json();list.replaceChildren();
  for(const request of requests){
   const row=node('article');row.className='member';row.append(node('h3',request.email));
   const form=node('form');form.className='member-actions';
   function select(label,choices){const wrapper=node('label',label);const field=node('select');field.setAttribute('aria-label',`${request.email} ${label}`);for(const [value,text] of choices){const opt=node('option',text);opt.value=value;field.append(opt);}wrapper.append(field);form.append(wrapper);return field;}
   const advanced=node('details');advanced.append(node('summary','Advanced role mapping'));
   const account=select('Account role',[['member','Member'],...(isOwner?[['admin','Account Admin']]:[])]);
   const label=node('label','Project');const project=node('input');project.required=true;project.maxLength=128;project.placeholder='Existing project';project.setAttribute('aria-label',`${request.email} project`);label.append(project);form.append(label);
   const role=select('Project role',[['member','Developer'],['viewer','Read-only'],...(isOwner?[['admin','Project Admin']]:[])]);
   advanced.append(account.parentElement,role.parentElement);const preset=node('div');accessPreset(preset,account,role,isOwner,`${request.email} Access`,advanced);form.append(preset,advanced);
   const approve=node('button','Approve access');approve.type='submit';const reject=node('button','Decline');reject.type='button';form.append(approve,reject);
   async function review(action){approve.disabled=true;reject.disabled=true;notice.textContent='Saving access decision…';try{
    const payload={id:request.id,action,...(action==='approve'?{accountRole:account.value,project:project.value.trim(),role:role.value}:{})};
    const result=await fetch('/api/access-requests',{method:'POST',credentials:'same-origin',redirect:'error',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)});
    if(!result.ok)throw new Error('Access decision not confirmed. Check your permissions and project, then refresh.');
    await refreshMembers();notice.textContent=action==='approve'?'Access approved. The user can sign in again through Portal or CLI.':'Access request declined.';
   }catch(error){notice.textContent=error.message;}finally{approve.disabled=false;reject.disabled=false;}}
   form.addEventListener('submit',e=>{e.preventDefault();review('approve');});reject.addEventListener('click',()=>review('reject'));row.append(form);list.append(row);
  }
  notice.textContent=requests.length?`${requests.length} pending requests.`:'No pending access requests.';
 }catch(error){list.replaceChildren();notice.textContent=error.message;}
}
