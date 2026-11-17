import ExpoModulesCore
import Foundation
import OpenhongguoCore

/// react-native-openhongguo 的原生层，直接调用 gomobile 编译的 Go core。
/// SDK 的每个方法都经过 call(client, method, argsJson)，结果是 core 返回的
/// JSON（{"result": …} 或 {"error": …}），由 JS 端解码。
public class OpenhongguoModule: Module {
  private let lock = NSLock()
  private var clients: [Int: CoreClient] = [:]
  private var nextId = 0

  public func definition() -> ModuleDefinition {
    Name("Openhongguo")

    Events("log")

    // Application Support 下的目录不参与备份，设备档案不会迁移到别的设备。
    Function("defaultProfilePath") { () throws -> String in
      var dir = try FileManager.default.url(
        for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true
      ).appendingPathComponent("openhongguo", isDirectory: true)
      try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
      var values = URLResourceValues()
      values.isExcludedFromBackup = true
      try dir.setResourceValues(values)
      return dir.appendingPathComponent("anonymous.json").path
    }

    // 返回 {"id": n} 或 {"error": {...}} 形式的 JSON，错误码不会丢。
    Function("createClient") { (profilePath: String, configJson: String, log: Bool) -> String in
      self.lock.lock()
      self.nextId += 1
      let id = self.nextId
      self.lock.unlock()
      let logger = log ? Logger { [weak self] line in self?.sendEvent("log", ["client": id, "line": line]) } : nil
      var err: NSError?
      guard let client = CoreNewClient(profilePath, configJson, logger, &err) else {
        let msg = err?.localizedDescription ?? "unknown error"
        if msg.hasPrefix("{") { return "{\"error\":\(msg)}" }
        let quoted = String(data: try JSONEncoder().encode(msg), encoding: .utf8) ?? "\"\""
        return "{\"error\":{\"code\":\"UNKNOWN\",\"message\":\(quoted)}}"
      }
      self.lock.lock()
      self.clients[id] = client
      self.lock.unlock()
      return "{\"id\":\(id)}"
    }

    Function("releaseClient") { (id: Int) in
      self.lock.lock()
      self.clients[id] = nil
      self.lock.unlock()
    }

    AsyncFunction("call") { (id: Int, method: String, argsJson: String, promise: Promise) in
      self.lock.lock()
      let client = self.clients[id]
      self.lock.unlock()
      guard let client else {
        promise.resolve("{\"error\":{\"code\":\"INVALID_OPTIONS\",\"message\":\"openhongguo: client was released\"}}")
        return
      }
      client.start(method, argsJSON: argsJson, callback: Callback { promise.resolve($0) })
    }
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
