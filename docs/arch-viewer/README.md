# Architecture Viewer

This is a static, local architecture viewer for Agenova.

Open `index.html` directly in a browser. The page reads:

- `data.js`: stable architecture subjects, relationships, flows, and examples;
- `../status/implementation-status.js`: current implementation status.

The viewer is documentation. It must not become a runtime dependency or a source of product truth separate from the phase docs.

## Tabs

- **Architecture**: SVG diagram with region boundaries (Application Layer, Agenova Core, Execution Substrate), nodes color-coded by ownership kind, and click-only detail popups showing YAML/pseudocode examples. Hover only highlights the node border; it does not open detail panels.
- **Run Path**: Step-by-step execution walkthrough; clicking a step highlights the corresponding nodes in the Architecture view.
- **Ownership**: Table mapping areas to owners (Application, Agenova core, Agenova interface, Runtime dependency, Reserved future) with rationale.
- **Status**: Phase targets and verification requirements.

## Layout

The SVG viewBox is `0 0 1160 720`. At a 1280 px viewport the diagram renders without scrolling and no node text overlaps.

To package a standalone folder for static hosting:

```powershell
.\docs\arch-viewer\package.ps1 -Clean
```

Publish the contents of `dist/arch-viewer-site/` with any static hosting provider and use `index.html` as the index document.
