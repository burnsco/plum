plugins {
    id("plum.android.library")
    alias(libs.plugins.kotlin.android)
}

android {
    namespace = "plum.tv.core.model"
    defaultConfig {
        consumerProguardFiles("consumer-rules.pro")
    }
    kotlinOptions { jvmTarget = "17" }
}

dependencies {
    implementation(libs.coroutines.core)
}
