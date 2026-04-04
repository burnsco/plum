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

/** Matches web onboarding “Quick start with default admin” (dev) credentials. */
private fun defaultAdminEmailBuildConfig(): String =
    localProperty("plumTv.defaultAdminEmail").ifEmpty { "admin@example.com" }.asBuildConfigString()

private fun defaultAdminPasswordBuildConfig(): String =
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
        targetSdk = 35
        versionCode = 1
        versionName = "0.1.0"

        buildConfigField("String", "DEFAULT_SERVER_URL", localProperty("plumTv.defaultServerUrl").asBuildConfigString())
        buildConfigField("String", "DEFAULT_ADMIN_EMAIL", defaultAdminEmailBuildConfig())
        buildConfigField("String", "DEFAULT_ADMIN_PASSWORD", defaultAdminPasswordBuildConfig())
    }
    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
        }
    }
    buildFeatures { buildConfig = true }
    kotlinOptions { jvmTarget = "17" }
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
}
