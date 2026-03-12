Converting a web player to Android TV is a great way to get your media onto the big screen. Since you've already built the web version, you have two main paths: the "Quick Port" (WebView) and the "Robust Rebuild" (Native/Hybrid).

For a media player, the biggest challenge isn't just showing the video—it's **spatial navigation** (using a remote instead of a mouse) and **video codec performance**.

---

## Option 1: The "Quick Port" (Capacitor / WebView)

This is the fastest method. You wrap your existing website in a native Android "shell" using a **WebView**.

* **Best for:** Rapid prototyping or if your web player is already very high-performance.
* **The Tool:** [Capacitor](https://capacitorjs.com/) is the modern successor to Cordova. It turns your web project into a real Android Studio project.
* **The Process:**

1. Install Capacitor in your web project: `npm install @capacitor/core @capacitor/cli`.
2. Add the Android platform: `npx cap add android`.
3. Open the project in **Android Studio**.
4. **Crucial Step:** You must modify the `AndroidManifest.xml` to declare it as a TV app:

* Add `<uses-feature android:name="android.software.leanback" android:required="false" />`.
* Add `<uses-feature android:name="android.hardware.touchscreen" android:required="false" />`.
* Set the intent filter to `CATEGORY_LEANBACK_LAUNCHER` so it shows up on the TV home screen.

> **Warning:** WebViews on older or cheaper Android TVs can be laggy. They often struggle with 4K playback or HDR compared to native players.

---

## Option 2: The "Hybrid Powerhouse" (React Native + Expo)

If your web player uses React, this is the gold standard. You reuse your logic (API calls, state management) but use native video components.

* **Best for:** A "Jellyfin-like" experience that feels smooth and professional.
* **The Tool:** [React Native](https://reactnative.dev/) with the [react-native-tvos](https://github.com/react-native-tvos/react-native-tvos) fork (which supports Android TV).
* **Key Advantage:** You can use **ExoPlayer** (the engine behind YouTube and Netflix on Android) via libraries like `react-native-video`. This ensures your TV handles hardware decoding properly.

---

## 3 Critical Adjustments for TV

Regardless of which route you take, your web code needs three specific "TV-first" changes:

### 1. Spatial Navigation (Focus Management)

On a TV, there is no mouse. Users move a "focus" box around.

* **Web/Capacitor:** Use a library like [Norigin Spatial Navigation](https://www.google.com/search?q=https://github.com/davedbase/react-spatial-navigation) to manage which button is highlighted when the user presses arrows on the remote.
* **Native:** Android handles focus automatically for standard buttons, but you must ensure your "Play/Pause" buttons are reachable via D-pad.

### 2. The "10-Foot UI"

Elements that look good on a monitor are too small for a TV.

* Increase font sizes (minimum 24–30pt).
* Increase padding between posters/thumbnails.
* Ensure high contrast for the "focused" state so the user knows exactly where they are.

### 3. Video Codecs & DRM

Browsers and Android TVs support different formats.

* **Web:** Often uses HLS or DASH via `hls.js`.
* **Android TV:** Native players prefer **ExoPlayer**. If you use a WebView, make sure your server provides a stream format the Android System WebView can handle (usually `.m3u8` or `.mp4`).

---

## Which one should you pick?

| Feature                | Capacitor (WebView)  | React Native (Hybrid) |
| ---------------------- | -------------------- | --------------------- |
| **Development Speed**  | 🚀 Extremely Fast     | 🛠️ Moderate            |
| **Code Reuse**         | ~95%                 | ~70%                  |
| **Performance**        | Average              | High                  |
| **Remote Support**     | Manual (via JS)      | Native / Built-in     |
| **App Store Approval** | Harder (Low quality) | Easier                |

Yes, you **can use a USB stick**, but it’s usually the **least convenient method**. There are three practical ways people install a dev app on an Android TV.

---

## 1️⃣ Best method: ADB over Wi-Fi (recommended)

Most developers install apps directly from their computer using **ADB**.

**Steps**

1. Enable **Developer Options** on the TV

   * Settings → About → click **Build number** 7 times

2. Enable

   * **USB debugging**
   * **Network debugging** (if available)

3. Find the TV's IP address

   * Settings → Network

4. Connect from your computer

```bash
adb connect TV_IP_ADDRESS
```

Example:

```bash
adb connect 192.168.1.42
```

1. Install the app

```bash
adb install myapp.apk
```

Your app immediately appears on the TV.

**Advantages**

* fastest dev workflow
* reinstall in seconds
* can stream logs
* can debug crashes

---

## 2️⃣ Android Studio direct install

If you build the app in **Android Studio**, it's even easier.

When the TV is connected via ADB:

```
Run → Select device → Your TV
```

Android Studio builds and installs automatically.

This is the **normal Android developer workflow**.

---

## 3️⃣ USB stick sideload (works but clunky)

Yes, you can do this.

Steps:

1. Build APK

```
./gradlew assembleRelease
```

1. Copy `app.apk` to a **USB stick**

2. Insert USB into TV

3. Use a **file manager app** (like X-plore or File Commander)

4. Open the APK → install

You must enable:

```
Settings → Security → Allow unknown apps
```

**Downside**

* slow for development
* reinstalling repeatedly is annoying

---

## 4️⃣ Bonus: Install from your phone (also common)

You can also:

* upload the APK to Google Drive
* download it on the TV
* install it

But again — slower than ADB.

---

## My recommendation for your project

Since you're building a **Jellyfin-style media app**, do this:

**Development**

```
Android Studio Emulator
+
ADB to real Android TV
```

**Testing playback**

Use a real device like:

* Nvidia Shield
* Chromecast with Google TV
* Fire TV (if you add compatibility)

Emulators sometimes struggle with video decoding.

---

## One important thing for your project

Because your backend is **Go**, make sure the TV app streams like this:

```
TV App
   ↓
Your Plum API
   ↓
Direct stream or transcoded stream
```

Usually:

```
HLS (.m3u8)
or
Direct file stream
```

with **ExoPlayer**.

That’s exactly how Jellyfin/Plex do it.

---

If you'd like, I can also show you something **extremely useful for your project**:

**the typical architecture of a media server TV client** (the screens, playback pipeline, API calls, caching, etc.).

That will save you a lot of trial and error when building your Jellyfin clone.
