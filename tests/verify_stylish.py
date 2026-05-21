import asyncio
from playwright.async_api import async_playwright
import os

async def verify_ui():
    async with async_playwright() as p:
        # Create screenshots directory
        os.makedirs('/home/jules/verification/screenshots', exist_ok=True)

        # Desktop view
        browser = await p.chromium.launch()
        page = await browser.new_page(viewport={'width': 1280, 'height': 800})
        await page.goto('http://localhost:8080')

        # Capture splash screen if possible (briefly)
        await page.screenshot(path='/home/jules/verification/screenshots/splash_desktop.png')

        # Wait for splash to disappear
        await page.wait_for_selector('#splash', state='hidden', timeout=10000)

        # Wait for instances to load
        await asyncio.sleep(2)

        await page.screenshot(path='/home/jules/verification/screenshots/dashboard_desktop.png')
        print("Desktop screenshot captured.")

        # Mobile view
        iphone_13 = p.devices['iPhone 13']
        mobile_context = await browser.new_context(**iphone_13)
        mobile_page = await mobile_context.new_page()
        await mobile_page.goto('http://localhost:8080')

        await mobile_page.wait_for_selector('#splash', state='hidden', timeout=10000)
        await asyncio.sleep(2)

        await mobile_page.screenshot(path='/home/jules/verification/screenshots/dashboard_mobile.png')
        print("Mobile screenshot captured.")

        await browser.close()

if __name__ == "__main__":
    asyncio.run(verify_ui())
