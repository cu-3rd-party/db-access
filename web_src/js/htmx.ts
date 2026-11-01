import htmx from 'htmx.org';
import {showErrorToast} from './modules/toast.ts';

// https://htmx.org/reference/#config
htmx.config.requestClass = 'is-loading';
htmx.config.scrollIntoViewOnBoost = false;
// render 422 validation errors etc. as toasts instead of swapping them in
htmx.config.responseHandling = [
  {code: '204', swap: false},
  {code: '[23]..', swap: true},
  {code: '[45]..', swap: false, error: true},
];

// https://htmx.org/events/#htmx:sendError
document.body.addEventListener('htmx:sendError', (event: Event) => {
  const {detail} = event as CustomEvent;
  showErrorToast(`Network error when calling ${detail.requestConfig.path}`);
});

// https://htmx.org/events/#htmx:responseError
document.body.addEventListener('htmx:responseError', (event: Event) => {
  const {detail} = event as CustomEvent;
  showErrorToast(detail.xhr.responseText || `Error ${detail.xhr.status} when calling ${detail.requestConfig.path}`);
});
