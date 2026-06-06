# Roadmap Themes

This roadmap is direction, not commitment. Items below describe possible product and project work that should be separately approved before implementation.

## P0: OSS Readiness

**Goal:** Prepare Nikki to be understandable, reviewable, and safer to publish as a self-hosted OSS project.

**Why it matters:** A public repository needs clear scope, contribution expectations, security reporting, license status, release notes, and safety boundaries before broader use.

**Non-goals:** Choosing a license without maintainer approval, adding GitHub Actions, accepting broad feature requests, or repositioning Nikki as SaaS.

**Risks:** Publishing too early can expose unclear security posture, missing license terms, unsafe deployment assumptions, or stale private-project language.

**Suggested acceptance criteria:**

- README describes Nikki as self-hosted, single-user or small trusted household software.
- Scope, frozen areas, and deployment warnings are easy to find.
- `SECURITY.md` and `CONTRIBUTING.md` exist.
- License decision is explicit, even if pending.
- Release and changelog expectations are documented.

## P1: PC Browser Writing Cockpit

**Goal:** Make the PC-browser writing flow feel reliable, calm, and focused for daily records.

**Why it matters:** Nikki's strongest product direction is text-first writing in a desktop browser, not mobile capture or broad workspace management.

**Non-goals:** Complex editor frameworks, rich block editing, PKM workflows, or broad visual redesign.

**Candidate but separate approval:** Focus mode, clean writing mode, keyboard shortcuts, and command palette may be considered later as writing-cockpit improvements. They are not approved or implemented by this roadmap.

**Risks:** Small editor changes can affect autosave, conflict handling, keyboard behavior, and the single-tab assumption.

**Suggested acceptance criteria:**

- Daily writing remains fast and readable on desktop.
- Autosave status and conflict fallback remain understandable.
- Existing single-tab assumptions are preserved.
- Any UI changes stay simple, calm, document-oriented, and low-clutter.

## P1: Archive/Search/Retrieval

**Goal:** Help users find, read, and recover older diary records.

**Why it matters:** A diary becomes more valuable as its archive grows. Retrieval should support memory and continuity without turning Nikki into a PKM system.

**Non-goals:** Backlinks, graph views, advanced knowledge-base workflows, or global document management.

**Risks:** Search can become broad and expensive; archive UI can become a second product surface; indexing decisions may affect privacy and backups.

**Suggested acceptance criteria:**

- Users can locate older entries by clear diary-oriented criteria.
- Archive reading is calm and distinct from heavy editing.
- Retrieval work has tests appropriate to query behavior.
- No schema or indexing change is made without backup/restore consideration.

## P1/P2: Lightweight Reflection

**Goal:** Add modest reflection support that helps users review their own records.

**Why it matters:** Reflection can make daily writing more useful while staying close to the diary's core purpose.

**Non-goals:** AI coaching, mental health diagnosis, gamified analytics, habit platforms, or statistics expansion for its own sake.

**Risks:** Reflection features can feel invasive, over-prescriptive, or misleading if they imply insight the product does not actually provide.

**Suggested acceptance criteria:**

- Reflection stays opt-in and diary-centered.
- The UI does not pressure users to quantify private life.
- Any summaries or prompts are clearly bounded.
- Existing diary data remains recoverable and exportable.

## P2: Cautious Installable Web App

**Goal:** Keep installability as a small browser convenience.

**Why it matters:** Some users may prefer launching Nikki like an app while still using online self-hosted data.

**Current status:** Nikki has installable web app metadata, app icons, standalone display, mobile web app meta tags, and basic service-worker app-shell caching.

**Non-goals:** Offline writing, offline sync, background recovery, background sync, push notifications, authenticated diary-data caching, or mobile-first product direction.

**Risks:** PWA language can imply offline guarantees that Nikki does not provide. Installability can also distract from PC-browser writing reliability.

**Acceptance criteria:**

- Scope is limited to manifest metadata, icons, theme color, standalone display, mobile web app meta tags, and static app-shell caching.
- Documentation states that offline writing is unsupported.
- The service worker does not cache authenticated API responses, uploads, or diary data.
- Desktop browser behavior remains the primary acceptance target.

## P3: Offline Draft, Mobile Image Upload, AI Reflection, Multiple Journals

**Goal:** Keep higher-risk or broader product ideas visible without approving them.

**Why it matters:** These ideas may be useful later, but each one changes Nikki's complexity, support burden, or product identity.

**Non-goals:** Implementing any of these items during OSS readiness, bundling them into unrelated refactors, or treating them as promised features.

**Risks:** Offline drafts require difficult conflict and recovery semantics; mobile image upload pushes Nikki toward a photo workflow; AI reflection raises privacy and trust concerns; multiple journals may introduce permission and data-model complexity.

**Suggested acceptance criteria:**

- A separate product decision exists before any implementation starts.
- Security, backup, and restore implications are documented first.
- User-facing language avoids promising these features.
- Each item has a narrow design proposal before code changes.
