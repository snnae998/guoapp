package dev.openhongguo.reactnative

import dev.openhongguo.core.Core
import dev.openhongguo.core.Logger
import expo.modules.kotlin.Promise
import expo.modules.kotlin.modules.Module
import expo.modules.kotlin.modules.ModuleDefinition
import java.io.File
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicInteger
import org.json.JSONObject

/**
 * react-native-openhongguo 的原生层，直接调用 gomobile 编译的 Go core。
 * SDK 的每个方法都经过 call(client, method, argsJson)，结果是 core 返回的
 * JSON（{"result": …} 或 {"error": …}），由 JS 端解码。
 */
class OpenhongguoModule : Module() {
  private val clients = ConcurrentHashMap<Int, dev.openhongguo.core.Client>()
  private val nextId = AtomicInteger()

  override fun definition() = ModuleDefinition {
    Name("Openhongguo")

    Events("log")

    // 不参与系统备份，设备档案不会随备份迁移到别的手机。
    Function("defaultProfilePath") {
      val context = appContext.reactContext ?: throw IllegalStateException("no React context")
      File(context.noBackupFilesDir, "openhongguo/anonymous.json").path
    }

    // 返回 {"id": n} 或 {"error": {...}} 形式的 JSON，错误码不会丢。
    Function("createClient") { profilePath: String, configJson: String, log: Boolean ->
      val id = nextId.incrementAndGet()
      val logger = if (log) Logger { line -> sendEvent("log", mapOf("client" to id, "line" to line)) } else null
      try {
        clients[id] = Core.newClient(profilePath, configJson, logger)
        """{"id":$id}"""
      } catch (e: Exception) {
        val msg = e.message.orEmpty()
        if (msg.startsWith("{")) """{"error":$msg}""" else """{"error":{"code":"UNKNOWN","message":${JSONObject.quote(msg)}}}"""
      }
    }

    Function("releaseClient") { id: Int ->
      clients.remove(id)
      Unit
    }

    AsyncFunction("call") { id: Int, method: String, argsJson: String, promise: Promise ->
      val client = clients[id]
      if (client == null) {
        promise.resolve("""{"error":{"code":"INVALID_OPTIONS","message":"openhongguo: client was released"}}""")
        return@AsyncFunction
      }
      client.start(method, argsJson) { promise.resolve(it) }
    }
  }
}
