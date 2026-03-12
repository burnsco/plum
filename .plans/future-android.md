Yes. You can absolutely add an Android TV client to your Jellyfin-style stack, and you do **not** need to deploy to a physical TV first to start building and testing it. Android Studio supports Android TV virtual devices through the Android Emulator and AVD Manager, so you can run and debug a TV app locally during development. Google still recommends final testing on real hardware before release, but emulator-first development is normal and supported. ([Android Developers][1])

For your setup, the cleanest architecture is usually this:

1. Keep your existing **backend** as the source of truth: auth, users, libraries, metadata, playback URLs, transcoding/session control, watch state, continue watching, subtitles, device sessions.
2. Build a separate **Android TV frontend client** that consumes that API.
3. Reuse business logic where it makes sense at the API/schema level, not by trying to force your web UI directly onto TV. Android TV has its own navigation, focus, remote-control, and “10-foot UI” needs. ([Android Developers][2])

If you want the **best long-term result**, I’d recommend this stack for the TV app:

* **Kotlin**
* **Android Studio**
* **Jetpack Compose**
* **Compose for TV**
* **ExoPlayer / Media3**
* **Retrofit or Ktor client for API calls**
* **Coil or Glide for images/posters**
* **Room or DataStore for local persistence**
* **Hilt or Koin for dependency injection**

That recommendation is grounded in the fact that Jetpack Compose is Android’s recommended modern UI toolkit, and Google provides a dedicated **Compose for TV** path for TV interfaces. Compose for TV is specifically intended for Android TV-style apps and works on Android TVs running Android 5.0+; Android’s TV docs also point developers toward Compose for TV for modern TV UIs. ([Android Developers][3])

For a Jellyfin-like TV app, the main library buckets you’ll need are:

* **UI/navigation:** Compose + Compose for TV for browse screens, rows, details pages, settings, focus states, and remote navigation.
* **Playback:** Android’s media stack, typically **Media3 / ExoPlayer**, for HLS/DASH/direct play/subtitles/audio track switching.
* **Networking:** Retrofit/Ktor + OkHttp for your API and image loading.
* **State/storage:** Room/DataStore for cached metadata, login token, continue-watching state, and preferences.
* **Images:** poster/backdrop loading and caching.
* **Crash/logging/testing:** Timber/Logcat, JUnit, UI tests, emulator/device testing.
  This isn’t a single official list from Google, but it follows the standard Android TV app architecture Google documents plus the media-app shape your project needs. ([Android Developers][4])

A few Android TV-specific things you will need to account for that are different from your web app:

* **D-pad / remote navigation**
* **focus management**
* **large-screen 10-foot layout**
* **playback controls optimized for remote input**
* **lean-back browsing patterns** like content rails, hero banners, details pages, and watch-next style flows

That matters because a TV app is not just “your web app on a big screen.” The design guidance for TV explicitly centers on readability from distance and simple directional-button navigation. ([Android Developers][5])

On the dev/testing question: **yes**, you can preview and test without a TV first.

You have a few good options:

* **Android TV Emulator / AVD** in Android Studio for full app testing.
* **Compose previews** for fast UI iteration if you use Compose.
* **ADB on a real Android TV / Chromecast with Google TV / Shield** later for final hardware checks.
* Emulator networking to hit your local backend during development. ([Android Developers][1])

The practical limitation is this: the emulator is great for layout, navigation, login flow, API integration, and a lot of playback testing, but **final playback and remote-feel validation should still be checked on real hardware** before you call it done. Google explicitly advises real-device testing before release. ([Android Developers][6])

You also have a second path if you care more about sharing code with your React frontend: **React Native TV** via the `react-native-tvos` ecosystem. That project supports Android TV and Apple TV, and it can be a viable route if you strongly want JS/TS across platforms. But it is community-maintained rather than the core Google-native path, so for a media-heavy TV app, native Android with Kotlin + Compose for TV is usually the safer and stronger foundation. ([GitHub][7])

My honest take for your Jellyfin clone:

* If you want **best quality, best TV UX, least fighting with the platform**: go **native Android TV client in Kotlin + Compose for TV**.
* If you want **maximum code-sharing with your React ecosystem** and accept some extra friction: consider **React Native TV**.
* Do **not** make the TV version just a webview unless it’s a temporary prototype. It’ll feel wrong fast on remote navigation and playback UX.

A solid starter stack for you would look like this:

* **Backend:** your existing Go API
* **TV app:** Kotlin, Android Studio, Compose for TV
* **Playback:** Media3 / ExoPlayer
* **API client:** Retrofit + OkHttp
* **Images:** Coil
* **State:** ViewModel + StateFlow
* **Storage:** Room or DataStore
* **DI:** Hilt
* **Testing:** Android TV emulator first, then real Android TV device

If you want, I can map out a **full Android TV client architecture** for your Jellyfin clone next, including screens, API endpoints, playback flow, and the exact libraries per module.

[1]: https://developer.android.com/studio/run/emulator?utm_source=chatgpt.com "Run apps on the Android Emulator | Android Studio"
[2]: https://developer.android.com/training/tv?utm_source=chatgpt.com "Android TV overview | Android Developers"
[3]: https://developer.android.com/compose?utm_source=chatgpt.com "Jetpack Compose UI App Development Toolkit"
[4]: https://developer.android.com/training/tv/get-started?utm_source=chatgpt.com "Get started with TV apps - Android Developers"
[5]: https://developer.android.com/tv?utm_source=chatgpt.com "Android TV | Multidevice - Android Developers"
[6]: https://developer.android.com/studio/run/device?utm_source=chatgpt.com "Run apps on a hardware device | Android Studio"
[7]: https://github.com/react-native-tvos/react-native-tvos/wiki?utm_source=chatgpt.com "Home · react-native-tvos/react-native-tvos Wiki"
