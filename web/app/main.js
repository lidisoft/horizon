// Entry point. Views are defined first and the stored theme applied, then
// defining <hzn-app> upgrades the one in index.html, which starts the app.
//
// Modules import each other through "horizon/…" specifiers only, never
// relative paths: the import map points each one at a content-hashed URL.

import { HznApp, applyStoredTheme } from "horizon/components/hzn-app.js";
import "horizon/views/hzn-home.js";
import "horizon/views/hzn-not-found.js";

applyStoredTheme();
customElements.define("hzn-app", HznApp);
