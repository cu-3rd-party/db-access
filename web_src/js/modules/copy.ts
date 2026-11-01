import {showErrorToast} from './toast.ts';

const copiedMs = 1500;

// Listens on document: the buttons come and go with htmx swaps.
document.addEventListener('click', async (event) => {
  const button = (event.target as Element).closest<HTMLButtonElement>('[data-copy]');
  const text = button && document.getElementById(button.dataset.copy!)?.textContent;
  if (!button || !text) return;
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    showErrorToast('Не удалось скопировать: выделите строку вручную');
    return;
  }
  const label = button.dataset.label ?? button.textContent;
  button.dataset.label = label;
  button.textContent = 'Скопировано';
  setTimeout(() => {
    button.textContent = label;
  }, copiedMs);
});
