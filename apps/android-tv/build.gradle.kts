import com.android.build.api.dsl.ApplicationExtension
import com.android.build.api.dsl.LibraryExtension

plugins {
    id("com.android.application") version "8.7.2" apply false
    id("com.android.library") version "8.7.2" apply false
    id("org.jetbrains.kotlin.android") version "2.0.21" apply false
    id("org.jetbrains.kotlin.plugin.compose") version "2.0.21" apply false
    id("com.google.dagger.hilt.android") version "2.51.1" apply false
    id("org.jetbrains.kotlin.kapt") version "2.0.21" apply false
}

subprojects {
    pluginManager.withPlugin("com.android.application") {
        extensions.configure<ApplicationExtension> {
            lint.disable += "NullSafeMutableLiveData"
            lint.disable += "RememberInComposition"
            lint.disable += "FrequentlyChangingValue"
        }
    }

    pluginManager.withPlugin("com.android.library") {
        extensions.configure<LibraryExtension> {
            lint.disable += "NullSafeMutableLiveData"
            lint.disable += "RememberInComposition"
            lint.disable += "FrequentlyChangingValue"
        }
    }
}
