<script>
  import { afterUpdate } from 'svelte'
  import {
    chatMessages, chatTarget, status, chatOpen, settings,
    addNotification, addChatMessage, updateMessageStatus, markThreadRead, activeThreadKey
  } from '../stores/app.js'
  import { t, fmt } from '../locales/index.js'

  let inputText = ''
  let chatContainer
  let textareaEl
  let isNearBottom = true

  $: {
    if ($chatOpen) {
      markThreadRead(activeThreadKey())
    }
  }

  afterUpdate(() => {
    if (chatContainer && isNearBottom) {
      chatContainer.scrollTop = chatContainer.scrollHeight
    }
  })

  function onScroll() {
    if (!chatContainer) return
    const diff = chatContainer.scrollHeight - chatContainer.scrollTop - chatContainer.clientHeight
    isNearBottom = diff < 60
  }

  async function sendMessage() {
    const text = inputText.trim()
    if (!text) return

    if (!$status.connected) {
      addNotification($t.error_not_connected, 'error')
      return
    }
    if (!$status.room) {
      addNotification($t.send_disabled_no_room, 'error')
      return
    }

    let payload = text
    const runes = [...text]
    if (runes.length > 1000) {
      payload = runes.slice(0, 1000).join('')
      addNotification($t.msg_truncated, 'info')
    }

    inputText = ''
    if (textareaEl) textareaEl.style.height = 'auto'

    const localId = 'local_' + Date.now() + '_' + Math.random().toString(36).slice(2, 7)
    addChatMessage(localId, $settings.nickname || $t.you_self, payload, true, 'pending')

    try {
      if ($chatTarget && $chatTarget.id) {
        await window.go.main.App.SendChat($chatTarget.id, payload)
      } else {
        await window.go.main.App.BroadcastChat(payload)
      }
      updateMessageStatus(localId, 'sent')
    } catch (e) {
      updateMessageStatus(localId, 'failed')
      addNotification(fmt($t.error_send, { err: e }), 'error')
    }
  }

  async function retryMessage(msg) {
    updateMessageStatus(msg.id, 'pending')
    try {
      if ($chatTarget && $chatTarget.id) {
        await window.go.main.App.SendChat($chatTarget.id, msg.message)
      } else {
        await window.go.main.App.BroadcastChat(msg.message)
      }
      updateMessageStatus(msg.id, 'sent')
    } catch (e) {
      updateMessageStatus(msg.id, 'failed')
      addNotification(fmt($t.error_send, { err: e }), 'error')
    }
  }

  function handleKeydown(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      sendMessage()
    }
  }

  function formatDateLabel(timestamp) {
    const d = new Date(timestamp)
    const now = new Date()
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
    const yesterday = new Date(today - 86400000)
    const msgDate = new Date(d.getFullYear(), d.getMonth(), d.getDate())
    if (msgDate.getTime() === today.getTime()) return $t.today
    if (msgDate.getTime() === yesterday.getTime()) return $t.yesterday
    return d.toLocaleDateString([], { weekday: 'long', month: 'short', day: 'numeric' })
  }

  function shouldShowDateSeparator(msg, i, msgs) {
    if (i === 0) return true
    const prev = msgs[i - 1]
    if (!prev) return true
    return new Date(msg.timestamp).toDateString() !== new Date(prev.timestamp).toDateString()
  }

  function onTextInput() {
    if (textareaEl) {
      textareaEl.style.height = 'auto'
      textareaEl.style.height = Math.min(textareaEl.scrollHeight, 120) + 'px'
    }
  }

  function closeChat() {
    chatOpen.set(false)
  }

  $: title = $chatTarget
    ? fmt($t.chat_with, { name: $chatTarget.nickname })
    : fmt($t.room_chat_title, { room: $status.room || $t.network })
  $: canSend = $status.connected && !!$status.room
</script>

<div class="chat-view">
  <div class="section-head">
    <div>
      <span class="label-track">{$t.chat}</span>
      <h2>{title}</h2>
    </div>
    <button type="button" class="bezel" on:click={closeChat}>{$t.close_chat}</button>
  </div>

  <div class="chat-messages" bind:this={chatContainer} on:scroll={onScroll}>
    {#if $chatMessages.length === 0}
      <div class="empty">
        <div class="empty-label">{$t.no_messages}</div>
        <p class="empty-desc">{$t.no_messages_desc}</p>
      </div>
    {/if}
    {#each $chatMessages as msg, i (msg.id)}
      {#if shouldShowDateSeparator(msg, i, $chatMessages)}
        <div class="date-sep">{formatDateLabel(msg.timestamp)}</div>
      {/if}
      {#if msg.system}
        <div class="system-line">{msg.message}</div>
      {:else}
        <div class="msg" class:self={msg.isSelf} class:failed={msg.status === 'failed'}>
          <div class="msg-meta">
            <span class="msg-time">{new Date(msg.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })}</span>
            <span class="msg-nick">{msg.isSelf ? ($settings.nickname || $t.you_self) : msg.nickname}</span>
            {#if msg.status === 'pending'}<span class="msg-status">…</span>{/if}
            {#if msg.status === 'failed'}
              <button class="retry-btn" on:click={() => retryMessage(msg)}>{$t.retry}</button>
            {/if}
          </div>
          <div class="msg-body">{msg.message}</div>
        </div>
      {/if}
    {/each}
  </div>

  <div class="composer">
    <textarea
      bind:this={textareaEl}
      bind:value={inputText}
      on:keydown={handleKeydown}
      on:input={onTextInput}
      aria-label={$t.type_message}
      placeholder={canSend ? $t.type_message : $t.send_disabled_no_room}
      rows="1"
      disabled={!canSend}
    ></textarea>
    <button class="btn btn-cta" on:click={sendMessage} disabled={!canSend || !inputText.trim()}>{$t.send}</button>
  </div>
</div>

<style>
  .chat-view {
    display: flex;
    flex-direction: column;
    height: 100%;
    flex: 1;
    padding: 12px 14px 12px;
    min-height: 0;
    background: var(--surface);
  }
  .section-head {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 8px;
    padding-bottom: 10px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }
  .section-head h2 {
    font-size: 1.05rem;
    font-weight: 600;
    letter-spacing: -0.03em;
    color: var(--text);
    margin: 2px 0 0;
  }
  .chat-messages {
    flex: 1;
    overflow-y: auto;
    min-height: 0;
    padding: 4px 0 12px;
    font-family: var(--font-mono);
    font-size: 12px;
  }
  .empty {
    padding: 12px 2px;
    color: var(--muted);
    font-family: var(--font-sans);
  }
  .empty-label { color: var(--text); margin-bottom: 4px; font-weight: 500; }
  .empty-desc {
    font-family: var(--font-serif);
    font-style: italic;
    font-size: 13px;
    margin: 0;
  }
  .date-sep {
    text-align: left;
    font-size: 10px;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--muted);
    margin: 14px 0 8px;
    font-family: var(--font-sans);
  }
  .system-line {
    font-size: 12px;
    color: var(--muted);
    margin: 6px 0;
  }
  .msg {
    margin-bottom: 8px;
  }
  .msg-meta {
    font-size: 11px;
    color: var(--muted);
    margin-bottom: 2px;
    display: flex;
    gap: 8px;
    align-items: baseline;
  }
  .msg.self .msg-nick { color: var(--text); }
  .msg-time { color: var(--text-dim); }
  .msg-nick { color: var(--text); font-weight: 500; }
  .msg-body {
    color: var(--text);
    white-space: pre-wrap;
    word-break: break-word;
    line-height: 1.45;
  }
  .msg.failed .msg-body { color: var(--fault); }
  .retry-btn {
    margin-left: auto;
    background: transparent;
    border: none;
    color: var(--fault);
    cursor: pointer;
    font-size: 11px;
    font-family: inherit;
  }
  .composer {
    display: flex;
    gap: 8px;
    padding: 10px 0 0;
    border-top: 1px solid var(--border);
    flex-shrink: 0;
    align-items: flex-end;
  }
  textarea {
    flex: 1;
    resize: none;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    color: var(--text);
    padding: 8px 10px;
    font-family: var(--font-sans);
    font-size: 14px;
    min-height: 40px;
    line-height: 1.4;
  }
  textarea:disabled { opacity: 0.5; }
</style>
