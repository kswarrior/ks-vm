const { chromium } = require('playwright');
const path = require('path');

(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage();

  // Start daemon in background with mock data
  const { exec } = require('child_process');
  const daemon = exec('LIBVIRT_DEFAULT_URI=test:///default KSVM_BG=true ./ksvm daemon --user admin --pass admin');

  await new Promise(resolve => setTimeout(resolve, 2000)); // Wait for server

  try {
    await page.goto('http://localhost:4000');
    await page.fill('input[type="text"]', 'admin');
    await page.fill('input[type="password"]', 'admin');
    await page.click('button:has-text("Login")');

    await page.waitForSelector('.instance-card');

    // Click RUN CODE on first instance
    const firstInstance = page.locator('.instance-card').first();
    await firstInstance.locator('.btn-action-menu').click();
    await page.locator('.menu-item:has-text("RUN CODE")').click();

    await page.waitForSelector('.xterm-rows');

    // Wait for the prompt
    await page.waitForFunction(() => {
        const rows = document.querySelector('.xterm-rows');
        return rows && rows.innerText.includes('root@ks:~#');
    });

    await page.screenshot({ path: 'terminal_v2.png', fullPage: true });
    console.log('Terminal prompt verified and screenshot saved.');

  } catch (e) {
    console.error('Verification failed:', e);
    process.exit(1);
  } finally {
    await browser.close();
    daemon.kill();
  }
})();
