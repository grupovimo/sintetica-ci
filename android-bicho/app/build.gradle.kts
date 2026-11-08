plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

// El bicho en un celu viejo: pantalla completa, el núcleo de SintéticaOS por NDK
// y los sensores del celu como sentidos.
android {
    namespace = "com.sinteticalabs.bicho"
    compileSdk = 35
    ndkVersion = "27.2.12479018"

    defaultConfig {
        applicationId = "com.sinteticalabs.bicho"
        minSdk = 24
        targetSdk = 35
        versionCode = 1
        versionName = "0.1.0"
        ndk { abiFilters += listOf("armeabi-v7a", "arm64-v8a") }
        externalNativeBuild { cmake { cppFlags += listOf("-std=c++17", "-fno-exceptions", "-fno-rtti") } }
    }

    externalNativeBuild {
        cmake {
            path = file("src/main/cpp/CMakeLists.txt")
            version = "3.22.1"
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions { jvmTarget = "17" }
    testOptions { unitTests.isReturnDefaultValues = true }
}

dependencies {
    implementation("com.google.mlkit:face-detection:16.1.7")
    implementation("org.eclipse.paho:org.eclipse.paho.client.mqttv3:1.2.5")
    testImplementation("junit:junit:4.13.2")
}
