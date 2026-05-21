import asyncio
from playwright.async_api import async_playwright
import os
import base64

async def verify_new_features():
    async with async_playwright() as p:
        browser = await p.chromium.launch()
        # Using basic auth headers for playwright
        auth_header = base64.b64encode(b"admin:admin").decode("utf-8")
        context = await browser.new_context(extra_http_headers={"Authorization": f"Basic {auth_header}"})
        page = await context.new_page()

        await page.goto('http://localhost:8080')

        # Wait for dashboard to load (splash screen fix check)
        await page.wait_for_selector('#splash', state='hidden', timeout=10000)
        print("Dashboard loaded successfully.")

        # 1. Test docker:// image registration
        await page.click('.nav-item[data-tab="images"]')
        await page.click('button:has-text("+ Add Image")')
        await page.fill('#img-name', 'nginx-container')
        await page.fill('#img-url', 'docker://nginx:alpine')
        await page.click('button:has-text("Register")')

        # Wait for image to appear in list
        await page.wait_for_selector('h3:has-text("nginx-container")')
        print("docker:// image registered successfully.")

        # 2. Test SSH button visibility and action
        await page.click('.nav-item[data-tab="instances"]')
        # Wait for instance cards
        await page.wait_for_selector('.card')

        # Open dropdown for 'test' instance
        await page.click('.dots-btn')

        # Check if SSH button exists
        ssh_btn = await page.query_selector('.dropdown-item:has-text("SSH")')
        if ssh_btn:
            print("SSH button found in dropdown.")

            # Catch the alert
            page.on("dialog", lambda dialog: print(f"Alert shown: {dialog.message}") or dialog.dismiss())
            await ssh_btn.click()
        else:
            print("SSH button NOT found (Instance might not be 'running'). Status was:", await page.inner_text('.instance-status'))

        await browser.close()

if __name__ == "__main__":
    asyncio.run(verify_new_features())
