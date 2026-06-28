# [OPEN] playback-login-500

## Symptom
- Browser refresh followed by immediate playback attempt can fail with `登录 HIK 设备失败：设备请求失败，状态码 500`。
- Navigating away from the playback page and returning makes playback work again.

## Scope
- Area under investigation: playback page login/config/session chain.
- Constraint: do not change business logic before runtime evidence is collected.

## Hypotheses
- H1: On first load after refresh, the playback page starts login before the channel's WebSDK config request has fully resolved or stabilized, causing a stale/incomplete login payload.
- H2: The backend `GetLiveWebControlConfig` path sometimes returns a transient 500 on first access because recorder/channel relations or decrypted credentials are not ready in the request path being used.
- H3: Frontend playback lifecycle state is racing on refresh, so an old teardown/reset or token-guarded callback invalidates the first login sequence but a second page entry succeeds.
- H4: WebSDK session/bootstrap state in the browser is not ready immediately after refresh, and the second page entry works because SDK initialization completes by then.
- H5: A backend dependency used only during first playback login after refresh is warming up lazily, producing a transient 500 that disappears on the next attempt.

## Plan
- Trace the refresh -> channel select -> config fetch -> player login -> playback start chain.
- Check whether the current code already exposes a likely race or transient-failure gap.
- If static analysis is inconclusive, add Ubuntu Docker friendly HTTP debug reporting with request/session markers for pre-fix evidence.

## Static Findings
- `PlaybackView.vue` refresh path can auto-run `searchSegments(false)` on mount when the route already carries `channelId`, so a cold page entry may immediately drive WebSDK login/search activity.
- Playback uses `getChannelLiveWebControlConfigApi()` first; backend `GetLiveWebControlConfig("channel", ...)` only reads channel/recorder records and decrypts the recorder password. This path would fail before SDK login and is not the most likely source of the observed `登录 HIK 设备失败：设备请求失败，状态码 500` message.
- The displayed error string is produced inside `HikWebControlPlaybackPlayer.vue` around `I_Login()`, which means the failure is happening in the browser WebSDK login/proxy layer rather than in the config fetch API itself.
- `HikWebControlGrid.vue` explicitly calls `ensureHikProxyRoutingInstalled()` before loading WebSDK scripts, but `HikWebControlPlaybackPlayer.vue` does not. This creates a static inconsistency in proxy bootstrap behavior between preview and playback.
- The symptom "switch to another page, then come back and playback works" matches the possibility that another page using `HikWebControlGrid.vue` installs the global proxy routing patch first, after which playback benefits from the warmed global state.

## Hypothesis Status
- H1: still plausible.
- H2: weakened by static evidence.
- H3: still plausible but secondary.
- H4: weakened because playback awaits SDK initialization before login.
- H5: possible but currently less supported than the proxy-bootstrap mismatch.

## Applied Fix
- Added `ensureHikProxyRoutingInstalled()` to `HikWebControlPlaybackPlayer.vue#getSdk()` so playback cold-start now bootstraps the same global proxy routing patch that the preview grid already installs before WebSDK login traffic begins.
