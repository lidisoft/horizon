/* @ds-bundle: {"format": 4, "namespace": "LDS", "components": [{"name": "Alert"}, {"name": "Avatar"}, {"name": "AvatarGroup"}, {"name": "Badge"}, {"name": "Banner"}, {"name": "Breadcrumb"}, {"name": "Button"}, {"name": "Card"}, {"name": "Checkbox"}, {"name": "Code"}, {"name": "Combobox"}, {"name": "Command"}, {"name": "DatePicker"}, {"name": "DateRange"}, {"name": "Divider"}, {"name": "Drawer"}, {"name": "EmptyState"}, {"name": "Field"}, {"name": "Icon"}, {"name": "Input"}, {"name": "Kbd"}, {"name": "Menu"}, {"name": "Modal"}, {"name": "Nav"}, {"name": "Navbar"}, {"name": "Pagination"}, {"name": "Popover"}, {"name": "Progress"}, {"name": "RadioGroup"}, {"name": "Segmented"}, {"name": "Select"}, {"name": "Skeleton"}, {"name": "Spinner"}, {"name": "Switch"}, {"name": "Table"}, {"name": "Tabs"}, {"name": "Tag"}, {"name": "Textarea"}, {"name": "Timeline"}, {"name": "Toast"}, {"name": "Tooltip"}]} */
/* Claude Design: inline the LDS icon sprite so <lds-icon> needs no network fetch
 * (LDS.ensureIcons skips fetching when #lds-icon-sprite exists). */
(function () {
  if (typeof document === 'undefined' || document.getElementById('lds-icon-sprite')) return;
  var wrap = document.createElement('div');
  wrap.innerHTML = "<svg xmlns=\"http://www.w3.org/2000/svg\" style=\"display:none\">\n  <symbol id=\"lds-icon-alert-circle\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><circle cx=\"12\" cy=\"12\" r=\"10\" />\n  <line x1=\"12\" x2=\"12\" y1=\"8\" y2=\"12\" />\n  <line x1=\"12\" x2=\"12.01\" y1=\"16\" y2=\"16\" /></symbol>\n  <symbol id=\"lds-icon-alert-triangle\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3\" />\n  <path d=\"M12 9v4\" />\n  <path d=\"M12 17h.01\" /></symbol>\n  <symbol id=\"lds-icon-archive\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><rect width=\"20\" height=\"5\" x=\"2\" y=\"3\" rx=\"1\" />\n  <path d=\"M4 8v11a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8\" />\n  <path d=\"M10 12h4\" /></symbol>\n  <symbol id=\"lds-icon-arrow-up-down\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"m21 16-4 4-4-4\" />\n  <path d=\"M17 20V4\" />\n  <path d=\"m3 8 4-4 4 4\" />\n  <path d=\"M7 4v16\" /></symbol>\n  <symbol id=\"lds-icon-at-sign\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><circle cx=\"12\" cy=\"12\" r=\"4\" />\n  <path d=\"M16 8v5a3 3 0 0 0 6 0v-1a10 10 0 1 0-4 8\" /></symbol>\n  <symbol id=\"lds-icon-bell\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M10.268 21a2 2 0 0 0 3.464 0\" />\n  <path d=\"M3.262 15.326A1 1 0 0 0 4 17h16a1 1 0 0 0 .74-1.673C19.41 13.956 18 12.499 18 8A6 6 0 0 0 6 8c0 4.499-1.411 5.956-2.738 7.326\" /></symbol>\n  <symbol id=\"lds-icon-bookmark\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M17 3a2 2 0 0 1 2 2v15a1 1 0 0 1-1.496.868l-4.512-2.578a2 2 0 0 0-1.984 0l-4.512 2.578A1 1 0 0 1 5 20V5a2 2 0 0 1 2-2z\" /></symbol>\n  <symbol id=\"lds-icon-calendar\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M8 2v4\" />\n  <path d=\"M16 2v4\" />\n  <rect width=\"18\" height=\"18\" x=\"3\" y=\"4\" rx=\"2\" />\n  <path d=\"M3 10h18\" /></symbol>\n  <symbol id=\"lds-icon-calendar-days\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M8 2v4\" />\n  <path d=\"M16 2v4\" />\n  <rect width=\"18\" height=\"18\" x=\"3\" y=\"4\" rx=\"2\" />\n  <path d=\"M3 10h18\" />\n  <path d=\"M8 14h.01\" />\n  <path d=\"M12 14h.01\" />\n  <path d=\"M16 14h.01\" />\n  <path d=\"M8 18h.01\" />\n  <path d=\"M12 18h.01\" />\n  <path d=\"M16 18h.01\" /></symbol>\n  <symbol id=\"lds-icon-chart-column\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M3 3v16a2 2 0 0 0 2 2h16\" />\n  <path d=\"M18 17V9\" />\n  <path d=\"M13 17V5\" />\n  <path d=\"M8 17v-3\" /></symbol>\n  <symbol id=\"lds-icon-chart-gantt\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M10 6h8\" />\n  <path d=\"M12 16h6\" />\n  <path d=\"M3 3v16a2 2 0 0 0 2 2h16\" />\n  <path d=\"M8 11h7\" /></symbol>\n  <symbol id=\"lds-icon-check\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M20 6 9 17l-5-5\" /></symbol>\n  <symbol id=\"lds-icon-chevron-down\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"m6 9 6 6 6-6\" /></symbol>\n  <symbol id=\"lds-icon-chevron-left\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"m15 18-6-6 6-6\" /></symbol>\n  <symbol id=\"lds-icon-chevron-right\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"m9 18 6-6-6-6\" /></symbol>\n  <symbol id=\"lds-icon-chevron-up\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"m18 15-6-6-6 6\" /></symbol>\n  <symbol id=\"lds-icon-circle\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><circle cx=\"12\" cy=\"12\" r=\"10\" /></symbol>\n  <symbol id=\"lds-icon-circle-check\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><circle cx=\"12\" cy=\"12\" r=\"10\" />\n  <path d=\"m9 12 2 2 4-4\" /></symbol>\n  <symbol id=\"lds-icon-circle-dashed\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M10.1 2.182a10 10 0 0 1 3.8 0\" />\n  <path d=\"M13.9 21.818a10 10 0 0 1-3.8 0\" />\n  <path d=\"M17.609 3.721a10 10 0 0 1 2.69 2.7\" />\n  <path d=\"M2.182 13.9a10 10 0 0 1 0-3.8\" />\n  <path d=\"M20.279 17.609a10 10 0 0 1-2.7 2.69\" />\n  <path d=\"M21.818 10.1a10 10 0 0 1 0 3.8\" />\n  <path d=\"M3.721 6.391a10 10 0 0 1 2.7-2.69\" />\n  <path d=\"M6.391 20.279a10 10 0 0 1-2.69-2.7\" /></symbol>\n  <symbol id=\"lds-icon-circle-dot\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><circle cx=\"12\" cy=\"12\" r=\"10\" />\n  <circle cx=\"12\" cy=\"12\" r=\"1\" /></symbol>\n  <symbol id=\"lds-icon-circle-x\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><circle cx=\"12\" cy=\"12\" r=\"10\" />\n  <path d=\"m15 9-6 6\" />\n  <path d=\"m9 9 6 6\" /></symbol>\n  <symbol id=\"lds-icon-copy\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><rect width=\"14\" height=\"14\" x=\"8\" y=\"8\" rx=\"2\" ry=\"2\" />\n  <path d=\"M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2\" /></symbol>\n  <symbol id=\"lds-icon-corner-down-right\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"m15 10 5 5-5 5\" />\n  <path d=\"M4 4v7a4 4 0 0 0 4 4h12\" /></symbol>\n  <symbol id=\"lds-icon-external-link\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M15 3h6v6\" />\n  <path d=\"M10 14 21 3\" />\n  <path d=\"M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6\" /></symbol>\n  <symbol id=\"lds-icon-eye\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M2.062 12.348a1 1 0 0 1 0-.696 10.75 10.75 0 0 1 19.876 0 1 1 0 0 1 0 .696 10.75 10.75 0 0 1-19.876 0\" />\n  <circle cx=\"12\" cy=\"12\" r=\"3\" /></symbol>\n  <symbol id=\"lds-icon-filter\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M10 20a1 1 0 0 0 .553.895l2 1A1 1 0 0 0 14 21v-7a2 2 0 0 1 .517-1.341L21.74 4.67A1 1 0 0 0 21 3H3a1 1 0 0 0-.742 1.67l7.225 7.989A2 2 0 0 1 10 14z\" /></symbol>\n  <symbol id=\"lds-icon-flag\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M4 22V4a1 1 0 0 1 .4-.8A6 6 0 0 1 8 2c3 0 5 2 7.333 2q2 0 3.067-.8A1 1 0 0 1 20 4v10a1 1 0 0 1-.4.8A6 6 0 0 1 16 16c-3 0-5-2-8-2a6 6 0 0 0-4 1.528\" /></symbol>\n  <symbol id=\"lds-icon-folder\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z\" /></symbol>\n  <symbol id=\"lds-icon-git-branch\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M15 6a9 9 0 0 0-9 9V3\" />\n  <circle cx=\"18\" cy=\"6\" r=\"3\" />\n  <circle cx=\"6\" cy=\"18\" r=\"3\" /></symbol>\n  <symbol id=\"lds-icon-git-commit-horizontal\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><circle cx=\"12\" cy=\"12\" r=\"3\" />\n  <line x1=\"3\" x2=\"9\" y1=\"12\" y2=\"12\" />\n  <line x1=\"15\" x2=\"21\" y1=\"12\" y2=\"12\" /></symbol>\n  <symbol id=\"lds-icon-git-merge\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><circle cx=\"18\" cy=\"18\" r=\"3\" />\n  <circle cx=\"6\" cy=\"6\" r=\"3\" />\n  <path d=\"M6 21V9a9 9 0 0 0 9 9\" /></symbol>\n  <symbol id=\"lds-icon-git-pull-request\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><circle cx=\"18\" cy=\"18\" r=\"3\" />\n  <circle cx=\"6\" cy=\"6\" r=\"3\" />\n  <path d=\"M13 6h3a2 2 0 0 1 2 2v7\" />\n  <line x1=\"6\" x2=\"6\" y1=\"9\" y2=\"21\" /></symbol>\n  <symbol id=\"lds-icon-grip-vertical\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><circle cx=\"9\" cy=\"12\" r=\"1\" />\n  <circle cx=\"9\" cy=\"5\" r=\"1\" />\n  <circle cx=\"9\" cy=\"19\" r=\"1\" />\n  <circle cx=\"15\" cy=\"12\" r=\"1\" />\n  <circle cx=\"15\" cy=\"5\" r=\"1\" />\n  <circle cx=\"15\" cy=\"19\" r=\"1\" /></symbol>\n  <symbol id=\"lds-icon-history\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8\" />\n  <path d=\"M3 3v5h5\" />\n  <path d=\"M12 7v5l4 2\" /></symbol>\n  <symbol id=\"lds-icon-inbox\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><polyline points=\"22 12 16 12 14 15 10 15 8 12 2 12\" />\n  <path d=\"M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z\" /></symbol>\n  <symbol id=\"lds-icon-info\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><circle cx=\"12\" cy=\"12\" r=\"10\" />\n  <path d=\"M12 16v-4\" />\n  <path d=\"M12 8h.01\" /></symbol>\n  <symbol id=\"lds-icon-kanban\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M5 3v14\" />\n  <path d=\"M12 3v8\" />\n  <path d=\"M19 3v18\" /></symbol>\n  <symbol id=\"lds-icon-keyboard\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M10 8h.01\" />\n  <path d=\"M12 12h.01\" />\n  <path d=\"M14 8h.01\" />\n  <path d=\"M16 12h.01\" />\n  <path d=\"M18 8h.01\" />\n  <path d=\"M6 8h.01\" />\n  <path d=\"M7 16h10\" />\n  <path d=\"M8 12h.01\" />\n  <rect width=\"20\" height=\"16\" x=\"2\" y=\"4\" rx=\"2\" /></symbol>\n  <symbol id=\"lds-icon-layers\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83z\" />\n  <path d=\"M2 12a1 1 0 0 0 .58.91l8.6 3.91a2 2 0 0 0 1.65 0l8.58-3.9A1 1 0 0 0 22 12\" />\n  <path d=\"M2 17a1 1 0 0 0 .58.91l8.6 3.91a2 2 0 0 0 1.65 0l8.58-3.9A1 1 0 0 0 22 17\" /></symbol>\n  <symbol id=\"lds-icon-link\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71\" />\n  <path d=\"M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71\" /></symbol>\n  <symbol id=\"lds-icon-list\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M3 5h.01\" />\n  <path d=\"M3 12h.01\" />\n  <path d=\"M3 19h.01\" />\n  <path d=\"M8 5h13\" />\n  <path d=\"M8 12h13\" />\n  <path d=\"M8 19h13\" /></symbol>\n  <symbol id=\"lds-icon-loader-circle\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M21 12a9 9 0 1 1-6.219-8.56\" /></symbol>\n  <symbol id=\"lds-icon-lock\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><rect width=\"18\" height=\"11\" x=\"3\" y=\"11\" rx=\"2\" ry=\"2\" />\n  <path d=\"M7 11V7a5 5 0 0 1 10 0v4\" /></symbol>\n  <symbol id=\"lds-icon-log-out\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"m16 17 5-5-5-5\" />\n  <path d=\"M21 12H9\" />\n  <path d=\"M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4\" /></symbol>\n  <symbol id=\"lds-icon-menu\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M4 5h16\" />\n  <path d=\"M4 12h16\" />\n  <path d=\"M4 19h16\" /></symbol>\n  <symbol id=\"lds-icon-message-square\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M22 17a2 2 0 0 1-2 2H6.828a2 2 0 0 0-1.414.586l-2.202 2.202A.71.71 0 0 1 2 21.286V5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2z\" /></symbol>\n  <symbol id=\"lds-icon-milestone\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M12 13v8\" />\n  <path d=\"M12 3v3\" />\n  <path d=\"M18.172 6a2 2 0 0 1 1.414.586l2.06 2.06a1.207 1.207 0 0 1 0 1.708l-2.06 2.06a2 2 0 0 1-1.414.586H4a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1z\" /></symbol>\n  <symbol id=\"lds-icon-minus\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M5 12h14\" /></symbol>\n  <symbol id=\"lds-icon-moon\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M20.985 12.486a9 9 0 1 1-9.473-9.472c.405-.022.617.46.402.803a6 6 0 0 0 8.268 8.268c.344-.215.825-.004.803.401\" /></symbol>\n  <symbol id=\"lds-icon-more-horizontal\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><circle cx=\"12\" cy=\"12\" r=\"1\" />\n  <circle cx=\"19\" cy=\"12\" r=\"1\" />\n  <circle cx=\"5\" cy=\"12\" r=\"1\" /></symbol>\n  <symbol id=\"lds-icon-octagon-alert\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M12 16h.01\" />\n  <path d=\"M12 8v4\" />\n  <path d=\"M15.312 2a2 2 0 0 1 1.414.586l4.688 4.688A2 2 0 0 1 22 8.688v6.624a2 2 0 0 1-.586 1.414l-4.688 4.688a2 2 0 0 1-1.414.586H8.688a2 2 0 0 1-1.414-.586l-4.688-4.688A2 2 0 0 1 2 15.312V8.688a2 2 0 0 1 .586-1.414l4.688-4.688A2 2 0 0 1 8.688 2z\" /></symbol>\n  <symbol id=\"lds-icon-paperclip\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"m16 6-8.414 8.586a2 2 0 0 0 2.829 2.829l8.414-8.586a4 4 0 1 0-5.657-5.657l-8.379 8.551a6 6 0 1 0 8.485 8.485l8.379-8.551\" /></symbol>\n  <symbol id=\"lds-icon-plus\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M5 12h14\" />\n  <path d=\"M12 5v14\" /></symbol>\n  <symbol id=\"lds-icon-repeat\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"m17 2 4 4-4 4\" />\n  <path d=\"M3 11v-1a4 4 0 0 1 4-4h14\" />\n  <path d=\"m7 22-4-4 4-4\" />\n  <path d=\"M21 13v1a4 4 0 0 1-4 4H3\" /></symbol>\n  <symbol id=\"lds-icon-rocket\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M12 15v5s3.03-.55 4-2c1.08-1.62 0-5 0-5\" />\n  <path d=\"M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09\" />\n  <path d=\"M9 12a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.4 22.4 0 0 1-4 2z\" />\n  <path d=\"M9 12H4s.55-3.03 2-4c1.62-1.08 5 .05 5 .05\" /></symbol>\n  <symbol id=\"lds-icon-search\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"m21 21-4.34-4.34\" />\n  <circle cx=\"11\" cy=\"11\" r=\"8\" /></symbol>\n  <symbol id=\"lds-icon-settings\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M9.671 4.136a2.34 2.34 0 0 1 4.659 0 2.34 2.34 0 0 0 3.319 1.915 2.34 2.34 0 0 1 2.33 4.033 2.34 2.34 0 0 0 0 3.831 2.34 2.34 0 0 1-2.33 4.033 2.34 2.34 0 0 0-3.319 1.915 2.34 2.34 0 0 1-4.659 0 2.34 2.34 0 0 0-3.32-1.915 2.34 2.34 0 0 1-2.33-4.033 2.34 2.34 0 0 0 0-3.831A2.34 2.34 0 0 1 6.35 6.051a2.34 2.34 0 0 0 3.319-1.915\" />\n  <circle cx=\"12\" cy=\"12\" r=\"3\" /></symbol>\n  <symbol id=\"lds-icon-signal-high\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M2 20h.01\" />\n  <path d=\"M7 20v-4\" />\n  <path d=\"M12 20v-8\" />\n  <path d=\"M17 20V8\" /></symbol>\n  <symbol id=\"lds-icon-signal-low\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M2 20h.01\" />\n  <path d=\"M7 20v-4\" /></symbol>\n  <symbol id=\"lds-icon-signal-medium\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M2 20h.01\" />\n  <path d=\"M7 20v-4\" />\n  <path d=\"M12 20v-8\" /></symbol>\n  <symbol id=\"lds-icon-sliders-horizontal\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M10 5H3\" />\n  <path d=\"M12 19H3\" />\n  <path d=\"M14 3v4\" />\n  <path d=\"M16 17v4\" />\n  <path d=\"M21 12h-9\" />\n  <path d=\"M21 19h-5\" />\n  <path d=\"M21 5h-7\" />\n  <path d=\"M8 10v4\" />\n  <path d=\"M8 12H3\" /></symbol>\n  <symbol id=\"lds-icon-sun\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><circle cx=\"12\" cy=\"12\" r=\"4\" />\n  <path d=\"M12 2v2\" />\n  <path d=\"M12 20v2\" />\n  <path d=\"m4.93 4.93 1.41 1.41\" />\n  <path d=\"m17.66 17.66 1.41 1.41\" />\n  <path d=\"M2 12h2\" />\n  <path d=\"M20 12h2\" />\n  <path d=\"m6.34 17.66-1.41 1.41\" />\n  <path d=\"m19.07 4.93-1.41 1.41\" /></symbol>\n  <symbol id=\"lds-icon-table\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M12 3v18\" />\n  <rect width=\"18\" height=\"18\" x=\"3\" y=\"3\" rx=\"2\" />\n  <path d=\"M3 9h18\" />\n  <path d=\"M3 15h18\" /></symbol>\n  <symbol id=\"lds-icon-tag\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M12.586 2.586A2 2 0 0 0 11.172 2H4a2 2 0 0 0-2 2v7.172a2 2 0 0 0 .586 1.414l8.704 8.704a2.426 2.426 0 0 0 3.42 0l6.58-6.58a2.426 2.426 0 0 0 0-3.42z\" />\n  <circle cx=\"7.5\" cy=\"7.5\" r=\".5\" fill=\"currentColor\" /></symbol>\n  <symbol id=\"lds-icon-trash-2\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M10 11v6\" />\n  <path d=\"M14 11v6\" />\n  <path d=\"M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6\" />\n  <path d=\"M3 6h18\" />\n  <path d=\"M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2\" /></symbol>\n  <symbol id=\"lds-icon-user\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2\" />\n  <circle cx=\"12\" cy=\"7\" r=\"4\" /></symbol>\n  <symbol id=\"lds-icon-users\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2\" />\n  <path d=\"M16 3.128a4 4 0 0 1 0 7.744\" />\n  <path d=\"M22 21v-2a4 4 0 0 0-3-3.87\" />\n  <circle cx=\"9\" cy=\"7\" r=\"4\" /></symbol>\n  <symbol id=\"lds-icon-x\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"M18 6 6 18\" />\n  <path d=\"m6 6 12 12\" /></symbol>\n</svg>";
  var svg = wrap.firstElementChild;
  svg.id = 'lds-icon-sprite';
  svg.setAttribute('aria-hidden', 'true');
  svg.style.cssText = 'position:absolute;width:0;height:0;overflow:hidden';
  document.documentElement.appendChild(svg);
})();
/*!
 * LiDiSoft Design System — lds.js
 * version 0.0.5
 *
 * GENERATED FILE — DO NOT EDIT.
 * Regenerate with: make build   (see docs/architecture.md)
 */
(function (global) {
  "use strict";

  /* ------------------------------------------------------------------------
   * LDS runtime
   *
   * Shared base for every generated custom element. The generator never emits
   * class syntax — it emits LDS.define({...}) descriptors and this runtime builds
   * the constructor. That keeps code generation to data, not to JavaScript source.
   * ---------------------------------------------------------------------- */

  /* One constructable stylesheet and one <template> per component, shared by
   * every instance: cloning a parsed template is far cheaper than re-parsing. */
  const sheetCache = new Map();
  const templateCache = new Map();

  function sheetFor(tag, cssText) {
    let sheet = sheetCache.get(tag);
    if (!sheet) {
      sheet = new CSSStyleSheet();
      sheet.replaceSync(cssText);
      sheetCache.set(tag, sheet);
    }
    return sheet;
  }

  function templateFor(tag, html) {
    let tpl = templateCache.get(tag);
    if (!tpl) {
      tpl = document.createElement("template");
      tpl.innerHTML = html;
      templateCache.set(tag, tpl);
    }
    return tpl;
  }

  /* Resolve the directory that served this script so icons.svg (and anything else
   * co-located with the bundle) resolves under the same mount prefix as fonts do
   * relative to lds.css. document.currentScript is only set while this file is
   * evaluating — capture it before any await. Fall back to scanning script tags
   * when currentScript is unavailable (some embedders strip it). */
  function resolveScriptBase() {
    if (typeof document === "undefined") return "";
    const current =
      document.currentScript && document.currentScript.src
        ? document.currentScript.src
        : "";
    if (current) return current.replace(/[^/]+$/, "");
    const scripts = document.getElementsByTagName("script");
    for (let i = scripts.length - 1; i >= 0; i--) {
      const src = scripts[i].src || "";
      if (/\/lds(\.min)?\.js(\?|#|$)/.test(src)) {
        return src.replace(/[^/]+$/, "");
      }
    }
    return "";
  }

  const scriptBase = resolveScriptBase();

  let iconsPromise = null;

  const LDS = {
    version: "0.0.0",

    /** Directory URL of the loaded lds.js (trailing slash), or "". */
    base: scriptBase,

    /** Sorted icon names; overwritten by the generator after this file. */
    icons: [],

    /** tag -> generated constructor. */
    registry: new Map(),

    /** tag -> behaviour hooks. */
    behaviors: new Map(),

    /**
     * Register behaviour hooks for a tag. Authored in
     * components/<name>/<name>.js and always emitted before the matching
     * LDS.define() call, because customElements.define() upgrades synchronously.
     *
     * Hooks, all optional:
     *   connected(el)
     *   disconnected(el)
     *   attributeChanged(el, name, prev, next)
     *   formAssociated(el)
     *   formReset(el)
     *   formDisabled(el, disabled)
     *   formStateRestore(el, state, mode)
     */
    behavior(tag, hooks) {
      LDS.behaviors.set(tag, hooks || {});
    },

    /**
     * Build and register a custom element from a generated descriptor:
     * { tag, className, observed, defaults, template, styles, formAssociated }
     */
    define(spec) {
      const existing = customElements.get(spec.tag);
      if (existing) return existing;

      const observed = spec.observed || [];
      const defaults = spec.defaults || {};
      const formAssociated = !!spec.formAssociated;

      class LdsElement extends HTMLElement {
        static get formAssociated() {
          return formAssociated;
        }

        static get observedAttributes() {
          return observed;
        }

        constructor() {
          super();
          this._lds = { ready: false, hooks: LDS.behaviors.get(spec.tag) || {} };
          if (formAssociated && typeof this.attachInternals === "function") {
            this._lds.internals = this.attachInternals();
          }
          this.attachShadow({
            mode: "open",
            delegatesFocus: formAssociated,
          });
        }

        connectedCallback() {
          if (!this._lds.ready) {
            this.shadowRoot.adoptedStyleSheets = [sheetFor(spec.tag, spec.styles)];
            this.shadowRoot.appendChild(
              templateFor(spec.tag, spec.template).content.cloneNode(true)
            );

            /* Reflect declared defaults so :host([variant="primary"]) matches
             * even when the author wrote a bare <lds-button>. */
            for (const name in defaults) {
              if (!this.hasAttribute(name)) this.setAttribute(name, defaults[name]);
            }
            this._lds.ready = true;
          }
          if (this._lds.hooks.connected) this._lds.hooks.connected(this);
        }

        disconnectedCallback() {
          if (this._lds.hooks.disconnected) this._lds.hooks.disconnected(this);
        }

        attributeChangedCallback(name, prev, next) {
          if (prev === next) return;
          if (this._lds.hooks.attributeChanged) {
            this._lds.hooks.attributeChanged(this, name, prev, next);
          }
        }

        formAssociatedCallback() {
          if (this._lds.hooks.formAssociated) this._lds.hooks.formAssociated(this);
        }

        formResetCallback() {
          if (this._lds.hooks.formReset) this._lds.hooks.formReset(this);
        }

        formDisabledCallback(disabled) {
          if (this._lds.hooks.formDisabled) {
            this._lds.hooks.formDisabled(this, disabled);
          }
        }

        formStateRestoreCallback(state, mode) {
          if (this._lds.hooks.formStateRestore) {
            this._lds.hooks.formStateRestore(this, state, mode);
          }
        }

        /** ElementInternals when formAssociated, else null. */
        get internals() {
          return this._lds.internals || null;
        }

        /** Query the shadow root. */
        $(selector) {
          return this.shadowRoot.querySelector(selector);
        }

        /** Query all matches in the shadow root. */
        $$(selector) {
          return Array.from(this.shadowRoot.querySelectorAll(selector));
        }

        /** Read a boolean attribute: present and not literally "false". */
        boolAttr(name) {
          return this.hasAttribute(name) && this.getAttribute(name) !== "false";
        }

        /**
         * Dispatch a composed, bubbling, cancelable custom event. Returns the
         * event so callers can check defaultPrevented.
         */
        emit(name, detail) {
          const event = new CustomEvent(name, {
            detail: detail,
            bubbles: true,
            composed: true,
            cancelable: true,
          });
          this.dispatchEvent(event);
          return event;
        }
      }

      Object.defineProperty(LdsElement, "name", { value: spec.className });
      customElements.define(spec.tag, LdsElement);
      LDS.registry.set(spec.tag, LdsElement);
      return LdsElement;
    },

    /**
     * Set the active theme. Themes are plain data attributes on <html> which the
     * generated theme blocks in lds.css key off; pass null to fall back to the
     * base layer.
     */
    setTheme(name) {
      if (name) {
        document.documentElement.setAttribute("data-lds-theme", name);
      } else {
        document.documentElement.removeAttribute("data-lds-theme");
      }
    },

    /** Resolve a design token, e.g. LDS.token("color.accent.default"). */
    token(path, element) {
      const scope = element || document.documentElement;
      return getComputedStyle(scope)
        .getPropertyValue("--lds-" + path.replace(/\./g, "-"))
        .trim();
    },

    /** Whether name is in the build-time icon list. */
    hasIcon(name) {
      return LDS.icons.indexOf(name) !== -1;
    },

    /**
     * Fetch dist/icons.svg once and inject it into the document. Each lds-icon
     * clones symbol children into its shadow SVG — <use href="#…"> from a shadow
     * root cannot resolve document-level IDs.
     */
    ensureIcons() {
      if (iconsPromise) return iconsPromise;
      if (typeof document === "undefined") {
        iconsPromise = Promise.resolve();
        return iconsPromise;
      }
      if (document.getElementById("lds-icon-sprite")) {
        iconsPromise = Promise.resolve();
        return iconsPromise;
      }

      const url = LDS.base + "icons.svg";
      iconsPromise = fetch(url)
        .then(function (res) {
          if (!res.ok) {
            throw new Error("LDS: failed to load icon sprite (" + res.status + ")");
          }
          return res.text();
        })
        .then(function (svgText) {
          if (document.getElementById("lds-icon-sprite")) return;
          const wrap = document.createElement("div");
          wrap.innerHTML = svgText.trim();
          const svg = wrap.firstElementChild;
          if (!svg || svg.tagName.toLowerCase() !== "svg") {
            throw new Error("LDS: icon sprite is not an <svg>");
          }
          svg.id = "lds-icon-sprite";
          svg.setAttribute("aria-hidden", "true");
          svg.style.position = "absolute";
          svg.style.width = "0";
          svg.style.height = "0";
          svg.style.overflow = "hidden";
          document.documentElement.appendChild(svg);
        })
        .catch(function (err) {
          iconsPromise = null;
          console.error(err);
          throw err;
        });
      return iconsPromise;
    },
  };

  /* ------------------------------------------------------------------------
   * Overlay helpers — non-modal dismiss plumbing for menus and future popovers.
   *
   * Menus do not lock scroll. Pass scrollLock: true for modals.
   * Callers own open/close state; this wires Escape, outside click, optional
   * focus trap, focus restoration, and optional document scroll lock.
   * ---------------------------------------------------------------------- */

  /**
   * Bind dismiss and focus behaviour to an open overlay root.
   *
   * @param {Element} root floating surface (panel)
   * @param {object} [opts]
   * @param {(reason: "escape"|"outside") => void} [opts.onDismiss]
   * @param {Element} [opts.restoreFocus] element to focus on disconnect
   * @param {boolean} [opts.trapFocus=true] cycle Tab within root
   * @param {boolean} [opts.scrollLock=false] lock documentElement overflow while bound
   * @param {Element|Element[]} [opts.ignoreOutside] clicks here are not "outside"
   * @returns {{ disconnect(): void }}
   */
  LDS.overlay = {
    bind(root, opts) {
      opts = opts || {};
      const onDismiss = typeof opts.onDismiss === "function" ? opts.onDismiss : function () {};
      const trapFocus = opts.trapFocus !== false;
      const restoreFocus = opts.restoreFocus || null;
      const scrollLock = !!opts.scrollLock;
      let previousOverflow = "";

      if (scrollLock && typeof document !== "undefined") {
        previousOverflow = document.documentElement.style.overflow;
        document.documentElement.style.overflow = "hidden";
      }

      let ignore = opts.ignoreOutside || [];
      if (!Array.isArray(ignore)) ignore = [ignore];
      ignore = ignore.filter(Boolean);

      const composedContains = function (container, target) {
        let node = target;
        while (node) {
          if (node === container) return true;
          if (node.nodeType === 11 && node.host) {
            node = node.host;
          } else {
            node = node.parentNode;
          }
        }
        return false;
      };

      const isInside = function (target) {
        if (!target || !target.nodeType) return false;
        if (composedContains(root, target)) return true;
        for (let i = 0; i < ignore.length; i++) {
          if (composedContains(ignore[i], target)) return true;
        }
        return false;
      };

      const deepestActive = function () {
        let el = document.activeElement;
        while (el && el.shadowRoot && el.shadowRoot.activeElement) {
          el = el.shadowRoot.activeElement;
        }
        return el;
      };

      const focusSelector =
        'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';

      const isVisible = function (el) {
        if (el.closest("[hidden]")) return false;
        if (el.getAttribute("aria-hidden") === "true") return false;
        const style = window.getComputedStyle(el);
        return style.visibility !== "hidden" && style.display !== "none";
      };

      /* Walk light DOM, slots and shadow roots — menu items live in slotted
       * custom elements whose real controls sit in their own shadows. */
      const focusables = function () {
        const out = [];
        const visit = function (node) {
          if (!node) return;
          if (node.nodeType === 1) {
            if (node.matches(focusSelector) && isVisible(node)) out.push(node);
            if (node.tagName === "SLOT") {
              node.assignedElements({ flatten: true }).forEach(visit);
              return;
            }
            if (node.shadowRoot) visit(node.shadowRoot);
            Array.prototype.forEach.call(node.children, visit);
            return;
          }
          if (node.nodeType === 11) {
            Array.prototype.forEach.call(node.children, visit);
          }
        };
        visit(root);
        return out;
      };

      /* Defer outside-pointer binding so the opening click does not dismiss. */
      let pointerReady = false;
      const armPointer = function () {
        pointerReady = true;
      };
      window.setTimeout(armPointer, 0);

      const onPointerDown = function (event) {
        if (!pointerReady) return;
        if (isInside(event.target)) return;
        onDismiss("outside");
      };

      const onKeydown = function (event) {
        if (event.key === "Escape") {
          event.preventDefault();
          onDismiss("escape");
          return;
        }
        if (!trapFocus || event.key !== "Tab") return;

        const list = focusables();
        if (!list.length) {
          event.preventDefault();
          if (typeof root.focus === "function") root.focus();
          return;
        }

        const first = list[0];
        const last = list[list.length - 1];
        const active = deepestActive();

        if (event.shiftKey && active === first) {
          event.preventDefault();
          last.focus();
        } else if (!event.shiftKey && active === last) {
          event.preventDefault();
          first.focus();
        } else if (!composedContains(root, active) && ignore.every(function (node) {
          return !composedContains(node, active);
        })) {
          event.preventDefault();
          (event.shiftKey ? last : first).focus();
        }
      };

      document.addEventListener("pointerdown", onPointerDown, true);
      document.addEventListener("keydown", onKeydown, true);

      return {
        disconnect: function () {
          document.removeEventListener("pointerdown", onPointerDown, true);
          document.removeEventListener("keydown", onKeydown, true);
          if (scrollLock && typeof document !== "undefined") {
            document.documentElement.style.overflow = previousOverflow;
          }
          if (restoreFocus && typeof restoreFocus.focus === "function") {
            try {
              restoreFocus.focus();
            } catch (_) {
              /* ignore */
            }
          }
        },
      };
    },
  };

  /* ------------------------------------------------------------------------
   * Anchored positioning — manual layout until CSS anchor positioning is safe.
   *
   * Places a floating element relative to an anchor with flip + viewport clamp.
   * Call again on resize/scroll while open; the consumer owns that loop.
   * ---------------------------------------------------------------------- */

  /**
   * Position `floating` next to `anchor`.
   *
   * Placement is `side` or `side-align`: side is top | bottom | left | right,
   * align is start | end, and a bare side centres on the anchor. The side flips
   * when the preferred one would overflow and the opposite one fits.
   *
   * @param {HTMLElement} floating
   * @param {Element} anchor
   * @param {object} [opts]
   * @param {string} [opts.placement] default "bottom-end"
   * @param {number} [opts.offset] gap in CSS pixels; default = control.gap.md
   * @param {boolean} [opts.matchWidth] min-width matches anchor width
   * @returns {string} the side actually used, after any flip
   */
  LDS.position = function (floating, anchor, opts) {
    opts = opts || {};
    if (!floating || !anchor) return "";

    const placement = opts.placement || "bottom-end";
    let offset = opts.offset;
    if (offset == null) {
      const raw = LDS.token("control.gap.md", anchor);
      offset = raw ? parseFloat(raw) : 8;
      if (Number.isNaN(offset)) offset = 8;
    }

    const prevVisibility = floating.style.visibility;
    const prevDisplay = floating.style.display;
    floating.style.visibility = "hidden";
    floating.style.display = "block";
    floating.style.position = "fixed";
    floating.style.top = "0";
    floating.style.left = "0";

    /* Measure the anchor only once the floating element is out of flow: while
     * it is still static it can widen the anchor's container and shift the
     * anchor, which is then measured where it will not stay. */
    const anchorRect = anchor.getBoundingClientRect();
    if (opts.matchWidth) {
      floating.style.minWidth = Math.ceil(anchorRect.width) + "px";
    }

    const floatRect = floating.getBoundingClientRect();
    const vw = window.innerWidth;
    const vh = window.innerHeight;

    const parts = placement.split("-");
    let side = parts[0];
    const align = parts[1] || "center";
    const vertical = side === "top" || side === "bottom";

    /* Main axis: the coordinate on the side, flipped if it does not fit. */
    const before = vertical
      ? anchorRect.top - offset - floatRect.height
      : anchorRect.left - offset - floatRect.width;
    const after = vertical ? anchorRect.bottom + offset : anchorRect.right + offset;
    const limit = vertical ? vh : vw;
    const size = vertical ? floatRect.height : floatRect.width;

    let main;
    if (side === "bottom" || side === "right") {
      main = after;
      if (after + size > limit && before >= 0) {
        main = before;
        side = side === "bottom" ? "top" : "left";
      }
    } else {
      main = before;
      if (before < 0 && after + size <= limit) {
        main = after;
        side = side === "top" ? "bottom" : "right";
      }
    }

    /* Cross axis: start, end or centre along the anchor. */
    const crossStart = vertical ? anchorRect.left : anchorRect.top;
    const crossEnd = vertical ? anchorRect.right : anchorRect.bottom;
    const crossSize = vertical ? floatRect.width : floatRect.height;
    let cross;
    if (align === "start") cross = crossStart;
    else if (align === "end") cross = crossEnd - crossSize;
    else cross = (crossStart + crossEnd) / 2 - crossSize / 2;

    let top = vertical ? main : cross;
    let left = vertical ? cross : main;

    /* Clamp into the viewport with a small inset. */
    const inset = 8;
    left = Math.min(Math.max(left, inset), Math.max(inset, vw - floatRect.width - inset));
    top = Math.min(Math.max(top, inset), Math.max(inset, vh - floatRect.height - inset));

    floating.style.top = Math.round(top) + "px";
    floating.style.left = Math.round(left) + "px";
    floating.style.visibility = prevVisibility || "";
    if (prevDisplay) {
      floating.style.display = prevDisplay;
    } else {
      floating.style.removeProperty("display");
    }
    return side;
  };

  LDS.icons = ["alert-circle", "alert-triangle", "archive", "arrow-up-down", "at-sign", "bell", "bookmark", "calendar", "calendar-days", "chart-column", "chart-gantt", "check", "chevron-down", "chevron-left", "chevron-right", "chevron-up", "circle", "circle-check", "circle-dashed", "circle-dot", "circle-x", "copy", "corner-down-right", "external-link", "eye", "filter", "flag", "folder", "git-branch", "git-commit-horizontal", "git-merge", "git-pull-request", "grip-vertical", "history", "inbox", "info", "kanban", "keyboard", "layers", "link", "list", "loader-circle", "lock", "log-out", "menu", "message-square", "milestone", "minus", "moon", "more-horizontal", "octagon-alert", "paperclip", "plus", "repeat", "rocket", "search", "settings", "signal-high", "signal-low", "signal-medium", "sliders-horizontal", "sun", "table", "tag", "trash-2", "user", "users", "x"];

  /* behaviour: lds-avatar */
  (function () {
    /* Behaviour for <lds-avatar>.
     *
     * Derives initials from name, loads src with error fallback, and always
     * exposes an accessible name via label or name.
     */

    LDS.behavior("lds-avatar", {
      connected(el) {
        const img = el.$(".img");

        img.addEventListener("load", () => {
          el._lds.imageFailed = false;
          syncFace(el);
        });
        img.addEventListener("error", () => {
          el._lds.imageFailed = true;
          syncFace(el);
        });

        syncAll(el);
      },

      attributeChanged(el, name) {
        if (!el._lds.ready) return;
        if (name === "src") el._lds.imageFailed = false;
        syncAll(el);
      },
    });

    function syncAll(el) {
      syncName(el);
      syncImage(el);
      syncFace(el);
    }

    function syncName(el) {
      const label = el.getAttribute("label") || el.getAttribute("name");
      if (label) {
        el.setAttribute("role", "img");
        el.setAttribute("aria-label", label);
      } else {
        el.setAttribute("role", "img");
        el.setAttribute("aria-label", "User");
      }
    }

    function syncImage(el) {
      const img = el.$(".img");
      const src = el.getAttribute("src");
      if (src) {
        if (img.getAttribute("src") !== src) img.setAttribute("src", src);
      } else {
        img.removeAttribute("src");
        el._lds.imageFailed = false;
      }
    }

    function syncFace(el) {
      const img = el.$(".img");
      const initialsEl = el.$(".initials");
      const fallback = el.$(".fallback");

      const src = el.getAttribute("src");
      const failed = !!el._lds.imageFailed;
      const showImage = !!(src && !failed && img.complete && img.naturalWidth > 0);
      const initials = resolveInitials(el);

      img.hidden = !showImage;
      initialsEl.hidden = !initials || showImage;
      fallback.hidden = !!(initials || showImage);

      initialsEl.textContent = initials || "";
    }

    function resolveInitials(el) {
      const explicit = (el.getAttribute("initials") || "").trim();
      if (explicit) return explicit.slice(0, 3);

      const name = (el.getAttribute("name") || "").trim();
      if (!name) return "";

      const parts = name.split(/\s+/).filter(Boolean);
      if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
      return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
    }
  })();

  /* behaviour: lds-avatar-group */
  (function () {
    /* Behaviour for <lds-avatar-group>.
     *
     * Caps visible lds-avatar children via max and shows a +N overflow chip.
     */

    LDS.behavior("lds-avatar-group", {
      connected(el) {
        const slot = el.$("slot");
        if (slot) slot.addEventListener("slotchange", () => syncOverflow(el));

        const mo = new MutationObserver(() => syncOverflow(el));
        mo.observe(el, { childList: true });
        el._lds.avatarGroupMo = mo;

        syncOverflow(el);
      },

      disconnected(el) {
        if (el._lds.avatarGroupMo) {
          el._lds.avatarGroupMo.disconnect();
          el._lds.avatarGroupMo = null;
        }
      },

      attributeChanged(el, name) {
        if (name === "max" || name === "size") syncOverflow(el);
      },
    });

    function avatarsOf(el) {
      return Array.prototype.slice.call(el.querySelectorAll(":scope > lds-avatar"));
    }

    function syncOverflow(el) {
      const chip = el.$(".overflow");
      if (!chip) return;

      const avatars = avatarsOf(el);
      const maxAttr = el.getAttribute("max");
      const max = maxAttr != null && maxAttr !== "" ? parseInt(maxAttr, 10) : NaN;

      if (!Number.isFinite(max) || max < 1 || avatars.length <= max) {
        avatars.forEach((avatar) => {
          avatar.hidden = false;
        });
        chip.hidden = true;
        chip.textContent = "";
        chip.removeAttribute("aria-label");
        return;
      }

      const visible = Math.max(max - 1, 0);
      const hiddenCount = avatars.length - visible;

      avatars.forEach((avatar, index) => {
        avatar.hidden = index >= visible;
      });

      chip.hidden = false;
      chip.textContent = "+" + hiddenCount;
      chip.setAttribute("aria-label", hiddenCount + " more");
      chip.removeAttribute("aria-hidden");
    }
  })();

  /* behaviour: lds-button */
  (function () {
    /* Behaviour for <lds-button>.
     *
     * Two problems the platform does not solve for us:
     *
     * 1. A <button type="submit"> inside a shadow root does not submit the form in
     *    the surrounding document — the shadow boundary stops form association. We
     *    forward the activation manually.
     * 2. Click events still bubble out of a disabled host, because the host element
     *    itself is not disabled, only the inner button. We stop them at the host.
     *
     * Hooks receive the host element; `el.$()`, `el.boolAttr()` and `el.emit()` come
     * from the runtime base class in runtime/base.js.
     */

    LDS.behavior("lds-button", {
      connected(el) {
        const button = el.$(".btn");

        syncDisabled(el, button);
        syncLabel(el, button);
        watchDefaultSlot(el);

        /* Capture on the host so a click on a disabled or loading button never
         * reaches listeners the consuming page attached. */
        el.addEventListener(
          "click",
          (event) => {
            if (el.boolAttr("disabled") || el.boolAttr("loading")) {
              event.preventDefault();
              event.stopImmediatePropagation();
              return;
            }

            const activation = el.emit("lds-click", { originalEvent: event });
            if (activation.defaultPrevented) return;

            forwardToForm(el);
          },
          true
        );
      },

      attributeChanged(el, name) {
        const button = el.$(".btn");
        if (!button) return; /* changed before first connect */

        if (name === "disabled" || name === "loading") syncDisabled(el, button);
        if (name === "label") syncLabel(el, button);
      },
    });

    function syncDisabled(el, button) {
      const loading = el.boolAttr("loading");

      /* Only `disabled` leaves the tab order. A loading button stays focusable so
       * keyboard users are not thrown back to the top of the document mid-request. */
      button.disabled = el.boolAttr("disabled");
      button.setAttribute("aria-busy", loading ? "true" : "false");
    }

    function syncLabel(el, button) {
      const label = el.getAttribute("label");
      if (label) {
        button.setAttribute("aria-label", label);
      } else {
        button.removeAttribute("aria-label");
      }
    }

    /* CSS has no "does this slot have content" selector, so mirror it onto an
     * attribute the stylesheet can key off. */
    function watchDefaultSlot(el) {
      const slot = el.$(".btn__label slot");
      const label = el.$(".btn__label");
      if (!slot || !label) return;

      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && node.textContent.trim() === ""
        );
        label.toggleAttribute("data-empty", empty);
      };

      slot.addEventListener("slotchange", update);
      update();
    }

    function forwardToForm(el) {
      const type = el.getAttribute("type");
      if (type !== "submit" && type !== "reset") return;

      const form = el.closest("form");
      if (!form) return;

      if (type === "reset") {
        form.reset();
        return;
      }

      /* requestSubmit runs validation and fires submit, unlike form.submit(). */
      if (typeof form.requestSubmit === "function") {
        form.requestSubmit();
      } else {
        form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
      }
    }
  })();

  /* behaviour: lds-divider */
  (function () {
    /* Behaviour for <lds-divider> — separator role and label emptiness. */

    LDS.behavior("lds-divider", {
      connected(el) {
        sync(el);
        watchSlot(el);
      },

      attributeChanged(el) {
        if (!el._lds.ready) return;
        sync(el);
      },
    });

    function sync(el) {
      const orientation = el.getAttribute("orientation") || "horizontal";
      el.setAttribute("role", "separator");
      el.setAttribute("aria-orientation", orientation);
    }

    function watchSlot(el) {
      const slot = el.$("slot");
      const label = el.$(".label");
      if (!slot || !label) return;

      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        label.toggleAttribute("data-empty", empty);
      };

      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-icon */
  (function () {
    /* Behaviour for <lds-icon>.
     *
     * The symbol sprite is injected into the light DOM once by LDS.ensureIcons.
     * SVG <use href="#…"> from inside a shadow root only resolves IDs in that
     * shadow tree (not the document), so we clone the symbol's children into the
     * host SVG instead of referencing them with <use>.
     */

    LDS.behavior("lds-icon", {
      connected(el) {
        syncA11y(el);
        syncIcon(el);
      },

      attributeChanged(el, name) {
        if (name === "label") syncA11y(el);
        if (name === "name") syncIcon(el);
      },
    });

    function syncA11y(el) {
      const label = el.getAttribute("label");
      const svg = el.$(".icon");
      if (!svg) return;

      if (label) {
        el.removeAttribute("aria-hidden");
        svg.setAttribute("role", "img");
        svg.setAttribute("aria-label", label);
      } else {
        el.setAttribute("aria-hidden", "true");
        svg.removeAttribute("role");
        svg.removeAttribute("aria-label");
      }
    }

    function syncIcon(el) {
      const name = el.getAttribute("name");
      const svg = el.$(".icon");
      if (!svg) return;

      clearChildren(svg);

      if (!name) return;

      if (!LDS.hasIcon(name)) {
        console.warn('LDS: unknown icon "' + name + '". Check LDS.icons.');
      }

      LDS.ensureIcons()
        .then(function () {
          /* name may have changed while the sprite was loading */
          if (el.getAttribute("name") !== name) return;

          const symbol = document.getElementById("lds-icon-" + name);
          if (!symbol) {
            console.warn('LDS: icon symbol "lds-icon-' + name + '" not in sprite.');
            return;
          }

          const viewBox = symbol.getAttribute("viewBox");
          if (viewBox) svg.setAttribute("viewBox", viewBox);

          /* Lift Lucide presentation attrs onto the host SVG so paths inherit. */
          ["fill", "stroke", "stroke-width", "stroke-linecap", "stroke-linejoin"].forEach(
            function (attr) {
              const value = symbol.getAttribute(attr);
              if (value) svg.setAttribute(attr, value);
              else svg.removeAttribute(attr);
            }
          );

          clearChildren(svg);
          for (let node = symbol.firstChild; node; node = node.nextSibling) {
            svg.appendChild(node.cloneNode(true));
          }
        })
        .catch(function () {
          /* ensureIcons already logged; leave the icon empty. */
        });
    }

    function clearChildren(node) {
      while (node.firstChild) node.removeChild(node.firstChild);
    }
  })();

  /* behaviour: lds-spinner */
  (function () {
    /* Behaviour for <lds-spinner> — accessible name when label is set. */

    LDS.behavior("lds-spinner", {
      connected(el) {
        syncLabel(el);
      },

      attributeChanged(el, name) {
        if (!el._lds.ready) return;
        if (name === "label") syncLabel(el);
      },
    });

    function syncLabel(el) {
      const label = el.getAttribute("label");
      if (label) {
        el.setAttribute("role", "status");
        el.setAttribute("aria-label", label);
        el.removeAttribute("aria-hidden");
      } else {
        el.removeAttribute("role");
        el.removeAttribute("aria-label");
        el.setAttribute("aria-hidden", "true");
      }
    }
  })();

  /* behaviour: lds-tag */
  (function () {
    /* Behaviour for <lds-tag> — dismiss control and start-slot emptiness. */

    LDS.behavior("lds-tag", {
      connected(el) {
        const close = el.$(".tag__close");
        watchSlot(el, '.tag__start slot[name="start"]', ".tag__start");
        syncDismiss(el, close);

        close.addEventListener("click", (event) => {
          event.stopPropagation();
          if (el.boolAttr("disabled")) return;
          const ev = el.emit("lds-remove");
          if (!ev.defaultPrevented) el.remove();
        });
      },

      attributeChanged(el, name) {
        if (!el._lds.ready) return;
        if (name === "dismissible" || name === "disabled") {
          syncDismiss(el, el.$(".tag__close"));
        }
      },
    });

    function syncDismiss(el, close) {
      if (!close) return;
      const show = el.boolAttr("dismissible");
      close.hidden = !show;
      close.disabled = el.boolAttr("disabled");
    }

    function watchSlot(el, slotSel, wrapSel) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;
      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        wrap.toggleAttribute("data-empty", empty);
      };
      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-checkbox */
  (function () {
    /* Behaviour for <lds-checkbox> — form-associated boolean with indeterminate. */

    LDS.behavior("lds-checkbox", {
      connected(el) {
        const input = el.$(".input");
        watchLabel(el);
        sync(el, input);

        input.addEventListener("change", () => {
          el.toggleAttribute("checked", input.checked);
          if (input.checked) el.removeAttribute("indeterminate");
          setValue(el);
          el.emit("lds-change", {
            checked: input.checked,
            indeterminate: false,
            value: el.getAttribute("value") || "on",
          });
        });

        el.addEventListener(
          "click",
          (event) => {
            if (el.boolAttr("disabled")) {
              event.preventDefault();
              event.stopImmediatePropagation();
            }
          },
          true
        );
      },

      attributeChanged(el, name) {
        if (!el._lds.ready) return;
        if (name === "indeterminate" || name === "checked" || name === "disabled" || name === "required" || name === "name" || name === "value") {
          sync(el, el.$(".input"));
        }
      },

      formReset(el) {
        if (el._lds.defaultChecked == null) el._lds.defaultChecked = false;
        el.toggleAttribute("checked", el._lds.defaultChecked);
        el.removeAttribute("indeterminate");
        sync(el, el.$(".input"));
      },

      formDisabled(el, disabled) {
        el.toggleAttribute("disabled", disabled);
        const input = el.$(".input");
        if (input) input.disabled = disabled;
      },

      formStateRestore(el, state) {
        const checked = state != null && state !== "";
        el.toggleAttribute("checked", checked);
        el.removeAttribute("indeterminate");
        sync(el, el.$(".input"));
      },
    });

    function sync(el, input) {
      if (!input) return;
      if (el._lds.defaultChecked == null) el._lds.defaultChecked = el.boolAttr("checked");

      input.disabled = el.boolAttr("disabled");
      input.required = el.boolAttr("required");
      input.name = el.getAttribute("name") || "";
      input.value = el.getAttribute("value") || "on";

      const indeterminate = el.boolAttr("indeterminate");
      input.indeterminate = indeterminate;
      input.checked = !indeterminate && el.boolAttr("checked");
      if (indeterminate) el.setAttribute("aria-checked", "mixed");
      else el.setAttribute("aria-checked", input.checked ? "true" : "false");

      setValue(el);
    }

    function setValue(el) {
      if (!el.internals) return;
      const input = el.$(".input");
      if (!input || input.indeterminate || !input.checked) {
        el.internals.setFormValue(null);
        return;
      }
      el.internals.setFormValue(el.getAttribute("value") || "on");
    }

    function watchLabel(el) {
      const slot = el.$("slot");
      const label = el.$(".label");
      if (!slot || !label) return;
      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        label.hidden = empty;
      };
      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-combobox */
  (function () {
    /* Behaviour for <lds-combobox> — filterable single or multiple choice.
     *
     * Focus stays in the shadow <input> the whole time; the highlighted option is
     * conveyed with aria-activedescendant. Options are light-DOM lds-option
     * children, so the reference crosses the shadow boundary outward, which only
     * ARIA element reflection (input.ariaActiveDescendantElement) can express. An
     * id string cannot.
     *
     * Values live in the `value` attribute: one value, or a comma-separated list
     * when `multiple` is set. The form value is a FormData with one entry per
     * value in multiple mode.
     */

    const comboState = new WeakMap();
    let comboSeq = 0;

    LDS.behavior("lds-combobox", {
      connected(el) {
        if (comboState.has(el)) {
          syncAll(el);
          return;
        }
        comboSeq += 1;
        const input = el.$(".input");
        const list = el.$(".list");
        const create = el.$(".create");
        list.id = "lds-combobox-list-" + comboSeq;
        create.id = "lds-combobox-create-" + comboSeq;
        input.setAttribute("aria-controls", list.id);

        const s = {
          open: false,
          editing: false,
          active: null,
          overlay: null,
          reposition: null,
          defaultValue: el.getAttribute("value") || "",
          syncing: false,
        };
        comboState.set(el, s);

        input.addEventListener("input", () => onInput(el));
        input.addEventListener("keydown", (event) => onKeydown(el, event));

        el.$(".control").addEventListener("click", (event) => {
          if (el.boolAttr("disabled")) return;
          if (event.target.closest && event.target.closest("lds-tag")) return;
          input.focus();
          /* Clicking into the text keeps the list open; the rest of the control,
           * chevron included, toggles it. */
          if (!s.open) openList(el);
          else if (event.target !== input) closeList(el);
        });

        /* Keep focus in the input while clicking options. */
        el.$(".panel").addEventListener("pointerdown", (event) => event.preventDefault());
        el.$(".panel").addEventListener("click", (event) => {
          const target = targetFrom(el, event);
          if (target) pick(el, target);
        });
        el.$(".panel").addEventListener("pointermove", (event) => {
          const target = targetFrom(el, event);
          if (target && target !== s.active) setActive(el, target, { scroll: false });
        });

        el.addEventListener("focusout", (event) => {
          if (event.relatedTarget && el.contains(event.relatedTarget)) return;
          /* Focus moving inside our own shadow (a chip's remove button). */
          if (el.shadowRoot.contains(event.relatedTarget)) return;
          closeList(el);
          finishEditing(el);
        });

        const mo = new MutationObserver(() => {
          if (s.syncing) return;
          syncAll(el);
        });
        mo.observe(el, { childList: true, subtree: true, characterData: true, attributeFilter: ["value", "label", "disabled", "keywords"] });
        s.mo = mo;

        syncAll(el);
      },

      disconnected(el) {
        closeList(el);
      },

      attributeChanged(el, name) {
        if (!el._lds.ready || !comboState.has(el)) return;
        if (name === "value" || name === "multiple" || name === "required" || name === "name") syncSelection(el);
        if (name === "disabled" || name === "placeholder" || name === "label" || name === "aria-label") syncInput(el);
        if (name === "disabled" && el.boolAttr("disabled")) closeList(el);
      },

      formReset(el) {
        const s = comboState.get(el);
        if (!s) return;
        el.setAttribute("value", s.defaultValue);
        finishEditing(el);
      },

      formDisabled(el, disabled) {
        el.toggleAttribute("disabled", disabled);
      },

      formStateRestore(el, state) {
        if (state == null) return;
        const values = typeof state === "string" ? splitValues(state) : state.getAll(el.getAttribute("name") || "value");
        el.setAttribute("value", values.join(","));
      },
    });

    /* ---------------------------------------------------------------- reading */

    function optionsOf(el) {
      return Array.prototype.slice.call(el.querySelectorAll(":scope > lds-option"));
    }

    function valueOf(option) {
      const v = option.getAttribute("value");
      return v != null ? v : labelOf(option);
    }

    /* The label is the default slot's text: everything but the start/end slots. */
    function labelOf(option) {
      const explicit = option.getAttribute("label");
      if (explicit) return explicit;
      let text = "";
      option.childNodes.forEach((node) => {
        if (node.nodeType === Node.ELEMENT_NODE && node.hasAttribute("slot")) return;
        text += node.textContent;
      });
      return text.replace(/\s+/g, " ").trim();
    }

    function splitValues(raw) {
      return (raw || "").split(",").map((v) => v.trim()).filter(Boolean);
    }

    function currentValues(el) {
      const values = splitValues(el.getAttribute("value"));
      return el.boolAttr("multiple") ? values : values.slice(0, 1);
    }

    function findOption(el, value) {
      return optionsOf(el).find((o) => valueOf(o) === value) || null;
    }

    function targetFrom(el, event) {
      const path = event.composedPath();
      for (const node of path) {
        if (node === el) break;
        if (node.classList && node.classList.contains("create")) return node;
        if (node.tagName === "LDS-OPTION" && node.parentElement === el) return node;
      }
      return null;
    }

    function isCreate(node) {
      return !!node && node.classList && node.classList.contains("create");
    }

    /* ---------------------------------------------------------------- syncing */

    function syncAll(el) {
      syncInput(el);
      syncSelection(el);
      const s = comboState.get(el);
      if (s && s.open) filter(el);
    }

    function syncInput(el) {
      const input = el.$(".input");
      input.disabled = el.boolAttr("disabled");
      const label = el.getAttribute("label") || el.getAttribute("aria-label");
      if (label) input.setAttribute("aria-label", label);
      else input.removeAttribute("aria-label");
      const list = el.$(".list");
      if (label) list.setAttribute("aria-label", label);
      list.setAttribute("aria-multiselectable", el.boolAttr("multiple") ? "true" : "false");
      syncPlaceholder(el);
    }

    function syncPlaceholder(el) {
      const input = el.$(".input");
      const empty = currentValues(el).length === 0;
      input.placeholder = empty || !el.boolAttr("multiple") ? el.getAttribute("placeholder") || "" : "";
    }

    function syncSelection(el) {
      const s = comboState.get(el);
      s.syncing = true;
      const values = currentValues(el);
      optionsOf(el).forEach((option) => {
        option.setAttribute("aria-selected", values.indexOf(valueOf(option)) >= 0 ? "true" : "false");
      });
      el.$(".list").setAttribute("aria-multiselectable", el.boolAttr("multiple") ? "true" : "false");

      if (el.boolAttr("multiple")) renderChips(el, values);
      else renderChips(el, []);
      renderLead(el, values);
      if (!s.editing && !el.boolAttr("multiple")) {
        const option = values[0] != null ? findOption(el, values[0]) : null;
        el.$(".input").value = option ? labelOf(option) : values[0] || "";
      }
      syncPlaceholder(el);
      syncForm(el, values);
      s.syncing = false;
    }

    function syncForm(el, values) {
      const internals = el.internals;
      if (!internals) return;
      if (el.boolAttr("multiple")) {
        const data = new FormData();
        const name = el.getAttribute("name") || "value";
        values.forEach((v) => data.append(name, v));
        internals.setFormValue(data);
      } else {
        internals.setFormValue(values[0] || "");
      }
      if (el.boolAttr("required") && values.length === 0) {
        internals.setValidity({ valueMissing: true }, "Select an option.", el.$(".input"));
      } else {
        internals.setValidity({});
      }
    }

    /* Single mode shows the selected option's start content (an avatar, a status
     * glyph) in front of the text. */
    function renderLead(el, values) {
      const lead = el.$(".lead");
      lead.textContent = "";
      lead.hidden = true;
      if (el.boolAttr("multiple") || !values.length) return;
      const option = findOption(el, values[0]);
      const start = option && option.querySelector(':scope > [slot="start"]');
      if (!start) return;
      const clone = start.cloneNode(true);
      clone.removeAttribute("slot");
      clone.removeAttribute("id");
      lead.appendChild(clone);
      lead.hidden = false;
    }

    function renderChips(el, values) {
      const chips = el.$(".chips");
      chips.textContent = "";
      values.forEach((value) => {
        const option = findOption(el, value);
        const label = option ? labelOf(option) : value;
        const tag = document.createElement("lds-tag");
        tag.setAttribute("size", "sm");
        tag.setAttribute("dismissible", "");
        if (el.boolAttr("disabled")) tag.setAttribute("disabled", "");
        const start = option && option.querySelector(':scope > [slot="start"]');
        if (start) {
          const clone = start.cloneNode(true);
          clone.removeAttribute("id");
          tag.appendChild(clone);
        }
        tag.appendChild(document.createTextNode(label));
        tag.addEventListener("lds-remove", (event) => {
          event.preventDefault();
          setValues(el, currentValues(el).filter((v) => v !== value));
          el.$(".input").focus();
        });
        chips.appendChild(tag);
        const close = tag.$ && tag.$(".tag__close");
        if (close) {
          close.setAttribute("aria-label", "Remove " + label);
          close.tabIndex = -1;
        }
      });
    }

    function setValues(el, values) {
      el.setAttribute("value", values.join(","));
      el.emit("lds-change", { value: values.join(","), values: values.slice() });
    }

    /* -------------------------------------------------------------- filtering */

    function filter(el) {
      const input = el.$(".input");
      const s = comboState.get(el);
      const query = s.editing ? input.value.trim() : "";
      const tokens = query.toLowerCase().split(/\s+/).filter(Boolean);
      const manual = el.getAttribute("filter") === "none";
      let visible = 0;
      let exact = false;

      optionsOf(el).forEach((option) => {
        const label = labelOf(option).toLowerCase();
        const haystack = label + " " + valueOf(option).toLowerCase() + " " + (option.getAttribute("keywords") || "").toLowerCase();
        const match = manual || tokens.every((t) => haystack.indexOf(t) >= 0);
        option.hidden = !match;
        if (match) visible += 1;
        if (label === query.toLowerCase()) exact = true;
      });

      const create = el.$(".create");
      const canCreate = el.boolAttr("creatable") && query !== "" && !exact;
      create.hidden = !canCreate;
      el.$(".create__label").textContent = canCreate ? "Create “" + query + "”" : "";

      const empty = el.$(".empty");
      empty.hidden = visible > 0 || canCreate;
      empty.textContent = el.getAttribute("empty") || "No results";
    }

    function candidates(el) {
      const list = optionsOf(el).filter((o) => !o.hidden && !o.boolAttr("disabled"));
      const create = el.$(".create");
      if (!create.hidden) list.push(create);
      return list;
    }

    function setActive(el, node, opts) {
      const s = comboState.get(el);
      const input = el.$(".input");
      optionsOf(el).forEach((o) => o.toggleAttribute("data-active", o === node));
      el.$(".create").toggleAttribute("data-active", isCreate(node));
      s.active = node || null;

      if ("ariaActiveDescendantElement" in input) {
        input.ariaActiveDescendantElement = node || null;
      } else if (node) {
        /* Without element reflection only the in-shadow create row resolves. */
        input.setAttribute("aria-activedescendant", node.id);
      } else {
        input.removeAttribute("aria-activedescendant");
      }

      if (node && (!opts || opts.scroll !== false)) node.scrollIntoView({ block: "nearest" });
    }

    function activateDefault(el) {
      const list = candidates(el);
      const values = currentValues(el);
      const selected = list.find((o) => !isCreate(o) && values.indexOf(valueOf(o)) >= 0);
      setActive(el, comboState.get(el).editing ? list[0] || null : selected || list[0] || null);
    }

    function move(el, delta) {
      const list = candidates(el);
      if (!list.length) return;
      const s = comboState.get(el);
      let index = list.indexOf(s.active);
      if (index < 0) index = delta > 0 ? -1 : list.length;
      index = Math.min(Math.max(index + delta, 0), list.length - 1);
      setActive(el, list[index]);
    }

    /* ---------------------------------------------------------------- open / close */

    function openList(el) {
      const s = comboState.get(el);
      if (s.open || el.boolAttr("disabled")) return;
      s.open = true;
      const panel = el.$(".panel");
      panel.hidden = false;
      el.$(".input").setAttribute("aria-expanded", "true");
      filter(el);
      activateDefault(el);
      position(el);

      s.reposition = () => position(el);
      window.addEventListener("resize", s.reposition);
      window.addEventListener("scroll", s.reposition, true);
      s.overlay = LDS.overlay.bind(panel, {
        trapFocus: false,
        ignoreOutside: [el],
        onDismiss: () => closeList(el),
      });
    }

    function closeList(el) {
      const s = comboState.get(el);
      if (!s || !s.open) return;
      s.open = false;
      el.$(".panel").hidden = true;
      el.$(".input").setAttribute("aria-expanded", "false");
      setActive(el, null);
      if (s.reposition) {
        window.removeEventListener("resize", s.reposition);
        window.removeEventListener("scroll", s.reposition, true);
        s.reposition = null;
      }
      if (s.overlay) {
        s.overlay.disconnect();
        s.overlay = null;
      }
    }

    function position(el) {
      LDS.position(el.$(".panel"), el.$(".control"), { placement: "bottom-start", matchWidth: true });
    }

    /* Leave edit mode: single shows the selection again, multiple clears the
     * query. */
    function finishEditing(el) {
      const s = comboState.get(el);
      s.editing = false;
      if (el.boolAttr("multiple")) el.$(".input").value = "";
      syncSelection(el);
    }

    /* ---------------------------------------------------------------- input */

    function onInput(el) {
      const s = comboState.get(el);
      const input = el.$(".input");
      s.editing = true;
      if (!s.open) openList(el);
      filter(el);
      setActive(el, input.value.trim() ? candidates(el)[0] || null : null);
      position(el);
      el.emit("lds-search", { query: input.value });
    }

    function onKeydown(el, event) {
      const s = comboState.get(el);
      const input = el.$(".input");

      switch (event.key) {
        case "ArrowDown":
          event.preventDefault();
          if (!s.open) openList(el);
          else if (event.altKey) return;
          else move(el, 1);
          return;
        case "ArrowUp":
          event.preventDefault();
          if (!s.open) openList(el);
          else if (event.altKey) closeList(el);
          else move(el, -1);
          return;
        case "Enter":
          if (!s.open || !s.active) return;
          event.preventDefault();
          pick(el, s.active);
          return;
        case "Escape":
          /* An open list is closed by the overlay helper, which handles Escape
           * first and cancels it. Only an Escape on a closed list abandons the
           * typed text. */
          if (s.open || event.defaultPrevented) return;
          if (s.editing || input.value) {
            event.preventDefault();
            finishEditing(el);
          }
          return;
        case "Backspace":
          if (el.boolAttr("multiple") && input.value === "" && input.selectionStart === 0) {
            const values = currentValues(el);
            if (values.length) {
              event.preventDefault();
              setValues(el, values.slice(0, -1));
            }
          }
          return;
        case "Tab":
          closeList(el);
          return;
        default:
          return;
      }
    }

    /* ---------------------------------------------------------------- choosing */

    function pick(el, node) {
      const s = comboState.get(el);
      const input = el.$(".input");

      if (isCreate(node)) {
        const query = input.value.trim();
        if (!query) return;
        const ev = el.emit("lds-create", { value: query });
        if (ev.defaultPrevented) return;
        const option = document.createElement("lds-option");
        option.setAttribute("value", query.replace(/,/g, " "));
        option.textContent = query;
        el.appendChild(option);
        node = option;
      }

      if (node.boolAttr && node.boolAttr("disabled")) return;
      const value = valueOf(node);

      if (el.boolAttr("multiple")) {
        const values = currentValues(el);
        const next = values.indexOf(value) >= 0 ? values.filter((v) => v !== value) : values.concat(value);
        input.value = "";
        s.editing = false;
        setValues(el, next);
        filter(el);
        setActive(el, node, { scroll: false });
        position(el);
        return;
      }

      s.editing = false;
      closeList(el);
      if (currentValues(el)[0] !== value) setValues(el, [value]);
      else syncSelection(el);
    }
  })();

  /* behaviour: lds-date-picker */
  (function () {
    /* Behaviour for <lds-date-picker> — form-associated native date/time input. */

    const TYPES = {
      date: true,
      time: true,
      "datetime-local": true,
      month: true,
      week: true,
    };

    LDS.behavior("lds-date-picker", {
      connected(el) {
        const input = el.$(".input");
        watchSlot(el, 'slot[name="prefix"]', ".prefix");
        watchSlot(el, 'slot[name="suffix"]', ".suffix");
        sync(el, input);

        input.addEventListener("input", () => {
          el.setAttribute("value", input.value);
          setValue(el, input.value);
          el.emit("lds-input", { value: input.value });
        });
        input.addEventListener("change", () => {
          el.emit("lds-change", { value: input.value });
        });

        el.addEventListener(
          "click",
          (event) => {
            if (el.boolAttr("disabled")) {
              event.preventDefault();
              event.stopImmediatePropagation();
            }
          },
          true
        );
      },

      attributeChanged(el) {
        if (!el._lds.ready) return;
        sync(el, el.$(".input"));
      },

      formReset(el) {
        const initial = el.getAttribute("value") || "";
        const input = el.$(".input");
        if (input) input.value = initial;
        setValue(el, initial);
      },

      formDisabled(el, disabled) {
        el.toggleAttribute("disabled", disabled);
        const input = el.$(".input");
        if (input) input.disabled = disabled;
      },

      formStateRestore(el, state) {
        if (state == null) return;
        const value = String(state);
        el.setAttribute("value", value);
        const input = el.$(".input");
        if (input) input.value = value;
        setValue(el, value);
      },
    });

    function sync(el, input) {
      if (!input) return;

      let type = el.getAttribute("type") || "date";
      if (!TYPES[type]) type = "date";
      input.type = type;

      input.disabled = el.boolAttr("disabled");
      input.readOnly = el.boolAttr("readonly");
      input.required = el.boolAttr("required");
      input.name = el.getAttribute("name") || "";

      if (el.hasAttribute("value") && input.value !== el.getAttribute("value")) {
        input.value = el.getAttribute("value") || "";
      }
      if (el.hasAttribute("min")) input.min = el.getAttribute("min");
      else input.removeAttribute("min");
      if (el.hasAttribute("max")) input.max = el.getAttribute("max");
      else input.removeAttribute("max");
      if (el.hasAttribute("step")) input.step = el.getAttribute("step");
      else input.removeAttribute("step");

      const invalid = el.boolAttr("invalid") || el.getAttribute("aria-invalid") === "true";
      if (invalid) input.setAttribute("aria-invalid", "true");
      else input.removeAttribute("aria-invalid");

      setValue(el, input.value);
    }

    function setValue(el, value) {
      if (el.internals) el.internals.setFormValue(value == null ? "" : String(value));
    }

    function watchSlot(el, slotSel, wrapSel) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;
      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        wrap.toggleAttribute("data-empty", empty);
      };
      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-date-range */
  (function () {
    /* Behaviour for <lds-date-range> — two native date inputs, one form value.
     *
     * The form value is an ISO 8601 interval, "start/end", so one field carries
     * both dates. Each input bounds the other (end.min = start, start.max = end)
     * and a reversed range is reported through the Constraint Validation API.
     */

    LDS.behavior("lds-date-range", {
      connected(el) {
        if (el._lds.rangeDefault == null) {
          el._lds.rangeDefault = { start: el.getAttribute("start") || "", end: el.getAttribute("end") || "" };
        }
        const start = el.$(".start");
        const end = el.$(".end");
        watchSlot(el, 'slot[name="prefix"]', ".prefix");

        for (const input of [start, end]) {
          const attr = input === start ? "start" : "end";
          input.addEventListener("input", () => {
            el.setAttribute(attr, input.value);
          });
          input.addEventListener("change", () => {
            el.emit("lds-change", { start: start.value, end: end.value, value: rangeValue(start.value, end.value) });
          });
        }

        el.addEventListener(
          "click",
          (event) => {
            if (el.boolAttr("disabled")) {
              event.preventDefault();
              event.stopImmediatePropagation();
            }
          },
          true
        );

        sync(el);
      },

      attributeChanged(el) {
        if (!el._lds.ready) return;
        sync(el);
      },

      formReset(el) {
        const d = el._lds.rangeDefault || { start: "", end: "" };
        el.setAttribute("start", d.start);
        el.setAttribute("end", d.end);
      },

      formDisabled(el, disabled) {
        el.toggleAttribute("disabled", disabled);
      },

      formStateRestore(el, state) {
        if (typeof state !== "string") return;
        const parts = state.split("/");
        el.setAttribute("start", parts[0] || "");
        el.setAttribute("end", parts[1] || "");
      },
    });

    function rangeValue(start, end) {
      return start || end ? start + "/" + end : "";
    }

    function sync(el) {
      const start = el.$(".start");
      const end = el.$(".end");
      const disabled = el.boolAttr("disabled");
      const required = el.boolAttr("required");

      /* Assign only on change: rewriting a native date input resets the segment
       * the user is typing in. */
      const startAttr = el.getAttribute("start") || "";
      const endAttr = el.getAttribute("end") || "";
      if (start.value !== startAttr) start.value = startAttr;
      if (end.value !== endAttr) end.value = endAttr;

      start.setAttribute("aria-label", el.getAttribute("start-label") || "Start date");
      end.setAttribute("aria-label", el.getAttribute("end-label") || "End date");
      const label = el.getAttribute("label");
      if (label) el.$(".wrap").setAttribute("aria-label", label);
      else el.$(".wrap").removeAttribute("aria-label");

      for (const input of [start, end]) {
        input.disabled = disabled;
        input.required = required;
        const invalid = el.boolAttr("invalid") || el.getAttribute("aria-invalid") === "true";
        if (invalid) input.setAttribute("aria-invalid", "true");
        else input.removeAttribute("aria-invalid");
      }

      /* Each date bounds the other, inside the overall min/max. */
      const min = el.getAttribute("min") || "";
      const max = el.getAttribute("max") || "";
      setBound(start, "min", min);
      setBound(start, "max", end.value || max);
      setBound(end, "min", start.value || min);
      setBound(end, "max", max);

      if (!el.internals) return;
      el.internals.setFormValue(rangeValue(start.value, end.value));

      if (required && (!start.value || !end.value)) {
        el.internals.setValidity({ valueMissing: true }, "Enter a start and an end date.", start.value ? end : start);
      } else if (start.value && end.value && end.value < start.value) {
        el.internals.setValidity({ rangeUnderflow: true }, "The end date is before the start date.", end);
      } else if ((min && start.value && start.value < min) || (max && end.value && end.value > max)) {
        el.internals.setValidity({ rangeOverflow: true }, "The dates are outside the allowed range.", start);
      } else {
        el.internals.setValidity({});
      }
    }

    function setBound(input, name, value) {
      if (value) input.setAttribute(name, value);
      else input.removeAttribute(name);
    }

    function watchSlot(el, slotSel, wrapSel) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;
      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        wrap.toggleAttribute("data-empty", empty);
      };
      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-field */
  (function () {
    /* Behaviour for <lds-field> — label / describedby wiring for the control. */

    let fieldSeq = 0;

    LDS.behavior("lds-field", {
      connected(el) {
        if (!el._lds.fieldId) {
          fieldSeq += 1;
          el._lds.fieldId = "lds-field-" + fieldSeq;
        }

        watchSlot(el, 'slot[name="help"]', ".help");
        watchSlot(el, 'slot[name="error"]', ".error", () => sync(el));

        const slot = el.$(".control slot");
        if (slot) slot.addEventListener("slotchange", () => sync(el));

        sync(el);
      },

      attributeChanged(el) {
        if (!el._lds.ready) return;
        sync(el);
      },
    });

    function sync(el) {
      const id = el._lds.fieldId;
      const labelEl = el.$(".label");
      const requiredMark = el.$(".required");
      const help = el.$(".help");
      const error = el.$(".error");
      const control = controlOf(el);

      const label = el.getAttribute("label") || "";
      labelEl.textContent = label;

      const required = el.boolAttr("required");
      requiredMark.hidden = !required;

      help.id = id + "-help";
      error.id = id + "-error";

      const hasError = !error.hasAttribute("data-empty");
      const hasHelp = !help.hasAttribute("data-empty");
      el.toggleAttribute("invalid", hasError);

      if (!control) return;

      const controlId = id + "-control";
      if (!control.id) control.id = controlId;
      labelEl.setAttribute("for", control.id);

      const described = [];
      if (hasHelp) described.push(help.id);
      if (hasError) described.push(error.id);
      if (described.length) control.setAttribute("aria-describedby", described.join(" "));
      else control.removeAttribute("aria-describedby");

      if (hasError) {
        control.setAttribute("aria-invalid", "true");
        if (control.setAttribute) control.setAttribute("invalid", "");
      } else {
        control.removeAttribute("aria-invalid");
        if (control.removeAttribute) control.removeAttribute("invalid");
      }

      if (required) {
        if (control.setAttribute) control.setAttribute("required", "");
        if ("required" in control) control.required = true;
      }

      if (label && control.getAttribute && !control.getAttribute("aria-label") && !control.getAttribute("aria-labelledby")) {
        /* label[for] covers native labelling when the control is the labelled target */
      }
    }

    function controlOf(el) {
      const slot = el.$(".control slot");
      if (!slot) return null;
      const assigned = slot.assignedElements({ flatten: true });
      for (let i = 0; i < assigned.length; i++) {
        const node = assigned[i];
        if (node.matches && node.matches("lds-input,lds-textarea,lds-select,lds-checkbox,lds-switch,lds-radio-group,lds-date-picker,input,textarea,select")) {
          return node;
        }
      }
      return assigned[0] || null;
    }

    function watchSlot(el, slotSel, wrapSel, onChange) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;
      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        wrap.toggleAttribute("data-empty", empty);
        if (onChange) onChange();
      };
      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-input */
  (function () {
    /* Behaviour for <lds-input> — form-associated text control. */

    LDS.behavior("lds-input", {
      connected(el) {
        const input = el.$(".input");
        watchSlot(el, 'slot[name="prefix"]', ".prefix");
        watchSlot(el, 'slot[name="suffix"]', ".suffix");
        sync(el, input);

        input.addEventListener("input", () => {
          el.setAttribute("value", input.value);
          setValue(el, input.value);
          el.emit("lds-input", { value: input.value });
        });
        input.addEventListener("change", () => {
          el.emit("lds-change", { value: input.value });
        });

        el.addEventListener(
          "click",
          (event) => {
            if (el.boolAttr("disabled")) {
              event.preventDefault();
              event.stopImmediatePropagation();
            }
          },
          true
        );
      },

      attributeChanged(el) {
        if (!el._lds.ready) return;
        sync(el, el.$(".input"));
      },

      formReset(el) {
        const initial = el.getAttribute("value") || "";
        const input = el.$(".input");
        if (input) input.value = initial;
        setValue(el, initial);
      },

      formDisabled(el, disabled) {
        el.toggleAttribute("disabled", disabled);
        const input = el.$(".input");
        if (input) input.disabled = disabled;
      },

      formStateRestore(el, state) {
        if (state == null) return;
        const value = String(state);
        el.setAttribute("value", value);
        const input = el.$(".input");
        if (input) input.value = value;
        setValue(el, value);
      },
    });

    function sync(el, input) {
      if (!input) return;
      const type = el.getAttribute("type") || "text";
      input.type = type;
      input.placeholder = el.getAttribute("placeholder") || "";
      input.disabled = el.boolAttr("disabled");
      input.readOnly = el.boolAttr("readonly");
      input.required = el.boolAttr("required");
      input.name = el.getAttribute("name") || "";

      if (el.hasAttribute("value") && input.value !== el.getAttribute("value")) {
        input.value = el.getAttribute("value") || "";
      }
      if (el.hasAttribute("min")) input.min = el.getAttribute("min");
      else input.removeAttribute("min");
      if (el.hasAttribute("max")) input.max = el.getAttribute("max");
      else input.removeAttribute("max");
      if (el.hasAttribute("step")) input.step = el.getAttribute("step");
      else input.removeAttribute("step");
      if (el.hasAttribute("maxlength")) input.maxLength = Number(el.getAttribute("maxlength"));
      else input.removeAttribute("maxlength");

      const invalid = el.boolAttr("invalid") || el.getAttribute("aria-invalid") === "true";
      input.setAttribute("aria-invalid", invalid ? "true" : "false");
      if (!invalid) input.removeAttribute("aria-invalid");

      setValue(el, input.value);
    }

    function setValue(el, value) {
      if (el.internals) el.internals.setFormValue(value == null ? "" : String(value));
    }

    function watchSlot(el, slotSel, wrapSel) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;
      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        wrap.toggleAttribute("data-empty", empty);
      };
      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-option */
  (function () {
    /* Behaviour for <lds-option> — role, id and slot emptiness. Selection and
     * the active highlight are set by the parent lds-combobox. */

    let optionSeq = 0;

    LDS.behavior("lds-option", {
      connected(el) {
        el.setAttribute("role", "option");
        if (!el.id) {
          optionSeq += 1;
          el.id = "lds-option-" + optionSeq;
        }
        if (!el.hasAttribute("aria-selected")) el.setAttribute("aria-selected", "false");
        syncDisabled(el);
        watchSlot(el, 'slot[name="start"]', ".start");
        watchSlot(el, 'slot[name="end"]', ".end");
      },

      attributeChanged(el, name) {
        if (!el._lds.ready) return;
        if (name === "disabled") syncDisabled(el);
      },
    });

    function syncDisabled(el) {
      if (el.boolAttr("disabled")) el.setAttribute("aria-disabled", "true");
      else el.removeAttribute("aria-disabled");
    }

    function watchSlot(el, slotSel, wrapSel) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;
      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        wrap.toggleAttribute("data-empty", empty);
      };
      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-radio-group */
  (function () {
    /* Behaviour for <lds-radio-group> — FACE group with roving tabindex. */

    LDS.behavior("lds-radio-group", {
      connected(el) {
        const group = el.$(".options");
        sync(el);

        group.addEventListener("click", (event) => {
          if (el.boolAttr("disabled")) return;
          const option = optionFromEvent(el, event.target);
          if (!option || optionDisabled(option)) return;
          select(el, valueOf(option), { focus: false });
        });

        group.addEventListener("keydown", (event) => {
          if (el.boolAttr("disabled")) return;
          const options = enabledOptions(el);
          if (!options.length) return;
          const current = options.indexOf(document.activeElement);
          if (current < 0 && !isOption(document.activeElement)) return;

          const horizontal = el.getAttribute("orientation") === "horizontal";
          let next = current < 0 ? 0 : current;
          if (event.key === "ArrowDown" || (horizontal && event.key === "ArrowRight")) {
            next = (Math.max(current, 0) + 1) % options.length;
          } else if (event.key === "ArrowUp" || (horizontal && event.key === "ArrowLeft")) {
            next = (Math.max(current, 0) - 1 + options.length) % options.length;
          } else if (event.key === "Home") next = 0;
          else if (event.key === "End") next = options.length - 1;
          else if (event.key === " " || event.key === "Enter") {
            const active = document.activeElement;
            if (isOption(active) && el.contains(active)) {
              event.preventDefault();
              select(el, valueOf(active), { focus: true });
            }
            return;
          } else return;

          event.preventDefault();
          select(el, valueOf(options[next]), { focus: true });
        });

        const slot = el.$("slot");
        if (slot) slot.addEventListener("slotchange", () => sync(el));

        const mo = new MutationObserver(() => {
          if (el._lds.radioSyncing) return;
          sync(el);
        });
        mo.observe(el, { childList: true });
        el._lds.radioMo = mo;
      },

      disconnected(el) {
        if (el._lds.radioMo) {
          el._lds.radioMo.disconnect();
          el._lds.radioMo = null;
        }
      },

      attributeChanged(el) {
        if (!el._lds.ready) return;
        sync(el);
      },

      formReset(el) {
        const initial = el._lds.defaultValue;
        if (initial == null) return;
        el.setAttribute("value", initial);
        sync(el);
      },

      formDisabled(el, disabled) {
        el.toggleAttribute("disabled", disabled);
        sync(el);
      },

      formStateRestore(el, state) {
        if (state == null) return;
        el.setAttribute("value", String(state));
        sync(el);
      },
    });

    function sync(el) {
      el._lds.radioSyncing = true;
      const group = el.$(".options");
      const labelEl = el.$(".label");
      const label = el.getAttribute("label") || "";
      labelEl.textContent = label;
      labelEl.hidden = !label;

      if (el._lds.defaultValue == null) {
        el._lds.defaultValue = el.getAttribute("value") || "";
      }

      const options = allOptions(el);
      let value = el.getAttribute("value");
      if (value == null || value === "") {
        const checked = options.find((o) => o.getAttribute("aria-checked") === "true");
        value = checked ? valueOf(checked) : options[0] ? valueOf(options[0]) : "";
        if (value) el.setAttribute("value", value);
      }

      group.setAttribute("role", "radiogroup");
      if (label) group.setAttribute("aria-label", label);
      else group.removeAttribute("aria-label");
      if (el.boolAttr("disabled")) group.setAttribute("aria-disabled", "true");
      else group.removeAttribute("aria-disabled");

      options.forEach((option, index) => {
        const v = valueOf(option);
        const selected = v === value;
        option.setAttribute("role", "radio");
        option.setAttribute("aria-checked", selected ? "true" : "false");
        option.tabIndex = selected ? 0 : -1;
        if (option.tagName === "BUTTON" && !option.getAttribute("type")) {
          option.setAttribute("type", "button");
        }
        if (el.boolAttr("disabled") || optionDisabled(option)) {
          option.setAttribute("aria-disabled", "true");
          option.tabIndex = -1;
        } else {
          option.removeAttribute("aria-disabled");
        }
      });

      setValue(el, value);
      el._lds.radioSyncing = false;
    }

    function select(el, value, opts) {
      if (value == null) return;
      el.setAttribute("value", value);
      sync(el);
      if (opts && opts.focus) {
        const option = allOptions(el).find((o) => valueOf(o) === value);
        if (option) option.focus();
      }
      el.emit("lds-change", { value: value });
    }

    function setValue(el, value) {
      if (el.internals) el.internals.setFormValue(value == null ? "" : String(value));
    }

    function allOptions(el) {
      const slot = el.$("slot");
      if (!slot) return [];
      return slot
        .assignedElements({ flatten: true })
        .filter((node) => node.hasAttribute("value") || node.tagName === "BUTTON");
    }

    function enabledOptions(el) {
      return allOptions(el).filter((o) => !optionDisabled(o) && !el.boolAttr("disabled"));
    }

    function optionFromEvent(el, target) {
      const options = allOptions(el);
      let node = target;
      while (node && node !== el) {
        if (options.indexOf(node) >= 0) return node;
        node = node.parentElement;
      }
      return null;
    }

    function isOption(node) {
      return node && node.getAttribute && node.getAttribute("role") === "radio";
    }

    function valueOf(option) {
      return option.getAttribute("value") || option.value || option.textContent.trim();
    }

    function optionDisabled(option) {
      return option.hasAttribute("disabled") || option.getAttribute("aria-disabled") === "true";
    }
  })();

  /* behaviour: lds-segment */
  (function () {
    /* Behaviour for <lds-segment> — accessible name and slot emptiness. Role,
     * checked state and tabindex are owned by the parent lds-segmented. */

    LDS.behavior("lds-segment", {
      connected(el) {
        el.setAttribute("role", "radio");
        syncLabel(el);
        watchSlot(el, 'slot[name="start"]', ".start");
        watchSlot(el, ".label slot", ".label");
      },

      attributeChanged(el, name) {
        if (!el._lds.ready) return;
        if (name === "label") syncLabel(el);
      },
    });

    function syncLabel(el) {
      const label = el.getAttribute("label");
      if (label) el.setAttribute("aria-label", label);
      else el.removeAttribute("aria-label");
    }

    function watchSlot(el, slotSel, wrapSel) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;
      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        wrap.toggleAttribute("data-empty", empty);
      };
      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-segmented */
  (function () {
    /* Behaviour for <lds-segmented> — a radiogroup of lds-segment children.
     *
     * Roving tabindex: only the checked segment is in the tab order, arrow keys
     * move and select at once, as native radios do. Form-associated, so a
     * segmented control inside a form submits its value under name.
     */

    LDS.behavior("lds-segmented", {
      connected(el) {
        if (el._lds.segDefault == null) el._lds.segDefault = el.getAttribute("value") || "";
        const track = el.$(".track");

        el.addEventListener("click", (event) => {
          const segment = segmentFrom(el, event.target);
          if (!segment || !enabled(el, segment)) return;
          choose(el, segment, false);
        });

        /* Key events from slotted segments pass through the track. */
        track.addEventListener("keydown", (event) => onKeydown(el, event));

        el.$("slot").addEventListener("slotchange", () => sync(el));
        sync(el);
      },

      attributeChanged(el) {
        if (!el._lds.ready) return;
        sync(el);
      },

      formReset(el) {
        el.setAttribute("value", el._lds.segDefault || "");
      },

      formDisabled(el, disabled) {
        el.toggleAttribute("disabled", disabled);
      },

      formStateRestore(el, state) {
        if (state != null) el.setAttribute("value", String(state));
      },
    });

    function segmentsOf(el) {
      return Array.prototype.slice.call(el.querySelectorAll(":scope > lds-segment"));
    }

    function segmentFrom(el, node) {
      while (node && node !== el) {
        if (node.tagName === "LDS-SEGMENT" && node.parentElement === el) return node;
        node = node.parentElement;
      }
      return null;
    }

    function valueOf(segment) {
      return segment.getAttribute("value") || segment.textContent.trim();
    }

    function enabled(el, segment) {
      return !el.boolAttr("disabled") && !segment.hasAttribute("disabled");
    }

    function sync(el) {
      const track = el.$(".track");
      const label = el.getAttribute("label");
      if (label) track.setAttribute("aria-label", label);
      else track.removeAttribute("aria-label");

      const segments = segmentsOf(el);
      let value = el.getAttribute("value");
      if (!value && segments.length) {
        value = valueOf(segments[0]);
        el.setAttribute("value", value);
        return;
      }

      segments.forEach((segment) => {
        const checked = valueOf(segment) === value;
        segment.setAttribute("role", "radio");
        segment.setAttribute("aria-checked", checked ? "true" : "false");
        segment.tabIndex = checked && enabled(el, segment) ? 0 : -1;
        if (enabled(el, segment)) segment.removeAttribute("aria-disabled");
        else segment.setAttribute("aria-disabled", "true");
      });

      /* If the checked segment cannot take focus, keep the group reachable. */
      if (!segments.some((s) => s.tabIndex === 0)) {
        const first = segments.find((s) => enabled(el, s));
        if (first) first.tabIndex = 0;
      }

      if (el.internals) el.internals.setFormValue(value || "");
    }

    function choose(el, segment, focus) {
      const value = valueOf(segment);
      if (focus) segment.focus();
      if (value === el.getAttribute("value")) return;
      el.setAttribute("value", value);
      el.emit("lds-change", { value: value });
    }

    function onKeydown(el, event) {
      const segments = segmentsOf(el).filter((s) => enabled(el, s));
      if (!segments.length) return;
      const current = segments.indexOf(segmentFrom(el, event.target));
      let next;
      if (event.key === "ArrowRight" || event.key === "ArrowDown") next = (current + 1) % segments.length;
      else if (event.key === "ArrowLeft" || event.key === "ArrowUp") next = (current - 1 + segments.length) % segments.length;
      else if (event.key === "Home") next = 0;
      else if (event.key === "End") next = segments.length - 1;
      else if (event.key === " " || event.key === "Enter") next = Math.max(current, 0);
      else return;
      event.preventDefault();
      event.stopPropagation();
      choose(el, segments[next], true);
    }
  })();

  /* behaviour: lds-select */
  (function () {
    /* Behaviour for <lds-select> — native <select> with cloned light-DOM options. */

    LDS.behavior("lds-select", {
      connected(el) {
        const select = el.$(".select");
        const slot = el.$("slot");

        syncOptions(el, select);
        sync(el, select);

        select.addEventListener("change", () => {
          el.setAttribute("value", select.value);
          setValue(el, select.value);
          el.emit("lds-change", { value: select.value });
        });

        if (slot) {
          slot.addEventListener("slotchange", () => {
            syncOptions(el, select);
            sync(el, select);
          });
        }

        const mo = new MutationObserver(() => {
          syncOptions(el, select);
          sync(el, select);
        });
        mo.observe(el, { childList: true, subtree: true, characterData: true, attributes: true });
        el._lds.selectMo = mo;

        el.addEventListener(
          "click",
          (event) => {
            if (el.boolAttr("disabled")) {
              event.preventDefault();
              event.stopImmediatePropagation();
            }
          },
          true
        );
      },

      disconnected(el) {
        if (el._lds.selectMo) {
          el._lds.selectMo.disconnect();
          el._lds.selectMo = null;
        }
      },

      attributeChanged(el) {
        if (!el._lds.ready) return;
        sync(el, el.$(".select"));
      },

      formReset(el) {
        const select = el.$(".select");
        const initial = el.getAttribute("value");
        if (select) {
          if (initial != null) select.value = initial;
          else select.selectedIndex = 0;
          setValue(el, select.value);
        }
      },

      formDisabled(el, disabled) {
        el.toggleAttribute("disabled", disabled);
        const select = el.$(".select");
        if (select) select.disabled = disabled;
      },

      formStateRestore(el, state) {
        if (state == null) return;
        const value = String(state);
        el.setAttribute("value", value);
        const select = el.$(".select");
        if (select) select.value = value;
        setValue(el, value);
      },
    });

    function syncOptions(el, select) {
      if (!select) return;
      const current = select.value;
      select.innerHTML = "";
      Array.prototype.forEach.call(el.querySelectorAll(":scope > option, :scope > optgroup"), (node) => {
        select.appendChild(node.cloneNode(true));
      });
      if (current) select.value = current;
    }

    function sync(el, select) {
      if (!select) return;
      select.disabled = el.boolAttr("disabled");
      select.required = el.boolAttr("required");
      select.name = el.getAttribute("name") || "";

      if (el.hasAttribute("value")) {
        select.value = el.getAttribute("value") || "";
      }
      setValue(el, select.value);
    }

    function setValue(el, value) {
      if (el.internals) el.internals.setFormValue(value == null ? "" : String(value));
    }
  })();

  /* behaviour: lds-switch */
  (function () {
    /* Behaviour for <lds-switch> — immediate toggle, form-associated. */

    LDS.behavior("lds-switch", {
      connected(el) {
        const input = el.$(".input");
        watchLabel(el);
        sync(el, input);

        input.addEventListener("change", () => {
          el.toggleAttribute("checked", input.checked);
          setValue(el);
          el.emit("lds-change", {
            checked: input.checked,
            value: el.getAttribute("value") || "on",
          });
        });

        el.addEventListener(
          "click",
          (event) => {
            if (el.boolAttr("disabled")) {
              event.preventDefault();
              event.stopImmediatePropagation();
            }
          },
          true
        );
      },

      attributeChanged(el) {
        if (!el._lds.ready) return;
        sync(el, el.$(".input"));
      },

      formReset(el) {
        if (el._lds.defaultChecked == null) el._lds.defaultChecked = false;
        el.toggleAttribute("checked", el._lds.defaultChecked);
        sync(el, el.$(".input"));
      },

      formDisabled(el, disabled) {
        el.toggleAttribute("disabled", disabled);
        const input = el.$(".input");
        if (input) input.disabled = disabled;
      },

      formStateRestore(el, state) {
        el.toggleAttribute("checked", state != null && state !== "");
        sync(el, el.$(".input"));
      },
    });

    function sync(el, input) {
      if (!input) return;
      if (el._lds.defaultChecked == null) el._lds.defaultChecked = el.boolAttr("checked");

      input.disabled = el.boolAttr("disabled");
      input.required = el.boolAttr("required");
      input.name = el.getAttribute("name") || "";
      input.value = el.getAttribute("value") || "on";
      input.checked = el.boolAttr("checked");
      input.setAttribute("aria-checked", input.checked ? "true" : "false");
      setValue(el);
    }

    function setValue(el) {
      if (!el.internals) return;
      if (!el.boolAttr("checked")) {
        el.internals.setFormValue(null);
        return;
      }
      el.internals.setFormValue(el.getAttribute("value") || "on");
    }

    function watchLabel(el) {
      const slot = el.$("slot");
      const label = el.$(".label");
      if (!slot || !label) return;
      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        label.hidden = empty;
      };
      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-textarea */
  (function () {
    /* Behaviour for <lds-textarea> — form-associated multi-line control. */

    LDS.behavior("lds-textarea", {
      connected(el) {
        const area = el.$(".textarea");
        sync(el, area);

        area.addEventListener("input", () => {
          el.setAttribute("value", area.value);
          setValue(el, area.value);
          updateCounter(el, area);
          if (el.boolAttr("auto-grow")) autoGrow(area);
          el.emit("lds-input", { value: area.value });
        });
        area.addEventListener("change", () => {
          el.emit("lds-change", { value: area.value });
        });

        el.addEventListener(
          "click",
          (event) => {
            if (el.boolAttr("disabled")) {
              event.preventDefault();
              event.stopImmediatePropagation();
            }
          },
          true
        );
      },

      attributeChanged(el) {
        if (!el._lds.ready) return;
        sync(el, el.$(".textarea"));
      },

      formReset(el) {
        const initial = el.getAttribute("value") || "";
        const area = el.$(".textarea");
        if (area) area.value = initial;
        setValue(el, initial);
        updateCounter(el, area);
      },

      formDisabled(el, disabled) {
        el.toggleAttribute("disabled", disabled);
        const area = el.$(".textarea");
        if (area) area.disabled = disabled;
      },

      formStateRestore(el, state) {
        if (state == null) return;
        const value = String(state);
        el.setAttribute("value", value);
        const area = el.$(".textarea");
        if (area) area.value = value;
        setValue(el, value);
        updateCounter(el, area);
      },
    });

    function sync(el, area) {
      if (!area) return;
      area.placeholder = el.getAttribute("placeholder") || "";
      area.disabled = el.boolAttr("disabled");
      area.readOnly = el.boolAttr("readonly");
      area.required = el.boolAttr("required");
      area.name = el.getAttribute("name") || "";
      area.rows = Math.max(1, Number(el.getAttribute("rows") || 3));

      if (el.hasAttribute("value") && area.value !== el.getAttribute("value")) {
        area.value = el.getAttribute("value") || "";
      }
      if (el.hasAttribute("maxlength")) area.maxLength = Number(el.getAttribute("maxlength"));
      else area.removeAttribute("maxlength");

      updateCounter(el, area);
      if (el.boolAttr("auto-grow")) autoGrow(area);
      setValue(el, area.value);
    }

    function autoGrow(area) {
      area.style.height = "auto";
      area.style.height = area.scrollHeight + "px";
    }

    function updateCounter(el, area) {
      const counter = el.$(".counter");
      if (!counter || !area) return;
      const max = el.hasAttribute("maxlength") ? Number(el.getAttribute("maxlength")) : NaN;
      if (!Number.isFinite(max) || max <= 0) {
        counter.hidden = true;
        counter.textContent = "";
        return;
      }
      counter.hidden = false;
      counter.textContent = area.value.length + " / " + max;
    }

    function setValue(el, value) {
      if (el.internals) el.internals.setFormValue(value == null ? "" : String(value));
    }
  })();

  /* behaviour: lds-card */
  (function () {
    /* Behaviour for <lds-card> — hide empty slots. */

    LDS.behavior("lds-card", {
      connected(el) {
        watchSlot(el, 'slot[name="media"]', ".media");
        watchSlot(el, 'slot[name="header"]', ".header");
        watchSlot(el, "slot:not([name])", ".body");
        watchSlot(el, 'slot[name="footer"]', ".footer");
      },
    });

    function watchSlot(el, slotSel, wrapSel) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;
      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        wrap.toggleAttribute("data-empty", empty);
      };
      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-tabs */
  (function () {
    /* Behaviour for <lds-tabs> — tablist from lds-tab-panel children. */

    LDS.behavior("lds-tabs", {
      connected(el) {
        const tablist = el.$(".tablist");

        tablist.addEventListener("click", (event) => {
          const btn = event.target.closest("button[data-name]");
          if (!btn || !tablist.contains(btn)) return;
          selectTab(el, btn.getAttribute("data-name"), { focus: false });
        });

        tablist.addEventListener("keydown", (event) => {
          const buttons = tabButtons(el);
          if (!buttons.length) return;
          const current = buttons.indexOf(document.activeElement);
          if (current < 0) return;

          let next = current;
          if (event.key === "ArrowRight") next = (current + 1) % buttons.length;
          else if (event.key === "ArrowLeft") next = (current - 1 + buttons.length) % buttons.length;
          else if (event.key === "Home") next = 0;
          else if (event.key === "End") next = buttons.length - 1;
          else return;

          event.preventDefault();
          const name = buttons[next].getAttribute("data-name");
          const activation = el.getAttribute("activation") || "auto";
          if (activation === "manual") {
            buttons[next].focus();
          } else {
            selectTab(el, name, { focus: true });
          }
        });

        const mo = new MutationObserver(() => sync(el));
        mo.observe(el, { childList: true, attributes: true, attributeFilter: ["name", "label"] });
        el._lds.tabsMo = mo;

        const slot = el.$("slot");
        if (slot) slot.addEventListener("slotchange", () => sync(el));

        sync(el);
      },

      disconnected(el) {
        if (el._lds.tabsMo) {
          el._lds.tabsMo.disconnect();
          el._lds.tabsMo = null;
        }
      },

      attributeChanged(el, name) {
        if (!el._lds.ready) return;
        if (name === "value") sync(el);
      },
    });

    function panelsOf(el) {
      return Array.prototype.slice.call(el.querySelectorAll(":scope > lds-tab-panel"));
    }

    function tabButtons(el) {
      return Array.prototype.slice.call(el.$$(".tablist .tab"));
    }

    function sync(el) {
      const tablist = el.$(".tablist");
      const panels = panelsOf(el);
      let value = el.getAttribute("value");
      if (!value && panels[0]) value = panels[0].getAttribute("name") || "";
      if (value && !panels.some((p) => p.getAttribute("name") === value) && panels[0]) {
        value = panels[0].getAttribute("name") || "";
      }
      if (value && el.getAttribute("value") !== value) {
        el.setAttribute("value", value);
      }

      tablist.innerHTML = "";
      panels.forEach((panel, index) => {
        const name = panel.getAttribute("name") || "tab-" + index;
        if (!panel.getAttribute("name")) panel.setAttribute("name", name);
        const label = panel.getAttribute("label") || name;
        const selected = name === value;

        const btn = document.createElement("button");
        btn.type = "button";
        btn.className = "tab";
        btn.setAttribute("role", "tab");
        btn.setAttribute("data-name", name);
        btn.id = el.tagName.toLowerCase() + "-tab-" + name;
        btn.setAttribute("aria-selected", selected ? "true" : "false");
        btn.tabIndex = selected ? 0 : -1;
        btn.textContent = label;
        tablist.appendChild(btn);

        panel.toggleAttribute("active", selected);
        panel.hidden = !selected;
        panel.setAttribute("role", "tabpanel");
        panel.setAttribute("aria-labelledby", btn.id);
      });
    }

    function selectTab(el, name, opts) {
      opts = opts || {};
      if (!name || el.getAttribute("value") === name) {
        if (opts.focus) {
          const btn = el.$('.tablist .tab[data-name="' + name + '"]');
          if (btn) btn.focus();
        }
        return;
      }
      const event = el.emit("lds-change", { value: name });
      if (event.defaultPrevented) return;
      el.setAttribute("value", name);
      if (opts.focus) {
        requestAnimationFrame(() => {
          const btn = el.$('.tablist .tab[data-name="' + name + '"]');
          if (btn) btn.focus();
        });
      }
    }
  })();

  /* behaviour: lds-breadcrumb */
  (function () {
    /* Behaviour for <lds-breadcrumb>.
     *
     * When the trail overflows, middle items get data-overflow and an ellipsis
     * control appears between the first and last items. expanded shows everything.
     */

    const state = new WeakMap();

    LDS.behavior("lds-breadcrumb", {
      connected(el) {
        const overflow = el.$(".overflow");

        syncLabel(el);
        syncOverflowA11y(el, overflow);

        const onOverflow = () => setExpanded(el, true);
        overflow.addEventListener("click", onOverflow);

        const slot = el.$("slot");
        const onSlot = () => scheduleMeasure(el);
        slot.addEventListener("slotchange", onSlot);

        const ro = new ResizeObserver(() => scheduleMeasure(el));
        ro.observe(el);

        state.set(el, { onOverflow, onSlot, ro, raf: 0 });
        scheduleMeasure(el);
      },

      disconnected(el) {
        const s = state.get(el);
        if (!s) return;
        if (s.raf) cancelAnimationFrame(s.raf);
        s.ro.disconnect();
        state.delete(el);
      },

      attributeChanged(el, name) {
        const overflow = el.$(".overflow");
        if (!overflow) return;

        if (name === "label") syncLabel(el);
        if (name === "expanded") {
          syncOverflowA11y(el, overflow);
          scheduleMeasure(el);
        }
      },
    });

    function itemsOf(el) {
      const slot = el.$("slot");
      if (!slot) return [];
      return slot.assignedElements({ flatten: true }).filter(function (node) {
        return node.tagName === "LDS-BREADCRUMB-ITEM";
      });
    }

    function scheduleMeasure(el) {
      const s = state.get(el);
      if (!s) return;
      if (s.raf) cancelAnimationFrame(s.raf);
      s.raf = requestAnimationFrame(function () {
        s.raf = 0;
        measure(el);
      });
    }

    function measure(el) {
      const items = itemsOf(el);
      const overflow = el.$(".overflow");
      if (!overflow) return;

      /* Reset so we measure the full trail. */
      items.forEach(function (item) {
        item.removeAttribute("data-overflow");
      });
      el.removeAttribute("data-collapsed");

      if (el.boolAttr("expanded") || items.length <= 2) {
        overflow.hidden = true;
        syncOverflowA11y(el, overflow);
        return;
      }

      /* If everything fits, stay expanded visually with no overflow control. */
      if (el.scrollWidth <= el.clientWidth + 1) {
        overflow.hidden = true;
        syncOverflowA11y(el, overflow);
        return;
      }

      /* Collapse the middle: keep first + last (and last-1 if max-items allows). */
      const keepTail = Math.max(1, parseInt(el.getAttribute("max-items") || "1", 10) || 1);
      const lastKept = Math.max(1, items.length - keepTail);

      for (let i = 1; i < lastKept; i++) {
        items[i].setAttribute("data-overflow", "");
      }

      el.setAttribute("data-collapsed", "");
      overflow.hidden = false;
      syncOverflowA11y(el, overflow);

      /* If still overflowing with only ends visible, drop extra tail items. */
      if (el.scrollWidth > el.clientWidth + 1 && keepTail > 1) {
        for (let i = items.length - keepTail; i < items.length - 1; i++) {
          if (i > 0) items[i].setAttribute("data-overflow", "");
        }
      }
    }

    function setExpanded(el, expanded) {
      if (el.boolAttr("expanded") === expanded) return;

      const event = el.emit("lds-expand", { expanded: expanded });
      if (event.defaultPrevented) return;

      el.toggleAttribute("expanded", expanded);
    }

    function syncLabel(el) {
      el.setAttribute("aria-label", el.getAttribute("label") || "Breadcrumb");
    }

    function syncOverflowA11y(el, overflow) {
      overflow.setAttribute("aria-expanded", el.boolAttr("expanded") ? "true" : "false");
      overflow.setAttribute("aria-label", "Show more path");
    }
  })();

  /* behaviour: lds-breadcrumb-item */
  (function () {
    /* Behaviour for <lds-breadcrumb-item>.
     *
     * Forwards href/target onto the inner <a>. The current page drops href and
     * sets aria-current="page" so it is not announced as a link.
     */

    LDS.behavior("lds-breadcrumb-item", {
      connected(el) {
        sync(el);
      },

      attributeChanged(el, name) {
        if (!el.$(".link")) return;
        if (name === "href" || name === "current" || name === "target") sync(el);
      },
    });

    function sync(el) {
      const link = el.$(".link");
      if (!link) return;

      const current = el.boolAttr("current");
      const href = el.getAttribute("href");

      if (current || !href) {
        link.removeAttribute("href");
        link.removeAttribute("target");
        link.removeAttribute("rel");
        link.setAttribute("aria-current", "page");
      } else {
        link.setAttribute("href", href);
        link.removeAttribute("aria-current");

        const target = el.getAttribute("target");
        if (target) {
          link.setAttribute("target", target);
          if (target === "_blank") link.setAttribute("rel", "noopener noreferrer");
        } else {
          link.removeAttribute("target");
          link.removeAttribute("rel");
        }
      }
    }
  })();

  /* behaviour: lds-command */
  (function () {
    /* Behaviour for <lds-command> — global search / command palette.
     *
     * Filterable listbox over light-DOM lds-command-item (and optional
     * lds-command-group) children. open shows a modal dialog; hotkey registers
     * Meta/Ctrl+K. Selection emits lds-select from the item; the palette closes.
     */

    const state = new WeakMap();
    let itemSeq = 0;
    const hotkeyHosts = new Set();

    LDS.behavior("lds-command", {
      connected(el) {
        const dialog = el.$(".dialog");
        const input = el.$(".input");
        const list = el.$(".list");

        input.id = input.id || nextId("lds-command-input");
        list.id = list.id || nextId("lds-command-list");
        input.setAttribute("aria-controls", list.id);

        input.addEventListener("input", () => {
          filter(el);
          activateFirst(el);
        });

        input.addEventListener("keydown", (event) => onInputKeydown(el, event));

        el.addEventListener("lds-select", (event) => {
          if (!el.contains(event.target)) return;
          closeCommand(el, "select");
        });

        dialog.addEventListener("cancel", (event) => {
          event.preventDefault();
          closeCommand(el, "escape");
        });

        dialog.addEventListener("click", (event) => {
          if (event.target !== dialog) return;
          closeCommand(el, "outside");
        });

        const slot = el.$("slot");
        if (slot) {
          slot.addEventListener("slotchange", () => {
            syncItems(el);
            filter(el);
          });
        }

        const mo = new MutationObserver(() => {
          syncItems(el);
          filter(el);
        });
        mo.observe(el, { childList: true, subtree: true, characterData: true });
        state.set(el, { mo: mo, overlay: null });

        syncPlaceholder(el);
        syncHotkey(el);
        syncOpen(el);
      },

      disconnected(el) {
        teardown(el);
        hotkeyHosts.delete(el);
        const s = state.get(el);
        if (s && s.mo) s.mo.disconnect();
      },

      attributeChanged(el, name) {
        if (!el._lds.ready) return;
        if (name === "open") syncOpen(el);
        if (name === "placeholder") syncPlaceholder(el);
        if (name === "hotkey") syncHotkey(el);
      },
    });

    function nextId(prefix) {
      itemSeq += 1;
      return prefix + "-" + itemSeq;
    }

    function syncPlaceholder(el) {
      const input = el.$(".input");
      if (!input) return;
      input.placeholder = el.getAttribute("placeholder") || "Search…";
    }

    function syncHotkey(el) {
      if (el.boolAttr("hotkey")) hotkeyHosts.add(el);
      else hotkeyHosts.delete(el);
    }

    function syncOpen(el) {
      const dialog = el.$(".dialog");
      const input = el.$(".input");
      const open = el.boolAttr("open");

      if (open) {
        const ev = el.emit("lds-open");
        if (ev.defaultPrevented) {
          el.removeAttribute("open");
          return;
        }
        if (!dialog.open) dialog.showModal();
        setupOverlay(el);
        syncItems(el);
        input.value = "";
        filter(el);
        activateFirst(el);
        requestAnimationFrame(() => input.focus());
      } else {
        teardown(el);
        if (dialog.open) dialog.close();
      }
    }

    function setupOverlay(el) {
      const s = state.get(el) || {};
      if (s.overlay) return;
      const dialog = el.$(".dialog");
      s.overlay = LDS.overlay.bind(dialog, {
        trapFocus: false,
        scrollLock: true,
        restoreFocus: null,
        onDismiss: function () {
          /* Escape / outside handled on the dialog itself. */
        },
      });
      state.set(el, s);
    }

    function teardown(el) {
      const s = state.get(el);
      if (!s) return;
      if (s.overlay) {
        s.overlay.disconnect();
        s.overlay = null;
      }
    }

    function closeCommand(el, reason) {
      if (!el.boolAttr("open")) return;
      const ev = el.emit("lds-close", { reason: reason });
      if (ev.defaultPrevented) return;
      el.removeAttribute("open");
    }

    function itemsOf(el) {
      return Array.prototype.slice.call(el.querySelectorAll("lds-command-item"));
    }

    function groupsOf(el) {
      return Array.prototype.slice.call(el.querySelectorAll(":scope > lds-command-group"));
    }

    function syncItems(el) {
      itemsOf(el).forEach((item) => {
        if (!item.id) item.id = nextId("lds-command-option");
        item.setAttribute("role", "option");
      });
    }

    function filter(el) {
      const input = el.$(".input");
      const empty = el.$(".empty");
      const query = (input.value || "").trim().toLowerCase();
      const tokens = query ? query.split(/\s+/).filter(Boolean) : [];
      let visible = 0;

      itemsOf(el).forEach((item) => {
        const match = tokens.length === 0 || tokens.every((token) => itemMatches(item, token));
        item.hidden = !match;
        if (match) visible += 1;
        item.removeAttribute("aria-selected");
        item.removeAttribute("data-active");
      });

      groupsOf(el).forEach((group) => {
        const any = Array.prototype.some.call(
          group.querySelectorAll("lds-command-item"),
          (item) => !item.hidden
        );
        group.hidden = !any;
      });

      const noResults = visible === 0;
      el.toggleAttribute("data-empty", noResults);
      empty.hidden = !noResults;
      empty.textContent = el.getAttribute("empty") || "No results";
    }

    function itemMatches(item, token) {
      const label = (item.textContent || "").toLowerCase();
      const keywords = (item.getAttribute("keywords") || "").toLowerCase();
      const value = (item.getAttribute("value") || "").toLowerCase();
      return label.indexOf(token) >= 0 || keywords.indexOf(token) >= 0 || value.indexOf(token) >= 0;
    }

    function visibleItems(el) {
      return itemsOf(el).filter((item) => !item.hidden && !item.boolAttr("disabled"));
    }

    function activeItem(el) {
      return visibleItems(el).find((item) => item.hasAttribute("data-active")) || null;
    }

    function setActive(el, item) {
      const input = el.$(".input");
      visibleItems(el).forEach((node) => {
        const on = node === item;
        node.toggleAttribute("data-active", on);
        node.setAttribute("aria-selected", on ? "true" : "false");
      });
      if (item) {
        input.setAttribute("aria-activedescendant", item.id);
        if (typeof item.scrollIntoView === "function") {
          item.scrollIntoView({ block: "nearest" });
        }
      } else {
        input.removeAttribute("aria-activedescendant");
      }
    }

    function activateFirst(el) {
      const items = visibleItems(el);
      setActive(el, items[0] || null);
    }

    function moveActive(el, delta) {
      const items = visibleItems(el);
      if (!items.length) return;
      const current = activeItem(el);
      let index = current ? items.indexOf(current) : -1;
      if (index < 0) index = delta > 0 ? -1 : 0;
      index = (index + delta + items.length) % items.length;
      setActive(el, items[index]);
    }

    function selectActive(el) {
      const item = activeItem(el);
      if (!item) return;
      item.click();
    }

    function onInputKeydown(el, event) {
      if (event.key === "ArrowDown") {
        event.preventDefault();
        moveActive(el, 1);
      } else if (event.key === "ArrowUp") {
        event.preventDefault();
        moveActive(el, -1);
      } else if (event.key === "Enter") {
        event.preventDefault();
        selectActive(el);
      } else if (event.key === "Home") {
        const items = visibleItems(el);
        if (items[0]) {
          event.preventDefault();
          setActive(el, items[0]);
        }
      } else if (event.key === "End") {
        const items = visibleItems(el);
        if (items.length) {
          event.preventDefault();
          setActive(el, items[items.length - 1]);
        }
      }
    }

    function onDocumentKeydown(event) {
      if (!(event.key === "k" || event.key === "K")) return;
      if (!(event.metaKey || event.ctrlKey)) return;
      if (event.altKey) return;

      const target = event.target;
      if (target && (target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName))) {
        /* Allow opening from inputs — command palettes usually still catch ⌘K. */
      }

      const hosts = Array.from(hotkeyHosts).filter((el) => el.isConnected);
      if (!hosts.length) return;

      event.preventDefault();
      const el = hosts[hosts.length - 1];
      if (el.boolAttr("open")) closeCommand(el, "hotkey");
      else el.setAttribute("open", "");
    }

    if (typeof document !== "undefined") {
      document.addEventListener("keydown", onDocumentKeydown, true);
    }

    /**
     * Open a command palette. Pass { el } or the first <lds-command> on the page
     * is used. Returns the element or null.
     */
    LDS.command = function (opts) {
      opts = opts || {};
      const el =
        opts.el ||
        (typeof document !== "undefined" ? document.querySelector("lds-command") : null);
      if (!el) return null;
      el.setAttribute("open", "");
      return el;
    };
  })();

  /* behaviour: lds-command-group */
  (function () {
    /* Behaviour for <lds-command-group>. */

    LDS.behavior("lds-command-group", {
      connected(el) {
        sync(el);
      },

      attributeChanged(el) {
        if (!el._lds.ready) return;
        sync(el);
      },
    });

    function sync(el) {
      const heading = el.$(".heading");
      const group = el.$(".group");
      const text = el.getAttribute("heading") || "";
      heading.textContent = text;
      if (text) group.setAttribute("aria-label", text);
      else group.removeAttribute("aria-label");
    }
  })();

  /* behaviour: lds-command-item */
  (function () {
    /* Behaviour for <lds-command-item> — option inside lds-command. */

    LDS.behavior("lds-command-item", {
      connected(el) {
        ensureControl(el);
        watchSlot(el, ".item__start slot", ".item__start");
        watchSlot(el, ".item__end slot", ".item__end");

        el.addEventListener(
          "click",
          (event) => {
            if (el.boolAttr("disabled")) {
              event.preventDefault();
              event.stopImmediatePropagation();
              return;
            }
            activate(el, event);
          },
          true
        );

        el.addEventListener("pointermove", () => {
          if (el.boolAttr("disabled") || el.hidden) return;
          const command = el.closest("lds-command");
          if (!command || !command.boolAttr("open")) return;
          /* Parent owns aria-activedescendant; mirror hover into data-active. */
          command.querySelectorAll("lds-command-item[data-active]").forEach((node) => {
            if (node !== el) {
              node.removeAttribute("data-active");
              node.setAttribute("aria-selected", "false");
            }
          });
          el.setAttribute("data-active", "");
          el.setAttribute("aria-selected", "true");
          const input = command.$ && command.$(".input");
          if (input && el.id) input.setAttribute("aria-activedescendant", el.id);
        });
      },

      attributeChanged(el, name) {
        if (!el._lds.ready) return;
        if (name === "href" || name === "disabled" || name === "value" || name === "target") {
          ensureControl(el);
        }
      },
    });

    function ensureControl(el) {
      const wantsLink = !!el.getAttribute("href");
      let item = el.$(".item");
      const tag = wantsLink ? "a" : "button";

      if (!item || item.tagName.toLowerCase() !== tag) {
        const next = document.createElement(tag);
        next.className = "item";
        next.setAttribute("part", "item");
        if (tag === "button") next.type = "button";

        if (item) {
          while (item.firstChild) next.appendChild(item.firstChild);
          item.replaceWith(next);
        } else {
          el.shadowRoot.appendChild(next);
        }
        item = next;
      }

      syncControl(el, item);
    }

    function syncControl(el, item) {
      const disabled = el.boolAttr("disabled");

      if (item.tagName.toLowerCase() === "a") {
        const href = el.getAttribute("href") || "#";
        item.setAttribute("href", href);
        const target = el.getAttribute("target");
        if (target) {
          item.setAttribute("target", target);
          if (target === "_blank" && !item.hasAttribute("rel")) {
            item.setAttribute("rel", "noopener noreferrer");
          }
        } else {
          item.removeAttribute("target");
        }
        if (disabled) {
          item.setAttribute("aria-disabled", "true");
          item.setAttribute("tabindex", "-1");
        } else {
          item.removeAttribute("aria-disabled");
          item.removeAttribute("tabindex");
        }
      } else {
        item.disabled = disabled;
        item.removeAttribute("aria-disabled");
      }
    }

    function activate(el, originalEvent) {
      if (el.boolAttr("disabled")) return;

      const detail = { originalEvent: originalEvent };
      const value = el.getAttribute("value");
      if (value != null) detail.value = value;

      el.emit("lds-select", detail);
    }

    function watchSlot(el, slotSel, wrapSel) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;

      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        wrap.toggleAttribute("data-empty", empty);
      };

      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-menu */
  (function () {
    /* Behaviour for <lds-menu>.
     *
     * Opens a positioned action list from a slotted trigger. Uses LDS.overlay for
     * dismiss / focus restore and LDS.position for anchored layout. Not a menubar
     * and not a submenu host.
     */

    const menuState = new WeakMap();

    LDS.behavior("lds-menu", {
      connected(el) {
        const panel = el.$(".panel");
        const triggerSlot = el.$('.trigger slot[name="trigger"]');

        const onTriggerClick = (event) => {
          event.preventDefault();
          toggle(el);
        };

        const onTriggerKeydown = (event) => {
          if (event.key === "ArrowDown" || event.key === "Enter" || event.key === " ") {
            if (el.boolAttr("open")) return;
            event.preventDefault();
            openMenu(el, { focus: "first" });
          } else if (event.key === "ArrowUp") {
            if (el.boolAttr("open")) return;
            event.preventDefault();
            openMenu(el, { focus: "last" });
          }
        };

        const onPanelKeydown = (event) => {
          if (!el.boolAttr("open")) return;
          const items = enabledItems(el);
          if (!items.length) return;

          const current = focusedItemIndex(items);
          let next = current;

          if (event.key === "ArrowDown") {
            event.preventDefault();
            next = current < 0 ? 0 : (current + 1) % items.length;
            focusItem(items[next]);
          } else if (event.key === "ArrowUp") {
            event.preventDefault();
            next = current < 0 ? items.length - 1 : (current - 1 + items.length) % items.length;
            focusItem(items[next]);
          } else if (event.key === "Home") {
            event.preventDefault();
            focusItem(items[0]);
          } else if (event.key === "End") {
            event.preventDefault();
            focusItem(items[items.length - 1]);
          }
        };

        const onSelect = () => {
          closeMenu(el, "select");
        };

        el.addEventListener("lds-select", onSelect);
        panel.addEventListener("keydown", onPanelKeydown);

        menuState.set(el, {
          onTriggerClick,
          onTriggerKeydown,
          onPanelKeydown,
          onSelect,
          overlay: null,
          reposition: null,
          trigger: null,
        });

        if (triggerSlot) {
          triggerSlot.addEventListener("slotchange", () => wireTrigger(el, onTriggerClick, onTriggerKeydown));
        }
        wireTrigger(el, onTriggerClick, onTriggerKeydown);

        syncOpen(el);
      },

      disconnected(el) {
        teardownOpen(el);
        const s = menuState.get(el);
        if (!s) return;
        el.removeEventListener("lds-select", s.onSelect);
        const panel = el.$(".panel");
        if (panel) panel.removeEventListener("keydown", s.onPanelKeydown);
        unwireTrigger(el, s.onTriggerClick, s.onTriggerKeydown);
        menuState.delete(el);
      },

      attributeChanged(el, name) {
        if (name === "open") syncOpen(el);
        if (name === "placement" && el.boolAttr("open")) positionPanel(el);
      },
    });

    function wireTrigger(el, onClick, onKeydown) {
      const trigger = triggerOf(el);
      const s = menuState.get(el);
      if (s && s.trigger) {
        s.trigger.removeEventListener("click", s.onTriggerClick);
        s.trigger.removeEventListener("keydown", s.onTriggerKeydown);
      }
      if (!trigger) return;
      trigger.addEventListener("click", onClick);
      trigger.addEventListener("keydown", onKeydown);
      if (s) s.trigger = trigger;
      syncTriggerA11y(el, trigger);
    }

    function unwireTrigger(el, onClick, onKeydown) {
      const trigger = triggerOf(el);
      if (!trigger) return;
      trigger.removeEventListener("click", onClick);
      trigger.removeEventListener("keydown", onKeydown);
    }

    function triggerOf(el) {
      const slot = el.$('.trigger slot[name="trigger"]');
      if (!slot) return null;
      const assigned = slot.assignedElements({ flatten: true });
      return assigned[0] || null;
    }

    function syncTriggerA11y(el, trigger) {
      if (!trigger) trigger = triggerOf(el);
      if (!trigger) return;
      trigger.setAttribute("aria-haspopup", "menu");
      trigger.setAttribute("aria-expanded", el.boolAttr("open") ? "true" : "false");
      const panel = el.$(".panel");
      if (panel.id) trigger.setAttribute("aria-controls", panel.id);
    }

    function toggle(el) {
      if (el.boolAttr("open")) closeMenu(el, "toggle");
      else openMenu(el, { focus: "first" });
    }

    function openMenu(el, opts) {
      opts = opts || {};
      if (el.boolAttr("open")) return;
      const ev = el.emit("lds-open");
      if (ev.defaultPrevented) return;
      el.setAttribute("open", "");
      if (opts.focus) {
        requestAnimationFrame(() => {
          const items = enabledItems(el);
          if (!items.length) return;
          focusItem(opts.focus === "last" ? items[items.length - 1] : items[0]);
        });
      }
    }

    function closeMenu(el, reason) {
      if (!el.boolAttr("open")) return;
      const ev = el.emit("lds-close", { reason: reason || "dismiss" });
      if (ev.defaultPrevented) return;
      el.removeAttribute("open");
    }

    function syncOpen(el) {
      const panel = el.$(".panel");
      const open = el.boolAttr("open");
      panel.hidden = !open;
      syncTriggerA11y(el);

      if (open) {
        setupOpen(el);
      } else {
        teardownOpen(el);
      }
    }

    function setupOpen(el) {
      const s = menuState.get(el);
      const panel = el.$(".panel");
      const trigger = triggerOf(el);
      if (!s || !panel) return;

      teardownOpen(el);

      positionPanel(el);

      const reposition = () => positionPanel(el);
      window.addEventListener("resize", reposition);
      window.addEventListener("scroll", reposition, true);
      s.reposition = reposition;

      const focusTarget = focusableIn(trigger) || trigger;
      s.overlay = LDS.overlay.bind(panel, {
        restoreFocus: focusTarget,
        ignoreOutside: trigger ? [trigger, el] : [el],
        onDismiss: (reason) => closeMenu(el, reason),
      });
    }

    function teardownOpen(el) {
      const s = menuState.get(el);
      if (!s) return;
      if (s.overlay) {
        s.overlay.disconnect();
        s.overlay = null;
      }
      if (s.reposition) {
        window.removeEventListener("resize", s.reposition);
        window.removeEventListener("scroll", s.reposition, true);
        s.reposition = null;
      }
    }

    function positionPanel(el) {
      const panel = el.$(".panel");
      const trigger = triggerOf(el);
      if (!panel || !trigger) return;
      const placement = el.getAttribute("placement") || "bottom-end";
      LDS.position(panel, trigger, { placement: placement, matchWidth: true });
    }

    function itemsOf(el) {
      return Array.prototype.slice.call(el.querySelectorAll(":scope > lds-menu-item"));
    }

    function enabledItems(el) {
      return itemsOf(el).filter((item) => !item.boolAttr("disabled") && !item.hidden);
    }

    function focusableIn(host) {
      if (!host) return null;
      if (host.shadowRoot) {
        const inner = host.shadowRoot.querySelector(
          'button:not([disabled]),a[href],[tabindex]:not([tabindex="-1"])'
        );
        if (inner) return inner;
      }
      return host;
    }

    function focusItem(item) {
      if (!item) return;
      const control = item.$ ? item.$(".item") : null;
      if (control && typeof control.focus === "function") control.focus();
      else if (typeof item.focus === "function") item.focus();
    }

    function focusedItemIndex(items) {
      let active = document.activeElement;
      while (active && active.shadowRoot && active.shadowRoot.activeElement) {
        active = active.shadowRoot.activeElement;
      }
      for (let i = 0; i < items.length; i++) {
        const control = items[i].$ ? items[i].$(".item") : null;
        if (items[i] === active || control === active) return i;
        if (control && control.contains && control.contains(active)) return i;
      }
      return -1;
    }
  })();

  /* behaviour: lds-menu-item */
  (function () {
    /* Behaviour for <lds-menu-item>.
     *
     * Renders an inner <a> when href is set, otherwise a <button>. Activation
     * emits lds-select; the parent lds-menu closes on that event.
     */

    LDS.behavior("lds-menu-item", {
      connected(el) {
        ensureControl(el);
        watchSlot(el, ".item__start slot", ".item__start");
        watchSlot(el, ".item__end slot", ".item__end");

        el.addEventListener(
          "click",
          (event) => {
            if (el.boolAttr("disabled")) {
              event.preventDefault();
              event.stopImmediatePropagation();
              return;
            }
            activate(el, event);
          },
          true
        );
      },

      attributeChanged(el, name) {
        if (!el._lds.ready) return;
        if (name === "href" || name === "disabled" || name === "value" || name === "target") {
          ensureControl(el);
        }
      },
    });

    function ensureControl(el) {
      const wantsLink = !!el.getAttribute("href");
      let item = el.$(".item");
      const tag = wantsLink ? "a" : "button";

      if (!item || item.tagName.toLowerCase() !== tag) {
        const next = document.createElement(tag);
        next.className = "item";
        next.setAttribute("part", "item");
        next.setAttribute("role", "menuitem");
        if (tag === "button") next.type = "button";

        if (item) {
          while (item.firstChild) next.appendChild(item.firstChild);
          item.replaceWith(next);
        } else {
          el.shadowRoot.appendChild(next);
        }
        item = next;
      }

      syncControl(el, item);
    }

    function syncControl(el, item) {
      const disabled = el.boolAttr("disabled");

      if (item.tagName.toLowerCase() === "a") {
        const href = el.getAttribute("href") || "#";
        item.setAttribute("href", href);
        const target = el.getAttribute("target");
        if (target) {
          item.setAttribute("target", target);
          if (target === "_blank" && !item.hasAttribute("rel")) {
            item.setAttribute("rel", "noopener noreferrer");
          }
        } else {
          item.removeAttribute("target");
        }
        if (disabled) {
          item.setAttribute("aria-disabled", "true");
          item.setAttribute("tabindex", "-1");
        } else {
          item.removeAttribute("aria-disabled");
          item.removeAttribute("tabindex");
        }
      } else {
        item.disabled = disabled;
        item.removeAttribute("aria-disabled");
      }
    }

    function activate(el, originalEvent) {
      if (el.boolAttr("disabled")) return;

      const detail = { originalEvent: originalEvent };
      const value = el.getAttribute("value");
      if (value != null) detail.value = value;

      el.emit("lds-select", detail);
    }

    function watchSlot(el, slotSel, wrapSel) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;

      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && node.textContent.trim() === ""
        );
        wrap.toggleAttribute("data-empty", empty);
      };

      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-nav */
  (function () {
    /* Behaviour for <lds-nav>.
     *
     * CSS cannot see whether a named slot has assigned nodes, so brand and footer
     * wrappers get data-empty for collapse.
     */

    LDS.behavior("lds-nav", {
      connected(el) {
        watchSlot(el, 'slot[name="brand"]', ".brand");
        watchSlot(el, 'slot[name="footer"]', ".footer");
      },
    });

    function watchSlot(el, slotSel, wrapSel) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;

      const update = () => {
        wrap.toggleAttribute("data-empty", slot.assignedNodes({ flatten: true }).length === 0);
      };

      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-nav-group */
  (function () {
    /* Behaviour for <lds-nav-group>.
     *
     * Toggles the `open` attribute; aria-expanded tracks it. Cancel lds-toggle to
     * keep the previous state.
     *
     * With href, the header is a link to the group's own page (a project) and the
     * disclosure moves to a separate button beside it, since a link cannot also
     * be a toggle.
     */

    LDS.behavior("lds-nav-group", {
      connected(el) {
        const trigger = el.$(".trigger");

        syncLabel(el, trigger);
        syncExpanded(el, trigger);
        syncHeader(el);
        watchSlot(el, 'slot[name="start"]', ".link__start");
        watchSlot(el, 'slot[name="end"]', ".link__end");

        trigger.addEventListener("click", () => {
          setOpen(el, !el.boolAttr("open"));
        });
        el.$(".toggle").addEventListener("click", () => {
          setOpen(el, !el.boolAttr("open"));
        });
      },

      attributeChanged(el, name) {
        const trigger = el.$(".trigger");
        if (!trigger) return;

        if (name === "label") syncLabel(el, trigger);
        if (name === "open") syncExpanded(el, trigger);
        if (name === "href" || name === "current" || name === "label") syncHeader(el);
      },
    });

    function setOpen(el, open) {
      if (el.boolAttr("open") === open) return;

      const event = el.emit("lds-toggle", { open: open });
      if (event.defaultPrevented) return;

      el.toggleAttribute("open", open);
    }

    function syncLabel(el, trigger) {
      const label = el.$(".trigger__label");
      const text = el.getAttribute("label") || "";
      if (label) label.textContent = text;
      if (text) {
        trigger.removeAttribute("aria-label");
      } else {
        trigger.setAttribute("aria-label", "Section");
      }
    }

    function syncExpanded(el, trigger) {
      const expanded = el.boolAttr("open") ? "true" : "false";
      trigger.setAttribute("aria-expanded", expanded);
      const toggle = el.$(".toggle");
      if (toggle) toggle.setAttribute("aria-expanded", expanded);
    }

    function syncHeader(el) {
      const header = el.$(".header");
      const link = el.$(".link");
      const toggle = el.$(".toggle");
      const href = el.getAttribute("href");
      const label = el.getAttribute("label") || "";

      header.hidden = !href;
      el.$(".trigger").hidden = !!href;
      if (!href) return;

      link.setAttribute("href", href);
      el.$(".link__label").textContent = label;
      /* In a collapsed rail only the start content shows, so the link keeps its
       * name through aria-label. */
      link.setAttribute("aria-label", label);
      if (el.boolAttr("current")) link.setAttribute("aria-current", "page");
      else link.removeAttribute("aria-current");
      toggle.setAttribute("aria-label", label ? "Pages in " + label : "Pages");
    }

    function watchSlot(el, slotSel, wrapSel) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;
      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        wrap.toggleAttribute("data-empty", empty);
      };
      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-nav-item */
  (function () {
    /* Behaviour for <lds-nav-item>.
     *
     * Mirrors href / target / active / disabled onto the inner <a>, and blocks
     * activation while disabled (the host itself is not a native disabled control).
     */

    LDS.behavior("lds-nav-item", {
      connected(el) {
        const item = el.$(".item");

        syncLink(el, item);
        syncActive(el, item);
        syncDisabled(el, item);
        watchSlot(el, ".item__start slot", ".item__start");
        watchSlot(el, ".item__end slot", ".item__end");
        watchSlot(el, ".item__label slot", ".item__label", () => syncName(el, item));
        syncName(el, item);

        el.addEventListener(
          "click",
          (event) => {
            if (el.boolAttr("disabled")) {
              event.preventDefault();
              event.stopImmediatePropagation();
            }
          },
          true
        );
      },

      attributeChanged(el, name) {
        const item = el.$(".item");
        if (!item) return;

        if (name === "href" || name === "target") syncLink(el, item);
        if (name === "active") syncActive(el, item);
        if (name === "disabled") syncDisabled(el, item);
        if (name === "label") syncName(el, item);
      },
    });

    function syncLink(el, item) {
      const href = el.getAttribute("href");
      if (href) {
        item.setAttribute("href", href);
      } else {
        item.removeAttribute("href");
      }

      const target = el.getAttribute("target");
      if (target) {
        item.setAttribute("target", target);
        if (target === "_blank" && !item.hasAttribute("rel")) {
          item.setAttribute("rel", "noopener noreferrer");
        }
      } else {
        item.removeAttribute("target");
      }
    }

    function syncActive(el, item) {
      if (el.boolAttr("active")) {
        item.setAttribute("aria-current", "page");
      } else {
        item.removeAttribute("aria-current");
      }
    }

    function syncDisabled(el, item) {
      const disabled = el.boolAttr("disabled");
      item.toggleAttribute("aria-disabled", disabled);
      if (disabled) {
        item.setAttribute("tabindex", "-1");
      } else {
        item.removeAttribute("tabindex");
      }
    }

    /* Keep an accessible name when the visible label is hidden in a collapsed rail. */
    function syncName(el, item) {
      const explicit = el.getAttribute("label");
      if (explicit) {
        item.setAttribute("aria-label", explicit);
        return;
      }

      const slot = el.$(".item__label slot");
      if (!slot) return;

      const text = slot
        .assignedNodes({ flatten: true })
        .map((node) => node.textContent || "")
        .join("")
        .trim();

      if (text) {
        item.setAttribute("aria-label", text);
      } else {
        item.removeAttribute("aria-label");
      }
    }

    function watchSlot(el, slotSel, wrapSel, onChange) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;

      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && node.textContent.trim() === ""
        );
        wrap.toggleAttribute("data-empty", empty);
        if (onChange) onChange();
      };

      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-navbar */
  (function () {
    /* Behaviour for <lds-navbar>.
     *
     * Compact mode is either forced with the `compact` attribute or inferred from
     * host width vs --lds-navbar-compact-at. The toggle expands an inline links
     * panel under the bar — not a modal, so there is no focus trap.
     */

    const state = new WeakMap();

    LDS.behavior("lds-navbar", {
      connected(el) {
        const toggle = el.$(".toggle");

        syncLabel(el, toggle);
        syncExpanded(el, toggle);

        const onToggle = () => setOpen(el, !el.boolAttr("open"));
        toggle.addEventListener("click", onToggle);

        const onKeydown = (event) => {
          if (event.key === "Escape" && el.boolAttr("open") && isCompact(el)) {
            setOpen(el, false);
          }
        };

        const onPointerDown = (event) => {
          if (!el.boolAttr("open") || !isCompact(el)) return;
          if (event.composedPath().includes(el)) return;
          setOpen(el, false);
        };

        document.addEventListener("keydown", onKeydown);
        document.addEventListener("pointerdown", onPointerDown, true);

        const ro = new ResizeObserver(() => updateCompact(el));
        ro.observe(el);
        updateCompact(el);

        state.set(el, { onToggle, onKeydown, onPointerDown, ro });
      },

      disconnected(el) {
        const s = state.get(el);
        if (!s) return;

        document.removeEventListener("keydown", s.onKeydown);
        document.removeEventListener("pointerdown", s.onPointerDown, true);
        s.ro.disconnect();
        state.delete(el);
      },

      attributeChanged(el, name) {
        const toggle = el.$(".toggle");
        if (!toggle) return;

        if (name === "label") syncLabel(el, toggle);
        if (name === "open") syncExpanded(el, toggle);
        if (name === "compact") updateCompact(el);
      },
    });

    function isCompact(el) {
      return el.boolAttr("compact") || el.hasAttribute("data-compact");
    }

    function compactAt(el) {
      const raw = getComputedStyle(el).getPropertyValue("--lds-navbar-compact-at").trim();
      const px = parseFloat(raw);
      return Number.isFinite(px) ? px : 768;
    }

    function updateCompact(el) {
      if (el.boolAttr("compact")) {
        el.removeAttribute("data-compact");
        return;
      }

      const should = el.clientWidth > 0 && el.clientWidth < compactAt(el);
      const was = el.hasAttribute("data-compact");
      el.toggleAttribute("data-compact", should);

      if (was && !should && el.boolAttr("open")) {
        el.removeAttribute("open");
      }
    }

    function setOpen(el, open) {
      if (el.boolAttr("open") === open) return;

      const event = el.emit("lds-toggle", { open: open });
      if (event.defaultPrevented) return;

      el.toggleAttribute("open", open);
    }

    function syncLabel(el, toggle) {
      toggle.setAttribute("aria-label", el.getAttribute("label") || "Menu");
    }

    function syncExpanded(el, toggle) {
      toggle.setAttribute("aria-expanded", el.boolAttr("open") ? "true" : "false");
    }
  })();

  /* behaviour: lds-code */
  (function () {
    /* Behaviour for <lds-code> — copy button and language metadata. */

    LDS.behavior("lds-code", {
      connected(el) {
        const copy = el.$(".copy");
        syncCopyable(el, copy);
        syncLanguage(el);

        copy.addEventListener("click", () => {
          const text = slotText(el);
          const write =
            navigator.clipboard && navigator.clipboard.writeText
              ? navigator.clipboard.writeText(text)
              : Promise.reject(new Error("clipboard unavailable"));

          write
            .then(() => {
              el.emit("lds-copy", { ok: true });
            })
            .catch(() => {
              el.emit("lds-copy", { ok: false });
            });
        });
      },

      attributeChanged(el, name) {
        if (!el._lds.ready) return;
        if (name === "copyable") syncCopyable(el, el.$(".copy"));
        if (name === "language") syncLanguage(el);
      },
    });

    function isCopyable(el) {
      /* Default true when attribute omitted; literal "false" turns it off. */
      if (!el.hasAttribute("copyable")) return true;
      return el.getAttribute("copyable") !== "false";
    }

    function syncCopyable(el, copy) {
      if (!copy) return;
      copy.hidden = !isCopyable(el);
    }

    function syncLanguage(el) {
      const lang = el.getAttribute("language");
      const code = el.$(".code");
      if (!code) return;
      if (lang) code.setAttribute("data-language", lang);
      else code.removeAttribute("data-language");
    }

    function slotText(el) {
      const slot = el.$("slot");
      if (!slot) return el.textContent || "";
      return slot
        .assignedNodes({ flatten: true })
        .map((node) => node.textContent || "")
        .join("")
        .replace(/\n$/, "");
    }
  })();

  /* behaviour: lds-empty-state */
  (function () {
    /* Behaviour for <lds-empty-state> — hide unused regions so the gap between
     * them stays even. CSS cannot see whether a slot has content. */

    LDS.behavior("lds-empty-state", {
      connected(el) {
        watchSlot(el, 'slot[name="icon"]', ".icon");
        watchSlot(el, 'slot[name="title"]', ".title");
        watchSlot(el, ".body slot", ".body");
        watchSlot(el, 'slot[name="actions"]', ".actions");
      },
    });

    function watchSlot(el, slotSel, wrapSel) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;
      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        wrap.toggleAttribute("data-empty", empty);
      };
      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-pagination */
  (function () {
    /* Behaviour for <lds-pagination>.
     *
     * Rebuilds page buttons with an ellipsis window, wires prev/next and a native
     * page-size select, and reflects page / page-size after successful events.
     */

    const DEFAULT_SIZES = [10, 25, 50, 100];

    LDS.behavior("lds-pagination", {
      connected(el) {
        const select = el.$(".sizes__select");
        const pages = el.$(".pages");

        select.addEventListener("change", () => {
          const pageSize = parseInt(select.value, 10);
          if (!Number.isFinite(pageSize) || pageSize < 1) return;
          setPageSize(el, pageSize);
        });

        pages.addEventListener("click", (event) => {
          const btn = event.target.closest("button[data-page]");
          if (!btn || btn.disabled || !pages.contains(btn)) return;
          const page = parseInt(btn.getAttribute("data-page"), 10);
          if (!Number.isFinite(page)) return;
          setPage(el, page);
        });

        syncAll(el);
      },

      attributeChanged(el) {
        if (!el._lds.ready) return;
        syncAll(el);
      },
    });

    function syncAll(el) {
      syncLabel(el);
      syncSizes(el);
      syncSummary(el);
      syncPages(el);
    }

    function syncLabel(el) {
      const nav = el.$(".root");
      const label = el.getAttribute("label") || "Pagination";
      nav.setAttribute("aria-label", label);
    }

    function num(el, name, fallback) {
      const raw = el.getAttribute(name);
      if (raw == null || raw === "") return fallback;
      const n = Number(raw);
      return Number.isFinite(n) ? n : fallback;
    }

    function totalOf(el) {
      return Math.max(0, Math.floor(num(el, "total", 0)));
    }

    function pageSizeOf(el) {
      return Math.max(1, Math.floor(num(el, "page-size", 25)));
    }

    function pageCountOf(el) {
      const total = totalOf(el);
      const size = pageSizeOf(el);
      if (total === 0) return 1;
      return Math.max(1, Math.ceil(total / size));
    }

    function pageOf(el) {
      const max = pageCountOf(el);
      const page = Math.floor(num(el, "page", 1));
      return Math.min(Math.max(1, page), max);
    }

    function parseSizes(el) {
      const raw = el.getAttribute("sizes");
      if (raw == null) return DEFAULT_SIZES.slice();
      const parts = raw
        .split(",")
        .map((s) => parseInt(s.trim(), 10))
        .filter((n) => Number.isFinite(n) && n > 0);
      return parts.length ? parts : DEFAULT_SIZES.slice();
    }

    function syncSizes(el) {
      const wrap = el.$(".sizes");
      const select = el.$(".sizes__select");
      const hidden = el.boolAttr("hide-sizes");
      wrap.toggleAttribute("data-hidden", hidden);

      const sizes = parseSizes(el);
      const current = pageSizeOf(el);
      if (sizes.indexOf(current) === -1) sizes.push(current);
      sizes.sort((a, b) => a - b);

      const prev = select.value;
      select.innerHTML = "";
      sizes.forEach((size) => {
        const opt = document.createElement("option");
        opt.value = String(size);
        opt.textContent = String(size);
        select.appendChild(opt);
      });
      select.value = String(current);
      if (select.value !== String(current) && prev) {
        /* keep best effort */
      }
      select.disabled = el.boolAttr("disabled") || hidden;
    }

    function syncSummary(el) {
      const summary = el.$(".summary");
      const total = totalOf(el);
      if (total === 0) {
        summary.textContent = "";
        summary.toggleAttribute("data-empty", true);
        return;
      }

      const size = pageSizeOf(el);
      const page = pageOf(el);
      const from = (page - 1) * size + 1;
      const to = Math.min(page * size, total);
      summary.textContent = "Showing " + from + "\u2013" + to + " of " + total;
      summary.toggleAttribute("data-empty", false);
    }

    function syncPages(el) {
      const host = el.$(".pages");
      const page = pageOf(el);
      const pageCount = pageCountOf(el);
      const disabled = el.boolAttr("disabled");

      host.innerHTML = "";

      host.appendChild(
        pageButton(el, {
          page: page - 1,
          label: "Previous page",
          icon: "chevron-left",
          disabled: disabled || page <= 1,
        })
      );

      pageWindow(page, pageCount).forEach((item) => {
        if (item === "…") {
          const span = document.createElement("span");
          span.className = "ellipsis";
          span.setAttribute("aria-hidden", "true");
          span.textContent = "\u2026";
          host.appendChild(span);
          return;
        }
        host.appendChild(
          pageButton(el, {
            page: item,
            label: "Page " + item,
            text: String(item),
            current: item === page,
            disabled: disabled,
          })
        );
      });

      host.appendChild(
        pageButton(el, {
          page: page + 1,
          label: "Next page",
          icon: "chevron-right",
          disabled: disabled || page >= pageCount,
        })
      );
    }

    function pageButton(el, opts) {
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "btn" + (opts.icon ? " btn--icon" : "");
      btn.setAttribute("data-page", String(opts.page));
      btn.setAttribute("aria-label", opts.label);
      if (opts.current) btn.setAttribute("aria-current", "page");
      btn.disabled = !!opts.disabled;

      if (opts.icon) {
        const icon = document.createElement("lds-icon");
        icon.setAttribute("name", opts.icon);
        icon.setAttribute("size", "sm");
        btn.appendChild(icon);
      } else {
        btn.textContent = opts.text || "";
      }
      return btn;
    }

    /* first, last, current ± 1, with ellipsis gaps */
    function pageWindow(page, pageCount) {
      if (pageCount <= 7) {
        const all = [];
        for (let i = 1; i <= pageCount; i++) all.push(i);
        return all;
      }

      const set = new Set([1, pageCount, page, page - 1, page + 1]);
      const nums = Array.from(set)
        .filter((n) => n >= 1 && n <= pageCount)
        .sort((a, b) => a - b);

      const out = [];
      for (let i = 0; i < nums.length; i++) {
        if (i > 0 && nums[i] - nums[i - 1] > 1) out.push("…");
        out.push(nums[i]);
      }
      return out;
    }

    function setPage(el, page) {
      const max = pageCountOf(el);
      const next = Math.min(Math.max(1, page), max);
      if (next === pageOf(el)) return;

      const event = el.emit("lds-page-change", { page: next });
      if (event.defaultPrevented) return;
      el.setAttribute("page", String(next));
    }

    function setPageSize(el, pageSize) {
      const size = Math.max(1, Math.floor(pageSize));
      let page = pageOf(el);
      const total = totalOf(el);
      const newCount = total === 0 ? 1 : Math.max(1, Math.ceil(total / size));
      if (page > newCount) page = 1;

      const event = el.emit("lds-page-size-change", { pageSize: size, page: page });
      if (event.defaultPrevented) return;

      el.setAttribute("page-size", String(size));
      el.setAttribute("page", String(page));
    }
  })();

  /* behaviour: lds-skeleton */
  (function () {
    /* Behaviour for <lds-skeleton> — render one bar per line and hide the
     * placeholder from assistive technology. The region being loaded should carry
     * aria-busy="true"; the skeleton itself says nothing. */

    LDS.behavior("lds-skeleton", {
      connected(el) {
        el.setAttribute("aria-hidden", "true");
        render(el);
      },

      attributeChanged(el) {
        if (!el._lds.ready) return;
        render(el);
      },
    });

    function render(el) {
      const lines = el.$(".lines");
      const shape = el.getAttribute("shape") || "text";
      let count = shape === "text" ? parseInt(el.getAttribute("lines"), 10) : 1;
      if (Number.isNaN(count) || count < 1) count = 1;

      lines.textContent = "";
      for (let i = 0; i < count; i++) {
        const bar = document.createElement("span");
        bar.className = "bar";
        if (count > 1 && i === count - 1) bar.setAttribute("data-last", "");
        lines.appendChild(bar);
      }
    }
  })();

  /* behaviour: lds-table */
  (function () {
    /* Behaviour for <lds-table>.
     *
     * The <table> stays in light DOM. When sortable, th[data-sortable] cycles
     * asc → desc → none, sets aria-sort, and reorders tbody rows unless lds-sort
     * is canceled (server-sort escape hatch).
     */

    const tableState = new WeakMap();

    LDS.behavior("lds-table", {
      connected(el) {
        const onClick = (event) => {
          if (!el.boolAttr("sortable")) return;
          const th = event.target.closest("th[data-sortable]");
          if (!th || !el.contains(th)) return;
          event.preventDefault();
          cycleSort(el, th);
        };

        const onKeydown = (event) => {
          if (!el.boolAttr("sortable")) return;
          if (event.key !== "Enter" && event.key !== " ") return;
          const th = event.target.closest("th[data-sortable]");
          if (!th || !el.contains(th)) return;
          event.preventDefault();
          cycleSort(el, th);
        };

        el.addEventListener("click", onClick);
        el.addEventListener("keydown", onKeydown);
        tableState.set(el, { onClick: onClick, onKeydown: onKeydown });

        syncSortable(el);
      },

      disconnected(el) {
        const s = tableState.get(el);
        if (!s) return;
        el.removeEventListener("click", s.onClick);
        el.removeEventListener("keydown", s.onKeydown);
        tableState.delete(el);
      },

      attributeChanged(el, name) {
        if (name === "sortable") syncSortable(el);
      },
    });

    function tableOf(el) {
      return el.querySelector(":scope > table");
    }

    function syncSortable(el) {
      const table = tableOf(el);
      if (!table) return;

      const headers = Array.prototype.slice.call(
        table.querySelectorAll("thead th[data-sortable]")
      );

      headers.forEach((th) => {
        if (el.boolAttr("sortable")) {
          if (!th.hasAttribute("tabindex")) th.setAttribute("tabindex", "0");
          th.setAttribute("role", "columnheader");
        } else {
          th.removeAttribute("tabindex");
          if (th.getAttribute("role") === "columnheader") th.removeAttribute("role");
          th.removeAttribute("aria-sort");
        }
      });
    }

    function cycleSort(el, th) {
      const current = th.getAttribute("aria-sort");
      let direction = null;
      if (current === "ascending") direction = "desc";
      else if (current === "descending") direction = null;
      else direction = "asc";

      const key = th.getAttribute("data-key") || null;
      const detail = {
        column: th,
        direction: direction,
      };
      if (key) detail.key = key;

      const event = el.emit("lds-sort", detail);
      if (event.defaultPrevented) {
        applyAriaSort(el, th, direction);
        return;
      }

      applyAriaSort(el, th, direction);
      if (direction) {
        sortBody(el, th, direction);
      } else {
        restoreOrder(el);
      }
    }

    function applyAriaSort(el, activeTh, direction) {
      const table = tableOf(el);
      if (!table) return;

      Array.prototype.forEach.call(table.querySelectorAll("thead th[data-sortable]"), (th) => {
        if (th === activeTh && direction === "asc") {
          th.setAttribute("aria-sort", "ascending");
        } else if (th === activeTh && direction === "desc") {
          th.setAttribute("aria-sort", "descending");
        } else {
          th.removeAttribute("aria-sort");
        }
      });
    }

    function cellIndex(th) {
      const row = th.parentElement;
      if (!row) return -1;
      return Array.prototype.indexOf.call(row.children, th);
    }

    function sortBody(el, th, direction) {
      const table = tableOf(el);
      if (!table) return;
      const tbody = table.tBodies[0];
      if (!tbody) return;

      rememberOrder(el, tbody);

      const index = cellIndex(th);
      if (index < 0) return;

      const type = th.getAttribute("data-type") || "string";
      const rows = Array.prototype.slice.call(tbody.rows);
      const factor = direction === "desc" ? -1 : 1;

      rows.sort((a, b) => {
        const av = cellValue(a.cells[index], type);
        const bv = cellValue(b.cells[index], type);
        if (av == null && bv == null) return 0;
        if (av == null) return 1;
        if (bv == null) return -1;
        if (typeof av === "number" && typeof bv === "number") {
          return (av - bv) * factor;
        }
        return String(av).localeCompare(String(bv), undefined, { numeric: true, sensitivity: "base" }) * factor;
      });

      rows.forEach((row) => tbody.appendChild(row));
    }

    function cellValue(cell, type) {
      if (!cell) return null;
      const raw = (cell.getAttribute("data-value") || cell.textContent || "").trim();
      if (!raw) return null;

      if (type === "number") {
        const n = Number(raw.replace(/,/g, ""));
        return Number.isFinite(n) ? n : null;
      }
      if (type === "date") {
        const t = Date.parse(raw);
        return Number.isFinite(t) ? t : null;
      }
      return raw;
    }

    function rememberOrder(el, tbody) {
      const s = tableState.get(el);
      if (!s || s.originalOrder) return;
      s.originalOrder = Array.prototype.slice.call(tbody.rows);
    }

    function restoreOrder(el) {
      const s = tableState.get(el);
      const table = tableOf(el);
      if (!s || !s.originalOrder || !table || !table.tBodies[0]) return;
      const tbody = table.tBodies[0];
      s.originalOrder.forEach((row) => {
        if (row.parentNode === tbody || table.contains(row)) tbody.appendChild(row);
      });
    }
  })();

  /* behaviour: lds-timeline-item */
  (function () {
    /* Behaviour for <lds-timeline-item> — sync time text. */

    LDS.behavior("lds-timeline-item", {
      connected(el) {
        syncTime(el);
      },

      attributeChanged(el, name) {
        if (!el._lds.ready) return;
        if (name === "time") syncTime(el);
      },
    });

    function syncTime(el) {
      const node = el.$(".time");
      if (!node) return;
      node.textContent = el.getAttribute("time") || "";
    }
  })();

  /* behaviour: lds-alert */
  (function () {
    /* Behaviour for <lds-alert> — inline persistent message. */

    LDS.behavior("lds-alert", {
      connected(el) {
        const close = el.$(".close");
        const root = el.$(".root");

        syncRole(el, root);
        syncDismissible(el, close);
        syncFallbackIcon(el);
        watchSlot(el, 'slot[name="title"]', ".title");
        watchDescription(el);
        watchSlot(el, 'slot[name="actions"]', ".actions");
        watchIconSlot(el);

        close.addEventListener("click", () => dismiss(el, "close"));
      },

      attributeChanged(el, name) {
        if (!el._lds.ready) return;
        const close = el.$(".close");
        const root = el.$(".root");
        if (name === "intent") {
          syncRole(el, root);
          syncFallbackIcon(el);
        }
        if (name === "dismissible" && close) syncDismissible(el, close);
      },
    });

    function isDismissible(el) {
      return el.boolAttr("dismissible");
    }

    function syncDismissible(el, close) {
      close.hidden = !isDismissible(el);
    }

    function syncRole(el, root) {
      root.setAttribute(
        "role",
        el.getAttribute("intent") === "danger" ? "alert" : "status"
      );
    }

    function syncFallbackIcon(el) {
      const intent = el.getAttribute("intent") || "info";
      el.$$(".icon__fallback").forEach((icon) => {
        icon.hidden = icon.getAttribute("data-intent") !== intent;
      });
    }

    function dismiss(el, reason) {
      if (el.hasAttribute("hidden")) return;

      const event = el.emit("lds-dismiss", { reason: reason });
      if (event.defaultPrevented) return;

      el.setAttribute("hidden", "");
    }

    function slotEmpty(slot) {
      return slot.assignedNodes({ flatten: true }).every(
        (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
      );
    }

    function watchSlot(el, slotSel, wrapSel) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;

      const update = () => wrap.toggleAttribute("data-empty", slotEmpty(slot));
      slot.addEventListener("slotchange", update);
      update();
    }

    function watchDescription(el) {
      const named = el.$('slot[name="description"]');
      const def = el.$(".description > slot:not([name])");
      const wrap = el.$(".description");
      if (!wrap) return;

      const update = () => {
        const empty =
          (!named || slotEmpty(named)) && (!def || slotEmpty(def));
        wrap.toggleAttribute("data-empty", empty);
      };

      if (named) named.addEventListener("slotchange", update);
      if (def) def.addEventListener("slotchange", update);
      update();
    }

    function watchIconSlot(el) {
      const slot = el.$('slot[name="icon"]');
      const wrap = el.$(".icon");
      if (!slot || !wrap) return;

      const update = () => {
        const empty = slotEmpty(slot);
        wrap.toggleAttribute("data-empty-slot", empty);
        el.$$(".icon__fallback").forEach((icon) => {
          if (!empty) icon.hidden = true;
          else icon.hidden = icon.getAttribute("data-intent") !== (el.getAttribute("intent") || "info");
        });
      };

      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-banner */
  (function () {
    /* Behaviour for <lds-banner> — page-level announcement strip. */

    LDS.behavior("lds-banner", {
      connected(el) {
        const close = el.$(".close");
        const root = el.$(".root");

        syncRole(el, root);
        syncDismissible(el, close);
        syncFallbackIcon(el);
        watchSlot(el, 'slot[name="title"]', ".title");
        watchSlot(el, ".body > slot", ".body");
        watchSlot(el, 'slot[name="action"]', ".action");
        watchIconSlot(el);

        close.addEventListener("click", () => dismiss(el, "close"));
      },

      attributeChanged(el, name) {
        if (!el._lds.ready) return;
        const close = el.$(".close");
        const root = el.$(".root");
        if (name === "intent") {
          syncRole(el, root);
          syncFallbackIcon(el);
        }
        if (name === "dismissible" && close) syncDismissible(el, close);
      },
    });

    function isDismissible(el) {
      return el.boolAttr("dismissible");
    }

    function syncDismissible(el, close) {
      close.hidden = !isDismissible(el);
    }

    function syncRole(el, root) {
      root.setAttribute(
        "role",
        el.getAttribute("intent") === "danger" ? "alert" : "status"
      );
    }

    function syncFallbackIcon(el) {
      const intent = el.getAttribute("intent") || "info";
      el.$$(".icon__fallback").forEach((icon) => {
        icon.hidden = icon.getAttribute("data-intent") !== intent;
      });
    }

    function dismiss(el, reason) {
      if (el.hasAttribute("hidden")) return;

      const event = el.emit("lds-dismiss", { reason: reason });
      if (event.defaultPrevented) return;

      el.setAttribute("hidden", "");
    }

    function watchSlot(el, slotSel, wrapSel) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;

      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        wrap.toggleAttribute("data-empty", empty);
      };

      slot.addEventListener("slotchange", update);
      update();
    }

    function watchIconSlot(el) {
      const slot = el.$('slot[name="icon"]');
      const wrap = el.$(".icon");
      if (!slot || !wrap) return;

      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        wrap.toggleAttribute("data-empty-slot", empty);
        /* Show fallbacks only when the icon slot is empty. */
        el.$$(".icon__fallback").forEach((icon) => {
          if (!empty) icon.hidden = true;
          else icon.hidden = icon.getAttribute("data-intent") !== (el.getAttribute("intent") || "info");
        });
      };

      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-progress */
  (function () {
    /* Behaviour for <lds-progress>. */

    LDS.behavior("lds-progress", {
      connected(el) {
        sync(el);
      },

      attributeChanged(el) {
        if (!el._lds.ready) return;
        sync(el);
      },
    });

    function sync(el) {
      const variant = el.getAttribute("variant") || "linear";
      const indeterminate = el.boolAttr("indeterminate");
      const max = Math.max(0.0001, num(el, "max", 100));
      let value = num(el, "value", 0);
      if (value < 0) value = 0;
      if (value > max) value = max;
      const ratio = indeterminate ? 0 : value / max;

      const linear = el.$(".linear");
      const circular = el.$(".circular");
      const bar = el.$(".bar");
      const ringBar = el.$(".ring__bar");
      const label = el.$(".label");

      linear.hidden = variant === "circular";
      circular.hidden = variant !== "circular";

      if (!indeterminate) {
        bar.style.width = percentage(ratio);
        const circ = 2 * Math.PI * 14;
        ringBar.style.strokeDasharray = String(circ);
        ringBar.style.strokeDashoffset = String(circ * (1 - ratio));
      } else {
        bar.style.width = "";
        ringBar.style.strokeDashoffset = "";
      }

      el.setAttribute("role", "progressbar");
      if (indeterminate) {
        el.removeAttribute("aria-valuenow");
        el.removeAttribute("aria-valuemin");
        el.removeAttribute("aria-valuemax");
      } else {
        el.setAttribute("aria-valuemin", "0");
        el.setAttribute("aria-valuemax", String(max));
        el.setAttribute("aria-valuenow", String(value));
      }

      const labelText = el.getAttribute("label");
      if (labelText) {
        label.hidden = false;
        label.textContent = labelText;
        el.setAttribute("aria-label", labelText);
      } else {
        label.hidden = true;
        label.textContent = "";
        if (!el.hasAttribute("aria-label")) {
          el.setAttribute("aria-label", "Progress");
        }
      }
    }

    function num(el, name, fallback) {
      const raw = el.getAttribute(name);
      if (raw == null || raw === "") return fallback;
      const n = Number(raw);
      return Number.isFinite(n) ? n : fallback;
    }

    function percentage(ratio) {
      return (Math.round(ratio * 1000) / 10) + "%";
    }
  })();

  /* behaviour: lds-toast */
  (function () {
    /* Behaviour for <lds-toast>.
     *
     * open controls visibility. duration (ms, default 5000; 0 = sticky) auto-
     * dismisses; the timer pauses while hovered or focus is inside. dismissible
     * defaults to true when the attribute is omitted.
     */

    const state = new WeakMap();

    LDS.behavior("lds-toast", {
      connected(el) {
        const close = el.$(".close");
        const root = el.$(".root");

        syncRole(el, root);
        syncDismissible(el, close);
        watchSlot(el, 'slot[name="title"]', ".title");
        watchSlot(el, ".body > slot", ".body");
        watchSlot(el, 'slot[name="action"]', ".action");

        close.addEventListener("click", function () {
          dismiss(el, "close");
        });

        el.addEventListener("mouseenter", function () {
          pauseTimer(el);
        });
        el.addEventListener("mouseleave", function () {
          resumeTimer(el);
        });
        el.addEventListener("focusin", function () {
          pauseTimer(el);
        });
        el.addEventListener("focusout", function () {
          requestAnimationFrame(function () {
            if (!el.matches(":focus-within")) resumeTimer(el);
          });
        });

        if (el.boolAttr("open")) startTimer(el);
      },

      disconnected(el) {
        clearTimer(el);
      },

      attributeChanged(el, name) {
        const close = el.$(".close");
        const root = el.$(".root");
        if (!root) return;

        if (name === "intent") syncRole(el, root);
        if (name === "dismissible" && close) syncDismissible(el, close);
        if (name === "open") {
          if (el.boolAttr("open")) startTimer(el);
          else clearTimer(el);
        }
        if (name === "duration" && el.boolAttr("open")) {
          clearTimer(el);
          startTimer(el);
        }
      },
    });

    function isDismissible(el) {
      if (!el.hasAttribute("dismissible")) return true;
      return el.boolAttr("dismissible");
    }

    function syncDismissible(el, close) {
      close.hidden = !isDismissible(el);
    }

    function syncRole(el, root) {
      root.setAttribute(
        "role",
        el.getAttribute("intent") === "danger" ? "alert" : "status"
      );
    }

    function durationMs(el) {
      const raw = el.getAttribute("duration");
      if (raw === null || raw === "") return 5000;
      const n = parseInt(raw, 10);
      return Number.isFinite(n) ? n : 5000;
    }

    function dismiss(el, reason) {
      if (!el.boolAttr("open")) return;

      const event = el.emit("lds-dismiss", { reason: reason });
      if (event.defaultPrevented) return;

      el.removeAttribute("open");
      clearTimer(el);

      if (el.hasAttribute("data-managed")) {
        el.remove();
      }
    }

    function clearTimer(el) {
      const s = state.get(el);
      if (!s) return;
      if (s.timer) clearTimeout(s.timer);
      s.timer = null;
      state.set(el, s);
    }

    function startTimer(el) {
      clearTimer(el);
      const ms = durationMs(el);
      if (ms <= 0) return;

      const s = state.get(el) || {};
      s.remaining = ms;
      s.started = Date.now();
      s.timer = setTimeout(function () {
        dismiss(el, "timeout");
      }, ms);
      state.set(el, s);
    }

    function pauseTimer(el) {
      const s = state.get(el);
      if (!s || !s.timer) return;
      clearTimeout(s.timer);
      s.timer = null;
      s.remaining = Math.max(0, s.remaining - (Date.now() - s.started));
      state.set(el, s);
    }

    function resumeTimer(el) {
      if (!el.boolAttr("open")) return;
      const s = state.get(el);
      if (!s || s.timer) return;
      if (s.remaining <= 0) {
        dismiss(el, "timeout");
        return;
      }
      s.started = Date.now();
      s.timer = setTimeout(function () {
        dismiss(el, "timeout");
      }, s.remaining);
      state.set(el, s);
    }

    function watchSlot(el, slotSel, wrapSel) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;

      const update = function () {
        const empty = slot.assignedNodes({ flatten: true }).every(function (node) {
          return node.nodeType === Node.TEXT_NODE && node.textContent.trim() === "";
        });
        wrap.toggleAttribute("data-empty", empty);
      };

      slot.addEventListener("slotchange", update);
      update();
    }

    /* Used by lds-toast-region Escape handling. */
    LDS._dismissToast = dismiss;
  })();

  /* behaviour: lds-toast-region */
  (function () {
    /* Behaviour for <lds-toast-region> and the LDS.toast() entry point.
     *
     * Escape dismisses the newest open toast. LDS.toast() ensures a region exists
     * on document.body, builds a managed <lds-toast>, and opens it.
     */

    const regionState = new WeakMap();

    LDS.behavior("lds-toast-region", {
      connected(el) {
        if (!el.hasAttribute("role")) el.setAttribute("role", "region");
        if (!el.hasAttribute("aria-label")) {
          el.setAttribute("aria-label", "Notifications");
        }
        el.setAttribute("aria-live", "polite");
        el.setAttribute("aria-relevant", "additions text");

        const onKeydown = function (event) {
          if (event.key !== "Escape") return;
          const open = openToasts(el);
          if (!open.length) return;
          const newest = open[open.length - 1];
          if (typeof LDS._dismissToast === "function") {
            LDS._dismissToast(newest, "escape");
          }
        };

        const syncListener = function () {
          const s = regionState.get(el);
          if (!s) return;
          const has = toastsOf(el).length > 0;
          if (has && !s.listening) {
            document.addEventListener("keydown", onKeydown);
            s.listening = true;
          } else if (!has && s.listening) {
            document.removeEventListener("keydown", onKeydown);
            s.listening = false;
          }
        };

        const slot = el.$("slot");
        if (slot) slot.addEventListener("slotchange", syncListener);

        const mo = new MutationObserver(syncListener);
        mo.observe(el, { childList: true });

        regionState.set(el, { onKeydown: onKeydown, mo: mo, listening: false });
        syncListener();
      },

      disconnected(el) {
        const s = regionState.get(el);
        if (!s) return;
        document.removeEventListener("keydown", s.onKeydown);
        s.mo.disconnect();
        regionState.delete(el);
      },
    });

    function toastsOf(el) {
      return Array.prototype.slice.call(el.querySelectorAll("lds-toast"));
    }

    function openToasts(el) {
      return toastsOf(el).filter(function (toast) {
        return toast.hasAttribute("open") && toast.getAttribute("open") !== "false";
      });
    }

    /**
     * Show a transient toast in the page toast region.
     *
     * @param {object} [opts]
     * @param {"info"|"success"|"warning"|"danger"} [opts.intent]
     * @param {string} [opts.title]
     * @param {string} [opts.description]
     * @param {number} [opts.duration] ms; 0 = sticky
     * @param {boolean} [opts.dismissible] default true
     * @param {"top-start"|"top-end"|"bottom-start"|"bottom-end"} [opts.placement]
     * @returns {HTMLElement} the <lds-toast> element
     */
    LDS.toast = function (opts) {
      opts = opts || {};

      let region = null;
      if (opts.placement) {
        region = document.querySelector(
          'lds-toast-region[placement="' + opts.placement + '"]'
        );
      }
      if (!region) region = document.querySelector("lds-toast-region");
      if (!region) {
        region = document.createElement("lds-toast-region");
        if (opts.placement) region.setAttribute("placement", opts.placement);
        document.body.appendChild(region);
      }

      const toast = document.createElement("lds-toast");
      toast.setAttribute("data-managed", "");
      toast.setAttribute("intent", opts.intent || "info");

      if (opts.duration != null) {
        toast.setAttribute("duration", String(opts.duration));
      }

      if (opts.dismissible === false) {
        toast.setAttribute("dismissible", "false");
      }

      if (opts.title) {
        const title = document.createElement("span");
        title.slot = "title";
        title.textContent = String(opts.title);
        toast.appendChild(title);
      }

      if (opts.description) {
        toast.appendChild(document.createTextNode(String(opts.description)));
      }

      region.appendChild(toast);
      toast.setAttribute("open", "");
      return toast;
    };
  })();

  /* behaviour: lds-drawer */
  (function () {
    /* Behaviour for <lds-drawer> — native <dialog> pinned to an edge.
     *
     * Modal drawers use showModal() and LDS.overlay's scroll lock, like lds-modal.
     * Non-modal drawers use show(): no backdrop, no scroll lock, no outside-click
     * dismissal, so a list beside an issue peek stays clickable. Escape still
     * closes them while focus is inside.
     */

    const drawerState = new WeakMap();

    LDS.behavior("lds-drawer", {
      connected(el) {
        const dialog = el.$(".dialog");

        watchSlot(el, 'slot[name="header"]', ".header__title");
        watchSlot(el, 'slot[name="footer"]', ".footer");

        el.$(".close").addEventListener("click", () => closeDrawer(el, "close-button"));

        /* Fired by modal dialogs on Escape; some browsers fire it for non-modal
         * ones too. Either way the drawer decides, not the platform. */
        dialog.addEventListener("cancel", (event) => {
          event.preventDefault();
          if (isDismissible(el)) closeDrawer(el, "escape");
        });

        dialog.addEventListener("keydown", (event) => {
          if (event.key !== "Escape" || event.defaultPrevented) return;
          if (!el.boolAttr("non-modal")) return;
          event.preventDefault();
          if (isDismissible(el)) closeDrawer(el, "escape");
        });

        /* On a modal dialog a click whose target is the dialog itself landed on
         * the backdrop: the chrome fills the whole panel. */
        dialog.addEventListener("click", (event) => {
          if (event.target !== dialog || el.boolAttr("non-modal")) return;
          if (isDismissible(el)) closeDrawer(el, "outside");
        });

        /* Keep the attribute honest if something else closes the dialog, e.g. a
         * slotted <form method="dialog">. */
        dialog.addEventListener("close", () => {
          if (el.boolAttr("open")) el.removeAttribute("open");
        });

        syncOpen(el);
      },

      disconnected(el) {
        teardown(el);
      },

      attributeChanged(el, name) {
        if (!el._lds.ready) return;
        if (name === "open") syncOpen(el);
        if (name === "dismissible") syncDismissible(el);
        if (name === "non-modal" && el.boolAttr("open")) {
          /* show() and showModal() cannot convert into each other. */
          teardown(el);
          el.$(".dialog").close();
          el.setAttribute("open", "");
          syncOpen(el);
        }
      },
    });

    function isDismissible(el) {
      if (!el.hasAttribute("dismissible")) return true;
      return el.getAttribute("dismissible") !== "false";
    }

    function syncDismissible(el) {
      const closeBtn = el.$(".close");
      if (closeBtn) closeBtn.hidden = !isDismissible(el);
    }

    function syncOpen(el) {
      const dialog = el.$(".dialog");
      const open = el.boolAttr("open");
      syncDismissible(el);

      if (!open) {
        teardown(el);
        if (dialog.open) dialog.close();
        return;
      }
      if (dialog.open) return;

      if (el.emit("lds-open").defaultPrevented) {
        el.removeAttribute("open");
        return;
      }

      if (el.boolAttr("non-modal")) {
        dialog.show();
        return;
      }

      dialog.showModal();
      const s = drawerState.get(el) || {};
      if (!s.overlay) {
        s.overlay = LDS.overlay.bind(dialog, {
          trapFocus: false,
          scrollLock: true,
          /* The overlay cancels Escape's keydown, which stops the dialog's own
           * cancel event, so Escape is handled here. Backdrop clicks land on the
           * dialog and are handled by its click listener. */
          onDismiss: function (reason) {
            if (reason === "escape" && isDismissible(el)) closeDrawer(el, "escape");
          },
        });
      }
      drawerState.set(el, s);
    }

    function teardown(el) {
      const s = drawerState.get(el);
      if (!s || !s.overlay) return;
      s.overlay.disconnect();
      s.overlay = null;
    }

    function closeDrawer(el, reason) {
      if (!el.boolAttr("open")) return;
      if (el.emit("lds-close", { reason: reason }).defaultPrevented) return;
      el.removeAttribute("open");
    }

    function watchSlot(el, slotSel, wrapSel) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;
      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        wrap.toggleAttribute("data-empty", empty);
      };
      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-modal */
  (function () {
    /* Behaviour for <lds-modal> — native <dialog> + overlay scroll lock. */

    const modalState = new WeakMap();

    LDS.behavior("lds-modal", {
      connected(el) {
        const dialog = el.$(".dialog");
        const closeBtn = el.$(".close");

        watchSlot(el, 'slot[name="header"]', ".header__title");
        watchSlot(el, 'slot[name="footer"]', ".footer");

        closeBtn.addEventListener("click", () => closeModal(el, "close-button"));

        dialog.addEventListener("cancel", (event) => {
          if (!isDismissible(el)) {
            event.preventDefault();
            return;
          }
          event.preventDefault();
          closeModal(el, "escape");
        });

        dialog.addEventListener("click", (event) => {
          if (event.target !== dialog) return;
          if (!isDismissible(el)) return;
          closeModal(el, "outside");
        });

        syncOpen(el);
      },

      disconnected(el) {
        teardown(el);
      },

      attributeChanged(el, name) {
        if (!el._lds.ready) return;
        if (name === "open") syncOpen(el);
        if (name === "dismissible") syncDismissible(el);
      },
    });

    function isDismissible(el) {
      if (!el.hasAttribute("dismissible")) return true;
      return el.getAttribute("dismissible") !== "false";
    }

    function syncDismissible(el) {
      const closeBtn = el.$(".close");
      if (closeBtn) closeBtn.hidden = !isDismissible(el);
    }

    function syncOpen(el) {
      const dialog = el.$(".dialog");
      const open = el.boolAttr("open");
      syncDismissible(el);

      if (open) {
        if (!dialog.open) {
          const ev = el.emit("lds-open");
          if (ev.defaultPrevented) {
            el.removeAttribute("open");
            return;
          }
          dialog.showModal();
        }
        setupOverlay(el);
      } else {
        teardown(el);
        if (dialog.open) dialog.close();
      }
    }

    function setupOverlay(el) {
      const s = modalState.get(el) || {};
      if (s.overlay) return;
      const dialog = el.$(".dialog");
      s.overlay = LDS.overlay.bind(dialog, {
        trapFocus: false,
        scrollLock: true,
        restoreFocus: null,
        /* The overlay cancels Escape's keydown, which stops the dialog's own
         * cancel event, so Escape is handled here. Backdrop clicks land on the
         * dialog and are handled by its click listener. */
        onDismiss: function (reason) {
          if (reason === "escape" && isDismissible(el)) closeModal(el, "escape");
        },
      });
      modalState.set(el, s);
    }

    function teardown(el) {
      const s = modalState.get(el);
      if (!s) return;
      if (s.overlay) {
        s.overlay.disconnect();
        s.overlay = null;
      }
    }

    function closeModal(el, reason) {
      if (!el.boolAttr("open")) return;
      const ev = el.emit("lds-close", { reason: reason });
      if (ev.defaultPrevented) return;
      el.removeAttribute("open");
    }

    function watchSlot(el, slotSel, wrapSel) {
      const slot = el.$(slotSel);
      const wrap = el.$(wrapSel);
      if (!slot || !wrap) return;
      const update = () => {
        const empty = slot.assignedNodes({ flatten: true }).every(
          (node) => node.nodeType === Node.TEXT_NODE && !node.textContent.trim()
        );
        wrap.toggleAttribute("data-empty", empty);
      };
      slot.addEventListener("slotchange", update);
      update();
    }
  })();

  /* behaviour: lds-popover */
  (function () {
    /* Behaviour for <lds-popover>.
     *
     * Opens a non-modal panel anchored to a slotted trigger. Uses LDS.overlay for
     * Escape and outside-click dismissal and LDS.position for layout. Unlike
     * lds-menu it does not trap focus: Tab leaves the panel and the page stays
     * usable, which is what property pickers and display options need.
     */

    const popoverState = new WeakMap();

    LDS.behavior("lds-popover", {
      connected(el) {
        const triggerSlot = el.$('slot[name="trigger"]');

        const onTriggerClick = (event) => {
          event.preventDefault();
          if (el.boolAttr("open")) closePopover(el, "toggle");
          else openPopover(el);
        };

        popoverState.set(el, {
          onTriggerClick,
          overlay: null,
          reposition: null,
          trigger: null,
          restore: false,
        });

        triggerSlot.addEventListener("slotchange", () => wireTrigger(el));
        wireTrigger(el);
        syncOpen(el);
      },

      disconnected(el) {
        teardownOpen(el);
        const s = popoverState.get(el);
        if (s && s.trigger) s.trigger.removeEventListener("click", s.onTriggerClick);
        popoverState.delete(el);
      },

      attributeChanged(el, name) {
        if (!el._lds.ready || !popoverState.has(el)) return;
        if (name === "open") syncOpen(el);
        if (name === "placement" && el.boolAttr("open")) positionPanel(el);
        if (name === "label") syncLabel(el);
      },
    });

    function triggerOf(el) {
      const slot = el.$('slot[name="trigger"]');
      if (!slot) return null;
      return slot.assignedElements({ flatten: true })[0] || null;
    }

    function wireTrigger(el) {
      const s = popoverState.get(el);
      if (!s) return;
      if (s.trigger) s.trigger.removeEventListener("click", s.onTriggerClick);
      s.trigger = triggerOf(el);
      if (!s.trigger) return;
      s.trigger.addEventListener("click", s.onTriggerClick);
      syncTriggerA11y(el);
    }

    function syncTriggerA11y(el) {
      const trigger = triggerOf(el);
      if (!trigger) return;
      trigger.setAttribute("aria-haspopup", "dialog");
      trigger.setAttribute("aria-expanded", el.boolAttr("open") ? "true" : "false");
    }

    function syncLabel(el) {
      const panel = el.$(".panel");
      const label = el.getAttribute("label");
      if (label) panel.setAttribute("aria-label", label);
      else panel.removeAttribute("aria-label");
    }

    function openPopover(el) {
      if (el.boolAttr("open")) return;
      if (el.emit("lds-open").defaultPrevented) return;
      el.setAttribute("open", "");
    }

    /* Focus returns to the trigger after Escape or a programmatic close, but not
     * after an outside click — the user has just put focus somewhere else. */
    function closePopover(el, reason) {
      if (!el.boolAttr("open")) return;
      if (el.emit("lds-close", { reason: reason }).defaultPrevented) return;
      const s = popoverState.get(el);
      if (s) s.restore = reason !== "outside";
      el.removeAttribute("open");
    }

    function syncOpen(el) {
      const panel = el.$(".panel");
      const open = el.boolAttr("open");
      panel.hidden = !open;
      syncTriggerA11y(el);
      syncLabel(el);
      if (open) setupOpen(el);
      else teardownOpen(el);
    }

    function setupOpen(el) {
      const s = popoverState.get(el);
      const panel = el.$(".panel");
      if (!s) return;
      teardownOpen(el);

      positionPanel(el);
      s.reposition = () => positionPanel(el);
      window.addEventListener("resize", s.reposition);
      window.addEventListener("scroll", s.reposition, true);

      const trigger = triggerOf(el);
      s.restore = true;
      s.overlay = LDS.overlay.bind(panel, {
        trapFocus: false,
        ignoreOutside: trigger ? [trigger] : [],
        onDismiss: (reason) => closePopover(el, reason),
      });

      requestAnimationFrame(() => {
        if (!el.boolAttr("open")) return;
        const first = firstFocusable(el);
        (first || panel).focus();
      });
    }

    function teardownOpen(el) {
      const s = popoverState.get(el);
      if (!s) return;
      if (s.reposition) {
        window.removeEventListener("resize", s.reposition);
        window.removeEventListener("scroll", s.reposition, true);
        s.reposition = null;
      }
      if (!s.overlay) return;
      s.overlay.disconnect();
      s.overlay = null;
      if (s.restore) focusTrigger(el);
    }

    function positionPanel(el) {
      const trigger = triggerOf(el);
      if (!trigger) return;
      LDS.position(el.$(".panel"), trigger, {
        placement: el.getAttribute("placement") || "bottom-start",
      });
    }

    function focusTrigger(el) {
      const trigger = triggerOf(el);
      if (!trigger) return;
      const inner = trigger.shadowRoot
        ? trigger.shadowRoot.querySelector('button:not([disabled]),a[href],input,[tabindex]:not([tabindex="-1"])')
        : null;
      (inner || trigger).focus();
    }

    /* First focusable slotted element. Custom elements with delegatesFocus or an
     * inner control focus through their host. */
    function firstFocusable(el) {
      const selector =
        'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"]),lds-button:not([disabled]),lds-input,lds-select,lds-textarea,lds-checkbox,lds-switch,lds-radio-group,lds-menu-item,lds-command-item';
      const slot = el.$(".panel slot");
      for (const node of slot.assignedElements({ flatten: true })) {
        if (node.matches(selector)) return focusTarget(node);
        const inner = node.querySelector(selector);
        if (inner) return focusTarget(inner);
      }
      return null;
    }

    function focusTarget(node) {
      if (node.shadowRoot) {
        const inner = node.shadowRoot.querySelector('button:not([disabled]),a[href],input,select,textarea');
        if (inner) return inner;
      }
      return node;
    }
  })();

  /* behaviour: lds-tooltip */
  (function () {
    /* Behaviour for <lds-tooltip>.
     *
     * Shows a hint for the slotted trigger after a hover delay, or at once when the
     * trigger takes keyboard focus. Meets WCAG 1.4.13: the bubble can be hovered
     * without disappearing, and Escape dismisses it without moving focus.
     */

    /* Grace period for the pointer to cross from trigger to bubble, and the window
     * in which the next tooltip skips its delay (moving along a toolbar). */
    const HIDE_GRACE_MS = 100;
    const WARM_MS = 300;

    const tooltipState = new WeakMap();
    let lastHiddenAt = 0;
    let current = null; /* the tooltip on screen, if any; only one at a time */

    LDS.behavior("lds-tooltip", {
      connected(el) {
        const s = { showTimer: 0, hideTimer: 0, reposition: null, onKeydown: null };
        tooltipState.set(el, s);

        el.addEventListener("pointerenter", (event) => {
          if (event.pointerType === "touch") return;
          clearTimeout(s.hideTimer);
          if (isShown(el)) return;
          /* Warm when another hint is still up (its grace period has not run out)
           * or has just gone. */
          const warm = current !== null || Date.now() - lastHiddenAt < WARM_MS;
          s.showTimer = setTimeout(() => show(el), warm ? 0 : delayOf(el));
        });

        el.addEventListener("pointerleave", () => {
          clearTimeout(s.showTimer);
          s.hideTimer = setTimeout(() => hide(el), HIDE_GRACE_MS);
        });

        el.addEventListener("focusin", () => {
          if (focusIsVisible()) show(el);
        });

        el.addEventListener("focusout", () => hide(el));

        /* A click is an action, not a request for the hint. */
        el.addEventListener("pointerdown", () => {
          clearTimeout(s.showTimer);
          hide(el);
        });

        syncText(el);
      },

      disconnected(el) {
        hide(el);
        tooltipState.delete(el);
      },

      attributeChanged(el, name) {
        if (!el._lds.ready) return;
        if (name === "content") syncText(el);
        if (name === "disabled" && el.boolAttr("disabled")) hide(el);
        if (name === "placement" && isShown(el)) position(el);
      },
    });

    function delayOf(el) {
      const n = parseInt(el.getAttribute("delay"), 10);
      return Number.isNaN(n) ? 500 : Math.max(0, n);
    }

    function syncText(el) {
      const text = el.$(".text");
      if (text) text.textContent = el.getAttribute("content") || "";
    }

    function hasContent(el) {
      if (el.getAttribute("content")) return true;
      return !!el.querySelector(':scope > [slot="content"]');
    }

    function triggerOf(el) {
      const slot = el.$(".anchor slot");
      return (slot && slot.assignedElements({ flatten: true })[0]) || el;
    }

    function isShown(el) {
      const bubble = el.$(".bubble");
      return !!bubble && !bubble.hidden;
    }

    function focusIsVisible() {
      let active = document.activeElement;
      while (active && active.shadowRoot && active.shadowRoot.activeElement) {
        active = active.shadowRoot.activeElement;
      }
      try {
        return !!active && active.matches(":focus-visible");
      } catch (_) {
        return true;
      }
    }

    function show(el) {
      const s = tooltipState.get(el);
      if (!s || isShown(el)) return;
      if (el.boolAttr("disabled") || !hasContent(el)) return;

      if (current && current !== el) hide(current);
      current = el;
      el.$(".bubble").hidden = false;
      position(el);

      s.reposition = () => position(el);
      window.addEventListener("resize", s.reposition);
      window.addEventListener("scroll", s.reposition, true);

      s.onKeydown = (event) => {
        if (event.key === "Escape") hide(el);
      };
      document.addEventListener("keydown", s.onKeydown, true);
    }

    function hide(el) {
      const s = tooltipState.get(el);
      if (!s) return;
      clearTimeout(s.showTimer);
      clearTimeout(s.hideTimer);
      if (!isShown(el)) return;

      el.$(".bubble").hidden = true;
      lastHiddenAt = Date.now();
      if (current === el) current = null;

      window.removeEventListener("resize", s.reposition);
      window.removeEventListener("scroll", s.reposition, true);
      document.removeEventListener("keydown", s.onKeydown, true);
      s.reposition = null;
      s.onKeydown = null;
    }

    function position(el) {
      const raw = LDS.token("space.1", el);
      const offset = raw ? parseFloat(raw) * parseFloat(getComputedStyle(document.documentElement).fontSize) : 4;
      LDS.position(el.$(".bubble"), triggerOf(el), {
        placement: el.getAttribute("placement") || "top",
        offset: Number.isNaN(offset) ? 4 : offset,
      });
    }
  })();

  /* component: lds-avatar (draft) */
  LDS.define({
    tag: "lds-avatar",
    className: "LdsAvatar",
    formAssociated: false,
    observed: ["src", "name", "initials", "size", "shape", "label"],
    defaults: {"size": "md", "shape": "circle"},
    template: `<img class="img" part="image" alt="" hidden>
<span class="initials" part="initials" aria-hidden="true"></span>
<span class="fallback" part="fallback" aria-hidden="true" hidden>
  <lds-icon name="user" size="sm"></lds-icon>
</span>`,
    styles: `/* Identity mark — image, initials, or decorative user icon. */

:host {
  --lds-avatar-box: var(--lds-avatar-size-md);
  --lds-avatar-font: var(--lds-avatar-font-size-md);
  --lds-avatar-radius: var(--lds-radius-full);
  --lds-avatar-bg: var(--lds-color-bg-muted);
  --lds-avatar-fg: var(--lds-color-fg-muted);
  --lds-avatar-border: var(--lds-color-border-default);

  display: inline-flex;
  vertical-align: middle;
  box-sizing: border-box;
  width: var(--lds-avatar-box);
  height: var(--lds-avatar-box);
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: var(--lds-avatar-radius);
  border: var(--lds-control-border-width-default) solid var(--lds-avatar-border);
  background: var(--lds-avatar-bg);
  color: var(--lds-avatar-fg);
  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-avatar-font);
  font-weight: var(--lds-font-weight-semibold);
  line-height: 1;
  user-select: none;
}

:host([hidden]) {
  display: none;
}

:host([size="sm"]) {
  --lds-avatar-box: var(--lds-avatar-size-sm);
  --lds-avatar-font: var(--lds-avatar-font-size-sm);
}

:host([size="lg"]) {
  --lds-avatar-box: var(--lds-avatar-size-lg);
  --lds-avatar-font: var(--lds-avatar-font-size-lg);
}

:host([shape="square"]) {
  --lds-avatar-radius: var(--lds-control-radius-md);
}

.img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.img[hidden],
.initials[hidden],
.fallback[hidden] {
  display: none;
}

.initials,
.fallback {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  text-transform: uppercase;
}

.fallback lds-icon {
  color: inherit;
}`,
  });

  /* component: lds-avatar-group (draft) */
  LDS.define({
    tag: "lds-avatar-group",
    className: "LdsAvatarGroup",
    formAssociated: false,
    observed: ["max", "size"],
    defaults: {"size": "md"},
    template: `<div class="stack" part="stack">
  <slot></slot>
  <span class="overflow" part="overflow" hidden aria-hidden="true"></span>
</div>`,
    styles: `/* Overlapping avatar stack with an optional +N overflow chip. */

:host {
  --lds-avatar-box: var(--lds-avatar-size-md);
  --lds-avatar-group-overlap: var(--lds-space-2);
  --lds-avatar-group-overflow-bg: var(--lds-color-bg-subtle);
  --lds-avatar-group-overflow-fg: var(--lds-color-fg-muted);
  --lds-avatar-group-overflow-border: var(--lds-color-border-default);

  display: inline-flex;
  vertical-align: middle;
}

:host([hidden]) {
  display: none;
}

:host([size="sm"]) {
  --lds-avatar-box: var(--lds-avatar-size-sm);
}

:host([size="lg"]) {
  --lds-avatar-box: var(--lds-avatar-size-lg);
}

.stack {
  display: inline-flex;
  flex-direction: row;
  align-items: center;
}

::slotted(lds-avatar) {
  position: relative;
  box-shadow: 0 0 0 var(--lds-space-px) var(--lds-color-bg-canvas);
}

::slotted(lds-avatar:not(:first-child)) {
  margin-inline-start: calc(var(--lds-avatar-group-overlap) * -1);
}

.overflow {
  display: inline-flex;
  box-sizing: border-box;
  align-items: center;
  justify-content: center;
  width: var(--lds-avatar-box);
  height: var(--lds-avatar-box);
  margin-inline-start: calc(var(--lds-avatar-group-overlap) * -1);
  border-radius: var(--lds-radius-full);
  border: var(--lds-control-border-width-default) solid var(--lds-avatar-group-overflow-border);
  background: var(--lds-avatar-group-overflow-bg);
  color: var(--lds-avatar-group-overflow-fg);
  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-avatar-font-size-sm);
  font-weight: var(--lds-font-weight-semibold);
  line-height: 1;
  box-shadow: 0 0 0 var(--lds-space-px) var(--lds-color-bg-canvas);
}

.overflow[hidden] {
  display: none;
}`,
  });

  /* component: lds-badge (draft) */
  LDS.define({
    tag: "lds-badge",
    className: "LdsBadge",
    formAssociated: false,
    observed: ["intent", "variant", "size"],
    defaults: {"intent": "neutral", "variant": "subtle", "size": "sm"},
    template: `<span class="badge" part="badge"><slot></slot></span>`,
    styles: `/* Short status label. Intents assign custom properties; variants remix them. */

:host {
  --lds-badge-bg: var(--lds-color-bg-muted);
  --lds-badge-fg: var(--lds-color-fg-muted);
  --lds-badge-border: var(--lds-color-border-default);
  --lds-badge-padding-y: var(--lds-space-1);
  --lds-badge-padding-x: var(--lds-space-2);
  --lds-badge-font-size: var(--lds-font-size-2xs);
  --lds-badge-radius: var(--lds-radius-full);

  display: inline-flex;
  vertical-align: middle;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

:host([size="md"]) {
  --lds-badge-padding-y: var(--lds-space-1);
  --lds-badge-padding-x: var(--lds-space-3);
  --lds-badge-font-size: var(--lds-font-size-xs);
}

:host([intent="info"]) {
  --lds-badge-bg: var(--lds-color-accent-subtle);
  --lds-badge-fg: var(--lds-color-accent-fg);
  --lds-badge-border: var(--lds-color-accent-border);
}

:host([intent="success"]) {
  --lds-badge-bg: var(--lds-color-success-subtle);
  --lds-badge-fg: var(--lds-color-success-fg);
  --lds-badge-border: var(--lds-color-success-border);
}

:host([intent="warning"]) {
  --lds-badge-bg: var(--lds-color-warning-subtle);
  --lds-badge-fg: var(--lds-color-warning-fg);
  --lds-badge-border: var(--lds-color-warning-border);
}

:host([intent="danger"]) {
  --lds-badge-bg: var(--lds-color-danger-subtle);
  --lds-badge-fg: var(--lds-color-danger-fg);
  --lds-badge-border: var(--lds-color-danger-border);
}

:host([variant="solid"]) {
  --lds-badge-bg: var(--lds-color-fg-muted);
  --lds-badge-fg: var(--lds-color-fg-on-accent);
  --lds-badge-border: transparent;
}

:host([variant="solid"][intent="info"]) {
  --lds-badge-bg: var(--lds-color-accent-default);
}

:host([variant="solid"][intent="success"]) {
  --lds-badge-bg: var(--lds-color-success-default);
}

:host([variant="solid"][intent="warning"]) {
  --lds-badge-bg: var(--lds-color-warning-default);
  --lds-badge-fg: var(--lds-color-fg-on-warning);
}

:host([variant="solid"][intent="danger"]) {
  --lds-badge-bg: var(--lds-color-danger-default);
}

:host([variant="outline"]) {
  --lds-badge-bg: transparent;
}

.badge {
  display: inline-flex;
  box-sizing: border-box;
  align-items: center;
  max-width: 100%;
  padding: var(--lds-badge-padding-y) var(--lds-badge-padding-x);
  overflow: hidden;
  border: var(--lds-control-border-width-default) solid var(--lds-badge-border);
  border-radius: var(--lds-badge-radius);
  background: var(--lds-badge-bg);
  color: var(--lds-badge-fg);
  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-badge-font-size);
  font-weight: var(--lds-font-weight-semibold);
  line-height: var(--lds-font-leading-none);
  letter-spacing: var(--lds-font-tracking-caps);
  text-transform: uppercase;
  text-overflow: ellipsis;
  white-space: nowrap;
}`,
  });

  /* component: lds-button (draft) */
  LDS.define({
    tag: "lds-button",
    className: "LdsButton",
    formAssociated: false,
    observed: ["variant", "size", "type", "disabled", "loading", "full-width", "pill", "label"],
    defaults: {"variant": "primary", "size": "md", "type": "button"},
    template: `<button class="btn" part="button" type="button">
  <span class="btn__spinner" part="spinner" aria-hidden="true"></span>
  <slot name="start"></slot>
  <span class="btn__label" part="label"><slot></slot></span>
  <slot name="end"></slot>
</button>`,
    styles: `/* Component styles are adopted into the shadow root, so selectors here are
 * scoped and can stay short. Only :host and ::slotted() reach outward, and
 * only --lds-* custom properties reach inward.
 *
 * Variants are expressed as custom-property assignments on :host, never as
 * duplicated rule blocks — the .btn rule below is the single place geometry
 * and colour are actually applied. */

:host {
  --lds-button-height: var(--lds-control-height-md);
  --lds-button-padding-x: var(--lds-control-padding-x-md);
  --lds-button-gap: var(--lds-control-gap-md);
  --lds-button-font-size: var(--lds-control-font-size-md);
  --lds-button-radius: var(--lds-control-radius-md);
  --lds-button-bg: var(--lds-color-accent-default);
  --lds-button-bg-hover: var(--lds-color-accent-hover);
  --lds-button-bg-active: var(--lds-color-accent-active);
  --lds-button-fg: var(--lds-color-fg-on-accent);
  --lds-button-border: transparent;
  --lds-button-shadow: none;

  display: inline-flex;
  vertical-align: middle;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

/* --- Sizes ------------------------------------------------------------- */

:host([size="sm"]) {
  --lds-button-height: var(--lds-control-height-sm);
  --lds-button-padding-x: var(--lds-control-padding-x-sm);
  --lds-button-gap: var(--lds-control-gap-sm);
  --lds-button-font-size: var(--lds-control-font-size-sm);
  --lds-button-radius: var(--lds-control-radius-sm);
}

:host([size="lg"]) {
  --lds-button-height: var(--lds-control-height-lg);
  --lds-button-padding-x: var(--lds-control-padding-x-lg);
  --lds-button-gap: var(--lds-control-gap-lg);
  --lds-button-font-size: var(--lds-control-font-size-lg);
  --lds-button-radius: var(--lds-control-radius-lg);
}

/* --- Variants ---------------------------------------------------------- */

:host([variant="secondary"]) {
  --lds-button-bg: var(--lds-color-bg-canvas);
  --lds-button-bg-hover: var(--lds-color-bg-subtle);
  --lds-button-bg-active: var(--lds-color-bg-muted);
  --lds-button-fg: var(--lds-color-fg-default);
  --lds-button-border: var(--lds-color-border-default);
  --lds-button-shadow: var(--lds-shadow-sm);
}

:host([variant="ghost"]) {
  --lds-button-bg: transparent;
  --lds-button-bg-hover: var(--lds-color-bg-subtle);
  --lds-button-bg-active: var(--lds-color-bg-muted);
  --lds-button-fg: var(--lds-color-fg-muted);
  --lds-button-border: transparent;
}

:host([variant="danger"]) {
  --lds-button-bg: var(--lds-color-danger-default);
  --lds-button-bg-hover: var(--lds-color-danger-hover);
  --lds-button-bg-active: var(--lds-color-danger-active);
  --lds-button-fg: var(--lds-color-fg-on-accent);
}

:host([variant="link"]) {
  --lds-button-bg: transparent;
  --lds-button-bg-hover: transparent;
  --lds-button-bg-active: transparent;
  --lds-button-fg: var(--lds-color-fg-link);
  --lds-button-padding-x: 0;
  --lds-button-height: auto;
}

/* --- Modifiers --------------------------------------------------------- */

:host([pill]) {
  --lds-button-radius: var(--lds-control-radius-pill);
}

:host([full-width]) {
  display: flex;
  width: 100%;
}

:host([full-width]) .btn {
  width: 100%;
}

/* --- The control ------------------------------------------------------- */

.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: var(--lds-button-gap);

  min-width: 0;
  height: var(--lds-button-height);
  padding-inline: var(--lds-button-padding-x);

  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-button-font-size);
  font-weight: var(--lds-font-weight-semibold);
  line-height: 1;
  white-space: nowrap;

  color: var(--lds-button-fg);
  background: var(--lds-button-bg);
  border: var(--lds-control-border-width-default) solid var(--lds-button-border);
  border-radius: var(--lds-button-radius);
  box-shadow: var(--lds-button-shadow);

  cursor: pointer;
  appearance: none;
  transition:
    background var(--lds-transition-control),
    border-color var(--lds-transition-control),
    box-shadow var(--lds-transition-control),
    transform var(--lds-transition-control);
}

.btn:hover:not(:disabled) {
  background: var(--lds-button-bg-hover);
}

:host([variant="link"]) .btn:hover:not(:disabled) {
  text-decoration: underline;
  text-underline-offset: 0.2em;
}

.btn:active:not(:disabled) {
  background: var(--lds-button-bg-active);
  transform: translateY(1px);
}

.btn:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.btn:disabled {
  cursor: not-allowed;
  opacity: 0.55;
  box-shadow: none;
}

.btn__label {
  min-width: 0;
  /* line-height: 1 (from .btn) + overflow:hidden clips descenders (g, y, p).
   * A slightly taller line box keeps ellipsis clipping horizontal only. */
  line-height: var(--lds-font-leading-snug);
  overflow: hidden;
  text-overflow: ellipsis;
}

/* CSS cannot see whether a slot has assigned nodes, so the behaviour script
 * sets data-empty on slotchange. Without this an icon-only button carries the
 * flex gap of a zero-width label. */
.btn__label[data-empty] {
  display: none;
}

/* --- Loading ----------------------------------------------------------- */

.btn__spinner {
  display: none;
  flex: none;
  width: 1em;
  height: 1em;
  border: 2px solid currentColor;
  border-top-color: transparent;
  border-radius: var(--lds-radius-full);
  animation: lds-button-spin var(--lds-duration-slower) linear infinite;
}

:host([loading]) .btn__spinner {
  display: block;
}

/* Adornments are redundant next to a spinner and would make the control
 * jump width mid-request. */
:host([loading]) ::slotted([slot="start"]) {
  display: none;
}

:host([loading]) .btn {
  cursor: progress;
}

@keyframes lds-button-spin {
  to {
    transform: rotate(360deg);
  }
}

/* --- Slotted adornments ------------------------------------------------ */

::slotted([slot="start"]),
::slotted([slot="end"]) {
  display: inline-flex;
  flex: none;
  align-items: center;
  line-height: 1;
}`,
  });

  /* component: lds-divider (draft) */
  LDS.define({
    tag: "lds-divider",
    className: "LdsDivider",
    formAssociated: false,
    observed: ["orientation"],
    defaults: {"orientation": "horizontal"},
    template: `<div class="root" part="root">
  <span class="line line--start" part="line"></span>
  <span class="label" part="label"><slot></slot></span>
  <span class="line line--end" part="line"></span>
</div>`,
    styles: `/* Visual separator. Optional label sits between two line segments. */

:host {
  --lds-divider-color: var(--lds-color-border-default);
  --lds-divider-thickness: var(--lds-control-border-width-default);
  --lds-divider-gap: var(--lds-space-3);
  --lds-divider-label-fg: var(--lds-color-fg-muted);

  display: block;
  width: 100%;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

:host([orientation="vertical"]) {
  display: inline-flex;
  flex-direction: column;
  align-self: stretch;
  width: auto;
  max-width: none;
  min-height: 1em;
  vertical-align: middle;
}

.root {
  display: flex;
  align-items: center;
  gap: var(--lds-divider-gap);
  width: 100%;
}

:host([orientation="vertical"]) .root {
  flex: 1 1 auto;
  flex-direction: column;
  align-self: stretch;
  align-items: center;
  width: auto;
  min-height: 0;
}

.line {
  flex: 1 1 auto;
  min-width: 0;
  min-height: 0;
  background: var(--lds-divider-color);
}

:host(:not([orientation="vertical"])) .line {
  height: var(--lds-divider-thickness);
}

:host([orientation="vertical"]) .line {
  flex: 1 1 auto;
  align-self: center;
  width: var(--lds-divider-thickness);
  min-height: 1em;
}

.label {
  flex: none;
  color: var(--lds-divider-label-fg);
  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-font-size-xs);
  font-weight: var(--lds-font-weight-medium);
  line-height: var(--lds-font-leading-snug);
  white-space: nowrap;
}

.label[data-empty] {
  display: none;
}

.label[data-empty] + .line--end {
  display: none;
}`,
  });

  /* component: lds-icon (draft) */
  LDS.define({
    tag: "lds-icon",
    className: "LdsIcon",
    formAssociated: false,
    observed: ["name", "size", "label"],
    defaults: {"size": "md"},
    template: `<svg class="icon" part="svg" viewBox="0 0 24 24" focusable="false"></svg>`,
    styles: `:host {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  width: var(--lds-icon-size-md);
  height: var(--lds-icon-size-md);
  color: currentColor;
  vertical-align: middle;
  line-height: 0;
}

:host([hidden]) {
  display: none;
}

:host([size="sm"]) {
  width: var(--lds-icon-size-sm);
  height: var(--lds-icon-size-sm);
}

:host([size="md"]) {
  width: var(--lds-icon-size-md);
  height: var(--lds-icon-size-md);
}

:host([size="lg"]) {
  width: var(--lds-icon-size-lg);
  height: var(--lds-icon-size-lg);
}

.icon {
  display: block;
  width: 100%;
  height: 100%;
  fill: none;
  stroke: currentColor;
}`,
  });

  /* component: lds-kbd (draft) */
  LDS.define({
    tag: "lds-kbd",
    className: "LdsKbd",
    formAssociated: false,
    observed: ["size"],
    defaults: {"size": "sm"},
    template: `<kbd class="kbd" part="kbd"><slot></slot></kbd>`,
    styles: `/* One keycap. A sequence (G then I) is two lds-kbd elements; a chord (⌘K)
 * is one. */

:host {
  --lds-kbd-bg: var(--lds-color-bg-subtle);
  --lds-kbd-fg: var(--lds-color-fg-muted);
  --lds-kbd-border: var(--lds-color-border-default);
  --lds-kbd-size: var(--lds-space-5);
  --lds-kbd-font-size: var(--lds-font-size-2xs);

  display: inline-flex;
  vertical-align: middle;
}

:host([hidden]) {
  display: none;
}

:host([size="md"]) {
  --lds-kbd-size: var(--lds-space-6);
  --lds-kbd-font-size: var(--lds-font-size-xs);
}

.kbd {
  display: inline-flex;
  box-sizing: border-box;
  align-items: center;
  justify-content: center;
  min-width: var(--lds-kbd-size);
  height: var(--lds-kbd-size);
  padding-inline: var(--lds-space-1);
  border: var(--lds-control-border-width-default) solid var(--lds-kbd-border);
  border-radius: var(--lds-radius-md);
  background: var(--lds-kbd-bg);
  color: var(--lds-kbd-fg);
  box-shadow: inset 0 calc(var(--lds-control-border-width-default) * -1) 0 var(--lds-kbd-border);
  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-kbd-font-size);
  font-weight: var(--lds-font-weight-medium);
  line-height: 1;
  white-space: nowrap;
}`,
  });

  /* component: lds-spinner (draft) */
  LDS.define({
    tag: "lds-spinner",
    className: "LdsSpinner",
    formAssociated: false,
    observed: ["size", "label"],
    defaults: {"size": "md"},
    template: `<span class="spinner" part="spinner"></span>`,
    styles: `/* Indeterminate progress mark. Put aria-busy on the busy region, not here. */

:host {
  --lds-spinner-size: var(--lds-icon-size-md);
  --lds-spinner-color: currentColor;
  --lds-spinner-track: transparent;

  display: inline-flex;
  vertical-align: middle;
  color: var(--lds-color-fg-muted);
}

:host([hidden]) {
  display: none;
}

:host([size="sm"]) {
  --lds-spinner-size: var(--lds-icon-size-sm);
}

:host([size="lg"]) {
  --lds-spinner-size: var(--lds-icon-size-lg);
}

.spinner {
  box-sizing: border-box;
  width: var(--lds-spinner-size);
  height: var(--lds-spinner-size);
  border: var(--lds-control-border-width-default) solid var(--lds-spinner-color);
  border-block-start-color: var(--lds-spinner-track);
  border-radius: var(--lds-radius-full);
  animation: lds-spinner-spin var(--lds-duration-slower) linear infinite;
}

@keyframes lds-spinner-spin {
  to {
    transform: rotate(360deg);
  }
}`,
  });

  /* component: lds-tag (draft) */
  LDS.define({
    tag: "lds-tag",
    className: "LdsTag",
    formAssociated: false,
    observed: ["intent", "variant", "size", "dismissible", "disabled"],
    defaults: {"intent": "neutral", "variant": "subtle", "size": "sm"},
    template: `<span class="tag" part="tag">
  <span class="tag__start" part="start"><slot name="start"></slot></span>
  <span class="tag__label" part="label"><slot></slot></span>
  <button class="tag__close" part="close" type="button" aria-label="Remove" hidden>
    <lds-icon name="x" size="sm"></lds-icon>
  </button>
</span>`,
    styles: `/* Removable chip. Shares intent colouring with lds-badge. */

:host {
  --lds-tag-bg: var(--lds-color-bg-muted);
  --lds-tag-fg: var(--lds-color-fg-muted);
  --lds-tag-border: var(--lds-color-border-default);
  --lds-tag-padding-y: var(--lds-space-1);
  --lds-tag-padding-x: var(--lds-space-2);
  --lds-tag-font-size: var(--lds-font-size-xs);
  --lds-tag-radius: var(--lds-radius-full);
  --lds-tag-gap: var(--lds-space-1);

  display: inline-flex;
  vertical-align: middle;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

:host([size="md"]) {
  --lds-tag-padding-y: var(--lds-space-1);
  --lds-tag-padding-x: var(--lds-space-3);
  --lds-tag-font-size: var(--lds-font-size-sm);
}

:host([intent="info"]) {
  --lds-tag-bg: var(--lds-color-accent-subtle);
  --lds-tag-fg: var(--lds-color-accent-fg);
  --lds-tag-border: var(--lds-color-accent-border);
}

:host([intent="success"]) {
  --lds-tag-bg: var(--lds-color-success-subtle);
  --lds-tag-fg: var(--lds-color-success-fg);
  --lds-tag-border: var(--lds-color-success-border);
}

:host([intent="warning"]) {
  --lds-tag-bg: var(--lds-color-warning-subtle);
  --lds-tag-fg: var(--lds-color-warning-fg);
  --lds-tag-border: var(--lds-color-warning-border);
}

:host([intent="danger"]) {
  --lds-tag-bg: var(--lds-color-danger-subtle);
  --lds-tag-fg: var(--lds-color-danger-fg);
  --lds-tag-border: var(--lds-color-danger-border);
}

:host([variant="solid"]) {
  --lds-tag-bg: var(--lds-color-fg-muted);
  --lds-tag-fg: var(--lds-color-fg-on-accent);
  --lds-tag-border: transparent;
}

:host([variant="solid"][intent="info"]) {
  --lds-tag-bg: var(--lds-color-accent-default);
}

:host([variant="solid"][intent="success"]) {
  --lds-tag-bg: var(--lds-color-success-default);
}

:host([variant="solid"][intent="warning"]) {
  --lds-tag-bg: var(--lds-color-warning-default);
  --lds-tag-fg: var(--lds-color-fg-on-warning);
}

:host([variant="solid"][intent="danger"]) {
  --lds-tag-bg: var(--lds-color-danger-default);
}

:host([variant="outline"]) {
  --lds-tag-bg: transparent;
}

:host([disabled]) {
  opacity: 0.6;
}

.tag {
  display: inline-flex;
  box-sizing: border-box;
  align-items: center;
  gap: var(--lds-tag-gap);
  max-width: 100%;
  padding: var(--lds-tag-padding-y) var(--lds-tag-padding-x);
  border: var(--lds-control-border-width-default) solid var(--lds-tag-border);
  border-radius: var(--lds-tag-radius);
  background: var(--lds-tag-bg);
  color: var(--lds-tag-fg);
  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-tag-font-size);
  font-weight: var(--lds-font-weight-medium);
  line-height: var(--lds-font-leading-none);
}

.tag__start,
.tag__label {
  display: inline-flex;
  align-items: center;
  min-width: 0;
}

.tag__start[data-empty] {
  display: none;
}

.tag__label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tag__close {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  margin: 0;
  margin-inline-end: calc(var(--lds-space-1) * -1);
  padding: 0;
  border: 0;
  border-radius: var(--lds-radius-full);
  background: transparent;
  color: inherit;
  cursor: pointer;
}

.tag__close[hidden] {
  display: none;
}

.tag__close:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.tag__close:disabled {
  cursor: not-allowed;
}`,
  });

  /* component: lds-checkbox (draft) */
  LDS.define({
    tag: "lds-checkbox",
    className: "LdsCheckbox",
    formAssociated: true,
    observed: ["name", "value", "checked", "indeterminate", "disabled", "required", "invalid"],
    defaults: {"value": "on"},
    template: `<label class="root" part="root">
  <span class="control" part="control">
    <input class="input" part="input" type="checkbox" />
    <span class="box" part="box" aria-hidden="true">
      <lds-icon class="icon icon--check" name="check" size="sm"></lds-icon>
      <lds-icon class="icon icon--indeterminate" name="minus" size="sm"></lds-icon>
    </span>
  </span>
  <span class="label" part="label"><slot></slot></span>
</label>`,
    styles: `:host {
  --lds-checkbox-size: var(--lds-icon-size-md);
  --lds-checkbox-radius: var(--lds-control-radius-sm);
  --lds-checkbox-border: var(--lds-color-border-strong);
  --lds-checkbox-bg: var(--lds-color-bg-canvas);
  --lds-checkbox-fg: var(--lds-color-fg-on-accent);

  display: inline-flex;
  max-width: 100%;
  vertical-align: middle;
}

:host([hidden]) {
  display: none;
}

:host([checked]),
:host([indeterminate]) {
  --lds-checkbox-border: var(--lds-color-accent-default);
  --lds-checkbox-bg: var(--lds-color-accent-default);
}

:host([invalid]),
:host([aria-invalid="true"]) {
  --lds-checkbox-border: var(--lds-color-danger-default);
}

.root {
  display: inline-flex;
  align-items: flex-start;
  gap: var(--lds-space-2);
  cursor: pointer;
  color: var(--lds-color-fg-default);
  font-size: var(--lds-font-size-sm);
  line-height: var(--lds-font-leading-snug);
}

.control {
  position: relative;
  display: inline-flex;
  flex: none;
  margin-block-start: 0.125em;
}

.input {
  position: absolute;
  inset: 0;
  margin: 0;
  opacity: 0;
  cursor: inherit;
}

.box {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
  width: var(--lds-checkbox-size);
  height: var(--lds-checkbox-size);
  border: var(--lds-control-border-width-default) solid var(--lds-checkbox-border);
  border-radius: var(--lds-checkbox-radius);
  background: var(--lds-checkbox-bg);
  color: var(--lds-checkbox-fg);
}

.input:focus-visible + .box {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.icon {
  display: none;
}

:host([checked]:not([indeterminate])) .icon--check,
:host([indeterminate]) .icon--indeterminate {
  display: inline-flex;
}

.label[hidden] {
  display: none;
}

:host([disabled]) {
  opacity: 0.55;
  cursor: not-allowed;
}

:host([disabled]) .root {
  cursor: not-allowed;
}`,
  });

  /* component: lds-combobox (draft) */
  LDS.define({
    tag: "lds-combobox",
    className: "LdsCombobox",
    formAssociated: true,
    observed: ["name", "value", "multiple", "creatable", "filter", "label", "placeholder", "empty", "size", "disabled", "required", "invalid"],
    defaults: {"filter": "auto", "size": "md"},
    template: `<div class="control" part="control">
  <span class="lead" part="lead" hidden></span>
  <span class="chips" part="chips"></span>
  <input
    class="input"
    part="input"
    type="text"
    role="combobox"
    aria-autocomplete="list"
    aria-expanded="false"
    autocomplete="off"
    spellcheck="false"
  />
  <span class="chevron" part="chevron" aria-hidden="true">
    <lds-icon name="chevron-down" size="sm"></lds-icon>
  </span>
</div>
<div class="panel" part="panel" hidden>
  <div class="list" part="list" role="listbox">
    <slot></slot>
    <div class="create" part="create" role="option" aria-selected="false" hidden>
      <lds-icon name="plus" size="sm"></lds-icon>
      <span class="create__label"></span>
    </div>
  </div>
  <div class="empty" part="empty" hidden></div>
</div>`,
    styles: `/* Filterable single or multiple choice. The control matches lds-input; the
 * list floats in a panel positioned with LDS.position. */

:host {
  --lds-combobox-height: var(--lds-control-height-md);
  --lds-combobox-padding-x: var(--lds-control-padding-x-md);
  --lds-combobox-radius: var(--lds-control-radius-md);
  --lds-combobox-font-size: var(--lds-control-font-size-md);
  --lds-combobox-bg: var(--lds-color-bg-canvas);
  --lds-combobox-fg: var(--lds-color-fg-default);
  --lds-combobox-border: var(--lds-color-border-default);
  --lds-combobox-max-height: 18rem;

  display: block;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

:host([size="sm"]) {
  --lds-combobox-height: var(--lds-control-height-sm);
  --lds-combobox-padding-x: var(--lds-control-padding-x-sm);
  --lds-combobox-radius: var(--lds-control-radius-sm);
  --lds-combobox-font-size: var(--lds-control-font-size-sm);
}

:host([size="lg"]) {
  --lds-combobox-height: var(--lds-control-height-lg);
  --lds-combobox-padding-x: var(--lds-control-padding-x-lg);
  --lds-combobox-radius: var(--lds-control-radius-lg);
  --lds-combobox-font-size: var(--lds-control-font-size-lg);
}

:host([invalid]),
:host([aria-invalid="true"]) {
  --lds-combobox-border: var(--lds-color-danger-border);
}

.control {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--lds-space-1);
  box-sizing: border-box;
  min-height: var(--lds-combobox-height);
  padding-block: var(--lds-space-1);
  padding-inline: var(--lds-combobox-padding-x) var(--lds-space-2);
  border: var(--lds-control-border-width-default) solid var(--lds-combobox-border);
  border-radius: var(--lds-combobox-radius);
  background: var(--lds-combobox-bg);
  color: var(--lds-combobox-fg);
  box-shadow: var(--lds-shadow-sm);
  cursor: text;
}

.control:focus-within {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.lead {
  display: inline-flex;
  flex: none;
  align-items: center;
}

.lead[hidden] {
  display: none;
}

.chips {
  display: contents;
}

.input {
  flex: 1 1 4rem;
  min-width: 4rem;
  height: calc(var(--lds-combobox-height) - var(--lds-space-2) - 2 * var(--lds-control-border-width-default));
  margin: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: var(--lds-combobox-font-size);
  outline: none;
}

.input::placeholder {
  color: var(--lds-color-fg-subtle);
}

.chevron {
  display: inline-flex;
  flex: none;
  margin-inline-start: auto;
  color: var(--lds-color-fg-muted);
  pointer-events: none;
}

:host([disabled]) .control {
  background: var(--lds-color-bg-subtle);
  color: var(--lds-color-fg-disabled);
  cursor: not-allowed;
}

.panel {
  box-sizing: border-box;
  position: fixed;
  z-index: var(--lds-z-popover);
  max-height: min(var(--lds-combobox-max-height), calc(100vh - var(--lds-space-4)));
  overflow: auto;
  padding: var(--lds-space-1);
  border: var(--lds-control-border-width-default) solid var(--lds-color-border-default);
  border-radius: var(--lds-overlay-radius);
  background: var(--lds-color-bg-raised);
  color: var(--lds-color-fg-default);
  box-shadow: var(--lds-overlay-shadow);
}

.panel[hidden] {
  display: none;
}

.list {
  display: flex;
  flex-direction: column;
  gap: var(--lds-space-px);
}

.create {
  display: flex;
  align-items: center;
  gap: var(--lds-control-gap-sm);
  min-height: var(--lds-control-height-sm);
  padding: var(--lds-space-1) var(--lds-control-padding-x-sm);
  border-radius: var(--lds-control-radius-sm);
  color: var(--lds-color-accent-fg);
  font-size: var(--lds-font-size-sm);
  cursor: pointer;
  overflow-wrap: anywhere;
}

.create[hidden] {
  display: none;
}

.create[data-active] {
  background: var(--lds-color-bg-subtle);
}

.empty {
  padding: var(--lds-space-2) var(--lds-control-padding-x-sm);
  color: var(--lds-color-fg-muted);
  font-size: var(--lds-font-size-sm);
}

.empty[hidden] {
  display: none;
}`,
  });

  /* component: lds-date-picker (draft) */
  LDS.define({
    tag: "lds-date-picker",
    className: "LdsDatePicker",
    formAssociated: true,
    observed: ["type", "name", "value", "min", "max", "step", "size", "disabled", "readonly", "required", "invalid"],
    defaults: {"type": "date", "size": "md"},
    template: `<div class="wrap" part="wrap">
  <span class="prefix" part="prefix"><slot name="prefix"></slot></span>
  <input class="input" part="input" type="date" />
  <span class="suffix" part="suffix"><slot name="suffix"></slot></span>
</div>`,
    styles: `/* Native date/time control styled to match lds-input. */

:host {
  --lds-date-picker-height: var(--lds-control-height-md);
  --lds-date-picker-padding-x: var(--lds-control-padding-x-md);
  --lds-date-picker-radius: var(--lds-control-radius-md);
  --lds-date-picker-font-size: var(--lds-control-font-size-md);
  --lds-date-picker-bg: var(--lds-color-bg-canvas);
  --lds-date-picker-fg: var(--lds-color-fg-default);
  --lds-date-picker-border: var(--lds-color-border-default);

  display: block;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

:host([size="sm"]) {
  --lds-date-picker-height: var(--lds-control-height-sm);
  --lds-date-picker-padding-x: var(--lds-control-padding-x-sm);
  --lds-date-picker-radius: var(--lds-control-radius-sm);
  --lds-date-picker-font-size: var(--lds-control-font-size-sm);
}

:host([size="lg"]) {
  --lds-date-picker-height: var(--lds-control-height-lg);
  --lds-date-picker-padding-x: var(--lds-control-padding-x-lg);
  --lds-date-picker-radius: var(--lds-control-radius-lg);
  --lds-date-picker-font-size: var(--lds-control-font-size-lg);
}

:host([invalid]),
:host([aria-invalid="true"]) {
  --lds-date-picker-border: var(--lds-color-danger-border);
}

.wrap {
  display: flex;
  align-items: center;
  box-sizing: border-box;
  min-height: var(--lds-date-picker-height);
  padding-inline: var(--lds-date-picker-padding-x);
  gap: var(--lds-control-gap-md);
  border: var(--lds-control-border-width-default) solid var(--lds-date-picker-border);
  border-radius: var(--lds-date-picker-radius);
  background: var(--lds-date-picker-bg);
  color: var(--lds-date-picker-fg);
  box-shadow: var(--lds-shadow-sm);
}

.wrap:focus-within {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.prefix,
.suffix {
  display: inline-flex;
  flex: none;
  align-items: center;
  color: var(--lds-color-fg-muted);
}

.prefix[data-empty],
.suffix[data-empty] {
  display: none;
}

.input {
  flex: 1 1 auto;
  min-width: 0;
  width: 100%;
  margin: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: var(--lds-date-picker-font-size);
  line-height: 1;
  color-scheme: inherit;
}

.input:focus {
  outline: none;
}

.input:disabled {
  color: var(--lds-color-fg-disabled);
  cursor: not-allowed;
}

.input::-webkit-calendar-picker-indicator {
  cursor: pointer;
  opacity: 0.7;
}

.input::-webkit-calendar-picker-indicator:hover {
  opacity: 1;
}

:host([disabled]) .wrap {
  background: var(--lds-color-bg-subtle);
  cursor: not-allowed;
}

:host([disabled]) .input::-webkit-calendar-picker-indicator {
  cursor: not-allowed;
  opacity: 0.4;
}`,
  });

  /* component: lds-date-range (draft) */
  LDS.define({
    tag: "lds-date-range",
    className: "LdsDateRange",
    formAssociated: true,
    observed: ["name", "start", "end", "min", "max", "label", "start-label", "end-label", "indicator", "size", "disabled", "required", "invalid"],
    defaults: {"indicator": "end", "size": "md"},
    template: `<div class="wrap" part="wrap" role="group">
  <span class="prefix" part="prefix"><slot name="prefix"></slot></span>
  <input class="input start" part="start" type="date" />
  <span class="separator" part="separator" aria-hidden="true">
    <lds-icon name="chevron-right" size="sm"></lds-icon>
  </span>
  <input class="input end" part="end" type="date" />
</div>`,
    styles: `/* Two native date inputs in one control, styled to match lds-date-picker. */

:host {
  --lds-date-range-height: var(--lds-control-height-md);
  --lds-date-range-padding-x: var(--lds-control-padding-x-md);
  --lds-date-range-radius: var(--lds-control-radius-md);
  --lds-date-range-font-size: var(--lds-control-font-size-md);
  --lds-date-range-bg: var(--lds-color-bg-canvas);
  --lds-date-range-fg: var(--lds-color-fg-default);
  --lds-date-range-border: var(--lds-color-border-default);

  display: block;
  max-width: 100%;
  /* The layout stacks by the component's own width, not the viewport's. */
  container-type: inline-size;
}

:host([hidden]) {
  display: none;
}

:host([size="sm"]) {
  --lds-date-range-height: var(--lds-control-height-sm);
  --lds-date-range-padding-x: var(--lds-control-padding-x-sm);
  --lds-date-range-radius: var(--lds-control-radius-sm);
  --lds-date-range-font-size: var(--lds-control-font-size-sm);
}

:host([size="lg"]) {
  --lds-date-range-height: var(--lds-control-height-lg);
  --lds-date-range-padding-x: var(--lds-control-padding-x-lg);
  --lds-date-range-radius: var(--lds-control-radius-lg);
  --lds-date-range-font-size: var(--lds-control-font-size-lg);
}

:host([invalid]),
:host([aria-invalid="true"]) {
  --lds-date-range-border: var(--lds-color-danger-border);
}

.wrap {
  display: flex;
  align-items: center;
  box-sizing: border-box;
  min-height: var(--lds-date-range-height);
  padding-inline: var(--lds-date-range-padding-x);
  gap: var(--lds-control-gap-md);
  border: var(--lds-control-border-width-default) solid var(--lds-date-range-border);
  border-radius: var(--lds-date-range-radius);
  background: var(--lds-date-range-bg);
  color: var(--lds-date-range-fg);
  box-shadow: var(--lds-shadow-sm);
}

.wrap:focus-within {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.prefix,
.separator {
  display: inline-flex;
  flex: none;
  align-items: center;
  color: var(--lds-color-fg-muted);
}

.prefix[data-empty] {
  display: none;
}

.input {
  flex: 1 1 8rem;
  min-width: 0;
  margin: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: var(--lds-date-range-font-size);
  line-height: 1;
  color-scheme: inherit;
}

.input:focus {
  outline: none;
}

.input:disabled {
  color: var(--lds-color-fg-disabled);
  cursor: not-allowed;
}

.input::-webkit-calendar-picker-indicator {
  cursor: pointer;
  opacity: 0.7;
}

.input::-webkit-calendar-picker-indicator:hover {
  opacity: 1;
}

/* Chromium only: Firefox draws its picker button internally and Safari has
 * none, so there the icon stays where the browser puts it. */
:host([indicator="start"]) .input::-webkit-calendar-picker-indicator {
  order: -1;
  margin-inline: 0 var(--lds-space-2);
}

:host([disabled]) .wrap {
  background: var(--lds-color-bg-subtle);
  cursor: not-allowed;
}

:host([disabled]) .input::-webkit-calendar-picker-indicator {
  cursor: not-allowed;
  opacity: 0.4;
}

/* Too narrow for both dates side by side: three lines, start date, a
 * downward chevron, end date. The dates fill the width so their calendar
 * buttons stay at the end; the chevron is centred under the date text. That
 * text is ten characters in the formats browsers use (10/01/2026,
 * 01.10.2026, 2026-10-01), and its separators are narrower than digits, so
 * its width is about 9ch. Last in the file so it overrides the base rules. */
@container (width < 20rem) {
  .wrap {
    flex-direction: column;
    align-items: stretch;
    gap: var(--lds-space-1);
    padding-block: var(--lds-space-2);
  }

  .input {
    flex: none;
  }

  .separator {
    align-self: flex-start;
    justify-content: center;
    width: 9ch;
    font-size: var(--lds-date-range-font-size);
  }

  /* The text starts after the calendar button: its width plus the gap set on
   * it above. */
  :host([indicator="start"]) .separator {
    margin-inline-start: calc(1.25em + var(--lds-space-2));
  }

  .separator lds-icon {
    rotate: 90deg;
  }
}`,
  });

  /* component: lds-field (draft) */
  LDS.define({
    tag: "lds-field",
    className: "LdsField",
    formAssociated: false,
    observed: ["label", "required"],
    defaults: {},
    template: `<div class="field" part="field">
  <div class="label-row" part="label-row">
    <label class="label" part="label"></label>
    <span class="required" part="required" hidden aria-hidden="true">*</span>
  </div>
  <div class="control" part="control"><slot></slot></div>
  <div class="help" part="help" id=""><slot name="help"></slot></div>
  <div class="error" part="error" id="" role="alert"><slot name="error"></slot></div>
</div>`,
    styles: `:host {
  display: block;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

.field {
  display: flex;
  flex-direction: column;
  gap: var(--lds-space-2);
}

.label-row {
  display: flex;
  align-items: baseline;
  gap: var(--lds-space-1);
}

.label {
  color: var(--lds-color-fg-default);
  font-size: var(--lds-font-size-sm);
  font-weight: var(--lds-font-weight-medium);
  line-height: var(--lds-font-leading-snug);
}

.required {
  color: var(--lds-color-danger-default);
  font-weight: var(--lds-font-weight-semibold);
}

.required[hidden] {
  display: none;
}

.control {
  min-width: 0;
}

.help,
.error {
  font-size: var(--lds-font-size-xs);
  line-height: var(--lds-font-leading-normal);
}

.help {
  color: var(--lds-color-fg-muted);
}

.error {
  color: var(--lds-color-danger-fg);
}

.help[data-empty],
.error[data-empty] {
  display: none;
}

:host([invalid]) .label {
  color: var(--lds-color-danger-fg);
}`,
  });

  /* component: lds-input (draft) */
  LDS.define({
    tag: "lds-input",
    className: "LdsInput",
    formAssociated: true,
    observed: ["type", "name", "value", "placeholder", "size", "disabled", "readonly", "required", "invalid", "maxlength"],
    defaults: {"type": "text", "size": "md"},
    template: `<div class="wrap" part="wrap">
  <span class="prefix" part="prefix"><slot name="prefix"></slot></span>
  <input class="input" part="input" />
  <span class="suffix" part="suffix"><slot name="suffix"></slot></span>
</div>`,
    styles: `:host {
  --lds-input-height: var(--lds-control-height-md);
  --lds-input-padding-x: var(--lds-control-padding-x-md);
  --lds-input-radius: var(--lds-control-radius-md);
  --lds-input-font-size: var(--lds-control-font-size-md);
  --lds-input-bg: var(--lds-color-bg-canvas);
  --lds-input-fg: var(--lds-color-fg-default);
  --lds-input-border: var(--lds-color-border-default);

  display: block;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

:host([size="sm"]) {
  --lds-input-height: var(--lds-control-height-sm);
  --lds-input-padding-x: var(--lds-control-padding-x-sm);
  --lds-input-radius: var(--lds-control-radius-sm);
  --lds-input-font-size: var(--lds-control-font-size-sm);
}

:host([size="lg"]) {
  --lds-input-height: var(--lds-control-height-lg);
  --lds-input-padding-x: var(--lds-control-padding-x-lg);
  --lds-input-radius: var(--lds-control-radius-lg);
  --lds-input-font-size: var(--lds-control-font-size-lg);
}

:host([invalid]),
:host([aria-invalid="true"]) {
  --lds-input-border: var(--lds-color-danger-border);
}

.wrap {
  display: flex;
  align-items: center;
  box-sizing: border-box;
  min-height: var(--lds-input-height);
  padding-inline: var(--lds-input-padding-x);
  gap: var(--lds-control-gap-md);
  border: var(--lds-control-border-width-default) solid var(--lds-input-border);
  border-radius: var(--lds-input-radius);
  background: var(--lds-input-bg);
  color: var(--lds-input-fg);
  box-shadow: var(--lds-shadow-sm);
}

.wrap:focus-within {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.prefix,
.suffix {
  display: inline-flex;
  flex: none;
  align-items: center;
  color: var(--lds-color-fg-muted);
}

.prefix[data-empty],
.suffix[data-empty] {
  display: none;
}

.input {
  flex: 1 1 auto;
  min-width: 0;
  margin: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: var(--lds-input-font-size);
  line-height: 1;
}

.input:focus {
  outline: none;
}

.input::placeholder {
  color: var(--lds-color-fg-subtle);
}

.input:disabled {
  color: var(--lds-color-fg-disabled);
  cursor: not-allowed;
}

:host([disabled]) .wrap {
  background: var(--lds-color-bg-subtle);
  cursor: not-allowed;
}`,
  });

  /* component: lds-option (draft) */
  LDS.define({
    tag: "lds-option",
    className: "LdsOption",
    formAssociated: false,
    observed: ["value", "label", "keywords", "disabled"],
    defaults: {},
    template: `<div class="option" part="option">
  <span class="check" part="check" aria-hidden="true"><lds-icon name="check" size="sm"></lds-icon></span>
  <span class="start" part="start"><slot name="start"></slot></span>
  <span class="label" part="label"><slot></slot></span>
  <span class="end" part="end"><slot name="end"></slot></span>
</div>`,
    styles: `/* One choice inside lds-combobox. The combobox owns selection, the active
 * highlight and keyboard handling; this component only renders the row. */

:host {
  --lds-option-bg: transparent;
  --lds-option-bg-active: var(--lds-color-bg-subtle);
  --lds-option-fg: var(--lds-color-fg-default);

  display: block;
  cursor: pointer;
}

:host([hidden]) {
  display: none;
}

:host([data-active]) {
  --lds-option-bg: var(--lds-option-bg-active);
}

.option {
  display: flex;
  box-sizing: border-box;
  align-items: center;
  gap: var(--lds-control-gap-sm);
  min-height: var(--lds-control-height-sm);
  padding: var(--lds-space-1) var(--lds-control-padding-x-sm);
  border-radius: var(--lds-control-radius-sm);
  background: var(--lds-option-bg);
  color: var(--lds-option-fg);
  font-size: var(--lds-font-size-sm);
  line-height: var(--lds-font-leading-snug);
}

/* The check keeps its width when hidden so labels line up whether or not a
 * row is selected. */
.check {
  display: inline-flex;
  flex: none;
  visibility: hidden;
  color: var(--lds-color-accent-fg);
}

:host([aria-selected="true"]) .check {
  visibility: visible;
}

.start,
.end {
  display: inline-flex;
  flex: none;
  align-items: center;
}

.end {
  color: var(--lds-color-fg-muted);
  font-size: var(--lds-font-size-xs);
}

.start[data-empty],
.end[data-empty] {
  display: none;
}

.label {
  flex: 1 1 auto;
  min-width: 0;
  overflow-wrap: anywhere;
}

:host([disabled]) {
  cursor: not-allowed;
}

:host([disabled]) .option {
  color: var(--lds-color-fg-disabled);
}`,
  });

  /* component: lds-radio-group (draft) */
  LDS.define({
    tag: "lds-radio-group",
    className: "LdsRadioGroup",
    formAssociated: true,
    observed: ["name", "value", "label", "orientation", "disabled"],
    defaults: {"orientation": "vertical"},
    template: `<div class="root" part="root" role="presentation">
  <div class="label" part="label" hidden></div>
  <div class="options" part="options" role="radiogroup">
    <slot></slot>
  </div>
</div>`,
    styles: `:host {
  display: block;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

.root {
  display: flex;
  flex-direction: column;
  gap: var(--lds-space-2);
}

.label {
  color: var(--lds-color-fg-default);
  font-size: var(--lds-font-size-sm);
  font-weight: var(--lds-font-weight-medium);
  line-height: var(--lds-font-leading-snug);
}

.label[hidden] {
  display: none;
}

.options {
  display: flex;
  flex-direction: column;
  gap: var(--lds-space-2);
}

:host([orientation="horizontal"]) .options {
  flex-direction: row;
  flex-wrap: wrap;
  gap: var(--lds-space-3);
}

::slotted(button),
::slotted([role="radio"]) {
  display: inline-flex;
  align-items: center;
  gap: var(--lds-space-2);
  margin: 0;
  padding: var(--lds-space-2) var(--lds-space-3);
  border: var(--lds-control-border-width-default) solid var(--lds-color-border-default);
  border-radius: var(--lds-control-radius-md);
  background: var(--lds-color-bg-canvas);
  color: var(--lds-color-fg-default);
  font: inherit;
  font-size: var(--lds-font-size-sm);
  line-height: var(--lds-font-leading-snug);
  text-align: start;
  cursor: pointer;
}

::slotted(button:hover),
::slotted([role="radio"]:hover) {
  background: var(--lds-color-bg-subtle);
}

::slotted([aria-checked="true"]) {
  border-color: var(--lds-color-accent-default);
  background: var(--lds-color-accent-subtle);
  color: var(--lds-color-accent-fg);
}

::slotted(button:focus-visible),
::slotted([role="radio"]:focus-visible) {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

::slotted([disabled]),
::slotted([aria-disabled="true"]) {
  opacity: 0.55;
  cursor: not-allowed;
}

:host([disabled]) {
  opacity: 0.55;
  pointer-events: none;
}`,
  });

  /* component: lds-segment (draft) */
  LDS.define({
    tag: "lds-segment",
    className: "LdsSegment",
    formAssociated: false,
    observed: ["value", "label", "disabled"],
    defaults: {},
    template: `<span class="segment" part="segment">
  <span class="start" part="start"><slot name="start"></slot></span>
  <span class="label" part="label"><slot></slot></span>
</span>`,
    styles: `/* One option in lds-segmented. The host is the radio: it takes focus and
 * carries aria-checked, set by the parent. */

:host {
  display: inline-flex;
  flex: none;
  min-width: 0;
  border-radius: var(--lds-control-radius-sm);
  color: var(--lds-color-fg-muted);
  cursor: pointer;
  transition:
    background var(--lds-transition-control),
    color var(--lds-transition-control);
}

:host([hidden]) {
  display: none;
}

:host(:hover) {
  color: var(--lds-color-fg-default);
}

:host([aria-checked="true"]) {
  background: var(--lds-color-bg-canvas);
  color: var(--lds-color-fg-default);
  box-shadow: var(--lds-shadow-sm);
}

:host(:focus-visible) {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

:host([aria-disabled="true"]) {
  color: var(--lds-color-fg-disabled);
  cursor: not-allowed;
}

.segment {
  display: inline-flex;
  box-sizing: border-box;
  flex: 1 1 auto;
  align-items: center;
  justify-content: center;
  gap: var(--lds-control-gap-sm);
  min-width: 0;
  padding-inline: var(--lds-control-padding-x-sm);
  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-segmented-font-size, var(--lds-control-font-size-md));
  font-weight: var(--lds-font-weight-medium);
  line-height: 1;
  white-space: nowrap;
}

.start {
  display: inline-flex;
  flex: none;
}

.start[data-empty],
.label[data-empty] {
  display: none;
}

.label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  /* A taller line box so overflow clipping does not cut descenders. */
  line-height: var(--lds-font-leading-snug);
}`,
  });

  /* component: lds-segmented (draft) */
  LDS.define({
    tag: "lds-segmented",
    className: "LdsSegmented",
    formAssociated: true,
    observed: ["name", "value", "label", "size", "full-width", "disabled"],
    defaults: {"size": "md"},
    template: `<div class="track" part="track" role="radiogroup">
  <slot></slot>
</div>`,
    styles: `/* Track of lds-segment children. Segment height follows the track size via
 * inherited custom properties. */

:host {
  --lds-segmented-height: var(--lds-control-height-md);
  --lds-segmented-font-size: var(--lds-control-font-size-md);
  --lds-segmented-padding: calc(var(--lds-space-1) / 2);
  --lds-segmented-bg: var(--lds-color-bg-muted);

  display: inline-flex;
  vertical-align: middle;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

:host([size="sm"]) {
  --lds-segmented-height: var(--lds-control-height-sm);
  --lds-segmented-font-size: var(--lds-control-font-size-sm);
}

:host([full-width]) {
  display: flex;
}

.track {
  display: inline-flex;
  box-sizing: border-box;
  flex: 1 1 auto;
  align-items: stretch;
  gap: var(--lds-segmented-padding);
  min-width: 0;
  height: var(--lds-segmented-height);
  padding: var(--lds-segmented-padding);
  border-radius: var(--lds-control-radius-md);
  background: var(--lds-segmented-bg);
  overflow-x: auto;
}

:host([full-width]) ::slotted(lds-segment) {
  flex: 1 1 0;
}

:host([disabled]) .track {
  opacity: 0.55;
}`,
  });

  /* component: lds-select (draft) */
  LDS.define({
    tag: "lds-select",
    className: "LdsSelect",
    formAssociated: true,
    observed: ["name", "value", "size", "disabled", "required", "invalid"],
    defaults: {"size": "md"},
    template: `<div class="wrap" part="wrap">
  <select class="select" part="select"></select>
  <span class="chevron" part="chevron" aria-hidden="true">
    <lds-icon name="chevron-down" size="sm"></lds-icon>
  </span>
</div>
<slot hidden></slot>`,
    styles: `:host {
  --lds-select-height: var(--lds-control-height-md);
  --lds-select-padding-x: var(--lds-control-padding-x-md);
  --lds-select-radius: var(--lds-control-radius-md);
  --lds-select-font-size: var(--lds-control-font-size-md);
  --lds-select-bg: var(--lds-color-bg-canvas);
  --lds-select-fg: var(--lds-color-fg-default);
  --lds-select-border: var(--lds-color-border-default);

  display: block;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

:host([size="sm"]) {
  --lds-select-height: var(--lds-control-height-sm);
  --lds-select-padding-x: var(--lds-control-padding-x-sm);
  --lds-select-radius: var(--lds-control-radius-sm);
  --lds-select-font-size: var(--lds-control-font-size-sm);
}

:host([size="lg"]) {
  --lds-select-height: var(--lds-control-height-lg);
  --lds-select-padding-x: var(--lds-control-padding-x-lg);
  --lds-select-radius: var(--lds-control-radius-lg);
  --lds-select-font-size: var(--lds-control-font-size-lg);
}

:host([invalid]),
:host([aria-invalid="true"]) {
  --lds-select-border: var(--lds-color-danger-border);
}

.wrap {
  position: relative;
  display: flex;
  align-items: center;
  box-sizing: border-box;
  min-height: var(--lds-select-height);
  border: var(--lds-control-border-width-default) solid var(--lds-select-border);
  border-radius: var(--lds-select-radius);
  background: var(--lds-select-bg);
  color: var(--lds-select-fg);
  box-shadow: var(--lds-shadow-sm);
}

.wrap:focus-within {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.select {
  appearance: none;
  flex: 1 1 auto;
  min-width: 0;
  width: 100%;
  height: var(--lds-select-height);
  margin: 0;
  padding-inline: var(--lds-select-padding-x) calc(var(--lds-select-padding-x) + var(--lds-icon-size-sm) + var(--lds-space-2));
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: var(--lds-select-font-size);
  line-height: 1;
  cursor: pointer;
}

.select:focus {
  outline: none;
}

.select:disabled {
  color: var(--lds-color-fg-disabled);
  cursor: not-allowed;
}

.chevron {
  position: absolute;
  inset-inline-end: var(--lds-select-padding-x);
  display: inline-flex;
  pointer-events: none;
  color: var(--lds-color-fg-muted);
}

:host([disabled]) .wrap {
  background: var(--lds-color-bg-subtle);
  cursor: not-allowed;
}`,
  });

  /* component: lds-switch (draft) */
  LDS.define({
    tag: "lds-switch",
    className: "LdsSwitch",
    formAssociated: true,
    observed: ["name", "value", "checked", "disabled", "required", "invalid"],
    defaults: {"value": "on"},
    template: `<label class="root" part="root">
  <span class="control" part="control">
    <input class="input" part="input" type="checkbox" role="switch" />
    <span class="track" part="track" aria-hidden="true">
      <span class="thumb" part="thumb"></span>
    </span>
  </span>
  <span class="label" part="label"><slot></slot></span>
</label>`,
    styles: `:host {
  --lds-switch-width: 2.75rem;
  --lds-switch-height: 1.5rem;
  --lds-switch-pad: 0.125rem;
  --lds-switch-track: var(--lds-color-bg-muted);
  --lds-switch-thumb: var(--lds-color-bg-canvas);

  display: inline-flex;
  max-width: 100%;
  vertical-align: middle;
}

:host([hidden]) {
  display: none;
}

:host([checked]) {
  --lds-switch-track: var(--lds-color-accent-default);
  --lds-switch-thumb: var(--lds-color-fg-on-accent);
}

:host([invalid]),
:host([aria-invalid="true"]) {
  --lds-switch-track: var(--lds-color-danger-muted);
}

.root {
  display: inline-flex;
  align-items: center;
  gap: var(--lds-space-2);
  cursor: pointer;
  color: var(--lds-color-fg-default);
  font-size: var(--lds-font-size-sm);
  line-height: var(--lds-font-leading-snug);
}

.control {
  position: relative;
  display: inline-flex;
  flex: none;
}

.input {
  position: absolute;
  inset: 0;
  margin: 0;
  opacity: 0;
  cursor: inherit;
}

.track {
  display: inline-flex;
  align-items: center;
  box-sizing: border-box;
  width: var(--lds-switch-width);
  height: var(--lds-switch-height);
  padding: var(--lds-switch-pad);
  border-radius: var(--lds-control-radius-pill);
  background: var(--lds-switch-track);
  transition: background-color 120ms ease;
}

.thumb {
  width: calc(var(--lds-switch-height) - (var(--lds-switch-pad) * 2));
  height: calc(var(--lds-switch-height) - (var(--lds-switch-pad) * 2));
  border-radius: var(--lds-control-radius-pill);
  background: var(--lds-switch-thumb);
  box-shadow: var(--lds-shadow-sm);
  transition: transform 120ms ease;
}

:host([checked]) .thumb {
  transform: translateX(calc(var(--lds-switch-width) - var(--lds-switch-height)));
}

.input:focus-visible + .track {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.label[hidden] {
  display: none;
}

:host([disabled]) {
  opacity: 0.55;
  cursor: not-allowed;
}

:host([disabled]) .root {
  cursor: not-allowed;
}`,
  });

  /* component: lds-textarea (draft) */
  LDS.define({
    tag: "lds-textarea",
    className: "LdsTextarea",
    formAssociated: true,
    observed: ["name", "value", "placeholder", "rows", "size", "auto-grow", "maxlength", "disabled", "readonly", "required", "invalid"],
    defaults: {"rows": "3", "size": "md"},
    template: `<div class="wrap" part="wrap">
  <textarea class="textarea" part="textarea" rows="3"></textarea>
</div>
<div class="counter" part="counter" hidden></div>`,
    styles: `:host {
  --lds-textarea-padding-x: var(--lds-control-padding-x-md);
  --lds-textarea-padding-y: var(--lds-space-3);
  --lds-textarea-radius: var(--lds-control-radius-md);
  --lds-textarea-font-size: var(--lds-control-font-size-md);
  --lds-textarea-bg: var(--lds-color-bg-canvas);
  --lds-textarea-fg: var(--lds-color-fg-default);
  --lds-textarea-border: var(--lds-color-border-default);

  display: block;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

:host([size="sm"]) {
  --lds-textarea-padding-x: var(--lds-control-padding-x-sm);
  --lds-textarea-padding-y: var(--lds-space-2);
  --lds-textarea-radius: var(--lds-control-radius-sm);
  --lds-textarea-font-size: var(--lds-control-font-size-sm);
}

:host([size="lg"]) {
  --lds-textarea-padding-x: var(--lds-control-padding-x-lg);
  --lds-textarea-padding-y: var(--lds-space-4);
  --lds-textarea-radius: var(--lds-control-radius-lg);
  --lds-textarea-font-size: var(--lds-control-font-size-lg);
}

:host([invalid]),
:host([aria-invalid="true"]) {
  --lds-textarea-border: var(--lds-color-danger-border);
}

.wrap {
  display: block;
  box-sizing: border-box;
  padding: var(--lds-textarea-padding-y) var(--lds-textarea-padding-x);
  border: var(--lds-control-border-width-default) solid var(--lds-textarea-border);
  border-radius: var(--lds-textarea-radius);
  background: var(--lds-textarea-bg);
  color: var(--lds-textarea-fg);
  box-shadow: var(--lds-shadow-sm);
}

.wrap:focus-within {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.textarea {
  display: block;
  width: 100%;
  margin: 0;
  padding: 0;
  border: 0;
  resize: vertical;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: var(--lds-textarea-font-size);
  line-height: var(--lds-font-leading-normal);
  field-sizing: content;
}

:host([auto-grow]) .textarea {
  resize: none;
  overflow: hidden;
}

.textarea:focus {
  outline: none;
}

.textarea::placeholder {
  color: var(--lds-color-fg-subtle);
}

.textarea:disabled {
  color: var(--lds-color-fg-disabled);
  cursor: not-allowed;
  resize: none;
}

:host([disabled]) .wrap {
  background: var(--lds-color-bg-subtle);
  cursor: not-allowed;
}

.counter {
  margin-block-start: var(--lds-space-1);
  color: var(--lds-color-fg-muted);
  font-size: var(--lds-font-size-xs);
  text-align: end;
}

.counter[hidden] {
  display: none;
}`,
  });

  /* component: lds-card (draft) */
  LDS.define({
    tag: "lds-card",
    className: "LdsCard",
    formAssociated: false,
    observed: ["interactive"],
    defaults: {},
    template: `<div class="card" part="card">
  <div class="media" part="media"><slot name="media"></slot></div>
  <div class="header" part="header"><slot name="header"></slot></div>
  <div class="body" part="body"><slot></slot></div>
  <div class="footer" part="footer"><slot name="footer"></slot></div>
</div>`,
    styles: `/* Grouped content surface with optional header, media and footer. */

:host {
  display: block;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

:host([interactive]) .card {
  transition:
    box-shadow var(--lds-transition-surface),
    border-color var(--lds-transition-surface);
  cursor: pointer;
}

:host([interactive]) .card:hover {
  border-color: var(--lds-color-border-strong);
  box-shadow: var(--lds-shadow-lg);
}

.card {
  display: flex;
  flex-direction: column;
  gap: var(--lds-space-4);
  box-sizing: border-box;
  padding: var(--lds-card-padding);
  border: var(--lds-control-border-width-default) solid var(--lds-card-border);
  border-radius: var(--lds-card-radius);
  background: var(--lds-color-bg-raised);
  color: var(--lds-color-fg-default);
  box-shadow: var(--lds-card-shadow);
}

.media,
.header,
.body,
.footer {
  min-width: 0;
}

.media[data-empty],
.header[data-empty],
.body[data-empty],
.footer[data-empty] {
  display: none;
}

.media {
  margin: calc(var(--lds-card-padding) * -1);
  margin-bottom: 0;
  overflow: hidden;
  border-radius: var(--lds-card-radius) var(--lds-card-radius) 0 0;
}

.media ::slotted(img) {
  display: block;
  width: 100%;
  height: auto;
}

.header {
  font-family: var(--lds-font-family-display);
  font-size: var(--lds-font-size-lg);
  font-weight: var(--lds-font-weight-semibold);
  line-height: var(--lds-font-leading-tight);
}

.footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--lds-space-3);
  padding-top: var(--lds-space-2);
  border-top: var(--lds-control-border-width-default) solid var(--lds-color-border-subtle);
}`,
  });

  /* component: lds-tab-panel (draft) */
  LDS.define({
    tag: "lds-tab-panel",
    className: "LdsTabPanel",
    formAssociated: false,
    observed: ["name", "label", "active"],
    defaults: {},
    template: `<div class="panel" part="panel">
  <slot></slot>
</div>`,
    styles: `:host {
  display: block;
}

:host([hidden]) {
  display: none;
}

.panel {
  min-width: 0;
}`,
  });

  /* component: lds-tabs (draft) */
  LDS.define({
    tag: "lds-tabs",
    className: "LdsTabs",
    formAssociated: false,
    observed: ["value", "activation"],
    defaults: {"activation": "auto"},
    template: `<div class="root" part="root">
  <div class="tablist" part="tablist" role="tablist"></div>
  <div class="panels" part="panels">
    <slot></slot>
  </div>
</div>`,
    styles: `/* Tablist + panels. Buttons are built in JS from lds-tab-panel children. */

:host {
  display: block;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

.root {
  display: flex;
  flex-direction: column;
  gap: var(--lds-space-4);
}

.tablist {
  display: flex;
  flex-wrap: wrap;
  gap: var(--lds-space-1);
  border-bottom: var(--lds-control-border-width-default) solid var(--lds-color-border-default);
}

.tab {
  box-sizing: border-box;
  margin: 0 0 calc(var(--lds-control-border-width-default) * -1);
  padding: var(--lds-space-2) var(--lds-space-4);
  border: 0;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: var(--lds-color-fg-muted);
  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-font-size-sm);
  font-weight: var(--lds-font-weight-medium);
  cursor: pointer;
}

.tab:hover {
  color: var(--lds-color-fg-default);
}

.tab:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.tab[aria-selected="true"] {
  border-bottom-color: var(--lds-color-accent-default);
  color: var(--lds-color-fg-default);
  font-weight: var(--lds-font-weight-semibold);
}

.panels {
  min-width: 0;
}`,
  });

  /* component: lds-breadcrumb (draft) */
  LDS.define({
    tag: "lds-breadcrumb",
    className: "LdsBreadcrumb",
    formAssociated: false,
    observed: ["label", "expanded", "max-items"],
    defaults: {"max-items": "1"},
    template: `<nav class="root" part="root">
  <div class="list" part="list" role="list">
    <slot></slot>
    <button type="button" class="overflow" part="overflow" hidden aria-expanded="false">
      <span class="overflow__sep" aria-hidden="true"></span>
      <span class="overflow__icon" aria-hidden="true"></span>
    </button>
  </div>
</nav>`,
    styles: `/* Hierarchy trail. Always one line: middle items collapse behind the overflow
 * control when the host is too narrow; expanded shows every step. */

:host {
  --lds-breadcrumb-fg: var(--lds-color-fg-muted);
  --lds-breadcrumb-fg-current: var(--lds-color-fg-default);
  --lds-breadcrumb-gap: var(--lds-space-1);

  display: block;
  max-width: 100%;
  min-width: 0;
  color: var(--lds-breadcrumb-fg);
}

:host([hidden]) {
  display: none;
}

.root {
  max-width: 100%;
  min-width: 0;
}

.list {
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: var(--lds-breadcrumb-gap);
  max-width: 100%;
  min-width: 0;
  margin: 0;
  padding: 0;
  list-style: none;
}

/* Expanded in a tight container: scroll instead of wrapping (wrapping puts
 * leading chevrons at the start of new lines). */
:host([expanded]) .list {
  overflow-x: auto;
}

.overflow {
  display: none;
  flex: none;
  align-items: center;
  gap: var(--lds-breadcrumb-gap);
  order: 2;

  height: var(--lds-control-height-sm);
  padding: 0;
  padding-inline-end: var(--lds-space-1);

  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-font-size-sm);
  font-weight: var(--lds-font-weight-medium);
  line-height: 1;
  color: var(--lds-color-fg-muted);

  background: transparent;
  border: none;
  border-radius: var(--lds-control-radius-sm);
  cursor: pointer;
  appearance: none;
  transition:
    background var(--lds-transition-control),
    color var(--lds-transition-control);
}

.overflow:hover {
  color: var(--lds-color-fg-default);
}

.overflow:hover .overflow__icon {
  background: var(--lds-color-bg-subtle);
}

.overflow:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

:host([data-collapsed]:not([expanded])) .overflow {
  display: inline-flex;
}

/* Match breadcrumb-item separators so Home > … > Current reads evenly. */
.overflow__sep {
  display: inline-flex;
  flex: none;
  width: 0.4em;
  height: 0.4em;
  margin-inline: var(--lds-space-1);
  border-inline-end: 2px solid var(--lds-color-border-strong);
  border-block-end: 2px solid var(--lds-color-border-strong);
  rotate: -45deg;
}

.overflow__icon {
  display: inline-flex;
  position: relative;
  flex: none;
  align-items: center;
  justify-content: center;
  min-width: var(--lds-control-height-sm);
  height: var(--lds-control-height-sm);
  border-radius: var(--lds-control-radius-sm);
  transition: background var(--lds-transition-control);
}

.overflow__icon::before,
.overflow__icon::after {
  content: "";
  position: absolute;
  inset-block-start: 50%;
  width: 0.25em;
  height: 0.25em;
  background: currentColor;
  border-radius: var(--lds-radius-full);
  translate: 0 -50%;
}

.overflow__icon::before {
  inset-inline-start: calc(50% - 0.45em);
  box-shadow: 0.45em 0 0 currentColor;
}

.overflow__icon::after {
  inset-inline-start: calc(50% + 0.45em);
}

::slotted(lds-breadcrumb-item) {
  flex: none;
  min-width: 0;
  max-width: 100%;
}

::slotted(lds-breadcrumb-item:first-child) {
  order: 1;
}

::slotted(lds-breadcrumb-item:not(:first-child)) {
  order: 3;
}

/* Collapsed single-line: last step fills the remainder and ellipsizes. */
:host([data-collapsed]:not([expanded])) ::slotted(lds-breadcrumb-item:last-child) {
  flex: 1 1 0;
  min-width: 0;
}`,
  });

  /* component: lds-breadcrumb-item (draft) */
  LDS.define({
    tag: "lds-breadcrumb-item",
    className: "LdsBreadcrumbItem",
    formAssociated: false,
    observed: ["href", "target", "current"],
    defaults: {},
    template: `<span class="item" part="item" role="listitem">
  <span class="sep" part="separator" aria-hidden="true"></span>
  <a class="link" part="link"><slot></slot></a>
</span>`,
    styles: `/* One step in an lds-breadcrumb. The leading separator hides on :first-child. */

:host {
  display: inline-flex;
  align-items: center;
  min-width: 0;
  max-width: 100%;
  vertical-align: middle;
}

:host([hidden]),
:host([data-overflow]) {
  display: none;
}

.item {
  display: inline-flex;
  align-items: center;
  gap: var(--lds-breadcrumb-gap, var(--lds-space-1));
  min-width: 0;
  max-width: 100%;
}

.sep {
  display: inline-flex;
  flex: none;
  width: 0.4em;
  height: 0.4em;
  margin-inline: var(--lds-space-1);
  border-inline-end: 2px solid var(--lds-color-border-strong);
  border-block-end: 2px solid var(--lds-color-border-strong);
  rotate: -45deg;
}

:host(:first-child) .sep {
  display: none;
}

.link {
  display: inline-block;
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;

  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-font-size-sm);
  font-weight: var(--lds-font-weight-medium);
  line-height: var(--lds-font-leading-snug);
  color: var(--lds-breadcrumb-fg, var(--lds-color-fg-muted));
  text-decoration: none;
  border-radius: var(--lds-control-radius-sm);
  transition: color var(--lds-transition-control);
}

a.link:hover {
  color: var(--lds-color-fg-link);
  text-decoration: underline;
  text-underline-offset: 0.15em;
}

.link:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

:host([current]) .link,
.link[aria-current="page"] {
  color: var(--lds-breadcrumb-fg-current, var(--lds-color-fg-default));
  font-weight: var(--lds-font-weight-semibold);
  text-decoration: none;
  pointer-events: none;
}`,
  });

  /* component: lds-command (draft) */
  LDS.define({
    tag: "lds-command",
    className: "LdsCommand",
    formAssociated: false,
    observed: ["open", "placeholder", "empty", "hotkey"],
    defaults: {"placeholder": "Search…", "empty": "No results"},
    template: `<dialog class="dialog" part="dialog">
  <div class="chrome" part="chrome">
    <div class="search" part="search">
      <span class="search__icon" aria-hidden="true">
        <lds-icon name="search" size="sm"></lds-icon>
      </span>
      <input
        class="input"
        part="input"
        type="search"
        role="combobox"
        aria-autocomplete="list"
        aria-expanded="true"
        autocomplete="off"
        spellcheck="false"
      />
      <span class="search__hint" part="hint" aria-hidden="true">esc</span>
    </div>
    <div class="list" part="list" role="listbox" tabindex="-1">
      <slot></slot>
    </div>
    <div class="empty" part="empty" hidden>No results</div>
  </div>
</dialog>`,
    styles: `/* Global command palette / search. Native <dialog> + filterable listbox. */

:host {
  --lds-command-width: 36rem;
  --lds-command-bg: var(--lds-color-bg-raised);
  --lds-command-fg: var(--lds-color-fg-default);
  --lds-command-radius: var(--lds-overlay-radius);
  --lds-command-shadow: var(--lds-overlay-shadow);
  --lds-command-max-height: min(28rem, calc(100vh - var(--lds-space-16)));

  display: contents;
}

:host([hidden]) {
  display: none;
}

.dialog {
  box-sizing: border-box;
  width: min(var(--lds-command-width), calc(100vw - var(--lds-space-8)));
  max-height: var(--lds-command-max-height);
  margin: 12vh auto auto;
  padding: 0;
  border: var(--lds-control-border-width-default) solid var(--lds-color-border-default);
  border-radius: var(--lds-command-radius);
  background: var(--lds-command-bg);
  color: var(--lds-command-fg);
  box-shadow: var(--lds-command-shadow);
  overflow: hidden;
}

.dialog::backdrop {
  background: var(--lds-color-bg-overlay);
}

.chrome {
  display: flex;
  flex-direction: column;
  max-height: var(--lds-command-max-height);
}

.search {
  display: flex;
  align-items: center;
  gap: var(--lds-space-3);
  padding: var(--lds-space-3) var(--lds-space-4);
  border-block-end: var(--lds-control-border-width-default) solid var(--lds-color-border-subtle);
}

.search__icon {
  display: inline-flex;
  flex: none;
  color: var(--lds-color-fg-muted);
}

.input {
  flex: 1 1 auto;
  min-width: 0;
  margin: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: var(--lds-font-size-md);
  line-height: var(--lds-font-leading-normal);
  outline: none;
}

.input::placeholder {
  color: var(--lds-color-fg-subtle);
}

.input::-webkit-search-cancel-button {
  appearance: none;
}

.search__hint {
  flex: none;
  padding: var(--lds-space-1) var(--lds-space-2);
  border: var(--lds-control-border-width-default) solid var(--lds-color-border-default);
  border-radius: var(--lds-control-radius-sm);
  color: var(--lds-color-fg-muted);
  font-family: var(--lds-font-family-mono);
  font-size: var(--lds-font-size-2xs);
  line-height: 1;
  text-transform: uppercase;
}

.list {
  flex: 1 1 auto;
  min-height: 0;
  max-height: calc(var(--lds-command-max-height) - 4rem);
  padding: var(--lds-space-2);
  overflow: auto;
}

.list:focus {
  outline: none;
}

.empty {
  padding: var(--lds-space-8) var(--lds-space-4);
  color: var(--lds-color-fg-muted);
  font-size: var(--lds-font-size-sm);
  text-align: center;
}

.empty[hidden] {
  display: none;
}

:host([data-empty]) .list {
  display: none;
}`,
  });

  /* component: lds-command-group (draft) */
  LDS.define({
    tag: "lds-command-group",
    className: "LdsCommandGroup",
    formAssociated: false,
    observed: ["heading"],
    defaults: {},
    template: `<div class="group" part="group" role="group">
  <div class="heading" part="heading"></div>
  <div class="items" part="items"><slot></slot></div>
</div>`,
    styles: `/* Labelled group of command items. */

:host {
  display: block;
}

:host([hidden]) {
  display: none;
}

:host(:not(:first-child)) {
  margin-block-start: var(--lds-space-2);
  padding-block-start: var(--lds-space-2);
  border-block-start: var(--lds-control-border-width-default) solid var(--lds-color-border-subtle);
}

.heading {
  padding: var(--lds-space-2) var(--lds-control-padding-x-sm);
  color: var(--lds-color-fg-muted);
  font-size: var(--lds-font-size-xs);
  font-weight: var(--lds-font-weight-semibold);
  line-height: var(--lds-font-leading-snug);
  letter-spacing: var(--lds-font-tracking-wide);
  text-transform: uppercase;
}

.heading:empty {
  display: none;
}

.items {
  display: flex;
  flex-direction: column;
  gap: var(--lds-space-1);
}`,
  });

  /* component: lds-command-item (draft) */
  LDS.define({
    tag: "lds-command-item",
    className: "LdsCommandItem",
    formAssociated: false,
    observed: ["value", "keywords", "href", "target", "disabled"],
    defaults: {},
    template: `<button class="item" part="item" type="button">
  <span class="item__start" part="start"><slot name="start"></slot></span>
  <span class="item__label" part="label"><slot></slot></span>
  <span class="item__end" part="end"><slot name="end"></slot></span>
</button>`,
    styles: `/* Selectable row inside lds-command. */

:host {
  --lds-command-item-bg: transparent;
  --lds-command-item-bg-active: var(--lds-color-bg-subtle);
  --lds-command-item-fg: var(--lds-color-fg-default);
  --lds-command-item-radius: var(--lds-control-radius-sm);
  --lds-command-item-padding-x: var(--lds-control-padding-x-sm);
  --lds-command-item-padding-y: var(--lds-space-2);
  --lds-command-item-gap: var(--lds-control-gap-sm);
  --lds-command-item-font-size: var(--lds-font-size-sm);

  display: block;
}

:host([hidden]) {
  display: none;
}

:host([data-active]) {
  --lds-command-item-bg: var(--lds-command-item-bg-active);
}

.item {
  display: flex;
  box-sizing: border-box;
  align-items: center;
  gap: var(--lds-command-item-gap);
  width: 100%;
  margin: 0;
  padding: var(--lds-command-item-padding-y) var(--lds-command-item-padding-x);
  border: 0;
  border-radius: var(--lds-command-item-radius);
  background: var(--lds-command-item-bg);
  color: var(--lds-command-item-fg);
  font: inherit;
  font-size: var(--lds-command-item-font-size);
  line-height: var(--lds-font-leading-normal);
  text-align: start;
  text-decoration: none;
  cursor: pointer;
}

.item:hover {
  background: var(--lds-command-item-bg-active);
}

.item:focus {
  outline: none;
}

.item:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: calc(var(--lds-focus-offset) * -1);
}

:host([disabled]) .item {
  color: var(--lds-color-fg-disabled);
  cursor: not-allowed;
  opacity: 0.7;
}

:host([disabled]) .item:hover {
  background: var(--lds-command-item-bg);
}

.item__start,
.item__end {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  color: var(--lds-color-fg-muted);
  font-size: var(--lds-font-size-xs);
}

.item__start[data-empty],
.item__end[data-empty] {
  display: none;
}

.item__label {
  flex: 1 1 auto;
  min-width: 0;
}`,
  });

  /* component: lds-menu (draft) */
  LDS.define({
    tag: "lds-menu",
    className: "LdsMenu",
    formAssociated: false,
    observed: ["open", "placement"],
    defaults: {"placement": "bottom-end"},
    template: `<div class="root" part="root">
  <div class="trigger" part="trigger">
    <slot name="trigger"></slot>
  </div>
  <div class="panel" part="panel" role="menu" hidden tabindex="-1">
    <slot></slot>
  </div>
</div>`,
    styles: `/* Action menu — trigger in flow, panel positioned with LDS.position. */

:host {
  --lds-menu-bg: var(--lds-color-bg-raised);
  --lds-menu-fg: var(--lds-color-fg-default);
  --lds-menu-border: var(--lds-color-border-default);
  --lds-menu-radius: var(--lds-overlay-radius);
  --lds-menu-shadow: var(--lds-overlay-shadow);
  --lds-menu-padding: var(--lds-space-1);
  --lds-menu-min-width: 12rem;

  display: inline-block;
  position: relative;
  vertical-align: middle;
}

:host([hidden]) {
  display: none;
}

.root {
  display: inline-block;
  max-width: 100%;
}

.trigger {
  display: inline-flex;
  max-width: 100%;
}

.panel {
  box-sizing: border-box;
  position: fixed;
  z-index: var(--lds-z-popover);
  display: none;
  flex-direction: column;
  gap: var(--lds-space-px);
  min-width: var(--lds-menu-min-width);
  padding: var(--lds-menu-padding);
  border: var(--lds-control-border-width-default) solid var(--lds-menu-border);
  border-radius: var(--lds-menu-radius);
  background: var(--lds-menu-bg);
  color: var(--lds-menu-fg);
  box-shadow: var(--lds-menu-shadow);
}

:host([open]) .panel {
  display: flex;
}

.panel[hidden] {
  display: none !important;
}

::slotted(hr) {
  display: block;
  height: 0;
  margin: var(--lds-space-1) 0;
  border: 0;
  border-block-start: var(--lds-control-border-width-default) solid var(--lds-color-border-subtle);
}`,
  });

  /* component: lds-menu-item (draft) */
  LDS.define({
    tag: "lds-menu-item",
    className: "LdsMenuItem",
    formAssociated: false,
    observed: ["href", "target", "disabled", "value"],
    defaults: {},
    template: `<button class="item" part="item" type="button" role="menuitem">
  <span class="item__start" part="start"><slot name="start"></slot></span>
  <span class="item__label" part="label"><slot></slot></span>
  <span class="item__end" part="end"><slot name="end"></slot></span>
</button>`,
    styles: `/* Single action inside lds-menu. */

:host {
  --lds-menu-item-bg: transparent;
  --lds-menu-item-bg-hover: var(--lds-color-bg-subtle);
  --lds-menu-item-fg: var(--lds-color-fg-default);
  --lds-menu-item-radius: var(--lds-control-radius-sm);
  --lds-menu-item-padding-x: var(--lds-control-padding-x-sm);
  --lds-menu-item-padding-y: var(--lds-space-2);
  --lds-menu-item-gap: var(--lds-control-gap-sm);
  --lds-menu-item-font-size: var(--lds-font-size-sm);

  display: block;
}

:host([hidden]) {
  display: none;
}

.item {
  display: flex;
  box-sizing: border-box;
  align-items: center;
  gap: var(--lds-menu-item-gap);
  width: 100%;
  margin: 0;
  padding: var(--lds-menu-item-padding-y) var(--lds-menu-item-padding-x);
  border: 0;
  border-radius: var(--lds-menu-item-radius);
  background: var(--lds-menu-item-bg);
  color: var(--lds-menu-item-fg);
  font: inherit;
  font-size: var(--lds-menu-item-font-size);
  line-height: var(--lds-font-leading-normal);
  text-align: start;
  text-decoration: none;
  cursor: pointer;
}

.item:hover {
  background: var(--lds-menu-item-bg-hover);
}

.item:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

:host([disabled]) .item {
  color: var(--lds-color-fg-disabled);
  cursor: not-allowed;
  opacity: 0.7;
}

:host([disabled]) .item:hover {
  background: var(--lds-menu-item-bg);
}

.item__start,
.item__end {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  color: var(--lds-color-fg-muted);
}

.item__start[data-empty],
.item__end[data-empty] {
  display: none;
}

.item__label {
  flex: 1 1 auto;
  min-width: 0;
}`,
  });

  /* component: lds-nav (draft) */
  LDS.define({
    tag: "lds-nav",
    className: "LdsNav",
    formAssociated: false,
    observed: ["collapsed"],
    defaults: {},
    template: `<aside class="root" part="root">
  <div class="brand" part="brand"><slot name="brand"></slot></div>
  <nav class="body" part="body"><slot></slot></nav>
  <div class="footer" part="footer"><slot name="footer"></slot></div>
</aside>`,
    styles: `/* App sidebar shell. Item and group chrome live in their own components;
 * --lds-nav-* custom properties inherit into their shadows for collapsed mode. */

:host {
  --lds-nav-bg: var(--lds-color-bg-subtle);
  --lds-nav-fg: var(--lds-color-fg-default);
  --lds-nav-border: var(--lds-color-border-subtle);
  --lds-nav-width: 15rem;
  --lds-nav-padding: var(--lds-space-3);
  --lds-nav-gap: var(--lds-space-1);
  --lds-nav-item-label-display: inline;
  --lds-nav-group-label-display: inline;
  --lds-nav-group-chevron-display: inline-flex;

  display: flex;
  flex-direction: column;
  width: var(--lds-nav-width);
  min-height: 100%;
  color: var(--lds-nav-fg);
  background: var(--lds-nav-bg);
  border-inline-end: 1px solid var(--lds-nav-border);
  transition:
    width var(--lds-transition-surface),
    background var(--lds-transition-surface),
    border-color var(--lds-transition-surface);
}

:host([hidden]) {
  display: none;
}

:host([collapsed]) {
  --lds-nav-width: calc(var(--lds-control-height-md) + var(--lds-space-6));
  --lds-nav-item-label-display: none;
  --lds-nav-group-trigger-display: none;
  --lds-nav-group-label-display: none;
  --lds-nav-group-chevron-display: none;
  --lds-nav-group-closed-display: flex;
  --lds-nav-group-closed-pad: 0;
}

.root {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: var(--lds-space-3);
  min-height: 0;
  padding: var(--lds-nav-padding);
}

.brand {
  display: flex;
  flex: none;
  align-items: center;
  min-width: 0;
  min-height: var(--lds-control-height-md);
  padding-inline: var(--lds-space-2);
}

.brand[data-empty] {
  display: none;
}

.body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: var(--lds-nav-gap);
  min-height: 0;
  overflow: auto;
}

.footer {
  display: flex;
  flex: none;
  flex-direction: column;
  gap: var(--lds-nav-gap);
  margin-block-start: auto;
  padding-block-start: var(--lds-space-3);
  border-block-start: 1px solid var(--lds-nav-border);
}

.footer[data-empty] {
  display: none;
}

::slotted([slot="brand"]) {
  font-family: var(--lds-font-family-display);
  font-size: var(--lds-font-size-md);
  font-weight: var(--lds-font-weight-semibold);
  line-height: var(--lds-font-leading-tight);
  color: var(--lds-nav-fg);
  text-decoration: none;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:host([collapsed]) .brand {
  justify-content: center;
  padding-inline: 0;
}

:host([collapsed]) ::slotted([slot="brand"]) {
  font-size: var(--lds-font-size-sm);
}`,
  });

  /* component: lds-nav-group (draft) */
  LDS.define({
    tag: "lds-nav-group",
    className: "LdsNavGroup",
    formAssociated: false,
    observed: ["label", "open", "href", "current"],
    defaults: {},
    template: `<div class="group" part="group">
  <button type="button" class="trigger" part="trigger" aria-expanded="false">
    <span class="trigger__label" part="label"></span>
    <span class="trigger__chevron" part="chevron" aria-hidden="true"></span>
  </button>
  <div class="header" part="header" hidden>
    <a class="link" part="link">
      <span class="link__start" part="start"><slot name="start"></slot></span>
      <span class="link__label"></span>
      <span class="link__end" part="end"><slot name="end"></slot></span>
    </a>
    <button type="button" class="toggle" part="toggle" aria-expanded="false">
      <span class="trigger__chevron" aria-hidden="true"></span>
    </button>
  </div>
  <div class="items" part="items" role="group">
    <slot></slot>
  </div>
</div>`,
    styles: `/* Collapsible section inside <lds-nav>. Open state is the \`open\` attribute.
 * A collapsed parent nav sets --lds-nav-group-* vars so the rail stays usable.
 *
 * Without href the header is a caps section trigger. With href it is a
 * destination styled like lds-nav-item, with its own disclosure button. */

:host {
  display: block;
}

:host([hidden]) {
  display: none;
}

.group {
  display: flex;
  flex-direction: column;
  gap: var(--lds-space-1);
}

.trigger {
  display: var(--lds-nav-group-trigger-display, flex);
  align-items: center;
  justify-content: space-between;
  gap: var(--lds-control-gap-md);

  width: 100%;
  min-height: var(--lds-control-height-sm);
  padding-inline: var(--lds-control-padding-x-sm);

  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-font-size-2xs);
  font-weight: var(--lds-font-weight-semibold);
  line-height: 1;
  letter-spacing: var(--lds-font-tracking-caps);
  text-transform: uppercase;
  color: var(--lds-color-fg-subtle);
  text-align: start;

  background: transparent;
  border: none;
  border-radius: var(--lds-control-radius-sm);
  cursor: pointer;
  appearance: none;
  transition:
    background var(--lds-transition-control),
    color var(--lds-transition-control);
}

.trigger[hidden] {
  display: none;
}

.trigger:hover {
  color: var(--lds-color-fg-muted);
  background: var(--lds-color-bg-muted);
}

.trigger:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.trigger__label {
  display: var(--lds-nav-group-label-display, inline);
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.trigger__chevron {
  display: var(--lds-nav-group-chevron-display, inline-flex);
  flex: none;
  width: 0.5em;
  height: 0.5em;
  border-inline-end: 2px solid currentColor;
  border-block-end: 2px solid currentColor;
  rotate: -45deg;
  translate: 0 -0.1em;
  transition: rotate var(--lds-transition-control);
}

:host([open]) .trigger__chevron {
  rotate: 45deg;
  translate: 0 0.05em;
}

.items {
  display: flex;
  flex-direction: column;
  gap: var(--lds-space-1);
  padding-inline-start: var(--lds-space-2);
}

:host(:not([open])) .items {
  display: var(--lds-nav-group-closed-display, none);
  padding-inline-start: var(--lds-nav-group-closed-pad, var(--lds-space-2));
}

/* --- Link header ------------------------------------------------------- */

.header {
  display: flex;
  align-items: center;
  gap: var(--lds-space-1);
  border-radius: var(--lds-control-radius-md);
  transition: background var(--lds-transition-control);
}

.header[hidden] {
  display: none;
}

.header:hover {
  background: var(--lds-color-bg-muted);
}

:host([current]) .header {
  background: var(--lds-color-accent-subtle);
}

.link {
  display: flex;
  flex: 1 1 auto;
  align-items: center;
  gap: var(--lds-control-gap-md);
  min-width: 0;
  min-height: var(--lds-control-height-md);
  padding-inline: var(--lds-control-padding-x-sm);
  border-radius: var(--lds-control-radius-md);
  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-font-size-sm);
  font-weight: var(--lds-font-weight-medium);
  line-height: 1;
  color: var(--lds-color-fg-muted);
  text-decoration: none;
}

.header:hover .link {
  color: var(--lds-color-fg-default);
}

:host([current]) .link {
  color: var(--lds-color-accent-fg);
}

.link:focus-visible,
.toggle:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.link__start,
.link__end {
  display: inline-flex;
  flex: none;
  align-items: center;
  line-height: 1;
}

.link__end {
  display: var(--lds-nav-group-label-display, inline-flex);
  color: var(--lds-color-fg-subtle);
  font-size: var(--lds-font-size-xs);
}

.link__start[data-empty],
.link__end[data-empty] {
  display: none;
}

.link__label {
  display: var(--lds-nav-group-label-display, inline);
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  line-height: var(--lds-font-leading-snug);
}

.toggle {
  display: var(--lds-nav-group-chevron-display, inline-flex);
  flex: none;
  align-items: center;
  justify-content: center;
  width: var(--lds-control-height-sm);
  height: var(--lds-control-height-sm);
  margin-inline-end: var(--lds-space-1);
  padding: 0;
  border: 0;
  border-radius: var(--lds-control-radius-sm);
  background: transparent;
  color: var(--lds-color-fg-subtle);
  cursor: pointer;
}

.toggle:hover {
  background: var(--lds-color-bg-subtle);
  color: var(--lds-color-fg-default);
}

/* Items under a destination header indent past its start content. */
:host([href]) .items {
  padding-inline-start: var(--lds-space-4);
}`,
  });

  /* component: lds-nav-item (draft) */
  LDS.define({
    tag: "lds-nav-item",
    className: "LdsNavItem",
    formAssociated: false,
    observed: ["href", "target", "active", "disabled", "label"],
    defaults: {},
    template: `<a class="item" part="item">
  <span class="item__start" part="start"><slot name="start"></slot></span>
  <span class="item__label" part="label"><slot></slot></span>
  <span class="item__end" part="end"><slot name="end"></slot></span>
</a>`,
    styles: `/* Single sidebar destination. Label visibility follows --lds-nav-item-label-display
 * inherited from a collapsed <lds-nav>. */

:host {
  --lds-nav-item-bg: transparent;
  --lds-nav-item-bg-hover: var(--lds-color-bg-muted);
  --lds-nav-item-fg: var(--lds-color-fg-muted);
  --lds-nav-item-fg-active: var(--lds-color-accent-fg);
  --lds-nav-item-bg-active: var(--lds-color-accent-subtle);

  display: block;
}

:host([hidden]) {
  display: none;
}

:host([active]) {
  --lds-nav-item-bg: var(--lds-nav-item-bg-active);
  --lds-nav-item-fg: var(--lds-nav-item-fg-active);
  --lds-nav-item-bg-hover: var(--lds-color-accent-muted);
}

.item {
  display: flex;
  align-items: center;
  gap: var(--lds-control-gap-md);

  min-height: var(--lds-control-height-md);
  padding-inline: var(--lds-control-padding-x-sm);

  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-font-size-sm);
  font-weight: var(--lds-font-weight-medium);
  line-height: 1;
  color: var(--lds-nav-item-fg);
  text-decoration: none;

  background: var(--lds-nav-item-bg);
  border-radius: var(--lds-control-radius-md);
  transition:
    background var(--lds-transition-control),
    color var(--lds-transition-control);
}

.item:hover {
  background: var(--lds-nav-item-bg-hover);
  color: var(--lds-color-fg-default);
}

:host([active]) .item:hover {
  color: var(--lds-nav-item-fg-active);
}

.item:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.item__start,
.item__end {
  display: inline-flex;
  flex: none;
  align-items: center;
  line-height: 1;
}

.item__start[data-empty],
.item__end[data-empty] {
  display: none;
}

.item__label {
  display: var(--lds-nav-item-label-display, inline);
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item__label[data-empty] {
  display: none;
}

:host([disabled]) .item {
  cursor: not-allowed;
  opacity: 0.55;
  pointer-events: none;
}

::slotted([slot="start"]),
::slotted([slot="end"]) {
  display: inline-flex;
  flex: none;
  align-items: center;
  line-height: 1;
}`,
  });

  /* component: lds-navbar (draft) */
  LDS.define({
    tag: "lds-navbar",
    className: "LdsNavbar",
    formAssociated: false,
    observed: ["sticky", "compact", "open", "label"],
    defaults: {},
    template: `<header class="bar" part="bar">
  <div class="brand" part="brand"><slot name="brand"></slot></div>
  <button type="button" class="toggle" part="toggle" aria-expanded="false" aria-controls="links">
    <span class="toggle__icon" aria-hidden="true"></span>
  </button>
  <nav class="links" part="links" id="links"><slot></slot></nav>
  <div class="actions" part="actions"><slot name="actions"></slot></div>
</header>`,
    styles: `/* Top-bar chrome. Compact mode collapses the link row into a toggle panel
 * under the bar — not a drawer. Variants are custom properties on :host. */

:host {
  --lds-navbar-bg: var(--lds-color-bg-raised);
  --lds-navbar-fg: var(--lds-color-fg-default);
  --lds-navbar-border: var(--lds-color-border-subtle);
  --lds-navbar-compact-at: var(--lds-breakpoint-md);
  --lds-navbar-padding-x: var(--lds-space-4);
  --lds-navbar-gap: var(--lds-space-3);

  display: block;
  color: var(--lds-navbar-fg);
}

:host([hidden]) {
  display: none;
}

:host([sticky]) {
  position: sticky;
  top: 0;
  z-index: var(--lds-z-sticky);
}

/* --- Bar --------------------------------------------------------------- */

.bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--lds-navbar-gap);

  min-height: var(--lds-control-height-lg);
  padding-block: var(--lds-space-2);
  padding-inline: var(--lds-navbar-padding-x);

  background: var(--lds-navbar-bg);
  border-bottom: 1px solid var(--lds-navbar-border);
  transition:
    background var(--lds-transition-surface),
    border-color var(--lds-transition-surface);
}

.brand {
  display: flex;
  flex: none;
  align-items: center;
  gap: var(--lds-space-2);
  min-width: 0;
}

.links {
  display: flex;
  flex: 1;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--lds-space-1);
  min-width: 0;
}

.actions {
  display: flex;
  flex: none;
  align-items: center;
  gap: var(--lds-space-2);
  margin-inline-start: auto;
}

/* --- Toggle ------------------------------------------------------------ */

.toggle {
  display: none;
  flex: none;
  align-items: center;
  justify-content: center;

  width: var(--lds-control-height-md);
  height: var(--lds-control-height-md);
  padding: 0;

  color: var(--lds-color-fg-muted);
  background: transparent;
  border: var(--lds-control-border-width-default) solid transparent;
  border-radius: var(--lds-control-radius-md);

  cursor: pointer;
  appearance: none;
  transition:
    background var(--lds-transition-control),
    color var(--lds-transition-control),
    border-color var(--lds-transition-control);
}

.toggle:hover {
  color: var(--lds-color-fg-default);
  background: var(--lds-color-bg-subtle);
}

.toggle:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.toggle__icon,
.toggle__icon::before,
.toggle__icon::after {
  display: block;
  width: 1.125em;
  height: 2px;
  background: currentColor;
  border-radius: var(--lds-radius-full);
}

.toggle__icon {
  position: relative;
}

.toggle__icon::before,
.toggle__icon::after {
  content: "";
  position: absolute;
  inset-inline-start: 0;
}

.toggle__icon::before {
  translate: 0 -0.35em;
}

.toggle__icon::after {
  translate: 0 0.35em;
}

/* --- Compact ----------------------------------------------------------- */

:host([compact]) .toggle,
:host([data-compact]) .toggle {
  display: inline-flex;
  order: 3;
}

:host([compact]) .actions,
:host([data-compact]) .actions {
  order: 2;
  margin-inline-start: auto;
}

:host([compact]) .links,
:host([data-compact]) .links {
  display: none;
  order: 4;
  flex: none;
  flex-basis: 100%;
  flex-direction: column;
  align-items: stretch;
  gap: var(--lds-space-1);
  padding-block: var(--lds-space-2) var(--lds-space-3);
  border-top: 1px solid var(--lds-navbar-border);
}

:host([compact][open]) .links,
:host([data-compact][open]) .links {
  display: flex;
}

:host([compact][open]) .toggle,
:host([data-compact][open]) .toggle {
  color: var(--lds-color-fg-default);
  background: var(--lds-color-bg-muted);
}

/* --- Slotted content --------------------------------------------------- */

::slotted([slot="brand"]) {
  font-family: var(--lds-font-family-display);
  font-size: var(--lds-font-size-lg);
  font-weight: var(--lds-font-weight-semibold);
  line-height: var(--lds-font-leading-tight);
  color: var(--lds-navbar-fg);
  text-decoration: none;
}

::slotted(a:not([slot])) {
  display: inline-flex;
  align-items: center;
  min-height: var(--lds-control-height-md);
  padding-inline: var(--lds-control-padding-x-sm);

  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-font-size-sm);
  font-weight: var(--lds-font-weight-medium);
  line-height: 1;
  color: var(--lds-color-fg-muted);
  text-decoration: none;
  border-radius: var(--lds-control-radius-md);
  transition:
    background var(--lds-transition-control),
    color var(--lds-transition-control);
}

::slotted(a:not([slot]):hover) {
  color: var(--lds-color-fg-default);
  background: var(--lds-color-bg-subtle);
}

::slotted(a:not([slot]):focus-visible) {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

::slotted(a[aria-current="page"]:not([slot])) {
  color: var(--lds-color-accent-fg);
  background: var(--lds-color-accent-subtle);
}

:host([compact]) ::slotted(a:not([slot])),
:host([data-compact]) ::slotted(a:not([slot])) {
  width: 100%;
  justify-content: flex-start;
  min-height: var(--lds-control-height-lg);
  padding-inline: var(--lds-control-padding-x-md);
}`,
  });

  /* component: lds-code (draft) */
  LDS.define({
    tag: "lds-code",
    className: "LdsCode",
    formAssociated: false,
    observed: ["language", "copyable"],
    defaults: {},
    template: `<div class="root" part="root">
  <pre class="pre" part="pre"><code class="code" part="code"><slot></slot></code></pre>
  <button class="copy" part="copy" type="button" aria-label="Copy" hidden>
    <lds-icon name="copy" size="sm"></lds-icon>
  </button>
</div>`,
    styles: `/* Code / payload block with optional copy control. */

:host {
  --lds-code-bg: var(--lds-color-bg-subtle);
  --lds-code-fg: var(--lds-color-fg-default);
  --lds-code-border: var(--lds-color-border-default);
  --lds-code-radius: var(--lds-radius-md);
  --lds-code-padding: var(--lds-space-4);
  --lds-code-font-size: var(--lds-font-size-sm);

  display: block;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

.root {
  position: relative;
  max-width: 100%;
}

.pre {
  box-sizing: border-box;
  margin: 0;
  padding: var(--lds-code-padding);
  overflow: auto;
  border: var(--lds-control-border-width-default) solid var(--lds-code-border);
  border-radius: var(--lds-code-radius);
  background: var(--lds-code-bg);
  color: var(--lds-code-fg);
  font-family: var(--lds-font-family-mono);
  font-size: var(--lds-code-font-size);
  line-height: var(--lds-font-leading-relaxed);
}

.code {
  font: inherit;
}

.copy {
  position: absolute;
  inset-block-start: var(--lds-space-2);
  inset-inline-end: var(--lds-space-2);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: var(--lds-control-height-sm);
  height: var(--lds-control-height-sm);
  margin: 0;
  padding: 0;
  border: var(--lds-control-border-width-default) solid var(--lds-code-border);
  border-radius: var(--lds-control-radius-sm);
  background: var(--lds-color-bg-raised);
  color: var(--lds-color-fg-muted);
  cursor: pointer;
}

.copy[hidden] {
  display: none;
}

.copy:hover {
  color: var(--lds-color-fg-default);
}

.copy:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

:host(:not([copyable="false"])) .pre {
  padding-inline-end: calc(var(--lds-code-padding) + var(--lds-control-height-sm));
}`,
  });

  /* component: lds-empty-state (draft) */
  LDS.define({
    tag: "lds-empty-state",
    className: "LdsEmptyState",
    formAssociated: false,
    observed: ["size", "align"],
    defaults: {"size": "md", "align": "center"},
    template: `<div class="root" part="root">
  <div class="icon" part="icon"><slot name="icon"></slot></div>
  <div class="title" part="title"><slot name="title"></slot></div>
  <div class="body" part="body"><slot></slot></div>
  <div class="actions" part="actions"><slot name="actions"></slot></div>
</div>`,
    styles: `/* Nothing to show yet: an icon, a title, one or two sentences and the action
 * that fills the space. */

:host {
  --lds-empty-state-padding: var(--lds-space-12) var(--lds-space-6);
  --lds-empty-state-gap: var(--lds-space-3);
  --lds-empty-state-max-width: 28rem;

  display: block;
}

:host([hidden]) {
  display: none;
}

:host([size="sm"]) {
  --lds-empty-state-padding: var(--lds-space-6) var(--lds-space-4);
  --lds-empty-state-gap: var(--lds-space-2);
}

.root {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--lds-empty-state-gap);
  max-width: var(--lds-empty-state-max-width);
  margin-inline: auto;
  padding: var(--lds-empty-state-padding);
  text-align: center;
}

:host([align="start"]) .root {
  align-items: flex-start;
  margin-inline: 0;
  text-align: start;
}

.icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: var(--lds-control-height-lg);
  height: var(--lds-control-height-lg);
  border-radius: var(--lds-radius-full);
  background: var(--lds-color-bg-muted);
  color: var(--lds-color-fg-muted);
}

:host([size="sm"]) .icon {
  width: var(--lds-control-height-sm);
  height: var(--lds-control-height-sm);
}

.title {
  font-family: var(--lds-font-family-display);
  font-size: var(--lds-font-size-lg);
  font-weight: var(--lds-font-weight-semibold);
  line-height: var(--lds-font-leading-tight);
  color: var(--lds-color-fg-default);
  overflow-wrap: anywhere;
}

:host([size="sm"]) .title {
  font-size: var(--lds-font-size-md);
}

/* Headings passed in the title slot keep the slot's typography. */
::slotted([slot="title"]) {
  margin: 0;
  font: inherit;
}

.body {
  color: var(--lds-color-fg-muted);
  font-size: var(--lds-font-size-sm);
  line-height: var(--lds-font-leading-normal);
}

.actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: var(--lds-space-2);
  margin-block-start: var(--lds-space-1);
}

:host([align="start"]) .actions {
  justify-content: flex-start;
}

.icon[data-empty],
.title[data-empty],
.body[data-empty],
.actions[data-empty] {
  display: none;
}`,
  });

  /* component: lds-pagination (draft) */
  LDS.define({
    tag: "lds-pagination",
    className: "LdsPagination",
    formAssociated: false,
    observed: ["total", "page", "page-size", "sizes", "hide-sizes", "disabled", "label"],
    defaults: {"total": "0", "page": "1", "page-size": "25", "sizes": "10,25,50,100"},
    template: `<nav class="root" part="root">
  <p class="summary" part="summary"></p>
  <div class="controls" part="controls">
    <div class="sizes" part="sizes">
      <label class="sizes__label">
        <span class="sizes__text">Rows</span>
        <select class="sizes__select" part="select"></select>
      </label>
    </div>
    <div class="pages" part="pages" role="group" aria-label="Pages"></div>
  </div>
</nav>`,
    styles: `/* Pagination — summary, page-size select, and page buttons. */

:host {
  --lds-pagination-gap: var(--lds-space-4);
  --lds-pagination-control-height: var(--lds-control-height-sm);
  --lds-pagination-control-radius: var(--lds-control-radius-sm);
  --lds-pagination-font-size: var(--lds-control-font-size-sm);
  --lds-pagination-fg: var(--lds-color-fg-default);
  --lds-pagination-muted: var(--lds-color-fg-muted);
  --lds-pagination-border: var(--lds-color-border-default);
  --lds-pagination-bg: var(--lds-color-bg-canvas);
  --lds-pagination-bg-hover: var(--lds-color-bg-subtle);
  --lds-pagination-bg-active: var(--lds-color-bg-muted);
  --lds-pagination-current-bg: var(--lds-color-accent-subtle);
  --lds-pagination-current-fg: var(--lds-color-accent-fg);
  --lds-pagination-current-border: var(--lds-color-accent-border);

  display: block;
  max-width: 100%;
  color: var(--lds-pagination-fg);
  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-pagination-font-size);
}

:host([hidden]) {
  display: none;
}

.root {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--lds-pagination-gap);
}

.summary {
  margin: 0;
  color: var(--lds-pagination-muted);
}

.summary[data-empty] {
  display: none;
}

.controls {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--lds-space-3);
}

.sizes[data-hidden] {
  display: none;
}

.sizes__label {
  display: inline-flex;
  align-items: center;
  gap: var(--lds-space-2);
  color: var(--lds-pagination-muted);
}

.sizes__select {
  box-sizing: border-box;
  height: var(--lds-pagination-control-height);
  padding-inline: var(--lds-control-padding-x-sm);
  border: var(--lds-control-border-width-default) solid var(--lds-pagination-border);
  border-radius: var(--lds-pagination-control-radius);
  background: var(--lds-pagination-bg);
  color: var(--lds-pagination-fg);
  font: inherit;
  cursor: pointer;
}

.sizes__select:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.sizes__select:disabled {
  color: var(--lds-color-fg-disabled);
  cursor: not-allowed;
  opacity: 0.7;
}

.pages {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--lds-space-1);
}

.btn {
  display: inline-flex;
  box-sizing: border-box;
  align-items: center;
  justify-content: center;
  min-width: var(--lds-pagination-control-height);
  height: var(--lds-pagination-control-height);
  margin: 0;
  padding-inline: var(--lds-space-2);
  border: var(--lds-control-border-width-default) solid var(--lds-pagination-border);
  border-radius: var(--lds-pagination-control-radius);
  background: var(--lds-pagination-bg);
  color: var(--lds-pagination-fg);
  font: inherit;
  font-weight: var(--lds-font-weight-medium);
  line-height: 1;
  cursor: pointer;
}

.btn:hover:not(:disabled) {
  background: var(--lds-pagination-bg-hover);
}

.btn:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.btn:disabled {
  color: var(--lds-color-fg-disabled);
  cursor: not-allowed;
  opacity: 0.7;
}

.btn[aria-current="page"] {
  border-color: var(--lds-pagination-current-border);
  background: var(--lds-pagination-current-bg);
  color: var(--lds-pagination-current-fg);
}

.btn--icon {
  padding-inline: 0;
}

.ellipsis {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: var(--lds-pagination-control-height);
  height: var(--lds-pagination-control-height);
  color: var(--lds-pagination-muted);
  user-select: none;
}`,
  });

  /* component: lds-skeleton (draft) */
  LDS.define({
    tag: "lds-skeleton",
    className: "LdsSkeleton",
    formAssociated: false,
    observed: ["shape", "lines"],
    defaults: {"shape": "text"},
    template: `<span class="lines" part="lines"></span>`,
    styles: `/* Loading placeholder. Text renders one bar per line; rect and circle are a
 * single block sized with --lds-skeleton-width / --lds-skeleton-height. */

:host {
  --lds-skeleton-bg: var(--lds-color-bg-muted);
  --lds-skeleton-shine: var(--lds-color-bg-subtle);
  --lds-skeleton-width: 100%;
  --lds-skeleton-height: 1em;
  --lds-skeleton-radius: var(--lds-radius-md);

  display: block;
  width: var(--lds-skeleton-width);
}

:host([hidden]) {
  display: none;
}

:host([shape="rect"]) {
  --lds-skeleton-height: var(--lds-space-24);
}

:host([shape="circle"]) {
  --lds-skeleton-width: var(--lds-control-height-md);
  --lds-skeleton-height: var(--lds-skeleton-width);
  --lds-skeleton-radius: var(--lds-radius-full);
}

.lines {
  display: flex;
  flex-direction: column;
  gap: var(--lds-space-2);
}

.bar {
  display: block;
  height: var(--lds-skeleton-height);
  border-radius: var(--lds-skeleton-radius);
  background: linear-gradient(90deg, var(--lds-skeleton-bg) 0%, var(--lds-skeleton-shine) 50%, var(--lds-skeleton-bg) 100%);
  background-size: 200% 100%;
  /* A slow sweep: four times the slowest UI duration. */
  animation: lds-skeleton-shine calc(var(--lds-duration-slower) * 4) linear infinite;
}

/* Text lines sit at the height of a line of text, not the font size. */
:host(:not([shape])) .bar,
:host([shape="text"]) .bar {
  height: calc(var(--lds-skeleton-height) * 0.8);
  margin-block: calc(var(--lds-skeleton-height) * 0.1);
}

/* The last line of a paragraph is shorter. */
.bar[data-last] {
  width: 60%;
}

@keyframes lds-skeleton-shine {
  from {
    background-position: 100% 0;
  }
  to {
    background-position: -100% 0;
  }
}

/* The document's reduced-motion reset cannot reach into a shadow root. */
@media (prefers-reduced-motion: reduce) {
  .bar {
    animation: none;
  }
}`,
  });

  /* component: lds-table (draft) */
  LDS.define({
    tag: "lds-table",
    className: "LdsTable",
    formAssociated: false,
    observed: ["density", "sticky", "sortable", "striped"],
    defaults: {"density": "comfortable"},
    template: `<div class="frame" part="frame">
  <slot></slot>
</div>`,
    styles: `/* Scroll frame and density tokens. Cell geometry lives in runtime/table.css
 * because ::slotted() cannot reach nested th/td. */

:host {
  --lds-table-font-size: var(--lds-font-size-sm);
  --lds-table-cell-padding-y: var(--lds-space-3);
  --lds-table-cell-padding-x: var(--lds-space-4);
  --lds-table-fg: var(--lds-color-fg-default);
  --lds-table-bg: var(--lds-color-bg-canvas);
  --lds-table-border: var(--lds-color-border-default);
  --lds-table-header-fg: var(--lds-color-fg-muted);
  --lds-table-header-bg: var(--lds-color-bg-muted);
  --lds-table-row-hover-bg: var(--lds-color-bg-muted);
  --lds-table-row-stripe-bg: var(--lds-color-bg-subtle);
  --lds-table-max-height: none;

  display: block;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

:host([density="compact"]) {
  --lds-table-cell-padding-y: var(--lds-space-2);
  --lds-table-cell-padding-x: var(--lds-space-3);
  --lds-table-font-size: var(--lds-font-size-xs);
}

.frame {
  box-sizing: border-box;
  max-width: 100%;
  max-height: var(--lds-table-max-height);
  overflow: auto;
  border: var(--lds-control-border-width-default) solid var(--lds-table-border);
  border-radius: var(--lds-radius-md);
  background: var(--lds-table-bg);
}

::slotted(table) {
  margin: 0;
}`,
  });

  /* component: lds-timeline (draft) */
  LDS.define({
    tag: "lds-timeline",
    className: "LdsTimeline",
    formAssociated: false,
    observed: [],
    defaults: {},
    template: `<div class="list" part="list">
  <slot></slot>
</div>`,
    styles: `:host {
  display: block;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

.list {
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
}`,
  });

  /* component: lds-timeline-item (draft) */
  LDS.define({
    tag: "lds-timeline-item",
    className: "LdsTimelineItem",
    formAssociated: false,
    observed: ["time", "intent"],
    defaults: {"intent": "info"},
    template: `<div class="item" part="item">
  <div class="rail" part="rail" aria-hidden="true">
    <span class="marker" part="marker"></span>
  </div>
  <div class="content" part="content">
    <div class="meta" part="meta">
      <span class="time" part="time"></span>
      <span class="title" part="title"><slot name="title"></slot></span>
    </div>
    <div class="body" part="body"><slot></slot></div>
  </div>
</div>`,
    styles: `:host {
  --lds-timeline-marker: var(--lds-color-accent-default);
  --lds-timeline-rail: var(--lds-color-border-default);

  display: block;
}

:host([hidden]) {
  display: none;
}

:host([intent="success"]) {
  --lds-timeline-marker: var(--lds-color-success-default);
}

:host([intent="warning"]) {
  --lds-timeline-marker: var(--lds-color-warning-default);
}

:host([intent="danger"]) {
  --lds-timeline-marker: var(--lds-color-danger-default);
}

:host([intent="neutral"]) {
  --lds-timeline-marker: var(--lds-color-fg-subtle);
}

.item {
  display: grid;
  grid-template-columns: var(--lds-space-6) minmax(0, 1fr);
  gap: var(--lds-space-3);
  min-height: var(--lds-space-10);
}

.rail {
  position: relative;
  display: flex;
  justify-content: center;
}

.rail::before {
  content: "";
  position: absolute;
  inset-block: 0;
  width: var(--lds-control-border-width-default);
  background: var(--lds-timeline-rail);
}

.marker {
  position: relative;
  z-index: var(--lds-z-raised);
  box-sizing: border-box;
  width: var(--lds-space-3);
  height: var(--lds-space-3);
  margin-block-start: var(--lds-space-1);
  border: var(--lds-control-border-width-default) solid var(--lds-color-bg-canvas);
  border-radius: var(--lds-radius-full);
  background: var(--lds-timeline-marker);
  box-shadow: 0 0 0 var(--lds-control-border-width-default) var(--lds-timeline-marker);
}

:host(:first-child) .rail::before {
  inset-block-start: var(--lds-space-3);
}

:host(:last-child) .rail::before {
  inset-block-end: calc(100% - var(--lds-space-4));
}

.content {
  display: flex;
  flex-direction: column;
  gap: var(--lds-space-1);
  padding-bottom: var(--lds-space-5);
}

:host(:last-child) .content {
  padding-bottom: 0;
}

.meta {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: var(--lds-space-2);
}

.time {
  color: var(--lds-color-fg-muted);
  font-size: var(--lds-font-size-xs);
  font-variant-numeric: tabular-nums;
}

.time:empty {
  display: none;
}

.title {
  font-size: var(--lds-font-size-sm);
  font-weight: var(--lds-font-weight-semibold);
  line-height: var(--lds-font-leading-snug);
}

.body {
  color: var(--lds-color-fg-muted);
  font-size: var(--lds-font-size-sm);
  line-height: var(--lds-font-leading-normal);
}`,
  });

  /* component: lds-alert (draft) */
  LDS.define({
    tag: "lds-alert",
    className: "LdsAlert",
    formAssociated: false,
    observed: ["intent", "dismissible"],
    defaults: {"intent": "info"},
    template: `<div class="root" part="root">
  <span class="icon" part="icon" aria-hidden="true">
    <slot name="icon"></slot>
    <lds-icon class="icon__fallback" data-intent="info" name="info"></lds-icon>
    <lds-icon class="icon__fallback" data-intent="success" name="circle-check" hidden></lds-icon>
    <lds-icon class="icon__fallback" data-intent="warning" name="alert-triangle" hidden></lds-icon>
    <lds-icon class="icon__fallback" data-intent="danger" name="alert-circle" hidden></lds-icon>
  </span>
  <div class="main">
    <div class="title" part="title"><slot name="title"></slot></div>
    <div class="description" part="description"><slot name="description"></slot><slot></slot></div>
    <div class="actions" part="actions"><slot name="actions"></slot></div>
  </div>
  <button type="button" class="close" part="close" aria-label="Dismiss" hidden>
    <lds-icon name="x"></lds-icon>
  </button>
</div>`,
    styles: `/* Inline persistent message. Distinct from toast (transient) and banner (page-level). */

:host {
  --lds-alert-bg: var(--lds-color-accent-subtle);
  --lds-alert-fg: var(--lds-color-accent-fg);
  --lds-alert-border: var(--lds-color-accent-border);
  --lds-alert-icon: var(--lds-color-accent-default);
  --lds-alert-radius: var(--lds-radius-md);

  display: block;
  max-width: 100%;
  color: var(--lds-alert-fg);
}

:host([hidden]) {
  display: none;
}

:host([intent="success"]) {
  --lds-alert-bg: var(--lds-color-success-subtle);
  --lds-alert-fg: var(--lds-color-success-fg);
  --lds-alert-border: var(--lds-color-success-border);
  --lds-alert-icon: var(--lds-color-success-default);
}

:host([intent="warning"]) {
  --lds-alert-bg: var(--lds-color-warning-subtle);
  --lds-alert-fg: var(--lds-color-warning-fg);
  --lds-alert-border: var(--lds-color-warning-border);
  --lds-alert-icon: var(--lds-color-warning-default);
}

:host([intent="danger"]) {
  --lds-alert-bg: var(--lds-color-danger-subtle);
  --lds-alert-fg: var(--lds-color-danger-fg);
  --lds-alert-border: var(--lds-color-danger-border);
  --lds-alert-icon: var(--lds-color-danger-default);
}

.root {
  display: flex;
  align-items: flex-start;
  gap: var(--lds-space-3);
  box-sizing: border-box;
  width: 100%;
  padding: var(--lds-space-4);
  background: var(--lds-alert-bg);
  border: var(--lds-control-border-width-default) solid var(--lds-alert-border);
  border-radius: var(--lds-alert-radius);
}

.icon {
  display: inline-flex;
  flex: none;
  align-items: center;
  margin-block-start: 0.1em;
  color: var(--lds-alert-icon);
}

.icon__fallback[hidden] {
  display: none;
}

.main {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  gap: var(--lds-space-1);
  min-width: 0;
}

.title {
  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-font-size-sm);
  font-weight: var(--lds-font-weight-semibold);
  line-height: var(--lds-font-leading-snug);
}

.title[data-empty] {
  display: none;
}

.description {
  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-font-size-sm);
  font-weight: var(--lds-font-weight-regular);
  line-height: var(--lds-font-leading-normal);
}

.description[data-empty] {
  display: none;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--lds-space-2);
  margin-block-start: var(--lds-space-2);
}

.actions[data-empty] {
  display: none;
}

.close {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;

  width: var(--lds-control-height-sm);
  height: var(--lds-control-height-sm);
  margin: calc(var(--lds-space-1) * -1);
  padding: 0;

  color: inherit;
  background: transparent;
  border: none;
  border-radius: var(--lds-control-radius-sm);
  cursor: pointer;
  appearance: none;
  opacity: 0.7;
  transition:
    background var(--lds-transition-control),
    opacity var(--lds-transition-control);
}

.close:hover {
  opacity: 1;
  background: var(--lds-color-bg-muted);
}

.close:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.close[hidden] {
  display: none;
}`,
  });

  /* component: lds-banner (draft) */
  LDS.define({
    tag: "lds-banner",
    className: "LdsBanner",
    formAssociated: false,
    observed: ["intent", "dismissible", "sticky"],
    defaults: {"intent": "info"},
    template: `<div class="root" part="root">
  <div class="inner" part="inner">
    <span class="icon" part="icon" aria-hidden="true">
      <slot name="icon"></slot>
      <lds-icon class="icon__fallback" data-intent="info" name="info"></lds-icon>
      <lds-icon class="icon__fallback" data-intent="success" name="circle-check" hidden></lds-icon>
      <lds-icon class="icon__fallback" data-intent="warning" name="alert-triangle" hidden></lds-icon>
      <lds-icon class="icon__fallback" data-intent="danger" name="alert-circle" hidden></lds-icon>
    </span>
    <div class="main">
      <div class="title" part="title"><slot name="title"></slot></div>
      <div class="body" part="body"><slot></slot></div>
    </div>
    <div class="action" part="action"><slot name="action"></slot></div>
    <button type="button" class="close" part="close" aria-label="Dismiss" hidden>
      <lds-icon name="x"></lds-icon>
    </button>
  </div>
</div>`,
    styles: `/* Page-level announcement. Full-bleed within its containing block. */

:host {
  --lds-banner-bg: var(--lds-color-accent-subtle);
  --lds-banner-fg: var(--lds-color-accent-fg);
  --lds-banner-border: var(--lds-color-accent-border);
  --lds-banner-icon: var(--lds-color-accent-default);

  display: block;
  width: 100%;
  color: var(--lds-banner-fg);
}

:host([hidden]) {
  display: none;
}

:host([intent="success"]) {
  --lds-banner-bg: var(--lds-color-success-subtle);
  --lds-banner-fg: var(--lds-color-success-fg);
  --lds-banner-border: var(--lds-color-success-border);
  --lds-banner-icon: var(--lds-color-success-default);
}

:host([intent="warning"]) {
  --lds-banner-bg: var(--lds-color-warning-subtle);
  --lds-banner-fg: var(--lds-color-warning-fg);
  --lds-banner-border: var(--lds-color-warning-border);
  --lds-banner-icon: var(--lds-color-warning-default);
}

:host([intent="danger"]) {
  --lds-banner-bg: var(--lds-color-danger-subtle);
  --lds-banner-fg: var(--lds-color-danger-fg);
  --lds-banner-border: var(--lds-color-danger-border);
  --lds-banner-icon: var(--lds-color-danger-default);
}

:host([sticky]) {
  position: sticky;
  inset-block-start: 0;
  z-index: var(--lds-z-sticky);
}

.root {
  box-sizing: border-box;
  width: 100%;
  background: var(--lds-banner-bg);
  border-block-end: var(--lds-control-border-width-default) solid var(--lds-banner-border);
}

.inner {
  display: flex;
  align-items: flex-start;
  gap: var(--lds-space-3);
  box-sizing: border-box;
  width: 100%;
  max-width: var(--lds-banner-max-width, none);
  margin-inline: auto;
  padding-block: var(--lds-space-3);
  padding-inline: var(--lds-space-4);
}

.icon {
  display: inline-flex;
  flex: none;
  align-items: center;
  margin-block-start: 0.1em;
  color: var(--lds-banner-icon);
}

.icon__fallback[hidden] {
  display: none;
}

.main {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  gap: var(--lds-space-1);
  min-width: 0;
}

.title {
  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-font-size-sm);
  font-weight: var(--lds-font-weight-semibold);
  line-height: var(--lds-font-leading-snug);
}

.title[data-empty] {
  display: none;
}

.body {
  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-font-size-sm);
  font-weight: var(--lds-font-weight-regular);
  line-height: var(--lds-font-leading-normal);
}

.body[data-empty] {
  display: none;
}

.action {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: var(--lds-space-2);
  align-self: center;
}

.action[data-empty] {
  display: none;
}

.close {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  align-self: flex-start;

  width: var(--lds-control-height-sm);
  height: var(--lds-control-height-sm);
  margin: calc(var(--lds-space-1) * -1);
  padding: 0;

  color: inherit;
  background: transparent;
  border: none;
  border-radius: var(--lds-control-radius-sm);
  cursor: pointer;
  appearance: none;
  opacity: 0.7;
  transition:
    background var(--lds-transition-control),
    opacity var(--lds-transition-control);
}

.close:hover {
  opacity: 1;
  background: var(--lds-color-bg-muted);
}

.close:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.close[hidden] {
  display: none;
}

@media (max-width: 40rem) {
  .inner {
    flex-wrap: wrap;
  }

  .action {
    width: 100%;
    padding-inline-start: calc(var(--lds-icon-size-md) + var(--lds-space-3));
  }

  .close {
    position: absolute;
    inset-block-start: var(--lds-space-2);
    inset-inline-end: var(--lds-space-2);
  }

  .root {
    position: relative;
  }
}`,
  });

  /* component: lds-progress (draft) */
  LDS.define({
    tag: "lds-progress",
    className: "LdsProgress",
    formAssociated: false,
    observed: ["variant", "value", "max", "indeterminate", "label"],
    defaults: {"variant": "linear", "value": "0", "max": "100"},
    template: `<div class="root" part="root">
  <div class="linear" part="linear" hidden>
    <div class="track" part="track">
      <div class="bar" part="bar"></div>
    </div>
  </div>
  <div class="circular" part="circular" hidden>
    <svg class="ring" part="ring" viewBox="0 0 32 32" aria-hidden="true">
      <circle class="ring__track" cx="16" cy="16" r="14" fill="none"></circle>
      <circle class="ring__bar" cx="16" cy="16" r="14" fill="none"></circle>
    </svg>
  </div>
  <span class="label" part="label" hidden></span>
</div>`,
    styles: `/* Determinate / indeterminate progress — linear or circular. */

:host {
  --lds-progress-track: var(--lds-color-bg-muted);
  --lds-progress-bar: var(--lds-color-accent-default);
  --lds-progress-size: var(--lds-space-8);
  --lds-progress-thickness: 3px;

  display: inline-flex;
  vertical-align: middle;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

:host([variant="linear"]),
:host(:not([variant])) {
  display: flex;
  width: 100%;
}

.root {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: var(--lds-space-2);
  min-width: 0;
}

:host([variant="circular"]) .root {
  flex: none;
  align-items: center;
}

.linear,
.circular {
  min-width: 0;
}

.linear[hidden],
.circular[hidden],
.label[hidden] {
  display: none;
}

.track {
  overflow: hidden;
  height: var(--lds-progress-thickness);
  border-radius: var(--lds-radius-full);
  background: var(--lds-progress-track);
}

.bar {
  height: 100%;
  width: 0%;
  border-radius: inherit;
  background: var(--lds-progress-bar);
  transition: width var(--lds-transition-surface);
}

:host([indeterminate]) .bar {
  width: 40%;
  animation: lds-progress-indeterminate 1.2s var(--lds-easing-standard) infinite;
}

.ring {
  width: var(--lds-progress-size);
  height: var(--lds-progress-size);
  transform: rotate(-90deg);
}

.ring__track,
.ring__bar {
  stroke-width: 3;
}

.ring__track {
  stroke: var(--lds-progress-track);
}

.ring__bar {
  stroke: var(--lds-progress-bar);
  stroke-linecap: round;
  stroke-dasharray: 88;
  stroke-dashoffset: 88;
  transition: stroke-dashoffset var(--lds-transition-surface);
}

:host([indeterminate]) .ring__bar {
  animation: lds-progress-spin 1s linear infinite;
  stroke-dashoffset: 66;
}

.label {
  color: var(--lds-color-fg-muted);
  font-size: var(--lds-font-size-xs);
}

@keyframes lds-progress-indeterminate {
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(350%);
  }
}

@keyframes lds-progress-spin {
  to {
    transform: rotate(270deg);
  }
}`,
  });

  /* component: lds-toast (draft) */
  LDS.define({
    tag: "lds-toast",
    className: "LdsToast",
    formAssociated: false,
    observed: ["intent", "open", "dismissible", "duration"],
    defaults: {"intent": "info", "duration": "5000"},
    template: `<div class="root" part="root">
  <div class="main">
    <div class="title" part="title"><slot name="title"></slot></div>
    <div class="body" part="body"><slot></slot></div>
    <div class="action" part="action"><slot name="action"></slot></div>
  </div>
  <button type="button" class="close" part="close" aria-label="Dismiss" hidden>
    <lds-icon name="x"></lds-icon>
  </button>
</div>`,
    styles: `/* Transient notification card. Intents are custom properties on :host. */

:host {
  --lds-toast-bg: var(--lds-color-accent-subtle);
  --lds-toast-fg: var(--lds-color-accent-fg);
  --lds-toast-border: var(--lds-color-accent-border);
  --lds-toast-width: 22rem;

  display: none;
  max-width: 100%;
  color: var(--lds-toast-fg);
}

:host([open]) {
  display: block;
}

:host([hidden]) {
  display: none;
}

:host([intent="success"]) {
  --lds-toast-bg: var(--lds-color-success-subtle);
  --lds-toast-fg: var(--lds-color-success-fg);
  --lds-toast-border: var(--lds-color-success-border);
}

:host([intent="warning"]) {
  --lds-toast-bg: var(--lds-color-warning-subtle);
  --lds-toast-fg: var(--lds-color-warning-fg);
  --lds-toast-border: var(--lds-color-warning-border);
}

:host([intent="danger"]) {
  --lds-toast-bg: var(--lds-color-danger-subtle);
  --lds-toast-fg: var(--lds-color-danger-fg);
  --lds-toast-border: var(--lds-color-danger-border);
}

.root {
  display: flex;
  align-items: flex-start;
  gap: var(--lds-space-3);

  box-sizing: border-box;
  width: var(--lds-toast-width);
  max-width: 100%;
  padding: var(--lds-space-4);

  background: var(--lds-toast-bg);
  border: 1px solid var(--lds-toast-border);
  border-radius: var(--lds-radius-md);
  box-shadow: var(--lds-shadow-md);
}

.main {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: var(--lds-space-1);
  min-width: 0;
}

.title {
  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-font-size-sm);
  font-weight: var(--lds-font-weight-semibold);
  line-height: var(--lds-font-leading-snug);
}

.title[data-empty] {
  display: none;
}

.body {
  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-font-size-sm);
  font-weight: var(--lds-font-weight-regular);
  line-height: var(--lds-font-leading-normal);
}

.body[data-empty] {
  display: none;
}

.action {
  margin-block-start: var(--lds-space-2);
}

.action[data-empty] {
  display: none;
}

.close {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;

  width: var(--lds-control-height-sm);
  height: var(--lds-control-height-sm);
  margin: calc(var(--lds-space-1) * -1);
  padding: 0;

  color: inherit;
  background: transparent;
  border: none;
  border-radius: var(--lds-control-radius-sm);
  cursor: pointer;
  appearance: none;
  opacity: 0.7;
  transition:
    background var(--lds-transition-control),
    opacity var(--lds-transition-control);
}

.close:hover {
  opacity: 1;
  background: var(--lds-color-bg-muted);
}

.close:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.close[hidden] {
  display: none;
}`,
  });

  /* component: lds-toast-region (draft) */
  LDS.define({
    tag: "lds-toast-region",
    className: "LdsToastRegion",
    formAssociated: false,
    observed: ["placement"],
    defaults: {"placement": "top-end"},
    template: `<div class="stack" part="stack">
  <slot></slot>
</div>`,
    styles: `/* Fixed toast stack. Placement is inset + alignment on :host. */

:host {
  --lds-toast-region-gap: var(--lds-space-3);
  --lds-toast-region-inset: var(--lds-space-4);

  position: fixed;
  z-index: var(--lds-z-toast);
  display: flex;
  flex-direction: column;
  gap: var(--lds-toast-region-gap);
  box-sizing: border-box;
  max-width: calc(100vw - (var(--lds-toast-region-inset) * 2));
  max-height: calc(100vh - (var(--lds-toast-region-inset) * 2));
  overflow: auto;
  pointer-events: none;
}

:host([hidden]) {
  display: none;
}

:host([placement="top-start"]) {
  inset-block-start: var(--lds-toast-region-inset);
  inset-inline-start: var(--lds-toast-region-inset);
  align-items: flex-start;
}

:host([placement="top-end"]),
:host(:not([placement])) {
  inset-block-start: var(--lds-toast-region-inset);
  inset-inline-end: var(--lds-toast-region-inset);
  align-items: flex-end;
}

:host([placement="bottom-start"]) {
  inset-block-end: var(--lds-toast-region-inset);
  inset-inline-start: var(--lds-toast-region-inset);
  align-items: flex-start;
}

:host([placement="bottom-end"]) {
  inset-block-end: var(--lds-toast-region-inset);
  inset-inline-end: var(--lds-toast-region-inset);
  align-items: flex-end;
}

.stack {
  display: flex;
  flex-direction: column;
  gap: var(--lds-toast-region-gap);
  align-items: inherit;
  min-width: 0;
}

::slotted(lds-toast) {
  pointer-events: auto;
}

:host(:not(:has(lds-toast))) {
  visibility: hidden;
}`,
  });

  /* component: lds-drawer (draft) */
  LDS.define({
    tag: "lds-drawer",
    className: "LdsDrawer",
    formAssociated: false,
    observed: ["open", "placement", "size", "non-modal", "dismissible"],
    defaults: {"placement": "end", "size": "md"},
    template: `<dialog class="dialog" part="dialog">
  <div class="chrome" part="chrome">
    <header class="header" part="header">
      <div class="header__title"><slot name="header"></slot></div>
      <slot name="actions"></slot>
      <button class="close" part="close" type="button" aria-label="Close">
        <lds-icon name="x" size="sm"></lds-icon>
      </button>
    </header>
    <div class="body" part="body"><slot></slot></div>
    <footer class="footer" part="footer"><slot name="footer"></slot></footer>
  </div>
</dialog>`,
    styles: `/* Side panel on native <dialog>. Modal by default (showModal: top layer,
 * backdrop, inert page); non-modal uses show() so the page stays usable. */

:host {
  --lds-drawer-width: 28rem;
  --lds-drawer-inset-top: 0;
  --lds-drawer-bg: var(--lds-color-bg-raised);
  --lds-drawer-fg: var(--lds-color-fg-default);
  --lds-drawer-shadow: var(--lds-overlay-shadow);

  display: contents;
}

:host([hidden]) {
  display: none;
}

:host([size="sm"]) {
  --lds-drawer-width: 20rem;
}

:host([size="lg"]) {
  --lds-drawer-width: 40rem;
}

.dialog {
  box-sizing: border-box;
  position: fixed;
  z-index: var(--lds-z-drawer);
  inset: var(--lds-drawer-inset-top) 0 0 auto;
  width: min(var(--lds-drawer-width), 100vw);
  max-width: none;
  height: auto;
  max-height: none;
  margin: 0;
  padding: 0;
  border: 0;
  border-inline-start: var(--lds-control-border-width-default) solid var(--lds-color-border-default);
  background: var(--lds-drawer-bg);
  color: var(--lds-drawer-fg);
  box-shadow: var(--lds-drawer-shadow);
}

:host([placement="start"]) .dialog {
  inset: var(--lds-drawer-inset-top) auto 0 0;
  border-inline-start: 0;
  border-inline-end: var(--lds-control-border-width-default) solid var(--lds-color-border-default);
}

.dialog[open] {
  animation: lds-drawer-in-end var(--lds-transition-surface);
}

:host([placement="start"]) .dialog[open] {
  animation-name: lds-drawer-in-start;
}

/* A non-modal panel sits beside the content it relates to; a heavy shadow
 * would read as blocking. */
:host([non-modal]) .dialog {
  box-shadow: var(--lds-shadow-md);
}

.dialog::backdrop {
  background: var(--lds-color-bg-overlay);
}

@keyframes lds-drawer-in-end {
  from {
    transform: translateX(100%);
  }
}

@keyframes lds-drawer-in-start {
  from {
    transform: translateX(-100%);
  }
}

.chrome {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.header {
  display: flex;
  align-items: center;
  gap: var(--lds-space-2);
  padding: var(--lds-space-3) var(--lds-space-4);
  border-bottom: var(--lds-control-border-width-default) solid var(--lds-color-border-subtle);
}

.header__title {
  flex: 1;
  min-width: 0;
  font-family: var(--lds-font-family-display);
  font-size: var(--lds-font-size-lg);
  font-weight: var(--lds-font-weight-semibold);
  line-height: var(--lds-font-leading-tight);
  overflow-wrap: anywhere;
}

::slotted([slot="actions"]) {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: var(--lds-space-1);
}

.close {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: var(--lds-control-height-sm);
  height: var(--lds-control-height-sm);
  margin: 0;
  padding: 0;
  border: 0;
  border-radius: var(--lds-control-radius-sm);
  background: transparent;
  color: var(--lds-color-fg-muted);
  cursor: pointer;
}

.close[hidden] {
  display: none;
}

.close:hover {
  background: var(--lds-color-bg-subtle);
  color: var(--lds-color-fg-default);
}

.close:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.body {
  flex: 1;
  min-height: 0;
  padding: var(--lds-space-4);
  overflow: auto;
}

.footer {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: var(--lds-space-3);
  padding: var(--lds-space-3) var(--lds-space-4);
  border-top: var(--lds-control-border-width-default) solid var(--lds-color-border-subtle);
}

.footer[data-empty] {
  display: none;
}`,
  });

  /* component: lds-modal (draft) */
  LDS.define({
    tag: "lds-modal",
    className: "LdsModal",
    formAssociated: false,
    observed: ["open", "size", "dismissible"],
    defaults: {"size": "md"},
    template: `<dialog class="dialog" part="dialog">
  <div class="chrome" part="chrome">
    <header class="header" part="header">
      <div class="header__title"><slot name="header"></slot></div>
      <button class="close" part="close" type="button" aria-label="Close">
        <lds-icon name="x" size="sm"></lds-icon>
      </button>
    </header>
    <div class="body" part="body"><slot></slot></div>
    <footer class="footer" part="footer"><slot name="footer"></slot></footer>
  </div>
</dialog>`,
    styles: `/* Blocking dialog built on native <dialog>. */

:host {
  --lds-modal-width: 28rem;
  --lds-modal-bg: var(--lds-color-bg-raised);
  --lds-modal-fg: var(--lds-color-fg-default);
  --lds-modal-radius: var(--lds-overlay-radius);
  --lds-modal-shadow: var(--lds-overlay-shadow);

  display: contents;
}

:host([hidden]) {
  display: none;
}

:host([size="sm"]) {
  --lds-modal-width: 22rem;
}

:host([size="lg"]) {
  --lds-modal-width: 40rem;
}

.dialog {
  box-sizing: border-box;
  width: min(var(--lds-modal-width), calc(100vw - var(--lds-space-8)));
  max-height: calc(100vh - var(--lds-space-8));
  margin: auto;
  padding: 0;
  border: var(--lds-control-border-width-default) solid var(--lds-color-border-default);
  border-radius: var(--lds-modal-radius);
  background: var(--lds-modal-bg);
  color: var(--lds-modal-fg);
  box-shadow: var(--lds-modal-shadow);
}

.dialog::backdrop {
  background: var(--lds-color-bg-overlay);
}

.chrome {
  display: flex;
  flex-direction: column;
  max-height: calc(100vh - var(--lds-space-8));
}

.header {
  display: flex;
  align-items: flex-start;
  gap: var(--lds-space-3);
  padding: var(--lds-space-4) var(--lds-space-5);
  border-bottom: var(--lds-control-border-width-default) solid var(--lds-color-border-subtle);
}

.header__title {
  flex: 1;
  min-width: 0;
  font-family: var(--lds-font-family-display);
  font-size: var(--lds-font-size-lg);
  font-weight: var(--lds-font-weight-semibold);
  line-height: var(--lds-font-leading-tight);
}

.close {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: var(--lds-control-height-sm);
  height: var(--lds-control-height-sm);
  margin: 0;
  padding: 0;
  border: 0;
  border-radius: var(--lds-control-radius-sm);
  background: transparent;
  color: var(--lds-color-fg-muted);
  cursor: pointer;
}

.close[hidden] {
  display: none;
}

.close:hover {
  background: var(--lds-color-bg-subtle);
  color: var(--lds-color-fg-default);
}

.close:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}

.body {
  padding: var(--lds-space-5);
  overflow: auto;
}

.footer {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: var(--lds-space-3);
  padding: var(--lds-space-4) var(--lds-space-5);
  border-top: var(--lds-control-border-width-default) solid var(--lds-color-border-subtle);
}

.footer[data-empty] {
  display: none;
}`,
  });

  /* component: lds-popover (draft) */
  LDS.define({
    tag: "lds-popover",
    className: "LdsPopover",
    formAssociated: false,
    observed: ["open", "placement", "label"],
    defaults: {"placement": "bottom-start"},
    template: `<div class="root" part="root">
  <div class="trigger" part="trigger">
    <slot name="trigger"></slot>
  </div>
  <div class="panel" part="panel" role="dialog" hidden tabindex="-1">
    <slot></slot>
  </div>
</div>`,
    styles: `/* Non-modal anchored panel — trigger in flow, panel positioned with
 * LDS.position. Shares its surface tokens with lds-menu. */

:host {
  --lds-popover-bg: var(--lds-color-bg-raised);
  --lds-popover-fg: var(--lds-color-fg-default);
  --lds-popover-border: var(--lds-color-border-default);
  --lds-popover-radius: var(--lds-overlay-radius);
  --lds-popover-shadow: var(--lds-overlay-shadow);
  --lds-popover-padding: var(--lds-space-3);
  --lds-popover-width: auto;
  --lds-popover-max-width: 22rem;

  display: inline-block;
  position: relative;
  vertical-align: middle;
}

:host([hidden]) {
  display: none;
}

.root {
  display: inline-block;
  max-width: 100%;
}

.trigger {
  display: inline-flex;
  max-width: 100%;
}

.panel {
  box-sizing: border-box;
  position: fixed;
  z-index: var(--lds-z-popover);
  display: none;
  width: var(--lds-popover-width);
  max-width: min(var(--lds-popover-max-width), calc(100vw - var(--lds-space-4)));
  max-height: calc(100vh - var(--lds-space-4));
  overflow: auto;
  padding: var(--lds-popover-padding);
  border: var(--lds-control-border-width-default) solid var(--lds-popover-border);
  border-radius: var(--lds-popover-radius);
  background: var(--lds-popover-bg);
  color: var(--lds-popover-fg);
  box-shadow: var(--lds-popover-shadow);
  font-size: var(--lds-font-size-sm);
  overflow-wrap: anywhere;
}

:host([open]) .panel {
  display: block;
}

.panel[hidden] {
  display: none !important;
}

.panel:focus-visible {
  outline: var(--lds-focus-width) solid var(--lds-color-focus-ring);
  outline-offset: var(--lds-focus-offset);
}`,
  });

  /* component: lds-tooltip (draft) */
  LDS.define({
    tag: "lds-tooltip",
    className: "LdsTooltip",
    formAssociated: false,
    observed: ["content", "placement", "delay", "disabled"],
    defaults: {"placement": "top", "delay": "500"},
    template: `<span class="anchor" part="anchor"><slot></slot></span>
<div class="bubble" part="bubble" role="tooltip" hidden>
  <slot name="content"><span class="text"></span></slot>
</div>`,
    styles: `/* Hover and focus hint. The bubble lives in the host's shadow root, so moving
 * the pointer from the trigger onto the bubble never leaves the host. */

:host {
  --lds-tooltip-bg: var(--lds-color-bg-inverse);
  --lds-tooltip-fg: var(--lds-color-fg-inverse);
  --lds-tooltip-max-width: 20rem;

  display: inline-flex;
  vertical-align: middle;
  max-width: 100%;
}

:host([hidden]) {
  display: none;
}

.anchor {
  display: inline-flex;
  max-width: 100%;
}

.bubble {
  box-sizing: border-box;
  position: fixed;
  z-index: var(--lds-z-tooltip);
  max-width: min(var(--lds-tooltip-max-width), calc(100vw - var(--lds-space-4)));
  padding: var(--lds-space-1) var(--lds-space-2);
  border-radius: var(--lds-radius-md);
  background: var(--lds-tooltip-bg);
  color: var(--lds-tooltip-fg);
  box-shadow: var(--lds-shadow-md);
  font-family: var(--lds-font-family-sans);
  font-size: var(--lds-font-size-xs);
  font-weight: var(--lds-font-weight-medium);
  line-height: var(--lds-font-leading-snug);
  overflow-wrap: anywhere;
  animation: lds-tooltip-in var(--lds-transition-control);
}

.bubble[hidden] {
  display: none;
}

@keyframes lds-tooltip-in {
  from {
    opacity: 0;
  }
}`,
  });

  LDS.version = "0.0.5";
  global.LDS = LDS;
})(typeof globalThis !== "undefined" ? globalThis : window);
