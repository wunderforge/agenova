# Architecture Viewer

This is a static, local architecture viewer for Agenova.

Open `index.html` directly in a browser. The page reads:

- `data.js`: stable architecture subjects, relationships, flows, and examples;
- `../status/implementation-status.js`: current implementation status.

The viewer is documentation. It must not become a runtime dependency or a source of product truth separate from the phase docs.

To package a standalone folder for static hosting:

```powershell
.\docs\arch-viewer\package.ps1 -Clean
```

Publish the contents of `dist/arch-viewer-site/` with any static hosting provider and use `index.html` as the index document.
