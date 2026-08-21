<script>
  import { onMount } from 'svelte'
  import { settings, addNotification } from '../stores/app.js'
  import { t, setLang, currentLang, fmt } from '../locales/index.js'
  import ThemeToggle from './ThemeToggle.svelte'
  import ConfirmDialog from './ConfirmDialog.svelte'

  let cfg = { ...$settings }
  let version = '1.0.0'
  let needsReconnect = false
  let updateBusy = false
  let updateStatus = '' // '', 'checking', 'latest', 'available', 'downloading', 'error'
  let updateLatest = ''
  let updateError = ''
  let pending = null

  async function checkForUpdate() {
    updateBusy = true
    updateStatus = 'checking'
    updateError = ''
    updateLatest = ''
    try {
      const info = await window.go.main.App.CheckForUpdate()
      if (info?.available) {
        updateStatus = 'available'
        updateLatest = info.latest || ''
      } else {
        updateStatus = 'latest'
      }
    } catch (e) {
      updateStatus = 'error'
      updateError = String(e)
      addNotification($t.error_update_check + e, 'error')
    } finally {
      updateBusy = false
    }
  }

  async function applyUpdate() {
    pending = {
      message: $t.confirm_update,
      confirmLabel: $t.apply_update,
      run: async () => {
        updateBusy = true
        updateStatus = 'downloading'
        updateError = ''
        try {
          await window.go.main.App.ApplyUpdate()
          addNotification($t.notif_updating, 'info')
        } catch (e) {
          updateStatus = 'error'
          updateError = String(e)
          updateBusy = false
          addNotification($t.error_update + e, 'error')
        }
      },
    }
  }

  async function save() {
    $settings = { ...cfg }
    try {
      needsReconnect = !!(await window.go.main.App.SaveSettings(cfg))
      if (cfg.lang) setLang(cfg.lang)
      addNotification($t.notif_settings_saved, 'info')
      if (needsReconnect) {
        addNotification($t.notif_reconnect_required, 'info')
      }
    } catch (e) {
      addNotification($t.error_save + e, 'error')
    }
  }

  async function resetDefaults() {
    pending = {
      message: $t.confirm_reset,
      confirmLabel: $t.reset,
      danger: true,
      run: async () => {
        try {
          cfg = await window.go.main.App.ResetSettings()
          $settings = { ...cfg }
          needsReconnect = false
          if (cfg.lang) setLang(cfg.lang)
          addNotification($t.notif_settings_saved, 'info')
        } catch (e) {
          addNotification($t.error_save + e, 'error')
        }
      },
    }
  }

  function onScaleInput(e) {
    const n = Math.min(150, Math.max(75, Number(e.target.value) || 100))
    cfg.uiScale = n
    $settings = { ...$settings, uiScale: n }
  }

  function handleLangChange(e) {
    cfg.lang = e.target.value
    setLang(e.target.value)
  }

  $: if ($settings.theme && cfg.theme !== $settings.theme) {
    cfg.theme = $settings.theme
  }

  let open = { connection: true, network: false, system: false, appearance: true }

  function toggle(key) {
    open = { ...open, [key]: !open[key] }
  }

  onMount(async () => {
    try {
      const saved = await window.go.main.App.GetSettings()
      if (saved) {
        const theme = $settings.theme || saved.theme
        cfg = { ...cfg, ...saved, theme }
        if (!cfg.connectionMode) cfg.connectionMode = 'direct'
        $settings = { ...cfg, theme }
        if (cfg.lang) setLang(cfg.lang)
      }
      if (window.go.main.App.GetVersion) {
        version = await window.go.main.App.GetVersion()
      }
    } catch (e) {}
  })
</script>

<div class="settings-view">
  <div class="section-head">
    <div>
      <span class="label-track">{$t.settings}</span>
      <h2>{$t.settings_title}</h2>
    </div>
    <span class="ver">v{version}</span>
  </div>
  <div class="settings-content">
  {#if needsReconnect}
    <p class="reconnect-banner" role="status">{$t.notif_reconnect_required}</p>
  {/if}

  <article class="card">
    <div class="card-head">
      <button type="button" class="card-disclosure" aria-expanded={open.connection} on:click={() => toggle('connection')}>
        <span class="collapse-chevron" class:collapsed={!open.connection} aria-hidden="true">▼</span>
        <span class="card-title">{$t.section_connection}</span>
      </button>
    </div>
    <div class="card-body" class:collapsed={!open.connection}>
      <div class="card-body-inner">
        <div class="card-body-pad">
          <div class="field">
            <label for="cfg-server">{$t.server_addr}</label>
            <input id="cfg-server" type="text" bind:value={cfg.serverAddr} placeholder="vps.example.com:8080" />
          </div>
          <div class="field">
            <label for="cfg-nick">{$t.nickname}</label>
            <input id="cfg-nick" type="text" bind:value={cfg.nickname} placeholder="Your nickname" />
          </div>
          <div class="field">
            <label for="cfg-token">{$t.server_token}</label>
            <input id="cfg-token" type="password" bind:value={cfg.serverToken} placeholder={$t.server_token_desc} />
          </div>
          <p class="hint reconnect-hint">{$t.reconnect_apply}</p>
          <label class="toggle-row">
            <input type="checkbox" bind:checked={cfg.autoConnect} />
            <span>
              <span class="toggle-title">{$t.auto_connect}</span>
              <span class="toggle-label">{$t.auto_connect_desc}</span>
            </span>
          </label>
          <label class="toggle-row">
            <input type="checkbox" bind:checked={cfg.autoJoinLastRoom} />
            <span>
              <span class="toggle-title">{$t.auto_join}</span>
              <span class="toggle-label">{$t.auto_join_desc}</span>
            </span>
          </label>
        </div>
      </div>
    </div>
  </article>

  <article class="card">
    <div class="card-head">
      <button type="button" class="card-disclosure" aria-expanded={open.network} on:click={() => toggle('network')}>
        <span class="collapse-chevron" class:collapsed={!open.network} aria-hidden="true">▼</span>
        <span class="card-title">{$t.section_network}</span>
      </button>
    </div>
    <div class="card-body" class:collapsed={!open.network}>
      <div class="card-body-inner">
        <div class="card-body-pad">
          <p class="hint">{$t.advanced_warn}</p>
          <div class="field">
            <label for="cfg-conn">{$t.connection_mode}</label>
            <select id="cfg-conn" bind:value={cfg.connectionMode}>
              <option value="direct">{$t.connection_direct}</option>
              <option value="relay">{$t.connection_relay}</option>
            </select>
          </div>
          <p class="hint">{$t.connection_mode_desc}</p>
          <div class="field">
            <label for="cfg-mtu">{$t.mtu}</label>
            <input id="cfg-mtu" type="number" bind:value={cfg.mtu} min="576" max="9000" placeholder="1500" />
          </div>
          <p class="hint">{$t.mtu_desc}</p>
          <div class="field">
            <label for="cfg-dns">{$t.dns_server}</label>
            <input id="cfg-dns" type="text" bind:value={cfg.dnsServer} placeholder="system default" />
          </div>
          <p class="hint">{$t.dns_desc}</p>
          <div class="field">
            <label for="cfg-socks">{$t.socks5}</label>
            <input id="cfg-socks" type="text" bind:value={cfg.socks5Proxy} placeholder={$t.socks5_placeholder} />
          </div>
          <p class="hint reconnect-hint">{$t.socks5_desc}</p>
          <div class="field">
            <label for="cfg-stun">{$t.stun_server}</label>
            <input id="cfg-stun" type="text" bind:value={cfg.stunServer} placeholder="stun.l.google.com:19302" />
          </div>
          <p class="hint reconnect-hint">{$t.stun_desc}</p>
        </div>
      </div>
    </div>
  </article>

  <article class="card">
    <div class="card-head">
      <button type="button" class="card-disclosure" aria-expanded={open.system} on:click={() => toggle('system')}>
        <span class="collapse-chevron" class:collapsed={!open.system} aria-hidden="true">▼</span>
        <span class="card-title">{$t.section_system}</span>
      </button>
    </div>
    <div class="card-body" class:collapsed={!open.system}>
      <div class="card-body-inner">
        <div class="card-body-pad">
          <label class="toggle-row">
            <input type="checkbox" bind:checked={cfg.startWithWindows} />
            <span class="toggle-title">{$t.start_windows}</span>
          </label>
          <div class="update-row">
            <span class="toggle-title">{$t.updates}</span>
            <button class="btn btn-ghost" type="button" disabled={updateBusy} on:click={checkForUpdate}>
              {$t.check_updates}
            </button>
            {#if updateStatus === 'available'}
              <button class="btn btn-cta" type="button" disabled={updateBusy} on:click={applyUpdate}>
                {$t.apply_update}
              </button>
            {/if}
          </div>
          {#if updateStatus === 'checking'}
            <p class="hint">{$t.update_checking}</p>
          {:else if updateStatus === 'latest'}
            <p class="hint">{$t.update_latest}</p>
          {:else if updateStatus === 'available'}
            <p class="hint">{fmt($t.update_available, { v: updateLatest })}</p>
          {:else if updateStatus === 'downloading'}
            <p class="hint">{$t.update_downloading}</p>
          {:else if updateStatus === 'error' && updateError}
            <p class="hint reconnect-hint">{$t.update_failed}: {updateError}</p>
          {/if}
        </div>
      </div>
    </div>
  </article>

  <article class="card">
    <div class="card-head">
      <button type="button" class="card-disclosure" aria-expanded={open.appearance} on:click={() => toggle('appearance')}>
        <span class="collapse-chevron" class:collapsed={!open.appearance} aria-hidden="true">▼</span>
        <span class="card-title">{$t.section_appearance}</span>
      </button>
    </div>
    <div class="card-body" class:collapsed={!open.appearance}>
      <div class="card-body-inner">
        <div class="card-body-pad">
          <div class="field">
            <label for="cfg-scale">{$t.ui_scale}</label>
            <div class="scale-row">
              <input
                id="cfg-scale"
                type="range"
                min="75"
                max="150"
                step="5"
                value={cfg.uiScale ?? 100}
                on:input={onScaleInput}
              />
              <span class="scale-value">{cfg.uiScale ?? 100}%</span>
            </div>
          </div>
          <p class="hint">{$t.ui_scale_desc}</p>
          <div class="theme-row">
            <span class="toggle-title">{$t.theme}</span>
            <ThemeToggle />
          </div>
          <div class="field">
            <label for="cfg-lang">{$t.language}</label>
            <select id="cfg-lang" value={cfg.lang || $currentLang} on:change={handleLangChange}>
              <option value="en">{$t.language_en}</option>
              <option value="ru">{$t.language_ru}</option>
              <option value="zh">{$t.language_zh}</option>
            </select>
          </div>
        </div>
      </div>
    </div>
  </article>
  </div>

  <div class="commit-plate">
    <button class="btn btn-ghost" on:click={resetDefaults}>{$t.reset}</button>
    <button class="btn btn-cta" on:click={save}>{$t.save}</button>
  </div>
</div>

<ConfirmDialog
  open={!!pending}
  message={pending?.message || ''}
  confirmLabel={pending?.confirmLabel || $t.save}
  danger={!!pending?.danger}
  onConfirm={async () => {
    const run = pending?.run
    pending = null
    if (run) await run()
  }}
  onCancel={() => (pending = null)}
/>

<style>
  .settings-view {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: 12px 16px 0;
    min-height: 0;
  }
  .section-head {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 12px;
    flex-shrink: 0;
  }
  .section-head h2 {
    font-size: 1.05rem;
    font-weight: 600;
    letter-spacing: -0.03em;
    color: var(--text);
    margin: 2px 0 0;
  }
  .settings-content {
    flex: 1;
    overflow-y: auto;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding-bottom: 12px;
  }
  .reconnect-banner {
    background: var(--surface-2);
    border: 1px solid var(--hairline);
    color: var(--text);
    padding: 10px 12px;
    border-radius: var(--radius);
    font-size: 13px;
  }
  .toggle-row {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    margin: 10px 0;
    cursor: pointer;
  }
  .toggle-title {
    display: block;
    font-size: 13px;
    font-weight: 600;
    color: var(--text);
  }
  .toggle-label {
    display: block;
    color: var(--muted);
    font-size: 12px;
    margin-top: 2px;
  }
  input[type="checkbox"] {
    width: 16px;
    height: 16px;
    margin-top: 2px;
    accent-color: var(--cta);
  }
  .scale-row {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .scale-row input[type="range"] {
    flex: 1;
    accent-color: var(--cta);
  }
  .scale-value {
    min-width: 44px;
    text-align: right;
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: 13px;
  }
  .theme-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    margin: 12px 0 16px;
  }
  .update-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin: 12px 0;
  }
  .ver {
    color: var(--muted);
    font-size: 12px;
    font-family: var(--font-mono);
  }
  .commit-plate {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding: 12px 0 16px;
    flex-shrink: 0;
    border-top: 1px solid var(--border);
    background: var(--bg);
  }
</style>
