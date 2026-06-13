(function () {
  const arch = window.ARCH;
  const status = window.AGENOVA_STATUS || { subjects: {} };
  const state = { view: "map", selected: "claim", flow: 0 };

  const byId = (id) => document.getElementById(id);
  const esc = (value) => String(value || "").replace(/[&<>]/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;" }[char]));
  const nodeById = Object.fromEntries(arch.nodes.map((node) => [node.id, node]));

  function subjectStatus(id) {
    return (status.subjects && status.subjects[id] && status.subjects[id].status) || "reserved";
  }

  function subjectNote(id) {
    return (status.subjects && status.subjects[id]) || {};
  }

  function renderMeta() {
    byId("meta").innerHTML = [
      `<div><strong>${esc(status.phase || "unknown phase")}</strong></div>`,
      `<div>${esc(status.step || "status unavailable")}</div>`,
      `<div>Updated ${esc(status.updated || "unknown")}</div>`
    ].join("");
  }

  function renderMap() {
    const edgeLines = arch.edges.map(([from, to, label]) => {
      const a = nodeById[from];
      const b = nodeById[to];
      const x1 = a.x + a.w;
      const y1 = a.y + a.h / 2;
      const x2 = b.x;
      const y2 = b.y + b.h / 2;
      const mid = (x1 + x2) / 2;
      return `<path class="edge" d="M ${x1} ${y1} C ${mid} ${y1}, ${mid} ${y2}, ${x2} ${y2}"></path><text class="small" x="${mid - 42}" y="${Math.min(y1, y2) + Math.abs(y2 - y1) / 2 - 6}">${esc(label)}</text>`;
    }).join("");

    const domains = arch.domains.map((domain) => `
      <rect class="domain" x="${domain.x}" y="${domain.y}" width="${domain.w}" height="${domain.h}" rx="8"></rect>
      <text class="domain-title" x="${domain.x + 14}" y="${domain.y + 24}">${esc(domain.name)}</text>
      <text class="small" x="${domain.x + 14}" y="${domain.y + 42}">${esc(domain.tier)}</text>
    `).join("");

    const nodes = arch.nodes.map((node) => {
      const cls = `node ${subjectStatus(node.id)} ${state.selected === node.id ? "active" : ""}`;
      return `
        <g data-node="${node.id}">
          <rect class="${cls}" x="${node.x}" y="${node.y}" width="${node.w}" height="${node.h}" rx="8"></rect>
          <text class="label" x="${node.x + 12}" y="${node.y + 24}">${esc(node.name)}</text>
          <text class="small" x="${node.x + 12}" y="${node.y + 42}">${esc(subjectStatus(node.id))}</text>
        </g>`;
    }).join("");

    byId("mapView").innerHTML = `
      <svg viewBox="0 0 1160 780" role="img" aria-label="Agenova architecture map">
        <defs><marker id="arrow" markerWidth="8" markerHeight="8" refX="7" refY="3" orient="auto"><path d="M0,0 L0,6 L7,3 z" fill="#667085"></path></marker></defs>
        ${domains}${edgeLines}${nodes}
      </svg>`;

    byId("mapView").querySelectorAll("[data-node]").forEach((el) => {
      el.addEventListener("click", () => {
        state.selected = el.dataset.node;
        render();
      });
    });
  }

  function renderFlow() {
    byId("flowView").innerHTML = arch.flows.map((step, index) => `
      <div class="flow-step ${index === state.flow ? "active" : ""}" data-flow="${index}">
        <strong>${index + 1}. ${esc(step.title)}</strong>
        <p>${esc(step.body)}</p>
        <div>${step.subjects.map((id) => `<span class="badge">${esc(nodeById[id].name)}</span>`).join("")}</div>
      </div>`).join("");

    byId("flowView").querySelectorAll("[data-flow]").forEach((el) => {
      el.addEventListener("click", () => {
        state.flow = Number(el.dataset.flow);
        state.selected = arch.flows[state.flow].subjects[0];
        render();
      });
    });
  }

  function renderPhase() {
    byId("phaseView").innerHTML = arch.phases.map((phase) => `
      <div class="flow-step">
        <strong>${esc(phase.name)}</strong> <span class="badge">${esc(phase.status)}</span>
        <p>${esc(phase.scope)}</p>
      </div>`).join("");
  }

  function renderDetail() {
    const node = nodeById[state.selected] || arch.nodes[0];
    const current = subjectNote(node.id);
    byId("detail").innerHTML = `
      <span class="badge">${esc(subjectStatus(node.id))}</span>
      <h2>${esc(node.name)}</h2>
      <p>${esc(node.desc)}</p>
      ${current.note ? `<p><strong>Status note:</strong> ${esc(current.note)}</p>` : ""}
      ${current.evidence ? `<p><strong>Evidence:</strong> ${esc(current.evidence)}</p>` : ""}
      ${node.example ? `<h3>Example</h3><pre>${esc(node.example)}</pre>` : ""}
    `;
  }

  function setView(view) {
    state.view = view;
    ["map", "flow", "phase"].forEach((name) => {
      byId(`${name}View`).classList.toggle("hidden", name !== view);
    });
    document.querySelectorAll(".tabs button").forEach((button) => {
      button.classList.toggle("active", button.dataset.view === view);
    });
    renderDetail();
  }

  function render() {
    renderMeta();
    renderMap();
    renderFlow();
    renderPhase();
    setView(state.view);
  }

  document.querySelectorAll(".tabs button").forEach((button) => {
    button.addEventListener("click", () => setView(button.dataset.view));
  });

  render();
}());
