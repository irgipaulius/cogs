#!/usr/bin/env node
const fs = require('fs');
const path = require('path');

const apiBase =
  process.env.COGS_PUBLIC_API_URL ||
  process.env.API_BASE ||
  'https://goldilocks-api.example.com';
const pageOrigin = process.env.COGS_PUBLIC_PAGE_ORIGIN || '';

const out = `window.COGS_CONFIG = {
  pageOrigin: ${JSON.stringify(pageOrigin)},
  apiBase: ${JSON.stringify(apiBase)},
};
`;

const dir = path.join(__dirname, '..', 'public');
fs.mkdirSync(dir, { recursive: true });
fs.writeFileSync(path.join(dir, 'config.js'), out);
console.log('wrote public/config.js apiBase=', apiBase);
