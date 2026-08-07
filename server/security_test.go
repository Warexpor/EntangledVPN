package main

import (
	"encoding/binary"
	"fmt"
	"testing"
)

func TestPasswordHashVerify(t *testing.T) {
	hash, err := hashPassword("secret")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if hash == "" || hash == "secret" {
		t.Fatal("expected argon2id hash")
	}
	if !verifyPassword(hash, "secret") {
		t.Fatal("verify failed for correct password")
	}
	if verifyPassword(hash, "wrong") {
		t.Fatal("verify should fail for wrong password")
	}
}

func TestPasswordEmpty(t *testing.T) {
	if !verifyPassword("", "") {
		t.Fatal("empty should match empty")
	}
	hash, err := hashPassword("x")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if verifyPassword(hash, "") {
		t.Fatal("empty should not match hashed")
	}
}

func TestLegacyPlaintextPassword(t *testing.T) {
	if !verifyPassword("legacy", "legacy") {
		t.Fatal("legacy plaintext compare failed")
	}
}

func TestGenerateIDUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id := generateID()
		if seen[id] {
			t.Fatalf("duplicate id %s", id)
		}
		seen[id] = true
	}
}

func TestRelayTokenValidate(t *testing.T) {
	h := NewHub()
	c := &Client{ID: "abc", VirtualIP: "10.242.0.2"}
	h.Clients[c.ID] = c
	tok := h.issueRelayToken(c, "10.242.0.2")
	if !h.ValidateRelayReg(tok, "10.242.0.2") {
		t.Fatal("expected valid token")
	}
	if h.ValidateRelayReg(tok, "10.242.0.3") {
		t.Fatal("vip mismatch should fail")
	}
	if h.ValidateRelayReg("nope", "10.242.0.2") {
		t.Fatal("bad token should fail")
	}
	if !h.VIPAssigned("10.242.0.2") {
		t.Fatal("expected vip assigned")
	}
	if h.VIPAssigned("10.242.0.9") {
		t.Fatal("unexpected vip")
	}
}

func TestParseRelayRegTokenOnly(t *testing.T) {
	h := NewHub()
	c := &Client{ID: "abc", VirtualIP: "10.242.0.2"}
	h.Clients[c.ID] = c
	r := NewRelay()
	r.Hub = h

	// Legacy VIP-only REG must be rejected
	legacy := make([]byte, 5+len("10.242.0.2"))
	binary.BigEndian.PutUint32(legacy[0:4], relayMagic)
	legacy[4] = relayTypeReg
	copy(legacy[5:], "10.242.0.2")
	if _, ok := r.parseRelayReg(legacy); ok {
		t.Fatal("legacy VIP-only REG should be rejected")
	}

	tok := h.issueRelayToken(c, "10.242.0.2")
	pkt := make([]byte, 5+1+len(tok)+len("10.242.0.2"))
	binary.BigEndian.PutUint32(pkt[0:4], relayMagic)
	pkt[4] = relayTypeReg
	pkt[5] = byte(len(tok))
	copy(pkt[6:], tok)
	copy(pkt[6+len(tok):], "10.242.0.2")
	vip, ok := r.parseRelayReg(pkt)
	if !ok || vip != "10.242.0.2" {
		t.Fatalf("token reg failed: ok=%v vip=%q", ok, vip)
	}

	// Empty token length (legacy-looking) must fail
	emptyTok := make([]byte, 5+1+len("10.242.0.2"))
	binary.BigEndian.PutUint32(emptyTok[0:4], relayMagic)
	emptyTok[4] = relayTypeReg
	emptyTok[5] = 0
	copy(emptyTok[6:], "10.242.0.2")
	if _, ok := r.parseRelayReg(emptyTok); ok {
		t.Fatal("empty-token REG should fail")
	}

	// Bad token must fail
	bad := make([]byte, 5+1+4+len("10.242.0.2"))
	binary.BigEndian.PutUint32(bad[0:4], relayMagic)
	bad[4] = relayTypeReg
	bad[5] = 4
	copy(bad[6:], "nope")
	copy(bad[10:], "10.242.0.2")
	if _, ok := r.parseRelayReg(bad); ok {
		t.Fatal("bad token should fail")
	}
}

func TestRoomOwnerPubKey(t *testing.T) {
	room := NewRoom("r", "", "owner-sess", "")
	owner := &Client{ID: "owner-sess", PublicKey: "pk-owner"}
	other := &Client{ID: "other", PublicKey: "pk-other"}
	rejoin := &Client{ID: "new-sess", PublicKey: "pk-owner"}

	if !room.isOwner(owner) {
		t.Fatal("session owner should match")
	}
	if room.isOwner(other) {
		t.Fatal("other should not be owner")
	}
	room.OwnerPubKey = "pk-owner"
	if !room.isOwner(rejoin) {
		t.Fatal("pubkey rejoin should be owner")
	}
	if room.isOwner(other) {
		t.Fatal("other pubkey should not be owner")
	}
}

func TestAssignVirtualIPExhaustion(t *testing.T) {
	h := NewHub()
	for i := 2; i <= 254; i++ {
		id := generateID()
		c := &Client{ID: id, VirtualIP: fmt.Sprintf("10.242.0.%d", i)}
		h.Clients[id] = c
	}
	newbie := &Client{ID: "newbie"}
	h.Clients[newbie.ID] = newbie
	h.mu.Lock()
	_, err := h.assignVirtualIPLocked(newbie)
	h.mu.Unlock()
	if err == nil {
		t.Fatal("expected exhaustion error")
	}
	if newbie.VirtualIP != "" {
		t.Fatalf("should not assign colliding VIP, got %s", newbie.VirtualIP)
	}
}
