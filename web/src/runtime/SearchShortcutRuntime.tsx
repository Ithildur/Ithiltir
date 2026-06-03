import React from 'react';

const searchInputSelector = '[data-search-input="true"]';
const searchTriggerSelector = '[data-search-trigger="true"]';

const isTextEntryElement = (element: Element | null): boolean => {
  if (!(element instanceof HTMLElement)) return false;
  const tagName = element.tagName.toLowerCase();
  return (
    tagName === 'input' ||
    tagName === 'textarea' ||
    tagName === 'select' ||
    element.isContentEditable
  );
};

const isVisible = (element: HTMLElement): boolean => element.getClientRects().length > 0;

const focusVisibleSearchInput = (): boolean => {
  const inputs = Array.from(document.querySelectorAll<HTMLInputElement>(searchInputSelector));
  const target = inputs.find((input) => !input.disabled && !input.readOnly && isVisible(input));
  if (!target) return false;
  target.focus();
  target.select();
  return true;
};

export const SearchShortcutRuntime: React.FC = () => {
  React.useEffect(() => {
    const keyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.isComposing) return;
      if (event.key !== '/') return;
      if (event.metaKey || event.ctrlKey || event.altKey) return;
      if (isTextEntryElement(document.activeElement)) return;

      if (focusVisibleSearchInput()) {
        event.preventDefault();
        return;
      }

      const triggers = Array.from(document.querySelectorAll<HTMLElement>(searchTriggerSelector));
      const trigger = triggers.find(isVisible);
      if (!trigger) return;

      event.preventDefault();
      trigger.click();
      window.setTimeout(focusVisibleSearchInput, 0);
    };

    window.addEventListener('keydown', keyDown);
    return () => {
      window.removeEventListener('keydown', keyDown);
    };
  }, []);

  return null;
};
