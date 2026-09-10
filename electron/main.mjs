import { app, BrowserWindow, clipboard, ipcMain, shell } from 'electron'
import { spawn, spawnSync } from 'node:child_process'
import { createInterface } from 'node:readline'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const isDev = process.argv.includes('--dev') || !app.isPackaged

let mainWindow = null
let sidecar = null
let sidecarReady = false
const pending = new Map()
let nextId = 1
const lineBufferWaiters = []

function uiRoot() {
  if (app.isPackaged) {
    return path.join(process.resourcesPath, 'ui')
  }
  const electronUi = path.join(__dirname, 'ui-dist')
  if (fs.existsSync(path.join(electronUi, 'index.html'))) {
    return electronUi
  }
  // Fallback for quick demos if ui-dist not built yet.
  return path.join(__dirname, '..', 'client', 'frontend', 'dist')
}

function sidecarPath() {
  const name = process.platform === 'win32' ? 'entangled-sidecar.exe' : 'entangled-sidecar'
  if (app.isPackaged) {
    return path.join(process.resourcesPath, name)
  }
  return path.join(__dirname, 'sidecar', name)
}

/** AppImage/squashfs can't keep file capabilities — copy sidecar to a writable path and setcap. */
function resolveRunnableSidecar() {
  const bundled = sidecarPath()
  if (process.platform !== 'linux' || !app.isPackaged) {
    return bundled
  }
  if (!fs.existsSync(bundled)) {
    throw new Error(`sidecar binary missing: ${bundled}`)
  }
  const destDir = path.join(app.getPath('userData'), 'bin')
  const dest = path.join(destDir, 'entangled-sidecar')
  fs.mkdirSync(destDir, { recursive: true })
  const bundledStat = fs.statSync(bundled)
  let needCopy = !fs.existsSync(dest)
  if (!needCopy) {
    const destStat = fs.statSync(dest)
    needCopy = bundledStat.size !== destStat.size || bundledStat.mtimeMs > destStat.mtimeMs
  }
  if (needCopy) {
    fs.copyFileSync(bundled, dest)
    fs.chmodSync(dest, 0o755)
  }
  ensureLinuxTunCaps(dest)
  return dest
}

function ensureLinuxTunCaps(bin) {
  try {
    const check = spawnSync('getcap', [bin], { encoding: 'utf8' })
    if ((check.stdout || '').includes('cap_net_admin')) {
      return
    }
  } catch (_) {}
  // Best-effort; user can cancel the pkexec dialog.
  try {
    const r = spawnSync('pkexec', ['setcap', 'cap_net_admin,cap_net_raw+ep', bin], {
      encoding: 'utf8',
      timeout: 120000,
    })
    if (r.status !== 0) {
      console.warn('setcap via pkexec failed; TUN may need: sudo setcap cap_net_admin,cap_net_raw+ep', bin)
    }
  } catch (err) {
    console.warn('setcap skipped:', err)
  }
}

function sendLine(obj) {
  if (!sidecar || !sidecar.stdin.writable) {
    throw new Error('sidecar not running')
  }
  sidecar.stdin.write(JSON.stringify(obj) + '\n')
}

function callSidecar(method, params) {
  return new Promise((resolve, reject) => {
    const id = nextId++
    pending.set(id, { resolve, reject })
    try {
      sendLine({ id, method, params })
    } catch (err) {
      pending.delete(id)
      reject(err)
    }
    setTimeout(() => {
      if (pending.has(id)) {
        pending.delete(id)
        reject(new Error(`sidecar timeout: ${method}`))
      }
    }, 120000)
  })
}

function handleSidecarMessage(msg) {
  if (msg.event) {
    if (msg.event === 'sidecar_ready') {
      sidecarReady = true
    }
    if (msg.event === 'clipboard_write' && msg.data?.text != null) {
      clipboard.writeText(String(msg.data.text))
      return
    }
    if (mainWindow && !mainWindow.isDestroyed()) {
      mainWindow.webContents.send('vpn:event', { event: msg.event, data: msg.data })
    }
    return
  }
  if (msg.id == null) return
  const waiter = pending.get(msg.id)
  if (!waiter) return
  pending.delete(msg.id)
  if (msg.error) waiter.reject(new Error(msg.error))
  else waiter.resolve(msg.result)
}

function startSidecar() {
  const bin = resolveRunnableSidecar()
  if (!fs.existsSync(bin)) {
    throw new Error(`sidecar binary missing: ${bin} (run npm run build:sidecar)`)
  }
  const env = {
    ...process.env,
    ENTANGLED_ELECTRON_EXEC: process.execPath,
  }
  sidecar = spawn(bin, [], {
    stdio: ['pipe', 'pipe', 'pipe'],
    env,
    windowsHide: true,
  })
  sidecarReady = false

  const rl = createInterface({ input: sidecar.stdout })
  rl.on('line', (line) => {
    try {
      handleSidecarMessage(JSON.parse(line))
    } catch (err) {
      console.error('sidecar bad json:', line, err)
    }
  })
  sidecar.stderr.on('data', (chunk) => {
    process.stderr.write(chunk)
  })
  sidecar.on('exit', (code, signal) => {
    console.error(`sidecar exited code=${code} signal=${signal}`)
    for (const [, w] of pending) w.reject(new Error('sidecar exited'))
    pending.clear()
    sidecar = null
    sidecarReady = false
  })
}

async function waitSidecarReady(ms = 8000) {
  const start = Date.now()
  while (!sidecarReady) {
    if (!sidecar) throw new Error('sidecar failed to start')
    if (Date.now() - start > ms) break
    await new Promise((r) => setTimeout(r, 50))
  }
}

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1100,
    height: 720,
    minWidth: 720,
    minHeight: 480,
    backgroundColor: '#080808',
    title: 'Entangled VPN',
    webPreferences: {
      preload: path.join(__dirname, 'preload.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: false,
    },
  })

  const index = path.join(uiRoot(), 'index.html')
  if (!fs.existsSync(index)) {
    throw new Error(`UI missing at ${index} — run npm run build:ui`)
  }
  mainWindow.loadFile(index)

  if (isDev) {
    mainWindow.webContents.openDevTools({ mode: 'detach' })
  }

  mainWindow.on('closed', () => {
    mainWindow = null
  })
}

app.whenReady().then(async () => {
  startSidecar()
  await waitSidecarReady()
  createWindow()

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow()
  })
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit()
})

app.on('before-quit', () => {
  try {
    if (sidecar) {
      sendLine({ id: nextId++, method: 'Quit', params: [] })
      sidecar.kill('SIGTERM')
    }
  } catch (_) {}
})

ipcMain.handle('vpn:call', async (_evt, method, args) => {
  const params = Array.isArray(args) ? args : []
  // Single-object methods: pass as one-element array; decodeOne accepts both.
  return callSidecar(method, params)
})

ipcMain.handle('window:isFullscreen', () => {
  return mainWindow ? mainWindow.isFullScreen() : false
})

ipcMain.handle('window:setFullscreen', (_evt, on) => {
  if (mainWindow) mainWindow.setFullScreen(!!on)
})

// Unused but keeps import for potential external links later.
void shell
void lineBufferWaiters
