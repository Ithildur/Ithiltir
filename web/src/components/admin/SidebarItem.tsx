import React from 'react';
import { Link } from 'react-router-dom';
import type { LucideIcon } from 'lucide-react';

interface Props {
  icon: LucideIcon;
  label: string;
  active?: boolean;
  onClick?: () => void;
  to?: string;
}

const SidebarItem: React.FC<Props> = ({ icon: Icon, label, active = false, onClick, to }) => {
  const className = `w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all duration-200 group relative ${
    active
      ? 'border-l-2 border-(--theme-border-underline-nav-active) bg-(--theme-bg-accent-muted) text-(--theme-fg-accent)'
      : 'border-l-2 border-transparent text-(--theme-fg-default) hover:bg-(--theme-bg-muted) hover:text-(--theme-fg-accent)'
  }`;

  const content = (
    <>
      <Icon
        size={18}
        className={`relative z-10 transition-colors ${
          active
            ? 'text-(--theme-fg-accent)'
            : 'text-(--theme-fg-muted) group-hover:text-(--theme-fg-accent)'
        }`}
      />
      <span className="relative z-10">{label}</span>
    </>
  );

  if (to) {
    return (
      <Link to={to} onClick={onClick} className={className}>
        {content}
      </Link>
    );
  }

  return (
    <button type="button" onClick={onClick} className={className}>
      {content}
    </button>
  );
};

export default SidebarItem;
