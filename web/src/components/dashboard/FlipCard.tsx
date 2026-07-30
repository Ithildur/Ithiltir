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

  const showBack = isFlipped && !disabled;

  return (
    <div className="relative h-102 w-full perspective-1000 group" onClick={toggle}>
      <button
        type="button"
        disabled={disabled}
        className="pointer-events-none absolute inset-0 z-10 rounded-xl border-0 bg-transparent p-0 focus:outline-none focus-visible:ring-2 focus-visible:ring-(--theme-focus-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--theme-bg-default) dark:focus-visible:ring-offset-(--theme-bg-default)"
        onClick={(event) => {
          event.stopPropagation();
          toggle();
        }}
        aria-pressed={showBack}
        aria-label={t('flip')}
      />
      <div
        className={`relative size-full transition-transform duration-500 motion-reduce:transition-none transform-style-3d ${showBack ? 'rotate-y-180' : ''} ${disabled ? 'cursor-not-allowed' : 'cursor-pointer'}`}
      >
        <div className="contents" aria-hidden={showBack} inert={showBack}>
          {front}
        </div>
        <div className="contents" aria-hidden={!showBack} inert={!showBack}>
          {back}
        </div>
      </div>
    </div>
  );
};

export default FlipCard;
