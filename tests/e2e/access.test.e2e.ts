import {expect, test} from './utils_e2e.ts';

test('first visit creates a user and shows the password without any click', async ({page, context}) => {
  await page.goto('/');
  await expect(page.locator('#password')).toHaveText(/^[a-zA-Z0-9]{32}$/);
  const login = await page.locator('#login').innerText();
  expect(login).toMatch(/^[a-zA-Z0-9]{8}$/);

  const cookies = await context.cookies();
  const cookie = cookies.find((c) => c.name === 'cu_login');
  expect(cookie?.value).toBe(login);
  expect(cookie?.httpOnly).toBe(true);

  await page.reload();
  await expect(page.locator('#login')).toHaveText(login);
  await expect(page.locator('#password')).toHaveCount(0);
});

test('write checkbox acts on one database and keeps the password on screen', async ({page}) => {
  await page.goto('/');
  const password = await page.locator('#password').innerText();
  await expect(page.locator('#write-note')).toContainText('будет забран через день');

  await page.getByLabel('Получить write-доступ к cu-db').check();
  await expect(page.locator('[id="db-cu-db"]')).toContainText('МСК');
  await expect(page.locator('[id="db-cu-sql"]')).not.toContainText('МСК');
  await expect(page.locator('#password')).toHaveText(password);
  const login = await page.locator('#login').innerText();
  await expect(page.locator('[id="db-cu-db"]')).toContainText(`postgresql://${login}:`);

  await page.reload();
  await expect(page.getByLabel('Получить write-доступ к cu-db')).toBeChecked();
  await expect(page.getByLabel('Получить write-доступ к cu-sql')).not.toBeChecked();

  await page.getByLabel('Получить write-доступ к cu-db').uncheck();
  await expect(page.locator('[id="db-cu-db"]')).not.toContainText('МСК');
});

test('rotation shows new credentials once', async ({page}) => {
  await page.goto('/');
  const oldLogin = await page.locator('#login').innerText();
  const oldPassword = await page.locator('#password').innerText();

  page.once('dialog', (dialog) => dialog.accept());
  await page.locator('#rotate').click();
  await expect(page.locator('#login')).not.toHaveText(oldLogin);
  await expect(page.locator('#password')).not.toHaveText(oldPassword);
  await expect(page.locator('#password')).toHaveText(/^[a-zA-Z0-9]{32}$/);
  const newLogin = await page.locator('#login').innerText();

  await page.reload();
  await expect(page.locator('#login')).toHaveText(newLogin);
  await expect(page.locator('#password')).toHaveCount(0);
});
