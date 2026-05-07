plugins {
    alias(libs.plugins.android.application) apply false
    alias(libs.plugins.android.library) apply false
    alias(libs.plugins.hilt) apply false
    alias(libs.plugins.ksp) apply false
    alias(libs.plugins.detekt) apply false
    alias(libs.plugins.spotless) apply false
}

// Built-in Kotlin can request compose-group-mapping at an unpublished version; pin to catalog Kotlin.
subprojects {
    val libsCatalog = rootProject.extensions.getByType<org.gradle.api.artifacts.VersionCatalogsExtension>().named("libs")
    val detektVersion = libsCatalog.findVersion("detekt").get().requiredVersion
    val kotlinVersion = libsCatalog.findVersion("kotlin").get().requiredVersion
    val ktlintVersion = libsCatalog.findVersion("ktlint").get().requiredVersion
    val ktlintEditorConfig =
        mapOf(
            "ktlint_code_style" to "android_studio",
            "ktlint_standard_backing-property-naming" to "disabled",
            "ktlint_standard_filename" to "disabled",
            "ktlint_standard_function-naming" to "disabled",
            "ktlint_standard_max-line-length" to "disabled",
        )

    pluginManager.apply("dev.detekt")
    pluginManager.apply("com.diffplug.spotless")

    extensions.configure<dev.detekt.gradle.extensions.DetektExtension>("detekt") {
        toolVersion = detektVersion
        source.setFrom("src/main/java", "src/test/java", "src/androidTest/java")
        config.setFrom(rootProject.files("config/detekt/detekt.yml"))
        buildUponDefaultConfig = true
        allRules = false
        parallel = true
        basePath.set(rootProject.projectDir)
    }

    tasks.withType<dev.detekt.gradle.Detekt>().configureEach {
        jvmTarget.set("17")
        exclude("**/build/**")
        reports {
            checkstyle.required.set(true)
            html.required.set(true)
            sarif.required.set(true)
            markdown.required.set(false)
        }
    }

    extensions.configure<com.diffplug.gradle.spotless.SpotlessExtension>("spotless") {
        kotlin {
            target("src/**/*.kt")
            ktlint(ktlintVersion).editorConfigOverride(ktlintEditorConfig)
            trimTrailingWhitespace()
            endWithNewline()
        }
        kotlinGradle {
            target("*.gradle.kts")
            ktlint(ktlintVersion).editorConfigOverride(ktlintEditorConfig)
            trimTrailingWhitespace()
            endWithNewline()
        }
    }

    configurations.configureEach {
        resolutionStrategy.eachDependency {
            if (requested.group == "org.jetbrains.kotlin" && requested.name == "compose-group-mapping") {
                useVersion(kotlinVersion)
            }
        }
    }
}
