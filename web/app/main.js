// Entry point. Views are defined first and the stored theme applied, then
// defining <hzn-app> upgrades the one in index.html, which starts the app.
//
// Modules import each other through "horizon/…" specifiers only, never
// relative paths: the import map points each one at a content-hashed URL.

import "horizon/components/hzn-api-tokens.js";
import { HznApp, applyStoredTheme } from "horizon/components/hzn-app.js";
import "horizon/views/hzn-account.js";
import "horizon/views/hzn-forgot-password.js";
import "horizon/views/hzn-home.js";
import "horizon/views/hzn-invitation.js";
import "horizon/views/hzn-not-found.js";
import "horizon/views/hzn-project-home.js";
import "horizon/views/hzn-reset-password.js";
import "horizon/views/hzn-settings-general.js";
import "horizon/views/hzn-settings-members.js";
import "horizon/views/hzn-settings-project.js";
import "horizon/views/hzn-settings-projects.js";
import "horizon/views/hzn-settings-roles.js";
import "horizon/views/hzn-sign-in.js";
import "horizon/views/hzn-workspace-home.js";

applyStoredTheme();
customElements.define("hzn-app", HznApp);
