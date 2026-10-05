// Optional full-browser check. Invoke with: bash .agent/run.sh node .agent/browser-test.cjs
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

if (!fs.existsSync('/.dockerenv')) throw new Error('Run this check through bash .agent/run.sh node .agent/browser-test.cjs');
if (!fs.existsSync('.agent/mock-api-server-linux')) throw new Error('Build first: bash .agent/run.sh go build -o .agent/mock-api-server-linux .');
const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'mock-browser-tools-'));
function run(command, args, env = process.env) {
  const result = spawnSync(command, args, { stdio: 'inherit', env });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`${command} failed (${result.status ?? result.signal})`);
}
try {
  run('npm', ['install', '--prefix', temp, '--no-audit', '--no-fund', 'playwright@1.58.2']);
  const playwright = path.join(temp, 'node_modules/playwright');
  run(process.execPath, [path.join(playwright, 'cli.js'), 'install', '--with-deps', 'chromium']);
  run(process.execPath, ['.agent/browser-smoke.cjs'], { ...process.env, PLAYWRIGHT_MODULE: playwright });
} finally {
  fs.rmSync(temp, { recursive: true, force: true });
}
