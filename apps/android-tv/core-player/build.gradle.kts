plugins {
    id("plum.android.library")
    alias(libs.plugins.kotlin.android)
}

android {
    namespace = "plum.tv.core.player"
    defaultConfig {
        consumerProguardFiles("consumer-rules.pro")
    }
    kotlinOptions { jvmTarget = "17" }
}

dependencies {
    implementation(project(":core-data"))
    implementation(project(":core-network"))
    implementation(libs.media3.exoplayer)
    implementation(libs.media3.exoplayer.hls)
    implementation(libs.media3.ui)
    implementation(libs.coroutines.android)
}
