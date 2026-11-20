require 'json'

package = JSON.parse(File.read(File.join(__dir__, '..', 'package.json')))

Pod::Spec.new do |s|
  s.name           = 'OpenhongguoReactNative'
  s.version        = package['version']
  s.summary        = package['description']
  s.license        = package['license']
  s.homepage       = 'https://github.com/beilunyang/openhongguo'
  s.author         = 'beilunyang'
  s.platforms      = { :ios => '15.1' }
  s.source         = { :git => 'https://github.com/beilunyang/openhongguo.git' }
  s.static_framework = true
  s.swift_version  = '5.9'

  s.dependency 'ExpoModulesCore'
  s.source_files = '*.swift'
  # gomobile 编译的 Go core，由 bindings/mobile/build.sh react-native 放进来。
  s.vendored_frameworks = 'OpenhongguoCore.xcframework'
end
