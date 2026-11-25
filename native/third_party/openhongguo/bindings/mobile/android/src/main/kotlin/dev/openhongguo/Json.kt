package dev.openhongguo

import org.json.JSONArray
import org.json.JSONObject

// Types.kt 解码用的辅助函数：值缺省、为 null 或类型不符时取零值，
// 与 Go 的 encoding/json 输出的零值规则一致。

internal fun Any?.asString(): String = this as? String ?: ""

internal fun Any?.asBoolean(): Boolean = this as? Boolean ?: false

internal fun Any?.asInt(): Int = (this as? Number)?.toInt() ?: 0

internal fun Any?.asLong(): Long = (this as? Number)?.toLong() ?: 0L

internal fun Any?.asDouble(): Double = (this as? Number)?.toDouble() ?: 0.0

internal fun Any?.asObject(): JSONObject = this as? JSONObject ?: JSONObject()

internal inline fun <T> Any?.asList(f: (Any?) -> T): List<T> {
    val a = this as? JSONArray ?: return emptyList()
    return List(a.length()) { f(a.opt(it)) }
}

internal inline fun <T> Any?.asMap(f: (Any?) -> T): Map<String, T> {
    val o = this as? JSONObject ?: return emptyMap()
    return o.keys().asSequence().associateWith { f(o.opt(it)) }
}
