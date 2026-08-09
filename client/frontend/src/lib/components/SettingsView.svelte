<script>
  import { onMount } from 'svelte'
  import { settings, addNotification } from '../stores/app.js'
  import { t, setLang, currentLang, fmt } from '../locales/index.js'

  let cfg = { ...$settings }
  let version = '1.0.0'
  let needsReconnect = false
  let updateBusy = false
  let updateStatus = '' // '', 'checking', 'latest', 'available', 'downloading', 'error'
  let updateLatest = ''
  let updateError = ''

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
    if (!confirm($t.confirm_update)) return
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
    if (!confirm($t.confirm_reset)) return
    try {
      cfg = await window.go.main.App.ResetSettings()
      $settings = { ...cfg }
      needsReconnect = false
      if (cfg.lang) setLang(cfg.lang)
      addNotification($t.notif_settings_saved, 'info')
    } catch (e) {
      addNotification($t.error_save + e, 'error')
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

  function onThemeChange() {
    $settings = { ...$settings, theme: cfg.theme }
  }

  onMount(async () => {
    try {
      const saved = await window.go.main.App.GetSettings()
      if (saved) {
        cfg = { ...cfg, ...saved }
        $settings = cfg
        if (cfg.lang) setLang(cfg.lang)
      }
      if (window.go.main.App.GetVersion) {
        version = await window.go.main.App.GetVersion()
      }
    } catch (e) {}
  })
</script>

<div class="settings-view">
  <h2 class="settings-title">{$t.settings_title}</h2>
  <div class="settings-content">
  {#if needsReconnect}
    <p class="reconnect-banner" role="status">{$t.notif_reconnect_required}</p>
  {/if}
  <div class="section">
    <div class="section-title">─── {$t.section_connection} ───</div>

    <div class="setting-row">
      <label class="setting-label" for="cfg-server">{$t.server_addr}</label>
      <input id="cfg-server" type="text" bind:value={cfg.serverAddr} placeholder="vps.example.com:8080" />
    </div>

    <div class="setting-row">
      <label class="setting-label" for="cfg-nick">{$t.nickname}</label>
      <input id="cfg-nick" type="text" bind:value={cfg.nickname} placeholder="Your nickname" />
    </div>

    <div class="setting-row">
      <label class="setting-label" for="cfg-token">{$t.server_token}</label>
      <input id="cfg-token" type="password" bind:value={cfg.serverToken} placeholder={$t.server_token_desc} />
    </div>
    <p class="hint reconnect-hint">{$t.reconnect_apply}</p>

    <label class="setting-row toggle-row">
      <span class="setting-label">{$t.auto_connect}</span>
      <input type="checkbox" bind:checked={cfg.autoConnect} />
      <span class="toggle-label">{$t.auto_connect_desc}</span>
    </label>

    <label class="setting-row toggle-row">
      <span class="setting-label">{$t.auto_join}</span>
      <input type="checkbox" bind:checked={cfg.autoJoinLastRoom} />
      <span class="toggle-label">{$t.auto_join_desc}</span>
    </label>
  </div>

  <div class="section">
    <div class="section-title">─── {$t.section_network} ───</div>
    <p class="hint">{$t.advanced_warn}</p>

    <label class="setting-row toggle-row">
      <span class="setting-label">{$t.p2p_only}</span>
      <input type="checkbox" bind:checked={cfg.p2pOnly} />
      <span class="toggle-label">{$t.p2p_only_desc}</span>
    </label>

    <div class="setting-row">
      <label class="setting-label" for="cfg-mtu">{$t.mtu}</label>
      <input id="cfg-mtu" type="number" bind:value={cfg.mtu} min="576" max="9000" placeholder="1500" />
    </div>
    <p class="hint">{$t.mtu_desc}</p>

    <div class="setting-row">
      <label class="setting-label" for="cfg-dns">{$t.dns_server}</label>
      <input id="cfg-dns" type="text" bind:value={cfg.dnsServer} placeholder="system default" />
    </div>
    <p class="hint">{$t.dns_desc}</p>

    <div class="setting-row">
      <label class="setting-label" for="cfg-socks">{$t.socks5}</label>
      <input id="cfg-socks" type="text" bind:value={cfg.socks5Proxy} placeholder={$t.socks5_placeholder} />
    </div>
    <p class="hint reconnect-hint">{$t.socks5_desc}</p>

    <div class="setting-row">
      <label class="setting-label" for="cfg-stun">{$t.stun_server}</label>
      <input id="cfg-stun" type="text" bind:value={cfg.stunServer} placeholder="stun.l.google.com:19302" />
    </div>
    <p class="hint reconnect-hint">{$t.stun_desc}</p>
  </div>

  <div class="section">
    <div class="section-title">─── {$t.section_system} ───</div>

    <label class="setting-row toggle-row">
      <span class="setting-label">{$t.start_windows}</span>
      <input type="checkbox" bind:checked={cfg.startWithWindows} />
    </label>

    <div class="setting-row update-row">
      <span class="setting-label">{$t.updates}</span>
      <button class="btn btn-check" type="button" disabled={updateBusy} on:click={checkForUpdate}>
        {$t.check_updates}
      </button>
      {#if updateStatus === 'available'}
        <button class="btn btn-update" type="button" disabled={updateBusy} on:click={applyUpdate}>
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

  <div class="section">
    <div class="section-title">─── {$t.section_appearance} ───</div>

    <div class="setting-row scale-row">
      <label class="setting-label" for="cfg-scale">{$t.ui_scale}</label>
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
    <p class="hint">{$t.ui_scale_desc}</p>

    <div class="setting-row radio-row">
      <span class="setting-label">{$t.theme}</span>
      <div class="radio-group" role="radiogroup" aria-label={$t.theme}>
        <label class="radio-option">
          <input type="radio" bind:group={cfg.theme} value="dark" on:change={onThemeChange} />
          {$t.theme_dark}
        </label>
        <label class="radio-option">
          <input type="radio" bind:group={cfg.theme} value="light" on:change={onThemeChange} />
          {$t.theme_light}
        </label>
      </div>
    </div>

    <div class="setting-row">
      <label class="setting-label" for="cfg-lang">{$t.language}</label>
      <select id="cfg-lang" value={cfg.lang || $currentLang} on:change={handleLangChange}>
        <option value="en">{$t.language_en}</option>
        <option value="ru">{$t.language_ru}</option>
        <option value="zh">{$t.language_zh}</option>
      </select>
    </div>
  </div>

  <div class="section actions-section">
    <button class="btn btn-save" on:click={save}>{$t.save}</button>
    <button class="btn btn-reset" on:click={resetDefaults}>{$t.reset}</button>
    <div class="ver">v{version}</div>
  </div>
</div>
</div>

<style>
  .settings-view {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: 24px 32px;
  }
  .settings-content {
    flex: 1;
    overflow-y: auto;
    min-height: 0;
    height: 0;
  }
  .settings-title {
    font-size: var(--font-size);
    color: var(--text-bright);
    text-transform: uppercase;
    letter-spacing: 1px;
    font-weight: 500;
    margin-bottom: 20px;
  }
  .reconnect-banner {
    background: var(--bg-raised);
    border: 1px solid var(--border-hover);
    color: var(--text-bright);
    padding: 10px 12px;
    margin-bottom: 16px;
    font-size: var(--font-size-sm);
  }
  .section { margin-bottom: 24px; }
  .section-title {
    font-size: var(--font-size-xs);
    color: var(--text-muted);
    letter-spacing: 1px;
  }
  .hint {
    font-size: var(--font-size-xs);
    color: var(--text-muted);
    margin: 6px 0 12px 0;
    line-height: 1.4;
  }
  .reconnect-hint { color: var(--text-secondary); }
  .setting-row {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 10px;
    min-height: 32px;
  }
  .setting-label {
    min-width: 140px;
    color: var(--text-secondary);
    font-size: var(--font-size-sm);
  }
  .toggle-label {
    color: var(--text-muted);
    font-size: var(--font-size-xs);
  }
  input[type="text"], input[type="password"], input[type="number"], select {
    flex: 1;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    color: var(--text-primary);
    padding: 8px 10px;
    min-height: 36px;
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
  }
  input[type="checkbox"] {
    width: 18px;
    height: 18px;
  }
  .radio-group { display: flex; gap: 12px; }
  .radio-option {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--text-secondary);
    font-size: var(--font-size-sm);
    min-height: 32px;
  }
  .scale-row input[type="range"] {
    flex: 1;
    min-width: 120px;
    accent-color: var(--accent);
  }
  .scale-value {
    min-width: 44px;
    text-align: right;
    color: var(--text-secondary);
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
  }
  .actions-section { display: flex; gap: 8px; align-items: center; }
  .actions-section .btn { min-height: 36px; padding: 8px 14px; }
  .update-row { flex-wrap: wrap; }
  .update-row .btn { min-height: 36px; padding: 8px 14px; }
  .btn-check, .btn-update {
    background: var(--bg-raised);
    border: 1px solid var(--border);
    color: var(--text-primary);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    cursor: pointer;
  }
  .btn-check:hover:not(:disabled), .btn-update:hover:not(:disabled) {
    border-color: var(--border-hover);
    color: var(--text-bright);
  }
  .btn-check:disabled, .btn-update:disabled { opacity: 0.5; cursor: default; }
  .ver { margin-left: auto; color: var(--text-dim); font-size: var(--font-size-xs); }
</style>
