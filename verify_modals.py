
import asyncio
from playwright.async_api import async_playwright
import os

async def run():
    async with async_playwright() as p:
        browser = await p.chromium.launch()
        # iPhone 12 Pro Max viewport
        context = await browser.new_context(
            viewport={'width': 428, 'height': 926},
            is_mobile=True,
            has_touch=True,
            user_agent="Mozilla/5.0 (iPhone; CPU iPhone OS 14_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0.3 Mobile/15E148 Safari/604.1"
        )
        page = await context.new_page()

        # Login
        await page.goto('http://admin:admin@localhost:8080')
        await asyncio.sleep(2)

        # Open Deploy Modal
        await page.click('button:has-text("+ Deploy VPS")')
        await asyncio.sleep(1)
        await page.screenshot(path='/home/jules/verification/screenshots/mobile_deploy_modal.png')

        # Close modal
        await page.click('.close-modal')
        await asyncio.sleep(0.5)

        # Open Images tab
        await page.click('a[data-tab="images"]')
        await asyncio.sleep(1)

        # Open Add Image Modal
        await page.click('button:has-text("+ Add Image")')
        await asyncio.sleep(1)
        await page.screenshot(path='/home/jules/verification/screenshots/mobile_add_image_modal.png')

        await browser.close()

if __name__ == "__main__":
    asyncio.run(run())
