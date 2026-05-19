document.addEventListener('DOMContentLoaded', () => {
    const tabs = document.querySelectorAll('nav li');
    const sections = document.querySelectorAll('.tab-content');

    tabs.forEach(tab => {
        tab.addEventListener('click', () => {
            tabs.forEach(t => t.classList.remove('active'));
            sections.forEach(s => s.classList.remove('active'));
            tab.classList.add('active');
            const section = document.getElementById(tab.dataset.tab);
            if (section) section.classList.add('active');

            if (tab.dataset.tab === 'instances') fetchInstances();
            if (tab.dataset.tab === 'images') fetchImages();
            if (tab.dataset.tab === 'monitor') fetchMonitor();
            if (tab.dataset.tab === 'log') fetchLogs();
            if (tab.dataset.tab === 'users') fetchUsers();
        });
    });

    document.getElementById('menu-toggle').addEventListener('click', () => {
        document.getElementById('sidebar').classList.toggle('open');
    });

    setInterval(() => {
        const activeTab = document.querySelector('nav li.active').dataset.tab;
        if (activeTab === 'monitor') fetchMonitor();
        if (activeTab === 'instances') fetchInstances();
    }, 5000);

    fetchInstances();
});

function showDeployForm(show = true) {
    document.getElementById('deploy-form').style.display = show ? 'block' : 'none';
}

async function deployInstance() {
    const name = document.getElementById('deploy-name').value;
    const image = document.getElementById('deploy-image').value;
    const res = await fetch('/api/v1/deploy', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, image })
    });
    if (res.ok) {
        showDeployForm(false);
        fetchInstances();
    } else {
        const err = await res.json();
        alert("Error: " + err.error);
    }
}

async function fetchInstances() {
    const res = await fetch('/api/v1/instances');
    const data = await res.json();
    const grid = document.getElementById('instance-grid');
    grid.innerHTML = '';
    data.forEach(vm => {
        const card = document.createElement('div');
        card.className = 'card glassmorphic';
        card.innerHTML = `
            <h3>${vm.Name} <span class="badge badge-${vm.Type}">${vm.Type}</span></h3>
            <div class="stats" style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin: 15px 0;">
                <div><span style="color:var(--accent-color)">STATUS:</span> ${vm.Status}</div>
                <div><span style="color:var(--accent-color)">CPU:</span> ${vm.CPUs || 0}</div>
                <div><span style="color:var(--accent-color)">RAM:</span> ${vm.MemoryMB || 0} MB</div>
                <div><span style="color:var(--accent-color)">DISK:</span> ${(vm.DiskUsage / 1024 / 1024).toFixed(2)} MB</div>
            </div>
            <p><span style="color:var(--accent-color)">IPs:</span> ${vm.IPs ? vm.IPs.join(', ') : '-'}</p>
            <div class="actions">
                <button title="Start" onclick="action('launch', '${vm.Name}')">▶</button>
                <button title="Stop" onclick="action('stop', '${vm.Name}')">■</button>
                <button title="Restart" onclick="action('restart', '${vm.Name}')">↻</button>
                <button title="Suspend" onclick="action('suspend', '${vm.Name}')">⏸</button>
                <button title="Resume" onclick="action('resume', '${vm.Name}')">⏵</button>
                <button title="Edit" onclick="editInstance('${vm.Name}')">✎</button>
                <button title="Delete" class="danger" onclick="action('delete', '${vm.Name}')">✖</button>
            </div>
        `;
        grid.appendChild(card);
    });
}

async function fetchImages() {
    const res = await fetch('/api/v1/images');
    const data = await res.json();
    const container = document.getElementById('image-list');
    container.innerHTML = '<div class="grid">' + data.map(img => `
        <div class="card glassmorphic">
            <h3>${img.Name}</h3>
            <p>Size: ${img.Size} bytes</p>
            <p>Path: ${img.Path}</p>
        </div>
    `).join('') + '</div>';
}

async function fetchMonitor() {
    const res = await fetch('/api/v1/monitor');
    const data = await res.json();
    document.getElementById('cpu-usage').innerText = data.cpu;
    document.getElementById('ram-usage').innerText = data.ram;
}

function showUserForm(show = true) {
    document.getElementById('user-form').style.display = show ? 'block' : 'none';
}

async function createUser() {
    const username = document.getElementById('user-name').value;
    const email = document.getElementById('user-email').value;
    const password = document.getElementById('user-pass').value;
    const perms = document.getElementById('user-perms').value.split(',');

    await fetch('/api/v1/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, email, password, permissions: perms })
    });
    showUserForm(false);
    fetchUsers();
}

async function fetchUsers() {
    const res = await fetch('/api/v1/users');
    const data = await res.json();
    const container = document.getElementById('user-list');
    container.innerHTML = data.map(user => `
        <div class="card glassmorphic">
            <h3>${user.username}</h3>
            <p>Email: ${user.email}</p>
            <p>Perms: ${user.permissions.join(', ')}</p>
        </div>
    `).join('');
}

async function fetchLogs() {
    const res = await fetch('/api/v1/logs');
    const data = await res.json();
    const logContainer = document.getElementById('audit-log');
    logContainer.innerHTML = data.map(log => {
        let undoBtn = '';
        if (['stop', 'launch', 'deploy'].includes(log.action)) {
            undoBtn = `<button onclick="undoAction(${log.id})" style="margin-left:10px; padding:2px 5px; font-size:10px; background:rgba(255,0,0,0.2); border:1px solid var(--accent-color); color:var(--accent-color); cursor:pointer;">UNDO</button>`;
        }
        return `<p>[${log.timestamp}] ${log.user}: ${log.action} ${log.target} ${undoBtn}</p>`;
    }).join('');
}

async function undoAction(id) {
    await fetch(`/api/v1/logs/undo/${id}`, { method: 'POST' });
    fetchLogs();
    fetchInstances();
}

async function editInstance(name) {
    const memory = prompt("Enter new memory (MiB):", "1024");
    const cpus = prompt("Enter new CPU count:", "1");
    if (memory && cpus) {
        await fetch(`/api/v1/update/${name}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ memory_mb: parseInt(memory), cpus: parseInt(cpus) })
        });
        fetchInstances();
    }
}

async function action(type, name) {
    const method = type === 'delete' ? 'DELETE' : 'POST';
    const url = `/api/v1/${type}/${name}`;
    await fetch(url, { method });
    fetchInstances();
}
