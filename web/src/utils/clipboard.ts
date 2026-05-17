export type ClipboardCopyResult = 'success' | 'https_required' | 'unsupported' | 'failed';

export type BannerPusher = (
  message: string,
  options?: { tone?: 'info' | 'warning' | 'error'; durationMs?: number | null },
) => number;

const isIpHostname = (hostname: string): boolean => {
  if (!hostname) return false;
  const normalized = hostname.replace(/^\[|\]$/g, '');
  const ipv4Pattern = /^(?:\d{1,3}\.){3}\d{1,3}$/;
  if (ipv4Pattern.test(normalized)) return true;
  return normalized.includes(':');
};

const isLocalHostname = (hostname: string): boolean => {
  const normalized = hostname.toLowerCase();
  return (
    normalized === 'localhost' ||
    normalized === '127.0.0.1' ||
    normalized === '::1' ||
    normalized.endsWith('.localhost')
  );
};

export const canBypassClipboardHttpsRequirement = (hostname: string): boolean =>
  isIpHostname(hostname) || isLocalHostname(hostname);

export const canBypassClipboardHttpsRequirementForCurrentHost = (): boolean => {
  if (typeof window === 'undefined') return false;
  return canBypassClipboardHttpsRequirement(window.location.hostname);
};

const attemptLegacyCopy = (text: string): boolean => {
  if (typeof document === 'undefined' || !document.body) return false;
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.style.position = 'fixed';
  textarea.style.left = '-9999px';
  textarea.style.opacity = '0';
  textarea.setAttribute('readonly', 'true');
  document.body.appendChild(textarea);
  textarea.focus();
  textarea.select();
  let succeeded = false;
  try {
    succeeded = document.execCommand('copy');
  } catch {
    succeeded = false;
  } finally {
    document.body.removeChild(textarea);
  }
  return succeeded;
};

export const copyTextToClipboard = async (
  text: string,
  opts: { allowInsecureContextBypass?: boolean } = {},
): Promise<ClipboardCopyResult> => {
  const clipboard = typeof navigator !== 'undefined' ? navigator.clipboard : undefined;
  const isSecureContext = typeof window !== 'undefined' ? window.isSecureContext : true;

  if (clipboard) {
    try {
      await clipboard.writeText(text);
      return 'success';
    } catch {
      if (opts.allowInsecureContextBypass && attemptLegacyCopy(text)) {
        return 'success';
      }
      if (!isSecureContext && !opts.allowInsecureContextBypass) {
        return 'https_required';
      }
      return 'failed';
    }
  }

  if (opts.allowInsecureContextBypass && attemptLegacyCopy(text)) {
    return 'success';
  }

  if (!isSecureContext && !opts.allowInsecureContextBypass) {
    return 'https_required';
  }

  return 'unsupported';
};

export const copyTextToClipboardWithFeedback = async (
  text: string,
  opts: {
    pushBanner: BannerPusher;
    successMessage: string;
    httpsRequiredMessage: string;
    failureMessage: string;
    allowInsecureContextBypass?: boolean;
  },
): Promise<boolean> => {
  const allowInsecureContextBypass =
    opts.allowInsecureContextBypass ?? canBypassClipboardHttpsRequirementForCurrentHost();

  const result = await copyTextToClipboard(text, { allowInsecureContextBypass });
  if (result === 'success') {
    opts.pushBanner(opts.successMessage, { tone: 'info' });
    return true;
  }
  if (result === 'https_required') {
    opts.pushBanner(opts.httpsRequiredMessage, { tone: 'warning' });
    return false;
  }
  opts.pushBanner(opts.failureMessage, { tone: 'error' });
  return false;
};
