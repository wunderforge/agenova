(function () {
  const arch = window.ARCH;
  const status = window.AGENOVA_STATUS || { subjects: {} };
  const state = { view: "architecture", selected: null, runStep: 0 };

  const byId = (id) => document.getElementById(id);
  const esc = (value) => String(value || "").replace(/[&<>]/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;" }[char]));
  const nodeById = Object.fromEntries(arch.nodes.map((node) => [node.id, node]));
  const ownerByKey = Object.fromEntries(arch.ownership.map((item) => [item.key, item]));

  function subjectStatus(id) {
    return (status.subjects && status.subjects[id] && status.subjects[id].status) || "";
  }

  function subjectNote(id) {
    return (status.subjects && status.subjects[id]) || {};
  }

  function ownerLabel(kind) {
    return ownerByKey[kind] ? ownerByKey[kind].label : kind;
  }

  function renderMeta() {
    byId("meta").innerHTML = [
      `<strong>${esc(status.phase || "architecture")}</strong>`,
      esc(status.step || "Agenova runtime architecture"),
      `Updated ${esc(status.updated || "unknown")}`
    ].map((line) => `<div>${line}</div>`).join("");
  }

  function renderValueStrip() {
    byId("valueStrip").innerHTML = `
      <strong>${esc(arch.headline)}</strong>
      ${arch.value.map((item) => `<span>${esc(item)}</span>`).join("")}
    `;
  }

  function renderLegend() {
    return `<div class="legend">${arch.ownership.map((item) => `
      <button class="legend-item ${esc(item.key)}" data-owner="${esc(item.key)}" title="${esc(item.body)}">
        <span></span>${esc(item.label)}
      </button>`).join("")}</div>`;
  }

  function edgePath(from, to) {
    const a = nodeById[from];
    const b = nodeById[to];
    const x1 = a.x + a.w;
    const y1 = a.y + a.h / 2;
    const x2 = b.x;
    const y2 = b.y + b.h / 2;

    if (x1 <= x2) {
      const mid = x1 + (x2 - x1) / 2;
      return { d: `M ${x1} ${y1} C ${mid} ${y1}, ${mid} ${y2}, ${x2} ${y2}`, lx: mid - 38, ly: y1 + (y2 - y1) / 2 - 8 };
    }

    const startX = a.x + a.w / 2;
    const startY = a.y + a.h;
    const endX = b.x + b.w / 2;
    const endY = b.y;
    const midY = startY + (endY - startY) / 2;
    return { d: `M ${startX} ${startY} C ${startX} ${midY}, ${endX} ${midY}, ${endX} ${endY}`, lx: Math.min(startX, endX) + Math.abs(endX - startX) / 2 - 34, ly: midY - 8 };
  }

  function renderArchitecture() {
    const selectedStep = arch.runPath[state.runStep] || { subjects: [] };
    const highlighted = new Set(selectedStep.subjects || []);

    const regionRects = arch.regions.map((region) => `
      <rect class="region ${esc(region.kind)}" x="${region.x}" y="${region.y}" width="${region.w}" height="${region.h}" rx="10"></rect>
    `).join("");

    const regionLabels = arch.regions.map((region) => `
      <text class="region-title" x="${region.x + 18}" y="${region.y + 28}">${esc(region.title)}</text>
      <text class="region-subtitle" x="${region.x + 18}" y="${region.y + 48}">${esc(region.subtitle)}</text>
    `).join("");

    const edges = arch.edges.map(([from, to, label]) => {
      const active = highlighted.has(from) && highlighted.has(to);
      return `
        <path class="edge ${active ? "active" : ""}" d="${edgePath(from, to).d}">
          <title>${esc(nodeById[from].name)} -> ${esc(nodeById[to].name)}: ${esc(label)}</title>
        </path>
      `;
    }).join("");

    const nodes = arch.nodes.map((node) => {
      const active = state.selected === node.id;
      const inStep = highlighted.has(node.id);
      return `
        <g class="node-wrap ${active ? "selected" : ""} ${inStep ? "in-step" : ""}" data-node="${esc(node.id)}">
          <rect class="node ${esc(node.kind)}" x="${node.x}" y="${node.y}" width="${node.w}" height="${node.h}" rx="8"></rect>
          <text class="label" x="${node.x + 12}" y="${node.y + 24}">${esc(node.name)}</text>
          <text class="node-kind" x="${node.x + 12}" y="${node.y + 46}">${esc(ownerLabel(node.kind))}</text>
        </g>`;
    }).join("");

    byId("architectureView").innerHTML = `
      ${renderLegend()}
      <svg viewBox="0 0 1160 720" role="img" aria-label="Agenova runtime architecture">
        <defs>
          <marker id="arrow" markerWidth="9" markerHeight="9" refX="8" refY="3" orient="auto">
            <path d="M0,0 L0,6 L8,3 z" fill="#64748b"></path>
          </marker>
        </defs>
        ${regionRects}
        ${edges}
        ${nodes}
        ${regionLabels}
      </svg>`;

    byId("architectureView").querySelectorAll("[data-node]").forEach((el) => {
      el.addEventListener("click", () => {
        state.selected = el.dataset.node;
        render();
      });
    });
  }

  function renderRunPath() {
    byId("runView").innerHTML = `
      ${renderLegend()}
      <div class="run-layout">
        ${arch.runPath.map((step, index) => `
          <button class="run-step ${index === state.runStep ? "active" : ""}" data-run-step="${index}">
            <strong>${esc(step.title)}</strong>
            <span>${esc(step.body)}</span>
            <small>${step.subjects.map((id) => esc(nodeById[id].name)).join(" -> ")}</small>
          </button>`).join("")}
      </div>`;

    byId("runView").querySelectorAll("[data-run-step]").forEach((el) => {
      el.addEventListener("click", () => {
        state.runStep = Number(el.dataset.runStep);
        state.selected = arch.runPath[state.runStep].subjects[0];
        state.view = "architecture";
        render();
      });
    });
  }

  function renderOwnership() {
    byId("ownershipView").innerHTML = `
      ${renderLegend()}
      <div class="ownership-table">
        ${arch.ownershipRows.map((row) => {
          const key = arch.ownership.find((item) => item.label === row.owner)?.key || "";
          return `
            <div class="ownership-row">
              <strong>${esc(row.area)}</strong>
              <span class="pill ${esc(key)}">${esc(row.owner)}</span>
              <p>${esc(row.why)}</p>
            </div>`;
        }).join("")}
      </div>`;
  }

  function renderPhase() {
    byId("phaseView").innerHTML = `
      <div class="phase-list">
        ${arch.phases.map((phase) => `
          <section class="phase-item">
            <span class="pill interface">${esc(phase.status)}</span>
            <h3>${esc(phase.name)}</h3>
            <p>${esc(phase.scope)}</p>
          </section>`).join("")}
      </div>`;
  }

  function renderDetail() {
    if (!state.selected) {
      byId("detail").classList.add("collapsed");
      byId("detail").innerHTML = "";
      return;
    }
    byId("detail").classList.remove("collapsed");
    const node = nodeById[state.selected];
    const current = subjectNote(node.id);
    const owner = ownerByKey[node.kind];
    byId("detail").innerHTML = `
      <span class="pill ${esc(node.kind)}">${esc(ownerLabel(node.kind))}</span>
      ${subjectStatus(node.id) ? `<span class="pill status-pill">${esc(subjectStatus(node.id))}</span>` : ""}
      <button class="detail-close" aria-label="Collapse details">x</button>
      <h2>${esc(node.name)}</h2>
      <p>${esc(node.desc)}</p>
      ${node.note ? `<p><strong>Architecture meaning:</strong> ${esc(node.note)}</p>` : ""}
      ${owner ? `<p><strong>Boundary:</strong> ${esc(owner.body)}</p>` : ""}
      ${current.note ? `<p><strong>Implementation note:</strong> ${esc(current.note)}</p>` : ""}
      ${current.evidence ? `<p><strong>Evidence:</strong> ${esc(current.evidence)}</p>` : ""}
      ${node.example ? `<h3>Example</h3><pre><code>${esc(node.example)}</code></pre>` : ""}
    `;
  }

  function setView(view) {
    state.view = view;
    ["architecture", "run", "ownership", "phase"].forEach((name) => {
      byId(`${name}View`).classList.toggle("hidden", name !== view);
    });
    document.querySelectorAll(".tabs button").forEach((button) => {
      button.classList.toggle("active", button.dataset.view === view);
    });
  }

  function render() {
    renderMeta();
    renderValueStrip();
    renderArchitecture();
    renderRunPath();
    renderOwnership();
    renderPhase();
    renderDetail();
    setView(state.view);
  }

  document.querySelectorAll(".tabs button").forEach((button) => {
    button.addEventListener("click", () => {
      state.view = button.dataset.view;
      render();
    });
  });

  byId("detail").addEventListener("click", (event) => {
    if (event.target && event.target.classList.contains("detail-close")) {
      state.selected = null;
      render();
    }
  });

  render();
}());
