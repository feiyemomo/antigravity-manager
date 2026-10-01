export function getDashboardHtml(): string {
  return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Antigravity 账号管理面板 - agy-tools</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg: #090d16;
      --card-bg: rgba(18, 26, 44, 0.7);
      --card-border: rgba(255, 255, 255, 0.08);
      --card-hover: rgba(25, 36, 60, 0.9);
      --primary: #3b82f6;
      --primary-hover: #2563eb;
      --accent: #10b981;
      --accent-glow: rgba(16, 185, 129, 0.25);
      --danger: #ef4444;
      --danger-hover: #dc2626;
      --warning: #f59e0b;
      --text: #f3f4f6;
      --text-muted: #94a3b8;
      --radius: 14px;
    }

    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: 'Inter', system-ui, -apple-system, sans-serif;
      background: radial-gradient(circle at 15% 15%, #111d33 0%, var(--bg) 60%), #060911;
      color: var(--text);
      min-height: 100vh;
      display: flex;
      flex-direction: column;
    }

    header {
      border-bottom: 1px solid var(--card-border);
      background: rgba(10, 15, 29, 0.85);
      backdrop-filter: blur(16px);
      position: sticky;
      top: 0;
      z-index: 50;
      padding: 16px 32px;
      display: flex;
      justify-content: space-between;
      align-items: center;
    }

    .brand {
      display: flex;
      align-items: center;
      gap: 12px;
    }
    .logo-badge {
      width: 40px;
      height: 40px;
      background: linear-gradient(135deg, #3b82f6 0%, #10b981 100%);
      border-radius: 10px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-weight: 700;
      font-size: 20px;
      box-shadow: 0 4px 14px rgba(59, 130, 246, 0.4);
    }
    .brand h1 {
      font-size: 19px;
      font-weight: 700;
      letter-spacing: -0.5px;
      background: linear-gradient(to right, #ffffff, #93c5fd);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
    }
    .brand p {
      font-size: 12px;
      color: var(--text-muted);
    }

    .nav-actions {
      display: flex;
      gap: 12px;
      align-items: center;
    }

    .btn {
      padding: 9px 18px;
      border-radius: 8px;
      font-size: 13px;
      font-weight: 600;
      cursor: pointer;
      transition: all 0.2s ease;
      border: 1px solid transparent;
      display: inline-flex;
      align-items: center;
      gap: 8px;
    }
    .btn-primary {
      background: #2563eb;
      color: #fff;
      box-shadow: 0 4px 12px rgba(37, 99, 235, 0.3);
    }
    .btn-primary:hover {
      background: #1d4ed8;
      transform: translateY(-1px);
    }
    .btn-secondary {
      background: rgba(255, 255, 255, 0.05);
      border-color: var(--card-border);
      color: var(--text);
    }
    .btn-secondary:hover {
      background: rgba(255, 255, 255, 0.1);
      border-color: rgba(255, 255, 255, 0.2);
    }
    .btn-success {
      background: #059669;
      color: #fff;
    }
    .btn-success:hover {
      background: #047857;
    }
    .btn-danger {
      background: rgba(239, 68, 68, 0.15);
      color: #f87171;
      border-color: rgba(239, 68, 68, 0.3);
    }
    .btn-danger:hover {
      background: #dc2626;
      color: #fff;
    }

    main {
      flex: 1;
      max-width: 1280px;
      width: 100%;
      margin: 0 auto;
      padding: 32px 24px;
    }

    .top-stats {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
      gap: 20px;
      margin-bottom: 28px;
    }

    .stat-card {
      background: var(--card-bg);
      border: 1px solid var(--card-border);
      backdrop-filter: blur(12px);
      padding: 20px;
      border-radius: var(--radius);
      display: flex;
      flex-direction: column;
      gap: 8px;
      box-shadow: 0 8px 24px rgba(0,0,0,0.25);
    }
    .stat-card .label {
      font-size: 12px;
      color: var(--text-muted);
      text-transform: uppercase;
      font-weight: 600;
      letter-spacing: 0.5px;
    }
    .stat-card .value {
      font-size: 22px;
      font-weight: 700;
      display: flex;
      align-items: center;
      gap: 10px;
      word-break: break-all;
    }

    .section-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 16px;
    }
    .section-title {
      font-size: 18px;
      font-weight: 600;
      letter-spacing: -0.3px;
    }

    .accounts-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
      gap: 20px;
      margin-bottom: 36px;
    }

    .account-card {
      background: var(--card-bg);
      border: 1px solid var(--card-border);
      backdrop-filter: blur(12px);
      border-radius: var(--radius);
      padding: 24px;
      transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
      display: flex;
      flex-direction: column;
      position: relative;
      overflow: hidden;
    }
    .account-card:hover {
      background: var(--card-hover);
      border-color: rgba(255, 255, 255, 0.18);
      transform: translateY(-2px);
      box-shadow: 0 12px 30px rgba(0, 0, 0, 0.35);
    }
    .account-card.is-current {
      border: 2px solid #10b981;
      box-shadow: 0 0 24px var(--accent-glow);
    }
    .current-badge {
      position: absolute;
      top: 16px;
      right: 16px;
      background: #10b981;
      color: #042f2e;
      font-size: 11px;
      font-weight: 700;
      padding: 4px 10px;
      border-radius: 20px;
      letter-spacing: 0.5px;
      display: flex;
      align-items: center;
      gap: 4px;
    }

    .account-header {
      display: flex;
      align-items: flex-start;
      gap: 14px;
      margin-bottom: 16px;
    }
    .avatar {
      width: 44px;
      height: 44px;
      border-radius: 12px;
      background: linear-gradient(135deg, #1e293b, #334155);
      border: 1px solid rgba(255, 255, 255, 0.1);
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 18px;
      font-weight: 600;
      color: #93c5fd;
      flex-shrink: 0;
    }
    .account-info {
      flex: 1;
      overflow: hidden;
    }
    .account-email {
      font-size: 15px;
      font-weight: 600;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      margin-bottom: 4px;
    }
    .account-meta {
      display: flex;
      gap: 8px;
      align-items: center;
      flex-wrap: wrap;
    }
    .tag {
      font-size: 11px;
      padding: 2px 8px;
      border-radius: 6px;
      font-weight: 500;
    }
    .tag-tier {
      background: rgba(59, 130, 246, 0.15);
      color: #60a5fa;
      border: 1px solid rgba(59, 130, 246, 0.3);
    }
    .tag-active {
      background: rgba(16, 185, 129, 0.15);
      color: #34d399;
      border: 1px solid rgba(16, 185, 129, 0.3);
    }
    .tag-disabled {
      background: rgba(239, 68, 68, 0.15);
      color: #f87171;
      border: 1px solid rgba(239, 68, 68, 0.3);
    }
    .tag-limited {
      background: rgba(245, 158, 11, 0.15);
      color: #fbbf24;
      border: 1px solid rgba(245, 158, 11, 0.3);
    }

    .quota-box {
      margin: 14px 0;
      background: rgba(15, 23, 42, 0.65);
      border-radius: 12px;
      padding: 14px;
      font-size: 13px;
      border: 1px solid rgba(255, 255, 255, 0.08);
      display: flex;
      flex-direction: column;
      gap: 12px;
    }
    .quota-group {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }
    .quota-group-header {
      font-size: 13px;
      font-weight: 700;
      color: #f8fafc;
      letter-spacing: -0.2px;
    }
    .quota-bucket-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 3px 0;
    }
    .quota-bucket-info {
      display: flex;
      flex-direction: column;
      gap: 2px;
    }
    .quota-bucket-title {
      font-size: 12.5px;
      color: #cbd5e1;
      font-weight: 500;
    }
    .quota-bucket-subtext {
      font-size: 11px;
      color: #94a3b8;
    }
    .quota-divider {
      height: 1px;
      background: rgba(255, 255, 255, 0.08);
      margin: 2px 0;
    }
    .quota-circle-wrap {
      width: 42px;
      height: 42px;
      flex-shrink: 0;
      display: flex;
      align-items: center;
      justify-content: center;
    }
    .quota-circle-svg {
      transform: rotate(-90deg);
    }

    .account-actions {
      display: flex;
      gap: 8px;
      margin-top: auto;
      padding-top: 14px;
      border-top: 1px solid var(--card-border);
    }
    .account-actions .btn {
      flex: 1;
      justify-content: center;
      padding: 8px 12px;
      font-size: 12px;
    }

    .settings-panel {
      background: var(--card-bg);
      border: 1px solid var(--card-border);
      border-radius: var(--radius);
      padding: 24px;
    }
    .settings-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
      gap: 20px;
    }
    .setting-item {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }
    .setting-item label {
      font-size: 13px;
      font-weight: 600;
    }
    .setting-item input[type="text"],
    .setting-item input[type="number"],
    .setting-item select {
      background: rgba(0, 0, 0, 0.35);
      border: 1px solid var(--card-border);
      color: var(--text);
      padding: 10px 14px;
      border-radius: 8px;
      font-size: 13px;
      outline: none;
    }
    .setting-item input:focus, .setting-item select:focus {
      border-color: var(--primary);
    }
    .switch-label {
      display: flex;
      align-items: center;
      gap: 10px;
      cursor: pointer;
      user-select: none;
      font-size: 14px;
    }
    .switch-label input {
      width: 18px;
      height: 18px;
      accent-color: var(--primary);
      cursor: pointer;
    }

    .toast {
      position: fixed;
      bottom: 24px;
      right: 24px;
      padding: 14px 22px;
      background: #1e293b;
      border: 1px solid rgba(255, 255, 255, 0.15);
      color: #fff;
      border-radius: 10px;
      box-shadow: 0 10px 30px rgba(0,0,0,0.5);
      font-size: 13px;
      font-weight: 500;
      opacity: 0;
      pointer-events: none;
      transform: translateY(10px);
      transition: all 0.3s ease;
      z-index: 100;
    }
    .toast.show {
      opacity: 1;
      pointer-events: auto;
      transform: translateY(0);
    }

    .modal-overlay {
      position: fixed;
      top: 0; left: 0; right: 0; bottom: 0;
      background: rgba(0, 0, 0, 0.7);
      backdrop-filter: blur(8px);
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: 1000;
      opacity: 0;
      pointer-events: none;
      transition: opacity 0.2s ease;
    }
    .modal-overlay.active {
      opacity: 1;
      pointer-events: auto;
    }
    .modal {
      background: #111827;
      border: 1px solid var(--card-border);
      border-radius: 16px;
      width: 90%;
      max-width: 480px;
      padding: 28px;
      box-shadow: 0 20px 50px rgba(0,0,0,0.6);
      text-align: center;
    }
    .modal h3 {
      font-size: 18px;
      margin-bottom: 12px;
    }
    .modal p {
      font-size: 13px;
      color: var(--text-muted);
      margin-bottom: 20px;
      line-height: 1.5;
    }
    .spinner {
      width: 40px;
      height: 40px;
      border: 4px solid rgba(255,255,255,0.1);
      border-top-color: var(--primary);
      border-radius: 50%;
      animation: spin 0.8s linear infinite;
      margin: 20px auto;
    }
    @keyframes spin { to { transform: rotate(360deg); } }
  </style>
</head>
<body>

  <header>
    <div class="brand">
      <div class="logo-badge">⚡</div>
      <div>
        <h1>Antigravity 控制台</h1>
        <p>agy-tools 多账号调度与一键切换系统</p>
      </div>
    </div>
    <div class="nav-actions">
      <button class="btn btn-primary" onclick="startLogin()">➕ 添加 Google 账号</button>
      <button class="btn btn-secondary" onclick="rotateCurrent()">🔄 轮换 Antigravity</button>
      <button class="btn btn-secondary" onclick="refreshQuotas()">📊 刷新配额</button>
    </div>
  </header>

  <main>
    <div class="top-stats">
      <div class="stat-card">
        <span class="label">当前 Antigravity IDE 活跃账号</span>
        <div class="value" id="currentAccountEmail" style="color: #34d399;">检测中...</div>
      </div>
      <div class="stat-card">
        <span class="label">可用账号池总数</span>
        <div class="value" id="totalAccounts">0</div>
      </div>
      <div class="stat-card">
        <span class="label">自动轮换保护</span>
        <div class="value" id="autoRotateStatus" style="font-size: 16px;">未启用</div>
      </div>
    </div>

    <div class="section-header">
      <h2 class="section-title">Google 账号管理池</h2>
    </div>

    <div class="accounts-grid" id="accountsGrid">
      <!-- 动态渲染账号卡片 -->
    </div>

    <div class="section-header">
      <h2 class="section-title">Antigravity 自动轮换与代理设置</h2>
    </div>

    <div class="settings-panel">
      <div class="settings-grid">
        <div class="setting-item">
          <label class="switch-label">
            <input type="checkbox" id="autoRotateAntigravity">
            <span>启用 Antigravity IDE 自动轮换 (遇限流或定时)</span>
          </label>
          <span style="font-size: 12px; color: var(--text-muted);">
            当请求遇到 429 配额用尽时，或到达定时周期时，自动将 Antigravity IDE 切换至下一个健康账号。
          </span>
        </div>
        <div class="setting-item">
          <label>定时轮换周期 (分钟)</label>
          <input type="number" id="autoRotateIntervalMinutes" min="5" max="1440" value="60">
        </div>
        <div class="setting-item">
          <label class="switch-label">
            <input type="checkbox" id="switchPreviewModel">
            <span>模型自动降级 (Pro 配额用尽转 Preview)</span>
          </label>
        </div>
      </div>
      <div style="margin-top: 20px; display: flex; justify-content: flex-end;">
        <button class="btn btn-primary" onclick="saveSettings()">保存配置</button>
      </div>
    </div>
  </main>

  <div class="modal-overlay" id="loginModal">
    <div class="modal">
      <h3>Google 账户授权</h3>
      <p id="loginPrompt">正在生成授权地址并唤起浏览器...</p>
      <div class="spinner" id="loginSpinner"></div>
      <div id="loginLinkContainer" style="display:none; margin: 15px 0;">
        <a id="loginAuthLink" href="#" target="_blank" class="btn btn-primary">手动前往 Google 授权</a>
      </div>
      <button class="btn btn-secondary" style="width: 100%; margin-top: 10px;" onclick="closeModal()">取消</button>
    </div>
  </div>

  <div class="toast" id="toast"></div>

  <script>
    let pollInterval = null;

    function showToast(msg, isError = false) {
      const toast = document.getElementById('toast');
      toast.innerText = msg;
      toast.style.borderColor = isError ? '#ef4444' : '#10b981';
      toast.classList.add('show');
      setTimeout(() => toast.classList.remove('show'), 3500);
    }

    async function loadData() {
      try {
        const [accRes, cfgRes] = await Promise.all([
          fetch('/api/accounts'),
          fetch('/api/config')
        ]);
        const accData = await accRes.json();
        const cfgData = await cfgRes.json();

        renderAccounts(accData);
        renderConfig(cfgData);
      } catch (err) {
        showToast('获取数据失败: ' + err.message, true);
      }
    }

    function escapeHtml(str) {
      if (!str) return '';
      return String(str).replace(/[&<>"']/g, function(m) {
        return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[m];
      });
    }

    function formatCountdown(resetTimeStr) {
      if (!resetTimeStr) return '';
      var target = new Date(resetTimeStr).getTime();
      if (isNaN(target)) return '';
      var diffMs = target - Date.now();
      if (diffMs <= 0) return 'Ready to reset';
      var totalMins = Math.floor(diffMs / 60000);
      var days = Math.floor(totalMins / 1440);
      var hours = Math.floor((totalMins % 1440) / 60);
      var mins = totalMins % 60;
      if (days > 0) return hours > 0 ? ('Resets in ' + days + 'd ' + hours + 'h') : ('Resets in ' + days + 'd');
      if (hours > 0) return mins > 0 ? ('Resets in ' + hours + 'h ' + mins + 'm') : ('Resets in ' + hours + 'h');
      return mins > 0 ? ('Resets in ' + mins + 'm') : 'Resets in <1m';
    }

    function renderQuotaCircle(pct) {
      var percentage = Math.max(0, Math.min(100, Math.round(pct)));
      var r = 16;
      var c = 2 * Math.PI * r;
      var offset = c * (1 - percentage / 100);
      var strokeColor = '#38bdf8';
      if (percentage >= 60) strokeColor = '#34d399';
      else if (percentage <= 25) strokeColor = '#f87171';
      else strokeColor = '#fbbf24';

      return '<div class="quota-circle-wrap" title="' + percentage + '%">' +
        '<svg width="42" height="42" viewBox="0 0 42 42">' +
          '<circle cx="21" cy="21" r="' + r + '" fill="none" stroke="rgba(255, 255, 255, 0.12)" stroke-width="3.5" />' +
          '<circle cx="21" cy="21" r="' + r + '" fill="none" stroke="' + strokeColor + '" stroke-width="3.5" ' +
            'stroke-dasharray="' + c.toFixed(2) + '" ' +
            'stroke-dashoffset="' + offset.toFixed(2) + '" ' +
            'stroke-linecap="round" class="quota-circle-svg" ' +
            'style="transform-origin: 21px 21px; transition: stroke-dashoffset 0.5s ease;" />' +
          '<text x="21" y="21.5" text-anchor="middle" dominant-baseline="central" fill="#f8fafc" font-size="11" font-weight="700">' + percentage + '%</text>' +
        '</svg>' +
      '</div>';
    }

    function renderAccounts(data) {
      const { accounts, currentAccountId } = data;
      document.getElementById('totalAccounts').innerText = accounts.length;

      const current = accounts.find(a => a.id === currentAccountId);
      document.getElementById('currentAccountEmail').innerText = current ? current.email : '未设置/外部独立账号';

      const grid = document.getElementById('accountsGrid');
      grid.innerHTML = '';

      if (!accounts.length) {
        grid.innerHTML = '<div style="color: var(--text-muted); padding: 40px; grid-column: 1/-1; text-align: center;">暂无已配置账号，请点击右上角【添加 Google 账号】</div>';
        return;
      }

      accounts.forEach(acc => {
        const isCurrent = acc.id === currentAccountId;
        const initial = (acc.name || acc.email || 'U')[0].toUpperCase();

        let quotaSummary = '';
        if (acc.quota && acc.quota.groups && acc.quota.groups.length > 0) {
          const groupsHtml = acc.quota.groups.map(group => {
            const bucketsHtml = (group.buckets || []).map(b => {
              const subtext = b.subtext || (b.resetTime ? formatCountdown(b.resetTime) : '');
              return '<div class="quota-bucket-row">' +
                '<div class="quota-bucket-info">' +
                  '<span class="quota-bucket-title">' + escapeHtml(b.displayName) + '</span>' +
                  (subtext ? '<span class="quota-bucket-subtext">' + escapeHtml(subtext) + '</span>' : '') +
                '</div>' +
                renderQuotaCircle(b.percentage) +
              '</div>';
            }).join('');

            return '<div class="quota-group">' +
              '<div class="quota-group-header">' + escapeHtml(group.displayName) + '</div>' +
              bucketsHtml +
            '</div>';
          }).join('<div class="quota-divider"></div>');

          quotaSummary = '<div class="quota-box">' + groupsHtml + '</div>';
        } else if (acc.quota && acc.quota.models && acc.quota.models.length > 0) {
          const proModel = acc.quota.models.find(m => m.name.includes('gemini-2.5-pro') || m.name.includes('gemini-3'));
          const claudeModel = acc.quota.models.find(m => m.name.includes('claude'));
          const proPct = proModel ? Math.round(proModel.percentage) : 100;
          const claudePct = claudeModel ? Math.round(claudeModel.percentage) : 100;

          quotaSummary = '<div class="quota-box">' +
            '<div class="quota-group">' +
              '<div class="quota-group-header">Gemini Models</div>' +
              '<div class="quota-bucket-row">' +
                '<div class="quota-bucket-info"><span class="quota-bucket-title">Limit Remaining</span></div>' +
                renderQuotaCircle(proPct) +
              '</div>' +
            '</div>' +
            '<div class="quota-divider"></div>' +
            '<div class="quota-group">' +
              '<div class="quota-group-header">Claude and GPT models</div>' +
              '<div class="quota-bucket-row">' +
                '<div class="quota-bucket-info"><span class="quota-bucket-title">Limit Remaining</span></div>' +
                renderQuotaCircle(claudePct) +
              '</div>' +
            '</div>' +
          '</div>';
        }

        const card = document.createElement('div');
        card.className = 'account-card' + (isCurrent ? ' is-current' : '');
        card.innerHTML = \`
          \${isCurrent ? '<div class="current-badge">✓ 当前活跃</div>' : ''}
          <div class="account-header">
            <div class="avatar">\${initial}</div>
            <div class="account-info">
              <div class="account-email" title="\${acc.email}">\${acc.email}</div>
              <div class="account-meta">
                <span class="tag tag-tier">\${acc.tier || 'FREE'}</span>
                <span class="tag \${acc.disabled ? 'tag-disabled' : 'tag-active'}">\${acc.disabled ? '已停用' : '活跃'}</span>
              </div>
            </div>
          </div>
          \${quotaSummary}
          <div class="account-actions">
            \${!isCurrent ? \`<button class="btn btn-success" onclick="switchAccount('\${acc.id}')">🚀 切换为当前</button>\` : \`<button class="btn btn-secondary" disabled>已激活</button>\`}
            <button class="btn btn-secondary" onclick="refreshAccount('\${acc.id}')">🔄 刷新</button>
            <button class="btn btn-danger" onclick="removeAccount('\${acc.id}')">🗑 移除</button>
          </div>
        \`;
        grid.appendChild(card);
      });
    }

    function renderConfig(cfg) {
      document.getElementById('autoRotateAntigravity').checked = !!cfg.proxy.autoRotateAntigravity;
      document.getElementById('autoRotateIntervalMinutes').value = cfg.proxy.autoRotateIntervalMinutes || 60;
      document.getElementById('switchPreviewModel').checked = cfg.proxy.switchPreviewModel !== false;

      const autoStatus = cfg.proxy.autoRotateAntigravity
        ? '已启用 (' + (cfg.proxy.autoRotateIntervalMinutes || 60) + ' 分钟轮换)'
        : '未启用';
      document.getElementById('autoRotateStatus').innerText = autoStatus;
    }

    async function switchAccount(id) {
      try {
        const res = await fetch('/api/antigravity/switch', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ id })
        });
        const data = await res.json();
        if (data.success) {
          showToast('已一键切换 Antigravity IDE 账号为: ' + data.account.email);
          loadData();
        } else {
          showToast(data.error || '切换失败', true);
        }
      } catch (err) {
        showToast(err.message, true);
      }
    }

    async function rotateCurrent() {
      try {
        const res = await fetch('/api/antigravity/rotate', { method: 'POST' });
        const data = await res.json();
        if (data.success) {
          showToast('已轮换 Antigravity IDE 账号为: ' + data.account.email);
          loadData();
        } else {
          showToast(data.error || '轮换失败', true);
        }
      } catch (err) {
        showToast(err.message, true);
      }
    }

    async function refreshAccount(id) {
      showToast('正在刷新账号凭据...');
      try {
        await fetch('/api/accounts/' + id + '/refresh', { method: 'POST' });
        showToast('账号刷新成功');
        loadData();
      } catch (err) {
        showToast(err.message, true);
      }
    }

    async function removeAccount(id) {
      if (!confirm('确定从账号池中移除该账号吗？')) return;
      try {
        await fetch('/api/accounts/' + id, { method: 'DELETE' });
        showToast('已移除账号');
        loadData();
      } catch (err) {
        showToast(err.message, true);
      }
    }

    async function refreshQuotas() {
      showToast('正在刷新所有账号配额...');
      try {
        await fetch('/api/accounts/refresh-all', { method: 'POST' });
        showToast('配额刷新完成');
        loadData();
      } catch (err) {
        showToast(err.message, true);
      }
    }

    async function saveSettings() {
      const payload = {
        proxy: {
          autoRotateAntigravity: document.getElementById('autoRotateAntigravity').checked,
          autoRotateIntervalMinutes: parseInt(document.getElementById('autoRotateIntervalMinutes').value, 10) || 60,
          switchPreviewModel: document.getElementById('switchPreviewModel').checked
        }
      };

      try {
        const res = await fetch('/api/config', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });
        const data = await res.json();
        if (data.success) {
          showToast('设置保存成功');
          loadData();
        }
      } catch (err) {
        showToast(err.message, true);
      }
    }

    async function startLogin() {
      const modal = document.getElementById('loginModal');
      const linkContainer = document.getElementById('loginLinkContainer');
      const authLink = document.getElementById('loginAuthLink');
      const prompt = document.getElementById('loginPrompt');
      modal.classList.add('active');
      linkContainer.style.display = 'none';

      try {
        const res = await fetch('/api/auth/start', { method: 'POST' });
        const data = await res.json();
        if (data.url) {
          authLink.href = data.url;
          linkContainer.style.display = 'block';
          prompt.innerText = '请在弹出的页面登录，若未自动跳转请点击下方按钮：';
          window.open(data.url, '_blank');
          startPollingLogin();
        }
      } catch (err) {
        prompt.innerText = '启动登录失败: ' + err.message;
      }
    }

    function startPollingLogin() {
      if (pollInterval) clearInterval(pollInterval);
      pollInterval = setInterval(async () => {
        try {
          const res = await fetch('/api/auth/status');
          const data = await res.json();
          if (data.status === 'success') {
            clearInterval(pollInterval);
            closeModal();
            showToast('🎉 Google 账户 ' + data.account.email + ' 登录成功！');
            loadData();
          } else if (data.status === 'error') {
            clearInterval(pollInterval);
            document.getElementById('loginPrompt').innerText = '登录失败: ' + (data.error || '未知错误');
          }
        } catch {}
      }, 2000);
    }

    function closeModal() {
      document.getElementById('loginModal').classList.remove('active');
      if (pollInterval) clearInterval(pollInterval);
    }

    window.onload = loadData;
  </script>
</body>
</html>
`;
}
