document.addEventListener('DOMContentLoaded', () => {
    const tabs = document.querySelectorAll('nav li');
    tabs.forEach(tab => {
        tab.addEventListener('click', () => {
            showTab(tab.dataset.tab);
        });
    });

    setInterval(() => {
        const activeTab = document.querySelector('nav li.active').dataset.tab;
        if (activeTab === 'monitor') fetchMonitor();
        if (activeTab === 'instances') fetchInstances();
    }, 5000);

    fetchInstances();
});

function showTab(tabId) {
    document.querySelectorAll('.tab-content').forEach(s => s.classList.remove('active'));
    document.querySelectorAll('nav li').forEach(t => t.classList.remove('active'));

    const target = document.getElementById(tabId);
    if (target) target.classList.add('active');

    const navItem = document.querySelector(`nav li[data-tab="${tabId}"]`);
    if (navItem) navItem.classList.add('active');

    if (tabId === 'instances') fetchInstances();
    if (tabId === 'images') fetchImages();
    if (tabId === 'monitor') fetchMonitor();
    if (tabId === 'log') fetchLogs();
    if (tabId === 'users') fetchUsers();
}

async function deployInstance() {
    const data = {
        name: document.getElementById('deploy-name').value,
        image: document.getElementById('deploy-image').value,
        cpus: parseInt(document.getElementById('deploy-cpu').value),
        memory_mb: parseInt(document.getElementById('deploy-ram').value),
        disk_gb: parseInt(document.getElementById('deploy-disk').value),
        user: document.getElementById('deploy-user').value,
        password: document.getElementById('deploy-pass').value
    };

    const res = await fetch('/api/v1/deploy', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    });

    if (res.ok) {
        showTab('instances');
    } else {
        const err = await res.json();
        alert("Provisioning failed: " + err.error);
    }
}

async function fetchInstances() {
    const res = await fetch('/api/v1/instances');
    const data = await res.json();
    const grid = document.getElementById('instance-grid');
    grid.innerHTML = '';
    data.forEach(vm => {
        const card = document.createElement('div');
        card.className = 'card';
        card.innerHTML = `
            <div style="display:flex; justify-content: space-between; align-items: center;">
                <h3 style="margin:0;">${vm.Name}</h3>
                <span class="badge badge-${vm.Type}">${vm.Type}</span>
            </div>
            <p style="color: #666; font-size: 0.9em; margin: 10px 0;">Status: ${vm.Status}</p>
            <div style="font-size: 0.85em; border-top: 1px solid #eee; padding-top:10px;">
                IPs: ${vm.IPs ? vm.IPs.join(', ') : '-'}
            </div>
            <div class="actions" style="margin-top:15px; display:flex; gap:5px; flex-wrap:wrap;">
                <button onclick="action('launch', '${vm.Name}')">START</button>
                <button onclick="action('stop', '${vm.Name}')">STOP</button>
                <button onclick="openEdit('${vm.Name}')">EDIT</button>
                <button class="danger" onclick="action('delete', '${vm.Name}')">DELETE</button>
            </div>
        `;
        grid.appendChild(card);
    });
}

function openEdit(name) {
    document.getElementById('edit-target-name').innerText = name;
    showTab('edit');
}

async function updateInstance() {
    const name = document.getElementById('edit-target-name').innerText;
    const data = {
        cpus: parseInt(document.getElementById('edit-cpu').value),
        memory_mb: parseInt(document.getElementById('edit-ram').value)
    };
    await fetch(`/api/v1/update/${name}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    });
    showTab('instances');
}

async function action(type, name) {
    const method = type === 'delete' ? 'DELETE' : 'POST';
    await fetch(`/api/v1/${type}/${name}`, { method });
    fetchInstances();
}

async function fetchImages() {
    const res = await fetch('/api/v1/images');
    const data = await res.json();
    const container = document.getElementById('image-list');
    container.innerHTML = data.map(img => `
        <div class="card">
            <h3>${img.Name}</h3>
            <p>Size: ${(img.Size / 1024 / 1024).toFixed(2)} MB</p>
        </div>
    `).join('');
}

async function fetchMonitor() {
    const res = await fetch('/api/v1/monitor');
    const data = await res.json();
    document.getElementById('cpu-usage').innerText = data.cpu + " %";
    document.getElementById('ram-usage').innerText = data.ram + " MiB";
}

async function fetchUsers() {
    const res = await fetch('/api/v1/users');
    const data = await res.json();
    const container = document.getElementById('user-list');
    container.innerHTML = data.map(u => `
        <div class="card">
            <h3>${u.username}</h3>
            <p>${u.email}</p>
        </div>
    `).join('');
}

async function fetchLogs() {
    const res = await fetch('/api/v1/logs');
    const data = await res.json();
    const container = document.getElementById('audit-log');
    container.innerHTML = data.map(l => `<p>[${l.timestamp}] ${l.action} -> ${l.target}</p>`).join('');
}
