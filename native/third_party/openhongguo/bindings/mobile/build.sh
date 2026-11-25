#!/bin/sh
# build.sh 用 gomobile 编译 Go core，并放进各个移动端的包。
#
#   ./build.sh android        Kotlin 库：android/build/outputs/aar/openhongguo-release.aar
#   ./build.sh ios            Swift 包：ios/OpenhongguoCore.xcframework
#   ./build.sh react-native   ../react-native 的 android/ 和 ios/
#   ./build.sh flutter        ../flutter 的 android/ 和 ios/
#   ./build.sh                以上全部
#
# 需要 gomobile（go install golang.org/x/mobile/cmd/gomobile@latest）。
# Android 还需要 ANDROID_HOME、Android NDK 和 JDK 17 以上；iOS 需要 Xcode。
set -eu

here=$(cd "$(dirname "$0")" && pwd)
core="$here/core"
out="$here/build"

# core_android 编译 Android 版 core，拆成 $out/android/core.jar 和 jni/。
# Gradle 不能把本地 aar 打进另一个 aar，所以各个库都直接用 jar 和 .so。
core_android() {
	[ -f "$out/android/core.jar" ] && return
	: "${ANDROID_HOME:?set ANDROID_HOME to the Android SDK}"
	if [ -z "${ANDROID_NDK_HOME:-}" ]; then
		ANDROID_NDK_HOME=$(ls -d "$ANDROID_HOME"/ndk/* | sort -V | tail -1)
		export ANDROID_NDK_HOME
	fi
	rm -rf "$out/android"
	mkdir -p "$out/android"
	(cd "$core" && gomobile bind -target android -androidapi 21 -javapkg dev.openhongguo \
		-trimpath -ldflags="-s -w" -o "$out/android/core.aar" .)
	(cd "$out/android" && unzip -q core.aar classes.jar 'jni/*' && mv classes.jar core.jar)
}

# core_ios 编译 iOS 和 macOS 版 core：$out/OpenhongguoCore.xcframework。
core_ios() {
	[ -d "$out/OpenhongguoCore.xcframework" ] && return
	mkdir -p "$out"
	(cd "$core" && gomobile bind -target ios,iossimulator,macos -iosversion 13 \
		-trimpath -ldflags="-s -w" -o "$out/OpenhongguoCore.xcframework" .)
}

# put_android DIR 把 core 放进 Android 库工程 DIR。
put_android() {
	rm -rf "$1/libs/core.jar" "$1/src/main/jniLibs"
	mkdir -p "$1/libs" "$1/src/main/jniLibs"
	cp "$out/android/core.jar" "$1/libs/core.jar"
	cp -R "$out/android/jni/." "$1/src/main/jniLibs/"
}

# put_ios DIR 把 xcframework 放进 DIR。
put_ios() {
	rm -rf "$1/OpenhongguoCore.xcframework"
	cp -R "$out/OpenhongguoCore.xcframework" "$1/"
}

# put_ios_only DIR 把去掉 macOS 部分的 xcframework 放进 DIR。React Native 和
# Flutter 只支持 iOS，macOS 部分约 18 MB，只会增大包的体积。
put_ios_only() {
	put_ios "$1"
	xcf="$1/OpenhongguoCore.xcframework"
	rm -rf "$xcf"/macos-*
	python3 -I - "$xcf/Info.plist" <<'PY'
import plistlib, sys
path = sys.argv[1]
with open(path, "rb") as f:
    info = plistlib.load(f)
info["AvailableLibraries"] = [l for l in info["AvailableLibraries"] if l["SupportedPlatform"] != "macos"]
with open(path, "wb") as f:
    plistlib.dump(info, f)
PY
}

android() {
	core_android
	put_android "$here/android"
	(cd "$here/android" && ./gradlew --quiet test assembleRelease)
	echo "built $here/android/build/outputs/aar/openhongguo-release.aar"
}

ios() {
	core_ios
	put_ios "$here/ios"
	echo "built $here/ios/OpenhongguoCore.xcframework"
}

react_native() {
	core_android
	put_android "$here/../react-native/android"
	if [ "$(uname)" = Darwin ]; then
		core_ios
		put_ios_only "$here/../react-native/ios"
	fi
	echo "prepared $(cd "$here/../react-native" && pwd)"
}

flutter() {
	core_android
	put_android "$here/../flutter/android"
	if [ "$(uname)" = Darwin ]; then
		core_ios
		put_ios_only "$here/../flutter/ios"
	fi
	echo "prepared $(cd "$here/../flutter" && pwd)"
}

# 每次运行都重新编译 core，避免用上旧的产物。
rm -rf "$out"
case "${1:-all}" in
android) android ;;
ios) ios ;;
react-native) react_native ;;
flutter) flutter ;;
all) android && ios && react_native && flutter ;;
*)
	echo "usage: $0 [android|ios|react-native|flutter|all]" >&2
	exit 2
	;;
esac
