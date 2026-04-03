# Plum Android TV — Build & Install Guide

This guide covers building and sideloading the Plum Android TV app onto your device **without Android Studio**. All steps use the command line.

---

## Prerequisites

### JDK

You need **JDK 17 or 21**. JDK 22+ causes an `IllegalArgumentException` in the Kotlin/Gradle DSL and will not work.

The Android TV Gradle wrapper auto-selects Android Studio’s bundled JBR when it is installed, so on Linux or macOS you usually do not need to set `JAVA_HOME` manually.

```bash
# Check your version
java -version

# Arch Linux
sudo pacman -S jdk17-openjdk

# Ubuntu / Debian
sudo apt install openjdk-17-jdk

# macOS (Homebrew)
brew install openjdk@17
```

If you want to override the auto-selection, set `JAVA_HOME` before running Gradle:

```bash
export JAVA_HOME=/usr/lib/jvm/java-17-openjdk
```

### Android SDK (command-line tools only)

You do **not** need Android Studio. Install only the command-line tools.

1. Download the latest **Command line tools only** zip from [developer.android.com/studio](https://developer.android.com/studio#command-tools) (scroll to the bottom of the page).

2. Unpack into a permanent location, with the required directory structure:

   ```bash
   mkdir -p ~/Android/Sdk/cmdline-tools/latest
   unzip commandlinetools-*.zip -d /tmp/cmdline
   mv /tmp/cmdline/cmdline-tools/* ~/Android/Sdk/cmdline-tools/latest/
   ```

3. Add the SDK tools to your `PATH`:

   ```bash
   export ANDROID_HOME=~/Android/Sdk
   export PATH="$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/platform-tools:$PATH"
   ```

   Add these lines to your shell's rc file (`~/.bashrc`, `~/.zshrc`, `~/.config/fish/config.fish`, etc.) to make them permanent.

4. Accept licenses and install the required SDK components:

   ```bash
   sdkmanager --licenses
   sdkmanager "platform-tools" "platforms;android-35" "build-tools;35.0.0"
   ```

---

## Project Setup

1. Navigate to the Android TV app directory:

   ```bash
   cd apps/plum/apps/android-tv
   ```

2. Copy the example properties file and set your SDK path:

   ```bash
   cp local.properties.example local.properties
   ```

   Edit `local.properties` and set the `sdk.dir` to where you installed the SDK:

   ```properties
   sdk.dir=/home/YOU/Android/Sdk
   ```

---

## Building the APK

From within `apps/android-tv`, run:

```bash
./gradlew assembleDebug
```

Gradle will download its own wrapper distribution on the first run. This may take a few minutes.

The built APK will be at:

```
app/build/outputs/apk/debug/app-debug.apk
```

---

## Installing on a Device

### Enable Developer Options and ADB on your Android TV

1. On your Android TV, go to **Settings → Device Preferences → About**.
2. Scroll to **Build** and press the select button **7 times** until "You are now a developer" appears.
3. Go back to **Settings → Device Preferences → Developer options**.
4. Enable **USB debugging** (for a USB connection) or **Network debugging** (for Wi-Fi/LAN).

### Connect via USB

```bash
adb devices
```

You should see your device listed. If prompted on the TV, allow the connection.

### Connect over the network (no USB cable)

Find your TV's IP address at **Settings → Network & Internet → [your network] → IP address**, then:

```bash
adb connect <TV_IP_ADDRESS>:5555
```

### Install the APK

```bash
adb install app/build/outputs/apk/debug/app-debug.apk
```

For subsequent installs (reinstall/upgrade):

```bash
adb install -r app/build/outputs/apk/debug/app-debug.apk
```

From the repo root, you can also run `make deploy-tv` to build the APK, reinstall it on a connected TV, and launch the app.

---

## First-Run Setup

When you launch **Plum** on your TV for the first time, the app walks through a short setup flow:

1. **Server URL screen** — Enter the base URL of your running Plum server, e.g.:
   - LAN: `http://192.168.1.x:8080`
   - Tailscale / VPN: `http://100.x.x.x:8080`
   - HTTPS reverse proxy: `https://plum.yourdomain.com`

   The default placeholder shown is `http://10.0.2.2:8080` (the Android emulator loopback — useful only if you're testing in an emulator on the same machine running the server).

2. **Login screen** — Enter your Plum account email and password.

After a successful login, you land on the main library view.

---

## Useful ADB Commands

| Command | Description |
|---|---|
| `adb devices` | List connected devices |
| `adb install -r <apk>` | Reinstall/update the app |
| `adb uninstall plum.tv.app` | Remove the app |
| `adb logcat -s PlumTV` | Stream Android TV app logs |
| `adb shell am start -n plum.tv.app/.MainActivity` | Launch the app from the terminal |
| `adb disconnect` | Disconnect a network ADB session |

### Log filtering

Use these when troubleshooting startup, Discover, login, or playback:

```bash
# Android TV app logs only
adb logcat -s PlumTV

# Android TV logs plus common crash markers
adb logcat | grep -E "PlumTV|AndroidRuntime|FATAL"

# Server startup and request logs
bun run dev:server 2>&1 | grep -E '"component":"server"|"startup config"|PlumTV'

# Just startup or just request events
bun run dev:server 2>&1 | grep '"event":"startup"'
bun run dev:server 2>&1 | grep '"event":"request"'

# Pretty-print JSON server lines if jq is installed
bun run dev:server 2>&1 | grep '"component":"server"' | jq -c .

# Count startup vs request events
bun run dev:server 2>&1 | grep '"component":"server"' | jq -r '.event' | sort | uniq -c

# Bucket request status codes by class
bun run dev:server 2>&1 | grep '"event":"request"' | jq -r '.status / 100 | floor' | sort | uniq -c

# Show only failed requests
bun run dev:server 2>&1 | grep '"event":"request"' | jq -c 'select(.status >= 400)'
```

What to look for:

- Android TV lines tagged `PlumTV` show app startup, HTTP requests, login/logout, and WebSocket activity.
- Server lines with `"component":"server"` show startup config and per-request summaries.
- Missing Discover metadata usually shows `metadata.tmdb=false` or a server startup line with `env_loaded=false`.

---

## Troubleshooting

**`IllegalArgumentException` during build**
Your JDK is too new. Switch to JDK 17 or 21 and re-run. See [Prerequisites](#jdk).

**`SDK location not found` error**
`local.properties` is missing or `sdk.dir` is wrong. Double-check the path — it should be the root of your Android SDK (the folder that contains `platform-tools/`).

**`adb: device unauthorized`**
On your TV, look for an "Allow USB debugging" dialog and accept it. If it never appears, revoke existing keys under Developer options and reconnect.

**`adb: no devices/emulators found` over network**
Make sure **Network debugging** is enabled on the TV and that both devices are on the same network (or reachable via Tailscale/VPN). Some routers block device-to-device traffic on the LAN — try a direct Ethernet connection or Tailscale.

**App shows "Login failed"**
Verify the server URL is reachable from the TV. The TV and the server need to be on the same network or connected via VPN. Test by opening a browser on another device and navigating to `http://<server-ip>:8080`.

**Blank screen or app crashes on launch**
Stream logs to diagnose:
```bash
adb logcat | grep -E "plum|AndroidRuntime|FATAL"
```
