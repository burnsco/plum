plugins {
    id("plum.android.library")
    id("plum.android.compose")
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.kotlin.compose)
}

android {
    namespace = "plum.tv.core.ui"
}

kotlin {
    compilerOptions {
        jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
    }
}

dependencies {
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.material3)
    implementation(libs.androidx.compose.icons.extended)
    implementation(libs.androidx.compose.foundation)
    implementation(libs.androidx.tv.foundation)
    implementation(libs.androidx.tv.material)
    implementation(libs.coil.compose)
}
