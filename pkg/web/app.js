let allImages = [];

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
        const activeNavItem = document.querySelector('.nav-item.active');
        if (activeNavItem && activeNavItem.dataset.tab === 'instances') fetchInstances();
    }, 4000);

    fetchInstances();
    preloadImages();

    setTimeout(() => {
        const splash = document.getElementById('splash');
        splash.style.opacity = '0';
        setTimeout(() => splash.style.visibility = 'hidden', 500);
    }, 1500);
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
        if (vm.Status === 'deploying') {
            const overlay = document.createElement('div');
            overlay.className = 'overlay';
            overlay.innerText = 'DEPLOYING...';
            card.appendChild(overlay);
        }
        const statusClass = `status-${vm.Status}`;

        const memUsed = (vm.MemoryUsage / 1024).toFixed(1);
        const memTotal = (vm.MemoryMB / 1024).toFixed(1);
        const diskUsed = (vm.DiskUsage / 1024 / 1024 / 1024).toFixed(1);

        card.innerHTML += `
            <div class="instance-header">
                <div class="instance-title">${vm.Name}</div>
                <div class="instance-status ${statusClass}">${vm.Status}</div>
            </div>

            <div style="margin-bottom:20px; display:flex; gap:12px; align-items:center;">
                <div style="font-size:0.75rem; font-weight:600; color:var(--primary); background:var(--primary-light); padding:2px 8px; border-radius:4px;">${vm.IPs && vm.IPs.length > 0 ? vm.IPs[0] : 'NO IP'}</div>
                <div style="font-size:0.75rem; color:var(--text-muted); font-weight:600;">${vm.Type.toUpperCase()}</div>
                <div style="font-size:0.75rem; color:var(--text-muted);">${vm.Image || 'DEFAULT'}</div>
            </div>

            <div class="stats-line">
                <div class="stat-item">
                    <svg viewBox="0 0 24 24"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zM11 7h2v2h-2zm0 4h2v6h-2z"/></svg>
                    ${vm.CPUs} vCPU
                </div>
                <div class="stat-item">
                    <svg viewBox="0 0 24 24"><path d="M2 17h20v2H2zm1.15-4.05L4 14h16l.85-1.05-1.2-1.5L18.8 12H5.2l-.85-.55zm1.3-1.6L4.5 12h15l.05-.65-1.2-1.5L17.5 10H6.5l-.85-.15zM7 2h10v2H7zm0 13h10v2H7z"/></svg>
                    ${memTotal} GB RAM
                </div>
                <div class="stat-item">
                    <svg viewBox="0 0 24 24"><path d="M19 5v14H5V5h14m0-2H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2z"/></svg>
                    ${diskUsed} GB Disk
                </div>
            </div>

            <div class="actions-menu">
                <button class="dots-btn" onclick="toggleDropdown(event, '${vm.Name}')">
                    <svg style="width:20px;height:20px;" viewBox="0 0 24 24"><path d="M12 16a2 2 0 110 4 2 2 0 010-4zm0-6a2 2 0 110 4 2 2 0 010-4zm0-6a2 2 0 110 4 2 2 0 010-4z"/></svg>
                </button>
                <div id="dropdown-${vm.Name}" class="dropdown">
                    <div class="dropdown-item" onclick="action('launch', '${vm.Name}')">START</div>
                    <div class="dropdown-item" onclick="action('stop', '${vm.Name}')">STOP</div>
                    <div class="dropdown-item" onclick="action('restart', '${vm.Name}')">RESTART</div>
                    <div class="dropdown-item" onclick="openEdit('${vm.Name}')">EDIT</div>
                    <div class="dropdown-item" style="color:var(--danger);" onclick="action('delete', '${vm.Name}')">DELETE</div>
                </div>
            </div>
        `;
        grid.appendChild(card);
    });
}

function toggleDropdown(e, name) {
    e.stopPropagation();
    document.querySelectorAll('.dropdown').forEach(d => {
        if (d.id !== 'dropdown-' + name) d.classList.remove('show');
    });
    document.getElementById('dropdown-' + name).classList.toggle('show');
}

window.onclick = () => {
    document.querySelectorAll('.dropdown').forEach(d => d.classList.remove('show'));
    document.getElementById('image-dropdown').style.display = 'none';
};

async function preloadImages() {
    const res = await fetch('/api/v1/images');
    allImages = await res.json();
}

function filterImages() {
    const query = document.getElementById('image-search').value.toLowerCase();
    const dd = document.getElementById('image-dropdown');
    const filtered = allImages.filter(img => img.Name.toLowerCase().includes(query));

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
    const res = await fetch('/api/v1/deploy', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    });
    if (res.ok) showTab('instances');
    else alert("Error: " + (await res.json()).error);
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
    await fetch(`/api/v1/${type}/${name}`, { method: type === 'delete' ? 'DELETE' : 'POST' });
    fetchInstances();
}

async function fetchImages() {
    const res = await fetch('/api/v1/images');
    allImages = await res.json();
    const list = document.getElementById('image-list');
    list.innerHTML = allImages.map(img => `
        <div class="card">
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:12px;">
                <h3 style="margin:0;">${img.Name}</h3>
                <span class="badge" style="color:var(--primary);">${img.Type || 'VM'}</span>
            </div>
            <p style="color:var(--text-muted); font-size:0.85rem; margin-bottom:20px;">Size: ${(img.Size / 1024 / 1024).toFixed(2)} MB</p>
            <div style="display:flex; gap:8px;">
                <button class="btn" onclick="renameImage('${img.Name}')" style="flex:1; font-size:0.7rem;">Rename</button>
                <button class="btn btn-danger" onclick="removeImage('${img.Name}')" style="flex:1; font-size:0.7rem;">Delete</button>
            </div>
        </div>
    `).join('');
}

function showAddImageForm(show = true) {
    document.getElementById('add-image-form').style.display = show ? 'block' : 'none';
}

async function addImage() {
    const card = document.getElementById('add-image-form');
    const overlay = document.createElement('div');
    overlay.className = 'overlay';
    overlay.innerText = 'DOWNLOADING...';
    card.appendChild(overlay);

    const data = {
        name: document.getElementById('img-name').value,
        url: document.getElementById('img-url').value,
        type: document.getElementById('img-type').value
    };
    const res = await fetch('/api/v1/images', { method: 'POST', body: JSON.stringify(data), headers: {'Content-Type': 'application/json'} });
    card.removeChild(overlay);
    if (res.ok) {
        showAddImageForm(false);
        fetchImages();
    } else alert("Error adding image");
}

async function removeImage(name) {
    if (!confirm(`Remove image ${name}?`)) return;
    await fetch(`/api/v1/images/${name}`, { method: 'DELETE' });
    fetchImages();
}

async function renameImage(name) {
    const newName = prompt("New name for " + name, name);
    if (!newName) return;
    await fetch('/api/v1/images/rename', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ old_name: name, new_name: newName }) });
    fetchImages();
}

async function fetchUsers() {
    const res = await fetch('/api/v1/users');
    const data = await res.json();
    const list = document.getElementById('user-list');
    list.innerHTML = data.map(u => `
        <div class="card">
            <div style="display:flex; align-items:center; gap:12px; margin-bottom:12px;">
                <div style="background:var(--primary-light); color:var(--primary); width:40px; height:40px; border-radius:20px; display:flex; align-items:center; justify-content:center; font-weight:700;">${u.username[0].toUpperCase()}</div>
                <div>
                    <h3 style="margin:0; font-size:1rem;">${u.username}</h3>
                    <div style="font-size:0.8rem; color:var(--text-muted);">${u.email}</div>
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
        <div class="card" style="padding:16px; border-left:4px solid var(--primary); display:flex; justify-content:space-between; align-items:center;">
            <div>
                <div style="font-size:0.7rem; color:var(--text-muted); margin-bottom:4px;">${l.timestamp} • IP: ${l.ip}</div>
                <div style="font-weight:600;"><span style="color:var(--primary);">${l.action.toUpperCase()}</span> on ${l.target}</div>
            </div>
            <div style="font-size:0.8rem; font-weight:700;">${l.user.toUpperCase()}</div>
        </div>
    `).join('');
}
