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
        });
    });

    document.getElementById('menu-toggle').addEventListener('click', () => {
        document.getElementById('sidebar').classList.toggle('open');
    });

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
            <p>Status: ${vm.Status}</p>
            <p>IPs: ${vm.IPs ? vm.IPs.join(', ') : '-'}</p>
            <div class="actions">
                <button onclick="action('launch', '${vm.Name}')">START</button>
                <button onclick="action('stop', '${vm.Name}')">STOP</button>
                <button onclick="action('restart', '${vm.Name}')">RESTART</button>
                <button onclick="action('suspend', '${vm.Name}')">SUSPEND</button>
                <button onclick="action('resume', '${vm.Name}')">RESUME</button>
                <button onclick="editInstance('${vm.Name}')">EDIT</button>
                <button class="danger" onclick="action('delete', '${vm.Name}')">DELETE</button>
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

async function fetchLogs() {
    const res = await fetch('/api/v1/logs');
    const data = await res.json();
    const logContainer = document.getElementById('audit-log');
    logContainer.innerHTML = data.map(log => `<p>[${log.timestamp}] ${log.user}: ${log.action} ${log.target}</p>`).join('');
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
