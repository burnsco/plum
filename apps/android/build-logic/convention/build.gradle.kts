import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    `kotlin-dsl`
}

group = "plum.android.buildlogic"

java {
    sourceCompatibility = JavaVersion.VERSION_17
    targetCompatibility = JavaVersion.VERSION_17
}

kotlin {
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_17)
    }
}

dependencies {
    compileOnly(libs.agp.gradle)
    compileOnly(libs.kotlin.gradle.plugin)
}

gradlePlugin {
    plugins {
        register("androidApplication") {
            id = "plum.android.application"
            implementationClass = "plum.android.buildlogic.AndroidApplicationConventionPlugin"
        }
        register("androidLibrary") {
            id = "plum.android.library"
            implementationClass = "plum.android.buildlogic.AndroidLibraryConventionPlugin"
        }
        register("androidCompose") {
            id = "plum.android.compose"
            implementationClass = "plum.android.buildlogic.AndroidComposeConventionPlugin"
        }
    }
}
