/* Dashboard state is shared by Alpine and the Node behavior tests. */
function dashboard() {
  const base = window.location.pathname.replace(/\/ui(?:\/.*)?$/, '');
  return {
    base, tabs: ['endpoints', 'requests', 'metrics', 'config'], activeTab: 'endpoints',
    error: '', notice: '', endpoints: [], selectedEndpoint: null, showEndpointModal: false,
    showEndpointEditor: false, endpointJson: '', editingEndpointIndex: null, editingEndpointGeneration: 0,
    endpointMutationPending: false, endpointsStale: false, endpointsGeneration: 0, endpointsFetchEpoch: 0,
    requests: [], requestsTotal: 0, requestsOffset: 0, requestsLimit: 50,
    requestsPaused: false, requestsEpoch: 0, requestsLoading: false,
    reqFilter: { method: '', status: '', path: '' }, selectedRequest: null, showModal: false,
    replayResult: null, replaying: false, requestDetailEpoch: 0, replayEpoch: 0,
    metricsData: [], uptimeStr: '—', configJson: '', reloadMsg: '', timers: [],

    get filteredRequests() { return this.requests; },
    get canLoadMoreRequests() { return this.requestsOffset < this.requestsTotal; },
    get canMutateEndpoints() { return !this.endpointMutationPending && !this.endpointsStale; },
    get endpointEditorStale() { return this.editingEndpointIndex !== null && this.editingEndpointGeneration !== this.endpointsGeneration; },
    async response(url, options) {
      const r = await fetch(url, options);
      if (!r.ok) {
        const body = await r.text();
        let message = body;
        try { const data = JSON.parse(body); message = data.error || data.message || body; } catch {}
        const error = new Error(message || `HTTP ${r.status} ${r.statusText}`);
        error.status = r.status;
        throw error;
      }
      return r;
    },
    async api(path, options) { return (await this.response(`${base}${path}`, options)).json(); },
    async init() {
      await this.fetchAll();
      this.timers.push(setInterval(() => this.fetchHealth(), 10000));
      this.timers.push(setInterval(() => {
        if (this.activeTab === 'requests' && !this.requestsPaused && !this.requestsLoading) this.fetchRequests();
        else if (this.activeTab === 'metrics') this.fetchMetrics();
      }, 5000));
    },
    destroy() { this.timers.forEach(clearInterval); },
    async switchTab(tab) {
      this.activeTab = tab;
      if (tab === 'requests' && !this.requestsPaused) await this.fetchRequests();
      else if (tab === 'endpoints') await this.fetchEndpoints();
      else if (tab === 'metrics') await this.fetchMetrics();
      else if (tab === 'config') await this.fetchConfig();
    },
    async fetchAll() { await Promise.all([this.fetchEndpoints(), this.fetchRequests(), this.fetchMetrics(), this.fetchConfig(), this.fetchHealth()]); },
    async fetchHealth() {
      try {
        const d = await this.api('/health');
        const secs = Math.round(d.uptime_sec || 0);
        this.uptimeStr = `${Math.floor(secs / 3600)}h ${Math.floor((secs % 3600) / 60)}m ${secs % 60}s`;
      } catch (e) { this.error = `Health: ${e.message}`; }
    },
    async fetchEndpoints(afterMutation = false) {
      if (this.endpointMutationPending && !afterMutation) return false;
      const epoch = ++this.endpointsFetchEpoch;
      try {
        const d = await this.api('/endpoints');
        if (epoch !== this.endpointsFetchEpoch) return false;
        const endpoints = d.endpoints || [];
        if (JSON.stringify(endpoints) !== JSON.stringify(this.endpoints)) ++this.endpointsGeneration;
        this.endpoints = endpoints;
        this.endpointsStale = false;
        return true;
      } catch (e) {
        if (epoch === this.endpointsFetchEpoch) { this.endpointsStale = true; this.error = `Endpoints: ${e.message}`; }
        return false;
      }
    },
    requestQuery(limit, offset) { return new URLSearchParams({ limit, offset, ...this.reqFilter }).toString(); },
    async fetchRequests(reset = false) {
      const epoch = ++this.requestsEpoch;
      this.requestsLoading = true;
      try {
        const limit = reset ? this.requestsLimit : Math.max(this.requestsLimit, this.requests.length);
        const d = await this.api(`/requests?${this.requestQuery(limit, 0)}`);
        if (epoch !== this.requestsEpoch) return;
        this.requests = d.requests || []; this.requestsTotal = d.total || 0;
        this.requestsOffset = this.requests.length;
      } catch (e) { this.error = `Requests: ${e.message}`; }
      finally { if (epoch === this.requestsEpoch) this.requestsLoading = false; }
    },
    async loadMoreRequests() {
      this.requestsPaused = true;
      const epoch = ++this.requestsEpoch;
      this.requestsLoading = true;
      try {
        const d = await this.api(`/requests?${this.requestQuery(this.requestsLimit, this.requestsOffset)}`);
        if (epoch !== this.requestsEpoch) return;
        const seen = new Set(this.requests.map(r => r.id));
        this.requests = [...this.requests, ...(d.requests || []).filter(r => !seen.has(r.id))];
        this.requestsTotal = d.total || 0; this.requestsOffset += (d.requests || []).length;
      } catch (e) { this.error = `Requests: ${e.message}`; }
      finally { if (epoch === this.requestsEpoch) this.requestsLoading = false; }
    },
    async toggleRequestsPaused() {
      this.requestsPaused = !this.requestsPaused;
      if (this.requestsPaused) { ++this.requestsEpoch; this.requestsLoading = false; }
      else await this.fetchRequests();
    },
    async clearFilters() { this.reqFilter = { method: '', status: '', path: '' }; await this.fetchRequests(true); },
    async clearRequests() {
      this.error = '';
      try {
        await this.api('/requests', { method: 'DELETE' });
        ++this.requestsEpoch; this.requestsLoading = false;
        this.requests = []; this.requestsTotal = 0; this.requestsOffset = 0; this.selectedRequest = null;
      } catch (e) { this.error = `Clear failed: ${e.message}`; }
    },
    async inspectRequest(req) {
      const epoch = ++this.requestDetailEpoch;
      ++this.replayEpoch;
      this.replayResult = null; this.replaying = false; this.error = '';
      this.selectedRequest = req; this.showModal = true;
      try {
        const detail = await this.api(`/requests/${encodeURIComponent(req.id)}`);
        if (epoch === this.requestDetailEpoch) this.selectedRequest = detail;
      } catch (e) { if (epoch === this.requestDetailEpoch) this.error = `Request detail: ${e.message}`; }
    },
    closeRequest() {
      ++this.requestDetailEpoch; ++this.replayEpoch;
      this.showModal = false; this.selectedRequest = null; this.replayResult = null; this.replaying = false;
    },
    async fetchMetrics() {
      try { const d = await this.api('/metrics'); this.metricsData = d.endpoints || []; }
      catch (e) { this.error = `Metrics: ${e.message}`; }
    },
    async fetchConfig() {
      try { this.configJson = JSON.stringify(await this.api('/config'), null, 2); }
      catch (e) { this.error = `Configuration: ${e.message}`; }
    },
    async triggerReload() {
      this.error = ''; this.reloadMsg = '';
      try {
        const d = await this.api('/config/reload', { method: 'POST' });
        this.reloadMsg = d.message || 'Reloaded';
        await this.fetchConfig(); await this.fetchEndpoints();
      } catch (e) { this.error = `Reload failed: ${e.message}`; }
    },
    openEndpointEditor(entry = null, duplicate = false) {
      if (this.endpointMutationPending) return;
      if (this.endpointsStale) { this.error = 'Refresh endpoints before making changes.'; return; }
      this.error = '';
      if (entry && entry.source !== 'runtime' && !duplicate) {
        this.error = 'File endpoints are read-only. Duplicate to create a runtime endpoint.'; return;
      }
      this.editingEndpointIndex = entry && !duplicate ? entry.index : null;
      this.editingEndpointGeneration = this.endpointsGeneration;
      this.endpointJson = JSON.stringify(entry ? entry.endpoint : {
        method: 'GET', path: '/example', description: '', selectors: [], rules: [],
        default: { status_code: 200, content_type: 'application/json', inline_body: '{"message":"hello"}' },
      }, null, 2);
      this.showEndpointModal = false; this.showEndpointEditor = true;
    },
    async saveEndpoint() {
      if (this.endpointMutationPending) return;
      if (this.endpointsStale) { this.error = 'Refresh endpoints before making changes.'; return; }
      if (this.endpointEditorStale) { this.error = 'The endpoint list changed. Reopen the endpoint before saving; your draft can still be exported.'; return; }
      this.error = '';
      let sent = false;
      try {
        const endpoint = JSON.parse(this.endpointJson);
        const path = this.editingEndpointIndex === null ? '/endpoints' : `/endpoints/${this.editingEndpointIndex}`;
        this.endpointMutationPending = true;
        ++this.endpointsFetchEpoch;
        sent = true;
        await this.api(path, { method: this.editingEndpointIndex === null ? 'POST' : 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(endpoint) });
        this.endpointsStale = true;
        this.showEndpointEditor = false; this.notice = 'Runtime endpoint saved. Export YAML to keep it after a restart.';
        await this.fetchEndpoints(true);
      } catch (e) {
        if (sent && e.status !== 400) this.endpointsStale = true;
        this.error = `Save failed: ${e.message}`;
      } finally { this.endpointMutationPending = false; }
    },
    async deleteEndpoint(entry) {
      if (this.endpointMutationPending) return;
      if (this.endpointsStale) { this.error = 'Refresh endpoints before making changes.'; return; }
      this.error = '';
      if (!entry || entry.source !== 'runtime') { this.error = 'Only runtime endpoints can be deleted here.'; return; }
      if (!this.endpoints.some(current => current.source === entry.source && current.index === entry.index && JSON.stringify(current.endpoint) === JSON.stringify(entry.endpoint))) {
        this.error = 'The endpoint list changed. Refresh before deleting.'; return;
      }
      this.endpointMutationPending = true;
      ++this.endpointsFetchEpoch;
      try {
        await this.api(`/endpoints/${entry.index}`, { method: 'DELETE' });
        this.endpointsStale = true;
        this.showEndpointModal = false; await this.fetchEndpoints(true);
      } catch (e) { this.endpointsStale = true; this.error = `Delete failed: ${e.message}`; }
      finally { this.endpointMutationPending = false; }
    },
    async exportEndpoint(entry = null) {
      this.error = '';
      try {
        const endpoint = entry ? entry.endpoint : JSON.parse(this.endpointJson);
        const r = await this.response(`${base}/endpoints/export`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(endpoint) });
        const url = URL.createObjectURL(await r.blob());
        const link = document.createElement('a'); link.href = url; link.download = 'endpoint.yaml';
        document.body.appendChild(link); link.click(); link.remove();
        setTimeout(() => URL.revokeObjectURL(url), 1000);
        this.notice = 'YAML downloaded. Add it to your endpoint configuration to persist it.';
      } catch (e) { this.error = `Export failed: ${e.message}`; }
    },
    prepareRequest(req) {
      if (!req) throw new Error('Select a request first.');
      if (!req.request_body_recorded || req.request_body_truncated) throw new Error('The complete request body was not recorded; replay and cURL are unavailable.');
      if (!/^\/(?!\/)/.test(req.path) || /[\\\r\n?#]/.test(req.path)) throw new Error('Only same-origin request paths are allowed.');
      const url = new URL(req.path, window.location.origin);
      let decoded = url.pathname;
      for (let i = 0; i < 3; i++) { const next = decodeURIComponent(decoded); if (next === decoded) break; decoded = next; }
      const normalized = new URL(decoded, window.location.origin);
      if (url.origin !== window.location.origin || normalized.origin !== window.location.origin) throw new Error('Only same-origin requests can be replayed.');
      if (normalized.pathname === base || normalized.pathname.startsWith(`${base}/`)) throw new Error('Admin requests cannot be replayed.');
      url.search = req.query || '';
      const method = (req.method || 'GET').toUpperCase();
      if (!['GET', 'HEAD', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS'].includes(method)) throw new Error('This method cannot be replayed by a browser.');
      const body = req.request_body || '';
      if (['GET', 'HEAD'].includes(method) && body) throw new Error('Browsers cannot replay a GET or HEAD request with a body.');
      const denied = new Set(['accept-charset', 'accept-encoding', 'access-control-request-headers', 'access-control-request-method', 'connection', 'content-length', 'cookie', 'cookie2', 'date', 'dnt', 'expect', 'host', 'keep-alive', 'origin', 'permissions-policy', 'proxy-connection', 'referer', 'set-cookie', 'te', 'trailer', 'transfer-encoding', 'upgrade', 'user-agent', 'via']);
      for (const [name, values] of Object.entries(req.request_headers || {})) {
        if (name.toLowerCase() === 'connection') String(values).split(',').forEach(v => denied.add(v.trim().toLowerCase()));
      }
      const headers = {};
      for (const [name, values] of Object.entries(req.request_headers || {})) {
        const lower = name.toLowerCase();
        if (!denied.has(lower) && !lower.startsWith('sec-') && !lower.startsWith('proxy-')) headers[name] = Array.isArray(values) ? values.join(', ') : String(values);
      }
      return { url: url.href, options: { method, headers, ...(!['GET', 'HEAD'].includes(method) ? { body } : {}), credentials: 'same-origin', redirect: 'error' } };
    },
    requestCurl(req) {
      const { url, options } = this.prepareRequest(req);
      const quote = value => "'" + String(value).replace(/'/g, "'\"'\"'") + "'";
      const parts = ['curl', '--globoff', '-X', quote(options.method), quote(url)];
      for (const [name, value] of Object.entries(options.headers)) parts.push('-H', quote(`${name}: ${value}`));
      if (options.body !== undefined) parts.push('--data-raw', quote(options.body));
      return parts.join(' ');
    },
    async copyCurl() {
      this.error = '';
      try { await this.copyText(this.requestCurl(this.selectedRequest)); }
      catch (e) { this.error = `Copy failed: ${e.message}`; }
    },
    async replayRequest() {
      const epoch = ++this.replayEpoch;
      const detailEpoch = this.requestDetailEpoch;
      const current = () => epoch === this.replayEpoch && detailEpoch === this.requestDetailEpoch;
      this.error = ''; this.replayResult = null; this.replaying = true;
      try {
        const { url, options } = this.prepareRequest(this.selectedRequest);
        const r = await fetch(url, options);
        const result = { status: r.status, body: await r.text(), headers: Object.fromEntries(r.headers.entries()) };
        if (current()) this.replayResult = result;
      } catch (e) { if (current()) this.error = `Replay failed: ${e.message}`; }
      finally { if (current()) this.replaying = false; }
    },
    async copyText(value) {
      try { await navigator.clipboard.writeText(value || ''); this.notice = 'Copied to clipboard.'; }
      catch (e) { this.error = `Copy failed: ${e.message}`; }
    },
    highlightJson(json) {
      const escaped = (json || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
      return escaped.replace(/("(?:\\u[0-9a-fA-F]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(?:true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g, match => {
        let color = 'text-purple-600';
        if (/^"/.test(match)) color = /:$/.test(match) ? 'text-blue-700 font-medium' : 'text-green-700';
        else if (/true|false/.test(match)) color = 'text-indigo-500';
        else if (match === 'null') color = 'text-red-400';
        return `<span class="${color}">${match}</span>`;
      });
    },
    formatTime(ts) { return ts ? new Date(ts).toLocaleString() : '—'; },
    methodColor(method) {
      return ({ GET: 'bg-green-100 text-green-800', POST: 'bg-blue-100 text-blue-800', PUT: 'bg-yellow-100 text-yellow-800', PATCH: 'bg-orange-100 text-orange-800', DELETE: 'bg-red-100 text-red-800' })[(method || '').toUpperCase()] || 'bg-gray-100 text-gray-700';
    },
    statusColor(status) {
      if (status >= 500) return 'bg-red-100 text-red-800';
      if (status >= 400) return 'bg-orange-100 text-orange-800';
      if (status >= 300) return 'bg-yellow-100 text-yellow-800';
      return 'bg-green-100 text-green-800';
    },
  };
}
if (typeof module !== 'undefined') module.exports = { dashboard };
