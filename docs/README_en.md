<div align="center">

# szuDesktop · Lychee Garden

A small green corner for everyday life at Shenzhen University: the campus network,
school services, study tools, and a little garden that keeps growing even offline.

<p>
  <img alt="platform" src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-4a6fa5?style=flat-square">
  <img alt="go" src="https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat-square&logo=go&logoColor=white">
  <img alt="license" src="https://img.shields.io/badge/license-MIT-2f7d32?style=flat-square">
  <img alt="release" src="https://img.shields.io/github/v/release/SzuDesktopTeam/szudesktop?include_prereleases&style=flat-square&label=release&color=c9a227">
  <img alt="runtime" src="https://img.shields.io/badge/runtime-no%20dependencies-6b7280?style=flat-square">
</p>

[简体中文](../README.md) · **English**

Unofficial · Built by a student · Not affiliated with Shenzhen University

[Download](#download) · [Features](#features) · [Command line](#command-line-szunet) · [FAQ](#faq) · [Privacy](#privacy-and-security) · [Development status](STATUS.md)

</div>

---

## Preview

| Main window | Lychee Garden | Network sign-in |
| :---------: | :-----------: | :-------------: |
| ![Main window](screenshot-desktop.png) | ![Lychee Garden](screenshot-garden.png) | ![Network sign-in](screenshot-login.png) |

---

## What it solves

The campus network at Shenzhen University is split into **two zones that do not
work with each other**, each using a completely different sign-in method:

| Zone | System | How it works |
| :--- | :----- | :----------- |
| Teaching / office / library areas | SRun (深澜) | Rolled out in early 2025. Password goes through HMAC-MD5, user data through XXTEA with a custom Base64, plus a SHA1 checksum |
| Dormitory / staff areas | Dr.COM (ePortal) | A single GET request is enough |

That creates two headaches:

1. **Every tutorial and script written before 2024 fails in the teaching area** —
   they were all built for the old Dr.COM.
2. **Nobody explains the error messages** — `ldap auth error`, `Rad:userid error1`,
   `登陆失败[05]`, `Unknow ac-type`… so people just guess and retry.

szuDesktop handles both: **one button to sign in, one button to tell you where it got stuck.**

It does not decide the zone by "can I reach the portal". It looks at the **protocol
fingerprint** — the handshake behaviour that only that protocol has (SRun: whether
`get_challenge` succeeds; Dr.COM: whether the ePortal login endpoint exists). The reason is
practical: the dorm portal also returns HTTP 200 on machines in the teaching area, so a
plain reachability test mis-detects the zone while offline and then fires the Dr.COM
protocol at an endpoint that isn't there.

---

## Download

The current published installer is **[szuDesktop-Setup-0.9.0.exe](https://github.com/SzuDesktopTeam/szudesktop/releases/download/beta0.9.0/szuDesktop-Setup-0.9.0.exe)**. Run it, then open szuDesktop. The Windows installer includes its window runtime and Go engine. Portable downloads, CLI binaries and checksums are on the [beta0.9.0 release page](https://github.com/SzuDesktopTeam/szudesktop/releases/tag/beta0.9.0). This branch is preparing beta0.9.1; the new version is not yet available for download.

| Item | Detail |
| :--- | :----- |
| Windows installer | An independent Electron window; a matching `.sha256` file verifies the download |
| Windows portable | `szudesktop-beta0.9.0-windows-amd64.zip`; unzip and run `szudesktop.exe`, using the local Edge / Chrome browser |
| Command line | Windows / macOS / Linux `szunet` binaries remain available |
| Source version | `beta0.9.1` release candidate; runtime, PR and release checks remain pending. Published downloads are still beta0.9.0 |
| Saved data | Both Windows editions use the same local garden and study records; export a backup from Settings before updating |

beta0.9.0 adds a floating desktop pet, a system tray and 40%–200% scaling that persists across restarts. Closing the main window keeps the pet available; click it or the tray to reopen. Official school login and booking pages open inside the installer edition, without a cookie-copy workflow. Calendar OCR now prefers Chinese. See [STATUS](STATUS.md) for the limits of live account validation. Installer autostart is still not implemented.

Published on 2026-09-26: [PR #13](https://github.com/SzuDesktopTeam/szudesktop/pull/13) is merged and the [release checks](https://github.com/SzuDesktopTeam/szudesktop/actions/runs/36220458700) passed, including installation, main window and pet, persistent size after restart, saved data after a same-version reinstall, and uninstall. A subsequent live check passed graduate alternative login and timetable reading. See section 47 of [STATUS](STATUS.md) for the remaining acceptance boundaries.

**beta0.9.1 release candidate, not yet in the downloads:** the pet gains a left/right-click menu, wheel scaling, dragging and remembered position; care actions use the existing garden save. The pixel campus and timber navigation remain, with lighter paper surfaces, green primary buttons and clearer typography. The home page has a shorter welcome section, no duplicate network status and a new companion scene. Forms, service links and narrow layouts share the updated styling. The candidate also includes the school authentication redirect fix. Changes and verification remain in [STATUS](STATUS.md).

### Desktop companion (beta0.9.1 candidate installer)

- **Left-click or right-click** the pet to open its menu, view its current state, pat, feed, play or toggle sleep. Care uses the existing garden rules and local save; insufficient food, low energy and cooldowns are reported.
- **Scroll while hovering over the pet** to change its size by 10 percentage points, from 40% to 200%. Menu controls, the Settings slider and tray presets are also available.
- **Hold and drag the pet** to move it. Its size and position are restored after restart.
- The menu opens the companion area, farm, Study tools or main window, and can hide the pet or quit the app. Use the tray to show a hidden pet again.
- The floating pet and these desktop controls are part of the **Electron installer edition**. The portable edition keeps garden care inside its main window and has no separate pet window.

### First run

1. Open the app and go to **Campus network**
2. Enter your campus card number and unified identity password; tick "remember" if you
   don't want to type it every time
3. Press **Sign in** — it detects whether you're in the teaching or dorm area and uses
   the matching protocol
4. If it fails, press **Diagnostics** — it lists the zone decision, portal reachability,
   protocol fingerprint and what that means

### Opening and exiting

- Starting the installer edition twice focuses the existing window; it can also reuse an already running portable engine of the same version. If an older engine is reported, exit the old edition before reopening
- Closing the main window keeps the pet and tray running. In the beta0.9.1 candidate, choose **Open main window** from the pet menu, or click the tray. Use **Pet menu → Quit app**, **Tray → Exit** or **Settings → Exit** to quit fully. Only the engine started by this instance is stopped; a reused service remains running. The portable edition exits about 10 seconds after all its windows close
- Reloading the page does not stop the service
- To start the service without a window, launch `szudesktop.exe` with `--no-open`
  (meant for headless / sidecar use — the Electron shell starts the Go engine the same way)
- A short, skippable guide appears on first launch; it explains where your data lives,
  how to quit, and what to try first
- The installer edition can disable an existing autostart entry but does not create one in this release. Portable and CLI autostart remain available; unreadable status is reported explicitly

### Where my data lives

- Under the `.szunet` directory in your user profile by default
- Campus network passwords are stored per platform: Windows DPAPI (only this machine and
  this Windows account can decrypt them), the macOS Keychain, and Secret Service
  (`secret-tool`) on Linux. **There is no plain-text fallback on any platform:** when the
  system secure store is unavailable, saving fails with an explicit error instead of quietly
  writing a plain-text file (F24, fixed)
- Garden saves are `workspace-v1.json`, plain JSON you can back up yourself; an export
  never contains your campus account or password
- Before switching computers, export your save in Settings and import it on the new one

---

## Features

These entries describe the current source. beta0.9.1 is a release candidate; published downloads remain beta0.9.0. Verified queries and pending school-account workflows are distinguished below, with evidence in STATUS.md.

| Capability | Status | Notes |
| :--------- | :----- | :---- |
| Campus network sign-in | ✅ | Teaching area (SRun) / dorm area (Dr.COM), zone detected automatically; sign-out and manual zone override |
| Access point ID (`ac_id`) discovery | ✅ | Tries in order: your manual value → what worked on this port before → the gateway redirect → a guess. Guesses are labelled as such |
| Connection diagnostics | ✅ | Lists zone decision, portal reachability, protocol fingerprint and the conclusion |
| Credential storage | ✅ | Windows DPAPI / macOS Keychain / Linux Secret Service; saved **only after a successful sign-in and only if you ticked "remember"**. Without a system secure store it refuses to save and says why — **no plain-text fallback** (F24, fixed). The macOS save path was validated on a real CI runner for beta0.7.3 |
| Notices | Partial | Current source groups notices by college or department: 28 academic-unit links, 17 readable college columns, plus Academic Affairs and the Graduate School. Dates and original links are preserved with a 10-minute cache; other units link to their official sites |
| Room availability and booking | Partial | Live community rooms and half-hour availability on the campus network (read-only). The interface opens the official school page for login and booking and never asks users to copy booking cookies. The server has **no booking write endpoints left** (F23, removed). Library services use a separate official system |
| Calendar and timetables | Partial | Official calendar updates and manual week overrides. Graduate alternative login and timetable reading passed with a real account, distinguishing no scheduled classes from selected courses without arrangements. Undergraduate access, populated schedules and the installer session handoff remain unverified |
| Study reminders | ✅ | Add a reminder manually, export a standard ICS calendar (15 minutes before start). **A reminder is not a booking** |
| Common contacts | Partial | Only numbers verifiable on official school pages (library help desks); other offices link to their official pages |
| Grades and GPA | Partial | Paste or import CSV / TSV grade tables and calculate GPA locally. Manual school queries remain pending live account validation, read only the first page and do not update local GPA records automatically. **No PDF / image / XLSX parsing** |
| Todo and focus timer | ✅ | Todo list plus 5 / 25 / 45-minute focus sessions |
| Desktop pet (installer) | Release candidate | Transparent pet window, left/right-click care menu, 40%–200% wheel/slider scaling, dragging and persistent size/position. This iteration's runtime and final installer checks remain pending |
| College piano rooms | Experimental | Login, paginated rooms and read-only reservations; memory-only session, pending validation with an authorized account |
| Lychee Garden | ✅ | Companion care and growth, crops, plots, watering, harvest, decorations, daily goals, achievements and a field guide. No purchases, no real-money trading |
| Save file | ✅ | Fixed local file, survives restarts and port changes, supports export / import and multi-window conflict protection |
| Launch at login | Partial (Windows only) | Portable and CLI editions retain silent autostart; the installer can disable old entries but does not create new ones. macOS / Linux explicitly report unsupported |

**Current limitations:** scores are limited to the first page and balance is not integrated. Graduate alternative login and the current timetable passed a live read. The undergraduate page returned 403 for the test account, the official graduate scores page did not render its list, and the college piano service could not be reached. The official browser allowed slot selection and opened the booking confirmation form; no reservation was submitted. The candidate installer still failed to load room details, and a later browser refresh also encountered a closed connection. The full booking flow and business-session handoff still require validation. See [STATUS.md](STATUS.md).

**College notice filtering:** selecting a college automatically reads its public column. Unsupported units have an official-site link; authenticated internal notices are not included. This update is included in beta0.6.1.

**Booking flow:** the installer opens a separate official school window for WebVPN, verification, selecting a slot, submitting and viewing results. Its isolated login state lasts only until full application exit. The Go service only reads public rooms and availability; it has no reservation write endpoints and does not confirm reservations automatically. The portable edition continues to use the system browser.

### School login, timetables and scores

In the installer edition, select undergraduate/graduate timetable or scores at the top of Study tools. Open the official login page, complete school verification, return to the main window and read the current login. Then query the selected business. Credentials stay in the school page; cookies never pass through the garden renderer or reach disk. Business access is verified separately. Closing the main window keeps the session; full application exit clears it.

The portable edition retains a collapsed alternative login workflow, including manual session import with OS-protected storage and no plaintext fallback. Never share passwords or cookies in feedback. Online scores are read manually and currently limited to the first page; incomplete results are labelled and are not automatically added to local GPA records. Undergraduate arrangements are preserved as school text, and graduate courses use the school's returned schedule.

A real graduate account passed alternative login and displayed the school's current term and courses without arrangements in the app; the official page likewise returned no scheduled classes. This does not validate populated schedules, scores or undergraduate access. The beta0.9.1 candidate also fixes a blocked WebVPN authentication redirect and an outdated login hint; these fixes are not included in the beta0.9.0 downloads above.

The official calendar is checked daily. Windows OCR prefers Simplified Chinese; failed refreshes preserve the cache and report the cause. Teaching weeks support manual overrides. College piano rooms use a separate campus system with read-only queries; its current HTTP service should only be used on a trusted campus network with its own credentials.

---

## Command line szunet

For scripting, or when you don't want a window. Sources live in `cmd/szunet`;
build with `go build -o dist/szunet ./cmd/szunet`.

| Command | What it does |
| :------ | :----------- |
| `login` | Sign in (pass `-u` card number and `-p` password for one-off use; not saved) |
| `logout` | Sign out |
| `status` | Show current state |
| `detect` | Detect the current zone and access point ID |
| `diag` | Diagnostics: zone decision and protocol fingerprint |
| `config` | View / change local configuration |
| `autostart` | Configure launch at login (Windows only) |
| `vpn` | Guidance for the three official ways to reach the campus network from outside (WebVPN / EasyConnect / zero trust). **Guidance only — it contains no experimental VPN protocol code** |
| `version` | Show the version |

```text
szunet detect                       # which zone and access point ID it detects
szunet login --zone teaching        # force a protocol (auto / teaching / dorm)
szunet login --ac-id 12             # set the access point ID by hand
szunet diag                         # run this first when the network is down
```

Run `--help` for all flags. **Never put real credentials in shared scripts or logs.**

On macOS, credentials saved with `config set` go into the system keychain. Before writing, the
CLI self-checks that the write path works using a throwaway item; if it does not, it fails with a
clear error instead of falling back to putting the password on the command line (where other
processes on the same machine could see it).

**A defect worth stating plainly (F26):** the new macOS CI probe once hit `passwords don't match`
on real hardware — `security -w` **can** ask twice (password, then confirmation), while beta0.7.1
and beta0.7.2 fed it a single line. On such machines the macOS CLI **could not save credentials at
all**: it failed with a clear error, leaked nothing and wrote no plaintext, but the feature did not
work. The behaviour is not stable — the four later runs on the same image (macOS 26.6.2) asked only
once. It now feeds the password plus a confirmation line, which works in both cases (the "asks once"
case is verified on real hardware). **The fix ships in beta0.7.3.** That probe no longer swallows
failures and now gates the release job. See F21 / F25 / F26 in STATUS.md.

---

## FAQ

<details>
<summary><b>Sign-in fails once I plug in my own router</b></summary>

Every network port on campus has its own access point ID (`ac_id`), and you have to report
the one for the port you're plugged into. Older versions always sent `1`, while the router
line expects `12`, and the server rejects a wrong ID outright.

The app now looks for the ID in this order, using the first one that works:

| Priority | Source | Confidence |
| :-- | :-- | :-- |
| 1 | You set it yourself (`--ac-id 12`) | Highest |
| 2 | What worked on **this port** on this machine before | High |
| 3 | Read from the gateway redirect when you're not signed in yet | Authoritative |
| 4 | Trying common IDs one by one | Low (shown as "guess") |

**The ID belongs to the wall port, not to the router**: same router back in its old port →
same ID; different port → different ID; different router in the same port → same ID.

If you see "authentication failed: wrong ac_id", run `szunet detect` to see what it found.
</details>

<details>
<summary><b>My antivirus flags it</b></summary>

The portable executable and Electron installer currently lack a code-signing certificate
and may trigger a SmartScreen prompt on first run. That said, **don't dismiss
every warning as a false positive** — the code
is open source, so you can read it or build it yourself (`go build`) and compare behaviour.
</details>

<details>
<summary><b>Does it keep running after I close the window?</b></summary>

Closing the main window keeps the pet and tray running. In the beta0.9.1 candidate, reopen from **Pet menu → Open main window** or the tray. Quit from the pet menu, tray or Settings to stop its own engine; a reused service stays running.
The portable edition exits about 10 seconds after all its windows close. **Settings → Exit** is also available.
</details>

<details>
<summary><b>Does it reconnect on its own in the background?</b></summary>

No. Whether to sign in is your call, made on the sign-in page. The app never fires
authentication requests on a timer behind your back.
</details>

<details>
<summary><b>Where is my password stored?</b></summary>

On Windows it's encrypted with DPAPI — only this machine and this Windows account can
decrypt it. macOS uses the system Keychain; Linux uses Secret Service. **No platform falls
back to plain text:** if the system secure store is unavailable, saving fails with an explicit
error, and "forget account" also removes any plain-text file an older version may have left
behind. It is saved **only after a successful sign-in and only if you ticked "remember"**, so
a wrong password won't overwrite the stored one. You can clear it at any time with
"forget account".
</details>

---

## Privacy and security

- **No data collection**: no telemetry, no analytics. Everything stays on your machine
- **Credentials encrypted locally**: Windows DPAPI, undecryptable on another machine or
  under another user account; macOS Keychain; Linux Secret Service. No plain-text fallback —
  saving fails with an explicit error when no system secure store is available
- **It never submits for you**: booking a room, picking courses, paying — the app gives you
  the entry point and reminders; the final action is yours, in the official system. The
  experimental server booking endpoints from beta0.6 have been removed entirely (F23), so no
  code path can submit a reservation
- **Loopback only**: the desktop service listens on `127.0.0.1` and refuses to start on a
  non-loopback address; every `/api/*` route checks Host, `Sec-Fetch-Site` and a same-origin
  `Origin`, returning 403 to any other page
- **Dorm-area sign-in is plain text**: the school's Dr.COM gateway is HTTP by default
  (`http://172.30.255.42`). That is the university's protocol, not this app's choice; the
  teaching-area SRun portal is HTTPS and **does** verify certificates
- **Nothing that bypasses billing or shares your connection**. Please follow your
  university's network rules
- **Official sources only**: notices are read from predefined school pages; the app is not
  a general-purpose web proxy

---

## Build and verify

You need Go (see `go.mod`), Python 3 and Node.js.

```text
python desktop/sync-assets.py      # sync interface assets
node   desktop/check-ui.mjs        # the regressions below all run in CI
node   desktop/check-campus.mjs
node   desktop/check-notices.mjs
node   desktop/check-session-ui.mjs
node   desktop/check-academic.mjs
node   desktop/check-school.mjs
node   desktop/check-booking.mjs
node   desktop/check-network-ui.mjs
node   desktop/check-workspace-ui.mjs
node   desktop/check-autostart-ui.mjs
node   desktop/electron/check-sidecar.mjs # Electron startup, shutdown and reuse regression
node   desktop/electron/check-window-policy.mjs
node   desktop/electron/check-pet-policy.mjs
node   desktop/electron/check-pet-settings.mjs
node   desktop/electron/check-pet-view.mjs
node   desktop/electron/check-school-policy.mjs
node   desktop/check-interactions.mjs
node   desktop/check-pet-commands.mjs
node   desktop/check-piano.mjs
go vet ./... && go test ./...      # static checks and unit tests
python desktop/check_release_notes.py # release-notes extraction regression
python desktop/build-windows.py    # build the Windows desktop exe (also the Electron Go sidecar)
node   desktop/electron/build.mjs  # build the Windows Electron installer (run `npm ci` in desktop/electron first)
python desktop/smoke_windows.py    # end-to-end smoke test
python desktop/make_release.py     # produce the release package (only when actually releasing; it overwrites same-named local artifacts)
```

`python desktop/electron/smoke_installer.py` installs, reopens, reinstalls and uninstalls the final package only on a disposable GitHub Windows runner. It must not be run as an installation check on a development machine. PR and tag workflows retain all build and installer checks; release assets are published only after they succeed.

Release notes live in the root [CHANGELOG.md](../CHANGELOG.md): when bumping the version,
rename the `## 未发布` ("unreleased") section to the new version. CI extracts that section as
the GitHub Release body and fails the release if it is missing or empty.
`python desktop/release_notes.py <version>` previews it locally.

The only interface sources are `desktop/index.html` and `desktop/assets/garden/`.
Don't hand-edit build outputs. The default desktop build **excludes the experimental VPN
protocol** and unverified third-party game artwork, and only links to the official WebVPN.
Experimental sources are kept for provenance review and protocol study, and must not be
presented as a finished, verified feature. The exclusion works through a build tag:
`internal/vpn` is referenced only from files guarded by `//go:build campusvpn`, and neither
CI nor the build scripts pass `-tags campusvpn`, so neither the desktop app nor the five CLI
release binaries contain that protocol code. The provenance and licensing of `internal/vpn`
are still unverified (STATUS.md F11): its package comment now says so plainly and makes no
claim of independent authorship, and F06 / F07 / F08 (false "connected" state, missing
timeouts, skipped certificate verification) are all still open. Until the provenance is
verified, this module must not be presented as a working feature or as clean-source code.

---

## Acknowledgements

- [teleostnacl/LoveSzu](https://github.com/teleostnacl/LoveSzu) — reference for undergraduate personal timetable endpoint and field names; implemented independently without copying its source code.
- [Fusion Pixel Font](https://github.com/TakWolf/fusion-pixel-font) — by TakWolf,
  SIL Open Font License 1.1; the licence ships with the package as `FONT-LICENSE-OFL.txt`
- [Sleepstars/SZU-login](https://github.com/Sleepstars/SZU-login) — attribution for the
  SRun xEncode implementation is kept in [LICENSE](../LICENSE)
- [Clash Verge Rev](https://github.com/clash-verge-rev/clash-verge-rev) and
  [FZU Helper](https://github.com/west2-online/fzuhelper-app) — references for interface
  hierarchy and how campus services are organised
- [MattDong123/tools4szu](https://github.com/MattDong123/tools4szu) — by Matt, used with the
  author's permission as research into the university's own system endpoints. No code was
  copied: the endpoint paths, dataset names and field names it documents were re-implemented
  in Go with explicit session-expiry detection. That repository declares no open-source
  licence, so this is an attribution of facts only and not a redistribution of its code

---

## Licence

MIT — see [LICENSE](../LICENSE).

Third-party components keep their own licences: Fusion Pixel Font is under OFL 1.1, and the
experimental VPN module's third-party provenance is still being verified and is not included
in default desktop builds. Names such as EasyConnect remain the property of their
respective owners.
