import '@awesome.me/webawesome/dist/components/callout/callout.js'
import '@awesome.me/webawesome/dist/components/icon/icon.js'
import { escapeHtml } from './func'

// Custom function to emit toast notifications
export const notify = (message: string, variant: string = 'neutral', duration: number = 3000) => {
  const container = document.getElementById('callout-toast-container');
  if (!container) return;

  const callout = document.createElement('wa-callout');
  callout.setAttribute('variant', variant);

  let iconName = 'circle-info';
  if (variant === 'success') iconName = 'circle-check';
  if (variant === 'warning') iconName = 'triangle-exclamation';
  if (variant === 'danger') iconName = 'circle-exclamation';

  callout.innerHTML = `
    <wa-icon slot="icon" name="${iconName}"></wa-icon>
    ${escapeHtml(message)}
  `;

  // Add smooth CSS transitions
  callout.style.transition = 'all 0.3s ease';
  callout.style.opacity = '0';
  callout.style.transform = 'translateY(20px)';

  container.appendChild(callout);

  // Trigger "fade in" animation
  setTimeout(() => {
    callout.style.opacity = '1';
    callout.style.transform = 'translateY(0)';
  }, 10);

  // Automatically fade out and remove after the duration
  setTimeout(() => {
    callout.style.opacity = '0';
    callout.style.transform = 'translateY(-20px)';

    // Wait for the transition to finish before dropping from DOM
    callout.addEventListener('transitionend', () => {
      callout.remove();
    });
  }, duration);
}
