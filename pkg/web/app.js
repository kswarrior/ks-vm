document.addEventListener('DOMContentLoaded', () => {
    const tabs = document.querySelectorAll('nav li');
    tabs.forEach(tab => {
        tab.addEventListener('click', () => {
            showTab(tab.dataset.tab);
            if (window.innerWidth <= 768) toggleSidebar(false);
        });
    });

    document.getElementById('menu-toggle').addEventListener('click', () => {
        toggleSidebar();
    });

    setInterval(() => {
        const activeTab = document.querySelector('nav li.active').dataset.tab;
        if (activeTab === 'instances') fetchInstances();
    }, 5000);

    fetchInstances();
});

function toggleSidebar(show) {
    const sb = document.getElementById('sidebar');
    if (show === undefined) sb.classList.toggle('open');
    else show ? sb.classList.add('open') : sb.classList.remove('open');
}

function showTab(tabId) {
    document.querySelectorAll('.tab-content').forEach(s => s.style.display = 'none');
    document.querySelectorAll('nav li').forEach(t => t.classList.remove('active'));

    const target = document.getElementById(tabId);
    if (target) target.style.display = 'block';

    const navItem = document.querySelector(`nav li[data-tab="${tabId}"]`);
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
        card.innerHTML = `
            <div style="display:flex; justify-content: space-between; align-items: flex-start;">
                <div>
                    <h3 style="margin:0; color:var(--accent-color);">${vm.Name}</h3>
                    <div style="margin-top:5px;"><span class="badge badge-${vm.Type}">${vm.Type}</span></div>
                </div>
                <div style="text-align:right;">
                    <div style="font-size:0.8em; font-weight:700; color:${vm.Status === 'running' ? 'var(--success-color)' : '#666'}">${vm.Status.toUpperCase()}</div>
                    <div style="font-size:0.7em; color:#888;">${vm.IPs && vm.IPs.length > 0 ? vm.IPs[0] : 'No IP'}</div>
                </div>
            </div>
            <div class="actions" style="margin-top:20px; display:flex; gap:8px; flex-wrap:wrap;">
                <button onclick="action('launch', '${vm.Name}')">Start</button>
                <button onclick="action('stop', '${vm.Name}')">Stop</button>
                <button onclick="openEdit('${vm.Name}')">Edit</button>
                <button class="danger" onclick="action('delete', '${vm.Name}')">Delete</button>
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
        alert("Deploy error: " + err.error);
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
            <h3>${img.Name}</h3>
            <p style="font-size:0.9em; color:#666;">Size: ${(img.Size / 1024 / 1024).toFixed(2)} MB</p>
            <div style="display:flex; gap:10px;">
                <button onclick="renameImage('${img.Name}')">Rename</button>
                <button class="danger" onclick="deleteImage('${img.Name}')">Delete</button>
            </div>
        </div>
    `).join('');
}

function showAddImageForm(show = true) {
    document.getElementById('add-image-form').style.display = show ? 'block' : 'none';
}

async function addImage() {
    const name = document.getElementById('img-name').value;
    const url = document.getElementById('img-url').value;
    await fetch('/api/v1/images', { // Note: need to implement this POST in API
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, url })
    });
    showAddImageForm(false);
    fetchImages();
}

async function fetchUsers() {
    const res = await fetch('/api/v1/users');
    const data = await res.json();
    const list = document.getElementById('user-list');
    list.innerHTML = data.map(u => `
        <div class="card">
            <h3>${u.username}</h3>
            <p style="color:#666;">${u.email}</p>
            <div style="display:flex; gap:10px;">
                <button onclick="editUser('${u.username}')">Edit</button>
                <button class="danger" onclick="deleteUser('${u.username}')">Delete</button>
            </div>
        </div>
    `).join('');
}

function showAddUserForm(show = true) {
    document.getElementById('add-user-form').style.display = show ? 'block' : 'none';
}

async function fetchLogs() {
    const res = await fetch('/api/v1/logs');
    const data = await res.json();
    const body = document.getElementById('log-body');
    body.innerHTML = data.map(l => `
        <tr>
            <td style="padding:12px; border-bottom:1px solid #eee; font-size:0.85em;">${l.timestamp}</td>
            <td style="padding:12px; border-bottom:1px solid #eee; font-weight:600;">${l.user}</td>
            <td style="padding:12px; border-bottom:1px solid #eee; color:#666;">${l.ip}</td>
            <td style="padding:12px; border-bottom:1px solid #eee;"><span style="color:var(--accent-color); font-weight:700;">${l.action.toUpperCase()}</span></td>
            <td style="padding:12px; border-bottom:1px solid #eee;">${l.target}</td>
        </tr>
    `).join('');
}
