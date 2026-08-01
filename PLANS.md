# placidus Development Plan

## Vision

placidus will be a web television application dedicated to Mirakurun. It should preserve the quick discoverability and practical television workflows that users value in KonomiTV while replacing its implementation with a smaller Go codebase and an original, YouTube-inspired interface.

The product should feel calm: fast pages, restrained motion, clear operational state, few dependencies, and no social noise. placidus will not contain viewer comments, live chat, floating comment overlays, reactions, or integrations with comment platforms.

## Non-Goals

- Supporting television backends other than Mirakurun.
- Copying or adapting KonomiTV source code.
- Reproducing KonomiTV's visual design pixel for pixel.
- Providing any viewer comment or chat functionality.
- Maintaining separate media executables or pipelines for each hardware vendor.
- Building a large single-page application when server-rendered Go templates are sufficient.

## Product Principles

1. **Mirakurun-native:** Model services, programs, tuners, and streams around Mirakurun's API rather than a generic multi-backend abstraction.
2. **Reliable recording first:** A reservation must not be displaced by opportunistic live viewing, and conflicts must be visible before airtime when possible.
3. **Hardware-first media processing:** Use FFmpeg for every transcoding path, prefer working hardware encoders, and make software fallback explicit.
4. **Server-rendered simplicity:** Use `.gotmpl` Go templates, embedded styles, and only modest progressive JavaScript.
5. **Original, familiar UI:** Borrow useful information hierarchy concepts from television applications while creating a distinct YouTube-like visual language.
6. **Quiet by design:** Show actionable system warnings, but omit comments, social engagement, visual clutter, and unnecessary notifications.
7. **Testable boundaries:** Keep external I/O behind narrow interfaces and test scheduling and allocation as deterministic domain logic.

## Proposed Architecture

The first implementation should remain a modular Go application rather than a distributed system:

```text
Browser
  -> Go HTTP server and .gotmpl views
      -> live/recording coordinator
          -> Mirakurun API and MPEG-TS streams
          -> FFmpeg process manager
          -> reservation/recording store
      -> HLS segments and recorded media
```

Suggested responsibilities:

- `cmd/placidus`: configuration loading, dependency wiring, lifecycle, and graceful shutdown.
- `internal/mirakurun`: typed API client, response validation, timeouts, and test doubles.
- `internal/tuner`: shared live/recording allocation, stream sharing, priorities, and conflict explanations.
- `internal/transcode`: FFmpeg discovery, hardware probes, command construction, process supervision, and fallback reporting.
- `internal/recording`: reservations, scheduler, recording lifecycle, file naming, recovery, and retention metadata.
- `internal/store`: persistence and schema migrations. Start with a local transactional store such as SQLite unless deployment requirements prove otherwise.
- `internal/http`: routes, handlers, middleware, view models, embedded assets, and `.gotmpl` rendering.
- `web/templates`: base, partial, and page templates with embedded CSS and small progressive-enhancement scripts.

The exact package layout may evolve, but dependency direction must keep domain decisions independent from HTTP handlers and external processes.

## Core User Experience

### Navigation and Visual Language

Use a responsive top bar and compact side navigation, content cards with clear thumbnails or channel marks, a prominent player, and a dense but readable program guide. The result should evoke the efficiency of YouTube without copying its branded assets or exact layout.

Shared CSS belongs in a base `.gotmpl` template. Page-specific CSS belongs in that page's `.gotmpl` file. No standalone CSS files should be introduced. Prefer semantic HTML, keyboard navigation, visible focus states, reduced-motion support, sufficient contrast, and layouts that work on phones, desktops, and television-sized displays.

### Primary Screens

- Home: currently airing programs, upcoming programs, recent recordings, and system status.
- Live TV: player, current/next program details, channel selection, quality selection, and encoder/fallback state.
- Program guide: time-and-channel grid with program details and recording actions.
- Reservations: upcoming, active, conflicting, failed, and completed states.
- Recordings: playback and metadata for completed recordings.
- Settings/status: Mirakurun connectivity, detected FFmpeg version, usable hardware encoders, storage status, and safe diagnostic details.

There will be no comments panel, comment count, chat drawer, floating messages, comment preferences, or related API routes on any screen.

## Streaming and Encoding Design

Mirakurun supplies the broadcast stream. placidus owns session sharing, FFmpeg supervision, and browser-compatible output. Initial browser delivery should use HLS unless an implementation spike demonstrates a simpler reliable alternative with comparable browser support.

At startup, and again on configuration changes, the transcoding service should:

1. Verify the configured FFmpeg executable and minimum supported capabilities.
2. Enumerate candidate hardware encoders appropriate to the host, such as NVENC, QSV, VAAPI, or VideoToolbox implementations exposed by FFmpeg.
3. Run a short real initialization probe for candidates rather than trusting encoder listings alone.
4. Cache successful capabilities and retain diagnostic failure reasons.
5. Select the configured encoder when usable, otherwise the best usable hardware option.

For each stream, FFmpeg should use a hardware encoder when one can be initialized. If hardware initialization fails, the same FFmpeg executable may restart the job with a software encoder. The fallback must produce a structured warning in server logs and a visible, concise warning in the player or recording status UI. A fallback must never be silent.

Encoder arguments, HLS settings, quality profiles, and subprocess lifecycle behavior should be centralized and tested. User-controlled input must be passed as process arguments, never interpolated into a shell command.

## Recording and Tuner Policy

Reservations are durable records with at least program/service identity, start/end timing, margins, state, and failure information. The scheduler must tolerate restarts, reconcile in-progress state, and record a specific outcome rather than silently dropping work.

One coordinator must account for both live sessions and recordings. Its policy is:

1. Active and imminent recordings have priority over unrelated live sessions.
2. Consumers that can safely share the same Mirakurun service or transport stream should reuse it.
3. When recordings consume all available tuner capacity, users may watch only the service/stream that can be shared with an active recording.
4. In that state, the UI must warn that recording occupies the tuners and that unrelated channel changes are unavailable.
5. Known reservation conflicts must be surfaced in the program guide and reservation list before airtime.
6. Allocation decisions and state transitions must be concurrency-safe, deterministic, and covered by race-oriented tests.

The initial recording format should favor preserving the original Mirakurun MPEG-TS stream. Playback-oriented transcoding can remain a separate concern so a transient encoder failure does not unnecessarily destroy the source recording.

## Delivery Phases

### Phase 0: Foundation

- Establish package boundaries, configuration, structured logging, graceful shutdown, and an English contributor README.
- Add template embedding and rendering for `.gotmpl` files, with shared embedded CSS and a minimal responsive application shell.
- Create a test harness, Mirakurun fake server, temporary storage fixtures, and CI commands.
- Decide and document supported Go, Mirakurun API, FFmpeg, browser, and operating-system versions.

Exit criterion: the application starts, renders a health/status page, reports Mirakurun and FFmpeg readiness, and passes unit and HTTP tests.

### Phase 1: Mirakurun and Program Discovery

- Implement the typed Mirakurun client for version, status, tuners, services, and programs.
- Normalize program times and service identifiers without hiding Mirakurun-specific semantics.
- Build the home screen, channel list, program details, and responsive program guide.
- Add optional channel-logo assets only after license and provenance review.

Exit criterion: users can browse current and upcoming programming with graceful loading, empty, and backend-error states.

### Phase 2: Live Viewing

- Implement the tuner/session coordinator and shared Mirakurun stream ownership.
- Implement FFmpeg capability probes, hardware encoder selection, supervised HLS output, and explicit software fallback.
- Build the live player, channel switching, current/next details, quality controls, and warning states.
- Test disconnect cleanup, concurrent viewers, shared streams, encoder failure, and software fallback.

Exit criterion: supported browsers can watch live TV, hardware encoding is used when available, and every software fallback is visible in both logs and UI.

### Phase 3: Reservations and Recording

- Add durable reservation storage and schema migrations.
- Implement program-based reservations, margins, cancellation, conflict detection, and scheduler recovery after restart.
- Save recordings reliably and expose lifecycle and failure state.
- Integrate recording priority and simultaneous-viewing restrictions into the coordinator and UI.
- Add completed-recording listing and browser playback.

Exit criterion: reservations survive restarts, recordings start and stop at the intended times, conflicts are explained, and tuner-occupancy restrictions are enforced and clearly displayed.

### Phase 4: Hardening and Release

- Add end-to-end tests against a controlled Mirakurun-compatible fixture and FFmpeg test inputs.
- Run concurrency and race tests around allocation, cancellation, and shutdown.
- Add storage monitoring, orphan-process cleanup, retention controls, and recovery documentation.
- Audit accessibility, responsive behavior, dependency licenses, reused asset provenance, and security-sensitive inputs.
- Document installation, configuration, FFmpeg hardware setup, fallback troubleshooting, backups, and upgrades in English.

Exit criterion: a fresh installation can be configured from documentation, core workflows pass automated tests, failure modes are actionable, and reused assets have auditable licensing records.

## Testing Strategy

- Unit tests: allocation policy, overlap/conflict calculation, reservation state transitions, FFmpeg argument generation, encoder ranking, fallback decisions, and view-model construction.
- HTTP tests: routes, status codes, template rendering, accessibility-critical warning text, validation, and error handling.
- Contract tests: Mirakurun request paths, parameters, timeouts, stream cleanup, and representative API responses.
- Integration tests: supervised FFmpeg runs using generated test inputs, HLS availability, cancellation, and fallback behavior where the test host permits it.
- Persistence tests: migrations, restart recovery, transaction rollback, duplicate scheduling, and concurrent updates.
- Race and lifecycle tests: simultaneous live/recording requests, tuner exhaustion, client disconnects, process termination, and application shutdown.

Hardware-dependent tests should skip with an explicit reason when the required device is absent. The software fallback path must always remain testable in CI.

## Engineering Cadence

Work in small vertical slices and commit frequently. Each commit should contain one reviewable idea, its practical tests, relevant English comments, and documentation updates. Prefer several independently passing commits over one large feature dump. Before each commit, run focused tests; before completing a phase or handing work off, run the full test suite and any applicable race or integration tests.

## Open Decisions

Resolve these through short documented implementation spikes before their dependent phase:

- Exact HLS segment duration, latency target, and cleanup policy.
- Supported hardware encoder ordering per operating system and the configuration override format.
- SQLite driver and migration mechanism.
- Recording filename, metadata, and retention conventions.
- Authentication and trusted-network defaults for the first release.
- Whether concurrent viewing can share at the service level or transport-stream level for each Mirakurun deployment mode.

Decisions should optimize for a simple, modern, lightweight, and quiet product rather than feature parity with KonomiTV.
