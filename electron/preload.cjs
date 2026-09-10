const { contextBridge, ipcRenderer } = require('electron')

function call(method, ...args) {
  return ipcRenderer.invoke('vpn:call', method, args)
}

const App = {
  GetVersion: () => call('GetVersion'),
  GetStatus: () => call('GetStatus'),
  GetPeers: () => call('GetPeers'),
  LoadConfig: () => call('LoadConfig'),
  SaveConfig: (cfg) => call('SaveConfig', cfg),
  GetSettings: () => call('GetSettings'),
  SaveSettings: (cfg) => call('SaveSettings', cfg),
  ResetSettings: () => call('ResetSettings'),
  SetStartWithWindows: (enabled) => call('SetStartWithWindows', enabled),
  Connect: (server, nick) => call('Connect', server, nick),
  Disconnect: () => call('Disconnect'),
  CreateRoom: (name, password) => call('CreateRoom', name, password),
  JoinRoom: (name, password) => call('JoinRoom', name, password),
  LeaveRoom: () => call('LeaveRoom'),
  DeleteRoom: (name) => call('DeleteRoom', name),
  GetSavedRooms: () => call('GetSavedRooms'),
  SaveRoom: (name, password) => call('SaveRoom', name, password),
  RemoveSavedRoom: (name) => call('RemoveSavedRoom', name),
  SendChat: (toID, message) => call('SendChat', toID, message),
  BroadcastChat: (message) => call('BroadcastChat', message),
  PingPeer: (peerID) => call('PingPeer', peerID),
  CopyText: (text) => call('CopyText', text),
  FormatInvite: (room, password) => call('FormatInvite', room, password),
  ParseInvite: (invite) => call('ParseInvite', invite),
  Quit: () => call('Quit'),
  OpenLogFolder: () => call('OpenLogFolder'),
  CheckForUpdate: () => call('CheckForUpdate'),
  ApplyUpdate: () => call('ApplyUpdate'),
}

const eventHandlers = new Map()

ipcRenderer.on('vpn:event', (_evt, payload) => {
  const name = payload && payload.event
  if (!name) return
  const set = eventHandlers.get(name)
  if (!set) return
  for (const cb of set) {
    try {
      cb(payload.data)
    } catch (_) {}
  }
})

const runtime = {
  EventsOn(name, callback) {
    if (!eventHandlers.has(name)) eventHandlers.set(name, new Set())
    eventHandlers.get(name).add(callback)
    return () => runtime.EventsOff(name, callback)
  },
  EventsOff(name, callback) {
    const set = eventHandlers.get(name)
    if (!set) return
    if (callback) set.delete(callback)
    else eventHandlers.delete(name)
  },
  async WindowIsFullscreen() {
    return ipcRenderer.invoke('window:isFullscreen')
  },
  WindowFullscreen() {
    return ipcRenderer.invoke('window:setFullscreen', true)
  },
  WindowUnfullscreen() {
    return ipcRenderer.invoke('window:setFullscreen', false)
  },
}

contextBridge.exposeInMainWorld('go', { main: { App } })
contextBridge.exposeInMainWorld('runtime', runtime)
contextBridge.exposeInMainWorld('entangledShell', { kind: 'electron' })
