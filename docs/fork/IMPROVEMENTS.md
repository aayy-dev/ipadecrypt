# Limitations and improvements

> **Shipped (2026-10-18, dev.aayy.ipadecrypt 0.7.3-aayy.4):** SAP sign-in,
> ipatool downloads + FairPlay-licence (.sinf) write, and verify that skips
> newer-iOS-only components instead of failing. On the test device YouTube
> (min iOS 17) decrypts 7/8 Mach-Os; the 8th is an iOS-26-only extension that
> can't run or be decrypted on iOS 16 and is now reported as skipped.


A running list of what makes ipadecrypt (korboy v0.7.3-korboy.2, `51ed4f3`) hard
for a casual user, and what to change. Every item cites the code it comes
from (`file:line@ref`). Severity is blocker, major or minor; improvements are
tagged `[priority/effort]` (priority: must, should, nice; effort: small,
medium, large).

Started 2026-10-18 from a source-level investigation; nothing here has been
checked on a device yet. See [INVESTIGATION.md](INVESTIGATION.md) for the
root cause and the plan, and keep this list updated as items are done,
dropped or found.

## Limitations
- [blocker] **App Store sign-in cannot succeed: the authenticate request is unsigned** — Tapping the circular-arrow auth button, 'Latest from App Store' or 'Select App Store version' shows 'Signing in to Apple ID', then a raw row such as 'login failed' or 'login: decode plist: ... unexpected hex digit `h' ... response status=403|404|301', then 'FAILED: appstore helper exit 1'. Nothing says that Apple changed its protocol or what the user can do instead.
  cause: Verified in the code: internal/appstore/login.go:48-81@51ed4f3 POSTs the login plist with no X-Apple-ActionSignature, and `git grep ActionSignature` at v0.7.3-korboy.2 returns nothing. interpretLogin (login.go:120-161@51ed4f3) collapses empty 403/204 bodies into 'login failed'. Inferred from ipatool and upstream history: Apple has required SAP signing since about 2026-08-20. ipatool fixed it in abd86cb (v2.4.0, 2026-08-28), forces the slashed path in f9aa653 and stops auto-following auth redirects in 1ec8b3e. korboy's CheckRedirect (client.go:31-40@51ed4f3) would also turn pod 302s into body-less GETs once the slash is added.
- [blocker] **Desktop CLI is fully blocked when App Store login fails, even for jobs that need no Apple ID** — `ipadecrypt bootstrap` aborts at step 1/5 with the login error, so SSH and helper setup are never reached. `ipadecrypt decrypt <installed app | local .ipa>` then refuses with 'environment not configured / run ipadecrypt bootstrap first'.
  cause: Verified: cmd/ipadecrypt/bootstrap.go:48-52@51ed4f3 returns on any authenticateApple error before step 2. cmd/ipadecrypt/decrypt.go:226-228@51ed4f3 requires cfg.Apple.Email != "" for every decrypt. londek fixed both in 8b02861 (bootstrap --skip-login; decrypt.go:383-385@2fa6924 only asks for an Apple ID when an App Store download is needed). korboy never merged it.
- [major] **After login is fixed, downloads, version lists and metadata may still fail** — A user who gets past sign-in can still see 'download: empty songList', or a loop of 'Refreshing Apple ID session' ending in 're-auth: ...' or 'exhausted retries', with no explanation.
  cause: Verified in the code: every call goes through volumeStoreDownloadProduct (internal/appstore/download.go:130-198@51ed4f3), and the payload has no serialNumber (`git grep serialNumber` returns nothing on korboy, londek or r0gueee). 5002 is mapped to ErrPasswordTokenExpired (download.go:181-186), which makes WithAuthAccount re-login and retry (internal/appstoreworkflow/auth.go:99-128@51ed4f3). Inferred from ipatool PR #500/5f776fe, ipatool 3411d57/30b2909 and londek #46: Apple now needs serialNumber=0 and, for some builds, kbsync or redownload fallbacks.
- [major] **Errors are vague and the cause is hidden or never logged** — The final line is just 'FAILED: appstore helper exit 1' (or 'helper exit N', or 'connect /var/jb/var/run/ipadecryptd.sock errno=61'). The real reason is an earlier status row or missing entirely, and app.log has neither the exit code nor the failure reason, so a bug report needs both the log file and a copied screen.
  cause: Verified:
- Exit code 1 covers network, Apple rejection, storefront, empty songList, appinst missing, purchase and config errors (cmd/ipadecrypt-appstore-helper/main.go:120-131@51ed4f3).
- The app replaces the message with 'appstore helper exit %d' whenever it contains the last stderr line (app/Views/IDHomeViewController.m:813-817, 976-980@51ed4f3).
- IDAppStoreHelperRunner.m:191-196@51ed4f3 writes only non-@evt stdout lines to app.log, so `@evt phase=failed reason=...` is never logged; the exit code isn't logged either (lines 223-239).
- IDIsExpectedAuthStderr filters 'Apple ID sign-in required' (IDHomeViewController.m:48-51), but the helper prints 'sign in with Apple ID required' (main.go:26), so that raw line leaks into the status view.
- The daemon path reports raw errno values (app/Services/IDHelperRunner.m:82-88@51ed4f3).
- [major] **No timeouts, no Cancel, and a progress screen you can't get back to** — One stalled Apple or CDN request leaves the spinner running forever, and every later sign-in, refresh or version-list request waits silently behind it with the auth button greyed out. The only way out is killing the app. Pressing Back leaves the job running invisibly, and its result is lost.
  cause: Verified:
- The http.Client has no Timeout (internal/appstore/client.go:31-40@51ed4f3).
- All App Store helper runs, including multi-GB download+install, go through one serialized queue (app/Services/IDAppStoreHelperRunner.m:59-104, 288@51ed4f3), and nothing shows that a job is queued.
- There is no kill/terminate anywhere in app/ (git grep), and the progress VC has no cancel item (app/Views/IDDecryptProgressViewController.m:44-49@51ed4f3).
- The daemon waits on the helper with no deadline (cmd/ipadecryptd/main.go:111-113@51ed4f3).
- [major] **'Latest from App Store' and 'Decrypt this version' replace the installed app without asking, and may install a build that can't run** — The user's working installed app is overwritten by the downloaded build. That build may be a version that needs a newer iOS, or an iPad-only app, because its MinimumOSVersion and UIDeviceFamily were patched so appinst accepts it. It may crash at launch or misread data written by a newer version. Only the picker's 'Install this version' shows a warning.
  cause: Verified:
- The option handler goes straight into the install flow (IDHomeViewController.m:450-458@51ed4f3).
- The picker's 'Decrypt this version' has no confirmation (app/Views/IDAppStoreVersionPickerViewController.m:291-299@51ed4f3); only 'Install this version' warns (273-276).
- main.go:235-252@51ed4f3 patches to the device's iOS and family, then runs appinst.
- internal/pipeline/patch.go:193-208@51ed4f3 lowers MinimumOSVersion and expands UIDeviceFamily.
- There is no compatibility check before the download.
- Inferred, not observed: that the patched app crashes on older iOS.
- [major] **Missing prerequisites are discovered late, or not at all** — Installing from Sileo pulls in nothing. The user downloads hundreds of MB, then gets 'appinst not found; install AppSync/appinst first'. Older-version installs stall on 'waiting for App Store confirmation...' unless the separate autoalert tweak is installed. The folder button shows 'Filza unavailable' with no explanation if Filza is absent.
  cause: Verified:
- app/control@51ed4f3 has no Depends, and neither does korboy's live Sileo repo index (korboybeats.github.io Packages: com.korboy.ipadecrypt 0.7.3-korboy.2 has no Depends line; only autoalert depends on mobilesubstrate).
- The appinst check comes after fetchEncrypted (cmd/ipadecrypt-appstore-helper/main.go:225-249@51ed4f3).
- The alert has no message (IDHomeViewController.m:277-285@51ed4f3).
- The README lists AppSync/appinst only for the CLI path (README.md 'Requirements'@51ed4f3).
- [major] **'Latest iOS-compatible' (StoreKit) can decrypt the old build and report success, and times out on big apps** — If the app is installed and the App Store takes more than about 15 s to fetch the update, ipadecrypt decrypts the old installed version and reports success; the version in the filename is the only clue. Large apps on slow Wi-Fi fail with 'timed out waiting for install' after 3 minutes, and the screen doesn't say the App Store download continues or that 'Decrypt installed' can be used afterwards.
  cause: Verified by reading the code; that iOS keeps the old bundle in place during an update is inferred:
- pollInstallChange returns the current app once it has been 'ready and unchanged' for 15 s (IDHomeViewController.m:1181-1188@51ed4f3), even while an update is still downloading.
- The deadline is a fixed 180 s regardless of size (IDHomeViewController.m:1080).
- The timeout message comes from IDHomeViewController.m:1082-1087.
- [major] **Search and lookup are hard-coded to the US store** — Non-US users don't find region-only apps in search. For installed apps not sold in the US, trackID is 0, so the 'Latest iOS-compatible' option silently disappears. Results can also differ from what the helper later downloads with the account's storefront.
  cause: Verified: IDITunesSearch search and lookup use country:@"US" (IDHomeViewController.m:338, 399@51ed4f3). A failed lookup passes result.trackID=0 (398-406), and IDDecryptOptionsViewController.m:34@51ed4f3 shows the StoreKit option only when trackID > 0. The helper uses the account storefront (cmd/ipadecrypt-appstore-helper/main.go:306-317@51ed4f3).
- [major] **Account state is opaque: no visible sign-in, sign-out or account switch, and 'green' doesn't mean working** — Sign-in hides behind an unlabeled coloured circular-arrow icon. Green only means config.json has fields, so a dead token still shows green. The app has no way to see which Apple ID is used, sign out, or switch. Changing accounts means deleting /var/mobile/Documents/ipadecrypt/config.json in Filza. Nothing in the app says the Apple ID must be the device's own App Store account (only the CLI wizard says so).
  cause: Verified:
- The auth button is in IDHomeViewController.m:119-124 and its colour states at 152-174@51ed4f3.
- --auth-status only checks config fields (cmd/ipadecrypt-appstore-helper/main.go:134-163@51ed4f3).
- The Settings sheet holds only the Filza toggle (IDHomeViewController.m:241-256).
- The same-account requirement appears only at cmd/ipadecrypt/bootstrap.go:40@51ed4f3.
- Inferred: an account mismatch makes the installed app fail FairPlay at launch, so decryption fails.
- [major] **2FA and re-auth are fragile** — A mistyped or expired 2FA code gives 'login failed' (exit 1) and no new prompt. An expired token mid-download re-logs in with no code, so a 2FA account gets a prompt in the middle of an operation or a failure. After signing in inside the version picker, every metadata tap or refresh does a full sign-in, which risks 429s and repeated 2FA prompts.
  cause: Verified:
- BadLogin maps to ErrAuthCodeRequired only when authCode is empty (internal/appstore/login.go:140-158@51ed4f3).
- Reauth uses the saved password and an empty code (internal/appstoreworkflow/auth.go:38-48@51ed4f3).
- The picker keeps email and password (IDAppStoreVersionPickerViewController.m:340-351@51ed4f3) and passes them on every call (172-178, 55).
- prepareAppStore logs in whenever any credential is present (cmd/ipadecrypt-appstore-helper/main.go:290-299@51ed4f3).
- The code is only stripped of spaces, with no 6-digit check (ipatool 735b689 adds one).
- [major] **Secrets handled in plaintext; SSH host keys never checked** — The Apple ID password is visible in `ps` while the helper runs and is stored in plaintext on the phone and the desktop. The device SSH/sudo password is stored in plaintext on the desktop. On Wi-Fi, the CLI will talk to any machine answering at the phone's IP.
  cause: Verified:
- --password goes on argv (app/Services/IDAppStoreHelperRunner.m:281-283, 308-310, 339-341, 358-360@51ed4f3).
- config.Apple.Password is stored plainly (internal/config/config.go:50@51ed4f3).
- The device Auth.Password is stored plainly (config.go:88).
- HostKeyCallback is ssh.InsecureIgnoreHostKey() (internal/device/client.go:43@51ed4f3), although an AcceptNewHostKey setting exists (config.go:83, bootstrap.go:30) and is never read.
- [major] **Root daemon runs any client-supplied binary, and its lifecycle isn't cleaned up** — Any local process can ask ipadecryptd to run an arbitrary path as root. After an uninstall or a package swap (for example to the merged fork), the old supervisor can stay loaded, keep restarting, and fight the new daemon.
  cause: Verified:
- The socket is chmod 0666 (cmd/ipadecryptd/main.go:31@51ed4f3).
- The first request line becomes the executable (main.go:53, 79).
- The supervisor runs `killall ipadecryptd` on any restart (app/layout/usr/libexec/ipadecryptd-supervisor:39-46@51ed4f3).
- The package has only a postinst (app/layout/DEBIAN/postinst@51ed4f3), with no prerm or postrm.
- Inferred: the effects of leftover daemons on upgrades.
- [major] **No storage management, and a failed patch poisons the cache** — Each App Store run can use about 3-4x the app's size: the encrypted cache, a patched copy, the installed app and the decrypted IPA. Nothing is ever cleaned up, and the app never checks free space. If writing the patched cache file fails (disk full, crash), the next run says 'Using cached IPA' and installs the corrupt file, failing with an obscure appinst or zip error.
  cause: Verified:
- The cache /var/mobile/Documents/ipadecrypt/cache/<bundle>_<version>.ipa is never evicted (internal/config/paths.go:43-50; cmd/ipadecrypt-appstore-helper/main.go:444-453@51ed4f3).
- applyPatches opens the final path with O_TRUNC and doesn't remove it on error (internal/appstore/download.go:345-350@51ed4f3).
- The cache hit is just fileExists (main.go:450-452, 477-479).
- No statfs or free-space check exists in app/, cmd/, internal/ or helper/ (git grep).
- [major] **Missing upstream decrypt fixes; unversioned helper binaries** — On arm64e (A12+) devices, decrypting frameworks that only app extensions link can crash the inject pass (PAC fault), and slow framework loads time out after 3 s, leaving images encrypted. If a bug is reported, no one can tell which helper build was used.
  cause: Verified:
- helper/inject.c:21@51ed4f3 still has the non-canonical 0x4141414141414141 sentinel and inject.c:835 the 3000 ms wait; londek fixed both in 0a0bf0d (#42) and 0d34f0b (#39).
- app/Resources/helper.arm64 and internal/device/ipadecrypt-helper-arm64 are different blobs (5cd2231 vs fd9bc3b), and both print 'ipadecrypt-helper (dev)' (per the idecrypt-merge report).
- [minor] **Long jobs aren't protected from screen lock or backgrounding** — If the screen auto-locks or the user switches apps during a long download or install, the app is suspended. The helper may stall once its stdout pipe fills, or die if iOS kills the app. The user comes back to a frozen or failed screen with no explanation.
  cause: Verified: there is no beginBackgroundTask, idleTimerDisabled or UIBackgroundModes in app/ (git grep), and helper events are read through pipes owned by the app process (IDAppStoreHelperRunner.m:135-221@51ed4f3); download progress is emitted about every 100 ms (internal/appstore/download.go:322-331). The stall or SIGPIPE behaviour is inferred. Downloads do resume from .tmp on retry (download.go:253-257).
- [minor] **Finding and managing outputs needs Filza** — After leaving the progress screen, the app gives no way to list, share or delete earlier decrypted IPAs. The folder button only opens Filza. The Files app can't browse /var/mobile/Documents/ipadecrypt.
  cause: Verified: no file-list view exists (app/Views contains only Home, DecryptOptions, DecryptProgress, VersionPicker and AppRowCell@51ed4f3); Share exists only on the finished progress screen (IDDecryptProgressViewController.m:83-88, 121-128), and the folder button only opens Filza (IDHomeViewController.m:258-288). The Files-app limitation is inferred.
- [minor] **Log hygiene: no date, no rotation, and paths built three different ways** — app.log grows forever and its lines carry only HH:mm:ss, so separate days can't be told apart. Support instructions differ between rootless and roothide because log, helper state and output paths are computed differently.
  cause: Verified:
- Logging.m:3@51ed4f3 hard-codes /var/mobile/Documents/ipadecrypt/logs/app.log, with format HH:mm:ss.SSS (Logging.m:8) and append-only writes.
- The output folder uses IDUserDocumentsPath, which switches to /rootfs on roothide (app/IDJailbreakPaths.m:19-27).
- The helper uses defaultRootDir /var/mobile/Documents/ipadecrypt (cmd/ipadecrypt-appstore-helper/main.go:22).
- [minor] **Version picker exposes internals** — Rows show raw external version IDs, 'tap to fetch details' for every row after the third, and 'devices 1,2' (raw UIDeviceFamily numbers). Each tap starts a full helper run, and nothing shows whether a version will run on this iOS.
  cause: Verified: metadata is fetched only for the first 3 IDs (cmd/ipadecrypt-appstore-helper/main.go:364@51ed4f3). The row rendering is IDAppStoreVersionPickerViewController.m:229-249@51ed4f3, and taps go to fetchMetadataForVersion (256-262).
- [minor] **TrollStore isn't actually supported, though the bundle advertises it** — A TrollStore (.tipa) install, or any setup where the daemon isn't loaded, fails at decrypt with 'connect .../ipadecryptd.sock errno=2'.
  cause: Verified: Info.plist still lists TSRootBinaries (app/Resources/Info.plist:39-42@51ed4f3), but decrypt only goes through the daemon socket (app/Services/IDHelperRunner.m:9-11, 69-89@51ed4f3). The persona-spawn path was removed in 9102126.
- [minor] **FairPlay and iOS limits aren't detected up front** — Apps that ship only the new SC_Info format (londek #34), and Apple's own apps on A12+ (PPL), fail mid-decrypt with generic per-image errors. Paid apps give 'purchase: paid apps are not supported', which doesn't say that buying the app with this Apple ID first would work. Arcade titles aren't retried.
  cause: Verified: the limits appear only under README.md 'Known issues'@51ed4f3; purchase.go:18-21@51ed4f3 rejects price > 0, and despite the comment at line 17 only STDQ is tried (line 28). What the user sees for the SC_Info case is inferred.
- [minor] **CLI onboarding friction and stale doctor hints** — The README tells users to clone and `go build`, although prebuilt binaries exist in every release. The CLI needs OpenSSH, the phone's Wi-Fi IP and a mobile/sudo password; there's no USB option. On roothide, `doctor` points to a non-existent 0.0.1 deb and /var/jb paths, and its 'App Store session' check passes on stale fields.
  cause: Verified:
- README.md 'Install' section@51ed4f3; `gh release view v0.7.3-korboy.2` lists darwin, linux and windows binaries.
- No usbmux or iproxy code exists (git grep).
- cmd/ipadecrypt/doctor.go:432 (stale deb name), 439-447 (/var/jb hard-coded) and 203-215 (field-presence session check)@51ed4f3.
- [minor] **Machine GUID can be a redacted, shared MAC** — Today this does little harm. Once SAP signing is added, a 02:00:00:00:00:00 or per-network private MAC could give every user the same machine ID, or a changing one. The user would see repeated 2FA prompts or 'account disabled'.
  cause: Verified: guid() takes en0, or the first interface with any MAC, and doesn't validate it (internal/appstore/client.go:46-74@51ed4f3); on the macOS 27 desktop it produced guid=020000000000 (auth-trace and ipatool-diff reports). What iOS returns to the root helper is unknown. ipatool rejects such MACs (50312a9, 0cc226a).
- [major] **The iDecrypt engine being folded in fails silently, and can kill itself** — trolldecryptjb or iDecrypt can report success while frameworks or extensions are still encrypted. An extension that fails to launch can make the tool vanish with no message, because kill(0, SIGKILL) signals its own process group. Progress lines in the CLI show raw localization keys.
  cause: Verified at iDecrypt 3a60fe8:
- NIL_LAUNCHD_RESPONSE sets pid=0 (Decryption/LaunchdResponse.h:21), but TDDecryptionTask.m:170 checks only for -1, and line 184 then calls kill(pid, SIGKILL) unconditionally.
- Framework and extension failures are only NSLogged (TDDecryptionTask.m:157-159, 180-182), and no verify step follows.
- The raw-key output is from the idecrypt-merge report.
- GitHub reports no license for TrollDecrypt, so copying its code is legally unclear.
- [minor] **Package identity collisions across forks** — R0GUEEE's 0.7.8-korboy.1 and korboy's 0.7.3-korboy.2 share the package ID com.korboy.ipadecrypt. Whichever repo has the higher version wins in Sileo, so a user can get silently 'upgraded' to another fork, or see a real update as a downgrade.
  cause: Verified: r0gueee/main:app/control@4be5fc2 keeps Package com.korboy.ipadecrypt and Maintainer Korboy, as does korboy's app/control@51ed4f3. Legacy IDs com.korboy.ipadecrypt-app and com.korboy.ipadecryptautoalert still exist (v0.5.6-korboy.1:app/control; per the idecrypt-merge report).

## Improvements
- [must/large] **Port ipatool v2.7.0's SAP-signed login into the shared Go client (desktop and on-device)** — Replace internal/appstore login with ipatool v2.7.0's flow:
- Read sign-sap-setup, sign-sap-setup-cert and sign-sap-version (must be 200) from the bag.
- Accept only buy./*-buy MZFinance hosts, and force the `/WebObjects/MZFinance.woa/wa/authenticate/` path.
- Sign every authenticate body with X-Apple-ActionSignature.
- Never auto-follow auth redirects: re-validate the Location and replay the same signed body with the same attempt value.
- Delete the native/fast fallback.
Build Unicorn for the on-device helper the way ipatool's tools/build-ios.sh does: a static libunicorn.a with the W^X patch, plus -force_load and -export_dynamic in app/build.sh:52-57.
Show first-run progress ('Preparing Apple sign-in, one-time ~38 MB download, can take a few minutes') as @evt steps, so the slow first login doesn't look like a hang. Pin ipatool v2.7.0; main's iOS build has an arm64/x86 mismatch.
  addresses: App Store sign-in cannot succeed: the authenticate request is unsigned; Machine GUID can be a redacted, shared MAC
  notes: Recommended route (inferred): a hybrid that uses ipatool pkg/appstore (or a copy of internal/sap with library_ios.go) for Login only, and keeps ipadecrypt's ticket-based download. londek's port (126cc67, a80aa26) can't run on iOS (no ios case in artifact.go), still has the 10 s guest limit (machine.go:434@2fa6924), and loses the auth URL after a -5000 retry (login.go:102@2fa6924).

Licensing: Unicorn is GPL-2.0 and is linked statically next to Apache-2.0 purego.

Device checks, before writing code:
- Run the upstream ipatool-2.7.0-ios-arm64 `auth login` on the phone (this contacts Apple; it's the user's call). Time it, and watch for 'unicorn is not linked', 'protect Unicorn translation buffer', or a JetsamEvent .ips.
- Report `ifconfig en0 | grep ether` as root, and whether Private Wi-Fi Address is on.
- Report free space, and where HOME/os.UserCacheDir resolves for the helper the app spawns.
- Frida, observe only: in appstored/itunesstored, hook -[NSMutableURLRequest setValue:forHTTPHeaderField:] and look for X-Apple-ActionSignature. Also check for AMSMescal* classes, a possible native-signer alternative.
- [must/medium] **Fix the download path: serialNumber, a correct 5002 meaning, and fallbacks** — Add "serialNumber":"0" to every volumeStoreDownloadProduct payload (download, list, metadata), as ipatool does in 5f776fe. Stop treating 5002 as an expired token in volumeDownload (download.go:181-186), which causes a re-login loop. Return a specific 'Apple requires an extra check for this app' error instead. Next, add the redownloadProduct and updateProduct fallbacks (ipatool 30b2909), and later ent/download with kbsync (3411d57), for the 'empty songList' cases.
  addresses: After login is fixed, downloads, version lists and metadata may still fail
  notes: serialNumber is a small change. kbsync needs the emulated StoreAgent, which is heavier, so defer it. Device check: once a signed build exists, run 'Latest from App Store' for one popular app (Google or Microsoft) and one ordinary app, and capture the stderr lines.
- [must/small] **Make an Apple ID optional everywhere, and offer the working paths when sign-in fails** — In the CLI, merge londek 8b02861: `bootstrap --skip-login`, and let decrypt of installed apps or local .ipa files work without cfg.Apple. Also let bootstrap continue past a failed login with a warning instead of aborting at bootstrap.go:48-52. In the app, when the helper fails with signature-required, network or rate-limited, show an action sheet: 'Apple sign-in is unavailable right now. You can still: Decrypt installed build / Latest iOS-compatible (uses your device's App Store account) / Import an .ipa'. Add 'Import .ipa' through the document picker or Files, then decrypt via the existing appinst+decrypt path, matching the community workaround in londek #46/#50: download with ipatool elsewhere, then decrypt locally.
  addresses: Desktop CLI is fully blocked when App Store login fails, even for jobs that need no Apple ID; App Store sign-in cannot succeed: the authenticate request is unsigned
  notes: Cheapest unblocker, and independent of the SAP work. merge-tree shows bootstrap.go and main.go conflict in only one hunk each (londek-upstream report). Device check: does 'Latest iOS-compatible' still install apps today, and does 'Decrypt installed' work? Both are inferred to be unaffected by the login breakage.
- [must/medium] **An error taxonomy with plain-English, actionable messages** — Give each failure class its own exit code and reason:
- signature-required / apple-rejected (reuse R0GUEEE's exit 22, reworded so it doesn't assume iOS)
- network / timeout
- rate-limited (with the Retry-After value)
- 2fa-invalid
- account-disabled (mention that wrong passwords also produce it, and include Apple's unlock link)
- storefront-missing
- download-blocked (5002 / empty songList)
- appinst-missing
- out-of-space
- daemon-unavailable
- verify-failed / partial
In the app, make the `@evt phase=failed message` the headline, add a one-line 'what to do', and put 'appstore helper exit N' plus stderr in a collapsible detail. Remove the replacement at IDHomeViewController.m:813-817 and 976-980. Fix the IDIsExpectedAuthStderr string mismatch. Map daemon errnos (ENOENT/ECONNREFUSED on ipadecryptd.sock) to 'Decrypt service isn't running: reinstall ipadecrypt or respring'. Never pass HTML or empty bodies to the plist parser; report HTTP status, content type and length, as ipatool's fd63034/d1845ba do.
  addresses: Errors are vague and the cause is hidden or never logged; FairPlay and iOS limits aren't detected up front
  notes: Keep the stable reason strings machine-readable for the diagnostics bundle. R0GUEEE's signatureRequiredError (login.go:11-38@4be5fc2) is a starting point, but its message is iOS-specific and it guesses on any 401/403/404.
- [must/medium] **An on-device 'Diagnostics' screen with a one-tap redacted report** — Add Settings > Diagnostics showing:
- app, helper, appstore-helper and daemon versions (stamp each with `git describe`)
- iOS version and build, device model and SoC, jailbreak type (rootless/roothide via jbroot)
- daemon loaded and socket mode
- AppSync/appinst, autoalert and Filza present
- free space
- auth: 'saved' vs 'verified at <time>' vs 'failed: <reason>'
- GUID source, flagged when it is 02:00:00:00:00:00
- the last 5 operations with their reason codes
'Share diagnostics' should zip app.log, the daemon logs, the progress transcript and this summary, with emails, tokens, DSIDs and cookie names redacted. Also:
- log every @evt line (redacted) plus the helper exit code to app.log;
- add the date to log lines;
- rotate at about 2 MB;
- route all paths through one helper.
Give the CLI `doctor` the same checks: replace the field-presence session check, the stale 0.0.1 deb hint and the hard-coded /var/jb.
  addresses: Errors are vague and the cause is hidden or never logged; Log hygiene: no date, no rotation, and paths built three different ways; CLI onboarding friction and stale doctor hints; Account state is opaque
  notes: The doctor's live session check should use a cheap authenticated call, or just the result of the last real operation; it shouldn't do a full login. Device questions this screen would answer automatically: iOS version, jailbreak type and version, `dpkg -l | grep -E 'korboy|rtrdd|34306'`, `launchctl print system/com.korboy.ipadecryptd`, and the socket mode.
- [should/medium] **Timeouts, Cancel, and a reachable activity view** — Add per-request timeouts: 15 s dial and 30 s total for auth with transient retries and Retry-After (ipatool 30d19a6/e258fed), and an idle-read timeout on the CDN fetch. Add a helper watchdog in the runner and the daemon. Put a Cancel button on the progress screen that sends SIGTERM to the helper; the daemon needs a 'cancel' request that kills its child. Show 'Waiting for the previous App Store task...' when a job is queued behind the serialized auth queue. Keep running jobs in an 'Activity' list on the home screen so Back doesn't orphan them.
  addresses: No timeouts, no Cancel, and a progress screen you can't get back to
  notes: Downloads already resume from .tmp (download.go:253-257@51ed4f3), so cancel-then-retry is safe for the download step. Make sure cancel during appinst can't leave a half-installed app; inferred risk, so test it on the device.
- [should/small] **Safe-install guardrails and a compatibility preview** — Before any path that replaces the installed app ('Latest from App Store', 'Decrypt this version'), show the same confirmation that 'Install this version' has. Include current and target version, 'app data is kept but may be incompatible', and, when the target's MinimumOSVersion is above the device's iOS, 'this version needs iOS X; it will be patched to install and may not launch (decryption usually still works)'. Default the home action to the newest build that is compatible with this iOS. Show the device family as 'iPhone / iPad' in the picker, and fetch details automatically as the user scrolls instead of 'tap to fetch details'.
  addresses: 'Latest from App Store' and 'Decrypt this version' replace the installed app without asking, and may install a build that can't run; Version picker exposes internals
  notes: GetVersionMetadata already returns SupportedDevices. MinimumOSVersion would need to come from the lookup or the downloaded Info.plist (internal/appstore/info_plist.go). Inferred: whether a downgrade via appinst keeps the data container. Check on the device with a throwaway app.
- [should/small] **Declare dependencies and check prerequisites before downloading** — Add Depends or Recommends to the deb control for AppSync Unified and appinst, plus uikittools for uicache. Make the autoalert tweak a Recommends, or fold it in behind a toggle. Before any App Store download, the helper should check appinst and AppSync, free space, and daemon reachability, and fail fast with 'Install AppSync Unified and appinst from https://lukezgd.github.io/repo'. Add an in-app 'Decrypted IPAs' list with Share, Delete and 'Open in Filza', so Filza becomes optional.
  addresses: Missing prerequisites are discovered late, or not at all; Finding and managing outputs needs Filza
  notes: The exact Procursus/lukezgd package IDs for appinst and AppSync are unverified here. Read them from the repo's Packages file before hard-coding Depends. Device check: `dpkg -l | grep -i -E 'appsync|appinst|filza|ellekit|substrate'`.
- [should/medium] **Make the StoreKit path correct: track the real download, not a 15 s guess** — Replace the 'ready and unchanged for 15 s means already latest' heuristic (IDHomeViewController.m:1181-1188) with a real check. Before starting, compare the installed version with the lookup's latest-compatible version; if they match, decrypt immediately. Otherwise wait for an actual bundle or version change, observing SKUIItemStateCenter or appstored progress if it's exposed. Scale the deadline with file size, or drop it while progress is still moving. On timeout say 'The App Store is still downloading. When it finishes, tap the app and choose Decrypt installed build.' Never decrypt a version older than requested without saying so.
  addresses: 'Latest iOS-compatible' (StoreKit) can decrypt the old build and report success, and times out on big apps
  notes: Device check, Frida: hook SKUIItemStateCenter's state-change callbacks during a 'Latest iOS-compatible' run of a large app that has a pending update. Confirm the old bundle path stays in place until the install phase; that's inferred, not verified.
- [should/small] **Search and lookup in the user's storefront** — Use the signed-in account's storefront (CountryCodeFromStoreFront), or the device App Store's country when signed out, for IDITunesSearch search and lookup instead of 'US'. Add a country picker in Settings. If lookup fails, fall back to the helper's lookup so trackID (and the StoreKit option) is still found.
  addresses: Search and lookup are hard-coded to the US store
  notes: storefronts.go already maps storefront IDs to country codes. Device question: which App Store country is the phone's account on?
- [should/small] **Explicit account management and session reuse** — Settings > Apple ID should show 'Signed in as j***@... (store: DE), last verified 10:42', with Sign out (clear config and cookies), Switch account and Check now. Label the toolbar auth button, or replace it with a status chip. Explain in the app that the Apple ID must be the one signed into the device's App Store (FairPlay). For 2FA:
- validate the 6 digits and accept pasted codes;
- when a code is rejected, say 'Apple didn't accept that code, request a new one' and prompt again (ipatool 735b689);
- prompt inline when Reauth needs a code mid-operation, then resume.
In the version picker, drop the stored email and password after the first successful sign-in so later taps reuse the session.
  addresses: Account state is opaque: no visible sign-in, sign-out or account switch, and 'green' doesn't mean working; 2FA and re-auth are fragile
  notes: Also stop SetAccount from overwriting a non-empty email or storefront with empty values, and fail loudly instead (ipatool v2.7.0 appstore_login.go:197-200), so a 'successful' login can't leave config that looks signed-out.
- [should/medium] **Credential hygiene, with an optional saved password** — Pass the password and 2FA code to appstore-helper on stdin or an inherited pipe fd, never argv. Make 'Remember Apple ID password' an explicit opt-in, in the spirit of ipatool 6baffed (2026-10-18), which made the file keychain's passphrase optional with clear errors. If it isn't saved, keep only passwordToken and cookies. When Reauth needs the password, return a clear 'Your Apple session expired; enter your password to continue' (exit 20) instead of a vague failure. Store it in the keychain where possible. In the CLI, implement trust-on-first-use host keys through the existing AcceptNewHostKey field, and keep the SSH password optional when a key is used.
  addresses: Secrets handled in plaintext; SSH host keys never checked; 2FA and re-auth are fragile
  notes: Silent re-auth currently depends on the saved plaintext password (internal/appstoreworkflow/auth.go:21-48@51ed4f3). Making it optional is safe only once 'session expired' errors are clear and re-prompt.
- [must/small] **Harden the daemon and give the package clean install and uninstall scripts** — In ipadecryptd:
- run only the fixed, packaged helper path and ignore the client-supplied first line, or allow-list it;
- create the socket 0660 with group mobile;
- check the peer's uid with getpeereid.
Add a preinst that boots out com.korboy.ipadecryptd and any old supervisor, a prerm that unloads the daemon and supervisor and removes the socket, and a postrm that runs `uicache -u`. Make the supervisor kill only its own PID, not `killall ipadecryptd`.
  addresses: Root daemon runs any client-supplied binary, and its lifecycle isn't cleaned up
  notes: This is a local root escalation (verified at cmd/ipadecryptd/main.go:31,53,79@51ed4f3), so fix it before publishing a merged package. Device check after the upgrade: `launchctl print system/com.korboy.ipadecryptd`, `ps -axo pid,user,command | grep ipadecryptd`, `ls -la /var/jb/var/run/ipadecryptd.sock`.
- [should/small] **Storage preflight, atomic cache writes and a cache cleaner** — Before downloading, check free space against about 3x the ticket's FileSize, which covers the cache, the patched copy and the decrypted output, and fail with 'Need 2.4 GB free, you have 900 MB'. Write applyPatches to a temp name and rename it on success, so a failed write can never be reused as a cached IPA. Then:
- delete the encrypted cache after a successful install and decrypt, by default;
- add Settings > Storage with 'Clear download cache (X GB)' and 'Delete decrypted IPAs';
- show a per-file list.
  addresses: No storage management, and a failed patch poisons the cache
  notes: The poisoned cache is verified in the code (download.go:345-350, main.go:450-452/477-479@51ed4f3), and the fix is a few lines.
- [nice/medium] **Run long jobs in the daemon so they survive app suspension** — Move App Store download and install into ipadecryptd, or run them under the daemon. The app would only subscribe to progress over the socket and could reattach after a relaunch. Until then, wrap runs in beginBackgroundTask, set idleTimerDisabled while a job is in the foreground, and post a local notification on completion or failure.
  addresses: Long jobs aren't protected from screen lock or backgrounding; No timeouts, no Cancel, and a progress screen you can't get back to
  notes: Stalls on a suspended pipe reader are inferred. Device check: start a large download, lock the phone for 2 minutes, unlock, and see whether progress resumed or the helper died. Check app.log and `ps`.
- [should/small] **Merge the upstream decrypt fixes and build one versioned helper in CI** — Cherry-pick londek 0a0bf0d (PAC-safe inject sentinel) and 0d34f0b (15 s dlopen timeout), keeping their authorship. Add `#include <stddef.h>` to helper/macho.c so it builds without Docker. Build one helper in CI, stamp HELPER_VERSION, and copy the same bytes into internal/device (go:embed) and the deb; stop committing two different blobs. Before decrypting, detect binaries that ship only the new SC_Info format (londek #34) and say 'This app's DRM format needs iOS 26+ to decrypt'.
  addresses: Missing upstream decrypt fixes; unversioned helper binaries; FairPlay and iOS limits aren't detected up front
  notes: Device questions: device SoC (arm64 vs arm64e decides whether the PAC fix matters), and whether any past decrypt logged 'target.inject' errors for frameworks only extensions use.
- [should/large] **Fold iDecrypt in as one engine and one package, not two** — Make helper/ the only decrypt core. Add `ipadecrypt-helper dump <bundle-id|name> [out-dir]` and `list`, plus a trolldecryptjb symlink for compatibility. Always verify, and use exit codes 0 for all decrypted, 3 for partial with images named, 1 for failure. Write a launchd/RunningBoard launch strategy from scratch for TrollStore and extensions; don't copy TrollDecrypt code, which has no license. If iDecrypt's engine is kept even briefly, fix the pid==0 check before kill() and fail on skipped images. Ship under a new package ID that Conflicts with and Replaces com.korboy.ipadecrypt, com.korboy.ipadecrypt-app, com.korboy.ipadecryptautoalert, dev.rtrdd.idecrypt and com.34306.trolldecryptjb. Use one output folder, listed in the app.
  addresses: The iDecrypt engine being folded in fails silently, and can kill itself; TrollStore isn't actually supported, though the bundle advertises it; Package identity collisions across forks
  notes: Merge plan details are in the idecrypt-merge report. Decide whether the .tipa/TrollStore target is in scope; if not, skip the launchd strategy and remove TSRootBinaries.

Device checks:
- `dpkg -S /var/jb/usr/local/bin/trolldecryptjb`.
- Does trolldecryptjb work as mobile or only as root?
- Run `otool -l | grep cryptid` on every Mach-O of a recent iDecrypt output.
- Frida probe: `kill` arguments and `_launch_job_routine` results while decrypting an app with extensions.
- [nice/medium] **Smoother desktop CLI onboarding** — Point the README's Install section at the release binaries and `ipadecrypt update`, keeping the Go build as an alternative. In the bootstrap wizard, explain how to set the mobile password on Dopamine or palera1n and suggest SSH keys. Add optional USB transport (usbmux or iproxy autodetect) so a Wi-Fi IP isn't needed.
  addresses: CLI onboarding friction and stale doctor hints
  notes: The default-password behaviour on Dopamine and palera1n is from memory, unverified; check it before writing wizard text.
- [should/small] **Stable, validated machine identity** — Pick the GUID deterministically and persist it in config. Reject 00:00:00:00:00:00, 02:00:00:00:00:00 and multicast addresses, as ipatool does in 50312a9 and 0cc226a. On iOS, if no real MAC is visible, generate a random locally-administered 6-byte ID once and keep it. Show the redacted GUID in Diagnostics, and warn when it changes between sign-ins.
  addresses: Machine GUID can be a redacted, shared MAC
  notes: Needed together with SAP, because the SAP HardwareID is derived from the same bytes. Device check: `ifconfig en0 | grep ether` as root and as mobile, with Private Wi-Fi Address on and off.
- [nice/medium] **Offline regression tests for the failure modes users hit** — Add fake-transport tests for:
- 403, 204, 301 without Location, 404 HTML and 429 responses;
- pod 302 replay with the same attempt value;
- the slashed path and the signer header;
- LoginAndSave not wiping the email or storefront;
- 5002 and empty-songList mapping;
- an atomic cache write after a simulated failure;
- the helper's exit-code and reason table.
Retire the native/fast tests (auth_endpoint_test.go, login_test.go:27-72@51ed4f3). Make CI fail if a committed helper binary differs from the CI build.
  addresses: App Store sign-in cannot succeed: the authenticate request is unsigned; Errors are vague and the cause is hidden or never logged; No storage management, and a failed patch poisons the cache
  notes: The investigators already wrote throwaway versions of these tests (zz_trace_test.go and the ipatool wire tests), so this is mostly turning them into permanent files. Run them with HTTPS_PROXY=127.0.0.1:9 so they can't reach Apple.

## RESOLVED: partial decrypts were a regression from the ipatool download (2026-10-18)

**Root cause (verified):** `Client.Download` called ipatool's `Download` but
not `ReplicateSinf`. ipatool returns the FairPlay licenses (sinfs) without
writing them into the package; its own CLI does that as a separate step. So
every IPA downloaded through the new path had **no SC_Info/<exec>.sinf**.
With no license the kernel can't decrypt the binary at exec, so:
- SpringBoard "launched" the app (SBSLaunchApplicationWithIdentifier returned
  0) but no process ever ran;
- the helper fell back to its ptrace path, whose vm_read of the still-
  undecryptable images failed (kr=2), giving 1/8 decrypted.

Found by comparing SC_Info across installed apps: every working App Store app
had <exec>.sinf; only the two downloaded by the new path (YouTube, Claude) did
not. Fixed in commit 0dc0ee5 (calls ReplicateSinf, errors if Apple returns no
license), with a regression test.

**Result on the device:** YouTube 21.41.5 (min iOS 17) on iOS 16.5.1 now
launches via SBS and decrypts 7 of 8 Mach-Os (main binary 180 MB, widevine
framework, 5 extensions).

**Corrections to earlier notes:** the Mach-O min-OS was NOT the gate (that
patch was tested, didn't help, and was reverted), and the ptrace fallback is
NOT structurally broken -- with a license present it decrypts extensions fine.
The decrypt engine was never the problem; korboy's approach does handle
higher-iOS apps, as the user said.

**Remaining, minor:** `Extensions/AppMigrationExtension.appex` stays
encrypted. It is built for iOS 26.1 (minos 26.1), uses the iOS-26-only
`com.apple.app-migration` extension point and links AppMigrationKit, so it
can't run on iOS 16.5.1 and goes through the ptrace path without dyld
patches. On iOS 16 that extension point doesn't exist, so it would never run.
Unverified whether korboy's original decrypted such extensions.
- Possible QoL: report it as "skipped: requires iOS 26" instead of a generic
  verify failure, or offer to strip extensions the device can't run.

**Also seen:** the first download progress event reported total="40" for a
268 MB IPA, so the app's download percentage may be wrong. To check.

## Why YouTube left one binary encrypted but Claude didn't (verified 2026-10-18)

The App Store "minimum iOS" is the MAIN APP's floor (YouTube 17.0, Claude
18.0). It says nothing about individual bundled components. Per-binary
LC_BUILD_VERSION min-OS, read from the decrypted IPAs:

- Claude (all cryptid=0): main 18.0; extensions 18.0; Sentry.framework
  **100.0**(!); LinkKit.framework 15.0.
- YouTube: main 17.0; most extensions 17.0; widevine framework (none);
  **AppMigrationExtension 26.1**.

Two lessons:
1. **min-OS alone isn't the gate.** Sentry is tagged min-OS 100.0 yet
   decrypted fine, because a *framework* is loaded into the main app's
   process: when the app runs (via SBS), dyld maps its frameworks and their
   __TEXT faults in, so they decrypt regardless of their own min-OS.
2. **Standalone extensions each need their own runnable process.** YouTube's
   AppMigrationExtension is built for iOS 26.1 and links AppMigrationKit (an
   iOS-26 framework absent on iOS 16) via the iOS-26-only
   com.apple.app-migration extension point. Its process can't launch on iOS
   16.5.1, so FairPlay never decrypts it. Everything else in YouTube (all
   17.0) runs and decrypts.

So it's not "YouTube needs a higher iOS than Claude." YouTube ships one
forward-compat extension built far ahead (26.1, higher than anything in
Claude), dormant until iOS 26. The verify step now reports exactly that
("requires iOS 26.1") as a skip, not a failure. The skip heuristic is safe
because it only examines binaries that are still encrypted — a high-min-OS
framework like Sentry that decrypted normally is never flagged.

