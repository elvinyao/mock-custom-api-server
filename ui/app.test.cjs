const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

function create(fetcher = async () => new Response('{}')) {
  const context = { URL, URLSearchParams, Blob, fetch: fetcher, console,
    window: { location: { pathname: '/admin/ui/', origin: 'https://mock.example' } },
    navigator: { clipboard: { writeText: async () => {} } },
    setInterval: () => 1, clearInterval() {}, setTimeout: () => 1,
  };
  const file = `${__dirname}/static/app.js`;
  const source = fs.existsSync(file) ? fs.readFileSync(file, 'utf8') :
    fs.readFileSync(`${__dirname}/static/index.html`, 'utf8').match(/<script>\s*([\s\S]*?)<\/script>/)[1];
  vm.createContext(context);
  vm.runInContext(source, context);
  return context.dashboard();
}
const json = (data, status = 200) => new Response(JSON.stringify(data), { status });
function deferred() {
  let resolve;
  const promise = new Promise(value => { resolve = value; });
  return { promise, resolve };
}

test('reload failure reports server error and preserves config without refreshing', async () => {
  const urls = [];
  const app = create(async url => { urls.push(url); return json({ error: 'invalid regex' }, 400); });
  app.configJson = '{"old":true}';
  await app.triggerReload();
  assert.match(app.error || '', /invalid regex/);
  assert.equal(app.configJson, '{"old":true}');
  assert.equal(app.reloadMsg, '');
  assert.equal(urls.length, 1);
});
test('clear failure preserves visible history and reports status for non-JSON errors', async () => {
  const app = create(async () => new Response('Unauthorized', { status: 401 }));
  app.requests = [{ id: 'keep' }];
  await app.clearRequests();
  assert.equal(app.requests.length, 1);
  assert.match(app.error || '', /401|Unauthorized/);
});
test('filter values are sent to the full recorder API', async () => {
  let url;
  const app = create(async value => { url = value; return json({ requests: [], total: 0 }); });
  app.reqFilter = { method: 'POST', status: '4', path: '/old path' };
  await app.fetchRequests();
  const params = new URL(url, 'https://mock.example').searchParams;
  assert.equal(params.get('method'), 'POST');
  assert.equal(params.get('status'), '4');
  assert.equal(params.get('path'), '/old path');
});
test('load more pauses refresh and tab changes retain loaded pages', async () => {
  const app = create(async () => json({ requests: [{ id: '2' }], total: 3 }));
  app.requests = [{ id: '1' }]; app.requestsOffset = 1;
  await app.loadMoreRequests();
  assert.equal(app.requestsPaused, true);
  await app.switchTab('requests');
  assert.equal(app.requests.length, 2);
  assert.equal(app.requestsPaused, true);
});
test('manual refresh retains page depth and remains paused', async () => {
  let url;
  const app = create(async value => { url = value; return json({ requests: [], total: 0 }); });
  app.requests = Array.from({ length: 100 }, (_, id) => ({ id }));
  app.requestsPaused = true;
  await app.fetchRequests();
  assert.equal(new URL(url, 'https://mock.example').searchParams.get('limit'), '100');
  assert.equal(app.requestsPaused, true);
});
test('pausing ignores a live refresh that was already in flight', async () => {
  let complete;
  const app = create(() => new Promise(resolve => { complete = resolve; }));
  app.requests = [{ id: 'keep' }];
  const refresh = app.fetchRequests();
  await app.toggleRequestsPaused();
  complete(json({ requests: [{ id: 'new' }], total: 1 }));
  await refresh;
  assert.equal(app.requests[0].id, 'keep');
  assert.equal(app.requestsLoading, false);
});
test('configuration highlighting escapes HTML and retains syntax colors', () => {
  const app = create();
  const html = app.highlightJson('{"message":"<script>danger</script>","enabled":true}');
  assert.doesNotMatch(html, /<script>/);
  assert.match(html, /&lt;script&gt;/);
  assert.match(html, /text-green-700/);
});
test('older request pagination ends when server offset is exhausted after deduplication', () => {
  const app = create();
  app.requests = Array.from({ length: 100 }, (_, id) => ({ id }));
  app.requestsTotal = 101; app.requestsOffset = 101;
  assert.equal(app.canLoadMoreRequests, false);
});
test('replay preserves method query body and filters browser-controlled headers', async () => {
  let request;
  const app = create(async (url, options) => { request = { url, options }; return new Response('reply', { status: 201 }); });
  app.selectedRequest = { method: 'POST', path: '/orders', query: 'tag=a&tag=b', request_body: '{"x":1}',
    request_body_recorded: true, request_body_truncated: false,
    request_headers: { 'Content-Type': ['application/json'], Host: ['old'], Cookie: ['secret'], Connection: ['X-Hop'], 'X-Hop': ['no'], 'Sec-Fetch-Site': ['same-origin'], 'X-Test': ['yes'] } };
  assert.equal(typeof app.replayRequest, 'function');
  await app.replayRequest();
  assert.equal(request.url, 'https://mock.example/orders?tag=a&tag=b');
  assert.equal(request.options.method, 'POST');
  assert.equal(request.options.body, '{"x":1}');
  assert.equal(request.options.headers['X-Test'], 'yes');
  for (const name of ['Host', 'Cookie', 'Connection', 'X-Hop', 'Sec-Fetch-Site']) assert.equal(request.options.headers[name], undefined);
  assert.equal(request.options.redirect, 'error');
  assert.equal(app.replayResult.status, 201);
  assert.equal(app.replayResult.body, 'reply');
});
test('replay refuses missing or truncated body and admin or external targets', async () => {
  let calls = 0;
  const app = create(async () => { calls++; return json({}); });
  assert.equal(typeof app.replayRequest, 'function');
  for (const overrides of [ { request_body_recorded: false }, { request_body_truncated: true },
    { path: '/admin/config/reload' }, { path: '/%61dmin/config/reload' }, { path: '//evil.example/x' } ]) {
    app.error = '';
    app.selectedRequest = { method: 'POST', path: '/orders', request_body: '', request_body_recorded: true, ...overrides };
    await app.replayRequest();
    assert.ok(app.error);
  }
  assert.equal(calls, 0);
});
test('recorded empty POST body can be replayed', async () => {
  let body;
  const app = create(async (_url, options) => { body = options.body; return json({}); });
  assert.equal(typeof app.replayRequest, 'function');
  app.selectedRequest = { method: 'POST', path: '/orders', request_body_recorded: true };
  await app.replayRequest();
  assert.equal(body, '');
});
test('an older replay cannot overwrite the selected request or stop its pending replay', async () => {
  const a = deferred(), b = deferred();
  const requestA = { id: 'a', method: 'GET', path: '/a', request_body_recorded: true };
  const requestB = { id: 'b', method: 'GET', path: '/b', request_body_recorded: true };
  const app = create(url => url === 'https://mock.example/a' ? a.promise :
    url === 'https://mock.example/b' ? b.promise : Promise.resolve(json(requestB)));
  app.selectedRequest = requestA; app.showModal = true;
  const first = app.replayRequest();
  await app.inspectRequest(requestB);
  const second = app.replayRequest();
  a.resolve(new Response('old request response', { status: 201 }));
  await first;
  assert.equal(app.replayResult, null);
  assert.equal(app.replaying, true);
  b.resolve(new Response('current request response', { status: 202 }));
  await second;
  assert.equal(app.replayResult.body, 'current request response');
});
test('closing request details invalidates a pending replay failure', async () => {
  const response = deferred();
  const app = create(() => response.promise.then(() => { throw new Error('late network failure'); }));
  app.selectedRequest = { method: 'GET', path: '/a', request_body_recorded: true }; app.showModal = true;
  const replay = app.replayRequest();
  assert.equal(typeof app.closeRequest, 'function');
  app.closeRequest();
  response.resolve();
  await replay;
  assert.equal(app.error, '');
  assert.equal(app.selectedRequest, null);
  assert.equal(app.replaying, false);
});
test('a late request detail response cannot replace a newer selection', async () => {
  const a = deferred(), b = deferred();
  const app = create(url => url.endsWith('/a') ? a.promise : b.promise);
  const first = app.inspectRequest({ id: 'a', path: '/a' });
  const second = app.inspectRequest({ id: 'b', path: '/b' });
  b.resolve(json({ id: 'b', path: '/b', request_body: 'current' }));
  await second;
  a.resolve(json({ id: 'a', path: '/a', request_body: 'old' }));
  await first;
  assert.equal(app.selectedRequest.id, 'b');
  assert.equal(app.selectedRequest.request_body, 'current');
});
test('curl copy shell-quotes body and includes method query safe headers', async () => {
  const app = create();
  assert.equal(typeof app.requestCurl, 'function');
  const curl = app.requestCurl({ method: 'POST', path: '/echo', query: 'q=x', request_body_recorded: true,
    request_body: "it's ok", request_headers: { 'Content-Type': ['text/plain'] } });
  assert.match(curl, /curl --globoff -X 'POST'/);
  assert.match(curl, /https:\/\/mock.example\/echo\?q=x/);
  assert.match(curl, /--data-raw 'it'"'"'s ok'/);
});
test('curl copy disables URL globbing for bracketed and braced query values', () => {
  const app = create();
  for (const query of ['filter[id]=1', 'ids={1,2}']) {
    const curl = app.requestCurl({ method: 'GET', path: '/items', query, request_body_recorded: true });
    assert.match(curl, /(?:^|\s)--globoff(?:\s|$)/);
    assert.ok(curl.includes(`'https://mock.example/items?${query}'`));
  }
});
test('runtime editor creates and updates while file endpoint editing is refused', async () => {
  const calls = [];
  const app = create(async (url, options) => { calls.push({ url, options }); return json({ endpoints: [] }); });
  assert.equal(typeof app.openEndpointEditor, 'function');
  app.openEndpointEditor();
  app.endpointJson = '{"path":"/new","method":"GET","default":{"inline_body":"hello"}}';
  await app.saveEndpoint();
  assert.equal(calls[0].options.method, 'POST');
  app.openEndpointEditor({ source: 'runtime', index: 2, endpoint: { path: '/old', method: 'GET' } });
  await app.saveEndpoint();
  assert.equal(calls[2].url, '/admin/endpoints/2');
  assert.equal(calls[2].options.method, 'PUT');
  app.openEndpointEditor({ source: 'file', index: 0, endpoint: { path: '/file', method: 'GET' } });
  assert.match(app.error, /file/i);
});
test('duplicating an endpoint deep copies all fields into a create draft', () => {
  const app = create();
  assert.equal(typeof app.openEndpointEditor, 'function');
  const endpoint = { path: '/sample', method: 'POST', selectors: [{ name: 'id', type: 'query', key: 'id' }],
    rules: [{ inline_body: 'ok', condition_groups: [{ logic: 'OR', conditions: [] }] }], proxy: { target: 'http://local' } };
  app.openEndpointEditor({ source: 'file', index: 0, endpoint }, true);
  assert.deepEqual(JSON.parse(app.endpointJson), endpoint);
  assert.equal(app.editingEndpointIndex, null);
});
test('failed saves leave editor open with its JSON and show error', async () => {
  const app = create(async () => json({ error: 'unknown selector' }, 400));
  assert.equal(typeof app.openEndpointEditor, 'function');
  app.openEndpointEditor();
  app.endpointJson = '{"method":"GET","path":"/test"}';
  await app.saveEndpoint();
  assert.equal(app.showEndpointEditor, true);
  assert.match(app.error, /unknown selector/);
  assert.match(app.endpointJson, /test/);
});
test('failed delete leaves endpoint list intact and shows error', async () => {
  const app = create(async () => json({ error: 'not found' }, 404));
  assert.equal(typeof app.deleteEndpoint, 'function');
  const ep = { source: 'runtime', index: 0 };
  app.endpoints = [ep];
  await app.deleteEndpoint(ep);
  assert.equal(app.endpoints.length, 1);
  assert.match(app.error, /not found/);
});
test('endpoint deletes stay serialized until refreshed indexes arrive', async () => {
  const deletion = deferred(), refresh = deferred(), refreshStarted = deferred();
  const writes = [];
  const app = create((url, options) => {
    if (options?.method === 'DELETE') {
      writes.push(url);
      return writes.length === 1 ? deletion.promise : Promise.resolve(json({}));
    }
    refreshStarted.resolve();
    return refresh.promise;
  });
  const a = { source: 'runtime', index: 0, endpoint: { path: '/a', method: 'GET' } };
  const b = { source: 'runtime', index: 1, endpoint: { path: '/b', method: 'GET' } };
  app.endpoints = [a, b];
  const first = app.deleteEndpoint(a);
  const overlapping = app.deleteEndpoint(b);
  assert.equal(writes.length, 1);
  await overlapping;
  deletion.resolve(json({}));
  await refreshStarted.promise;
  await app.deleteEndpoint(b);
  assert.equal(writes.length, 1);
  refresh.resolve(json({ endpoints: [{ ...b, index: 0 }] }));
  await first;
  assert.equal(app.canMutateEndpoints, true);
});
test('failed endpoint refresh blocks stale index mutations until a successful refresh', async () => {
  let refreshFails = true;
  const writes = [];
  const b = { source: 'runtime', index: 0, endpoint: { path: '/b', method: 'GET' } };
  const app = create(async (url, options) => {
    if (options?.method === 'DELETE') { writes.push(url); return json({}); }
    return refreshFails ? json({ error: 'refresh unavailable' }, 503) : json({ endpoints: [b] });
  });
  app.endpoints = [{ source: 'runtime', index: 0 }, { source: 'runtime', index: 1 }];
  await app.deleteEndpoint(app.endpoints[0]);
  await app.deleteEndpoint(app.endpoints[1]);
  assert.equal(writes.length, 1);
  assert.equal(app.canMutateEndpoints, false);
  refreshFails = false;
  await app.fetchEndpoints();
  assert.equal(app.canMutateEndpoints, true);
});
test('an editor opened before indexes changed refuses to save to its stale index', async () => {
  const writes = [];
  const b = { source: 'runtime', index: 1, endpoint: { path: '/b', method: 'GET' } };
  const app = create(async (_url, options) => {
    if (options?.method === 'PUT') writes.push(options.body);
    return json({ endpoints: [{ ...b, index: 0 }] });
  });
  app.endpoints = [{ source: 'runtime', index: 0, endpoint: { path: '/a', method: 'GET' } }, b];
  app.openEndpointEditor(b);
  await app.fetchEndpoints();
  await app.saveEndpoint();
  assert.equal(writes.length, 0);
  assert.equal(app.showEndpointEditor, true);
  assert.match(app.error, /changed|reopen/i);
});
test('repeated save clicks create a runtime endpoint only once', async () => {
  const saved = deferred();
  const writes = [];
  const app = create((url, options) => {
    if (options?.method === 'POST') { writes.push(url); return saved.promise; }
    return Promise.resolve(json({ endpoints: [] }));
  });
  app.openEndpointEditor();
  const first = app.saveEndpoint();
  const second = app.saveEndpoint();
  assert.equal(writes.length, 1);
  saved.resolve(json({}));
  await Promise.all([first, second]);
});
test('failed YAML export shows server error', async () => {
  const app = create(async () => json({ error: 'invalid endpoint' }, 400));
  assert.equal(typeof app.exportEndpoint, 'function');
  app.endpointJson = '{}';
  await app.exportEndpoint();
  assert.match(app.error, /invalid endpoint/);
});
test('Swagger page uses embedded pinned assets without an online validator', () => {
  const page = fs.readFileSync(`${__dirname}/static/swagger.html`, 'utf8');
  assert.doesNotMatch(page, /https:\/\/unpkg/);
  assert.match(page, /validatorUrl:\s*null/);
  assert.ok(fs.existsSync(`${__dirname}/static/vendor/swagger-ui-LICENSE`));
});
