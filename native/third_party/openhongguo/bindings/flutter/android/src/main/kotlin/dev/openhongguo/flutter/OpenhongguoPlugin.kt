package dev.openhongguo.flutter

import android.os.Handler
import android.os.Looper
import dev.openhongguo.core.Core
import dev.openhongguo.core.Logger
import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import java.io.File
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicInteger
import org.json.JSONObject

/**
 * openhongguo Flutter 插件的 Android 端，直接调用 gomobile 编译的 Go core。
 * SDK 的每个方法都经过 call(client, method, args)，结果是 core 返回的 JSON
 * （{"result": …} 或 {"error": …}），由 Dart 端解码。
 */
class OpenhongguoPlugin : FlutterPlugin, MethodChannel.MethodCallHandler {
    private lateinit var channel: MethodChannel
    private lateinit var noBackupDir: File
    private val clients = ConcurrentHashMap<Int, dev.openhongguo.core.Client>()
    private val nextId = AtomicInteger()
    private val main = Handler(Looper.getMainLooper())

    override fun onAttachedToEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        noBackupDir = binding.applicationContext.noBackupFilesDir
        channel = MethodChannel(binding.binaryMessenger, "dev.openhongguo/flutter")
        channel.setMethodCallHandler(this)
    }

    override fun onDetachedFromEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        channel.setMethodCallHandler(null)
        clients.clear()
    }

    override fun onMethodCall(call: MethodCall, result: MethodChannel.Result) {
        when (call.method) {
            // 返回 {"id": n} 或 {"error": {...}} 形式的 JSON，错误码不会丢。
            "createClient" -> {
                // 默认放在不参与系统备份的目录，设备档案不会随备份迁移到别的手机。
                val profile = call.argument<String>("profilePath")
                    ?: File(noBackupDir, "openhongguo/anonymous.json").path
                val id = nextId.incrementAndGet()
                val logger = if (call.argument<Boolean>("log") == true) {
                    Logger { line -> main.post { channel.invokeMethod("log", mapOf("client" to id, "line" to line)) } }
                } else {
                    null
                }
                try {
                    clients[id] = Core.newClient(profile, call.argument<String>("config").orEmpty(), logger)
                    result.success("""{"id":$id}""")
                } catch (e: Exception) {
                    val msg = e.message.orEmpty()
                    result.success(
                        if (msg.startsWith("{")) """{"error":$msg}"""
                        else """{"error":{"code":"UNKNOWN","message":${JSONObject.quote(msg)}}}""",
                    )
                }
            }
            "call" -> {
                val client = clients[call.argument<Int>("client")]
                if (client == null) {
                    result.success("""{"error":{"code":"INVALID_OPTIONS","message":"openhongguo: client was released"}}""")
                    return
                }
                client.start(call.argument<String>("method").orEmpty(), call.argument<String>("args").orEmpty()) { out ->
                    // 结果要在主线程交回。
                    main.post { result.success(out) }
                }
            }
            "releaseClient" -> {
                clients.remove(call.argument<Int>("client"))
                result.success(null)
            }
            else -> result.notImplemented()
        }
    }
}
