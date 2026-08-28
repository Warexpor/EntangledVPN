import App from './App.svelte'
import './app.css'

if (import.meta.env.DEV && !window.go && new URLSearchParams(window.location.search).has('demo')) {
  const demoStatus = {
    connected: true,
    reconnecting: false,
    server: 'demo.entangled.app:8080',
    room: 'Friday LAN',
    virtualIP: '10.42.0.7',
    peerCount: 3,
    isOwner: true,
    phase: 'ready',
  }
  const demoPeers = [
    { id: 'atlas', nickname: 'Atlas', virtualIP: '10.42.0.8', connected: true, ping: 18, path: 'p2p' },
    { id: 'nova', nickname: 'Nova', virtualIP: '10.42.0.9', connected: true, ping: 42, path: 'relay' },
    { id: 'moss', nickname: 'Moss', virtualIP: '10.42.0.10', connected: false, ping: -1, path: '' },
  ]
  const callbacks = {}
  const appApi = {
    LoadConfig: async () => ({ serverAddr: demoStatus.server, nickname: 'You', theme: 'dark', lang: 'en', uiScale: 100 }),
    GetSettings: async () => ({ serverAddr: demoStatus.server, nickname: 'You', theme: 'dark', lang: 'en', uiScale: 100 }),
    GetStatus: async () => demoStatus,
    GetPeers: async () => demoPeers,
    GetSavedRooms: async () => [{ name: demoStatus.room, server: demoStatus.server, locked: false }],
    GetVersion: async () => '1.3.0-demo',
    CopyText: async () => {},
    PingPeer: async () => {},
    SaveSettings: async () => false,
    ResetSettings: async () => ({ serverAddr: '', nickname: '', theme: 'dark', lang: 'en', uiScale: 100 }),
    Connect: async () => demoStatus,
    Disconnect: async () => {},
    CreateRoom: async () => {},
    JoinRoom: async () => {},
    LeaveRoom: async () => {},
    DeleteRoom: async () => {},
    RemoveSavedRoom: async () => {},
    SendChat: async () => {},
    BroadcastChat: async () => {},
    CheckForUpdate: async () => ({ available: false }),
    ApplyUpdate: async () => {},
  }
  window.go = { main: { App: appApi } }
  window.runtime = {
    EventsOn(name, callback) {
      callbacks[name] = callback
      if (name === 'status_changed') callback(demoStatus)
      if (name === 'peers_changed') callback(demoPeers)
    },
    EventsOff(name) {
      delete callbacks[name]
    },
  }
}

const app = new App({
  target: document.getElementById('app'),
})

export default app
