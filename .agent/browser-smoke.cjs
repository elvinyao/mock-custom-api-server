// Run only inside the Docker runner with Playwright installed in the container.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');

async function eventually(check, message) {
  const deadline = Date.now() + 15000;
  do {
    try { if (await check()) return; } catch {}
    await new Promise(resolve => setTimeout(resolve, 50));
  } while (Date.now() < deadline);
  throw new Error(message);
}

async function main() {
  const temp = await fs.mkdtemp(path.join(os.tmpdir(), 'mock-browser-'));
  const origin = 'http://127.0.0.1:18081';
  const configPath = path.join(temp, 'config.yaml');
  const fixturePath = path.join(temp, 'fixture.json');
  await fs.writeFile(fixturePath, 'version-one');
  const config = {
    port: 18081, hot_reload: false, logging: { level: 'info', access_log: false },
    admin_api: { enabled: true, prefix: '/dev-admin' },
    recording: { enabled: true, record_body: true, max_entries: 1000, max_body_bytes: 65536 },
    health_check: { enabled: true, path: '/health' },
    endpoints: [
      { path: '/fixture', method: 'GET', default: { response_file: fixturePath } },
      { path: '/echo/:id', method: 'POST', selectors: [
        { name: 'id', type: 'path', key: 'id' }, { name: 'kind', type: 'body', key: 'kind' },
      ], rules: [{ conditions: [{ selector: 'kind', match_type: 'exact', value: 'paid' }],
        inline_body: '{"id":"{{.id}}","kind":"{{.kind}}"}', template: { enabled: true },
      }], default: { inline_body: '{"unmatched":true}' } },
    ],
  };
  const writeConfig = () => fs.writeFile(configPath, JSON.stringify(config));
  await writeConfig();
  const server = spawn(path.resolve('.agent/mock-api-server-linux'), ['-config', configPath], { stdio: ['ignore', 'pipe', 'pipe'] });
  let logs = '';
  server.stdout.on('data', chunk => { logs += chunk; });
  server.stderr.on('data', chunk => { logs += chunk; });
  let browser;
  try {
    await eventually(async () => (await fetch(`${origin}/health`)).ok, 'server failed to start');
    assert.equal(await (await fetch(`${origin}/fixture`)).text(), 'version-one');
    await fs.writeFile(fixturePath, 'version-two');
    assert.equal(await (await fetch(`${origin}/fixture`)).text(), 'version-two');
    browser = await chromium.launch({ headless: true });
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, permissions: ['clipboard-read', 'clipboard-write'] });
    const external = [];
    const errors = [];
    await context.route('**/*', route => {
      const url = route.request().url();
      if (url.startsWith('http') && !url.startsWith(origin + '/')) {
        external.push(url); return route.abort();
      }
      return route.continue();
    });
    const page = await context.newPage();
    page.on('pageerror', err => errors.push(err.message));
    await page.goto(`${origin}/dev-admin/ui/`);
    await page.getByRole('cell', { name: '/fixture', exact: true }).waitFor();
    await page.getByRole('button', { name: 'Create endpoint', exact: true }).click();
    const editor = page.getByRole('dialog', { name: 'Endpoint editor' });
    await editor.getByLabel('Endpoint JSON', { exact: true }).fill('{"path":"","method":"GET"}');
    await editor.getByRole('button', { name: 'Save runtime endpoint' }).click();
    await editor.getByRole('alert').waitFor();
    assert.match(await editor.getByRole('alert').textContent(), /path/i);
    const runtime = { path: '/created', method: 'GET', default: { inline_body: 'created' } };
    await editor.getByLabel('Endpoint JSON', { exact: true }).fill(JSON.stringify(runtime));
    if (process.env.SMOKE_SCREENSHOT_PREFIX) await page.screenshot({ path: `${process.env.SMOKE_SCREENSHOT_PREFIX}-editor.png` });
    await editor.getByRole('button', { name: 'Save runtime endpoint' }).click();
    const row = page.locator('tr:visible').filter({ has: page.getByText('/created', { exact: true }) });
    await row.waitFor();
    assert.equal(await (await fetch(`${origin}/created`)).text(), 'created');
    await row.getByRole('button', { name: 'Edit', exact: true }).click();
    runtime.default.inline_body = 'updated';
    await editor.getByLabel('Endpoint JSON', { exact: true }).fill(JSON.stringify(runtime));
    await editor.getByRole('button', { name: 'Save runtime endpoint' }).click();
    await editor.waitFor({ state: 'hidden' });
    assert.equal(await (await fetch(`${origin}/created`)).text(), 'updated');
    const downloadPromise = page.waitForEvent('download');
    await row.getByRole('button', { name: 'Export YAML' }).click();
    const download = await downloadPromise;
    const yaml = await fs.readFile(await download.path(), 'utf8');
    assert.match(yaml, /path: \/created/);
    assert.match(yaml, /inline_body: updated/);
    await row.getByRole('button', { name: 'Delete', exact: true }).click();
    await row.waitFor({ state: 'hidden' });
    assert.equal((await fetch(`${origin}/created`)).status, 404);

    await fetch(`${origin}/echo/42?tag=a&tag=b`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{"kind":"paid"}' });
    for (let i = 0; i < 60; i++) await fetch(`${origin}/fixture?i=${i}`);
    await page.getByRole('button', { name: 'requests', exact: true }).click();
    await page.getByRole('button', { name: /^Load more/ }).click();
    await page.getByRole('button', { name: 'Resume live refresh' }).waitFor();
    await page.waitForFunction(() => Alpine.$data(document.body).requests.length > 50);
    const loaded = await page.evaluate(() => Alpine.$data(document.body).requests.length);
    assert.ok(loaded > 50);
    await page.waitForTimeout(5200);
    assert.equal(await page.evaluate(() => Alpine.$data(document.body).requests.length), loaded);
    await page.getByLabel('Filter path', { exact: true }).fill('/echo/42');
    const echoRow = page.locator('tr:visible').filter({ hasText: '/echo/42?tag=a&tag=b' });
    await echoRow.getByRole('button', { name: 'Inspect' }).click();
    await page.getByText('Why this response?', { exact: true }).waitFor();
    await page.getByRole('button', { name: 'Copy cURL' }).click();
    const curl = await page.evaluate(() => navigator.clipboard.readText());
    assert.match(curl, /echo\/42\?tag=a&tag=b/);
    assert.match(curl, /paid/);
    await page.getByRole('button', { name: 'Replay request' }).click();
    await page.getByText('Replay result — HTTP 200', { exact: true }).waitFor();
    const replay = await page.evaluate(() => Alpine.$data(document.body).replayResult);
    assert.deepEqual(JSON.parse(replay.body), { id: '42', kind: 'paid' });
    if (process.env.SMOKE_SCREENSHOT_PREFIX) await page.screenshot({ path: `${process.env.SMOKE_SCREENSHOT_PREFIX}-request.png` });
    await page.keyboard.press('Escape');

    await page.getByRole('button', { name: 'config', exact: true }).click();
    await fs.writeFile(configPath, 'endpoints: [');
    await page.getByRole('button', { name: 'Trigger Reload' }).click();
    await eventually(async () => /Reload failed/.test(await page.locator('[role=alert]').first().textContent()), 'reload error not shown');
    assert.equal(await (await fetch(`${origin}/fixture`)).text(), 'version-two');
    await writeConfig();
    await page.getByRole('button', { name: 'Trigger Reload' }).click();
    await page.getByText('Configuration reloaded', { exact: true }).waitFor();
    await page.goto(`${origin}/dev-admin/ui/swagger.html`);
    await page.locator('.swagger-ui .opblock').first().waitFor();
    const spec = await (await fetch(`${origin}/dev-admin/openapi.json`)).json();
    assert.equal(spec.servers[0].url, '/');
    assert.deepEqual(external, [], 'offline UI attempted external network requests');
    assert.deepEqual(errors, [], 'browser JavaScript errors');
    console.log('Browser smoke passed: fresh fixture, invalid/save/edit/export/delete endpoint, retained history, full-buffer filter, trace, cURL, replay, reload failure/recovery, offline Swagger.');
  } catch (error) {
    console.error(logs);
    throw error;
  } finally {
    if (browser) await browser.close();
    server.kill('SIGTERM');
    await new Promise(resolve => server.exitCode !== null ? resolve() : server.once('exit', resolve));
    await fs.rm(temp, { recursive: true, force: true });
  }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
