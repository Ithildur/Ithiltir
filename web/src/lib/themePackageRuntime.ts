import type {
  ThemeDensity,
  ThemeFrame,
  ThemeManifest,
  ThemeShell,
  ThemeSpec,
  ThemeSummary,
} from '@app-types/admin';

export const themeRefreshEvent = 'dash:theme-refresh';

const defaultThemeSpec: ThemeSpec = {
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

const themeIdPattern = /^[a-z0-9][a-z0-9_-]{0,63}$/;
const themeShells = ['sidebar', 'topbar'] as const satisfies readonly ThemeShell[];
const themeFrames = ['layered', 'flat'] as const satisfies readonly ThemeFrame[];
const themeSummaries = ['cards', 'strip'] as const satisfies readonly ThemeSummary[];
const themeDensities = ['comfortable', 'compact'] as const satisfies readonly ThemeDensity[];

const isObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

const invalidThemeManifest = (path: string, reason: string): Error =>
  new Error(`invalid theme manifest: ${path} ${reason}`);

const readObject = (source: Record<string, unknown>, key: string): Record<string, unknown> => {
  const value = source[key];
  if (!isObject(value)) throw invalidThemeManifest(key, 'must be an object');
  return value;
};

const readRequiredText = (source: Record<string, unknown>, key: string): string => {
  const value = source[key];
  if (typeof value !== 'string') throw invalidThemeManifest(key, 'must be a string');

  const text = value.trim();
  if (!text) throw invalidThemeManifest(key, 'must not be empty');
  return text;
};

const readText = (source: Record<string, unknown>, key: string): string => {
  const value = source[key];
  if (value === undefined || value === null) return '';
  if (typeof value !== 'string') throw invalidThemeManifest(key, 'must be a string');
  return value.trim();
};

const readThemeChoice = <T extends string>(
  source: Record<string, unknown>,
  key: string,
  values: readonly T[],
): T => {
  const value = source[key];
  if (typeof value !== 'string') throw invalidThemeManifest(key, 'must be a string');

  const text = value.trim();
  if (!text) throw invalidThemeManifest(key, 'must not be empty');
  if (!values.includes(text as T)) {
    throw invalidThemeManifest(key, `must be one of ${values.join(', ')}`);
  }
  return text as T;
};

const parseThemeManifest = (input: unknown): ThemeManifest => {
  if (!isObject(input)) throw invalidThemeManifest('root', 'must be an object');

  const id = readRequiredText(input, 'id');
  if (!themeIdPattern.test(id)) {
    throw invalidThemeManifest('id', 'must match [a-z0-9][a-z0-9_-]{0,63}');
  }

  const skin = readObject(input, 'skin');
  const admin = readObject(skin, 'admin');
  const dashboard = readObject(skin, 'dashboard');

  return {
    id,
    name: readRequiredText(input, 'name'),
    version: readRequiredText(input, 'version'),
    author: readText(input, 'author'),
    description: readText(input, 'description'),
    skin: {
      admin: {
        shell: readThemeChoice(admin, 'shell', themeShells),
        frame: readThemeChoice(admin, 'frame', themeFrames),
      },
      dashboard: {
        summary: readThemeChoice(dashboard, 'summary', themeSummaries),
        density: readThemeChoice(dashboard, 'density', themeDensities),
      },
    },
  };
};

const themePackageRuntime = (): NonNullable<Window['__themePackage']> => {
  if (!window.__themePackage) {
    throw new Error('Theme package runtime is not installed');
  }
  return window.__themePackage;
};

export const ensureThemePackageRuntime = (): void => {
  themePackageRuntime();
};

const readThemeManifest = (): ThemeManifest | null => {
  const manifest = themePackageRuntime().manifest;
  if (!manifest) return null;
  return parseThemeManifest(manifest);
};

const saveThemeManifest = (manifest: ThemeManifest): ThemeManifest => {
  themePackageRuntime().manifest = manifest;
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
  return parseThemeManifest(await response.json());
};

export const resolveThemeManifest = async (signal?: AbortSignal): Promise<ThemeManifest> => {
  const cached = readThemeManifest();
  if (cached) return cached;

  try {
    const manifest = await themePackageRuntime().manifestPromise;
    if (manifest) return saveThemeManifest(parseThemeManifest(manifest));
  } catch {
    // Ignore bootstrap prefetch failure and fetch active.json below.
  }

  return saveThemeManifest(await fetchActiveThemeManifest(signal));
};

export const refreshThemeManifest = async (signal?: AbortSignal): Promise<ThemeManifest> =>
  saveThemeManifest(await fetchActiveThemeManifest(signal));

export const refreshActiveThemeStyles = async (): Promise<void> => {
  await themePackageRuntime().refresh();
  window.dispatchEvent(new Event(themeRefreshEvent));
};
