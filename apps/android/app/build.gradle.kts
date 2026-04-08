import java.util.Properties

fun loadLocalProperties(): Properties {
    val props = Properties()
    val localProps = file("../local.properties")
    if (localProps.exists()) {
        localProps.inputStream().use { props.load(it) }
    }
    return props
}

fun String.asBuildConfigString(): String = "\"" + replace("\\", "\\\\").replace("\"", "\\\"") + "\""

val localProps = loadLocalProperties()

fun localProperty(name: String): String =
    localProps.getProperty(name)
        ?.trim()
        ?.takeIf { it.isNotEmpty() }
        ?: ""

/** Matches web onboarding “Quick start with default admin” credentials for debug builds only. */
private fun debugDefaultAdminEmailBuildConfig(): String =
    localProperty("plumTv.defaultAdminEmail").ifEmpty { "admin@example.com" }.asBuildConfigString()

private fun debugDefaultAdminPasswordBuildConfig(): String =
    localProperty("plumTv.defaultAdminPassword").ifEmpty { "passwordpassword" }.asBuildConfigString()

plugins {
    id("plum.android.application")
    id("plum.android.compose")
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.hilt)
    alias(libs.plugins.ksp)
}

android {
    namespace = "plum.tv.app"
    defaultConfig {
        applicationId = "com.plum.android.tv"
        targetSdk = 36
        versionCode = 1
        versionName = "0.1.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"

        buildConfigField("String", "DEFAULT_SERVER_URL", localProperty("plumTv.defaultServerUrl").asBuildConfigString())
        buildConfigField("String", "DEFAULT_ADMIN_EMAIL", "\"\"")
        buildConfigField("String", "DEFAULT_ADMIN_PASSWORD", "\"\"")
    }
    // Release signing only when local.properties defines credentials (never commit keystores or passwords).
    val releaseStoreFile = localProperty("plumTv.releaseStoreFile")
    val releaseStorePassword = localProperty("plumTv.releaseStorePassword")
    val releaseKeyAlias = localProperty("plumTv.releaseKeyAlias")
    val releaseKeyPassword = localProperty("plumTv.releaseKeyPassword")
    signingConfigs {
        if (
            releaseStoreFile.isNotEmpty() &&
            releaseStorePassword.isNotEmpty() &&
            releaseKeyAlias.isNotEmpty() &&
            releaseKeyPassword.isNotEmpty()
        ) {
            val store = file(releaseStoreFile)
            if (store.isFile) {
                create("releaseLocal") {
                    this.storeFile = store
                    storePassword = releaseStorePassword
                    keyAlias = releaseKeyAlias
                    keyPassword = releaseKeyPassword
                }
            }
        }
    }
    buildTypes {
        debug {
            buildConfigField("String", "DEFAULT_ADMIN_EMAIL", debugDefaultAdminEmailBuildConfig())
            buildConfigField("String", "DEFAULT_ADMIN_PASSWORD", debugDefaultAdminPasswordBuildConfig())
        }
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            // Prefer local.properties release keystore; otherwise debug signing so assembleRelease is installable via adb.
            signingConfig =
                signingConfigs.findByName("releaseLocal")
                    ?: signingConfigs.getByName("debug")
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
        }
    }
    buildFeatures { buildConfig = true }
}

kotlin {
    compilerOptions {
        jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
    }
}

dependencies {
    implementation(project(":core-model"))
    implementation(project(":core-network"))
    implementation(project(":core-data"))
    implementation(project(":core-ui"))
    implementation(project(":core-player"))
    implementation(project(":feature-auth"))
    implementation(project(":feature-home"))
    implementation(project(":feature-library"))
    implementation(project(":feature-details"))
    implementation(project(":feature-search"))
    implementation(project(":feature-settings"))

    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.material3)
    implementation(libs.androidx.compose.icons.extended)
    implementation(libs.androidx.lifecycle.runtime.ktx)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.tv.foundation)
    implementation(libs.androidx.tv.material)
    implementation(libs.androidx.navigation.compose)
    implementation(libs.androidx.hilt.navigation.compose)
    implementation(libs.hilt.android)
    ksp(libs.hilt.compiler)
    implementation(libs.media3.exoplayer)
    implementation(libs.media3.exoplayer.hls)
    implementation(libs.media3.datasource)
    implementation(libs.media3.database)
    implementation(libs.media3.datasource.okhttp)
    implementation(libs.media3.ui)
    implementation(libs.okhttp)
    implementation(libs.coil)
    implementation(libs.coil.compose)
    implementation(libs.coil.network.okhttp)

    androidTestImplementation(platform(libs.androidx.compose.bom))
    androidTestImplementation("androidx.compose.ui:ui-test-junit4")
    androidTestImplementation("androidx.test.ext:junit:1.2.1")
    androidTestImplementation("androidx.test:runner:1.6.2")
    androidTestImplementation(libs.androidx.datastore.preferences)
    androidTestImplementation(libs.coroutines.android)
    androidTestImplementation(libs.okhttp.mockwebserver)
}
