package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"entangled-client/vpncore"
)

// NDJSON protocol on stdin/stdout:
//   request:  {"id":1,"method":"Connect","params":["host:8080","nick"]}
//   response: {"id":1,"result":{...}} | {"id":1,"error":"..."}
//   event:    {"event":"status_changed","data":{...}}

type rpcRequest struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type rpcResponse struct {
	ID     json.RawMessage `json:"id"`
	Result interface{}     `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

type rpcEvent struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

func main() {
	if err := vpncore.InitLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "logger: %v\n", err)
		os.Exit(1)
	}
	defer vpncore.CloseLogger()

	var outMu sync.Mutex
	writeJSON := func(v interface{}) {
		outMu.Lock()
		defer outMu.Unlock()
		enc := json.NewEncoder(os.Stdout)
		_ = enc.Encode(v)
	}

	quitCh := make(chan struct{}, 1)
	app := NewApp(
		func(event string, data interface{}) {
			writeJSON(rpcEvent{Event: event, Data: data})
		},
		func() {
			select {
			case quitCh <- struct{}{}:
			default:
			}
		},
		func(text string) error {
			// Electron owns clipboard; sidecar asks shell via event.
			writeJSON(rpcEvent{Event: "clipboard_write", Data: map[string]string{"text": text}})
			return nil
		},
	)
	defer app.Shutdown()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-sigCh:
		case <-quitCh:
		}
		app.Shutdown()
		os.Exit(0)
	}()

	vpncore.Logger.Printf("electron sidecar ready (pid=%d)", os.Getpid())
	writeJSON(rpcEvent{Event: "sidecar_ready", Data: map[string]string{"version": vpncore.AppVersion}})

	sc := bufio.NewScanner(os.Stdin)
	// Allow large chat / config payloads.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 8*1024*1024)

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			writeJSON(rpcResponse{Error: "invalid json: " + err.Error()})
			continue
		}
		result, err := dispatch(app, req.Method, req.Params)
		if err != nil {
			writeJSON(rpcResponse{ID: req.ID, Error: err.Error()})
			continue
		}
		writeJSON(rpcResponse{ID: req.ID, Result: result})
	}
	if err := sc.Err(); err != nil && err != io.EOF {
		vpncore.Logger.Printf("stdin error: %v", err)
	}
}

func dispatch(app *App, method string, params json.RawMessage) (interface{}, error) {
	switch method {
	case "GetVersion":
		return app.GetVersion(), nil
	case "GetStatus":
		return app.GetStatus(), nil
	case "GetPeers":
		peers := app.GetPeers()
		if peers == nil {
			return []PeerInfo{}, nil
		}
		return peers, nil
	case "LoadConfig", "GetSettings":
		return app.LoadConfig(), nil
	case "SaveConfig":
		var cfg ClientConfig
		if err := decodeOne(params, &cfg); err != nil {
			return nil, err
		}
		app.SaveConfig(cfg)
		return nil, nil
	case "SaveSettings":
		var cfg ClientConfig
		if err := decodeOne(params, &cfg); err != nil {
			return nil, err
		}
		return app.SaveSettings(cfg), nil
	case "ResetSettings":
		return app.ResetSettings(), nil
	case "SetStartWithWindows":
		var enabled bool
		if err := decodeOne(params, &enabled); err != nil {
			return nil, err
		}
		app.SetStartWithWindows(enabled)
		return nil, nil
	case "Connect":
		var args []string
		if err := decodeParams(params, &args); err != nil || len(args) < 2 {
			return nil, fmt.Errorf("Connect requires [server, nickname]")
		}
		return app.Connect(args[0], args[1])
	case "Disconnect":
		app.Disconnect()
		return nil, nil
	case "CreateRoom":
		var args []string
		if err := decodeParams(params, &args); err != nil || len(args) < 2 {
			return nil, fmt.Errorf("CreateRoom requires [name, password]")
		}
		return nil, app.CreateRoom(args[0], args[1])
	case "JoinRoom":
		var args []string
		if err := decodeParams(params, &args); err != nil || len(args) < 2 {
			return nil, fmt.Errorf("JoinRoom requires [name, password]")
		}
		return nil, app.JoinRoom(args[0], args[1])
	case "LeaveRoom":
		app.LeaveRoom()
		return nil, nil
	case "DeleteRoom":
		var name string
		if err := decodeOne(params, &name); err != nil {
			return nil, err
		}
		return nil, app.DeleteRoom(name)
	case "GetSavedRooms":
		return app.GetSavedRooms(), nil
	case "SaveRoom":
		var args []string
		if err := decodeParams(params, &args); err != nil || len(args) < 2 {
			return nil, fmt.Errorf("SaveRoom requires [name, password]")
		}
		app.SaveRoom(args[0], args[1])
		return nil, nil
	case "RemoveSavedRoom":
		var name string
		if err := decodeOne(params, &name); err != nil {
			return nil, err
		}
		app.RemoveSavedRoom(name)
		return nil, nil
	case "SendChat":
		var args []string
		if err := decodeParams(params, &args); err != nil || len(args) < 2 {
			return nil, fmt.Errorf("SendChat requires [toID, message]")
		}
		return nil, app.SendChat(args[0], args[1])
	case "BroadcastChat":
		var msg string
		if err := decodeOne(params, &msg); err != nil {
			return nil, err
		}
		return nil, app.BroadcastChat(msg)
	case "PingPeer":
		var id string
		if err := decodeOne(params, &id); err != nil {
			return nil, err
		}
		return nil, app.PingPeer(id)
	case "CopyText":
		var text string
		if err := decodeOne(params, &text); err != nil {
			return nil, err
		}
		app.CopyText(text)
		return nil, nil
	case "FormatInvite":
		var args []string
		if err := decodeParams(params, &args); err != nil || len(args) < 2 {
			return nil, fmt.Errorf("FormatInvite requires [room, password]")
		}
		return app.FormatInvite(args[0], args[1]), nil
	case "ParseInvite":
		var invite string
		if err := decodeOne(params, &invite); err != nil {
			return nil, err
		}
		return app.ParseInvite(invite)
	case "Quit":
		app.Quit()
		return nil, nil
	case "OpenLogFolder":
		return nil, app.OpenLogFolder()
	case "CheckForUpdate":
		return app.CheckForUpdate()
	case "ApplyUpdate":
		return nil, app.ApplyUpdate()
	case "Ping":
		return "pong", nil
	default:
		return nil, fmt.Errorf("unknown method: %s", method)
	}
}

func decodeParams(raw json.RawMessage, dest interface{}) error {
	if len(raw) == 0 || string(raw) == "null" {
		return fmt.Errorf("missing params")
	}
	return json.Unmarshal(raw, dest)
}

func decodeOne(raw json.RawMessage, dest interface{}) error {
	if len(raw) == 0 || string(raw) == "null" {
		return fmt.Errorf("missing params")
	}
	// Accept either a bare value or a one-element array (Wails-style).
	if raw[0] == '[' {
		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err != nil {
			return err
		}
		if len(arr) == 0 {
			return fmt.Errorf("empty params")
		}
		return json.Unmarshal(arr[0], dest)
	}
	return json.Unmarshal(raw, dest)
}
