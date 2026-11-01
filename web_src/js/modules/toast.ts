export type ToastLevel = 'info' | 'error';

const levelClasses: Record<ToastLevel, string> = {
  info: 'border border-line bg-code text-ink',
  error: 'border border-danger-border bg-danger-bg text-danger-text',
};

export function showToast(message: string, level: ToastLevel = 'info', duration = 5000): HTMLElement {
  let container = document.querySelector<HTMLElement>('#toast-container');
  if (!container) {
    container = document.createElement('div');
    container.id = 'toast-container';
    document.body.append(container);
  }
  const toast = document.createElement('div');
  toast.className = `toast rounded-box px-4 py-2 text-sm shadow ${levelClasses[level]}`;
  toast.setAttribute('role', level === 'error' ? 'alert' : 'status');
  toast.textContent = message;
  container.append(toast);
  if (duration > 0) setTimeout(() => toast.remove(), duration);
  return toast;
}

export function showErrorToast(message: string, duration?: number): HTMLElement {
  return showToast(message, 'error', duration);
}
