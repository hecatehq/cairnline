import { defineComponents } from "./define.js";

// Runtime entry: register the custom elements so the <project-status-app> mounted
// in index.html upgrades and drives the ext-apps handshake. Registration is all
// the production bundle needs; the element wires up the host bridge itself.
defineComponents();

// Preview-only fixture injector. import.meta.env.DEV is statically false in the
// production build, so the dynamic import is dead-code-eliminated and never ships
// in dist/project-status.html — the embedded bundle carries no preview UI.
if (import.meta.env.DEV) {
  void import("./preview.js").then((module) => {
    module.mountPreview();
  });
}
