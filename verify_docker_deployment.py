from playwright.sync_api import sync_playwright
import subprocess
import time
import os

def run_cuj(page):
    page.goto("http://localhost:4000")
    page.wait_for_timeout(2000)

    print("Deploying Docker container...")
    page.click('button:has-text("Deploy VPS")')
    page.fill('#deploy-name', 'docker-test')
    page.select_option('#deploy-type', 'docker')
    # Since we don't have images in pool, note said we can type name
    page.fill('#image-search', 'hello-world')

    page.click('button:has-text("Deploy Now")')
    page.wait_for_timeout(5000) # Wait for pull/create/start

    print("Checking instance list...")
    page.click('.nav-item[data-tab="instances"]')
    page.wait_for_timeout(2000)

    content = page.locator('#instance-list').inner_text()
    print(f"Instance list content: {content}")

    if "docker-test" in content:
        print("SUCCESS: docker-test found in list")
        if "INITIATING" in content:
            print("FAIL: Still showing INITIATING")
        else:
            print("SUCCESS: INITIATING overlay removed")
    else:
        print("FAIL: docker-test NOT found in list")

    page.screenshot(path="/home/jules/verification/screenshots/docker_deployment_check.png")

if __name__ == "__main__":
    env = os.environ.copy()
    env["LIBVIRT_DEFAULT_URI"] = "test:///default"
    env["KSVM_BG"] = "true"
    # Ensure KSVM_BASE_DIR is somewhere writable
    env["KSVM_BASE_DIR"] = "/tmp/ksvm_test"
    os.makedirs("/tmp/ksvm_test/instances", exist_ok=True)

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
