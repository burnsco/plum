pluginManagement {
    includeBuild("build-logic")
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}
plugins {
    id("org.gradle.toolchains.foojay-resolver-convention") version "1.0.0"
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
    }
}

rootProject.name = "plum-android-tv"

include(
    ":app",
    ":core-model",
    ":core-network",
    ":core-data",
    ":core-ui",
    ":core-player",
    ":feature-auth",
    ":feature-home",
    ":feature-library",
    ":feature-details",
    ":feature-search",
    ":feature-settings",
)
