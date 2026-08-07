package main

import (
	"flag"
	"log"
	"net/http"
	"os"
)

func main() {
	addr := flag.String("addr", ":8080", "WebSocket listen address")
	relayAddr := flag.String("relay", ":3478", "UDP relay listen address")
	flag.Parse()

	relay := NewRelay()
	if err := relay.Start(*relayAddr); err != nil {
		log.Fatalf("Relay start failed: %v", err)
	}

	hub := NewHub()
	hub.Relay = relay
	relay.Hub = hub
	hub.LoadRooms()
	go hub.Run()

	if hub.AuthToken != "" {
		log.Printf("Server token auth enabled (ENTANGLED_TOKEN set)")
	} else {
		log.Printf("Server token auth disabled (open join) — set ENTANGLED_TOKEN to require a shared secret")
	}

	http.HandleFunc("/ws", hub.HandleWS)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	log.Printf("Entangled server starting on %s, relay on %s (pid %d)", *addr, *relayAddr, os.Getpid())
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal(err)
	}
}
