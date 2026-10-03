export const requestLogs = [
  { id: 'req_8f2a01', time: '14:32:58', ip: '203.0.113.42', country: 'United States', code: 'US', method: 'POST', path: '/api/auth/login', action: 'Blocked', rule: 'SQL injection', status: 403 },
  { id: 'req_8f2a02', time: '14:32:56', ip: '198.51.100.18', country: 'Germany', code: 'DE', method: 'GET', path: '/products', action: 'Allowed', rule: 'Passed all rules', status: 200 },
  { id: 'req_8f2a03', time: '14:32:54', ip: '192.0.2.156', country: 'Netherlands', code: 'NL', method: 'GET', path: '/admin/config.php', action: 'Blocked', rule: 'Path traversal', status: 403 },
  { id: 'req_8f2a04', time: '14:32:51', ip: '203.0.113.87', country: 'Singapore', code: 'SG', method: 'POST', path: '/api/comments', action: 'Blocked', rule: 'Cross-site scripting', status: 403 },
  { id: 'req_8f2a05', time: '14:32:49', ip: '198.51.100.64', country: 'United Kingdom', code: 'GB', method: 'GET', path: '/api/search?q=shoes', action: 'Challenged', rule: 'Bot protection', status: 429 },
  { id: 'req_8f2a06', time: '14:32:47', ip: '192.0.2.23', country: 'Poland', code: 'PL', method: 'GET', path: '/', action: 'Allowed', rule: 'Passed all rules', status: 200 },
  { id: 'req_8f2a07', time: '14:32:44', ip: '203.0.113.106', country: 'United States', code: 'US', method: 'POST', path: '/api/auth/login', action: 'Blocked', rule: 'Rate limit exceeded', status: 429 },
  { id: 'req_8f2a08', time: '14:32:40', ip: '198.51.100.92', country: 'France', code: 'FR', method: 'GET', path: '/assets/app.css', action: 'Allowed', rule: 'Passed all rules', status: 200 },
  { id: 'req_8f2a09', time: '14:32:38', ip: '192.0.2.77', country: 'Germany', code: 'DE', method: 'GET', path: '/.env', action: 'Blocked', rule: 'Sensitive file access', status: 403 },
  { id: 'req_8f2a10', time: '14:32:35', ip: '203.0.113.219', country: 'Japan', code: 'JP', method: 'POST', path: '/api/checkout', action: 'Challenged', rule: 'Bot protection', status: 429 },
  { id: 'req_8f2a11', time: '14:32:32', ip: '198.51.100.35', country: 'Canada', code: 'CA', method: 'GET', path: '/collections/new', action: 'Allowed', rule: 'Passed all rules', status: 200 },
  { id: 'req_8f2a12', time: '14:32:29', ip: '192.0.2.201', country: 'Netherlands', code: 'NL', method: 'POST', path: '/api/upload', action: 'Blocked', rule: 'Malicious payload', status: 403 }
]

export const attacks = [
  { name: 'SQL injection', value: 1842, color: '#6366f1' },
  { name: 'Cross-site scripting', value: 1286, color: '#a78bfa' },
  { name: 'Malicious bots', value: 962, color: '#38bdf8' },
  { name: 'Path traversal', value: 574, color: '#fbbf24' },
  { name: 'Other threats', value: 198, color: '#cbd5e1' }
]

export const blockedSources = [
  { ip: '203.0.113.42', country: 'United States', code: 'US', reason: 'SQL injection', requests: 1248 },
  { ip: '192.0.2.156', country: 'Netherlands', code: 'NL', reason: 'Path traversal', requests: 986 },
  { ip: '203.0.113.87', country: 'Singapore', code: 'SG', reason: 'Cross-site scripting', requests: 742 },
  { ip: '203.0.113.106', country: 'United States', code: 'US', reason: 'Rate limit exceeded', requests: 518 }
]

export const hourlyTraffic = [18200, 16800, 15200, 14100, 16200, 21400, 26300, 31800, 29400, 35600, 41200, 38400, 46200, 52800, 48600, 43100, 39800, 44600, 37200, 32400, 29100, 25400, 22600, 19800]
export const hourlyBlocked = [120, 98, 86, 75, 98, 145, 192, 226, 174, 211, 268, 243, 324, 363, 386, 312, 256, 289, 236, 198, 174, 143, 112, 133]
