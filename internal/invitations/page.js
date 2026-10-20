'use strict';
const titles={'platform-admin':'Platform Admin','platform-operator':'Platform Operator',member:'Member'};
const registrationEnabled=document.body.dataset.registration==='true';
const memberList=document.querySelector('#members');
const memberNotice=document.querySelector('#member-notice');
function node(tag,text){const n=document.createElement(tag);if(text!==undefined)n.textContent=text;return n;}
function roleOf(m){
 if(m.accountRole==='owner')return 'owner';
 if(m.accountRole==='admin' && m.projects.length && m.projects.every(g=>g.role==='admin'))return 'platform-admin';
 if(m.accountRole==='member' && !m.projects.length && m.roles?.length && m.roles.every(g=>g.role==='platform-operator'))return 'platform-operator';
 if(m.accountRole==='member' && m.projects.every(g=>g.role==='member') && !(m.roles||[]).length)return 'member';
 return 'legacy';
}
function selector(label,owner,current){
 const s=node('select');s.setAttribute('aria-label',label);
 for(const value of ['member','platform-operator',...(owner?['platform-admin']:[])]){const o=node('option',titles[value]);o.value=value;s.append(o);}
 if(current==='legacy'){const o=node('option','Select a role to replace legacy permissions');o.value='';o.disabled=true;s.prepend(o);s.value='';}else s.value=current||'member';
 s.required=true;return s;
}
async function post(path,payload){const r=await fetch(path,{method:'POST',credentials:'same-origin',redirect:'error',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)});if(!r.ok)throw new Error('Change not confirmed. Check permissions and refresh before retrying.');}
async function changeMember(change,button){button.disabled=true;memberNotice.textContent='Saving…';try{await post('/api/members',change);await refreshMembers();memberNotice.textContent='User access updated.';}catch(e){memberNotice.textContent=e.message;}finally{button.disabled=false;}}
async function refreshMembers(){
 const loading=document.querySelector('#members-loading');loading.textContent='Loading users…';
 try{
  const [r,mr]=await Promise.all(['/api/members','/auth/me'].map(p=>fetch(p,{credentials:'same-origin',redirect:'error',cache:'no-store'})));
  if(!r.ok||!mr.ok)throw new Error('Administrator access is required.');
  const rows=await r.json(),me=await mr.json();if(!Array.isArray(rows))throw new Error('Users unavailable.');
  const actor=rows.find(m=>m.principal===me.subject),owner=actor?.accountRole==='owner',admin=owner||actor?.accountRole==='admin';
  memberList.replaceChildren();
  if(!document.querySelector('#governance-project').value)document.querySelector('#governance-project').value=actor?.projects.find(g=>g.role==='admin')?.project||'';
  for(const m of rows){
   const who=m.email||m.principal,role=roleOf(m),row=node('article');row.className='member';
   row.append(node('h3',who),node('p',`${role==='owner'?'Owner':titles[role]||'Legacy permissions — review required'} · ${m.active?'Active':'Disabled'}`));
   row.append(node('small',m.projects.length?'Projects: '+m.projects.map(g=>g.project).join(', '):'No business project access'));
   if(role==='platform-operator')row.append(node('small','Installations: '+m.roles.map(g=>g.scopeRef).join(', ')));
   if(m.accountRole!=='owner' && admin && m.principal!==me.subject){
    const form=node('form');form.className='member-actions';
    const select=selector(`Role for ${who}`,admin,role);
    const project=node('input');project.setAttribute('aria-label',`Project for ${who}`);project.placeholder='Existing project';project.required=true;project.maxLength=128;project.value=m.projects[0]?.project||actor.projects[0]?.project||'';
    const save=node('button','Save role');save.type='submit';save.disabled=!m.active;
    form.append(select,project,save,node('small','Changing role replaces legacy duties. Operators receive deployment access only; the project identifies the administration scope.'));
    form.addEventListener('submit',e=>{e.preventDefault();changeMember({principal:m.principal,action:'platform-role',role:select.value,project:project.value.trim()},save);});row.append(form);
    const toggle=node('button',m.active?'Disable user':'Enable user');toggle.type='button';toggle.addEventListener('click',()=>changeMember({principal:m.principal,action:m.active?'disable':'enable'},toggle));if(owner || m.accountRole!=='admin' && !(m.roles||[]).length)row.append(toggle);
   }
   memberList.append(row);
  }
  loading.textContent=`${rows.length} users. Owner and Platform Admins can update roles. Owner and your own role are protected.`;
  document.querySelector('#access-section').hidden=!registrationEnabled||!admin;
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
refreshMembers();

// The editor only displays configuration. Authorization and validation stay on the server.
const gp=document.querySelector('#governance-project'),gk=document.querySelector('#governance-kind'),gt=document.querySelector('#governance-template'),gd=document.querySelector('#governance-document'),gs=document.querySelector('#governance-save'),gl=document.querySelector('#governance-load'),gn=document.querySelector('#governance-notice');
let loadedGovernance=null,governanceBusy=false;
function invalidateGovernance(){loadedGovernance=null;gd.disabled=true;gs.disabled=true;gd.value='';document.querySelector('#governance-source').textContent='';document.querySelector('#governance-template-label').hidden=gk.value!=='template';gn.textContent='Load the selected configuration before editing.';}
[gp,gk,gt].forEach(input=>input.addEventListener('input',invalidateGovernance));
function governanceLock(busy){governanceBusy=busy;[gp,gk,gt,gl].forEach(e=>e.disabled=busy);gs.disabled=busy||!loadedGovernance;gd.disabled=busy||!loadedGovernance;}
gl.addEventListener('click',async()=>{
 if(governanceBusy)return;
 const project=gp.value.trim(),kind=gk.value,template=gt.value.trim();if(!project){gn.textContent='Choose an existing project.';return;}
 invalidateGovernance();governanceLock(true);gn.textContent='Loading configuration…';
 try{const q=new URLSearchParams({project});if(kind==='template'&&template)q.set('template',template);
 const r=await fetch('/api/setup?'+q,{credentials:'same-origin',redirect:'error',cache:'no-store'});if(!r.ok)throw new Error('Configuration unavailable. Check project access and template name.');const setup=await r.json();if(setup.project!==project)throw new Error('Project selection mismatch. Nothing loaded.');
 const doc=kind==='policy'?{apiVersion:'agenova.io/v1alpha1',kind:'PolicyBundle',metadata:{name:setup.policy.ID,version:setup.policy.Version},spec:{rules:setup.policy.Rules}}:setup.template;
 if(!doc?.metadata?.name)throw new Error('No registered configuration found.');
 gd.value=JSON.stringify(doc,null,2);loadedGovernance={project,kind};document.querySelector('#governance-source').textContent='Loaded '+doc.metadata.name+(doc.metadata.version?' · version '+doc.metadata.version:'')+' from '+project+'.';gn.textContent='Ready to edit. Use a new policy version or template name for changed content.';
 }catch(e){gn.textContent=e.message;}finally{governanceLock(false);}
});
document.querySelector('#governance-form').addEventListener('submit',async e=>{
 e.preventDefault();if(governanceBusy||!loadedGovernance)return;
 let doc;try{doc=JSON.parse(gd.value);}catch{gn.textContent='Invalid JSON. Correct the document before saving.';return;}
 if(doc?.kind!==(loadedGovernance.kind==='policy'?'PolicyBundle':'AgentTemplate')){gn.textContent='Document kind does not match the selected type.';return;}
 const target={...loadedGovernance};governanceLock(true);gn.textContent='Saving configuration…';
 try{const r=await fetch('/api/governance',{method:'POST',credentials:'same-origin',redirect:'error',headers:{'Content-Type':'application/json'},body:JSON.stringify({...target,document:gd.value})});
 if(!r.ok)throw new Error(r.status===409?'Version conflict. Use a new policy version or template name; existing content was not overwritten.':r.status===403?'Permission denied. Platform Admin access to this project is required.':r.status===400?'Configuration rejected. Check fields, project rules and the compatible worker image/entrypoint.':'Save not confirmed. Reload the configuration before retrying.');
 const result=await r.json();gn.textContent='Configuration saved'+(target.kind==='policy'?' and activated for new Work.':'. Select this template in new Work and ensure Policy allows it.');document.querySelector('#governance-source').textContent='Saved '+(result.name||doc.metadata.name)+' in '+target.project+'.';
 }catch(e){gn.textContent=e.message;}finally{governanceLock(false);}
});
