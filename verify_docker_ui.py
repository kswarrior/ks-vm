from playwright.sync_api import sync_playwright
import subprocess
import time
import os

def run_cuj(page):
    page.goto("http://localhost:4000")
    page.wait_for_timeout(2000)

    # Check Deploy VPS tab
    print("Checking Deploy VPS tab...")
    page.click('button:has-text("Deploy VPS")')
    page.wait_for_timeout(1000)

    options = page.locator('#deploy-type option').all_inner_texts()
    print(f"Deploy types: {options}")
    if "Docker Container" in options:
        print("SUCCESS: Docker Container found in deploy-type")
    else:
        print("FAIL: Docker Container NOT found in deploy-type")

    # Check Add Image tab
    print("Checking OS Images -> Add Image tab...")
    page.click('.nav-item[data-tab="images"]')
    page.wait_for_timeout(1000)
    page.click('button:has-text("Add Image")')
    page.wait_for_timeout(1000)

    img_options = page.locator('#img-type option').all_inner_texts()
    print(f"Image types: {img_options}")
    if "Docker Container" in img_options:
        print("SUCCESS: Docker Container found in img-type")
    else:
        print("FAIL: Docker Container NOT found in img-type")

    page.screenshot(path="/home/jules/verification/screenshots/docker_ui_check.png")

if __name__ == "__main__":
    env = os.environ.copy()
    env["LIBVIRT_DEFAULT_URI"] = "test:///default"
    env["KSVM_BG"] = "true"
    daemon = subprocess.Popen(["./ksvm", "daemon", "-P", "w:4000 m:4001", "--user", "admin", "--pass", "admin"], env=env)
    time.sleep(3)
    try:
        with sync_playwright() as p:
            browser = p.chromium.launch(headless=True)
            context = browser.new_context(http_credentials={"username": "admin", "password": "admin"})
            page = context.new_page()
            run_cuj(page)
            browser.close()
    finally:
        daemon.terminate()
