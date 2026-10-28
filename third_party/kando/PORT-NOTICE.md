# Kando renderer port

Upstream: https://github.com/kando-menu/kando
Pinned commit: d860e24da203f8969872f62846fad5a6a859e522 (retrieved 2026-10-28 via GitHub).

The selected original sources and menu themes in this directory retain their upstream SPDX notices. Source copyright: Simon Schneegans <code@simonschneegans.de>, MIT. Theme configuration/CSS and theme SVG assets: upstream per-file notices, generally CC0-1.0. Converted theme.json files derive from the matching theme.json5 files and retain author/license metadata.

Working adapted sources are under desktop/floating-src. Menu, geometry, center text, wedge rendering, pointer/gesture/keyboard/gamepad implementations are carried over. Adaptations: HTTP CSS loading, MacroLeprechaun icon adapter, renderer-host bridge instead of Electron IPC, build-time constants/default settings, local sound URLs. The original Electron application shell, action system and application-wide account/update/achievement features are not part of this renderer port.

The sound assets in desktop/effects/sounds-simple-clicks are from Kando's Simple Clicks sound theme. Each sound's original author/source is documented in REUSE.toml; LicenseRef-Pixabay.txt is shipped with them. License copies for MIT/CC0 and runtime dependencies are also in docs/licenses.
