package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"entangled-client/vpncore"
)

type ClientConfig struct {
	ServerAddr       string `json:"serverAddr"`
	Nickname         string `json:"nickname"`
	AutoConnect      bool   `json:"autoConnect"`
	AutoJoinLastRoom bool   `json:"autoJoinLastRoom"`
	LastRoomName     string `json:"lastRoomName"`
	LastRoomLocked   bool   `json:"lastRoomLocked"`
	StartWithWindows bool   `json:"startWithWindows"`
	ConnectionMode   string `json:"connectionMode"`
	P2POnly          bool   `json:"p2pOnly,omitempty"`
	MTU              int    `json:"mtu"`
	DNSServer        string `json:"dnsServer"`
	SOCKS5Proxy      string `json:"socks5Proxy"`
	STUNServer       string `json:"stunServer"`
	FontSize         int    `json:"fontSize,omitempty"`
	UiScale          int    `json:"uiScale"`
	Theme            string `json:"theme"`
	Lang             string `json:"lang"`
	ServerToken      string `json:"serverToken"`
}

type App struct {
	vpn          *vpncore.VPNCore
	mu           sync.Mutex
	opMu         sync.Mutex
	lastRoomName string
	lastRoomPass string
	emit         func(event string, data interface{})
	quitFn       func()
	clipboardFn  func(text string) error
}

func NewApp(emit func(string, interface{}), quitFn func(), clipboardFn func(string) error) *App {
	return &App{emit: emit, quitFn: quitFn, clipboardFn: clipboardFn}
}

func defaultConfig() ClientConfig {
	return ClientConfig{
		AutoConnect:      false,
		AutoJoinLastRoom: false,
		StartWithWindows: false,
		ConnectionMode:   "direct",
		MTU:              1500,
		DNSServer:        "",
		SOCKS5Proxy:      "",
		STUNServer:       "stun.l.google.com:19302",
		UiScale:          100,
		Theme:            "dark",
		Lang:             "en",
		ServerToken:      "",
	}
}

func configDir() string {
	if appData := os.Getenv("APPDATA"); appData != "" {
		return filepath.Join(appData, "EntangledVPN")
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "entangledvpn")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".config", "entangledvpn")
	}
	return filepath.Join(os.TempDir(), "entangledvpn")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

type SavedRoomEntry struct {
	Name       string `json:"name"`
	Password   string `json:"password,omitempty"`
	Server     string `json:"server"`
	Locked     bool   `json:"locked,omitempty"`
	OwnerToken string `json:"owner_token,omitempty"`
}

func roomsPath() string {
	return filepath.Join(configDir(), "rooms.json")
}

func (a *App) GetSavedRooms() []SavedRoomEntry {
	rooms := a.loadRoomsRaw()
	if rooms == nil {
		return []SavedRoomEntry{}
	}
	out := make([]SavedRoomEntry, len(rooms))
	for i, r := range rooms {
		out[i] = SavedRoomEntry{Name: r.Name, Server: r.Server, Locked: r.Locked}
	}
	return out
}

func (a *App) SaveRoom(name, password string) {
	cfg := a.LoadConfig()
	rooms := a.loadRoomsRaw()
	prevToken := ""
	for _, r := range rooms {
		if r.Name == name {
			prevToken = r.OwnerToken
			break
		}
	}
	entry := SavedRoomEntry{Name: name, Server: cfg.ServerAddr, Locked: password != "", OwnerToken: prevToken}
	found := false
	for i, r := range rooms {
		if r.Name == name {
			rooms[i] = entry
			found = true
			break
		}
	}
	if !found {
		rooms = append(rooms, entry)
	}
	a.writeRooms(rooms)
}

func (a *App) saveOwnerToken(room, token string) {
	if room == "" || token == "" {
		return
	}
	rooms := a.loadRoomsRaw()
	found := false
	for i, r := range rooms {
		if r.Name == room {
			rooms[i].OwnerToken = token
			found = true
			break
		}
	}
	if !found {
		cfg := a.LoadConfig()
		rooms = append(rooms, SavedRoomEntry{Name: room, Server: cfg.ServerAddr, OwnerToken: token})
	}
	a.writeRooms(rooms)
	a.mu.Lock()
	if a.vpn != nil {
		a.vpn.SetOwnerToken(room, token)
	}
	a.mu.Unlock()
}

func (a *App) loadRoomsRaw() []SavedRoomEntry {
	data, err := os.ReadFile(roomsPath())
	if err != nil {
		return []SavedRoomEntry{}
	}
	var rooms []SavedRoomEntry
	json.Unmarshal(data, &rooms)
	if rooms == nil {
		return []SavedRoomEntry{}
	}
	for i := range rooms {
		if rooms[i].Password != "" {
			rooms[i].Locked = true
		}
	}
	return rooms
}

func (a *App) writeRooms(rooms []SavedRoomEntry) {
	clean := make([]SavedRoomEntry, len(rooms))
	for i, r := range rooms {
		clean[i] = SavedRoomEntry{Name: r.Name, Server: r.Server, Locked: r.Locked, OwnerToken: r.OwnerToken}
	}
	if err := os.MkdirAll(configDir(), 0755); err != nil {
		vpncore.Logger.Printf("SaveRoom: mkdir error: %v", err)
		return
	}
	data, err := json.MarshalIndent(clean, "", "  ")
	if err != nil {
		vpncore.Logger.Printf("SaveRoom: marshal error: %v", err)
		return
	}
	if err := os.WriteFile(roomsPath(), data, 0600); err != nil {
		vpncore.Logger.Printf("SaveRoom: write error: %v", err)
	}
}

func (a *App) RemoveSavedRoom(name string) {
	rooms := a.loadRoomsRaw()
	filtered := make([]SavedRoomEntry, 0, len(rooms))
	for _, r := range rooms {
		if r.Name != name {
			filtered = append(filtered, SavedRoomEntry{Name: r.Name, Server: r.Server, Locked: r.Locked, OwnerToken: r.OwnerToken})
		}
	}
	a.writeRooms(filtered)
}

func (a *App) persistLastRoom(name string, locked bool) {
	cfg := a.LoadConfig()
	if cfg.LastRoomName == name && cfg.LastRoomLocked == locked {
		return
	}
	cfg.LastRoomName = name
	cfg.LastRoomLocked = locked
	a.SaveConfig(cfg)
}

type PeerInfo struct {
	ID        string `json:"id"`
	Nickname  string `json:"nickname"`
	VirtualIP string `json:"virtualIP"`
	Connected bool   `json:"connected"`
	Ping      int    `json:"ping"`
	Path      string `json:"path"`
}

type AppStatus struct {
	Connected    bool   `json:"connected"`
	Reconnecting bool   `json:"reconnecting"`
	Server       string `json:"server"`
	Room         string `json:"room"`
	VirtualIP    string `json:"virtualIP"`
	PeerCount    int    `json:"peerCount"`
	IsOwner      bool   `json:"isOwner"`
	Phase        string `json:"phase"`
}

func (a *App) Shutdown() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.vpn != nil {
		a.vpn.Stop()
		a.vpn = nil
	}
	vpncore.Logger.Println("App shutdown")
}

func (a *App) GetVersion() string {
	return vpncore.AppVersion
}

func (a *App) GetStatus() AppStatus {
	if a.vpn == nil {
		return AppStatus{}
	}
	return statusFrom(a.vpn.GetStatus())
}

func statusFrom(s vpncore.ConnectionStatus) AppStatus {
	return AppStatus{
		Connected:    s.Connected,
		Reconnecting: s.Reconnecting,
		Server:       s.Server,
		Room:         s.Room,
		VirtualIP:    s.VirtualIP,
		PeerCount:    s.PeerCount,
		IsOwner:      s.IsOwner,
		Phase:        s.Phase,
	}
}

func normalizeConnectionMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "relay":
		return "relay"
	default:
		return "direct"
	}
}

func (c ClientConfig) ForceRelay() bool {
	return normalizeConnectionMode(c.ConnectionMode) == "relay"
}

func (a *App) LoadConfig() ClientConfig {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return defaultConfig()
	}
	cfg := defaultConfig()
	json.Unmarshal(data, &cfg)
	if cfg.UiScale == 0 {
		cfg.UiScale = 100
	}
	if cfg.UiScale < 75 {
		cfg.UiScale = 75
	}
	if cfg.UiScale > 150 {
		cfg.UiScale = 150
	}
	if cfg.ConnectionMode == "" {
		cfg.ConnectionMode = "direct"
	}
	cfg.ConnectionMode = normalizeConnectionMode(cfg.ConnectionMode)
	cfg.P2POnly = false
	return cfg
}

func (a *App) SaveConfig(cfg ClientConfig) {
	if err := os.MkdirAll(configDir(), 0755); err != nil {
		vpncore.Logger.Printf("SaveConfig: mkdir error: %v", err)
		return
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		vpncore.Logger.Printf("SaveConfig: marshal error: %v", err)
		return
	}
	if err := os.WriteFile(configPath(), data, 0600); err != nil {
		vpncore.Logger.Printf("SaveConfig: write error: %v", err)
	}
}

func (a *App) GetSettings() ClientConfig {
	return a.LoadConfig()
}

func (a *App) SaveSettings(cfg ClientConfig) bool {
	old := a.LoadConfig()
	if cfg.LastRoomName == "" {
		cfg.LastRoomName = old.LastRoomName
		cfg.LastRoomLocked = old.LastRoomLocked
	}
	if cfg.UiScale == 0 {
		cfg.UiScale = 100
	}
	if cfg.UiScale < 75 {
		cfg.UiScale = 75
	}
	if cfg.UiScale > 150 {
		cfg.UiScale = 150
	}
	cfg.FontSize = 0
	cfg.ConnectionMode = normalizeConnectionMode(cfg.ConnectionMode)
	cfg.P2POnly = false
	a.SaveConfig(cfg)
	if old.StartWithWindows != cfg.StartWithWindows {
		a.SetStartWithWindows(cfg.StartWithWindows)
	}

	a.mu.Lock()
	vpn := a.vpn
	a.mu.Unlock()
	needsReconnect := false
	if vpn != nil {
		vpn.SetForceRelay(cfg.ForceRelay())
		vpn.ApplyTUNSettings(cfg.MTU, cfg.DNSServer)
		if old.SOCKS5Proxy != cfg.SOCKS5Proxy || old.STUNServer != cfg.STUNServer || old.ServerToken != cfg.ServerToken {
			needsReconnect = true
		}
	}
	return needsReconnect
}

func (a *App) SetStartWithWindows(enabled bool) {
	if runtime.GOOS == "windows" {
		if enabled {
			exePath, err := os.Executable()
			if err != nil {
				vpncore.Logger.Printf("SetStartWithWindows: get exe path error: %v", err)
				return
			}
			cmd := vpncore.HiddenCommand("reg", "add",
				"HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run",
				"/v", "EntangledVPN",
				"/t", "REG_SZ",
				"/d", exePath,
				"/f",
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				vpncore.Logger.Printf("SetStartWithWindows: reg add error: %v, output: %s", err, string(out))
			}
		} else {
			cmd := vpncore.HiddenCommand("reg", "delete",
				"HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run",
				"/v", "EntangledVPN",
				"/f",
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				vpncore.Logger.Printf("SetStartWithWindows: reg delete error: %v, output: %s", err, string(out))
			}
		}
		return
	}
	// Linux: XDG autostart for the Electron wrapper if ENTANGLED_ELECTRON_EXEC is set.
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".config", "autostart")
	desktop := filepath.Join(dir, "entangledvpn.desktop")
	if !enabled {
		_ = os.Remove(desktop)
		return
	}
	execPath := os.Getenv("ENTANGLED_ELECTRON_EXEC")
	if execPath == "" {
		vpncore.Logger.Printf("SetStartWithWindows: ENTANGLED_ELECTRON_EXEC unset; skip autostart")
		return
	}
	_ = os.MkdirAll(dir, 0755)
	body := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Entangled VPN\nExec=%s\nX-GNOME-Autostart-enabled=true\n", execPath)
	_ = os.WriteFile(desktop, []byte(body), 0644)
}

func (a *App) ResetSettings() ClientConfig {
	old := a.LoadConfig()
	cfg := defaultConfig()
	a.SaveConfig(cfg)
	if old.StartWithWindows {
		a.SetStartWithWindows(false)
	}
	a.mu.Lock()
	vpn := a.vpn
	a.mu.Unlock()
	if vpn != nil {
		vpn.SetForceRelay(cfg.ForceRelay())
		vpn.ApplyTUNSettings(cfg.MTU, cfg.DNSServer)
	}
	return cfg
}

func (a *App) Connect(serverAddr, nickname string) (AppStatus, error) {
	a.opMu.Lock()
	defer a.opMu.Unlock()

	cfg := a.LoadConfig()
	cfg.ServerAddr = serverAddr
	cfg.Nickname = nickname
	a.SaveConfig(cfg)
	vpncore.Logger.Printf("Connect called: server=%s nickname=%s", serverAddr, nickname)

	a.mu.Lock()
	oldVPN := a.vpn

	vpnCfg := &vpncore.VPNConfig{
		ServerAddr: serverAddr,
		Nickname:   nickname,
		ForceRelay: cfg.ForceRelay(),
		STUNServer: cfg.STUNServer,
		MTU:        cfg.MTU,
		DNSServer:  cfg.DNSServer,
		SOCKS5Addr: cfg.SOCKS5Proxy,
		AuthToken:  cfg.ServerToken,
	}
	newVPN := vpncore.NewVPNCore(vpnCfg)
	newVPN.SetForceRelay(cfg.ForceRelay())
	for _, r := range a.loadRoomsRaw() {
		if r.OwnerToken != "" {
			newVPN.SetOwnerToken(r.Name, r.OwnerToken)
		}
	}
	a.wireVPN(newVPN)
	a.vpn = newVPN
	a.mu.Unlock()
	if oldVPN != nil {
		oldVPN.Stop()
	}

	err := newVPN.Start()
	if err != nil {
		vpncore.Logger.Printf("Connect failed: %v", err)
		a.mu.Lock()
		if a.vpn == newVPN {
			a.vpn = nil
		}
		a.mu.Unlock()
		newVPN.Stop()
		return AppStatus{}, err
	}

	a.autoJoinLastRoom(cfg)
	return a.GetStatus(), nil
}

func (a *App) autoJoinLastRoom(cfg ClientConfig) {
	if !cfg.AutoJoinLastRoom {
		return
	}
	room := cfg.LastRoomName
	if room == "" {
		room = a.lastRoomName
	}
	if room == "" {
		return
	}
	pass := a.lastRoomPass
	locked := cfg.LastRoomLocked
	if locked && pass == "" {
		vpncore.Logger.Printf("AutoJoinLastRoom: skipped %s (password required)", room)
		a.emitEvent("auto_join_skipped", map[string]string{"room": room})
		return
	}
	vpncore.Logger.Printf("AutoJoinLastRoom: room=%s (sessionPass=%v)", room, pass != "")
	a.mu.Lock()
	vpn := a.vpn
	a.mu.Unlock()
	if vpn != nil {
		if err := vpn.JoinRoom(room, pass); err != nil {
			vpncore.Logger.Printf("AutoJoinLastRoom failed: %v", err)
			a.emitEvent("error", err.Error())
		}
	}
}

func (a *App) wireVPN(vpn *vpncore.VPNCore) {
	vpn.OnLog = func(msg string) {
		vpncore.Logger.Println(msg)
	}
	vpn.OnStatusChange = func(s vpncore.ConnectionStatus) {
		a.emitEvent("status_changed", statusFrom(s))
	}
	vpn.OnError = func(msg string) {
		a.emitEvent("error", msg)
	}
	vpn.OnPeersChange = func(peers []*vpncore.Peer) {
		info := make([]PeerInfo, len(peers))
		for i, p := range peers {
			id, nick, vip, conn, ping, path := p.Snapshot()
			info[i] = PeerInfo{
				ID:        id,
				Nickname:  nick,
				VirtualIP: vip,
				Connected: conn,
				Ping:      ping,
				Path:      path,
			}
		}
		a.emitEvent("peers_changed", info)
	}
	vpn.OnChat = func(fromID, nickname, message string, isDM bool) {
		a.emitEvent("chat_message", map[string]interface{}{
			"fromID":   fromID,
			"nickname": nickname,
			"message":  message,
			"isDM":     isDM,
		})
	}
	vpn.OnSystemChat = func(text string) {
		a.emitEvent("system_chat", map[string]string{"message": text})
	}
	vpn.OnRoomDeleted = func(name string) {
		a.emitEvent("room_deleted", map[string]string{"name": name})
	}
	vpn.OnOwnerToken = func(room, token string) {
		a.saveOwnerToken(room, token)
	}
}

func (a *App) emitEvent(event string, data interface{}) {
	if a.emit != nil {
		a.emit(event, data)
	}
}

func (a *App) Disconnect() {
	a.opMu.Lock()
	defer a.opMu.Unlock()

	a.mu.Lock()
	vpn := a.vpn
	a.vpn = nil
	a.mu.Unlock()
	if vpn != nil {
		vpn.Stop()
	}
}

func (a *App) CreateRoom(name, password string) error {
	a.opMu.Lock()
	defer a.opMu.Unlock()

	a.mu.Lock()
	vpn := a.vpn
	a.mu.Unlock()
	if vpn == nil {
		return fmt.Errorf("not connected")
	}
	if err := vpn.CreateRoom(name, password); err != nil {
		return err
	}
	a.lastRoomName = name
	a.lastRoomPass = password
	a.SaveRoom(name, password)
	a.persistLastRoom(name, password != "")
	return nil
}

func (a *App) JoinRoom(name, password string) error {
	a.opMu.Lock()
	defer a.opMu.Unlock()

	a.mu.Lock()
	vpn := a.vpn
	a.mu.Unlock()
	if vpn == nil {
		return fmt.Errorf("not connected")
	}
	if err := vpn.JoinRoom(name, password); err != nil {
		return err
	}
	a.lastRoomName = name
	a.lastRoomPass = password
	a.SaveRoom(name, password)
	a.persistLastRoom(name, password != "")
	return nil
}

func (a *App) LeaveRoom() {
	a.opMu.Lock()
	defer a.opMu.Unlock()

	a.mu.Lock()
	vpn := a.vpn
	a.mu.Unlock()
	if vpn != nil {
		vpn.LeaveRoom()
	}
}

func (a *App) DeleteRoom(name string) error {
	a.opMu.Lock()
	defer a.opMu.Unlock()

	a.mu.Lock()
	vpn := a.vpn
	a.mu.Unlock()
	if vpn != nil {
		return vpn.DeleteRoom(name)
	}
	return fmt.Errorf("not connected")
}

func (a *App) GetPeers() []PeerInfo {
	if a.vpn == nil {
		return nil
	}
	peers := a.vpn.GetPeers()
	info := make([]PeerInfo, len(peers))
	for i, p := range peers {
		id, nick, vip, conn, ping, path := p.Snapshot()
		info[i] = PeerInfo{
			ID:        id,
			Nickname:  nick,
			VirtualIP: vip,
			Connected: conn,
			Ping:      ping,
			Path:      path,
		}
	}
	return info
}

func (a *App) SendChat(toID, message string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.vpn == nil {
		return fmt.Errorf("not connected")
	}
	return a.vpn.SendChat(toID, message)
}

func (a *App) BroadcastChat(message string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.vpn == nil {
		return fmt.Errorf("not connected")
	}
	return a.vpn.BroadcastChat(message)
}

func (a *App) PingPeer(peerID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.vpn == nil {
		return fmt.Errorf("not connected")
	}
	return a.vpn.PingPeer(peerID)
}

func (a *App) CopyText(text string) {
	if a.clipboardFn != nil {
		if err := a.clipboardFn(text); err != nil {
			vpncore.Logger.Printf("CopyText: %v", err)
		}
	}
}

func (a *App) FormatInvite(room, password string) string {
	cfg := a.LoadConfig()
	if password == "" && room == a.lastRoomName {
		password = a.lastRoomPass
	}
	return cfg.ServerAddr + "|" + room + "|" + password
}

func (a *App) ParseInvite(invite string) (map[string]string, error) {
	parts := strings.SplitN(strings.TrimSpace(invite), "|", 3)
	out := map[string]string{"server": "", "room": "", "password": ""}
	if len(parts) >= 1 {
		out["server"] = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		out["room"] = strings.TrimSpace(parts[1])
	}
	if len(parts) >= 3 {
		out["password"] = parts[2]
	}
	if out["server"] == "" || out["room"] == "" {
		return out, fmt.Errorf("invalid invite: server and room are required")
	}
	return out, nil
}

func (a *App) Quit() {
	if a.quitFn != nil {
		a.quitFn()
	}
}

func (a *App) OpenLogFolder() error {
	if err := vpncore.OpenLogFolder(); err != nil {
		vpncore.Logger.Printf("OpenLogFolder: %v", err)
		return err
	}
	return nil
}
