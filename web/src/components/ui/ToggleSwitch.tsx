import React from 'react';

export interface ToggleSwitchProps {
  checked: boolean;
  onChange: () => void;
  onContent: React.ReactNode;
  offContent: React.ReactNode;
  width?: string;
  height?: string;
  className?: string;
  onColor?: string;
  offColor?: string;
}

const ToggleSwitch: React.FC<ToggleSwitchProps> = ({
  checked,
  onChange,
  onContent,
  offContent,
  width = 'w-24',
  height = 'h-8',
  className = '',
  onColor = 'bg-(--theme-fg-success-muted)',
  offColor = 'bg-(--theme-fg-danger)',
}) => {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      onClick={(e) => {
        e.stopPropagation();
        onChange();
      }}
      className={`relative flex items-center justify-center rounded transition-colors duration-300 focus:outline-none focus:ring-2 focus:ring-(--theme-fg-interactive)/20 ${
        checked ? onColor : offColor
      } ${width} ${height} ${className}`}
    >
      <span
        className={`absolute inset-y-0.5 w-3 rounded-sm bg-(--theme-bg-default) shadow-sm transition-[left] duration-300 ease-[cubic-bezier(0.23,1,0.32,1)] ${
          checked ? 'left-[calc(100%-0.875rem)]' : 'left-0.5'
        }`}
      />
      <span className="relative z-10 text-sm font-bold text-(--theme-fg-on-emphasis) select-none tracking-wide">
        {checked ? onContent : offContent}
      </span>
    </button>
  );
};

export default ToggleSwitch;
