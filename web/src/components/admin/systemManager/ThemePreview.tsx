import React from 'react';
import type { ThemeManifest, ThemePackage } from '@app-types/admin';

type ThemeVisual = Pick<ThemeManifest, 'id' | 'skin'>;

const hueFromID = (id: string): number => {
  let hash = 0;
  for (let i = 0; i < id.length; i += 1) {
    hash = (hash * 31 + id.charCodeAt(i)) % 360;
  }
  return hash;
};

const previewPalette = (item: ThemeVisual) => {
  if (item.id === 'default') {
    return {
      accent: '#0969da',
      soft: '#ddf4ff',
      panel: '#f6f8fa',
      line: 'rgba(9, 105, 218, 0.16)',
    };
  }
  if (item.id === 'operator') {
    return {
      accent: '#2f7b3d',
      soft: '#e8f3e6',
      panel: '#f4f7ef',
      line: 'rgba(47, 123, 61, 0.16)',
    };
  }

  const hue = hueFromID(item.id);
  return {
    accent: `hsl(${hue} 68% 42%)`,
    soft: `hsl(${hue} 72% 95%)`,
    panel: `hsl(${hue} 48% 97%)`,
    line: `hsla(${hue} 48% 36% / 0.16)`,
  };
};

const themePreviewSrc = (item: ThemePackage): string | null => {
  if (!item.has_preview) return null;
  const base = `/theme/preview/${encodeURIComponent(item.id)}.png`;
  if (!item.updated_at) return base;
  return `${base}?v=${encodeURIComponent(item.updated_at)}`;
};

export const FallbackThemePreview: React.FC<{ item: ThemeVisual }> = ({ item }) => {
  const palette = previewPalette(item);
  const topbar = item.skin.admin.shell === 'topbar';
  const strip = item.skin.dashboard.summary === 'strip';
  const compact = item.skin.dashboard.density === 'compact';

  return (
    <div
      className="h-18 w-30 shrink-0 overflow-hidden rounded-lg border"
      style={{
        backgroundColor: palette.panel,
        borderColor: 'rgba(15, 23, 42, 0.08)',
      }}
    >
      <div
        className="flex h-4 items-center px-2"
        style={{ backgroundColor: palette.soft, borderBottom: `1px solid ${palette.line}` }}
      >
        <div className="flex gap-1">
          <span className="size-1.5 rounded-full" style={{ backgroundColor: palette.line }} />
          <span className="size-1.5 rounded-full" style={{ backgroundColor: palette.line }} />
          <span className="size-1.5 rounded-full" style={{ backgroundColor: palette.line }} />
        </div>
      </div>

      <div className="flex h-[calc(100%-1rem)] gap-2 p-2">
        {!topbar && (
          <div className="w-3 rounded-[5px]" style={{ backgroundColor: palette.accent }} />
        )}

        <div className="flex min-w-0 flex-1 flex-col gap-1.5">
          {topbar && (
            <div className="h-2 rounded-[5px]" style={{ backgroundColor: palette.accent }} />
          )}

          {strip ? (
            <div className={`grid flex-1 ${compact ? 'grid-cols-4 gap-1' : 'grid-cols-3 gap-1'}`}>
              {Array.from({ length: compact ? 4 : 3 }).map((_, index) => (
                <div
                  key={index}
                  className="rounded-[5px]"
                  style={{
                    backgroundColor: index === 0 ? palette.soft : 'rgba(255,255,255,0.72)',
                    outline: `1px solid ${palette.line}`,
                    outlineOffset: '-1px',
                  }}
                >
                  <div
                    className="mx-1 mt-1 h-1 rounded-full"
                    style={{
                      backgroundColor: palette.accent,
                      opacity: 0.82 - index * 0.12,
                    }}
                  />
                </div>
              ))}
            </div>
          ) : (
            <div className={`grid flex-1 ${compact ? 'grid-cols-3 gap-1' : 'grid-cols-2 gap-1.5'}`}>
              {Array.from({ length: compact ? 3 : 2 }).map((_, index) => (
                <div
                  key={index}
                  className="rounded-[5px] bg-white/75"
                  style={{
                    outline: `1px solid ${palette.line}`,
                    outlineOffset: '-1px',
                  }}
                >
                  <div
                    className="h-1 rounded-t-[5px]"
                    style={{
                      backgroundColor: index === 0 ? palette.accent : 'rgba(15, 23, 42, 0.08)',
                    }}
                  />
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

const ThemePreview: React.FC<{ item: ThemePackage }> = ({ item }) => {
  const src = themePreviewSrc(item);
  const [failed, setFailed] = React.useState(false);

  React.useEffect(() => {
    setFailed(false);
  }, [src]);

  if (!src || failed) {
    return <FallbackThemePreview item={item} />;
  }

  return (
    <img
      src={src}
      alt={`${item.name} preview`}
      loading="lazy"
      onError={() => setFailed(true)}
      className="h-18 w-30 shrink-0 rounded-lg border border-(--theme-border-subtle)/70 object-cover dark:border-(--theme-border-default)"
    />
  );
};

export default ThemePreview;
