from playwright.sync_api import sync_playwright
import time
import os
import base64

def run_cuj(page):
    # Authenticate via Basic Auth (mock credentials)
    auth = base64.b64encode(b"admin:admin").decode("utf-8")
    page.set_extra_http_headers({"Authorization": f"Basic {auth}"})

    # Load the dashboard
    page.goto("http://localhost:8080")
    page.wait_for_function('document.getElementById("splash").style.visibility === "hidden"')

    # 1. Dashboard / Instances
    page.screenshot(path="/home/jules/verification/screenshots/mobile_instances.png")

    # Open Sidebar
    page.click('#menu-toggle')
    page.wait_for_timeout(500)
    page.screenshot(path="/home/jules/verification/screenshots/mobile_sidebar.png")

    # 2. System Status
    page.click('.nav-item[data-tab="system"]')
    page.wait_for_timeout(1000)
    page.screenshot(path="/home/jules/verification/screenshots/mobile_system.png")

    # 3. OS Images
    page.click('#menu-toggle')
    page.wait_for_timeout(500)
    page.click('.nav-item[data-tab="images"]')
    page.wait_for_timeout(1000)
    page.screenshot(path="/home/jules/verification/screenshots/mobile_images.png")

    # 4. Users
    page.click('#menu-toggle')
    page.wait_for_timeout(500)
    page.click('.nav-item[data-tab="users"]')
    page.wait_for_timeout(1000)
    page.screenshot(path="/home/jules/verification/screenshots/mobile_users.png")

if __name__ == "__main__":
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        # iPhone SE viewport
        context = browser.new_context(
            viewport={'width': 375, 'height': 667},
            user_agent='Mozilla/5.0 (iPhone; CPU iPhone OS 14_8 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.2 Mobile/15E148 Safari/604.1'
        )
        page = context.new_page()
        try:
            run_cuj(page)
        finally:
            context.close()
            browser.close()
