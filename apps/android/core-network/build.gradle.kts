plugins {
    id("plum.android.library")
    alias(libs.plugins.ksp)
}

android {
    namespace = "plum.tv.core.network"
    defaultConfig {
        consumerProguardFiles("consumer-rules.pro")
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
        // Moshi KSP emits `.toInt()` on int literals in bitmask code; FIR flags it as redundant noise.
        freeCompilerArgs.add("-Xwarning-level=REDUNDANT_CALL_OF_CONVERSION_METHOD:disabled")
    }
}

dependencies {
    implementation(project(":core-model"))
    api(libs.retrofit)
    implementation(libs.retrofit.converter.moshi)
    implementation(libs.okhttp)
    implementation(libs.okhttp.logging.interceptor)
    implementation(libs.moshi)
    ksp(libs.moshi.kotlin.codegen)

    testImplementation(libs.junit)
}
