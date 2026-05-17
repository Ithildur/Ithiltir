type PreIdentifier = {
  value: string;
  numeric: boolean;
};

type ParsedVersion = {
  core: [string, string, string];
  pre: PreIdentifier[] | null;
};

const NUMERIC_RE = /^(0|[1-9]\d*)$/;
const IDENTIFIER_RE = /^[0-9A-Za-z-]+$/;

const isNumeric = (value: string): boolean => /^\d+$/.test(value);

const validNumber = (value: string): boolean => NUMERIC_RE.test(value);

const validIdentifiers = (value: string, checkNumber: boolean): boolean => {
  if (!value) return false;
  const identifiers = value.split('.');
  if (identifiers.some((item) => item === '' || !IDENTIFIER_RE.test(item))) return false;
  return !checkNumber || identifiers.every((item) => !isNumeric(item) || validNumber(item));
};

const compareNumber = (a: string, b: string): number => {
  if (a.length < b.length) return -1;
  if (a.length > b.length) return 1;
  if (a < b) return -1;
  if (a > b) return 1;
  return 0;
};

const compareIdentifier = (a: PreIdentifier, b: PreIdentifier): number => {
  if (a.numeric && b.numeric) return compareNumber(a.value, b.value);
  if (a.numeric) return -1;
  if (b.numeric) return 1;
  if (a.value < b.value) return -1;
  if (a.value > b.value) return 1;
  return 0;
};

export const parseVersion = (raw: string): ParsedVersion | null => {
  const value = raw.trim();
  if (!value || value.startsWith('v') || /\s/.test(value)) return null;

  const plusIndex = value.indexOf('+');
  const main = plusIndex >= 0 ? value.slice(0, plusIndex) : value;
  const build = plusIndex >= 0 ? value.slice(plusIndex + 1) : '';
  if (
    plusIndex >= 0 &&
    (value.indexOf('+', plusIndex + 1) >= 0 || !validIdentifiers(build, false))
  ) {
    return null;
  }

  const preIndex = main.indexOf('-');
  const core = preIndex >= 0 ? main.slice(0, preIndex) : main;
  const prerelease = preIndex >= 0 ? main.slice(preIndex + 1) : '';
  if (preIndex >= 0 && !validIdentifiers(prerelease, true)) return null;

  const coreParts = core.split('.');
  if (coreParts.length !== 3 || coreParts.some((part) => !validNumber(part))) return null;

  return {
    core: coreParts as [string, string, string],
    pre:
      preIndex >= 0
        ? prerelease.split('.').map((item) => ({ value: item, numeric: isNumeric(item) }))
        : null,
  };
};

export const compareVersions = (a: string, b: string): number | null => {
  const left = parseVersion(a);
  const right = parseVersion(b);
  if (!left || !right) return null;

  for (let i = 0; i < 3; i += 1) {
    const coreCompare = compareNumber(left.core[i], right.core[i]);
    if (coreCompare !== 0) return coreCompare;
  }

  if (!left.pre && !right.pre) return 0;
  if (!left.pre) return 1;
  if (!right.pre) return -1;

  const limit = Math.min(left.pre.length, right.pre.length);
  for (let i = 0; i < limit; i += 1) {
    const preCompare = compareIdentifier(left.pre[i], right.pre[i]);
    if (preCompare !== 0) return preCompare;
  }
  if (left.pre.length < right.pre.length) return -1;
  if (left.pre.length > right.pre.length) return 1;
  return 0;
};

export const isVersionOlder = (current: string, target: string): boolean =>
  compareVersions(current, target) === -1;
