import { fileURLToPath } from 'node:url';
import { resolveConfig } from 'vite';
import { expect, it } from 'vitest';

const entries = ['vite.config.js', 'vite.config.cloud.js', 'vite.config.desktop.js', 'vite.config.example.js'];
const config = async (entry, command = 'serve', mode = 'development') => resolveConfig({
  configFile: fileURLToPath(new URL(`../${entry}`, import.meta.url)),
}, command, mode);

it('keeps concurrent Cloud, Desktop and example dependency optimizers in separate cache directories', async () => {
  const resolved = await Promise.all(entries.map(entry => config(entry)));
  expect(new Set(resolved.map(value => value.cacheDir)).size).toBe(entries.length);
});

it('does not let test or production builds replace a running entry dependency cache', async () => {
  for (const entry of entries) {
    const resolved = await Promise.all([
      config(entry), config(entry, 'serve', 'test'), config(entry, 'build', 'production'),
    ]);
    expect(new Set(resolved.map(value => value.cacheDir)).size, entry).toBe(3);
  }
});
