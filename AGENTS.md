# placidus Agent Guidelines

## Product Direction

placidus is a simple, modern, lightweight, and quiet replacement for KonomiTV. It provides television viewing and recording through a web browser, with Mirakurun as its only backend.

Keep the product focused. Do not add social features, viewer chat, live comments, comment overlays, comment APIs, or integrations with external comment services. In this document, "code comments" means documentation written in source files; it never means a viewer-facing comment feature.

## Required Workflow

- Make small, focused commits frequently. Each commit must represent one coherent change and leave the repository in a usable state whenever practical.
- Do not mix refactors, formatting churn, generated files, or unrelated fixes into a feature commit.
- Write or update tests for every practical behavior change. Prioritize tests for scheduling, tuner allocation, Mirakurun communication, encoder selection and fallback, and HTTP handlers.
- Run the relevant tests before every commit. Run the complete test suite before handing work off.
- Write all project documentation in English. Write code comments and identifiers in clear English unless a protocol or broadcast value must retain its original language. Keep user-facing text ready for localization rather than hard-coding language assumptions into domain logic.
- Add clear English comments to non-obvious code, concurrency-sensitive logic, operational workarounds, and important design decisions. Add Go documentation comments to exported declarations. Comments must explain intent or constraints, not merely restate the code.
- Preserve unrelated user changes and never copy implementation code from `KonomiTV/`.

## Architecture Boundaries

- The backend integration target is Mirakurun only. Do not add EDCB, Mirakc, or other backend-specific implementations or compatibility layers unless the project direction is explicitly changed.
- KonomiTV may be studied for product behavior and broad UI information architecture, but placidus code must be written from scratch.
- A limited set of assets, such as program-guide channel logos, may be reused from KonomiTV only after verifying their license, attribution, and redistribution requirements. Record the origin and applicable license of every reused asset.
- Prefer the Go standard library and small, well-maintained dependencies. Avoid a frontend build toolchain unless it becomes strictly necessary.
- Keep boundaries between Mirakurun access, tuner/session coordination, recording, transcoding, persistence, and HTTP presentation so each can be tested independently.

## Frontend Rules

- Render the frontend with Go templates. Template filenames must use the `.gotmpl` extension.
- Do not create standalone CSS files. Put CSS in `<style>` blocks in `.gotmpl` templates; shared styles should live in a shared/base `.gotmpl` template.
- JavaScript may be embedded in templates. A browser library may be imported from a CDN when it materially reduces complexity; pin its version and use integrity and CORS attributes when the CDN provides them.
- The information architecture may feel familiar to KonomiTV users, but the visual design must be original and YouTube-inspired: content-first, responsive, accessible, restrained, and usable in both light and dark environments.
- Prefer server-rendered interactions and small amounts of progressive enhancement over a client-side application framework.
- Never implement or display comments, chat, reaction streams, floating messages, or comment settings.
- Make warnings operationally useful and accessible. Do not communicate important state using color alone.

## Streaming and FFmpeg

- Use FFmpeg as the single media processing executable. Do not select different media binaries for different encoder types.
- Prefer FFmpeg hardware-accelerated encoding. Detect usable encoders through capability checks and an actual initialization/probe, rather than assuming that an advertised encoder works.
- Make hardware encoder selection configurable and choose a sensible available encoder automatically when no preference is configured.
- Fall back to FFmpeg software encoding only when no hardware encoder can be initialized or when hardware initialization fails for the stream.
- Every software-encoding fallback must emit a clear structured log warning and expose a visible warning in the relevant UI. Include the failed hardware path and actionable reason when it is safe to do so.
- Keep FFmpeg argument construction centralized, deterministic, and unit-testable. Never construct commands by concatenating untrusted values into a shell string.
- Ensure spawned FFmpeg processes are tied to request/session contexts, shut down cleanly, and cannot leak after a client disconnects or a recording ends.

## Recording and Tuner Coordination

- Support recording reservations, starting and stopping recordings, persistence of reservation state, conflict detection, and access to completed recordings.
- Treat recordings as higher priority than ordinary live-viewing sessions. Never silently sacrifice a scheduled recording to start unrelated live viewing.
- Coordinate all live and recording consumers through one tuner/session allocation service; do not let individual handlers make isolated allocation decisions.
- When recording occupies the available tuner capacity, allow only simultaneous viewing that can share the already-recorded service/transport stream. The UI must clearly warn that other live channels are unavailable because tuners are occupied by recording.
- Display reservation conflicts and partial failures before they cause a silent missed recording whenever they can be determined in advance.
- Make concurrent allocation and reservation transitions race-safe and cover them with tests.

## Definition of Done

A change is complete only when its behavior is implemented end to end, practical tests pass, relevant documentation is updated in English, non-obvious code is commented, user-visible failures have useful messages, and no comment/social feature has been introduced.
