package plum.android.buildlogic

import com.android.build.api.dsl.ApplicationExtension
import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.kotlin.dsl.configure

private const val compileSdkVersion = 35
private const val minSdkVersion = 24
private val lintRules =
    listOf(
        "NullSafeMutableLiveData",
    )

class AndroidApplicationConventionPlugin : Plugin<Project> {
    override fun apply(target: Project) = with(target) {
        pluginManager.apply("com.android.application")
        pluginManager.apply("org.jetbrains.kotlin.android")

        extensions.configure<ApplicationExtension> {
            compileSdk = compileSdkVersion
            defaultConfig {
                minSdk = minSdkVersion
            }
            compileOptions {
                sourceCompatibility = org.gradle.api.JavaVersion.VERSION_17
                targetCompatibility = org.gradle.api.JavaVersion.VERSION_17
            }
            lint.disable += lintRules
        }
    }
}
