import Flutter
import Foundation
import OpenhongguoCore

/// openhongguo Flutter 插件的 iOS 端，直接调用 gomobile 编译的 Go core。
/// SDK 的每个方法都经过 call(client, method, args)，结果是 core 返回的 JSON
/// （{"result": …} 或 {"error": …}），由 Dart 端解码。
public class OpenhongguoPlugin: NSObject, FlutterPlugin {
  private let channel: FlutterMethodChannel
  private let lock = NSLock()
  private var clients: [Int: CoreClient] = [:]
  private var nextId = 0

  init(channel: FlutterMethodChannel) {
    self.channel = channel
  }

  public static func register(with registrar: FlutterPluginRegistrar) {
    let channel = FlutterMethodChannel(name: "dev.openhongguo/flutter", binaryMessenger: registrar.messenger())
    registrar.addMethodCallDelegate(OpenhongguoPlugin(channel: channel), channel: channel)
  }

  public func handle(_ call: FlutterMethodCall, result: @escaping FlutterResult) {
    let args = call.arguments as? [String: Any] ?? [:]
    switch call.method {
    case "createClient":
      // 返回 {"id": n} 或 {"error": {...}} 形式的 JSON，错误码不会丢。
      do {
        let profile = try (args["profilePath"] as? String) ?? Self.defaultProfilePath()
        lock.lock()
        nextId += 1
        let id = nextId
        lock.unlock()
        let logger = (args["log"] as? Bool) == true
          ? Logger { [weak self] line in
            DispatchQueue.main.async { self?.channel.invokeMethod("log", arguments: ["client": id, "line": line]) }
          }
          : nil
        var err: NSError?
        guard let client = CoreNewClient(profile, args["config"] as? String ?? "", logger, &err) else {
          result(Self.errorJSON(err?.localizedDescription ?? "unknown error"))
          return
        }
        lock.lock()
        clients[id] = client
        lock.unlock()
        result("{\"id\":\(id)}")
      } catch {
        result(Self.errorJSON(error.localizedDescription))
      }
    case "call":
      lock.lock()
      let client = clients[args["client"] as? Int ?? 0]
      lock.unlock()
      guard let client else {
        result("{\"error\":{\"code\":\"INVALID_OPTIONS\",\"message\":\"openhongguo: client was released\"}}")
        return
      }
      client.start(args["method"] as? String ?? "", argsJSON: args["args"] as? String ?? "", callback: Callback { out in
        // 结果要在主线程交回。
        DispatchQueue.main.async { result(out) }
      })
    case "releaseClient":
      lock.lock()
      clients[args["client"] as? Int ?? 0] = nil
      lock.unlock()
      result(nil)
    default:
      result(FlutterMethodNotImplemented)
    }
  }

  // Application Support 下的目录不参与备份，设备档案不会迁移到别的设备。
  private static func defaultProfilePath() throws -> String {
    var dir = try FileManager.default.url(
      for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true
    ).appendingPathComponent("openhongguo", isDirectory: true)
    try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    var values = URLResourceValues()
    values.isExcludedFromBackup = true
    try dir.setResourceValues(values)
    return dir.appendingPathComponent("anonymous.json").path
  }

  private static func errorJSON(_ msg: String) -> String {
    if msg.hasPrefix("{") { return "{\"error\":\(msg)}" }
    let quoted = (try? JSONEncoder().encode(msg)).flatMap { String(data: $0, encoding: .utf8) } ?? "\"\""
    return "{\"error\":{\"code\":\"UNKNOWN\",\"message\":\(quoted)}}"
  }
}

private final class Callback: NSObject, CoreCallbackProtocol {
  let handler: (String) -> Void
  init(_ handler: @escaping (String) -> Void) { self.handler = handler }
  func done(_ resultJSON: String?) { handler(resultJSON ?? "") }
}

private final class Logger: NSObject, CoreLoggerProtocol {
  let sink: (String) -> Void
  init(_ sink: @escaping (String) -> Void) { self.sink = sink }
  func log(_ line: String?) { sink(line ?? "") }
}
