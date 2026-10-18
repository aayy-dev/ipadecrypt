# Investigation: why korboy's ipadecrypt can't sign in, and the plan

Source-level investigation, 2026-10-18, of korboy's v0.7.3-korboy.2 (`51ed4f3`),
compared with londek's original project (v0.8.0-rc.2, `2fa6924`), R0GUEEE's fork
(v0.7.8-korboy.1, `4be5fc2`), majd/ipatool v2.7.0 (`4484b55`) and aayy-dev/iDecrypt
(`3a60fe8`). Nothing here has touched the iOS device or Apple's servers. Claims
are marked **verified** (read or run) or **inferred**. The limitations and
improvement ideas are tracked separately in [IMPROVEMENTS.md](IMPROVEMENTS.md).

## The cause

**Apple now requires every App Store sign-in request to be signed, and
ipadecrypt doesn't sign them.** High confidence: inferred from the code and
ipatool's history. It hasn't been observed on the device yet.

- **The code (verified).** ipadecrypt doesn't use ipatool. Its App Store
  client is its own Go code in `internal/appstore`, "based on" ipatool's Apple
  Configurator approach. `login.go:48-81` POSTs the login plist with no
  `X-Apple-ActionSignature` header, and nothing in the tree produces one.
- **Apple's change (inferred from the record).** Around 2026-08-20 Apple began
  rejecting unsigned `authenticate` requests: the bag marks them SAP-signed
  (majd/ipatool#522, #520, #523, #530). ipatool switched to SAP signing in
  v2.4.0 (`abd86cb`, 2026-08-28), and londek copied it a day later
  (`126cc67`). korboy's last release, 2026-07-16, predates both.
- **The symptom (verified offline).** Replaying Apple's documented rejections
  through korboy's real client:
  - an empty 403 or 204 gives `login failed`;
  - a 301 or 404 HTML page gives `login: decode plist: ... unexpected hex digit 'h' ... response status=...`.
  - The helper exits 1, and the app shows `FAILED: appstore helper exit 1` with
    the stderr line above it. londek's users report the same strings
    (londek#43, #45).
- **It's fixable.** The user's own ipatool v2.7.0, which signs requests by
  running Apple's code in an emulator, logged in successfully on the Pixel
  Fold on 2026-10-18. So the signed login works today from this network and
  account.

### Problems queued behind it (verified in code; each bites once signing works)

1. **The path needs a trailing slash.** Without it, `/WebObjects/MZFinance.woa/wa/authenticate`
   gives 301 or 204 even when signed (ipatool `f9aa653`, londek `a80aa26`).
2. **Redirects break.** `CheckRedirect` (`client.go:33-36`) only stops a redirect
   when the Referer exactly equals the bare URL. Once the slash is added, Go
   follows Apple's pod 302 as a body-less GET, and the pod attempt is sent as
   `attempt=2`. ipatool never auto-follows; it replays the same signed body
   (`1ec8b3e`, `ab79e42`).
3. **Downloads loop.** Downloads leave out `"serialNumber":"0"`, and Apple's
   5002 is treated as an expired token (`download.go:156-186`). That loops into
   re-login (ipatool `5f776fe`, PR #500). Some builds also need ipatool's
   redownload, update or kbsync fallbacks (`30b2909`, `3411d57`; londek#46
   "empty songList").
4. **The machine ID may be fake.** It's taken from en0's MAC without
   validation (`client.go:46-74`), and macOS 27 reports 02:00:00:00:00:00. What
   iOS gives the root helper is unknown. SAP derives its hardware ID from the
   same bytes.

### What the user sees, and where to look

The app runs `appstore-helper.arm64` (from `cmd/ipadecrypt-appstore-helper`)
through `IDAppStoreHelperRunner`. It passes `--email`/`--password`/`--auth-code`
on argv, and stores credentials in `/var/mobile/Documents/ipadecrypt/config.json`
and `cookies`. The helper's exit codes:

| Exit | Meaning |
|---|---|
| 0 | success |
| 1 | everything else: network, Apple rejection, `login failed`, `account disabled`, 429, `re-auth: …`, storefront, `empty songList`, purchase, patch, appinst, config |
| 2 | bad arguments, or a Go panic |
| 20 | no saved credentials, or wrong password twice |
| 21 | 2FA code needed |
| 30 / 31 | verify failed / decrypt helper failed |
| -1 | set by the app: the helper is missing, failed to spawn, or was killed by a signal |

Logs are in `/var/mobile/Documents/ipadecrypt/logs/app.log`, which has every
helper stderr line as `appstore-helper-stderr: …`. The exit code and the
`@evt` failure events are **not** in app.log; they're only on the progress
screen (its Copy logs button).

**Workarounds that should still work (inferred).** "Latest iOS-compatible"
(StoreKit, using the device's own App Store account) and "Decrypt installed
build" never use the helper's login.

## The fix

**Base: korboy main at `51ed4f3`.** It contains all of londek v0.7.3 (six
upstream merges, 52 korboy-only commits), plus the app, the daemon, the
autoalert tweak and the apt packaging. All reports agree, and the skeptic
couldn't refute it.

**Sign-in: take ipatool v2.7.0's login, not londek's** (londek's copy is stale
and can't run on iOS). Recommended: import `github.com/majd/ipatool/v2@v2.7.0`
`pkg/appstore` for the bag, login and SAP only, and keep ipadecrypt's own
download flow (plus the serialNumber fix). That flow fits its two-step ticket
model better than ipatool's Download. Details:

- **It fits (verified compile-only).** It compiles for `GOOS=ios`, uses the same
  Go 1.25 and the same MIT license, and its Account JSON tags match
  ipadecrypt's config.
- **On iOS, SAP needs a statically linked, W^X-patched Unicorn**, the way
  ipatool's `tools/build-ios.sh` builds it (shipped since v2.6.0). That has to
  go into `app/build.sh`.
- **The first login downloads about 38 MB of Apple assets** (an OS X 10.9
  update package from swcdn.apple.com). The app should say so instead of
  looking hung.
- **Pin v2.7.0.** ipatool main switched its emulator to ARM64 (`0259df3`), but its
  iOS build script still builds x86, so main is likely broken on iOS.

**Port alongside it:**
- the slashed path and validation of the bag's SAP settings;
- the no-auto-follow redirect replay;
- serialNumber, and the right meaning for 5002;
- timeouts and retries, capped so retries don't lock the account (#598);
- a validated MAC (`50312a9`, `0cc226a`);
- 2FA input normalization (`735b689`).

**Take from londek** (cherry-pick, keeping authorship):
- `0a0bf0d`: a PAC-safe inject sentinel, which fixes crashes on A12+ (#42).
- `0d34f0b`: the dlopen wait raised from 3 s to 15 s (#39).
- Optionally `8b02861`: `bootstrap --skip-login`, and decrypting without an Apple ID.

Don't take londek's `login.go`: after a wrong-password retry it sends the next
request to an empty URL (`login.go:102`).

**Take from R0GUEEE:** only the CI improvements (a `go test` job, `goreleaser
check`, no hard-coded release owner) and the idea of a "signature required"
error class.
- **Not the cookie bypass.** Their "App Store access without an Apple ID
  sign-in" scans every app's cookie files and found no Apple session. Their
  own v0.7.8 notes say so.
- **It also brings problems:**
  - a regression that suppresses the Apple ID prompt (`IDHomeViewController.m:789`);
  - accepting a third-party cookie named DSID as an App Store session;
  - a parser offset bug.
- **Their committed binaries do match their CI.** Their inject.c change
  duplicates londek's fixes without credit, so cherry-pick londek's originals.

## iDecrypt: not what we thought

**It contains no ipadecrypt code (verified).** It's donato-fiore/TrollDecrypt
v1.2.4 (`5f9e973`), renamed to `dev.rtrdd.idecrypt`. On top of that come a
62-line headless CLI (`trolldecryptjb`, its usage line copied from
34306/TrollDecryptJB), an output-directory option, removal of the Watch app,
and new icons.

Both upstreams have **no license** (verified via GitHub). So their code can't
safely be copied into this MIT repo.

ipadecrypt's C helper is the stronger engine. It does a ptrace or
SpringBoard spawn, patches dyld so newer-iOS apps decrypt, loads frameworks
the app hasn't loaded through dlopen injection, and verifies cryptid
afterwards. iDecrypt adds:
- a CLI that only needs a bundle ID;
- a launchd launch method that works under TrollStore;
- a file browser;
- a "main binary only" mode;
- localizations.

**Plan:** make `helper/` the one decrypt engine, then:
- add `dump <bundle-id> [out-dir]` and `list` subcommands, with a
  `trolldecryptjb` compatibility symlink;
- write a launchd launch strategy from scratch only if TrollStore is in scope;
- ship one package under a new ID. It should Conflict with and Replace
  `com.korboy.ipadecrypt`, `com.korboy.ipadecrypt-app`,
  `com.korboy.ipadecryptautoalert`, `dev.rtrdd.idecrypt` and
  `com.34306.trolldecryptjb`.
- give it a preinst/prerm that unload korboy's daemon and supervisor. korboy's
  package has no prerm, so they keep running after an upgrade.

**Bugs in iDecrypt as it stands (verified by reading):**
- A failed extension launch returns pid 0, which is then passed to
  `kill(0, SIGKILL)`. That kills iDecrypt's own process group.
- It reports success while frameworks or extensions are still encrypted.

## Security issues to fix before publishing

- **Local root escalation (verified).** `ipadecryptd` listens on a 0666 socket
  and runs whatever helper path the client sends, as root
  (`cmd/ipadecryptd/main.go:31,53,79`).
- **The Apple ID password is on the helper's argv**, so `ps` shows it, and it's
  stored in plaintext in config.json.
- **The desktop CLI ignores SSH host keys** (`ssh.InsecureIgnoreHostKey`), and
  stores the device's sudo password in plaintext.

## Licensing

- ipadecrypt (londek, korboy), ipatool: MIT.
- Unicorn: GPL-2.0, statically linked into the iOS helper.
- purego: Apache-2.0.
- TrollDecrypt, TrollDecryptJB: no license.

A binary that statically links Unicorn is distributed under GPL-2.0 terms. MIT
code is compatible, and publishing the source satisfies it, but the README
should say so.

## Device plan (for when Frida is ready)

In order. Steps marked ⚠ contact Apple with the user's account, so only with
the user's go-ahead. Repeated failed logins can lock an account.

1. **Inventory.**
   - iOS version and build, device model (arm64 vs arm64e decides whether the
     PAC fix matters), jailbreak and version (rootless or roothide), Frida
     version.
   - `dpkg -l | grep -i -E 'korboy|rtrdd|34306|appsync|appinst|filza|ellekit|substrate'`.
2. **Capture today's failure.**
   - ⚠ Tap the auth refresh once.
   - Then `grep appstore-helper-stderr /var/mobile/Documents/ipadecrypt/logs/app.log | tail`,
     plus Copy logs from the progress screen.
   - `appstore-helper.arm64 --auth-status; echo $?`, which makes no network call.
3. **Machine identity.** `ifconfig en0 | grep ether` as root and as mobile, and
   whether Private Wi-Fi Address is on.
4. **Can SAP run on this phone?** ⚠ Run upstream `ipatool-2.7.0-ios-arm64 auth login`.
   - Time it, and watch for `unicorn is not linked`,
     `protect Unicorn translation buffer`, or a JetsamEvent in
     `/var/mobile/Library/Logs/CrashReporter`.
   - Note where its ~38 MB asset cache lands.
5. **Workarounds.** Check that "Latest iOS-compatible" and "Decrypt installed
   build" still work.
6. **Daemon.**
   - `launchctl print system/com.korboy.ipadecryptd`
   - `ps -axo pid,user,command | grep ipadecryptd`
   - `ls -la /var/jb/var/run/ipadecryptd.sock`
7. **iDecrypt.**
   - `dpkg -S /var/jb/usr/local/bin/trolldecryptjb`
   - Run it as mobile and as root.
   - `otool -l … | grep cryptid` on each Mach-O of a recent output.
8. **Frida, observe only.** In `appstored`/`itunesstored`:
   - hook `-[NSMutableURLRequest setValue:forHTTPHeaderField:]` for
     `X-Apple-ActionSignature`;
   - look for `AMSMescal*` classes, a possible way to use the device's own
     signer instead of emulation.

## Open decisions

1. **The sign-in approach.** Import ipatool v2.7.0's `pkg/appstore` for login
   (recommended: least code to maintain), or vendor its `internal/sap`.
2. **iDecrypt.** Fold in its *features*, reimplemented in `helper/`, since its
   code is unlicensed TrollDecrypt. And is TrollStore support in scope?
3. **The new fork's package ID and name**, to stop colliding with korboy's and
   R0GUEEE's `com.korboy.ipadecrypt`.
4. **`aayy-dev/korboy-ipadecrypt`.** It's exactly korboy's v0.7.0-korboy.1 tree
   (`bd9bf91`) minus `.github/` and the executable bits, with nothing of the
   user's own. Delete it, or keep it?

## Local setup

- `~/Git/ipadecrypt`: a clone of korboy. Its remotes `londek`, `r0gueee` and
  `mysnapshot` all have push disabled. The work branch is `aayy/work`.
- `~/Git/iDecrypt`: a clone of aayy-dev/iDecrypt.
- `~/Git/ipatool`: aayy-dev/ipatool. Tag `v2.7.0` is the sign-in reference.
- Once tested, publish as a real public fork of korboybeats/ipadecrypt; it will
  default to `aayy-dev/ipadecrypt`.
