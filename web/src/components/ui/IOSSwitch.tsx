import React from 'react';
import { useI18n } from '@i18n';

interface Props {
  checked: boolean;
  onChange: () => void;
  ariaLabel?: string;
  className?: string;
  disabled?: boolean;
  size?: 'sm' | 'md';
}

const IOSSwitch: React.FC<Props> = ({
  checked,
  onChange,
  ariaLabel,
  className = '',
  disabled = false,
  size = 'md',
}) => {
  const { t } = useI18n();

  const sizeClasses = {
    sm: {
      switch: 'h-5 w-9',
      knob: 'h-4 w-4',
      translate: 'translate-x-4',
    },
    md: {
      switch: 'h-7 w-12',
      knob: 'h-6 w-6',
      translate: 'translate-x-5',
    },
  };

  const currentSize = sizeClasses[size];

  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={(e) => {
        e.stopPropagation();
        if (!disabled) {
          onChange();
        }
      }}
      className={`
        relative inline-flex ${currentSize.switch} shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent
        transition-colors duration-200 ease-in-out focus:outline-none focus-visible:ring-2
        focus-visible:ring-indigo-600 focus-visible:ring-offset-2
        ${checked ? 'bg-[#34c759] dark:bg-[#09ac3c]' : 'bg-[#e9e9ea] dark:bg-[#262626]'}
        ${disabled ? 'opacity-50 cursor-not-allowed' : ''}
        ${className}
      `}
    >
      <span className="sr-only">{ariaLabel ?? t('common_toggle_setting')}</span>
      <span
        aria-hidden="true"
        className={`
          pointer-events-none inline-block ${currentSize.knob} transform rounded-full bg-white shadow ring-0
          transition duration-200 ease-in-out
          ${checked ? currentSize.translate : 'translate-x-0'}
        `}
      />
    </button>
  );
};

export default IOSSwitch;
