import Cocoa
import FlutterMacOS

class MainFlutterWindow: NSWindow {
  override func awakeFromNib() {
    let flutterViewController = FlutterViewController()
    let windowFrame = self.frame
    self.contentViewController = flutterViewController
    self.setFrame(windowFrame, display: true)

    // 窗口标题使用中文显示名（真果鉴 / 红果鉴），而非 ASCII 的 PRODUCT_NAME。
    if let name = Bundle.main.object(forInfoDictionaryKey: "CFBundleDisplayName") as? String,
       !name.isEmpty {
      self.title = name
    }

    RegisterGeneratedPlugins(registry: flutterViewController)

    super.awakeFromNib()
  }
}
