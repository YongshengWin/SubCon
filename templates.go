package main

import "html/template"

var indexTemplate = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>订阅转换</title>
  <style>
    :root {
      --bg: #f5f5f0;
      --panel: #ffffff;
      --text: #1a1d1b;
      --muted: #65706a;
      --border: #e3e7e3;
      --accent: #0e6b4d;
      --danger: #b54545;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      background: var(--bg);
      color: var(--text);
      font-family: "PingFang SC", "Microsoft YaHei", sans-serif;
    }
    .shell { max-width: 760px; margin: 0 auto; padding: 40px 16px; }
    .panel {
      background: var(--panel);
      border: 1px solid var(--border);
      border-radius: 20px;
      padding: 24px;
    }
    h1 { margin: 0 0 10px; font-size: 30px; }
    p { margin: 0 0 20px; color: var(--muted); line-height: 1.7; }
    .field { margin-bottom: 16px; }
    label {
      display: block;
      margin-bottom: 8px;
      font-size: 14px;
      font-weight: 600;
    }
    input[type="text"], textarea, select {
      width: 100%;
      border: 1px solid rgba(43, 62, 52, 0.16);
      border-radius: 14px;
      background: #fff;
      color: var(--text);
      font: inherit;
      padding: 14px 16px;
      outline: none;
    }
    textarea {
      min-height: 140px;
      resize: vertical;
    }
    .actions {
      display: flex;
      gap: 12px;
      flex-wrap: wrap;
      margin-top: 4px;
    }
    button {
      border: 0;
      border-radius: 999px;
      padding: 13px 18px;
      font: inherit;
      cursor: pointer;
    }
    button.primary {
      background: var(--accent);
      color: #fff;
    }
    button.secondary {
      background: #fff;
      color: var(--text);
      border: 1px solid var(--border);
    }
    .result-box { margin-top: 24px; display: flex; flex-direction: column; gap: 14px; }
    .copy-row {
      display: flex;
      gap: 10px;
      align-items: stretch;
    }
    .copy-row input { flex: 1; }
    .pill-row {
      display: flex;
      gap: 10px;
      flex-wrap: wrap;
      min-height: 34px;
    }
    .pill {
      display: inline-flex;
      align-items: center;
      padding: 8px 12px;
      border-radius: 999px;
      font-size: 13px;
      background: rgba(14,107,77,0.10);
      color: var(--accent);
    }
    .pill.warn {
      background: rgba(181,69,69,0.10);
      color: var(--danger);
    }
    .status { font-size: 14px; min-height: 22px; }
    .status.error { color: var(--danger); }
    .status.ok { color: var(--accent); }
    pre {
      margin: 0;
      padding: 18px;
      border-radius: 14px;
      background: #19221d;
      color: #ecf6ef;
      min-height: 180px;
      max-height: 420px;
      overflow: auto;
      white-space: pre-wrap;
      word-break: break-word;
      line-height: 1.65;
      font-size: 13px;
    }
    .version-tag {
      margin-top: 2rem;
      font-size: 0.75rem;
      color: var(--text-dim);
      text-align: center;
      opacity: 0.5;
    }
    @media (max-width: 640px) {
      .shell { padding: 20px 14px 32px; }
      .panel { padding: 18px; }
      .copy-row { display: grid; grid-template-columns: 1fr; }
      button { width: 100%; }
      .actions { display: grid; grid-template-columns: 1fr; }
    }
  </style>
</head>
<body>
  <div class="shell">
    <div style="background:#0e6b4d;color:#fff;text-align:center;padding:8px;border-radius:10px;margin-bottom:15px;font-weight:bold;">✨ v0.3.9 核心已同步，短链功能就绪</div>
    <div class="panel">
      <h1>订阅转换 [终极补丁版]</h1>
      <p>输入原始订阅 URL，选择目标客户端，直接生成转换链接。</p>

      <div class="field">
        <label for="sub-url">原始订阅 URL</label>
        <textarea id="sub-url" placeholder="http://your-server:2096/sub/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"></textarea>
      </div>

      <div class="field">
        <label for="target">目标客户端</label>
        <select id="target">
          <option value="surge" selected>Surge</option>
          <option value="clash">Clash</option>
          <option value="stash">Stash</option>
          <option value="quantumultx">Quantumult X</option>
        </select>
      </div>

      <div class="actions">
        <button class="primary" id="preview-btn">生成链接</button>
        <button class="secondary" id="copy-link-btn" type="button">复制链接</button>
        <button class="secondary" id="open-link-btn" type="button">打开结果</button>
      </div>

      <div class="result-box">
        <div class="field">
          <label for="convert-link">转换链接</label>
          <div class="copy-row">
            <input id="convert-link" type="text" readonly placeholder="点击生成后出现">
            <button class="secondary" id="copy-config-btn" type="button">复制结果</button>
          </div>
        </div>

        <div class="pill-row" id="pill-row"></div>
        <div class="status" id="status"></div>
        <pre id="output">等待生成...</pre>
      </div>

      <div class="version-tag">v0.3.8 ULTIMATE</div>
    </div>
  </div>

  <script>
    const el = {
      subURL: document.getElementById('sub-url'),
      target: document.getElementById('target'),
      convertLink: document.getElementById('convert-link'),
      output: document.getElementById('output'),
      status: document.getElementById('status'),
      pillRow: document.getElementById('pill-row'),
      previewBtn: document.getElementById('preview-btn'),
      copyLinkBtn: document.getElementById('copy-link-btn'),
      openLinkBtn: document.getElementById('open-link-btn'),
      copyConfigBtn: document.getElementById('copy-config-btn')
    };

    function buildConvertURL(apiMode) {
      const url = new URL(apiMode ? '/api/convert' : '/convert', window.location.origin);
      url.searchParams.set('url', el.subURL.value.trim());
      url.searchParams.set('target', el.target.value || '{{.DefaultTarget}}');
      return url.toString();
    }

    function setStatus(message, type) {
      el.status.textContent = message || '';
      el.status.className = 'status' + (type ? ' ' + type : '');
    }

    function renderPills(data) {
      el.pillRow.innerHTML = '';
      const items = [{ text: '节点数 ' + (data.nodeCount ?? 0), warn: false }];
      if (data.target) items.push({ text: '目标 ' + data.target, warn: false });
      if (Array.isArray(data.ignoredTypes) && data.ignoredTypes.length > 0) {
        items.push({ text: '已忽略 ' + data.ignoredTypes.join(', '), warn: true });
      }
      items.forEach(item => {
        const span = document.createElement('span');
        span.className = 'pill' + (item.warn ? ' warn' : '');
        span.textContent = item.text;
        el.pillRow.appendChild(span);
      });
    }

    async function preview() {
      const source = el.subURL.value.trim();
      if (!source) {
        setStatus('先填原始订阅 URL。', 'error');
        return;
      }
      const apiURL = buildConvertURL(true);
      const directURL = buildConvertURL(false);
      el.convertLink.value = directURL;
      setStatus('正在转换...', '');
      el.output.textContent = '处理中...';
      try {
        const resp = await fetch(apiURL);
        const data = await resp.json();
        if (!resp.ok || !data.success) throw new Error(data.error || '转换失败');
        renderPills(data);
        el.output.textContent = data.config || '';
        setStatus('转换完成。', 'ok');

        try {
          const targetKey = el.target.value || '{{.DefaultTarget}}';
          const shortResp = await fetch('/api/shorten', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ target: targetKey, url: source })
          });
          const shortData = await shortResp.json();
          if (shortData.success && shortData.shortUrl) {
            el.convertLink.value = window.location.origin + shortData.shortUrl;
          } else {
            setStatus('[API ERROR] ' + (shortData.error || '短链生成失败'), 'error');
          }
        } catch (e) {
          console.error('Failed to get short URL', e);
          setStatus('[FETCH ERROR] 无法连接后台短链 API', 'error');
        }
      } catch (err) {
        renderPills({ nodeCount: 0, ignoredTypes: [] });
        el.output.textContent = '转换失败';
        setStatus(err.message || '转换失败', 'error');
      }
    }

    async function copyText(text, okMessage) {
      if (!text) {
        setStatus('当前没有可复制的内容。', 'error');
        return;
      }
      try {
        await navigator.clipboard.writeText(text);
        setStatus(okMessage, 'ok');
      } catch (_) {
        setStatus('复制失败，请手动复制。', 'error');
      }
    }

    el.previewBtn.addEventListener('click', preview);
    el.copyLinkBtn.addEventListener('click', () => copyText(buildConvertURL(false), '转换链接已复制。'));
    el.copyConfigBtn.addEventListener('click', () => copyText(el.output.textContent, '转换结果已复制。'));
    el.openLinkBtn.addEventListener('click', () => {
      const source = el.subURL.value.trim();
      if (!source) {
        setStatus('先填原始订阅 URL。', 'error');
        return;
      }
      window.open(buildConvertURL(false), '_blank', 'noopener,noreferrer');
    });
  </script>
</body>
</html>`))
