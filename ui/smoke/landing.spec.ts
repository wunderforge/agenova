// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { expect, test } from '@playwright/test';

test('built public landing uses real login links and labelled product captures', async ({page},info) => {
  const apiRequests: string[]=[];
  page.on('request',request=>{if(new URL(request.url()).pathname.startsWith('/api/'))apiRequests.push(request.url());});
  await page.setViewportSize({width:1440,height:1000});
  await page.goto('/landing/');
  await expect(page.getByRole('heading',{level:1})).toHaveText('Give agents work.Keep control.');
  for(const label of ['Sign in','Sign in to your workspace','Open your workspace']) {
    await expect(page.getByRole('link',{name:label,exact:label==='Sign in'})).toHaveAttribute('href','/login');
  }
  await expect(page.getByText('Actual React UI · illustrative test data',{exact:true})).toBeVisible();
  expect(await page.locator('#product-image').evaluate((el:HTMLImageElement)=>el.complete&&el.naturalWidth>0)).toBe(true);
  await page.screenshot({path:info.outputPath('landing-desktop.png'),fullPage:true});
  await page.getByRole('tab',{name:'Work evidence',exact:true}).click();
  await expect(page.locator('#product-image')).toHaveAttribute('src','/landing/assets/work-evidence.png');
  await page.getByRole('button',{name:'Enlarge product screenshot'}).click();
  await expect(page.getByRole('dialog')).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await page.getByRole('tab',{name:'Work evidence',exact:true}).focus();
  await page.keyboard.press('ArrowLeft');
  await expect(page.getByRole('tab',{name:'Work explorer',exact:true})).toBeFocused();
  await page.emulateMedia({colorScheme:'dark',reducedMotion:'reduce'});
  await expect(page.locator('html')).toHaveCSS('color-scheme','light only');
  await page.setViewportSize({width:390,height:844});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await page.screenshot({path:info.outputPath('landing-mobile.png'),fullPage:true});
  expect(apiRequests).toEqual([]);
  expect(await page.evaluate(()=>localStorage.length+sessionStorage.length)).toBe(0);
});
