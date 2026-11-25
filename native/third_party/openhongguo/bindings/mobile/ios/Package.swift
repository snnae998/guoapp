// swift-tools-version:5.9
// openhongguo 的 iOS 和 macOS 库。先运行 ../build.sh ios，生成 Go 编译出的
// OpenhongguoCore.xcframework。
import PackageDescription

let package = Package(
    name: "Openhongguo",
    platforms: [.iOS(.v13), .macOS(.v11)],
    products: [
        .library(name: "Openhongguo", targets: ["Openhongguo"]),
    ],
    targets: [
        .binaryTarget(name: "OpenhongguoCore", path: "OpenhongguoCore.xcframework"),
        .target(name: "Openhongguo", dependencies: ["OpenhongguoCore"]),
        .testTarget(name: "OpenhongguoTests", dependencies: ["Openhongguo"]),
    ]
)
