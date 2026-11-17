part of '../openhongguo.dart';

// types.dart 解码用的辅助函数：值缺省、为 null 或类型不符时取零值，
// 与 Go 的 encoding/json 输出的零值规则一致。

String _asString(Object? v) => v is String ? v : '';

bool _asBool(Object? v) => v is bool ? v : false;

int _asInt(Object? v) => v is num ? v.toInt() : 0;

// 生成器遇到浮点数和 map 字段时才用到，现在的类型里没有这两种字段。
// ignore: unused_element
double _asDouble(Object? v) => v is num ? v.toDouble() : 0.0;

Map<String, dynamic> _asMap(Object? v) => v is Map ? v.cast<String, dynamic>() : const {};

List<T> _asList<T>(Object? v, T Function(Object? e) f) => v is List ? v.map(f).toList() : const [];

// 生成器遇到浮点数和 map 字段时才用到，现在的类型里没有这两种字段。
// ignore: unused_element
Map<String, T> _asMapOf<T>(Object? v, T Function(Object? e) f) =>
    v is Map ? v.map((k, e) => MapEntry(k as String, f(e))) : const {};
