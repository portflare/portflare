# Research: Portflare client TUI/GUI options

> Status: To be completed. This research brief supports the upcoming client refactor, TUI, GUI, and approval tooling work.

## Summary
Portflare should treat the daemon/local API as the stable product core and build TUI/GUI surfaces as thin clients over that API. For the TUI, Bubble Tea + Lip Gloss is the best Go-native choice for a polished, maintainable interface. For GUI, the most practical first step is either a local web UI served by the client daemon or Wails if a packaged desktop app is required; Fyne/Gio are viable but have higher UX/design cost for a modern product UI.

## Findings
1. **Use the daemon/local API as the boundary** — Portflare already exposes a local API (`/apps`, `/stats`, discovery rescan) and maintains connection/app state in the daemon. Extending this API for server-backed actions such as approve, refresh remote state, login/register, and traffic stats avoids duplicating tunnel/client logic in UI frontends. This also lets CLI, TUI, and GUI share behavior.
   - Recommended shape: `portflare daemon` owns state and websocket tunnel; `portflare tui` and GUI call `http://127.0.0.1:9901`.

2. **TUI: Bubble Tea + Lip Gloss is the strongest fit** — Bubble Tea provides an Elm-style update loop for terminal apps; Lip Gloss provides styling/layout. The ecosystem also includes Bubbles components for tables, lists, spinners, inputs, pagination, etc. This is ideal for a polished terminal dashboard with tabs for Apps, Traffic, Server, Discovery, Logs, and Actions.
   - Sources: [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), [Bubbles](https://github.com/charmbracelet/bubbles)
   - Portflare fit: excellent. Single Go binary, no native GUI dependencies, works over SSH, good for operators.

3. **Alternative TUI libraries are less compelling for a modern product feel** — tview/tcell are mature and widget-heavy, but generally feel more traditional/admin-console than modern. Bubble Tea has a stronger design ecosystem and better fit for a branded CLI product.
   - Sources: [tview](https://github.com/rivo/tview), [tcell](https://github.com/gdamore/tcell)
   - Recommendation: only choose tview if widget completeness is more important than polished brand/UI design.

4. **GUI option A: local web UI is the simplest and most maintainable** — The client daemon can serve a lightweight web UI on the existing local API address. This avoids desktop framework complexity and works anywhere with a browser. It also aligns with the upcoming auth redirect flow and makes it easy to reuse frontend components later.
   - Portflare fit: very strong first GUI. Add `portflare gui` to open `http://127.0.0.1:9901/ui`, but always print the URL for headless servers.
   - Tradeoff: not a native desktop app; browser dependency.

5. **GUI option B: Wails is best for a polished packaged desktop app** — Wails lets Go provide the backend and a web frontend provide the UI. This enables a modern interface with standard web tooling while retaining Go integration. It is better suited than pure Go GUI frameworks if the goal is a modern marketing-quality desktop UI.
   - Source: [Wails](https://wails.io/)
   - Portflare fit: strong if a desktop app is desired after the local web UI proves the workflows.
   - Tradeoff: adds Node/web build pipeline and platform packaging complexity.

6. **GUI option C: Fyne is Go-native and easy to distribute, but less ideal for custom modern UI** — Fyne is mature and cross-platform. It avoids web tooling, but custom high-polish layouts can be harder than HTML/CSS/Wails.
   - Source: [Fyne](https://fyne.io/)
   - Portflare fit: good for simple utility UI, less ideal for highly branded interface.

7. **GUI option D: Gio is powerful but lower-level** — Gio is immediate-mode and highly capable, but teams must build more UI patterns themselves. It is best if Portflare wants a fully custom Go-native app and accepts more design/engineering effort.
   - Source: [Gio](https://gioui.org/)
   - Portflare fit: defer unless there is a strong reason to avoid web UI and Fyne.

8. **GUI option E: webview is lightweight but minimal** — webview wrappers can show local HTML with a native shell, but lifecycle, packaging, OS quirks, and richer app integration are usually better handled by Wails.
   - Source: [webview](https://github.com/webview/webview)
   - Portflare fit: useful for a tiny wrapper around local web UI, but likely not the primary long-term GUI.

9. **Approval workflow should be first-class in all interfaces** — The user wants app approvals from the client tool, not only the server UI. The client daemon should expose local endpoints that proxy authenticated server actions:
   - `GET /server/me/state` or `GET /remote/apps`
   - `POST /remote/apps/{app}/approve`
   - `GET /remote/traffic`
   - `POST /auth/register/start` / callback helpers later
   The TUI/GUI should clearly separate local registration state from server approval state.

10. **Security: do not expose privileged local APIs broadly** — The local API currently defaults to `127.0.0.1`, which is good. TUI/GUI expansion increases the sensitivity of local endpoints because they may trigger server-side approvals, rotate keys, or manage auth. Keep local bind default loopback-only, add optional local API token/CSRF protection for browser UI, and avoid CORS wildcarding.

## Recommended Portflare architecture

### Product surfaces
- `portflare daemon`: long-running tunnel process and local API.
- `portflare expose/list/stats`: simple scriptable CLI commands.
- `portflare tui`: terminal dashboard using Bubble Tea.
- `portflare gui`: opens local browser UI served by daemon; later can become Wails desktop wrapper.

### Local API expansion
Add versioned local endpoints instead of having UI code mutate internal service state directly:

```text
GET  /v1/apps
POST /v1/apps
DELETE /v1/apps/{app}
GET  /v1/stats
GET  /v1/server/state
POST /v1/server/apps/{app}/approve
GET  /v1/server/traffic
POST /v1/discovery/rescan
GET  /v1/events            # SSE for live UI updates, optional
```

### TUI UX model
Use a split dashboard:

```text
┌ Portflare ─ connected as alice ─ r.example.dev ─ 3 apps ─ 12 req/min ┐
│ Apps | Approvals | Traffic | Discovery | Logs | Settings            │
├───────────────────────────────────────────────────────────────────────┤
│ Apps table: name, target, source, connected, approved, public URL     │
│ Right panel: selected app details + actions                           │
└───────────────────────────────────────────────────────────────────────┘
```

Keybindings:
- `a`: add/expose app
- `d`: delete app
- `r`: rescan discovery
- `p`: approve selected app when allowed
- `o`: open public URL
- `s`: stats/details
- `?`: help
- `q`: quit

### GUI UX model
A modern local web UI:
- sidebar: Overview, Apps, Approvals, Traffic, Discovery, Settings
- top status bar: server URL, user, connection status, version
- app cards/table with public URL copy buttons
- approval queue with clear “Approve” action and policy messaging
- traffic charts from interval stats
- onboarding screen when no client key/server configured

## Distribution tradeoffs

1. **TUI in existing binary**
   - Pros: easiest distribution, excellent SSH/server UX, no extra assets required.
   - Cons: terminal-only.
   - Recommendation: implement first.

2. **Local web UI served by daemon**
   - Pros: polished UI with plain static assets, no desktop packaging, works cross-platform.
   - Cons: local browser, local API security/CSRF concerns.
   - Recommendation: implement alongside/after TUI as the first “GUI”.

3. **Wails desktop app**
   - Pros: best polished native-feeling app with web UI quality.
   - Cons: build/release complexity.
   - Recommendation: Phase 2 GUI after local web UI stabilizes.

4. **Fyne/Gio native Go GUI**
   - Pros: Go-native.
   - Cons: harder to achieve very modern branded UX quickly.
   - Recommendation: not first choice for Portflare.

## Security considerations

- Keep local API bound to `127.0.0.1` by default.
- Add a local UI/session token for browser GUI if it can approve apps or rotate keys.
- Use CSRF protection for local browser UI POSTs.
- Do not store or display full client key except in explicit settings/onboarding flows.
- Redact keys in logs and TUI/GUI by default.
- Server approval actions should still enforce server-side permissions; the client UI is not an authority.
- Prefer short request timeouts for UI-to-daemon and daemon-to-server API calls.
- Treat GUI browser-open as best effort; always print local URL for SSH/headless use.

## Suggested phased implementation

### Phase A: refactor client internals
- Split `client/cmd/portflare/main.go` into files:
  - `config.go`
  - `service.go`
  - `localapi.go`
  - `cli.go`
  - `stats.go`
  - `discovery.go`
  - `tunnel.go`
  - `register.go`
- Add typed local API client used by CLI/TUI/GUI.
- Add tests around the API client and handlers.

### Phase B: TUI
- Add Bubble Tea/Lip Gloss/Bubbles dependencies.
- Implement `portflare tui`.
- Start with read-only dashboard: apps, stats, connection.
- Add app expose/delete/discovery rescan.
- Add remote approvals once local API server proxy endpoints exist.

### Phase C: local web GUI
- Add `GET /ui` serving embedded static assets.
- Add `portflare gui` command to open and print URL.
- Use the same local API as TUI.
- Add approval queue and traffic charts.

### Phase D: packaged desktop GUI
- Evaluate Wails wrapper around the same UI and daemon APIs.
- Only proceed if a separate desktop app adds real value over local web UI.

## Final recommendation
Use **Bubble Tea + Lip Gloss for the TUI** and **daemon-served local web UI for the first GUI**. Defer Wails until the UX and workflows are proven. This gives Portflare the fastest path to a polished operator experience without compromising the clean daemon/API architecture or making packaging too complex too early.

## Sources
- Kept: Bubble Tea (https://github.com/charmbracelet/bubbletea) — best-fit Go TUI framework.
- Kept: Lip Gloss (https://github.com/charmbracelet/lipgloss) — styling/layout for polished TUIs.
- Kept: Bubbles (https://github.com/charmbracelet/bubbles) — ready-made TUI components.
- Kept: Wails (https://wails.io/) — practical Go + web desktop app framework.
- Kept: Fyne (https://fyne.io/) — mature Go-native GUI toolkit.
- Kept: Gio (https://gioui.org/) — capable lower-level immediate-mode Go GUI.
- Kept: webview (https://github.com/webview/webview) — lightweight native webview option.
- Kept: tview (https://github.com/rivo/tview) and tcell (https://github.com/gdamore/tcell) — mature terminal UI alternatives.

## Gaps
No live web fetching tool was available in this delegated environment, so recommendations are based on established Go ecosystem knowledge and known project documentation URLs. Before final dependency selection, verify current platform support, release activity, license compatibility, and binary-size impact for the selected GUI/TUI libraries.
