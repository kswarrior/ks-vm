document.addEventListener('DOMContentLoaded', () => {
    const tabs = document.querySelectorAll('.nav-item');
    tabs.forEach(tab => {
        tab.addEventListener('click', () => {
            showTab(tab.dataset.tab);
            if (window.innerWidth <= 768) {
                document.getElementById('sidebar').classList.remove('open');
            }
        });
    });

    document.getElementById('menu-toggle').addEventListener('click', () => {
        document.getElementById('sidebar').classList.toggle('open');
    });

    setInterval(() => {
        const activeTab = document.querySelector('.nav-item.active').dataset.tab;
        if (activeTab === 'instances') fetchInstances();
    }, 5000);

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
        card.innerHTML = `
            <div style="display:flex; justify-content: space-between; align-items: flex-start; margin-bottom: 20px;">
                <div>
                    <h3 style="margin:0; font-size:1.1rem;">${vm.Name}</h3>
                    <span class="badge badge-${vm.Type}" style="margin-top:6px; display:inline-block;">${vm.Type}</span>
                </div>
                <span class="badge ${statusClass}">${vm.Status}</span>
            </div>
            <div style="font-size:0.85rem; color:var(--text-muted); display:grid; grid-template-columns:1fr 1fr; gap:8px;">
                <div>IP: ${vm.IPs && vm.IPs.length > 0 ? vm.IPs[0] : '-'}</div>
                <div>CPU: ${vm.CPUs || '-'}</div>
                <div>RAM: ${vm.MemoryMB ? vm.MemoryMB + 'MB' : '-'}</div>
            </div>
            <div style="margin-top:20px; display:flex; gap:8px; border-top:1px solid var(--border-color); padding-top:16px;">
                <button class="btn btn-outline" onclick="action('launch', '${vm.Name}')" style="padding:6px 12px; font-size:0.75rem;">Start</button>
                <button class="btn btn-outline" onclick="action('stop', '${vm.Name}')" style="padding:6px 12px; font-size:0.75rem;">Stop</button>
                <button class="btn btn-outline" onclick="openEdit('${vm.Name}')" style="padding:6px 12px; font-size:0.75rem;">Edit</button>
                <button class="btn btn-danger" onclick="action('delete', '${vm.Name}')" style="padding:6px 12px; font-size:0.75rem;">Delete</button>
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
    else {
        const err = await res.json();
        alert("Provisioning failed: " + err.error);
    }
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
    await fetch(`/api/v1/${type}/${name}`, { method: type === 'delete' ? 'DELETE' : 'POST' });
    fetchInstances();
}

async function fetchImages() {
    const res = await fetch('/api/v1/images');
    const data = await res.json();
    const list = document.getElementById('image-list');
    list.innerHTML = data.map(img => `
        <div class="card">
            <h3 style="margin-bottom:10px;">${img.Name}</h3>
            <p style="color:var(--text-muted); font-size:0.9rem; margin-bottom:15px;">Size: ${(img.Size / 1024 / 1024).toFixed(2)} MB</p>
            <div style="display:flex; gap:10px;">
                <button class="btn btn-outline" onclick="renameImage('${img.Name}')" style="font-size:0.7rem; padding:5px 10px;">Rename</button>
                <button class="btn btn-danger" onclick="deleteImage('${img.Name}')" style="font-size:0.7rem; padding:5px 10px;">Remove</button>
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

async function fetchUsers() {
    const res = await fetch('/api/v1/users');
    const data = await res.json();
    const list = document.getElementById('user-list');
    list.innerHTML = data.map(u => `
        <div class="card">
            <h3 style="margin-bottom:6px;">${u.username}</h3>
            <p style="color:var(--text-muted); margin-bottom:16px;">${u.email}</p>
            <button class="btn btn-danger" onclick="deleteUser('${u.username}')" style="font-size:0.7rem; padding:5px 10px;">Delete Account</button>
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
    await fetch(`/api/v1/users/${user}`, { method: 'DELETE' });
    fetchUsers();
}

async function fetchLogs() {
    const res = await fetch('/api/v1/logs');
    const data = await res.json();
    const body = document.getElementById('log-body');
    body.innerHTML = data.map(l => `
        <tr>
            <td>${l.timestamp}</td>
            <td style="font-weight:600;">${l.user}</td>
            <td style="color:var(--text-muted);">${l.ip}</td>
            <td><span style="color:var(--primary-blue); font-weight:700; font-size:0.7rem;">${l.action.toUpperCase()}</span></td>
            <td>${l.target}</td>
        </tr>
    `).join('');
}
