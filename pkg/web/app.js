let allImages = [];

function toast(msg, type = 'info') {
    const container = document.getElementById('toast-container');
    const el = document.createElement('div');
    el.className = `toast toast-${type}`;
    el.innerText = msg;
    container.appendChild(el);
    setTimeout(() => {
        el.classList.add('toast-fade-out');
        setTimeout(() => el.remove(), 300);
    }, 4000);
}

document.addEventListener('DOMContentLoaded', async () => {
    const tabs = document.querySelectorAll('.nav-item, .icon-nav-item');
    tabs.forEach(tab => {
        tab.addEventListener('click', () => {
            showTab(tab.dataset.tab);
            if (window.innerWidth <= 768) {
                document.getElementById('sidebar').classList.remove('open');
            }
        });
    });

    const menuToggle = document.getElementById('menu-toggle');
    if (menuToggle) {
        menuToggle.addEventListener('click', () => {
            document.getElementById('sidebar').classList.toggle('open');
        });
    }

    setInterval(() => {
        const activeNavItem = document.querySelector('.nav-item.active');
        if (activeNavItem && activeNavItem.dataset.tab === 'instances') fetchInstances();
        if (activeNavItem && activeNavItem.dataset.tab === 'system') fetchHostMetrics();
    }, 2000);

    // Initial load
    try {
        await Promise.all([fetchInstances(true), preloadImages()]);
    } catch (e) {
        console.error("Initial load failed:", e);
    } finally {
        hideSplash();
    }
});

function hideSplash() {
    const splash = document.getElementById('splash');
    if (!splash) return;
    splash.style.opacity = '0';
    setTimeout(() => {
        splash.style.visibility = 'hidden';
    }, 500);
}

function showTab(tabId) {
    document.querySelectorAll('.view').forEach(v => v.style.display = 'none');
    document.querySelectorAll('.nav-item, .icon-nav-item').forEach(t => t.classList.remove('active'));

    const target = document.getElementById(tabId);
    if (target) target.style.display = 'block';

    document.querySelectorAll(`[data-tab="${tabId}"]`).forEach(t => t.classList.add('active'));

    if (tabId === 'instances') fetchInstances(true);
    if (tabId === 'images') fetchImages(true);
    if (tabId === 'users') fetchUsers(true);
    if (tabId === 'log') fetchLogs(true);
    if (tabId === 'system') fetchHostMetrics(true);
}

async function fetchInstances(animate = false) {
    try {
        const res = await fetch('/api/v1/instances');
        const data = await res.json();
        const listContainer = document.getElementById('instance-list');

        // Identify instances to remove
        const newNames = data.map(vm => vm.Name);
        const currentCards = listContainer.querySelectorAll('.instance-card');
        currentCards.forEach(card => {
            const name = card.id.replace('card-', '');
            if (!newNames.includes(name) && !card.dataset.optimistic) card.remove();
        });

        data.forEach(vm => {
            let card = document.getElementById(`card-${vm.Name}`);
            const isNew = !card;

            if (isNew) {
                card = document.createElement('div');
                card.id = `card-${vm.Name}`;
                card.className = 'card instance-card' + (animate ? ' animate-in' : '');
                listContainer.appendChild(card);
            }

            // Skip update if user is interacting with dropdown
            if (card.querySelector('.dropdown.show')) return;

            const statusText = vm.Status || 'unknown';
            const statusClass = `status-${statusText}`;
            const memUsed = ((vm.MemoryUsage || 0) / 1024).toFixed(1);
            const memTotal = ((vm.MemoryMB || 1024) / 1024).toFixed(1);
            const diskUsed = ((vm.DiskUsage || 0) / 1024 / 1024 / 1024).toFixed(1);
            const diskTotal = vm.DiskGB ? vm.DiskGB.toFixed(1) : diskUsed;

            const cpuPerc = (vm.CPUUsage || 0).toFixed(1);
            const typeLabel = vm.Type === 'container' ? 'LXD' : 'VM';
            const memTotalVal = vm.MemoryMB || 1024;
            const memPerc = Math.min(100, ((vm.MemoryUsage || 0) / memTotalVal * 100)).toFixed(1);
            const diskTotalBytes = (vm.DiskGB || 0) * 1024 * 1024 * 1024;
            const diskPerc = diskTotalBytes > 0 ? Math.min(100, ((vm.DiskUsage || 0) / diskTotalBytes * 100)).toFixed(1) : 0;

            const primaryIP = (vm.IPs && vm.IPs.length > 0) ? vm.IPs[0] : 'N/A';

            card.innerHTML = `
                ${vm.Status === 'deploying' ? `<div class="overlay" id="deploy-overlay-${vm.Name}">DEPLOYING...</div>` : ''}
                <div class="instance-header">
                    <div style="display:flex; align-items:center; gap:12px;">
                        <div class="instance-icon">
                            ${vm.Type === 'container' ?
                                '<svg viewBox="0 0 24 24"><path d="M12,2L4.5,20.29L5.21,21L12,18L18.79,21L19.5,20.29L12,2Z" fill="currentColor"/></svg>' :
                                '<svg viewBox="0 0 24 24"><path d="M21,16.5C21,16.88 20.79,17.21 20.47,17.38L12.57,21.82C12.41,21.94 12.21,22 12,22C11.79,22 11.59,21.94 11.43,21.82L3.53,17.38C3.21,17.21 3,16.88 3,16.5V7.5C3,7.12 3.21,6.79 3.53,6.62L11.43,2.18C11.59,2.06 11.79,2 12,2C12.21,2 12.41,2.06 12.57,2.18L20.47,6.62C20.79,6.79 21,7.12 21,7.5V16.5Z" fill="currentColor"/></svg>'
                            }
                        </div>
                        <div>
                            <div class="instance-title">${vm.Name}</div>
                            <div class="instance-status ${statusClass}">${statusText.toUpperCase()}</div>
                        </div>
                    </div>
                    <div class="actions-menu">
                        <button class="dots-btn" onclick="toggleDropdown(event, '${vm.Name}')">
                            <svg viewBox="0 0 24 24"><path d="M12 16a2 2 0 110 4 2 2 0 010-4zm0-6a2 2 0 110 4 2 2 0 010-4zm0-6a2 2 0 110 4 2 2 0 010-4z"/></svg>
                        </button>
                        <div id="dropdown-${vm.Name}" class="dropdown">
                            <div class="dropdown-item" onclick="action('launch', '${vm.Name}')">START</div>
                            <div class="dropdown-item" onclick="action('stop', '${vm.Name}')">STOP</div>
                            <div class="dropdown-item" onclick="action('restart', '${vm.Name}')">RESTART</div>
                            <div class="dropdown-item" onclick="openEdit('${vm.Name}')">EDIT</div>
                            <div class="dropdown-item" onclick="openExec('${vm.Name}')">RUN CODE</div>
                            ${vm.Status === 'running' ? `<div class="dropdown-item" style="color:var(--primary);" onclick="getSSH('${vm.Name}')">SSH TOKEN</div>` : ''}
                            <div class="dropdown-divider"></div>
                            <div class="dropdown-item" style="color:var(--danger);" onclick="action('delete', '${vm.Name}')">DELETE</div>
                        </div>
                    </div>
                </div>

                <div class="instance-info-row">
                    <div class="info-group">
                        <div class="info-label">IP Address</div>
                        <div class="info-value">${primaryIP}</div>
                    </div>
                    <div class="info-group">
                        <div class="info-label">Type</div>
                        <div class="info-value" style="text-transform:uppercase;">${typeLabel}</div>
                    </div>
                </div>

                <div class="metrics-grid">
                    <div class="stat-item">
                        <div class="stat-header">
                            <span>CPU usage</span>
                            <span>${cpuPerc}%</span>
                        </div>
                        <div class="stat-progress"><div class="progress-fill" style="width: ${cpuPerc}%"></div></div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-header">
                            <span>Memory usage</span>
                            <span>${memUsed} / ${memTotal} GB</span>
                        </div>
                        <div class="stat-progress"><div class="progress-fill" style="width: ${memPerc}%"></div></div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-header">
                            <span>Disk usage</span>
                            <span>${diskUsed} / ${diskTotal} GB</span>
                        </div>
                        <div class="stat-progress"><div class="progress-fill" style="width: ${diskPerc}%"></div></div>
                    </div>
                </div>

                <div style="margin-top:20px;">
                    <button class="btn btn-outline" onclick="action('restart', '${vm.Name}')" style="width:100%;">Reboot VPS</button>
                </div>
            `;
            listContainer.appendChild(card);
        });
    } catch (e) {
        console.error("Failed to fetch instances:", e);
    }
}

function toggleDropdown(e, name) {
    e.stopPropagation();
    document.querySelectorAll('.dropdown').forEach(d => {
        if (d.id !== 'dropdown-' + name) d.classList.remove('show');
    });
    const dropdown = document.getElementById('dropdown-' + name);
    if (dropdown) dropdown.classList.toggle('show');
}

window.onclick = () => {
    document.querySelectorAll('.dropdown').forEach(d => d.classList.remove('show'));
};

async function preloadImages() {
    try {
        const res = await fetch('/api/v1/images');
        allImages = await res.json();
    } catch(e) {}
}

function filterImages() {
    const query = document.getElementById('image-search').value.toLowerCase();
    const type = document.getElementById('deploy-type').value;
    const dd = document.getElementById('image-dropdown');
    const filtered = allImages.filter(img => {
        const matchesQuery = img.Name.toLowerCase().includes(query);
        const matchesType = img.Type === type;
        return matchesQuery && matchesType;
    });

    if (filtered.length > 0 && query.length > 0) {
        dd.style.display = 'block';
        dd.innerHTML = filtered.map(img => `
            <div class="dropdown-item" onclick="selectImage('${img.Name}')">
                ${img.Name} <span style="font-size:0.7rem; color:#888;">(${img.Type})</span>
            </div>
        `).join('');
    } else dd.style.display = 'none';
}

function selectImage(name) {
    document.getElementById('image-search').value = name;
    document.getElementById('deploy-image').value = name;
    document.getElementById('image-dropdown').style.display = 'none';
}

function toggleDeployFields() {
    const type = document.getElementById('deploy-type').value;
    const note = document.getElementById('lxd-deploy-note');
    if (type === 'container') {
        note.style.display = 'block';
    } else {
        note.style.display = 'none';
    }
}

function toggleAddImgFields() {
    const type = document.getElementById('img-type').value;
    const nameLabel = document.getElementById('img-name-label');
    const urlLabel = document.getElementById('img-url-label');
    const help = document.getElementById('img-help');
    const urlInput = document.getElementById('img-url');

    if (type === 'container') {
        nameLabel.innerText = "LXD Image Alias";
        urlLabel.innerText = "LXD Source (Format: ubuntu:24.04)";
        urlInput.placeholder = "e.g. ubuntu:24.04 or images:debian/12";
        help.innerText = "This will register an alias in the LXD image store.";
    } else {
        nameLabel.innerText = "Image Label";
        urlLabel.innerText = "Source URL (QCOW2)";
        urlInput.placeholder = "https://cloud-images.ubuntu.com/...";
        help.innerText = "";
    }
}

async function deployInstance() {
    const btn = document.querySelector('#deploy .btn-primary');
    const oldText = btn.innerText;
    btn.innerText = "STARTING...";
    btn.disabled = true;

    const name = document.getElementById('deploy-name').value;
    const type = document.getElementById('deploy-type').value;
    const data = {
        name: name,
        image: document.getElementById('deploy-image').value || document.getElementById('image-search').value,
        type: type,
        cpus: parseInt(document.getElementById('deploy-cpu').value),
        memory_mb: parseInt(document.getElementById('deploy-ram').value),
        disk_gb: parseInt(document.getElementById('deploy-disk').value),
        user: document.getElementById('deploy-user').value,
        password: document.getElementById('deploy-pass').value
    };

    // Optimistic Deploy Card
    const listContainer = document.getElementById('instance-list');
    const optCard = document.createElement('div');
    optCard.id = `card-${name}`;
    optCard.className = 'card instance-card animate-in';
    optCard.dataset.optimistic = "true";
    optCard.innerHTML = `
        <div class="overlay">INITIATING...</div>
        <div class="instance-header">
            <div style="display:flex; align-items:center; gap:12px;">
                <div class="instance-icon">${type === 'container' ? 'LXD' : 'VM'}</div>
                <div><div class="instance-title">${name}</div><div class="instance-status status-processing">PREPARING</div></div>
            </div>
        </div>
    `;
    listContainer.prepend(optCard);
    showTab('instances');

    try {
        const res = await fetch('/api/v1/deploy', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        const result = await res.json();
        if (!res.ok) {
            toast(result.error || "Deployment failed", 'error');
            optCard.remove();
        } else {
            toast("Deployment started", 'success');
            delete optCard.dataset.optimistic;
        }
    } catch (e) {
        toast("Network error", 'error');
        optCard.remove();
    } finally {
        btn.innerText = oldText;
        btn.disabled = false;
    }
}

async function openEdit(name) {
    const res = await fetch(`/api/v1/info/${name}`);
    const info = await res.json();
    document.getElementById('edit-title').innerText = "Edit: " + name;
    document.getElementById('edit-name').value = info.Name;
    document.getElementById('edit-cpu').value = info.CPUs;
    document.getElementById('edit-ram').value = info.MemoryMB;
    document.getElementById('edit-user').value = info.User || 'ubuntu';
    document.getElementById('edit-pass').value = '';
    showTab('edit');
}

async function updateInstance() {
    const oldName = document.getElementById('edit-title').innerText.split(': ')[1];
    const data = {
        name: document.getElementById('edit-name').value,
        cpus: parseInt(document.getElementById('edit-cpu').value),
        memory_mb: parseInt(document.getElementById('edit-ram').value),
        user: document.getElementById('edit-user').value,
        password: document.getElementById('edit-pass').value
    };
    const res = await fetch(`/api/v1/update/${oldName}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    });
    if (res.ok) showTab('instances');
    else alert("Error: " + (await res.json()).error);
}

async function action(type, name) {
    if (type === 'delete' && !confirm(`Are you sure you want to delete ${name}?`)) return;

    // Optimistic UI
    const card = document.getElementById(`card-${name}`);
    let oldStatus = '';
    if (card) {
        const statusEl = card.querySelector('.instance-status');
        oldStatus = statusEl.innerText;
        statusEl.innerText = type === 'delete' ? 'DELETING...' : 'PROCESSING...';
        statusEl.className = 'instance-status status-processing';
    }

    try {
        const res = await fetch(`/api/v1/${type}/${name}`, { method: type === 'delete' ? 'DELETE' : 'POST' });
        const data = await res.json();
        if (res.ok) {
            toast(`${name}: ${type} success`, 'success');
            if (type === 'delete') {
                if (card) card.style.opacity = '0.5';
            }
        } else {
            toast(data.error || "Operation failed", 'error');
            // Revert on error
            if (card) {
                const statusEl = card.querySelector('.instance-status');
                statusEl.innerText = oldStatus;
                statusEl.className = `instance-status status-${oldStatus.toLowerCase()}`;
            }
        }
    } catch (e) {
        toast("Network error", 'error');
    }
    fetchInstances();
}

async function getSSH(name) {
    const output = document.getElementById('exec-output');
    document.getElementById('exec-title').innerText = "SSH Setup: " + name;
    output.innerText = "Initializing persistent SSH tunnel inside " + name + "...\n";
    document.getElementById('exec-command').value = "";
    showTab('exec');

    try {
        const res = await fetch(`/api/v1/ssh/${name}`, { method: 'POST' });
        const data = await res.json();
        if (res.ok) {
            output.innerText += "\n--- SUCCESS ---\n";
            output.innerText += "SSH Token: " + data.token + "\n";
            output.innerText += "The tunnel is now running in the background of your VPS.\n";
            output.innerText += "You can use this token at https://ks-ssh.pages.dev\n";
        } else {
            output.innerText += "\n--- ERROR ---\n" + data.error + "\n";
        }
    } catch (e) {
        output.innerText += "\n--- FETCH FAILED ---\n" + e.message + "\n";
    }
}

function openExec(name) {
    document.getElementById('exec-title').innerText = "Terminal: " + name;
    document.getElementById('exec-output').innerText = "";
    document.getElementById('exec-command').value = "";
    showTab('exec');
}

async function runExec() {
    const name = document.getElementById('exec-title').innerText.split(': ')[1];
    const input = document.getElementById('exec-command');
    const cmd = input.value;
    const output = document.getElementById('exec-output');
    if (!cmd) return;

    output.innerText += "> " + cmd + "\n[RUNNING...]\n";
    input.value = "";
    output.scrollTop = output.scrollHeight;

    try {
        const res = await fetch(`/api/v1/exec/${name}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ command: cmd })
        });
        const data = await res.json();
        // Remove the [RUNNING...] line
        output.innerText = output.innerText.replace("[RUNNING...]\n", "");
        if (data.output) output.innerText += data.output + "\n";
        if (data.error) output.innerText += "ERROR: " + data.error + "\n";
    } catch (e) {
        output.innerText = output.innerText.replace("[RUNNING...]\n", "");
        output.innerText += "FETCH FAILED: " + e.message + "\n";
    }
    output.scrollTop = output.scrollHeight;
}

async function fetchImages(animate = false) {
    const res = await fetch('/api/v1/images');
    allImages = await res.json();
    const list = document.getElementById('image-list');
    list.innerHTML = allImages.map(img => `
        <div class="card ${animate ? 'animate-in' : ''}">
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:12px;">
                <h3 style="margin:0;">${img.Name}</h3>
                <span class="badge" style="color:var(--primary); font-size:0.7rem; font-weight:800;">${img.Type.toUpperCase()}</span>
            </div>
            <p style="color:var(--text-muted); font-size:0.8rem; margin-bottom:20px;">Size: ${(img.Size / 1024 / 1024).toFixed(1)} MB</p>
            <div style="display:flex; gap:8px;">
                <button class="btn btn-outline" onclick="renameImage('${img.Name}')" style="flex:1; font-size:0.7rem;">Rename</button>
                <button class="btn btn-danger" onclick="removeImage('${img.Name}')" style="flex:1; font-size:0.7rem;">Delete</button>
            </div>
        </div>
    `).join('');
}

let charts = {};
async function fetchHostMetrics(init = false) {
    try {
        const res = await fetch('/api/v1/monitor');
        if (!res.ok) throw new Error("API status " + res.status);
        const data = await res.json();
        const m = data.metrics;

        const activeInstEl = document.getElementById('sys-active-inst');
        if (activeInstEl) activeInstEl.innerText = data.active_instances !== undefined ? data.active_instances : "N/A";

        const uptimeEl = document.getElementById('sys-uptime');
        if (uptimeEl) uptimeEl.innerText = (m && m.uptime) ? (m.uptime / 3600).toFixed(1) + " hours" : "N/A";

        const kernelEl = document.getElementById('sys-kernel');
        if (kernelEl && m && m.kernel) kernelEl.innerText = m.kernel;

        if (!m) {
            console.warn("No metrics data received");
            if (init) {
                document.querySelectorAll('.view#system .card').forEach(c => {
                    if (!c.querySelector('.error-msg')) {
                        c.innerHTML += '<p class="error-msg" style="color:var(--warning);font-size:0.7rem;margin-top:10px;">Metrics temporarily unavailable.</p>';
                    }
                });
            }
            return;
        }

        if (!window.Chart) {
            if (init) {
                document.querySelectorAll('.view#system .card').forEach(c => {
                    if (!c.querySelector('.error-msg')) {
                        c.innerHTML += '<p class="error-msg" style="color:var(--danger);font-size:0.7rem;margin-top:10px;">Monitoring component missing.</p>';
                    }
                });
            }
            return;
        }

        if (init || !charts.cpu || !document.getElementById('cpuChart')) initCharts(m);
        else updateCharts(m);

    } catch (e) {
        console.error("Host metrics failed:", e);
        if (init) {
            const activeInstEl = document.getElementById('sys-active-inst');
            if (activeInstEl) activeInstEl.innerText = "N/A";
            const uptimeEl = document.getElementById('sys-uptime');
            if (uptimeEl) uptimeEl.innerText = "Unavailable";
        }
    }
}

function initCharts(m) {
    // Destroy existing charts to prevent "blank" or "overlapping" issues
    Object.keys(charts).forEach(key => {
        if (charts[key] && typeof charts[key].destroy === 'function') charts[key].destroy();
    });

    const cpuCanvas = document.getElementById('cpuChart');
    if (!cpuCanvas) return;
    const ctxCpu = cpuCanvas.getContext('2d');
    charts.cpu = new Chart(ctxCpu, {
        type: 'line',
        data: { labels: Array(10).fill(''), datasets: [{ label: 'CPU %', data: Array(10).fill(0), borderColor: '#673de6', tension: 0.4 }] },
        options: { animation: false, scales: { y: { min: 0, max: 100 } } }
    });

    const ctxRam = document.getElementById('ramChart').getContext('2d');
    charts.ram = new Chart(ctxRam, {
        type: 'doughnut',
        data: { labels: ['Used', 'Free'], datasets: [{ data: [m.mem_used, m.mem_total - m.mem_used], backgroundColor: ['#673de6', '#f3f4f6'] }] },
        options: { animation: false }
    });

    const ctxDisk = document.getElementById('diskChart').getContext('2d');
    charts.disk = new Chart(ctxDisk, {
        type: 'pie',
        data: { labels: ['Used', 'Free'], datasets: [{ data: [m.disk_used, m.disk_total - m.disk_used], backgroundColor: ['#ef4444', '#f3f4f6'] }] },
        options: { animation: false }
    });

    const ctxNet = document.getElementById('netChart').getContext('2d');
    charts.net = new Chart(ctxNet, {
        type: 'line',
        data: { labels: Array(10).fill(''), datasets: [
            { label: 'Recv', data: Array(10).fill(0), borderColor: '#10b981' },
            { label: 'Sent', data: Array(10).fill(0), borderColor: '#f59e0b' }
        ] },
        options: { animation: false }
    });
    charts.lastNet = { r: m.net_recv, s: m.net_sent, t: Date.now() };
}

function updateCharts(m) {
    // Update CPU
    charts.cpu.data.datasets[0].data.push(m.cpu_usage);
    charts.cpu.data.datasets[0].data.shift();
    charts.cpu.update();

    // Update RAM
    charts.ram.data.datasets[0].data = [m.mem_used, m.mem_total - m.mem_used];
    charts.ram.update();

    // Update Disk
    charts.disk.data.datasets[0].data = [m.disk_used, m.disk_total - m.disk_used];
    charts.disk.update();

    // Update Network
    const now = Date.now();
    const dt = (now - charts.lastNet.t) / 1000;
    const rx = (m.net_recv - charts.lastNet.r) / 1024 / dt;
    const tx = (m.net_sent - charts.lastNet.s) / 1024 / dt;

    charts.net.data.datasets[0].data.push(rx);
    charts.net.data.datasets[0].data.shift();
    charts.net.data.datasets[1].data.push(tx);
    charts.net.data.datasets[1].data.shift();
    charts.net.update();

    charts.lastNet = { r: m.net_recv, s: m.net_sent, t: now };
}

function showAddImageForm(show = true) {
    showTab('add-image');
}

async function addImage() {
    const card = document.querySelector('#add-image .card');
    const overlay = document.createElement('div');
    overlay.className = 'overlay';
    overlay.innerText = 'DOWNLOADING...';
    card.appendChild(overlay);

    const data = {
        name: document.getElementById('img-name').value,
        url: document.getElementById('img-url').value,
        type: document.getElementById('img-type').value
    };
    const res = await fetch('/api/v1/images', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    });
    card.removeChild(overlay);
    if (res.ok) {
        showTab('images');
        fetchImages();
    } else alert("Error adding image");
}

async function removeImage(name) {
    if (!confirm(`Remove image ${name}?`)) return;
    toast(`Removing image ${name}...`);
    try {
        const res = await fetch(`/api/v1/images/${name}`, { method: 'DELETE' });
        if (res.ok) toast("Image removed", 'success');
        else toast("Failed to remove image", 'error');
    } catch (e) { toast("Network error", 'error'); }
    fetchImages();
}

async function renameImage(name) {
    const newName = prompt("New name for " + name, name);
    if (!newName) return;
    toast(`Renaming ${name} to ${newName}...`);
    try {
        const res = await fetch('/api/v1/images/rename', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ old_name: name, new_name: newName })
        });
        if (res.ok) toast("Image renamed", 'success');
        else toast("Rename failed", 'error');
    } catch (e) { toast("Network error", 'error'); }
    fetchImages();
}

async function fetchUsers(animate = false) {
    const res = await fetch('/api/v1/users');
    const data = await res.json();
    const list = document.getElementById('user-list');
    list.innerHTML = data.map(u => `
        <div class="card ${animate ? 'animate-in' : ''}">
            <div style="display:flex; align-items:center; gap:12px; margin-bottom:12px;">
                <div style="background:var(--primary-light); color:var(--primary); width:40px; height:40px; border-radius:20px; display:flex; align-items:center; justify-content:center; font-weight:800; border:1px solid rgba(103, 61, 230, 0.1);">${u.username[0].toUpperCase()}</div>
                <div>
                    <h3 style="margin:0; font-size:1rem;">${u.username}</h3>
                    <div style="font-size:0.75rem; color:var(--text-muted);">${u.email}</div>
                </div>
            </div>
            <button class="btn btn-danger" onclick="deleteUser('${u.username}')" style="width:100%; font-size:0.7rem;">Delete User</button>
        </div>
    `).join('');
}

function showAddUserForm(show = true) {
    document.getElementById('add-user-form').style.display = show ? 'block' : 'none';
}

async function addUser() {
    const data = {
        username: document.getElementById('user-name').value,
        email: document.getElementById('user-email').value,
        password: document.getElementById('user-pass').value
    };
    await fetch('/api/v1/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    });
    showAddUserForm(false);
    fetchUsers();
}

async function deleteUser(user) {
    if (!confirm(`Delete user ${user}?`)) return;
    const res = await fetch(`/api/v1/users/${user}`, { method: 'DELETE' });
    if (!res.ok) alert((await res.json()).error);
    fetchUsers();
}

async function fetchLogs(animate = false) {
    const res = await fetch('/api/v1/logs');
    const data = await res.json();
    const list = document.getElementById('log-list');
    list.innerHTML = data.map(l => `
        <div class="card ${animate ? 'animate-in' : ''}" style="padding:16px; border-left:4px solid var(--primary); display:flex; justify-content:space-between; align-items:center; margin-bottom:8px;">
            <div>
                <div style="font-size:0.7rem; color:var(--text-muted); margin-bottom:4px;">${l.timestamp} • IP: ${l.ip}</div>
                <div style="font-weight:700; font-size:0.9rem;"><span style="color:var(--primary);">${l.action.toUpperCase()}</span> on ${l.target}</div>
            </div>
            <div style="font-size:0.75rem; font-weight:800; color:var(--text-muted); background:var(--bg-main); padding:4px 8px; border-radius:4px;">${l.user.toUpperCase()}</div>
        </div>
    `).join('');
}
