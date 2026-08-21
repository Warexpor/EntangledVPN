<script>
  import { afterUpdate } from 'svelte'
  import {
    chatMessages, chatTarget, status, view, settings,
    addNotification, addChatMessage, updateMessageStatus, markThreadRead, activeThreadKey
  } from '../stores/app.js'
  import { t, fmt } from '../locales/index.js'

  let inputText = ''
  let chatContainer
  let textareaEl
  let isNearBottom = true

  $: {
    if ($view === 'chat') {
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
    chatTarget.set(null)
    view.set('network')
  }

  $: title = $chatTarget
    ? fmt($t.chat_with, { name: $chatTarget.nickname })
    : fmt($t.room_chat_title, { room: $status.room || $t.network })
  $: canSend = $status.connected && !!$status.room
</script>

<div class="chat-view">
  <div class="chat-header">
    <button class="back-btn" on:click={closeChat} title={$t.back} aria-label={$t.back}>
      <svg width="14" height="14" viewBox="0 0 16 16" aria-hidden="true">
        <path d="M10.5 3.5 L5.5 8 L10.5 12.5" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
    </button>
    <span class="chat-title">{title}</span>
  </div>

  <div class="chat-messages" bind:this={chatContainer} on:scroll={onScroll}>
    {#if $chatMessages.length === 0}
      <div class="empty-chat">
        <div class="empty-chat-label">{$t.no_messages}</div>
        <div class="empty-chat-desc">{$t.no_messages_desc}</div>
      </div>
    {/if}
    {#each $chatMessages as msg, i (msg.id)}
      {#if shouldShowDateSeparator(msg, i, $chatMessages)}
        <div class="date-sep">{formatDateLabel(msg.timestamp)}</div>
      {/if}
      {#if msg.system}
        <div class="system-line">{msg.message}</div>
      {:else}
        <div class="msg" class:self={msg.isSelf}>
          <div class="msg-meta">
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

  <div class="chat-input-area">
    <textarea
      bind:this={textareaEl}
      bind:value={inputText}
      on:keydown={handleKeydown}
      on:input={onTextInput}
      placeholder={canSend ? $t.type_message : $t.send_disabled_no_room}
      rows="1"
      disabled={!canSend}
    ></textarea>
    <button class="send-btn" on:click={sendMessage} disabled={!canSend || !inputText.trim()}>{$t.send}</button>
  </div>
</div>

<style>
  .chat-view {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  .chat-header {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border);
    background: var(--bg-surface);
  }
  .back-btn {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm, 4px);
    color: var(--text-secondary);
    cursor: pointer;
  }
  .chat-title {
    color: var(--text-bright);
    font-size: var(--font-size);
  }
  .chat-messages {
    flex: 1;
    overflow-y: auto;
    padding: 16px;
    min-height: 0;
  }
  .empty-chat {
    text-align: center;
    padding: 40px 16px;
    color: var(--text-muted);
  }
  .empty-chat-label { color: var(--text-secondary); margin: 8px 0 4px; }
  .empty-chat-desc { font-size: var(--font-size-xs); }
  .date-sep {
    text-align: center;
    font-size: var(--font-size-xs);
    color: var(--text-dim);
    margin: 12px 0;
  }
  .system-line {
    text-align: center;
    font-size: var(--font-size-xs);
    color: var(--text-muted);
    margin: 8px 0;
  }
  .msg {
    margin-bottom: 10px;
    max-width: 80%;
  }
  .msg.self { margin-left: auto; text-align: right; }
  .msg-meta {
    font-size: var(--font-size-xs);
    color: var(--text-muted);
    margin-bottom: 2px;
  }
  .msg-body {
    display: inline-block;
    padding: 8px 10px;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm, 4px);
    color: var(--text-primary);
    white-space: pre-wrap;
    word-break: break-word;
    text-align: left;
  }
  .msg.self .msg-body {
    background: var(--bg-active);
  }
  .retry-btn {
    margin-left: 6px;
    background: transparent;
    border: none;
    color: var(--error);
    cursor: pointer;
    font-size: var(--font-size-xs);
  }
  .chat-input-area {
    display: flex;
    gap: 8px;
    padding: 12px 16px;
    border-top: 1px solid var(--border);
    background: var(--bg-surface);
  }
  textarea {
    flex: 1;
    resize: none;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm, 4px);
    color: var(--text-primary);
    padding: 8px 10px;
    font-family: inherit;
    font-size: var(--font-size);
    min-height: 36px;
  }
  textarea:disabled { opacity: 0.5; }
  .send-btn {
    background: var(--accent);
    color: var(--accent-ink);
    border: none;
    border-radius: var(--radius-md, 6px);
    padding: 0 16px;
    cursor: pointer;
    font-family: inherit;
    text-transform: uppercase;
  }
  .send-btn:disabled { opacity: 0.4; cursor: not-allowed; }
</style>
