const {chromium}=require(process.cwd()+'/ui/node_modules/playwright');
const fs=require('fs');
(async()=>{
const credentials=JSON.parse(fs.readFileSync('.tmp/e13/public/credentials.json'));
const url=fs.readFileSync('.tmp/e13/public/url.txt','utf8').trim();
const browser=await chromium.launch({headless:true});
const context=await browser.newContext({httpCredentials:credentials,viewport:{width:1440,height:1000}});
const page=await context.newPage();
await page.goto(url+'/?mode=connected#/work/investigate-payment-retries');
await page.locator('main').waitFor();
await page.getByText('Succeeded',{exact:true}).filter({visible:true}).first().waitFor();
await page.screenshot({path:'work/0175-shared-eks-bedrock/evidence/https-portal.png',fullPage:true});
console.log('HTTPS Portal rendered: real Work and Succeeded status visible; TLS checks enabled.');
await browser.close();
})();
