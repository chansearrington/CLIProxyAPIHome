// Home Usage: a macOS menu bar item that shows the quota and usage of the
// accounts CLIProxyAPIHome manages, read from Home's Management API.
//
// It only reads: GET quota snapshots and the usage overview every two minutes
// (database reads on Home, no provider calls). "Refresh now" is the only write
// (POST /quota/collect for the shown credentials) and runs only on click.
// The management key is read from the login Keychain via /usr/bin/security and
// never written anywhere else. Polling stops on the first 401/403 so a bad key
// can never trip Home's five-strikes ban.

import AppKit
import Foundation

// MARK: - Settings

enum Settings {
    // The app bundle ID is com.chansearrington.home-usage, so `defaults write` that domain.
    static let defaults = UserDefaults.standard
    // The Ark's Tailscale (MagicDNS) name: it only resolves on the tailnet, so the key is never
    // sent in clear text to some other network's 100.x address when Tailscale is off.
    static var homeURL: String {
        defaults.string(forKey: "homeURL") ?? "http://home-server-the-ark.taile4a41.ts.net:8327"
    }
    /// Survives relaunches so a rejected key is never retried automatically (Home bans an IP
    /// after five failures); only the menu's "Retry once" clears it.
    static var authPaused: Bool {
        get { defaults.bool(forKey: "authPaused") }
        set { defaults.set(newValue, forKey: "authPaused") }
    }
    static var pollSeconds: TimeInterval {
        let v = defaults.double(forKey: "pollSeconds")
        return v >= 30 ? v : 120
    }
    static let keychainService = "cpa-home-management"
    static let keychainAccount = "home-usage"
}

// MARK: - API types (subset of Home's JSON)

struct QuotaWindow: Decodable {
    let id: String
    let label: String?
    let unit: String
    let currency: String?
    let used: Double?
    let limit: Double?
    let usedRatio: Double?
    let isUnlimited: Bool
    let resetAt: String?
    let windowSeconds: Int64?

    enum CodingKeys: String, CodingKey {
        case id, label, unit, currency, used, limit
        case usedRatio = "used_ratio"
        case isUnlimited = "is_unlimited"
        case resetAt = "reset_at"
        case windowSeconds = "window_seconds"
    }
}

struct QuotaPlan: Decodable { let name: String }

struct QuotaCredential: Decodable {
    let credentialId: String
    let provider: String
    let label: String
    let credentialStatus: String
    let quotaStatus: String
    let freshness: String
    let observedAt: String?
    let primaryWindows: [QuotaWindow]?
    let windowCount: Int
    let plan: QuotaPlan?

    enum CodingKeys: String, CodingKey {
        case provider, label, freshness, plan
        case credentialId = "credential_id"
        case credentialStatus = "credential_status"
        case quotaStatus = "quota_status"
        case observedAt = "observed_at"
        case primaryWindows = "primary_windows"
        case windowCount = "window_count"
    }
}

struct QuotaList: Decodable { let items: [QuotaCredential] }
struct QuotaDetail: Decodable { let windows: [QuotaWindow] }

struct UsageEntry: Decodable {
    let id: String
    let requestCount: Int
    let totalTokens: Double
    enum CodingKeys: String, CodingKey {
        case id
        case requestCount = "request_count"
        case totalTokens = "total_tokens"
    }
}

struct UsageTotals: Decodable {
    let requestCount: Int
    let totalTokens: Double
    let cachedTokens: Double?
    enum CodingKeys: String, CodingKey {
        case requestCount = "request_count"
        case totalTokens = "total_tokens"
        case cachedTokens = "cached_tokens"
    }
}

struct UsageOverview: Decodable { let totals: UsageTotals }
struct UsageAggregates: Decodable { let items: [UsageEntry] }

/// One account as shown in the menu: the snapshot plus its full window list.
struct Account {
    let credential: QuotaCredential
    let windows: [QuotaWindow]
    let usage: UsageEntry?
}

enum FetchError: Error {
    case noKey
    case auth(Int)
    case http(Int)
    case transport(String)
}

// MARK: - Home client

@MainActor
final class HomeClient {
    private let session: URLSession
    private var cachedKey: String?

    init() {
        let config = URLSessionConfiguration.ephemeral
        config.timeoutIntervalForRequest = 15
        config.httpCookieStorage = nil
        config.urlCache = nil
        session = URLSession(configuration: config)
    }

    func forgetKey() { cachedKey = nil }

    private func key() async throws -> String {
        if let k = cachedKey { return k }
        // Run `security` off the main thread and give up after 10 s (a locked keychain can block).
        let k = await Task.detached { () -> String? in
            let p = Process()
            p.executableURL = URL(fileURLWithPath: "/usr/bin/security")
            p.arguments = ["find-generic-password", "-s", Settings.keychainService,
                           "-a", Settings.keychainAccount, "-w"]
            let out = Pipe()
            p.standardOutput = out
            p.standardError = FileHandle.nullDevice
            do { try p.run() } catch { return nil }
            let deadline = Date().addingTimeInterval(10)
            while p.isRunning && Date() < deadline { usleep(50_000) }
            if p.isRunning { p.terminate(); return nil }
            let data = out.fileHandleForReading.readDataToEndOfFile()
            let k = String(decoding: data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
            return p.terminationStatus == 0 && !k.isEmpty ? k : nil
        }.value
        guard let k else { throw FetchError.noKey }
        cachedKey = k
        return k
    }

    private func request(_ path: String, method: String = "GET", body: Data? = nil) async throws -> URLRequest {
        guard let url = URL(string: Settings.homeURL + "/v0/management" + path) else {
            throw FetchError.transport("bad Home URL")
        }
        var req = URLRequest(url: url)
        req.httpMethod = method
        req.setValue("Bearer " + (try await key()), forHTTPHeaderField: "Authorization")
        if let body {
            req.httpBody = body
            req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }
        return req
    }

    private func send(_ req: URLRequest) async throws -> Data {
        let data: Data
        let resp: URLResponse
        do { (data, resp) = try await session.data(for: req) } catch {
            throw FetchError.transport(error.localizedDescription)
        }
        let code = (resp as? HTTPURLResponse)?.statusCode ?? 0
        if code == 401 || code == 403 { throw FetchError.auth(code) }
        guard (200..<300).contains(code) else { throw FetchError.http(code) }
        return data
    }

    func get<T: Decodable>(_ path: String, as type: T.Type) async throws -> T {
        let data = try await send(try await request(path))
        do { return try JSONDecoder().decode(T.self, from: data) } catch {
            throw FetchError.transport("unexpected reply from Home")
        }
    }

    /// Like `get`, but a non-auth failure returns nil so one optional call cannot sink the poll.
    /// Auth failures always propagate: every rejected request counts toward Home's ban.
    private func optional<T: Decodable>(_ path: String, as type: T.Type) async throws -> T? {
        do { return try await get(path, as: type) } catch FetchError.auth(let code) {
            throw FetchError.auth(code)
        } catch { return nil }
    }

    func collect(credentialIDs: [String]) async throws {
        // Home treats an empty list as "every eligible credential", which is never what we want.
        guard !credentialIDs.isEmpty else { return }
        let body = try JSONSerialization.data(withJSONObject: ["credential_ids": credentialIDs])
        _ = try await send(try await request("/quota/collect", method: "POST", body: body))
    }

    /// Loads every account with all of its windows and last-24h usage.
    func loadAccounts() async throws -> ([Account], UsageTotals?) {
        let list = try await get("/quota/credentials?limit=200", as: QuotaList.self)
        // Both default to the last 24 hours on Home.
        let overview = try await optional("/usage/overview", as: UsageOverview.self)
        let perCredential = try await optional(
            "/usage/aggregates?group_by=credential&limit=200", as: UsageAggregates.self)
        var usageByID: [String: UsageEntry] = [:]
        for u in perCredential?.items ?? [] { usageByID[u.id] = u }

        var accounts: [Account] = []
        for c in list.items {
            // Credentials Home cannot measure (no windows at all) are noise here.
            if c.quotaStatus == "unsupported" && c.windowCount == 0 { continue }
            var windows = c.primaryWindows ?? []
            // The list carries at most two windows; fetch the rest when there are more.
            if c.windowCount > windows.count,
               let detail = try await optional("/quota/credentials/" + c.credentialId, as: QuotaDetail.self) {
                windows = detail.windows
            }
            accounts.append(Account(credential: c, windows: windows, usage: usageByID[c.credentialId]))
        }
        return (accounts, overview?.totals)
    }
}

// MARK: - Formatting

enum Fmt {
    static let iso: ISO8601DateFormatter = {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return f
    }()
    static let isoPlain = ISO8601DateFormatter()

    static func date(_ s: String?) -> Date? {
        guard let s else { return nil }
        // Go emits up to nanoseconds; Foundation accepts at most milliseconds.
        if let d = isoPlain.date(from: s) { return d }
        let trimmed = s.replacingOccurrences(of: #"(\.\d{3})\d+"#, with: "$1", options: .regularExpression)
        return iso.date(from: trimmed)
    }

    private static func formatter(_ format: String) -> DateFormatter {
        let f = DateFormatter()
        f.dateFormat = format
        return f
    }
    private static let timeOnly = formatter("h:mm a")
    private static let dayAndTime = formatter("EEE h:mm a")
    private static let dayOnly = formatter("MMM d")

    static func when(_ d: Date) -> String {
        if Calendar.current.isDateInToday(d) { return timeOnly.string(from: d) }
        if d.timeIntervalSinceNow < 6 * 86400 && d.timeIntervalSinceNow > 0 { return dayAndTime.string(from: d) }
        return dayOnly.string(from: d)
    }

    static func ago(_ d: Date) -> String {
        let s = max(0, -d.timeIntervalSinceNow)
        if s < 90 { return "just now" }
        if s < 5400 { return "\(Int(s / 60)) min ago" }
        if s < 2 * 86400 { return "\(Int(s / 3600)) h ago" }
        return "\(Int(s / 86400)) days ago"
    }

    static func count(_ v: Double) -> String {
        switch v {
        case 1e9...: return String(format: "%.2fB", v / 1e9)
        case 1e6...: return String(format: "%.1fM", v / 1e6)
        case 1e4...: return String(format: "%.0fK", v / 1e3)
        default:
            let f = NumberFormatter()
            f.numberStyle = .decimal
            return f.string(from: NSNumber(value: v)) ?? String(Int(v))
        }
    }

    static func provider(_ p: String) -> String {
        switch p {
        case "claude": return "Claude"
        case "codex": return "Codex"
        case "copilot": return "Copilot"
        case "xai": return "xAI"
        case "antigravity": return "Antigravity"
        case "kimi": return "Kimi"
        default: return p.capitalized
        }
    }

    static func windowName(_ w: QuotaWindow) -> String {
        let period: String?
        switch w.windowSeconds {
        case 18000: period = "5-hour"
        case 86400: period = "Daily"
        case 604800: period = "Weekly"
        default: period = nil
        }
        guard let l = w.label, !l.isEmpty else { return period ?? w.id }
        // Providers such as Antigravity reuse one label for several periods.
        return period.map { l + ", " + $0.lowercased() } ?? l
    }

    static func plural(_ n: Int, _ word: String) -> String {
        "\(count(Double(n))) \(word)\(n == 1 ? "" : "s")"
    }

    /// Used percentage for percent-style windows, nil when it is not a ratio.
    static func usedPercent(_ w: QuotaWindow) -> Double? {
        if w.isUnlimited { return nil }
        if let r = w.usedRatio, w.unit == "percentage" || w.unit == "requests" { return r * 100 }
        return nil
    }

    static func bar(_ pct: Double) -> String {
        let filled = Int((min(max(pct, 0), 100) / 10).rounded())
        return String(repeating: "█", count: filled) + String(repeating: "░", count: 10 - filled)
    }

    static func windowLine(_ w: QuotaWindow) -> String {
        var name = windowName(w)
        if name.count > 30 { name = String(name.prefix(29)) + "…" }
        name = name.padding(toLength: 31, withPad: " ", startingAt: 0)
        var value: String
        if w.isUnlimited {
            value = "unlimited"
        } else if w.unit == "currency" {
            let cur = w.currency.map { $0 == "USD" ? "$" : $0 + " " } ?? "$"
            value = String(format: "%@%.2f", cur, w.used ?? 0)
            if let limit = w.limit { value += String(format: " of %@%.2f", cur, limit) }
        } else if w.unit == "requests" || w.unit == "credits" {
            value = count(w.used ?? 0) + (w.limit.map { " of " + count($0) } ?? "") + " " + w.unit
        } else if let p = usedPercent(w) {
            value = bar(p) + String(format: " %3.0f%%", p)
        } else {
            value = "–"
        }
        if let r = date(w.resetAt) {
            // A reset time in the past means Home has not re-measured since; the number is old.
            value += r > Date() ? "  resets " + when(r) : "  (window reset since; old number)"
        }
        return "   " + name + value
    }
}

// MARK: - Menu bar controller

@MainActor
final class Controller: NSObject, NSApplicationDelegate {
    private let client = HomeClient()
    /// `--print` fetches once, prints the bar title and menu as text, and exits.
    private let printMode = CommandLine.arguments.contains("--print")
    private var item: NSStatusItem?
    private var barTitle = ""
    private var timer: Timer?
    private var accounts: [Account] = []
    private var totals: UsageTotals?
    private var lastSuccess: Date?
    private var problem: String?
    private var paused = Settings.authPaused
    private var polling = false

    func applicationDidFinishLaunching(_ notification: Notification) {
        if printMode {
            // A manual one-shot run counts as "Retry once".
            paused = false
            Task {
                await poll()
                printMenu()
                exit(problem == nil ? 0 : 1)
            }
            return
        }
        let statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        statusItem.autosaveName = "home-usage"
        item = statusItem
        // `--show-menu` (another process) asks the running app to open its menu for a few
        // seconds, so a screenshot can be taken without moving the mouse.
        DistributedNotificationCenter.default().addObserver(
            self, selector: #selector(showMenuBriefly),
            name: Self.showMenuNotification, object: nil)
        setTitle("CPA …", color: nil)
        if paused {
            problem = "Paused earlier because Home rejected the key. Fix the Keychain item, then Retry."
            render()
            return
        }
        rebuildMenu()
        startTimer()
        Task { await poll() }
    }

    private func startTimer() {
        timer?.invalidate()
        timer = Timer.scheduledTimer(withTimeInterval: Settings.pollSeconds, repeats: true) { [weak self] _ in
            Task { @MainActor in await self?.poll() }
        }
    }

    private func pause(_ message: String) {
        paused = true
        Settings.authPaused = true
        timer?.invalidate()
        client.forgetKey()
        problem = message
    }

    private func poll() async {
        // One poll at a time: overlapping polls could each send a doomed request.
        if paused || polling { return }
        polling = true
        defer { polling = false }
        do {
            let (a, t) = try await client.loadAccounts()
            accounts = a
            totals = t
            lastSuccess = Date()
            problem = nil
        } catch FetchError.auth(let code) {
            // Never retry a rejected key on a timer: Home bans an IP after five failures.
            pause("Home rejected the key (HTTP \(code)). Paused — fix the Keychain item, then Retry.")
        } catch FetchError.noKey {
            pause("No Home key in the Keychain (\(Settings.keychainService)). Paused.")
        } catch FetchError.http(let code) {
            problem = "Home answered HTTP \(code); showing last known numbers."
        } catch FetchError.transport(let msg) {
            problem = "Can't reach Home (\(msg)); showing last known numbers."
        } catch {
            problem = "Unexpected error: \(error.localizedDescription)"
        }
        render()
    }

    // MARK: Rendering

    /// The bar shows the fullest Claude/Codex window that still applies right now.
    private func headline() -> (Double, Account, QuotaWindow)? {
        var best: (Double, Account, QuotaWindow)?
        // Disabled or never-measured accounts would pin the bar to a number nobody can act on.
        for a in accounts where ["claude", "codex"].contains(a.credential.provider)
            && a.credential.credentialStatus == "enabled" && a.credential.freshness != "never" {
            for w in a.windows {
                guard let p = Fmt.usedPercent(w) else { continue }
                if let r = Fmt.date(w.resetAt), r < Date() { continue }
                if best == nil || p > best!.0 { best = (p, a, w) }
            }
        }
        return best
    }

    private func setTitle(_ text: String, color: NSColor?) {
        barTitle = text
        guard let button = item?.button else { return }
        var attrs: [NSAttributedString.Key: Any] = [
            .font: NSFont.monospacedDigitSystemFont(ofSize: NSFont.systemFontSize, weight: .medium),
        ]
        if let color { attrs[.foregroundColor] = color }
        button.attributedTitle = NSAttributedString(string: text, attributes: attrs)
    }

    private func render() {
        if paused {
            setTitle("CPA ⚠︎", color: .systemRed)
        } else if let (p, _, _) = headline() {
            let color: NSColor? = p >= 95 ? .systemRed : (p >= 80 ? .systemOrange : nil)
            setTitle(String(format: "CPA %.0f%%", p) + (problem == nil ? "" : " ?"), color: color)
        } else {
            setTitle(problem == nil ? "CPA –" : "CPA ?", color: nil)
        }
        rebuildMenu()
    }

    private func line(_ text: String, mono: Bool = false, bold: Bool = false, dim: Bool = false) -> NSMenuItem {
        let mi = NSMenuItem(title: text, action: nil, keyEquivalent: "")
        let size = NSFont.menuFont(ofSize: 0).pointSize
        let font: NSFont = mono
            ? NSFont.monospacedSystemFont(ofSize: size - 1, weight: .regular)
            : (bold ? NSFont.boldSystemFont(ofSize: size) : NSFont.menuFont(ofSize: 0))
        var attrs: [NSAttributedString.Key: Any] = [.font: font]
        attrs[.foregroundColor] = dim ? NSColor.secondaryLabelColor : NSColor.labelColor
        mi.attributedTitle = NSAttributedString(string: text, attributes: attrs)
        mi.isEnabled = true
        return mi
    }

    private func rebuildMenu() {
        let menu = NSMenu()
        menu.autoenablesItems = false
        var header = "Home (CPA) accounts"
        if let t = lastSuccess { header += " — updated " + Fmt.when(t) }
        menu.addItem(line(header, bold: true))
        if let h = headline() {
            let a = h.1.credential
            menu.addItem(line("Bar shows the fullest limit: \(Fmt.provider(a.provider)) \(a.label), "
                + "\(Fmt.windowName(h.2).lowercased()) window", dim: true))
        }
        if let problem { menu.addItem(line("⚠︎ " + problem)) }
        menu.addItem(.separator())

        let order = ["claude", "codex", "copilot", "xai", "antigravity"]
        let sorted = accounts.sorted {
            let i = order.firstIndex(of: $0.credential.provider) ?? order.count
            let j = order.firstIndex(of: $1.credential.provider) ?? order.count
            return i != j ? i < j : $0.credential.label < $1.credential.label
        }
        for a in sorted {
            let c = a.credential
            var title = "\(Fmt.provider(c.provider)) · \(c.label)"
            if let plan = c.plan { title += " (\(plan.name))" }
            menu.addItem(line(title, bold: true))
            for w in a.windows { menu.addItem(line(Fmt.windowLine(w), mono: true)) }
            var notes: [String] = []
            if let u = a.usage {
                notes.append("last 24 h: \(Fmt.plural(u.requestCount, "request")), \(Fmt.count(u.totalTokens)) tokens")
            }
            if c.freshness != "fresh" {
                let seen = Fmt.date(c.observedAt).map { "numbers from " + Fmt.ago($0) } ?? "never measured"
                notes.append("not recently used — \(seen)")
            }
            if c.credentialStatus != "enabled" { notes.append("Home marks it \(c.credentialStatus)") }
            if c.quotaStatus == "exhausted" || c.quotaStatus == "low" { notes.append("quota \(c.quotaStatus)") }
            if !notes.isEmpty { menu.addItem(line("   " + notes.joined(separator: " · "), dim: true)) }
        }
        if accounts.isEmpty && problem == nil { menu.addItem(line("Loading…", dim: true)) }

        if let t = totals {
            menu.addItem(.separator())
            var s = "Whole fleet, last 24 h: \(Fmt.count(Double(t.requestCount))) requests, \(Fmt.count(t.totalTokens)) tokens"
            if let cached = t.cachedTokens, cached > 0 { s += " (\(Fmt.count(cached)) from cache)" }
            menu.addItem(line(s, dim: true))
        }

        menu.addItem(.separator())
        if paused {
            menu.addItem(action("Retry once", #selector(retry)))
        } else if !accounts.isEmpty {
            menu.addItem(action("Refresh now (asks the providers)", #selector(refreshNow), key: "r"))
        }
        menu.addItem(action("Open Home panel", #selector(openPanel)))
        menu.addItem(action("Quit Home Usage", #selector(quit), key: "q"))
        item?.menu = menu
        lastMenu = menu
    }

    private var lastMenu: NSMenu?

    private func printMenu() {
        print("[bar] " + barTitle)
        for mi in lastMenu?.items ?? [] { print(mi.isSeparatorItem ? "-----" : mi.title) }
    }

    private func action(_ title: String, _ sel: Selector, key: String = "") -> NSMenuItem {
        let mi = NSMenuItem(title: title, action: sel, keyEquivalent: key)
        mi.target = self
        return mi
    }

    @objc private func refreshNow() {
        let ids = accounts.map { $0.credential.credentialId }
        Task { @MainActor in
            do { try await client.collect(credentialIDs: ids) } catch FetchError.auth(let code) {
                pause("Home rejected the key (HTTP \(code)). Paused — fix the Keychain item, then Retry.")
                render()
                return
            } catch {}
            // Probes run in the background on Home; give them a moment before re-reading.
            try? await Task.sleep(nanoseconds: 15_000_000_000)
            await poll()
        }
    }

    @objc private func retry() {
        paused = false
        Settings.authPaused = false
        problem = nil
        startTimer()
        Task { await poll() }
    }

    @objc private func openPanel() {
        if let url = URL(string: Settings.homeURL + "/management.html") { NSWorkspace.shared.open(url) }
    }

    @objc private func quit() { NSApp.terminate(nil) }

    nonisolated static let showMenuNotification = Notification.Name("com.chansearrington.home-usage.show-menu")

    @objc private func showMenuBriefly() {
        guard let menu = item?.menu, let button = item?.button else { return }
        menu.perform(#selector(NSMenu.cancelTracking), with: nil, afterDelay: 4,
                     inModes: [.eventTracking, .default])
        button.performClick(nil)
    }
}

// MARK: - Entry point

if CommandLine.arguments.contains("--show-menu") {
    DistributedNotificationCenter.default().postNotificationName(
        Controller.showMenuNotification, object: nil, userInfo: nil, deliverImmediately: true)
    exit(0)
}

MainActor.assumeIsolated {
    let app = NSApplication.shared
    let controller = Controller()
    app.delegate = controller
    app.setActivationPolicy(.accessory)
    app.run()
}
