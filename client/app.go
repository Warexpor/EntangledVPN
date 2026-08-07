package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"entangled-client/vpncore"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ClientConfig struct {
	ServerAddr       string `json:"serverAddr"`
	Nickname         string `json:"nickname"`
	AutoConnect      bool   `json:"autoConnect"`
	AutoJoinLastRoom bool   `json:"autoJoinLastRoom"`
	LastRoomName     string `json:"lastRoomName"`
	LastRoomLocked   bool   `json:"lastRoomLocked"` // room needs password; password itself is never on disk
	StartWithWindows bool   `json:"startWithWindows"`
	P2POnly          bool   `json:"p2pOnly"`
	MTU              int    `json:"mtu"`
	DNSServer        string `json:"dnsServer"`
	SOCKS5Proxy      string `json:"socks5Proxy"`
	STUNServer       string `json:"stunServer"`
	FontSize         int    `json:"fontSize,omitempty"` // legacy (ignored); prefer UiScale
	UiScale          int    `json:"uiScale"`             // percent, 75–150
	Theme            string `json:"theme"`
	Lang             string `json:"lang"`
	ServerToken      string `json:"serverToken"`
}

type App struct {
	vpn          *vpncore.VPNCore
	mu           sync.Mutex
	ctx          context.Context
	lastRoomName string
	lastRoomPass string // session-only; never written to rooms.json
}

func NewApp() *App {
	return &App{}
}

func defaultConfig() ClientConfig {
	return ClientConfig{
		AutoConnect:      false,
		AutoJoinLastRoom: false,
		StartWithWindows: false,
		P2POnly:          false,
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
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.TempDir()
	}
	return filepath.Join(appData, "EntangledVPN")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

// SavedRoomEntry is persisted to rooms.json. Password is never written
// (omitempty strips any legacy field on rewrite). Locked is a non-secret flag.
type SavedRoomEntry struct {
	Name     string `json:"name"`
	Password string `json:"password,omitempty"`
	Server   string `json:"server"`
	Locked   bool   `json:"locked,omitempty"`
}

func roomsPath() string {
	return filepath.Join(configDir(), "rooms.json")
}

func (a *App) GetSavedRooms() []SavedRoomEntry {
	rooms := a.loadRoomsRaw()
	if rooms == nil {
		return []SavedRoomEntry{}
	}
	// Never expose disk passwords to UI (legacy rooms.json may still have them).
	out := make([]SavedRoomEntry, len(rooms))
	for i, r := range rooms {
		out[i] = SavedRoomEntry{Name: r.Name, Server: r.Server, Locked: r.Locked}
	}
	return out
}

// SaveRoom persists name+server+locked. Password stays in-memory via JoinRoom/CreateRoom.
func (a *App) SaveRoom(name, password string) {
	cfg := a.LoadConfig()
	entry := SavedRoomEntry{Name: name, Server: cfg.ServerAddr, Locked: password != ""}
	rooms := a.loadRoomsRaw()
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
	// Legacy rooms.json may still have passwords — treat as locked, never keep plaintext.
	for i := range rooms {
		if rooms[i].Password != "" {
			rooms[i].Locked = true
		}
	}
	return rooms
}

func (a *App) writeRooms(rooms []SavedRoomEntry) {
	// Strip passwords before write (migrate legacy files). Keep Locked.
	clean := make([]SavedRoomEntry, len(rooms))
	for i, r := range rooms {
		clean[i] = SavedRoomEntry{Name: r.Name, Server: r.Server, Locked: r.Locked}
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
			filtered = append(filtered, SavedRoomEntry{Name: r.Name, Server: r.Server, Locked: r.Locked})
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

func (a *App) startup(ctx context.Context) {
	frontendCtx = ctx
	a.ctx = ctx
	vpncore.Logger.Printf("Entangled VPN %s starting up", vpncore.AppVersion)
	runtime.WindowSetTitle(ctx, "Entangled VPN "+vpncore.AppVersion)
}

func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.vpn != nil {
		a.vpn.Stop()
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
	s := a.vpn.GetStatus()
	return statusFrom(s)
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
	// Preserve lastRoomName/locked unless settings UI overwrites intentionally.
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
	cfg.FontSize = 0 // stop rewriting legacy field
	a.SaveConfig(cfg)
	if old.StartWithWindows != cfg.StartWithWindows {
		a.SetStartWithWindows(cfg.StartWithWindows)
	}

	a.mu.Lock()
	vpn := a.vpn
	a.mu.Unlock()
	needsReconnect := false
	if vpn != nil {
		vpn.SetP2POnly(cfg.P2POnly)
		vpn.ApplyTUNSettings(cfg.MTU, cfg.DNSServer)
		if old.SOCKS5Proxy != cfg.SOCKS5Proxy || old.STUNServer != cfg.STUNServer || old.ServerToken != cfg.ServerToken {
			needsReconnect = true
		}
	}
	return needsReconnect
}

func (a *App) SetStartWithWindows(enabled bool) {
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
		vpn.SetP2POnly(cfg.P2POnly)
		vpn.ApplyTUNSettings(cfg.MTU, cfg.DNSServer)
	}
	return cfg
}

func (a *App) Connect(serverAddr, nickname string) (AppStatus, error) {
	a.mu.Lock()

	cfg := a.LoadConfig()
	cfg.ServerAddr = serverAddr
	cfg.Nickname = nickname
	a.SaveConfig(cfg)
	vpncore.Logger.Printf("Connect called: server=%s nickname=%s", serverAddr, nickname)

	if a.vpn != nil {
		a.vpn.Stop()
	}

	vpnCfg := &vpncore.VPNConfig{
		ServerAddr: serverAddr,
		Nickname:   nickname,
		P2POnly:    cfg.P2POnly,
		STUNServer: cfg.STUNServer,
		MTU:        cfg.MTU,
		DNSServer:  cfg.DNSServer,
		SOCKS5Addr: cfg.SOCKS5Proxy,
		AuthToken:  cfg.ServerToken,
	}
	a.vpn = vpncore.NewVPNCore(vpnCfg)
	a.vpn.SetP2POnly(cfg.P2POnly)
	a.wireVPN(a.vpn)
	a.mu.Unlock()

	err := a.vpn.Start()
	if err != nil {
		vpncore.Logger.Printf("Connect failed: %v", err)
		return AppStatus{}, err
	}

	a.autoJoinLastRoom(cfg)
	return a.GetStatus(), nil
}

// autoJoinLastRoom joins the persisted last room after Connect.
// Passwords are never on disk. Locked rooms without an in-memory session
// password are skipped (UI gets auto_join_skipped) instead of failing with
// "wrong password".
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
		emitEvent("auto_join_skipped", map[string]string{"room": room})
		return
	}
	vpncore.Logger.Printf("AutoJoinLastRoom: room=%s (sessionPass=%v)", room, pass != "")
	a.vpn.JoinRoom(room, pass)
}

func (a *App) wireVPN(vpn *vpncore.VPNCore) {
	vpn.OnLog = func(msg string) {
		vpncore.Logger.Println(msg)
	}
	vpn.OnStatusChange = func(s vpncore.ConnectionStatus) {
		emitEvent("status_changed", statusFrom(s))
	}
	vpn.OnError = func(msg string) {
		emitEvent("error", msg)
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
		emitEvent("peers_changed", info)
	}
	vpn.OnChat = func(fromID, nickname, message string, isDM bool) {
		emitEvent("chat_message", map[string]interface{}{
			"fromID":   fromID,
			"nickname": nickname,
			"message":  message,
			"isDM":     isDM,
		})
	}
	vpn.OnSystemChat = func(text string) {
		emitEvent("system_chat", map[string]string{"message": text})
	}
	vpn.OnRoomDeleted = func(name string) {
		emitEvent("room_deleted", map[string]string{"name": name})
	}
}

func (a *App) Disconnect() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.vpn != nil {
		a.vpn.Stop()
	}
}

func (a *App) CreateRoom(name, password string) {
	a.lastRoomName = name
	a.lastRoomPass = password
	a.SaveRoom(name, password)
	a.persistLastRoom(name, password != "")
	if a.vpn != nil {
		a.vpn.CreateRoom(name, password)
	}
}

func (a *App) JoinRoom(name, password string) {
	a.lastRoomName = name
	a.lastRoomPass = password
	a.SaveRoom(name, password)
	a.persistLastRoom(name, password != "")
	if a.vpn != nil {
		a.vpn.JoinRoom(name, password)
	}
}

func (a *App) LeaveRoom() {
	if a.vpn != nil {
		a.vpn.LeaveRoom()
	}
}

func (a *App) DeleteRoom(name string) {
	if a.vpn != nil {
		a.vpn.DeleteRoom(name)
	}
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
	if a.ctx != nil {
		runtime.ClipboardSetText(a.ctx, text)
	}
}

// FormatInvite returns server|room|password for sharing.
func (a *App) FormatInvite(room, password string) string {
	cfg := a.LoadConfig()
	if password == "" && room == a.lastRoomName {
		password = a.lastRoomPass
	}
	return cfg.ServerAddr + "|" + room + "|" + password
}

// ParseInvite parses server|room|password (password may be empty).
// Rejects empty server or empty room.
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
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

var frontendCtx context.Context

func emitEvent(event string, data interface{}) {
	if frontendCtx == nil {
		return
	}
	runtime.EventsEmit(frontendCtx, event, data)
}
