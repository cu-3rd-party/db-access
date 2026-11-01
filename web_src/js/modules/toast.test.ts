import {showErrorToast, showToast} from './toast.ts';

test('showToast appends a toast to the container', () => {
  document.body.innerHTML = '<div id="toast-container"></div>';
  const toast = showToast('hello', 'info', 0);
  expect(toast.textContent).toEqual('hello');
  expect(toast.getAttribute('role')).toEqual('status');
  expect(document.querySelectorAll('#toast-container .toast')).toHaveLength(1);
});

test('showErrorToast creates the container if missing and removes itself', () => {
  vi.useFakeTimers();
  document.body.innerHTML = '';
  const toast = showErrorToast('boom', 100);
  expect(toast.getAttribute('role')).toEqual('alert');
  expect(document.querySelector('#toast-container')).not.toBeNull();
  vi.advanceTimersByTime(100);
  expect(toast.isConnected).toBe(false);
  vi.useRealTimers();
});
