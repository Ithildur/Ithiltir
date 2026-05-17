import React from 'react';
import { useI18n } from '@i18n';

interface Props {
  disabled?: boolean;
  front: React.ReactNode;
  back: React.ReactNode;
}

const FlipCard: React.FC<Props> = ({ disabled, front, back }) => {
  const [isFlipped, setIsFlipped] = React.useState(false);
  const { t } = useI18n();

  const toggle = () => {
    if (disabled) return;
    setIsFlipped((prev) => !prev);
  };

  const keyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (disabled) return;
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      setIsFlipped((prev) => !prev);
    }
  };

  return (
    <div className="relative h-102 w-full perspective-1000 group">
      <div
        className={`relative size-full transition-transform duration-500 motion-reduce:transition-none transform-style-3d focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--theme-focus-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--theme-bg-default) dark:focus-visible:ring-offset-(--theme-bg-default) ${isFlipped && !disabled ? 'rotate-y-180' : ''} ${disabled ? 'cursor-not-allowed' : 'cursor-pointer'}`}
        role="button"
        tabIndex={disabled ? -1 : 0}
        onClick={toggle}
        onKeyDown={keyDown}
        aria-pressed={isFlipped}
        aria-label={t('flip')}
      >
        {front}
        {back}
      </div>
    </div>
  );
};

export default FlipCard;
