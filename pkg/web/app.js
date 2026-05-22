let allImages = [];

document.addEventListener('DOMContentLoaded', async () => {
    const tabs = document.querySelectorAll('.nav-item, .icon-nav-item');
    tabs.forEach(tab => {
        tab.addEventListener('click', () => {
            const tabId = tab.dataset.tab;
            showTab(tabId);
        });
    });

    setInterval(() => {
        const activeNavItem = document.querySelector('.nav-item.active');
        if (activeNavItem && activeNavItem.dataset.tab === 'instances') fetchInstances();
    }, 5000);

    try {
        await Promise.all([fetchInstances(), preloadImages()]);
    } finally {
        const splash = document.getElementById('splash');
        if (splash) {
            splash.style.opacity = '0';
            setTimeout(() => splash.style.visibility = 'hidden', 500);
        }
    }
});

function showTab(tabId) {
    document.querySelectorAll('.view').forEach(v => v.style.display = 'none');
    document.querySelectorAll('.nav-item, .icon-nav-item').forEach(t => t.classList.remove('active'));

    const target = document.getElementById(tabId);
    if (target) target.style.display = 'block';

    document.querySelectorAll(`[data-tab="${tabId}"]`).forEach(t => t.classList.add('active'));

    if (tabId === 'instances') fetchInstances();
    if (tabId === 'images') fetchImages();
    if (tabId === 'users') fetchUsers();
    if (tabId === 'log') fetchLogs();
}

async function fetchInstances() {
    const res = await fetch('/api/v1/instances');
    const data = await res.json();
    const list = document.getElementById('instance-list');
    list.innerHTML = '';

    data.forEach(vm => {
        const cpuPerc = (vm.CPUUsage || 0).toFixed(0);
        const memUsed = (vm.MemoryUsage / 1024).toFixed(1);
        const memTotal = (vm.MemoryMB / 1024).toFixed(1);
        const memPerc = Math.min(100, ((vm.MemoryUsage || 0) / (vm.MemoryMB || 1024) * 100)).toFixed(0);
        const diskUsed = (vm.DiskUsage / 1024 / 1024 / 1024).toFixed(1);
        const diskTotal = vm.DiskGB || 20;
        const diskPerc = Math.min(100, (vm.DiskUsage / (diskTotal * 1024 * 1024 * 1024) * 100)).toFixed(0);

        const card = document.createElement('div');
        card.innerHTML = `
            <div class="card">
                <div class="instance-main-card">
                    <div class="os-info">
                        <div class="os-icon">
                            <svg style="width:24px;height:24px;fill:var(--primary);" viewBox="0 0 24 24"><path d="M12,2L4.5,20.29L5.21,21L12,18L18.79,21L19.5,20.29L12,2Z"/></svg>
                        </div>
                        <div class="os-details">
                            <h3>${vm.Name}</h3>
                            <div class="status-badge status-${vm.Status}">${vm.Status.toUpperCase()}</div>
                        </div>
                    </div>

                    <div class="access-info">
                        <div class="access-item">
                            <label>Hostname</label>
                            <div>${vm.Name}.hstgr.cloud</div>
                        </div>
                        <div class="access-item">
                            <label>IP Address</label>
                            <div>${vm.IPs && vm.IPs.length > 0 ? vm.IPs[0] : '148.230.111.186'}</div>
                        </div>
                    </div>

                    <div style="display:flex; gap:12px;">
                        <button class="btn btn-primary" onclick="action('restart', '${vm.Name}')">Reboot VPS</button>
                        <button class="btn" onclick="toggleMoreMenu('${vm.Name}')">...</button>
                    </div>
                </div>

                <div class="metrics-grid">
                    <div class="metric-card">
                        <div class="metric-header">CPU usage ></div>
                        <div class="metric-value">${cpuPerc}%</div>
                        <div class="metric-chart" style="width:${cpuPerc}%"></div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-header">Memory usage ></div>
                        <div class="metric-value">${memPerc}%</div>
                        <div style="font-size:0.8rem; color:var(--text-muted);">${memUsed} GB / ${memTotal} GB</div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-header">Disk usage ></div>
                        <div class="metric-value">${diskUsed} GB / ${diskTotal} GB</div>
                        <div style="font-size:0.8rem; color:var(--text-muted);">${diskPerc}% used</div>
                    </div>
                </div>

                <div id="more-menu-${vm.Name}" style="display:none; margin-top:20px; border-top:1px solid #eee; padding-top:20px; gap:12px;">
                    <button class="btn" onclick="openExec('${vm.Name}')">Terminal</button>
                    <button class="btn" onclick="openEdit('${vm.Name}')">Settings</button>
                    <button class="btn" onclick="action('stop', '${vm.Name}')">Stop</button>
                    <button class="btn" style="color:var(--danger);" onclick="action('delete', '${vm.Name}')">Delete</button>
                </div>
            </div>
        `;
        list.appendChild(card);
    });
}

function toggleMoreMenu(name) {
    const m = document.getElementById('more-menu-' + name);
    m.style.display = m.style.display === 'none' ? 'flex' : 'none';
}

async function preloadImages() {
    const res = await fetch('/api/v1/images');
    allImages = await res.json();
}

function filterImages() {
    const q = document.getElementById('image-search').value.toLowerCase();
    const dd = document.getElementById('image-dropdown');
    const filtered = allImages.filter(i => i.Name.toLowerCase().includes(q));
    if (q && filtered.length > 0) {
        dd.style.display = 'block';
        dd.innerHTML = filtered.map(i => `<div style="padding:10px; cursor:pointer;" onclick="selectImage('${i.Name}')">${i.Name}</div>`).join('');
    } else dd.style.display = 'none';
}

function selectImage(n) {
    document.getElementById('image-search').value = n;
    document.getElementById('deploy-image').value = n;
    document.getElementById('image-dropdown').style.display = 'none';
}

async function deployInstance() {
    const data = {
        name: document.getElementById('deploy-name').value,
        image: document.getElementById('deploy-image').value || document.getElementById('image-search').value,
        cpus: parseInt(document.getElementById('deploy-cpu').value),
        memory_mb: parseInt(document.getElementById('deploy-ram').value),
        disk_gb: parseInt(document.getElementById('deploy-disk').value),
        user: document.getElementById('deploy-user').value,
        password: document.getElementById('deploy-pass').value
    };
    await fetch('/api/v1/deploy', { method: 'POST', body: JSON.stringify(data) });
    showTab('instances');
}

async function openExec(n) {
    document.getElementById('exec-title').innerText = "Terminal: " + n;
    document.getElementById('exec-output').innerText = "Ready.";
    showTab('exec');
}

async function runExec() {
    const n = document.getElementById('exec-title').innerText.split(': ')[1];
    const c = document.getElementById('exec-command').value;
    const res = await fetch(`/api/v1/exec/${n}`, { method: 'POST', body: JSON.stringify({command: c}) });
    const data = await res.json();
    document.getElementById('exec-output').innerText = data.output;
}

async function action(type, name) {
    await fetch(`/api/v1/${type}/${name}`, { method: type === 'delete' ? 'DELETE' : 'POST' });
    fetchInstances();
}

async function fetchImages() {
    const res = await fetch('/api/v1/images');
    const images = await res.json();
    const list = document.getElementById('image-list');
    list.innerHTML = images.map(img => `
        <div class="card">
            <div style="font-weight:700; margin-bottom:12px;">${img.Name}</div>
            <div style="font-size:0.8rem; color:var(--text-muted); margin-bottom:20px;">${(img.Size / 1024 / 1024).toFixed(0)} MB</div>
            <button class="btn" style="width:100%;" onclick="removeImage('${img.Name}')">Remove</button>
        </div>
    `).join('');
}

async function fetchUsers() {
    const res = await fetch('/api/v1/users');
    const users = await res.json();
    const list = document.getElementById('user-list');
    list.innerHTML = users.map(u => `
        <div class="card">
            <div style="display:flex; align-items:center; gap:12px;">
                <div style="width:32px; height:32px; background:var(--primary-light); border-radius:16px; display:flex; align-items:center; justify-content:center; color:var(--primary); font-weight:700;">${u.username[0].toUpperCase()}</div>
                <div>
                    <div style="font-weight:600;">${u.username}</div>
                    <div style="font-size:0.75rem; color:var(--text-muted);">${u.email}</div>
                </div>
            </div>
        </div>
    `).join('');
}

async function fetchLogs() {
    const res = await fetch('/api/v1/logs');
    const logs = await res.json();
    const list = document.getElementById('log-list');
    list.innerHTML = logs.map(l => `
        <div class="card" style="padding:16px; font-size:0.85rem;">
            <span style="color:var(--text-muted);">${l.timestamp}</span> -
            <span style="font-weight:600;">${l.user}</span> executed
            <span style="color:var(--primary); font-weight:600;">${l.action}</span> on
            <span style="font-weight:600;">${l.target}</span>
        </div>
    `).join('');
}
