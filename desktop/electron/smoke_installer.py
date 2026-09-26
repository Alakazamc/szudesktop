"""Exercise the final NSIS package on a disposable GitHub Windows runner.

Never installs on a developer's desktop. Uses an isolated Chinese/space path and
profile, refuses existing installations, and only stops processes it started.
The published baseline is installed, given synthetic data, upgraded to the
candidate, opened twice, and removed. No real account or school login is used.
"""
import ctypes
from contextlib import contextmanager
import hashlib
import http.client
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import time
import uuid
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent
EVIDENCE = HERE / "release" / "smoke-evidence"
# electron-builder's stable NSIS UUID v5 namespace and this app's configured ID.
APP_GUID = str(uuid.uuid5(uuid.UUID("50e065bc-3134-11e6-9bab-38c9862bdaf3"), "com.szudesktop.app"))
INSTALL_KEY = "Software\\" + APP_GUID
UNINSTALL_KEY = "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\" + APP_GUID
RUN_KEY = "Software\\Microsoft\\Windows\\CurrentVersion\\Run"
# Keep this baseline fixed: a moving latest URL could silently test a reinstall.
BASELINE_VERSION = "beta0.9.1"
BASELINE_ELECTRON = "44.4.5"
BASELINE_SHA256 = "c45de44c63b74b1f9a17f2f1b698ee04f1786211c2239b762e823dded20484a3"


def check(label, condition):
    if not condition:
        raise AssertionError(label)
    print("PASS", label, flush=True)


def reg_values(hive, key, view):
    import winreg
    try:
        with winreg.OpenKey(hive, key, 0, winreg.KEY_READ | view) as opened:
            return {winreg.EnumValue(opened, i)[0]: winreg.EnumValue(opened, i)[1]
                    for i in range(winreg.QueryInfoKey(opened)[1])}
    except FileNotFoundError:
        return {}


def installation():
    import winreg
    return reg_values(winreg.HKEY_CURRENT_USER, INSTALL_KEY, winreg.KEY_WOW64_64KEY)


def no_existing_installation():
    import winreg
    for hive in (winreg.HKEY_CURRENT_USER, winreg.HKEY_LOCAL_MACHINE):
        for view in (winreg.KEY_WOW64_32KEY, winreg.KEY_WOW64_64KEY):
            check("no pre-existing installation in registry", not reg_values(hive, INSTALL_KEY, view)
                  and not reg_values(hive, UNINSTALL_KEY, view))


def run_nsis(exe, final_argument):
    # NSIS explicitly requires /D= and _?= to be LAST and UNQUOTED, even with
    # spaces. Pass a raw command line directly to CreateProcess (never a shell).
    # builder 26's assisted installer (/S + oneClick:false) starts the app only
    # with --force-run. Never pass that flag: launch() supplies isolated config.
    command = subprocess.list2cmdline([str(exe), "/S", "/currentuser"])
    proc = subprocess.Popen(command + " " + final_argument,
                            creationflags=subprocess.CREATE_NO_WINDOW)
    try:
        check("NSIS exits successfully", proc.wait(timeout=150) == 0)
    finally:
        stop_owned_tree(proc)


def stop_owned_tree(proc):
    if proc.poll() is None:
        # A Popen object that still holds the running process prevents guessing
        # by name or targeting unrelated user instances.
        subprocess.run(["taskkill", "/PID", str(proc.pid), "/T", "/F"],
                       capture_output=True, timeout=15,
                       creationflags=subprocess.CREATE_NO_WINDOW)
        proc.wait(timeout=15)


def wait_process_gone(pid, timeout=15):
    """Hold a Windows process handle so a recycled PID cannot fool the check."""
    from ctypes import wintypes
    kernel = ctypes.WinDLL("kernel32", use_last_error=True)
    kernel.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
    kernel.OpenProcess.restype = wintypes.HANDLE
    kernel.WaitForSingleObject.argtypes = [wintypes.HANDLE, wintypes.DWORD]
    kernel.CloseHandle.argtypes = [wintypes.HANDLE]
    handle = kernel.OpenProcess(0x00100000, False, pid)  # SYNCHRONIZE only
    if not handle:
        check("sidecar process is gone", ctypes.get_last_error() == 87)
        return
    try:
        check("owned sidecar exits with its window", kernel.WaitForSingleObject(handle, timeout * 1000) == 0)
    finally:
        kernel.CloseHandle(handle)


def launch(exe, cfg, version, label, owned=True, initial_scale=1.7, runtime=None):
    report = EVIDENCE / (label + ".json")
    shot = EVIDENCE / (label + ".png")
    report.unlink(missing_ok=True)
    shot.unlink(missing_ok=True)
    env = dict(os.environ, SZUNET_CONFIG_DIR=str(cfg), SZU_SMOKE_REPORT=str(report),
               SZU_SMOKE_SCREENSHOT=str(shot), SZU_SMOKE_QUIT_AFTER_REPORT="1")
    # An inherited developer diagnostic variable must not compete with smoke.
    env.pop("SZU_SHOT", None)
    with (EVIDENCE / (label + ".log")).open("wb") as log:
        proc = subprocess.Popen([str(exe)], env=env, stdout=log, stderr=log,
                                creationflags=subprocess.CREATE_NO_WINDOW)
        try:
            deadline = time.monotonic() + 60
            while not report.exists() and time.monotonic() < deadline:
                if proc.poll() is not None:
                    raise RuntimeError("installed application exited before its rendered-page report")
                time.sleep(.1)
            check(label + ": real UI report created", report.exists())
            # main writes the report atomically before its normal quit path.
            result = json.loads(report.read_text(encoding="utf-8"))
            if "error" in result:
                raise RuntimeError("installed app smoke failed: " + result["error"])
            check(label + ": Go engine version", result["version"] == version)
            check(label + ": installer metadata version", result["packageVersion"] == re.sub(r"^(?:beta|v)", "", version))
            expected_runtime = runtime or json.loads((HERE / "package-lock.json").read_text(encoding="utf-8"))["packages"]["node_modules/electron"]["version"]
            check(label + ": supported Electron runtime packaged", result["electron"] == expected_runtime)
            check(label + ": actual main process", result["appPid"] == proc.pid)
            check(label + ": real garden rendered", result["rendered"] is True and bool(result["title"]))
            check(label + ": loopback UI", re.fullmatch(r"http://127\.0\.0\.1:\d+/?", result["baseUrl"]) is not None)
            check(label + ": screenshot captured", shot.is_file() and shot.stat().st_size > 1000)
            check(label + ": engine ownership", result["owned"] is owned)
            pet = result["pet"]
            check(label + ": real pet and tray", pet["rendered"] and pet["tray"])
            check(label + ": main closes and reopens", pet["closeAndReopen"])
            check(label + ": pet visibility and actions", pet["hideAndShow"] and pet["actionsReturnToBase"])
            check(label + ": scale through settings and tray", pet["settingsAndPresets"] and pet["finalScale"] == 1.7)
            check(label + ": scale survives restart or upgrade", pet["initialScale"] == initial_scale)
            if version != BASELINE_VERSION:
                check(label + ": four-companion selection", pet.get("petSelection") is True and pet.get("petSelectionSync") is True)
                check(label + ": backup export and restore", pet.get("backupRestore") is True)
            check(label + ": normal window exit", proc.wait(timeout=25) == 0)
            if owned:
                wait_process_gone(result["sidecarPid"])
            return result
        finally:
            stop_owned_tree(proc)


def assert_install_path(install_dir):
    stored = installation().get("InstallLocation", "")
    check("installer registry points only to this test directory",
          bool(stored) and Path(stored).resolve() == install_dir)


def local_request(base_url, endpoint, method="GET", data=None):
    parsed = urlsplit(base_url)
    check("probe only calls loopback", parsed.hostname == "127.0.0.1")
    conn = http.client.HTTPConnection(parsed.hostname, parsed.port, timeout=5)
    try:
        body = json.dumps(data if data is not None else {}).encode("utf-8") if method == "POST" else None
        conn.request(method, endpoint, body=body,
                     headers={"Content-Type": "application/json"})
        response = conn.getresponse()
        check("local probe HTTP success", response.status == 200)
        return json.loads(response.read())
    finally:
        conn.close()


@contextmanager
def running_engine(sidecar, cfg, version, label):
    """Own a no-auto-login engine with isolated state for local-only probes."""
    log_path = EVIDENCE / (label + ".log")
    with log_path.open("wb") as log:
        proc = subprocess.Popen([str(sidecar), "--no-open", "--no-auto-login", "--addr", "127.0.0.1:0"],
                                env=dict(os.environ, SZUNET_CONFIG_DIR=str(cfg)), stdout=log, stderr=log,
                                creationflags=subprocess.CREATE_NO_WINDOW)
        try:
            deadline = time.monotonic() + 25
            base_url = None
            while time.monotonic() < deadline:
                check("isolated portable engine remains alive", proc.poll() is None)
                matches = re.findall(r"http://127\.0\.0\.1:\d+(?:/)?(?=\s|$)",
                                     log_path.read_text(encoding="utf-8", errors="replace"))
                if matches:
                    base_url = matches[-1].rstrip("/")
                    break
                time.sleep(.2)
            check("isolated portable engine reports URL", base_url is not None)
            check(label + ": engine version", local_request(base_url, "/api/health")["app_version"] == version)
            yield base_url, proc
            local_request(base_url, "/api/shutdown", "POST")
            check("test's portable engine shuts down normally", proc.wait(timeout=10) == 0)
        finally:
            stop_owned_tree(proc)


def coexist_with_portable(exe, sidecar, cfg, version):
    """A pre-existing portable engine belongs to its launcher, not Electron."""
    with running_engine(sidecar, cfg, version, "portable-engine") as (base_url, proc):
        result = launch(exe, cfg, version, "reuse-portable", owned=False)
        check("installer reuses the pre-existing engine", result["baseUrl"].rstrip("/") == base_url)
        check("closing installer leaves portable engine alive", proc.poll() is None
              and local_request(base_url, "/api/health")["app_version"] == version)


def seed_upgrade_data(base_url):
    snapshot = local_request(base_url, "/api/workspace")
    data = snapshot["data"]
    check("baseline really has the two original companions", len(data["game"]["pets"]) == 2)
    data["profile"] = {"name": "升级验收", "college": "合成资料"}
    data["preferences"] = {"theme": "night", "motion": False, "onboarded": True}
    data["todos"] = [{"id": "upgrade-task", "text": "升级后保留的合成待办", "done": True, "rewarded": True}]
    data["courses"] = [{"name": "合成课程", "credit": 3, "point": 3.5, "term": "升级测试学期",
                        "code": "UPGRADE", "level": "undergrad", "grade": "", "source": "手动录入", "included": True}]
    data["semester"] = "2026-09-01"
    now = int(time.time() * 1000)
    data["reminders"] = [{"id": "upgrade-reminder", "place": "合成提醒", "start": now + 3600000, "end": now + 5400000}]
    game = data["game"]
    game["coins"], game["food"] = 137, 5
    game["seeds"]["radish"], game["stock"]["strawberry"] = 9, 4
    game["plots"][1] = {"crop": "strawberry", "planted": now, "ready": now + 300000, "watered": True}
    game["stats"].update({"focus": 3, "minutes": 75, "tasks": 1})
    game["pets"][0].update({"name": "留住荔宝", "xp": 125, "hunger": 100})
    game["pets"][1].update({"name": "留住栗栗", "xp": 75})
    saved = local_request(base_url, "/api/workspace", "POST", snapshot)
    # This deliberately invalid account is stored locally, never authenticated.
    fake = {"username": "installer-upgrade-fixture", "password": "synthetic-local-only-password"}
    local_request(base_url, "/api/credential", "POST", fake)
    check("synthetic account is readable on the old version", local_request(base_url, "/api/credential?reveal=1")["username"] == fake["username"])
    return saved


def assert_user_data(data, expected):
    # The UI smoke intentionally feeds/switches pets. Compare personal records
    # and progression, not time-decaying hunger or the last interaction message.
    for key in ("profile", "preferences", "todos", "courses", "semester", "reminders"):
        check("upgrade preserves " + key, data[key] == expected[key])
    for key in ("coins", "seeds", "stock", "plots", "stats"):
        check("upgrade preserves garden " + key, data["game"][key] == expected["game"][key])
    for index, pet in enumerate(expected["game"]["pets"]):
        current = data["game"]["pets"][index]
        check("upgrade keeps original companion and progression", current["species"] == pet["species"]
              and current["name"] == pet["name"] and current["xp"] >= pet["xp"])
    check("upgrade adds new companions without removing old ones",
          [pet["species"] for pet in data["game"]["pets"]] == ["libao", "chestnut", "egret", "turtle"])


def main():
    if os.name != "nt" or os.environ.get("GITHUB_ACTIONS") != "true" or not os.environ.get("RUNNER_TEMP"):
        raise SystemExit("Installer smoke is only allowed on a disposable GitHub Windows runner; no installation performed.")
    import winreg
    EVIDENCE.mkdir(parents=True, exist_ok=True)
    version = (ROOT / "internal" / "version" / "VERSION").read_text(encoding="utf-8").strip()
    check("valid release version", re.fullmatch(r"(?:beta|v)?\d+\.\d+\.\d+", version) is not None)
    semver = re.sub(r"^(?:beta|v)", "", version)
    installer = HERE / "release" / ("szuDesktop-Setup-" + semver + ".exe")
    check("final installer exists", installer.is_file())
    digest = hashlib.sha256(installer.read_bytes()).hexdigest()
    check("installer checksum matches", Path(str(installer) + ".sha256").read_text(encoding="ascii")
          == digest + "  " + installer.name + "\n")
    baseline = Path(os.environ["SZU_UPGRADE_INSTALLER"]).resolve()
    check("baseline is the separately downloaded published installer", baseline.is_file() and baseline != installer.resolve()
          and baseline.name == "szuDesktop-Setup-0.9.1.exe")
    check("candidate differs from the baseline version", version != BASELINE_VERSION)
    check("published baseline matches pinned release digest", hashlib.sha256(baseline.read_bytes()).hexdigest() == BASELINE_SHA256)
    check("published baseline checksum file agrees", Path(str(baseline) + ".sha256").read_text(encoding="ascii")
          == BASELINE_SHA256 + "  " + baseline.name + "\n")
    no_existing_installation()
    startup_before = reg_values(winreg.HKEY_CURRENT_USER, RUN_KEY, winreg.KEY_WOW64_64KEY)
    runner_temp = Path(os.environ["RUNNER_TEMP"]).resolve()
    # The enclosing TemporaryDirectory is the only recursively cleaned path.
    # It is freshly allocated beneath runner temp and never an installed user path.
    with tempfile.TemporaryDirectory(prefix="szu-installer-", dir=runner_temp) as tmp:
        test_root = Path(tmp).resolve()
        check("test root stays within runner temp", test_root.parent == runner_temp and test_root.name.startswith("szu-installer-"))
        install_dir = (test_root / "安装 测试" / "szuDesktop").resolve()
        cfg = (test_root / "独立 用户配置").resolve()
        cfg.mkdir()
        workspace = cfg / "workspace-v1.json"
        preserved_files = (workspace, cfg / "credentials.json", cfg / "electron-profile" / "pet-settings.json")
        exe = install_dir / "szuDesktop.exe"
        uninstaller = install_dir / "Uninstall szuDesktop.exe"
        installed = False
        try:
            run_nsis(baseline, "/D=" + str(install_dir))
            assert_install_path(install_dir)
            installed = True
            check("main executable installed in Chinese/space path", exe.is_file())
            sidecar = install_dir / "resources" / "szudesktop-windows-amd64.exe"
            launch(exe, cfg, BASELINE_VERSION, "baseline-open", initial_scale=1, runtime=BASELINE_ELECTRON)
            with running_engine(sidecar, cfg, BASELINE_VERSION, "baseline-data") as (base_url, _):
                expected = seed_upgrade_data(base_url)
            preserved = {file: file.read_bytes() for file in preserved_files}
            check("test account is not stored in plaintext", b"synthetic-local-only-password" not in preserved[cfg / "credentials.json"])
            obsolete = install_dir / "resources" / "obsolete-upgrade-smoke.txt"
            obsolete.write_text("old version test resource", encoding="ascii")

            run_nsis(installer, "/D=" + str(install_dir))
            assert_install_path(install_dir)
            check("upgrade removes obsolete program files", not obsolete.exists())
            for file, contents in preserved.items():
                check("upgrade preserves " + file.name + " byte for byte before opening", file.read_bytes() == contents)
            check("installed Go engine matches final build", sidecar.read_bytes()
                  == (ROOT / "dist" / "szudesktop-windows-amd64.exe").read_bytes())
            with running_engine(sidecar, cfg, version, "upgraded-data") as (base_url, _):
                check("new engine reads the unchanged old save", local_request(base_url, "/api/workspace") == expected)
                check("new engine decrypts the old synthetic account", local_request(base_url, "/api/credential?reveal=1")["username"]
                      == "installer-upgrade-fixture")
                check("saved account remains hidden by default", local_request(base_url, "/api/credential")["username"] == "")
            first = launch(exe, cfg, version, "after-upgrade")
            assert_user_data(json.loads(workspace.read_bytes())["data"], expected["data"])
            launch(exe, cfg, version, "reopen")
            coexist_with_portable(exe, sidecar, cfg, version)
            assert_user_data(json.loads(workspace.read_bytes())["data"], expected["data"])
            check("no startup entries changed", startup_before == reg_values(winreg.HKEY_CURRENT_USER, RUN_KEY, winreg.KEY_WOW64_64KEY))
            (EVIDENCE / "summary.json").write_text(json.dumps({
                "version": version, "electron": first["electron"], "installer_sha256": digest,
                "upgrade_from": BASELINE_VERSION, "baseline_sha256": BASELINE_SHA256,
                "installed": True, "rendered": True, "reopened": True,
                "portable_engine_coexistence": True,
                "cross_version_upgrade_preserved_data": True, "obsolete_program_files_removed": True,
                "synthetic_account_decrypts_after_upgrade": True, "original_companion_progress_preserved": True,
                "backup_restore": first["pet"]["backupRestore"],
                "pet_and_tray": True, "pet_scale_persists": True,
            }, ensure_ascii=False, indent=2), encoding="utf-8")
        finally:
            if installed:
                # Startup may fail before the first save exists. Do not obscure
                # that primary failure by asserting it was created during cleanup.
                saved_before_uninstall = {file: file.read_bytes() for file in preserved_files if file.is_file()}
                # The uninstaller recursively removes INSTDIR: verify the exact
                # canonical location and registry ownership immediately beforehand.
                check("uninstall target is inside this test root", install_dir.is_relative_to(test_root))
                assert_install_path(install_dir)
                check("uninstaller belongs to this installation", uninstaller.is_file()
                      and uninstaller.resolve().parent == install_dir)
                run_nsis(uninstaller, "_?=" + str(install_dir))
                check("uninstall removes main executable and engine", not exe.exists()
                      and not (install_dir / "resources").exists())
                check("uninstall removes its registration", not installation()
                      and not reg_values(winreg.HKEY_CURRENT_USER, UNINSTALL_KEY, winreg.KEY_WOW64_64KEY))
                for file, contents in saved_before_uninstall.items():
                    check("uninstall keeps " + file.name + " byte for byte", file.is_file() and file.read_bytes() == contents)
                check("uninstall leaves startup entries unchanged", startup_before
                      == reg_values(winreg.HKEY_CURRENT_USER, RUN_KEY, winreg.KEY_WOW64_64KEY))
    summary = json.loads((EVIDENCE / "summary.json").read_text(encoding="utf-8"))
    summary["uninstalled"] = True
    (EVIDENCE / "summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print("ALL INSTALLER SMOKE CHECKS PASSED", flush=True)


if __name__ == "__main__":
    for stream in (sys.stdout, sys.stderr):
        if hasattr(stream, "reconfigure"):
            stream.reconfigure(encoding="utf-8", errors="replace")
    main()
