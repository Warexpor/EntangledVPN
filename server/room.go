package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

type StoredRoom struct {
	Name           string `json:"name"`
	PasswordHash   string `json:"password_hash,omitempty"`
	Password       string `json:"password,omitempty"` // legacy plaintext; migrated on load
	CreatedAt      string `json:"created_at"`
	OwnerID        string `json:"owner_id"`
	OwnerToken     string `json:"owner_token,omitempty"` // legacy plaintext; migrated to hash on save
	OwnerTokenHash string `json:"owner_token_hash,omitempty"`
	OwnerPubKey    string `json:"owner_pubkey,omitempty"` // legacy metadata only; not used for auth
}

type Room struct {
	Name           string
	PasswordHash   string
	OwnerID        string
	OwnerToken     string // in-memory plaintext when freshly issued
	OwnerTokenHash string
	OwnerPubKey    string // legacy metadata only; not used for auth
	CreatedAt      string
	Clients        map[string]*Client
	mu             sync.RWMutex
}

type RelayTokenEntry struct {
	VirtualIP string
	ClientID  string
	Expiry    time.Time
	BoundUDP  string // first successful REG remote; empty until bound
}

func NewRoom(name, passwordHash, ownerID, ownerToken string) *Room {
	return &Room{
		Name:         name,
		PasswordHash: passwordHash,
		OwnerID:      ownerID,
		OwnerToken:   ownerToken,
		CreatedAt:    time.Now().Format(time.RFC3339),
		Clients:      make(map[string]*Client),
	}
}

func hashPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return "argon2id$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(hash), nil
}

// isOwner is session-ID only. Reclaim across reconnects uses OwnerToken
// (see claimOwner), not the advertised peer public key (forgeable).
func (r *Room) isOwner(c *Client) bool {
	if r == nil || c == nil {
		return false
	}
	return r.OwnerID != "" && r.OwnerID == c.ID
}

func (r *Room) validOwnerToken(token string) bool {
	if r == nil || token == "" {
		return false
	}
	if r.OwnerToken != "" {
		return subtle.ConstantTimeCompare([]byte(r.OwnerToken), []byte(token)) == 1
	}
	if r.OwnerTokenHash != "" {
		sum := sha256.Sum256([]byte(token))
		got := hex.EncodeToString(sum[:])
		return subtle.ConstantTimeCompare([]byte(r.OwnerTokenHash), []byte(got)) == 1
	}
	return false
}

// claimOwner binds this session as owner when token matches.
// Empty-token first-claimer reclaim is refused (fail closed).
func (r *Room) claimOwner(c *Client, token string) (isOwner bool, issuedToken string) {
	if r == nil || c == nil {
		return false, ""
	}
	if r.validOwnerToken(token) {
		r.OwnerID = c.ID
		if r.OwnerToken == "" && token != "" {
			r.OwnerToken = token
		}
		return true, r.OwnerToken
	}
	if r.isOwner(c) {
		return true, r.OwnerToken
	}
	return false, ""
}

func newOwnerToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func verifyPassword(stored, password string) bool {
	if stored == "" && password == "" {
		return true
	}
	if stored == "" || password == "" {
		return false
	}
	// Legacy plaintext migration path
	if !strings.HasPrefix(stored, "argon2id$") {
		return subtle.ConstantTimeCompare([]byte(stored), []byte(password)) == 1
	}
	parts := strings.Split(stored, "$")
	if len(parts) != 3 {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[1])
	want, err2 := base64.RawStdEncoding.DecodeString(parts[2])
	if err1 != nil || err2 != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return subtle.ConstantTimeCompare(got, want) == 1
}

func newRelayToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		log.Printf("newRelayToken: rand.Read failed: %v", err)
		return ""
	}
	return hex.EncodeToString(b)
}

func ownerTokenHash(token string) string {
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (r *Room) Broadcast(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Room.Broadcast marshal error: %v", err)
		return
	}
	critical := isCriticalSignaling(msg.Type)
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.Clients {
		if critical {
			select {
			case c.Send <- data:
			case <-time.After(2 * time.Second):
				log.Printf("Room.Broadcast: critical %s drop after timeout for %s", msg.Type, c.ID)
			}
			continue
		}
		select {
		case c.Send <- data:
		default:
		}
	}
}

func (r *Room) BroadcastExcept(exclude *Client, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Room.BroadcastExcept marshal error: %v", err)
		return
	}
	critical := isCriticalSignaling(msg.Type)
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.Clients {
		if c == exclude {
			continue
		}
		if critical {
			select {
			case c.Send <- data:
			case <-time.After(2 * time.Second):
				log.Printf("Room.BroadcastExcept: critical %s drop after timeout for %s", msg.Type, c.ID)
			}
			continue
		}
		select {
		case c.Send <- data:
		default:
		}
	}
}

func isCriticalSignaling(t string) bool {
	switch t {
	case "auth_ok", "auth_fail", "room_joined", "room_error", "room_deleted",
		"peer_joined", "peer_left", "peer_updated", "error":
		return true
	default:
		return false
	}
}

func (r *Room) GetPeersList(excludeID string) []map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	peers := make([]map[string]string, 0, len(r.Clients))
	for _, c := range r.Clients {
		if c.ID == excludeID {
			continue
		}
		peers = append(peers, map[string]string{
			"id":       c.ID,
			"nickname": c.Nickname,
		})
	}
	return peers
}

func (r *Room) HasClient(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.Clients[id]
	return ok
}

func (h *Hub) issueRelayToken(client *Client, vip string) string {
	token := newRelayToken()
	h.mu.Lock()
	if h.RelayTokens == nil {
		h.RelayTokens = make(map[string]*RelayTokenEntry)
	}
	h.RelayTokens[token] = &RelayTokenEntry{
		VirtualIP: vip,
		ClientID:  client.ID,
		Expiry:    time.Now().Add(3 * time.Minute),
	}
	h.mu.Unlock()
	return token
}

func (h *Hub) ValidateRelayReg(token, vip, remote string) bool {
	if token == "" || vip == "" {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	e, ok := h.RelayTokens[token]
	if !ok {
		return false
	}
	if time.Now().After(e.Expiry) {
		delete(h.RelayTokens, token)
		return false
	}
	if e.VirtualIP != vip {
		return false
	}
	if e.BoundUDP == "" {
		e.BoundUDP = remote
	} else if remote != "" && e.BoundUDP != remote {
		return false
	}
	// refresh on successful use
	e.Expiry = time.Now().Add(3 * time.Minute)
	return true
}

// VIPAssigned is true when some connected client currently holds this virtual IP.
func (h *Hub) VIPAssigned(vip string) bool {
	if h == nil || vip == "" {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.Clients {
		if c.VirtualIP == vip {
			return true
		}
	}
	return false
}

func (h *Hub) CreateRoom(name, password string, creator *Client) {
	hash, err := hashPassword(password)
	if err != nil {
		log.Printf("create_room failed: hashPassword: %v", err)
		creator.sendError("failed to create room")
		return
	}

	ownerToken := newOwnerToken()
	if ownerToken == "" {
		log.Printf("create_room failed: owner token entropy")
		creator.sendError("failed to create room")
		return
	}

	h.mu.Lock()

	if _, exists := h.Rooms[name]; exists {
		h.mu.Unlock()
		creator.sendError("room already exists")
		return
	}

	room := NewRoom(name, hash, creator.ID, ownerToken)
	h.Rooms[name] = room

	h.leaveRoom(creator)

	room.mu.Lock()
	room.Clients[creator.ID] = creator
	creator.mu.Lock()
	creator.Room = room
	creator.mu.Unlock()
	room.mu.Unlock()

	vip, err := h.assignVirtualIPLocked(creator)
	h.mu.Unlock()

	if err != nil {
		h.LeaveRoom(creator)
		h.mu.Lock()
		delete(h.Rooms, name)
		h.mu.Unlock()
		h.SaveRooms()
		creator.sendError(err.Error())
		return
	}

	h.SaveRooms()
	log.Printf("room created: %s by %s (%s)", name, creator.Nickname, creator.ID)

	token := h.issueRelayToken(creator, vip)
	creator.sendMessage("room_joined", mustMarshal(map[string]interface{}{
		"room":         name,
		"is_owner":     true,
		"virtual_ip":   vip,
		"peer_list":    room.GetPeersList(creator.ID),
		"relay_token":  token,
		"owner_token":  ownerToken,
	}))
}

func (h *Hub) JoinRoom(name, password, ownerToken string, client *Client) {
	h.mu.Lock()

	room, exists := h.Rooms[name]
	if !exists {
		h.mu.Unlock()
		log.Printf("join_room failed: room %s not found for %s", name, client.Nickname)
		client.sendError("room not found")
		return
	}

	if !verifyPassword(room.PasswordHash, password) {
		h.mu.Unlock()
		log.Printf("join_room failed: wrong password for %s by %s", name, client.Nickname)
		client.sendError("wrong password")
		return
	}

	h.leaveRoom(client)

	room.mu.Lock()
	room.Clients[client.ID] = client
	client.mu.Lock()
	client.Room = room
	client.mu.Unlock()

	type existingData struct {
		ID, VirtualIP, LocalAddr, PublicKey, Crypto string
		PublicIP                                    net.IP
		ExternalAddr                                string
	}
	existingList := make([]existingData, 0, len(room.Clients))
	for _, existing := range room.Clients {
		if existing.ID == client.ID {
			continue
		}
		existing.mu.Lock()
		existingList = append(existingList, existingData{
			ID:           existing.ID,
			VirtualIP:    existing.VirtualIP,
			LocalAddr:    existing.LocalAddr,
			PublicKey:    existing.PublicKey,
			PublicIP:     existing.PublicIP,
			ExternalAddr: existing.ExternalAddr,
			Crypto:       existing.Crypto,
		})
		existing.mu.Unlock()
	}
	room.mu.Unlock()

	owner, issuedToken := room.claimOwner(client, ownerToken)
	dirty := owner && issuedToken != ""

	vip, err := h.assignVirtualIPLocked(client)
	h.mu.Unlock()

	if dirty {
		h.SaveRooms()
	}

	if err != nil {
		h.LeaveRoom(client)
		client.sendError(err.Error())
		return
	}

	for _, ed := range existingList {
		publicAddr := choosePublicAddr(ed.PublicIP, ed.ExternalAddr, ed.LocalAddr)
		payload := map[string]string{
			"id":          ed.ID,
			"virtual_ip":  ed.VirtualIP,
			"local_addr":  ed.LocalAddr,
			"public_addr": publicAddr,
			"public_key":  ed.PublicKey,
		}
		if ed.Crypto != "" {
			payload["crypto"] = ed.Crypto
		}
		client.sendMessage("peer_updated", mustMarshal(payload))
	}

	peers := room.GetPeersList(client.ID)
	token := h.issueRelayToken(client, vip)

	log.Printf("room joined: %s by %s (%s), peers=%d, vip=%s, owner=%v", name, client.Nickname, client.ID, len(peers), vip, owner)

	payload := map[string]interface{}{
		"room":        name,
		"is_owner":    owner,
		"virtual_ip":  vip,
		"peer_list":   peers,
		"relay_token": token,
	}
	if owner && issuedToken != "" {
		payload["owner_token"] = issuedToken
	}
	client.sendMessage("room_joined", mustMarshal(payload))

	room.BroadcastExcept(client, Message{
		Type: "peer_joined",
		Payload: mustMarshal(map[string]string{
			"id":       client.ID,
			"nickname": client.Nickname,
		}),
	})
}

func (h *Hub) LeaveRoom(client *Client) {
	h.mu.Lock()
	h.leaveRoom(client)
	h.mu.Unlock()
}

func (h *Hub) leaveRoom(client *Client) {
	client.mu.Lock()
	room := client.Room
	if room == nil {
		client.VirtualIP = ""
		client.mu.Unlock()
		return
	}
	client.Room = nil
	client.VirtualIP = ""
	client.mu.Unlock()

	room.mu.Lock()
	delete(room.Clients, client.ID)
	room.mu.Unlock()

	log.Printf("client left room: %s (%s) from %s", client.Nickname, client.ID, room.Name)

	room.Broadcast(Message{
		Type: "peer_left",
		Payload: mustMarshal(map[string]string{
			"id": client.ID,
		}),
	})
}

func (h *Hub) DeleteRoom(name, ownerToken string, requester *Client) {
	h.mu.Lock()
	room, exists := h.Rooms[name]
	if !exists {
		h.mu.Unlock()
		requester.sendError("room not found")
		return
	}
	if !room.isOwner(requester) && !room.validOwnerToken(ownerToken) {
		h.mu.Unlock()
		requester.sendError("only the room owner can delete this room")
		return
	}
	delete(h.Rooms, name)
	h.mu.Unlock()
	h.SaveRooms()

	room.mu.RLock()
	members := make([]*Client, 0, len(room.Clients))
	for _, c := range room.Clients {
		c.mu.Lock()
		c.Room = nil
		c.VirtualIP = ""
		c.mu.Unlock()
		members = append(members, c)
	}
	room.mu.RUnlock()

	payload := mustMarshal(map[string]string{"name": name, "reason": "deleted"})
	for _, c := range members {
		c.sendMessage("room_deleted", payload)
	}

	log.Printf("room deleted: %s by %s", name, requester.Nickname)
}

func roomsPath() string {
	if d := os.Getenv("ENTANGLED_DATA_DIR"); d != "" {
		return filepath.Join(d, "rooms.json")
	}
	return "rooms.json"
}

func (h *Hub) SaveRooms() {
	h.mu.Lock()
	defer h.mu.Unlock()
	rooms := make(map[string]StoredRoom)
	for name, room := range h.Rooms {
		hash := room.OwnerTokenHash
		if hash == "" && room.OwnerToken != "" {
			hash = ownerTokenHash(room.OwnerToken)
			room.OwnerTokenHash = hash
		}
		rooms[name] = StoredRoom{
			Name:           room.Name,
			PasswordHash:   room.PasswordHash,
			CreatedAt:      room.CreatedAt,
			OwnerID:        room.OwnerID,
			OwnerTokenHash: hash,
			OwnerPubKey:    room.OwnerPubKey,
		}
	}
	data, err := json.MarshalIndent(rooms, "", "  ")
	if err != nil {
		log.Printf("failed to marshal rooms: %v", err)
		return
	}
	path := roomsPath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		log.Printf("failed to save rooms: %v", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		log.Printf("failed to rename rooms: %v", err)
		_ = os.Remove(tmp)
		return
	}
	log.Printf("saved %d rooms to %s", len(rooms), path)
}

func (h *Hub) LoadRooms() {
	h.mu.Lock()
	defer h.mu.Unlock()
	data, err := os.ReadFile(roomsPath())
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("no rooms file at %s, starting fresh", roomsPath())
			return
		}
		log.Printf("failed to load rooms: %v", err)
		return
	}
	var stored map[string]StoredRoom
	if err := json.Unmarshal(data, &stored); err != nil {
		log.Printf("failed to parse rooms: %v", err)
		return
	}
	migrated := false
	for name, sr := range stored {
		hash := sr.PasswordHash
		if hash == "" && sr.Password != "" {
			h2, err := hashPassword(sr.Password)
			if err != nil {
				log.Printf("failed to migrate password for room %s: %v", name, err)
				continue
			}
			hash = h2
			migrated = true
		}
		tokenHash := sr.OwnerTokenHash
		ownerToken := ""
		if tokenHash == "" && sr.OwnerToken != "" {
			tokenHash = ownerTokenHash(sr.OwnerToken)
			migrated = true
		}
		h.Rooms[name] = &Room{
			Name:           sr.Name,
			PasswordHash:   hash,
			OwnerID:        sr.OwnerID,
			OwnerToken:     ownerToken,
			OwnerTokenHash: tokenHash,
			OwnerPubKey:    sr.OwnerPubKey,
			CreatedAt:      sr.CreatedAt,
			Clients:        make(map[string]*Client),
		}
	}
	log.Printf("loaded %d rooms from %s", len(stored), roomsPath())
	if migrated {
		go func() {
			time.Sleep(100 * time.Millisecond)
			h.SaveRooms()
			log.Printf("migrated plaintext room passwords to argon2id")
		}()
	}
}

// assignVirtualIPLocked picks a free VIP and assigns it. Caller must hold h.mu.
func (h *Hub) assignVirtualIPLocked(client *Client) (string, error) {
	client.mu.Lock()
	if client.VirtualIP != "" {
		ip := client.VirtualIP
		client.mu.Unlock()
		return ip, nil
	}
	client.mu.Unlock()

	taken := map[string]bool{}
	for _, c := range h.Clients {
		c.mu.Lock()
		vip := c.VirtualIP
		c.mu.Unlock()
		if vip != "" {
			taken[vip] = true
		}
	}

	for i := 2; i <= 254; i++ {
		ip := fmt.Sprintf("10.242.0.%d", i)
		if !taken[ip] {
			client.mu.Lock()
			client.VirtualIP = ip
			client.mu.Unlock()
			return ip, nil
		}
	}
	return "", fmt.Errorf("no virtual IPs available")
}
