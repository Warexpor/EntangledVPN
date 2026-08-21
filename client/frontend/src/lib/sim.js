import { status, view, peers, settings, addChatMessage, chatOpen } from './stores/app.js'

const SIM = import.meta.env.DEV && !window.go
const jumpConnected = SIM && new URLSearchParams(location.search).has('connected')

const cfg = {
  serverAddr: 'sim.local:8080',
  nickname: 'Ada',
  autoConnect: false,
  autoJoinLastRoom: false,
  startWithWindows: false,
  connectionMode: 'direct',
  mtu: 1500,
  dnsServer: '',
  socks5Proxy: '',
  stunServer: 'stun.l.google.com:19302',
  uiScale: 100,
  theme: 'dark',
  lang: 'en',
  serverToken: '',
}

let rooms = [
  { name: 'nightwatch', password: '', server: 'sim.local:8080', locked: false },
  { name: 'lan-party', password: 'secret', server: 'sim.local:8080', locked: true },
]

const simPeers = [
  {
    id: 'peer-bob',
    nickname: 'Bob',
    virtualIP: '10.242.0.3',
    connected: true,
    ping: 18,
    path: 'p2p',
  },
  {
    id: 'peer-cy',
    nickname: 'Cy',
    virtualIP: '10.242.0.4',
    connected: true,
    ping: 84,
    path: 'relay',
  },
]

function connectedStatus(room = 'nightwatch') {
  return {
    connected: true,
    reconnecting: false,
    server: 'sim.local:8080',
    room,
    virtualIP: '10.242.0.2',
    peerCount: simPeers.filter((p) => p.connected).length,
    isOwner: room === 'nightwatch',
    phase: 'ready',
  }
}

function enterConnected(room) {
  status.set(connectedStatus(room))
  peers.set(simPeers.map((p) => ({ ...p })))
  view.set('network')
  chatOpen.set(true)
}

if (SIM) {
  window.go = {
    main: {
      App: {
        LoadConfig: async () => ({ ...cfg }),
        GetSettings: async () => ({ ...cfg }),
        SaveSettings: async (next) => {
          Object.assign(cfg, next)
          return false
        },
        ResetSettings: async () => {
          cfg.theme = 'dark'
          cfg.uiScale = 100
          cfg.lang = 'en'
          return { ...cfg }
        },
        GetVersion: async () => '1.3.1-sim',
        GetSavedRooms: async () => rooms.slice(),
        CreateRoom: async (name, password) => {
          rooms = [...rooms, { name, password: password || '', server: cfg.serverAddr, locked: !!password }]
          enterConnected(name)
        },
        JoinRoom: async (name) => {
          enterConnected(name)
        },
        LeaveRoom: async () => {
          status.set({ ...connectedStatus(''), room: '', peerCount: 0, isOwner: false, connected: true })
        },
        RemoveSavedRoom: async (name) => {
          rooms = rooms.filter((r) => r.name !== name)
        },
        DeleteRoom: async (name) => {
          rooms = rooms.filter((r) => r.name !== name)
          status.set({ ...connectedStatus(''), room: '', peerCount: 0, isOwner: false, connected: true })
        },
        FormatInvite: async (name, password) => `sim.local:8080|${name}|${password || ''}`,
        ParseInvite: async (raw) => {
          const [server, room, password] = String(raw).split('|')
          return { server, room, password }
        },
        CopyText: async (text) => {
          try {
            await navigator.clipboard.writeText(String(text))
          } catch (_) {}
        },
        Connect: async (serverAddr, nickname) => {
          cfg.serverAddr = serverAddr
          cfg.nickname = nickname
          const s = connectedStatus('nightwatch')
          s.server = serverAddr
          peers.set(simPeers.map((p) => ({ ...p })))
          return s
        },
        Disconnect: async () => {
          status.set({
            connected: false,
            reconnecting: false,
            server: '',
            room: '',
            virtualIP: '',
            peerCount: 0,
            isOwner: false,
            phase: 'idle',
          })
          peers.set([])
          chatOpen.set(false)
        },
        SendChat: async (peerId, message) => {
          const peer = simPeers.find((p) => p.id === peerId)
          if (peer) {
            addChatMessage(peer.id, peer.nickname, `ack: ${message}`, false, 'sent', false, true)
          }
        },
        BroadcastChat: async (message) => {
          const peer = simPeers[0]
          if (peer) {
            addChatMessage(peer.id, peer.nickname, `heard: ${message}`, false, 'sent', false, false)
          }
        },
        PingPeer: async (peerId) => {
          const i = simPeers.findIndex((p) => p.id === peerId)
          if (i >= 0) {
            simPeers[i] = { ...simPeers[i], ping: 12 + Math.floor(Math.random() * 40) }
            peers.set(simPeers.map((p) => ({ ...p })))
          }
        },
        CheckForUpdate: async () => ({ available: false, latest: '1.3.1-sim' }),
        ApplyUpdate: async () => {},
      },
    },
  }
  window.runtime = {
    EventsOn() {},
    EventsOff() {},
    WindowIsFullscreen: async () => false,
    WindowFullscreen() {},
    WindowUnfullscreen() {},
  }

  settings.set({ ...cfg })
  if (jumpConnected) {
    enterConnected()
    addChatMessage('peer-bob', 'Bob', 'Sim is live — tweak away.', false, 'sent')
  }
}
