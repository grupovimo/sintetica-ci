pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}
dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
        // Solo para el audioswitch que usa LiveKit (lo trae el SDK de ElevenLabs).
        maven("https://jitpack.io") { content { includeGroup("com.github.davidliu") } }
    }
}
rootProject.name = "Sintetica"
include(":app")
