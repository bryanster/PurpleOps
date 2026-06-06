import { FullConfig } from '@playwright/test';

export default async function globalSetup(config: FullConfig) {
  const baseURL = config.projects[0].use.baseURL || 'http://localhost:8888';

  try {
    const res = await fetch(baseURL + '/login', { signal: AbortSignal.timeout(3000) });
    if (!res.ok && res.status !== 302) {
      throw new Error(`Server returned ${res.status}`);
    }
  } catch (err) {
    console.log(`\nSkipping E2E tests: server not reachable at ${baseURL}`);
    console.log(`Start the app with 'docker compose up' then re-run 'npx playwright test'.\n`);
    process.exit(0);
  }
}
