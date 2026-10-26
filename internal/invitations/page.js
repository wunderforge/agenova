'use strict';
document.addEventListener('pointerdown',()=>{document.documentElement.dataset.inputModality='pointer';},true);
document.addEventListener('keydown',event=>{if(event.key==='Tab')document.documentElement.dataset.inputModality='keyboard';},true);
const titles={'platform-admin':'Admin','platform-operator':'Platform Operator',member:'Member'};
const registrationEnabled=document.body.dataset.registration==='true';
const memberList=document.querySelector('#members');
const memberNotice=document.querySelector('#member-notice');
function node(tag,text){const n=document.createElement(tag);if(text!==undefined)n.textContent=text;return n;}
function operatorRules(rules){return (rules||[]).map(rule=>{const copy=structuredClone(rule);if(typeof copy.constraints?.maxTimeout==='number')copy.constraints.maxTimeout=(copy.constraints.maxTimeout/1e9)+'s';return copy;});}
function roleOf(m){
 if(m.accountRole==='owner')return 'owner';
 if(m.accountRole==='admin')return 'platform-admin';
 if(m.accountRole==='member' && !m.projects.length && m.roles?.length && m.roles.every(g=>g.role==='platform-operator'))return 'platform-operator';
 if(m.accountRole==='member' && !(m.roles||[]).length)return 'member';
 return 'legacy';
}
function selector(label,owner,current){
 const s=node('select');s.setAttribute('aria-label',label);
 for(const value of ['member','platform-operator',...(owner?['platform-admin']:[])]){const o=node('option',titles[value]);o.value=value;s.append(o);}
 if(current==='legacy'){const o=node('option','Select a role to replace legacy permissions');o.value='';o.disabled=true;s.prepend(o);s.value='';}else s.value=current||'member';
 s.required=true;return s;
}
async function post(path,payload){const r=await fetch(path,{method:'POST',credentials:'same-origin',redirect:'error',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)});if(!r.ok)throw new Error('Change not confirmed. Check permissions and refresh before retrying.');}
async function changeMember(change,button){button.disabled=true;memberNotice.textContent='Saving…';try{await post('/api/members',change);document.querySelector('#member-editor').close();await refreshMembers();memberNotice.textContent='User access updated. Only the selected operation was applied.';}catch(e){const notice=document.querySelector('#member-editor-content [role=status]');if(notice)notice.textContent=e.message;memberNotice.textContent=e.message;}finally{button.disabled=false;}}
async function refreshMembers(){
 const loading=document.querySelector('#members-loading');loading.textContent='Loading users…';
 try{
  const [r,mr]=await Promise.all(['/api/members','/auth/me'].map(p=>fetch(p,{credentials:'same-origin',redirect:'error',cache:'no-store'})));
  if(!r.ok||!mr.ok)throw new Error('Administrator access is required.');
  const rows=await r.json(),me=await mr.json();if(!Array.isArray(rows))throw new Error('Users unavailable.');
  const actor=rows.find(m=>m.principal===me.subject),owner=actor?.accountRole==='owner',admin=owner||actor?.accountRole==='admin';
  memberList.replaceChildren();
  const table=node('table');table.className='users-table';const head=node('thead'),headRow=node('tr');for(const title of ['User','Platform role','Projects','Status','Action'])headRow.append(node('th',title));head.append(headRow);const body=node('tbody');table.append(head,body);memberList.append(table);
  if(!document.querySelector('#governance-project').value)document.querySelector('#governance-project').value=actor?.projects.find(g=>g.role==='admin')?.project||'';
  for(const m of rows){
   const who=m.email||m.principal,role=roleOf(m),row=node('article');row.className='member';
   row.append(node('h3',who),node('p',`${role==='owner'?'Owner':titles[role]||'Legacy permissions — review required'} · ${m.active?'Active':'Disabled'}`));
   row.append(node('small',role==='owner'||role==='platform-admin'?'Configuration access: all organization projects. Work Policy still applies.':m.projects.length?'Projects: '+m.projects.map(g=>g.project).join(', '):'No business project access'));
   if(role==='platform-operator')row.append(node('small','Installations: '+m.roles.map(g=>g.scopeRef).join(', ')));
   if(m.accountRole!=='owner' && admin && m.principal!==me.subject){
    const form=node('form');form.className='member-actions';
    const select=selector(`Role for ${who}`,admin,role);
    const project=node('input');project.setAttribute('aria-label',`Project for ${who}`);project.placeholder='Existing project';project.required=true;project.maxLength=128;project.value=m.projects[0]?.project||actor.projects[0]?.project||'';
    const save=node('button','Save role');save.type='submit';save.disabled=!m.active;
    function roleFields(){project.hidden=select.value!=='member';project.required=select.value==='member';}
    select.addEventListener('change',roleFields);roleFields();
    form.append(select,project,save,node('small','Role changes replace operational duties. Member requires an initial project; Admin inherits all organization projects.'));
    form.addEventListener('submit',e=>{e.preventDefault();changeMember({principal:m.principal,action:'platform-role',role:select.value,...(select.value==='member'?{project:project.value.trim()}:{})},save);});row.append(form);
    const access=node('div');access.className='project-access';access.append(node('h4','Project access · independent grants'));
    for(const grant of m.projects){
     const grantForm=node('form');grantForm.className='member-actions';const label=node('strong',grant.project),roleSelect=node('select');roleSelect.setAttribute('aria-label','Project role '+grant.project+' for '+who);
     for(const value of ['member','viewer','admin','']){const option=node('option',value||'Remove access');option.value=value;roleSelect.append(option);}roleSelect.value=grant.role;
     const saveGrant=node('button','Save project access');saveGrant.type='submit';grantForm.append(label,roleSelect,saveGrant);grantForm.addEventListener('submit',e=>{e.preventDefault();changeMember({principal:m.principal,action:'project-role',project:grant.project,role:roleSelect.value},saveGrant);});access.append(grantForm);
    }
    const add=node('form');add.className='member-actions';const addProject=node('select');addProject.setAttribute('aria-label','Add project for '+who);
    for(const name of [...new Set([...knownProjects,...rows.flatMap(person=>person.projects.map(g=>g.project))])].filter(name=>!m.projects.some(g=>g.project===name))){const o=node('option',name);o.value=name;addProject.append(o);}
    const addRole=node('select');addRole.setAttribute('aria-label','New project role for '+who);for(const name of ['member','viewer','admin']){const o=node('option',name);addRole.append(o);}
    const addButton=node('button','Add project access');addButton.type='submit';addButton.disabled=!addProject.options.length;add.append(addProject,addRole,addButton);add.addEventListener('submit',e=>{e.preventDefault();changeMember({principal:m.principal,action:'project-role',project:addProject.value,role:addRole.value},addButton);});access.append(add);
    if(role==='platform-admin'){row.append(node('p','Project access · All organization projects (inherited)'));}
    else if(role==='platform-operator'){row.append(node('p','Installation duties only. No business project access.'));}
    else{row.append(access);}
    const toggle=node('button',m.active?'Disable user':'Enable user');toggle.type='button';toggle.addEventListener('click',()=>changeMember({principal:m.principal,action:m.active?'disable':'enable'},toggle));row.append(toggle);
   }
   const summary=node('tr');summary.dataset.search=(who+' '+m.principal+' '+m.projects.map(g=>g.project).join(' ')).toLowerCase();
   summary.append(node('td',who),node('td',role==='owner'?'Owner':titles[role]||'Scoped roles'),node('td',role==='owner'||role==='platform-admin'?'All organization projects':m.projects.map(g=>g.project+' · '+g.role).join(', ')||'No project access'),node('td',m.active?'Active':'Disabled'));
   const action=node('td'),edit=node('button',role==='owner'?'View':'Edit');edit.type='button';edit.setAttribute('aria-label',(role==='owner'?'View ':'Edit ')+who);edit.addEventListener('click',()=>{document.querySelector('#member-editor-title').textContent=(role==='owner'?'User details':'Edit user')+' · '+who;const content=document.querySelector('#member-editor-content');content.replaceChildren(row);const status=node('p');status.setAttribute('role','status');content.append(status);document.querySelector('#member-editor').showModal();});action.append(edit);summary.append(action);body.append(summary);
  }
  if(!document.querySelector('#governance-project').options.length || !document.querySelector('#governance-project').value){const names=[...new Set(rows.flatMap(m=>m.projects.map(g=>g.project)))];const el=document.querySelector('#governance-project');el.replaceChildren();for(const name of names){const o=node('option',name);el.append(o);}}
  loading.textContent=`${rows.length} users. Select a person to inspect or edit their access.`;
  document.querySelector('#access-section').hidden=!registrationEnabled||!admin||location.hash!=='#access-section';
  if(registrationEnabled&&admin){document.querySelector('#refresh-access').onclick=()=>refreshAccessRequests(admin);await refreshAccessRequests(admin);}
 }catch(e){memberList.replaceChildren();loading.textContent=e.message;}
}
async function refreshAccessRequests(admin){
 const notice=document.querySelector('#access-notice'),list=document.querySelector('#access-requests');notice.textContent='Loading requests…';
 try{const r=await fetch('/api/access-requests',{credentials:'same-origin',redirect:'error',cache:'no-store'});if(!r.ok)throw new Error('Access requests unavailable.');const requests=await r.json();list.replaceChildren();
 for(const request of requests){const row=node('article');row.className='member';row.append(node('h3',request.email));const form=node('form');form.className='member-actions';
 const role=selector(`Role for ${request.email}`,admin,'member'),project=node('input');project.required=true;project.maxLength=128;project.placeholder='Existing project';project.setAttribute('aria-label',`Project for ${request.email}`);
 const approve=node('button','Approve access');approve.type='submit';const reject=node('button','Decline');reject.type='button';form.append(role,project,approve,reject);
 async function review(action){approve.disabled=true;reject.disabled=true;try{await post('/api/access-requests',{id:request.id,action,...(action==='approve'?{platformRole:role.value,project:project.value.trim()}:{})});await refreshMembers();notice.textContent='Access decision saved. The user can sign in again after approval.';}catch(e){notice.textContent=e.message;}finally{approve.disabled=false;reject.disabled=false;}}
 form.addEventListener('submit',e=>{e.preventDefault();review('approve');});reject.addEventListener('click',()=>review('reject'));row.append(form);list.append(row);}
 notice.textContent=requests.length?`${requests.length} pending requests.`:'No pending access requests.';
 }catch(e){list.replaceChildren();notice.textContent=e.message;}
}
document.querySelector('#refresh-members').addEventListener('click',refreshMembers);
document.querySelector('#member-editor-close').addEventListener('click',()=>document.querySelector('#member-editor').close());
document.querySelector('#user-search').addEventListener('input',event=>{const term=event.target.value.toLowerCase();memberList.querySelectorAll('tbody tr').forEach(row=>row.hidden=!row.dataset.search.includes(term));});
function showAdminSection(){const id=location.hash.slice(1)||'member-section';for(const name of ['organization','member-section','access-section','governance']){const section=document.getElementById(name);section.hidden=name!==id;}document.querySelectorAll('.access-nav a').forEach(a=>a.setAttribute('aria-current',a.hash===('#'+id)?'page':'false'));}
window.addEventListener('hashchange',showAdminSection);showAdminSection();
// Initial refresh waits until all configuration state has been initialized.

// The editor only displays configuration. Authorization and validation stay on the server.
const gp=document.querySelector('#governance-project'),gk=document.querySelector('#governance-kind'),gt=document.querySelector('#governance-template'),gd=document.querySelector('#governance-document'),gs=document.querySelector('#governance-save'),gl=document.querySelector('#governance-load'),gn=document.querySelector('#governance-notice');
let loadedGovernance=null,governanceBusy=false,expectedActive,reviewedDocument='',originalDocument='';
let knownProjects=[];
function setProjectOptions(projects){knownProjects=[...new Set(projects)];const before=gp.value;gp.replaceChildren();for(const project of knownProjects){const o=node('option',project);o.value=project;gp.append(o);}if(knownProjects.includes(before))gp.value=before;}

function invalidateGovernance(){reviewedDocument='';document.querySelector('#governance-preview').hidden=true;loadedGovernance=null;gd.disabled=true;gs.disabled=true;gd.value='';document.querySelector('#governance-source').textContent='';document.querySelector('#governance-template-label').hidden=gk.value!=='template';gn.textContent='Load the selected configuration before editing.';}
[gp,gk,gt].forEach(input=>input.addEventListener('input',invalidateGovernance));
function governanceLock(busy){governanceBusy=busy;[gp,gk,gt,gl].forEach(e=>e.disabled=busy);gs.disabled=busy||!loadedGovernance;gd.disabled=busy||!loadedGovernance;}
gl.addEventListener('click',async()=>{
 if(governanceBusy)return;
 const project=gp.value.trim(),kind=gk.value,template=gt.value.trim();if(!project){gn.textContent='Choose an existing project.';return;}
 invalidateGovernance();governanceLock(true);gn.textContent='Loading configuration…';
 try{const q=new URLSearchParams({project});if(kind==='template'&&template)q.set('template',template);
 const r=await fetch('/api/setup?'+q,{credentials:'same-origin',redirect:'error',cache:'no-store'});if(!r.ok)throw new Error('Configuration unavailable. Check project access and template name.');const setup=await r.json();if(setup.project!==project)throw new Error('Project selection mismatch. Nothing loaded.');
 const doc=kind==='policy'?{apiVersion:'agenova.io/v1alpha1',kind:'PolicyBundle',metadata:{name:setup.policy.ID,version:setup.policy.Version},spec:{rules:operatorRules(setup.policy.Rules)}}:setup.template;
 if(!doc?.metadata?.name)throw new Error('No registered configuration found.');
 gd.value=JSON.stringify(doc,null,2);originalDocument=gd.value;loadedGovernance={project,kind};
 if(kind==='policy')await loadPolicyHistory(project);document.querySelector('#governance-source').textContent='Loaded '+doc.metadata.name+(doc.metadata.version?' · version '+doc.metadata.version:'')+' from '+project+'.';gn.textContent='Ready to edit. Use a new policy version or template name for changed content.';
 }catch(e){gn.textContent=e.message;}finally{governanceLock(false);}
});
document.querySelector('#governance-form').addEventListener('submit',async e=>{
 e.preventDefault();if(governanceBusy||!loadedGovernance)return;
 let doc;try{doc=JSON.parse(gd.value);}catch{gn.textContent='Invalid JSON. Correct the document before saving.';return;}
 if(doc?.kind!==(loadedGovernance.kind==='policy'?'PolicyBundle':'AgentTemplate')){gn.textContent='Document kind does not match the selected type.';return;}
 if(typeof doc.metadata?.name!=='string'){gn.textContent='Document metadata.name is required.';return;}
 const before=JSON.parse(originalDocument||'{}');const changes=[];
 function diff(a,b,path){if(JSON.stringify(a)===JSON.stringify(b))return;if(a&&b&&typeof a==='object'&&typeof b==='object'&&!Array.isArray(a)&&!Array.isArray(b)){for(const key of new Set([...Object.keys(a),...Object.keys(b)]))diff(a[key],b[key],path?path+'.'+key:key);}else changes.push(path+'\n  Before: '+(JSON.stringify(a)??'(not set)')+'\n  After:  '+(JSON.stringify(b)??'(removed)'));}
 diff(before,doc,'');reviewedDocument=gd.value;document.querySelector('#governance-diff').textContent='Selected project: '+loadedGovernance.project+'\nChanges compared with the document loaded for editing:\n\n'+(changes.join('\n\n')||'No content changes. Publishing may reactivate this immutable version.')+'\n\nComplete candidate:\n'+JSON.stringify(doc,null,2);document.querySelector('#governance-preview').hidden=false;gn.textContent='Review the changes and complete candidate above. No changes have been applied.';
});
gd.addEventListener('input',()=>{reviewedDocument='';document.querySelector('#governance-preview').hidden=true;});
document.querySelector('#governance-confirm').addEventListener('click',async()=>{
 if(governanceBusy||!loadedGovernance||!reviewedDocument||reviewedDocument!==gd.value)return;
 const doc=JSON.parse(reviewedDocument);
 if(loadedGovernance.kind==='policy'&&expectedActive===undefined){gn.textContent='Reload Policy history before publishing. Concurrency state is unavailable.';return;}
 const target={...loadedGovernance};governanceLock(true);gn.textContent='Saving configuration…';
 try{const r=await fetch('/api/governance',{method:'POST',credentials:'same-origin',redirect:'error',headers:{'Content-Type':'application/json'},body:JSON.stringify({...target,document:gd.value,...(target.kind==='policy'?{expectedActive:expectedActive||{id:'',version:''}}:{})})});
 if(!r.ok)throw new Error(r.status===409?'Version conflict. Use a new policy version or template name; existing content was not overwritten.':r.status===403?'Permission denied. Platform Admin access to this project is required.':r.status===400?'Configuration rejected. Check fields, project rules and the compatible worker image/entrypoint.':'Save not confirmed. Reload the configuration before retrying.');
 const result=await r.json();gn.textContent='Configuration saved'+(target.kind==='policy'?' and activated for new Work.':'. Select this template in new Work and ensure Policy allows it.');document.querySelector('#governance-source').textContent='Saved '+(result.name||doc.metadata.name)+' in '+target.project+'.';reviewedDocument='';document.querySelector('#governance-preview').hidden=true;if(target.kind==='policy')await loadPolicyHistory(target.project);
 }catch(e){gn.textContent=e.message;}finally{governanceLock(false);}
});

async function loadPolicyHistory(project){
 expectedActive=undefined;const host=document.querySelector('#governance-history');host.replaceChildren();
 try{const r=await fetch('/api/governance?'+new URLSearchParams({project}),{credentials:'same-origin',redirect:'error',cache:'no-store'});if(!r.ok)throw new Error('Policy history unavailable. Reload before publishing.');const history=await r.json();expectedActive=history.active||null;
 host.append(node('h3','Immutable versions'));
 for(const entry of history.versions||[]){const button=node('button',entry.policy.ID+' / '+entry.policy.Version+(entry.active?' · Active':' · Review for reactivation'));button.type='button';button.addEventListener('click',()=>{if(governanceBusy)return;gk.value='policy';loadedGovernance={project,kind:'policy'};gd.disabled=false;gs.disabled=false;gd.value=JSON.stringify({apiVersion:'agenova.io/v1alpha1',kind:'PolicyBundle',metadata:{name:entry.policy.ID,version:entry.policy.Version},spec:{rules:operatorRules(entry.policy.Rules)}},null,2);reviewedDocument='';document.querySelector('#governance-preview').hidden=true;gn.textContent='Historical document loaded. Review before reactivating; existing Work is unchanged.';});host.append(button);}
 const audit=node('details'),title=node('summary','Activation audit');audit.append(title);for(const event of history.events||[])audit.append(node('p',event.at+' · '+event.actor+' · '+event.target));host.append(audit);
 }catch(e){host.append(node('p',e.message));}
}
async function loadOrganization(){
 const notice=document.querySelector('#organization-notice');try{const r=await fetch('/api/organization',{credentials:'same-origin',redirect:'error',cache:'no-store'});if(!r.ok)throw new Error('Organization metadata is unavailable.');const value=await r.json();document.querySelector('#organization-id').textContent='Immutable organization ID: '+value.id;document.querySelector('#organization-name').value=value.displayName;document.querySelector('#organization-description').value=value.description||'';document.querySelector('#organization-save').disabled=false;}catch(e){notice.textContent=e.message;}
}
document.querySelector('#organization-form').addEventListener('submit',async e=>{e.preventDefault();const button=document.querySelector('#organization-save');button.disabled=true;try{await post('/api/organization',{displayName:document.querySelector('#organization-name').value,description:document.querySelector('#organization-description').value});document.querySelector('#organization-notice').textContent='Display information saved. Identity and permissions are unchanged.';}catch(e){document.querySelector('#organization-notice').textContent=e.message;}finally{button.disabled=false;}});
void refreshMembers();void loadOrganization();
fetch('/api/setup',{credentials:'same-origin',redirect:'error',cache:'no-store'}).then(r=>r.ok?r.json():null).then(async setup=>{if(setup?.projects?.length){setProjectOptions(setup.projects);await refreshMembers();}}).catch(()=>{});
