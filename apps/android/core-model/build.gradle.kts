plugins {
    id("plum.android.library")
    alias(libs.plugins.kotlin.android)
}

android {
    namespace = "plum.tv.core.model"
    defaultConfig {
        consumerProguardFiles("consumer-rules.pro")
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
    }
}

dependencies {
    implementation(libs.coroutines.core)
}
