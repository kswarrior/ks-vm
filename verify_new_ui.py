
import asyncio
from playwright.async_api import async_playwright
import os

async def run():
    async with async_playwright() as p:
        browser = await p.chromium.launch()
        context = await browser.new_context(viewport={'width': 1280, 'height': 800})
        page = await context.new_page()

        # Mocking or connecting to a live instance if possible
        # For this verification, we use a mock dashboard with the new styles
        await page.goto('http://admin:admin@localhost:8080')
        await asyncio.sleep(2)

        # Take desktop screenshot
        await page.screenshot(path='/home/jules/verification/screenshots/desktop_new_layout.png')

        # Emulate Mobile
        mobile_context = await browser.new_context(
            viewport={'width': 390, 'height': 844},
            is_mobile=True,
            has_touch=True,
            user_agent="Mozilla/5.0 (iPhone; CPU iPhone OS 14_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0.3 Mobile/15E148 Safari/604.1"
        )
        mobile_page = await mobile_context.new_page()
        await mobile_page.goto('http://admin:admin@localhost:8080')
        await asyncio.sleep(2)
        await mobile_page.screenshot(path='/home/jules/verification/screenshots/mobile_new_layout.png')

        await browser.close()

if __name__ == "__main__":
    asyncio.run(run())
