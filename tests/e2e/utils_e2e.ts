import {expect, test as baseTest, type Browser, type BrowserContextOptions} from '@playwright/test';

export type TestOptions = {
  resetFixtures: void;
};

export const test = baseTest.extend<TestOptions>({
  // every page fails the test on an uncaught JS error
  context: async ({browser, contextOptions}, use) => {
    await use(await test_context(browser, contextOptions));
  },
  // each test starts from models/fixtures (endpoint provided by tests/e2e/e2e_test.go)
  resetFixtures: [async ({request}, use) => {
    const response = await request.post('/_test/reset-fixtures');
    expect(response.status()).toBe(204);
    await use();
  }, {auto: true}],
});

export async function test_context(browser: Browser, options?: BrowserContextOptions) {
  const context = await browser.newContext(options);
  context.on('page', (page) => {
    page.on('pageerror', (err) => expect(err).toBeUndefined());
  });
  return context;
}

export {expect};
