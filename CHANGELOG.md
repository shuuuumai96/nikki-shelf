# Changelog

All notable changes to Nikki will be documented in this file.

This changelog follows a simple Keep a Changelog-style structure. Nikki has not yet made a stable public release, and this file does not attempt to reconstruct detailed private development history that has not been release-curated.

## [Unreleased]

### Added

- Added a Today-screen memory shelf for revisiting older diary entries, with mood exclusions, collapse/expand behavior, and a direct return-to-Today flow after opening a memory.

### Fixed

- Fixed production backup uploads-volume detection so Compose project names derived from deployment directories are handled automatically.
- Fixed duplicate rendering of uploaded entry images.
- Fixed mobile layout collapse on iPhone Safari around the bottom navigation and editor chrome.

## [0.3.0] - 2026-06-07

### Added

- Added frontend characterization tests for the app shell, setup flow, authenticated navigation, and offline banner behavior.

### Changed

- Split the main frontend app shell into smaller view, shell, navigation, and loading components.
- Lazy-loaded heavier entry and settings views to reduce the initial frontend bundle.
- Clarified release-boundary documentation around limited installable web app behavior, unsupported offline diary-data caching, and supported restore behavior.

### Fixed

- Fixed setup restore page scrolling so long restore forms remain usable on constrained viewports.
- Fixed Docker-only validation so git diff checks do not invoke a pager.

### Notes

- No database schema, authentication, or backup/restore format changes are included in this release.

## [0.2.0] - 2026-06-05

### Added

- Added installable web app assets, app icons, manifest metadata, and basic service-worker shell caching for the diary frontend.
- Improved mobile diary writing ergonomics while keeping mobile image upload and offline writing outside the supported release scope.
- Adopted Apache-2.0 as the explicit project license.
- Added initial changelog and release/versioning policy documentation.
- Added Docker-only repository validation for contributors who prefer not to install the local frontend/backend toolchains.

### Fixed

- Replaced the Apache license appendix placeholder with the Nikki copyright notice.

### Notes

- Dependency license audit remains a separate future task before a formal public release.
