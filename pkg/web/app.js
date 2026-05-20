document.addEventListener('DOMContentLoaded', () => {
    const tabs = document.querySelectorAll('.nav-item');
    tabs.forEach(tab => {
        tab.addEventListener('click', () => {
            showTab(tab.dataset.tab);
            if (window.innerWidth <= 768) document.getElementById('sidebar').classList.remove('open');
        });
    });

    document.getElementById('menu-toggle').addEventListener('click', () => {
        document.getElementById('sidebar').classList.toggle('open');
    });

    setInterval(() => {
        const activeTab = document.querySelector('.nav-item.active').dataset.tab;
        if (activeTab === 'instances') fetchInstances();
    }, 4000);

    fetchInstances();
});

function showTab(tabId) {
    document.querySelectorAll('.view').forEach(v => v.style.display = 'none');
    document.querySelectorAll('.nav-item').forEach(t => t.classList.remove('active'));

    const target = document.getElementById(tabId);
    if (target) target.style.display = 'block';

    const navItem = document.querySelector(`.nav-item[data-tab="${tabId}"]`);
    if (navItem) navItem.classList.add('active');

    if (tabId === 'instances') fetchInstances();
    if (tabId === 'images') fetchImages();
    if (tabId === 'users') fetchUsers();
    if (tabId === 'log') fetchLogs();
}

async function fetchInstances() {
    const res = await fetch('/api/v1/instances');
    const data = await res.json();
    const grid = document.getElementById('instance-grid');
    grid.innerHTML = '';
    data.forEach(vm => {
        const card = document.createElement('div');
        card.className = 'card';
        const statusClass = `status-${vm.Status}`;

        // Simple usage calculation for UI
        const memPerc = vm.MemoryUsage && vm.MemoryMB ? (vm.MemoryUsage / vm.MemoryMB * 100).toFixed(0) : 0;

        card.innerHTML = `
            <div style="display:flex; justify-content: space-between; align-items: flex-start; margin-bottom: 24px;">
                <div>
                    <h3 style="margin:0; font-size:1.15rem;">${vm.Name}</h3>
                    <div style="margin-top:6px; display:flex; gap:6px;">
                        <span class="badge badge-vm">${vm.Type}</span>
                    </div>
                </div>
                <span class="badge ${statusClass}">${vm.Status}</span>
            </div>

            <div class="metric-row">
                <span>CPU Allocation</span>
                <span style="font-weight:600; color:var(--text-main);">${vm.CPUs} vCPUs</span>
            </div>

            <div class="metric-row" style="margin-bottom:4px;">
                <span>Memory (RAM)</span>
                <span style="font-weight:600; color:var(--text-main);">${vm.MemoryUsage || 0} / ${vm.MemoryMB} MB</span>
            </div>
            <div class="progress-bg"><div class="progress-fill" style="width:${memPerc}%"></div></div>

            <div style="margin-top:20px; font-size:0.8rem; color:var(--text-muted);">
                IP Address: <span style="color:var(--text-main); font-weight:600;">${vm.IPs && vm.IPs.length > 0 ? vm.IPs[0] : '-'}</span>
            </div>

            <div style="margin-top:24px; display:flex; gap:8px; flex-wrap:wrap; border-top:1px solid var(--light-border); padding-top:16px;">
                <button class="btn" onclick="action('launch', '${vm.Name}')" style="flex:1;">Start</button>
                <button class="btn" onclick="action('stop', '${vm.Name}')" style="flex:1;">Stop</button>
                <button class="btn" onclick="openEdit('${vm.Name}')">Edit</button>
                <button class="btn btn-danger" onclick="action('delete', '${vm.Name}')">Remove</button>
            </div>
        `;
        grid.appendChild(card);
    });
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
    if (res.ok) showTab('instances');
    else alert("Provisioning Error: " + (await res.json()).error);
}

async function openEdit(name) {
    const res = await fetch(`/api/v1/info/${name}`);
    const info = await res.json();
    document.getElementById('edit-target-name').innerText = name;
    document.getElementById('edit-cpu').value = info.CPUs;
    document.getElementById('edit-ram').value = info.MemoryMB;
    showTab('edit');
}

async function updateInstance() {
    const name = document.getElementById('edit-target-name').innerText;
    const data = {
        cpus: parseInt(document.getElementById('edit-cpu').value),
        memory_mb: parseInt(document.getElementById('edit-ram').value)
    };
    const res = await fetch(`/api/v1/update/${name}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    });
    if (res.ok) showTab('instances');
    else alert("Update Error: " + (await res.json()).error);
}

async function action(type, name) {
    await fetch(`/api/v1/${type}/${name}`, { method: type === 'delete' ? 'DELETE' : 'POST' });
    fetchInstances();
}

async function fetchImages() {
    const res = await fetch('/api/v1/images');
    const data = await res.json();
    const list = document.getElementById('image-list');
    list.innerHTML = data.map(img => `
        <div class="card">
            <h3 style="margin-bottom:8px;">${img.Name}</h3>
            <p style="color:var(--text-muted); font-size:0.9rem; margin-bottom:20px;">Size: ${(img.Size / 1024 / 1024).toFixed(2)} MB</p>
            <div style="display:flex; gap:8px;">
                <button class="btn" onclick="renameImage('${img.Name}')" style="flex:1;">Rename</button>
                <button class="btn btn-danger" onclick="removeImage('${img.Name}')" style="flex:1;">Delete</button>
            </div>
        </div>
    `).join('');
}

function showAddImageForm(show = true) {
    document.getElementById('add-image-form').style.display = show ? 'block' : 'none';
}

async function addImage() {
    const data = { name: document.getElementById('img-name').value, url: document.getElementById('img-url').value };
    await fetch('/api/v1/images', { method: 'POST', body: JSON.stringify(data), headers: {'Content-Type': 'application/json'} });
    showAddImageForm(false);
    fetchImages();
}

async function removeImage(name) {
    if (!confirm(`Remove image ${name}?`)) return;
    await fetch(`/api/v1/images/${name}`, { method: 'DELETE' });
    fetchImages();
}

async function renameImage(name) {
    const newName = prompt("New name for " + name, name);
    if (!newName) return;
    await fetch('/api/v1/images/rename', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ old_name: name, new_name: newName })
    });
    fetchImages();
}

async function fetchUsers() {
    const res = await fetch('/api/v1/users');
    const data = await res.json();
    const list = document.getElementById('user-list');
    list.innerHTML = data.map(u => `
        <div class="card">
            <div style="display:flex; align-items:center; gap:12px; margin-bottom:16px;">
                <svg viewBox="0 0 24 24" style="width:32px; height:32px; fill:var(--primary-blue);"><path d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/></svg>
                <h3 style="margin:0;">${u.username}</h3>
            </div>
            <p style="color:var(--text-muted); font-size:0.9rem; margin-bottom:20px;">${u.email}</p>
            <button class="btn btn-danger" onclick="deleteUser('${u.username}')" style="width:100%;">Remove Account</button>
        </div>
    `).join('');
}

function showAddUserForm(show = true) {
    document.getElementById('add-user-form').style.display = show ? 'block' : 'none';
}

async function addUser() {
    const data = { username: document.getElementById('user-name').value, email: document.getElementById('user-email').value, password: document.getElementById('user-pass').value };
    await fetch('/api/v1/users', { method: 'POST', body: JSON.stringify(data), headers: {'Content-Type': 'application/json'} });
    showAddUserForm(false);
    fetchUsers();
}

async function deleteUser(user) {
    if (!confirm(`Delete user ${user}?`)) return;
    const res = await fetch(`/api/v1/users/${user}`, { method: 'DELETE' });
    if (!res.ok) alert((await res.json()).error);
    fetchUsers();
}

async function fetchLogs() {
    const res = await fetch('/api/v1/logs');
    const data = await res.json();
    const list = document.getElementById('log-list');
    list.innerHTML = data.map(l => `
        <div class="card audit-card">
            <div class="audit-time">${l.timestamp} (IP: ${l.ip})</div>
            <div>
                <span class="audit-action">${l.action.toUpperCase()}</span>
                on <span style="font-weight:600;">${l.target}</span>
                by <span style="color:var(--text-muted);">${l.user}</span>
            </div>
        </div>
    `).join('');
}
