/// 红果短剧（Hongguo）的非官方 Flutter SDK：匿名浏览、搜索与播放。
///
/// 原生层是 gomobile 编译的 Go SDK（Android 的 .so，iOS 的 xcframework），
/// 通过平台通道调用。接口与 Go、Node.js、Kotlin 和 Swift 版一一对应。
library;

import 'dart:async' as async;
import 'dart:convert';

import 'package:flutter/services.dart';

part 'src/client.dart';
part 'src/json.dart';
part 'src/types.dart';
