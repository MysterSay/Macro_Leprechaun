#!/usr/bin/env bash
# JDK 17+, SDK platform android-35, build-tools 35.0.0. No Gradle needed.
set -euo pipefail
cd "$(dirname "$0")"
: "${ANDROID_HOME:?Set ANDROID_HOME to the Android SDK root}"
: "${MACRO_KEYSTORE:?Set MACRO_KEYSTORE to your signing keystore}"
: "${MACRO_KEY_ALIAS:?Set MACRO_KEY_ALIAS}"
: "${MACRO_STORE_PASSWORD:?Set MACRO_STORE_PASSWORD (not committed)}"
bt="$ANDROID_HOME/build-tools/35.0.0"
jar="$ANDROID_HOME/platforms/android-35/android.jar"
mkdir -p out/classes out/dex
javac -encoding UTF-8 -source 17 -target 17 -classpath "$jar" -d out/classes app/src/main/java/ua/macroleprechaun/MainActivity.java
sed 's/<manifest /<manifest package="ua.macroleprechaun" /' app/src/main/AndroidManifest.xml > out/AndroidManifest.xml
"$bt/aapt" package -f -M out/AndroidManifest.xml -S app/src/main/res -A app/src/main/assets -I "$jar" --min-sdk-version 26 --target-sdk-version 34 --version-code 5 --version-name 0.5.0 -F out/unsigned.apk
find out/classes -name '*.class' -print0 | xargs -0 "$bt/d8" --lib "$jar" --min-api 26 --output out/dex
(cd out/dex && zip -q -j ../unsigned.apk classes.dex)
"$bt/zipalign" -f 4 out/unsigned.apk out/aligned.apk
"$bt/apksigner" sign --ks "$MACRO_KEYSTORE" --ks-key-alias "$MACRO_KEY_ALIAS" --ks-pass env:MACRO_STORE_PASSWORD --out ../build/MacroLeprechaun.apk out/aligned.apk
"$bt/apksigner" verify ../build/MacroLeprechaun.apk
