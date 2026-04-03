---
name: Android Kotlin Gradle audit
overview: Comprehensive audit of the Plum Android TV app's Kotlin and Gradle codebase identifying 20+ concrete improvements across build performance, runtime performance, architecture, and code quality -- prioritized by impact.
todos:
  - id: gradle-properties
    content: Add org.gradle.caching=true and org.gradle.configuration-cache=true to gradle.properties
    status: completed
  - id: kapt-to-ksp
    content: Migrate all 7+1 modules from KAPT to KSP for Hilt annotation processing
    status: completed
  - id: moshi-codegen
    content: Switch from moshi-kotlin (reflection) to moshi-kotlin-codegen (KSP) with @JsonClass annotations on all DTOs
    status: completed
  - id: enable-r8
    content: Enable isMinifyEnabled=true and isShrinkResources=true for release, add ProGuard rules
    status: completed
  - id: version-catalog
    content: Create gradle/libs.versions.toml and migrate all 12 build files to catalog references
    status: completed
  - id: fix-regex-dedup
    content: Fix Regex recompilation in ShowGrouping.kt, deduplicate normalizeShowKeyTitle, extract shared CastSection
    status: completed
  - id: dead-code
    content: Remove PlumPlayerHolder.kt, LibraryPlaceholder.kt, goBack() no-op, redundant onCleared, fix missing consumer-rules.pro
    status: completed
isProject: false
---

# Plum Android TV -- Kotlin + Gradle Audit & Optimization Plan

---

## 1. Executive Summary

**Biggest strengths:**

- Clean multi-module architecture with 12 properly scoped modules
- Consistent ViewModel + StateFlow + Repository pattern throughout
- Solid TV-specific UX: focus management, d-pad handling, low-RAM adaptation
- Good WebSocket reconnection and media cache strategy
- Reasonable Compose theming system with `PlumTheme` compositionLocals

**Biggest problems identified:**

- KAPT used for Hilt where KSP is fully supported (build time)
- Moshi reflection adapter instead of compile-time codegen (runtime perf + startup)
- No version catalog or convention plugins (12 build files with duplicated config)
- R8/minification disabled in release (APK size, no tree-shaking)
- `material-icons-extended` bloats APK by ~40MB without R8
- Missing Gradle caching and configuration cache properties

**Highest-value fixes delivered (in order):**

1. Switch KAPT to KSP for Hilt -- measurable build speed improvement
2. Switch Moshi reflection to Moshi codegen (KSP) -- runtime perf + startup + APK size
3. Enable R8 minification in release -- required for production
4. Add version catalog (`libs.versions.toml`) -- maintainability
5. Enable Gradle build cache + configuration cache -- build speed

---

## 2. Findings by Category

### 2A. Build Performance

**[HIGH] KAPT to KSP migration for Hilt**

7 modules use `org.jetbrains.kotlin.kapt` + `kapt("com.google.dagger:hilt-compiler:2.51.1")`. Since Hilt 2.48+, KSP is officially supported and significantly faster than KAPT (KAPT runs a full Java annotation processing round; KSP is a native Kotlin compiler plugin).

Affected modules: `:app`, `:core-data`, `:feature-auth`, `:feature-home`, `:feature-library`, `:feature-details`, `:feature-search`, `:feature-settings`.

Change in each:

- Replace `id("org.jetbrains.kotlin.kapt")` with `id("com.google.devtools.ksp")`
- Replace `kapt("com.google.dagger:hilt-compiler:2.51.1")` with `ksp("com.google.dagger:hilt-compiler:2.51.1")`
- In root `build.gradle.kts`: replace `id("org.jetbrains.kotlin.kapt")` with `id("com.google.devtools.ksp")` and add KSP version

**[HIGH] Missing Gradle caching properties**

`[gradle.properties](apps/android-tv/gradle.properties)` is missing:

```properties
org.gradle.caching=true
org.gradle.configuration-cache=true
```

These are free wins. Configuration cache especially helps repeat builds.

**[HIGH] `subprojects {}` block is a configuration-cache anti-pattern**

The `[build.gradle.kts](apps/android-tv/build.gradle.kts)` root file used `subprojects { pluginManager.withPlugin(...) }`, which eagerly configures all subprojects and breaks configuration cache compatibility. The logic was moved to convention plugins and the root block was removed.

**[MEDIUM] No version catalog**

Versions are scattered across 12 `build.gradle.kts` files. For example:

- `"com.google.dagger:hilt-android:2.51.1"` appears in 8 files
- `"androidx.compose:compose-bom:2024.10.01"` appears in 7 files
- `"io.coil-kt:coil-compose:2.7.0"` appears in 5 files
- `"androidx.tv:tv-foundation:1.0.0-beta01"` appears in 7 files

A `[gradle/libs.versions.toml](apps/android-tv/gradle/libs.versions.toml)` version catalog would centralize all of these.

**[MEDIUM] No convention plugins**

Every module repeats:

```kotlin
android {
    compileSdk = 35
    defaultConfig { minSdk = 24 }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions { jvmTarget = "17" }
}
```

A `buildSrc` or `build-logic` convention plugin like `plum.android.library` would eliminate this duplication across 11 library modules. That convention layer was added here.

**[LOW] Gradle wrapper uses `-bin` distribution**

`gradle-8.11.1-bin.zip` works but `-all` provides source for IDE navigation into Gradle internals. Minor.

---

### 2B. Runtime Performance

**[HIGH] Moshi reflection adapter (`KotlinJsonAdapterFactory`)**

`[DataModule.kt](apps/android-tv/core-data/src/main/java/plum/tv/core/data/di/DataModule.kt)` uses:

```kotlin
Moshi.Builder().addLast(KotlinJsonAdapterFactory()).build()
```

This uses Kotlin reflection for every JSON parse operation. With 30+ DTO classes, each containing 10-60 fields, this means:

- Startup overhead from reflection initialization
- Per-parse overhead from reflective field access
- APK bloat from pulling `kotlin-reflect` (~2MB)

**Fix:** Switched to `moshi-kotlin-codegen` with KSP. Added `@JsonClass(generateAdapter = true)` to DTOs and removed `KotlinJsonAdapterFactory()`.

**[HIGH] R8 / minification disabled in release**

`[app/build.gradle.kts](apps/android-tv/app/build.gradle.kts)` line 42:

```kotlin
release { isMinifyEnabled = false }
```

Without R8:

- `material-icons-extended` adds ~40MB of icon vectors (thousands of unused icons)
- All Compose compiler metadata, debug info, unused classes ship in the APK
- No code optimization or dead code elimination
- Significantly larger APK on constrained TV devices

**Fix:** Enabled `isMinifyEnabled = true` and `isShrinkResources = true` for release, with ProGuard rules in place for the relevant dependencies.

**[MEDIUM] Regex recompilation in hot path**

`[ShowGrouping.kt](apps/android-tv/core-network/src/main/java/plum/tv/core/network/ShowGrouping.kt)` line 5:

```kotlin
fun getShowName(title: String): String {
    val match = Regex("^(.+?)\\s*-\\s*S\\d+", RegexOption.IGNORE_CASE).find(title)
```

This compiles a new `Regex` object on every call. When browsing a library with hundreds of episodes, this is called per-item. The same regex exists as a file-level `val` in `[ShowKey.kt](apps/android-tv/core-network/src/main/java/plum/tv/core/network/ShowKey.kt)` line 3.

**Fix:** Extracted to a shared file-level helper.

**[LOW] `PlumPlayerController.close()` creates an orphan CoroutineScope**

`[PlumPlayerController.kt](apps/android-tv/core-player/src/main/java/plum/tv/core/player/PlumPlayerController.kt)` lines 762-788:

```kotlin
CoroutineScope(SupervisorJob() + Dispatchers.Default).launch { ... }
```

This scope was replaced with an application-scoped coroutine and a timeout-bound shutdown path.

---

### 2C. Architecture

**[MEDIUM] Duplicate `normalizeShowKeyTitle` function**

The private function `normalizeShowKeyTitle` is duplicated in two files:

- `[ShowGrouping.kt](apps/android-tv/core-network/src/main/java/plum/tv/core/network/ShowGrouping.kt)` line 9
- `[ShowKey.kt](apps/android-tv/core-network/src/main/java/plum/tv/core/network/ShowKey.kt)` line 5

Both do identical work. Extract to one shared location.

**[MEDIUM] Duplicate CastSection composable**

`[MovieDetailScreen.kt](apps/android-tv/feature-details/src/main/java/plum/tv/feature/details/MovieDetailScreen.kt)` lines 170-192 and `[ShowDetailScreen.kt](apps/android-tv/feature-details/src/main/java/plum/tv/feature/details/ShowDetailScreen.kt)` lines 311-358 contained near-identical `CastSection`/`ShowCastSection` composables. These were extracted to a shared composable in `core-ui`.

**[MEDIUM] Feature modules depend directly on network DTOs**

ViewModels expose network DTOs (`LibraryJson`, `MediaItemJson`, etc.) directly to UI. This couples the presentation layer tightly to the wire format. Not urgent to fix, but worth noting for when the API evolves.

**[LOW] Dead code files**

- `[PlumPlayerHolder.kt](apps/android-tv/core-player/src/main/java/plum/tv/core/player/PlumPlayerHolder.kt)` -- contains only `object PlumPlayerPlaceholder` (dead)
- `[LibraryPlaceholder.kt](apps/android-tv/feature-library/src/main/java/plum/tv/feature/library/LibraryPlaceholder.kt)` -- contains only a placeholder composable (dead)
- `DiscoverBrowseViewModel.goBack()` -- no-op method (dead)

**[LOW] Missing `consumer-rules.pro` files**

Four modules reference `consumerProguardFiles("consumer-rules.pro")` but the files don't exist: `:core-model`, `:core-network`, `:core-data`, `:core-player`. Either create empty files or remove the reference.

---

### 2D. Dependency Management

**[HIGH] `material-icons-extended` without R8**

`[app/build.gradle.kts](apps/android-tv/app/build.gradle.kts)` and `[core-ui/build.gradle.kts](apps/android-tv/core-ui/build.gradle.kts)` both pull `material-icons-extended`. Without R8, this adds ~40MB. With R8 enabled (fix 2B above), this shrinks to only the icons actually used.

**[MEDIUM] Coil 2.x is outdated**

The project uses `io.coil-kt:coil:2.7.0` / `coil-compose:2.7.0`. Coil 3.x has been stable for some time with Kotlin Multiplatform support and performance improvements. Not urgent, but a good future upgrade.

**[LOW] OkHttp version alignment**

`okhttp:4.12.0` is explicitly declared in multiple modules. Consider letting it resolve through the BOM or version catalog to avoid drift.

---

### 2E. Kotlin Code Quality

**[MEDIUM] Mixed synchronization primitives in `BrowseRepository`**

`[BrowseRepository.kt](apps/android-tv/core-data/src/main/java/plum/tv/core/data/BrowseRepository.kt)` uses:

- `synchronized(librariesCacheLock)` (Java monitor)
- `synchronized(mediaCacheLock)` (Java monitor)
- `Mutex()` (coroutine-aware) for `prefetchMutex`

For a class used exclusively from coroutines, prefer `Mutex` consistently or accept `synchronized` for simple cache-peek operations (which is actually fine since they're non-suspending). The current mix works but is inconsistent.

**[LOW] `SearchViewModel.onCleared()` is redundant**

`[SearchViewModel.kt](apps/android-tv/feature-search/src/main/java/plum/tv/feature/search/SearchViewModel.kt)` no longer needs manual cancellation because the search collection runs in `viewModelScope`.

---

### 2F. Concurrency / Coroutines

Generally solid. The project uses Dispatchers correctly, employs mutex where needed, and handles cancellation properly in WebSocket code. The main issues are covered in 2B (orphan scope in `close()`) and 2E (mixed synchronization).

---

### 2G. Testing

No test files exist in the project. No `testImplementation` or `androidTestImplementation` dependencies are declared. This is a gap but not the focus of this audit.

---

### 2H. Maintainability

The biggest maintainability issues are the build config duplication (2A) and the code duplication (2C). Everything else is reasonably clean and idiomatic.

---

## 3. What Changed

### Implemented

1. Gradle cache + configuration-cache properties were added to `gradle.properties`
2. Regex recompilation was fixed by sharing the show-title normalization helper
3. Dead code was removed: `PlumPlayerHolder.kt`, `LibraryPlaceholder.kt`, and the no-op `goBack()` path
4. The redundant `SearchViewModel.onCleared()` cancellation was removed
5. Missing `consumer-rules.pro` files were added
6. KAPT was migrated to KSP for Hilt
7. Moshi reflection was replaced with Moshi codegen
8. R8 minification and resource shrinking were enabled for release
9. A version catalog was added and build scripts were migrated to use it
10. The shared cast section was extracted into `core-ui`
11. `PlumPlayerController.close()` was moved onto an application scope and bounded with a timeout

### Left For Later

1. Coil 3.x upgrade
2. Domain model layer between network DTOs and UI
3. Any broader architecture changes that would require cross-cutting discussion

---

## 4. Concrete Changes

All of the originally planned quick wins and medium-effort items were implemented. The remaining larger refactors are listed above as future work.

### Change 1: gradle.properties -- enable caching

Added `org.gradle.caching=true` and `org.gradle.configuration-cache=true`.

### Change 2: KAPT to KSP migration

- Root: added `id("com.google.devtools.ksp")` plugin declaration, removed `kapt`
- Each of 7 modules: replaced kapt plugin + dependency with ksp

### Change 3: Moshi codegen migration

- Replaced `moshi-kotlin` (reflection) with `moshi` (base) + `moshi-kotlin-codegen` (KSP)
- Added `@JsonClass(generateAdapter = true)` to DTO classes
- Removed `KotlinJsonAdapterFactory()` from DataModule

### Change 4: Enable R8 in release

- Set `isMinifyEnabled = true`, `isShrinkResources = true`
- Added ProGuard rules for Moshi, Retrofit, Hilt

### Change 5: Version catalog

- Created `gradle/libs.versions.toml` with centralized versions
- Updated the Android TV build files to reference catalog entries

### Change 6: Fix Regex, dead code, deduplication

- Shared regex and `normalizeShowKeyTitle` between ShowGrouping/ShowKey
- Deleted dead files
- Extracted a shared CastSection composable

---

## 5. Final Assessment

**Already good:**

- Module structure and boundaries
- Compose theming and TV focus management
- WebSocket reconnection with coroutine cancellation
- Media caching with LRU evictor and low-RAM adaptation
- Image loading strategy with Coil + hardware bitmap detection
- StateFlow-driven UI architecture
- SearchViewModel debounce pattern

**Already improved:**

- KAPT to KSP (build speed)
- Moshi reflection to codegen (runtime perf)
- R8 minification for release (production readiness)
- Version catalog (maintainability)

**Not worth changing:**

- Module count is appropriate; no need to merge or split
- `synchronized` for simple non-suspending cache peeks is fine
- Data classes for DTOs (not worth sealed interfaces or value classes here)
- Navigation setup (string routes are standard for now)
- `BuildConfig` approach for dev defaults is practical
