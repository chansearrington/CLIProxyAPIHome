# CodexBar UI freezes on browser and audio service calls

Status: local mitigation verified, 2026-10-15 01:14 UTC — captured blocks absent across refresh/sync observation; all ten Claude usage checks and token sync pass. Awaiting optional user feedback on the menu.

The user reported repeated unresponsiveness and force quits after the latest account setup,
then confirmed the affected program is **CodexBar**, not the Codex desktop app or CLI.
The running binary is still **0.60.3 (146)**. The recent fleet work added credential scopes,
priorities and local token mappings; it did not upgrade the CodexBar executable.

## Captured evidence

Two independent three-second samples of the original app, PID 71872, show every main-thread
sample waiting in WebKit's media-routing initialization:

`WebMediaSessionManager::taskTimerFired` → `AVRoutePickerView` → MediaExperience routing
context → synchronous XPC reply wait.

The second sample retained the same main-thread call chain. This is an actual native UI
thread block, independently of model inference or whether Anthropic's quota API succeeds.
macOS is **27.0.1 (26A434)**. The captured stacks do not identify the exact system-service
failure or establish that adding a particular account caused it.

The pre-change preference `openAIWebAccessEnabled` was true. Pinned CodexBar source
`UsageStore+OpenAIWeb.swift` gates the optional dashboard/WebKit refresh on this setting.
Upstream [issue #678](https://github.com/steipete/CodexBar/issues/678) describes hangs in that
optional path and a disable-Web-Extras workaround, but its signatures are not identical to
this Mac's captured audio-routing wait. Treat it as supporting context, not proof of an
identical defect.

Disabling that optional feature and restarting exposed another synchronous main-thread
block in the next process, PID 45430:

`NSSound.play` → `AVAudioPlayer` → `AudioQueueNewOutput` → CoreAudio device initialization
→ Mach reply wait.

Pinned `SessionQuotaNotifications.swift` calls `NSSound.play()` on the main actor for quota
and predictive warnings when sound is enabled. `SettingsStore.swift` defaults
`quotaWarningSoundEnabled` to true. The second stack therefore establishes a second blocking
audio entry point; disabling Web Extras alone was insufficient. An AppleEvent version query
can still return and is not, by itself, evidence that the menu thread stays responsive.

## Local mitigation

Saved private mode-0600 config, sync settings, preference and diagnostic snapshots under
`/private/tmp/cpa-codex-freeze-20261015/` (directory mode 0700). Held the helper's sync lock
while terminating only the sampled hung CodexBar process and relaunching it.

- Set **`openAIWebAccessEnabled = false`** to remove the optional dashboard/WebKit path.
- Set **`quotaWarningSoundEnabled = false`** to remove synchronous warning audio playback.
- Preserve quota warnings, predictive warnings, on-screen alerts, all ten Claude accounts,
  the original token-sync mappings and automatic account following.

The app itself normalized the Codex provider's `cookieSource` from unset to `off` when Web
Extras was disabled. This is the expected browser-extras configuration change, not an account
or token change. No Claude provider source change was made: the selected-token source-mode
resolver already routes those account entries to OAuth.

The OpenAI Docs skill was initially used while the product name was ambiguous. Once the
user confirmed CodexBar, diagnosis relied on this process's samples and the pinned
CodexBar source. The unrelated Codex desktop/CLI processes and CPA Home/node runtime were
not restarted or reconfigured. No fleet lock or Home database mutation is required for
these local usage-app preferences.

## Completed verification and limits

All ten stored Claude accounts still pass OAuth usage checks, completing in **2.59 seconds**.
`ClaudeProviderDescriptor.swift:64` confirms selected OAuth token accounts use OAuth even though the saved
Claude provider's historical base source says web; changing that base source was unnecessary.

The final app process **51300** stayed running through multiple one-minute usage refreshes
and the two-minute sync interval. Samples after the second mitigation show normal menu event
tracking, drawing and event-loop waits, with neither captured audio-block signature. At
**01:14:16 UTC**, uptime was **3:38**, and the subsequent process read showed **0.0% CPU**.
The scheduled sync completed successfully at **01:13:50 UTC**, retained all **ten** accounts
and reported healthy automatic following. No other application's settings or source code changed.

Compared pre/post account structures and every sync mapping. All identities, tokens' owning
workflow, labels, order, enabled states and unrelated configuration remain intact; token values
and selected index may advance normally through the existing helper. The only provider-JSON
change was the app's Codex cookie-source normalization described above. Silent threshold and
predictive warnings and on-screen alerts remain enabled.

The native computer-use pipe failed at startup, so no visual menu inspection is claimed.
An optional user confirmation question is pending. The observed interval is bounded; it is
evidence of current recovery, not proof that every intermittent OS or app defect is eliminated.

Checked upstream's latest release metadata: **v0.75.0**, published **2026-10-14 00:59:30 UTC**.
The inspected release notes do not establish a fix for these exact audio calls, and no binary
upgrade was performed as part of this narrow mitigation. The OS audio-service root failure
and whether the latest account addition triggered it remain unproven.

Rollback preferences individually from the private pre-change snapshot, preserving current
account objects and rotating tokens. Re-enabling either affected path may reintroduce the
captured UI block; do not restore a stale whole JSON file or preference domain.
