import type { ThemeManifest, ThemeSpec } from '@app-types/admin';

export const themeRefreshEvent = 'dash:theme-refresh';

export const defaultThemeSpec: ThemeSpec = {
  admin: {
    shell: 'sidebar',
    frame: 'layered',
  },
  dashboard: {
    summary: 'cards',
    density: 'comfortable',
  },
};

export const defaultThemeManifest: ThemeManifest = {
  id: 'default',
  name: 'Default',
  version: '1.0.0',
  author: 'Ithiltir',
  description: '',
  skin: defaultThemeSpec,
};

export const normalizeThemeManifest = (
  input: Partial<ThemeManifest> | null | undefined,
): ThemeManifest => ({
  id: input?.id?.trim() || defaultThemeManifest.id,
  name: input?.name?.trim() || defaultThemeManifest.name,
  version: input?.version?.trim() || defaultThemeManifest.version,
  author: input?.author?.trim() || defaultThemeManifest.author,
  description: input?.description?.trim() || defaultThemeManifest.description,
  skin: {
    admin: {
      shell: input?.skin?.admin?.shell === 'topbar' ? 'topbar' : 'sidebar',
      frame: input?.skin?.admin?.frame === 'flat' ? 'flat' : 'layered',
    },
    dashboard: {
      summary: input?.skin?.dashboard?.summary === 'strip' ? 'strip' : 'cards',
      density: input?.skin?.dashboard?.density === 'compact' ? 'compact' : 'comfortable',
    },
  },
});

const getThemePackageRuntime = () => globalThis.window?.__themePackage;

const readThemeManifest = (): ThemeManifest | null => {
  const manifest = getThemePackageRuntime()?.manifest;
  return manifest ? normalizeThemeManifest(manifest) : null;
};

const saveThemeManifest = (input: Partial<ThemeManifest> | null | undefined): ThemeManifest => {
  const manifest = normalizeThemeManifest(input);
  const runtime = getThemePackageRuntime();
  if (runtime) {
    runtime.manifest = manifest;
  }
  return manifest;
};

export const getThemeManifest = (): ThemeManifest => readThemeManifest() ?? defaultThemeManifest;

const fetchActiveThemeManifest = async (signal?: AbortSignal): Promise<ThemeManifest> => {
  const response = await fetch('/theme/active.json', {
    cache: 'no-store',
    credentials: 'same-origin',
    signal,
  });
  if (response.status === 404) {
    return defaultThemeManifest;
  }
  if (!response.ok) {
    throw new Error(`failed to fetch active theme: ${response.status}`);
  }
  return normalizeThemeManifest((await response.json()) as Partial<ThemeManifest>);
};

export const resolveThemeManifest = async (signal?: AbortSignal): Promise<ThemeManifest> => {
  const cached = readThemeManifest();
  if (cached) return cached;

  const pending = getThemePackageRuntime()?.manifestPromise;
  if (pending) {
    try {
      const manifest = await pending;
      if (manifest) return saveThemeManifest(manifest);
    } catch {
      // Ignore bootstrap prefetch failure and fallback to active.json fetch below.
    }
  }

  return saveThemeManifest(await fetchActiveThemeManifest(signal));
};

export const refreshThemeManifest = async (signal?: AbortSignal): Promise<ThemeManifest> =>
  saveThemeManifest(await fetchActiveThemeManifest(signal));

export const refreshActiveThemeStyles = (): void => {
  getThemePackageRuntime()?.refresh?.();
  globalThis.window?.dispatchEvent(new Event(themeRefreshEvent));
};
