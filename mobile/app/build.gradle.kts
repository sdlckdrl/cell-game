import java.util.Properties

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

val appUrlProps = Properties().apply {
    val configFile = rootProject.file("app-url.properties")
    if (configFile.exists()) {
        configFile.inputStream().use(::load)
    }
}

val keystoreProps = Properties().apply {
    val configFile = rootProject.file("keystore.properties")
    if (configFile.exists()) {
        configFile.inputStream().use(::load)
    }
}

fun quotedConfig(name: String, fallback: String): String {
    val raw = (appUrlProps.getProperty(name) ?: fallback).trim()
    val escaped = raw.replace("\\", "\\\\").replace("\"", "\\\"")
    return "\"$escaped\""
}

val releaseStoreFile = keystoreProps.getProperty("storeFile")?.trim().orEmpty()
val releaseStorePassword = keystoreProps.getProperty("storePassword")?.trim().orEmpty()
val releaseKeyAlias = keystoreProps.getProperty("keyAlias")?.trim().orEmpty()
val releaseKeyPassword = keystoreProps.getProperty("keyPassword")?.trim().orEmpty()
val hasReleaseSigning =
    releaseStoreFile.isNotBlank() &&
        releaseStorePassword.isNotBlank() &&
        releaseKeyAlias.isNotBlank() &&
        releaseKeyPassword.isNotBlank()

android {
    namespace = "com.cellgame.mobile"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.cellgame.mobile"
        minSdk = 24
        targetSdk = 36
        versionCode = 2
        versionName = "1.0.1"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        vectorDrawables.useSupportLibrary = true
    }

    signingConfigs {
        if (hasReleaseSigning) {
            create("release") {
                storeFile = rootProject.file(releaseStoreFile)
                storePassword = releaseStorePassword
                keyAlias = releaseKeyAlias
                keyPassword = releaseKeyPassword
            }
        }
    }

    buildTypes {
        debug {
            applicationIdSuffix = ".debug"
            versionNameSuffix = "-debug"
            buildConfigField("String", "GAME_BASE_URL", quotedConfig("debugBaseUrl", "http://10.0.2.2:8000"))
        }

        release {
            isMinifyEnabled = false
            if (hasReleaseSigning) {
                signingConfig = signingConfigs.getByName("release")
            }
            buildConfigField("String", "GAME_BASE_URL", quotedConfig("releaseBaseUrl", "https://your-domain.example.com"))
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
        }
    }

    buildFeatures {
        buildConfig = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_11
        targetCompatibility = JavaVersion.VERSION_11
    }

    kotlinOptions {
        jvmTarget = "11"
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.10.1")
    implementation("androidx.appcompat:appcompat:1.6.1")
    implementation("com.google.android.material:material:1.10.0")
}
