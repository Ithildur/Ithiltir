interface DashReleaseNoteSection {
  title: string;
  items: string[];
}

export interface DashReleaseNote {
  version: string;
  publishedAt: string;
  releaseUrl: string;
  sections: DashReleaseNoteSection[];
}

const cleanText = (value: string | null | undefined): string =>
  (value ?? '')
    .replace(/\u200b/g, '')
    .replace(/\s+/g, ' ')
    .trim();

const releaseDateFromText = (value: string): string => {
  const match = value.match(/(?:发布日期|Release date)[:：]\s*(.+)$/i);
  return cleanText(match?.[1]);
};

const releasePath = '/Ithildur/Ithiltir/releases';

const cleanReleaseURL = (value: string | null | undefined): string => {
  const href = cleanText(value);
  if (!href) return '';

  try {
    const url = new URL(href, 'https://github.com');
    const validPath = url.pathname === releasePath || url.pathname.startsWith(`${releasePath}/`);
    if (url.protocol !== 'https:' || url.hostname !== 'github.com' || !validPath) return '';
    return url.toString();
  } catch {
    return '';
  }
};

export const parseDashReleaseNotes = (html: string): DashReleaseNote[] => {
  if (typeof DOMParser === 'undefined') return [];

  const doc = new DOMParser().parseFromString(html, 'text/html');
  const root = doc.querySelector('.theme-doc-markdown.markdown');
  if (!root) return [];

  const notes: DashReleaseNote[] = [];
  let inDashSection = false;
  let currentNote: DashReleaseNote | null = null;
  let currentSection: DashReleaseNoteSection | null = null;

  const commitNote = () => {
    if (!currentNote) return;
    notes.push({
      ...currentNote,
      sections: currentNote.sections.filter((section) => section.items.length > 0),
    });
    currentNote = null;
    currentSection = null;
  };

  Array.from(root.children).forEach((element) => {
    const tag = element.tagName.toLowerCase();
    const text = cleanText(element.textContent);

    if (tag === 'h2') {
      if (inDashSection) commitNote();
      inDashSection = text.toLowerCase() === 'dash';
      return;
    }

    if (!inDashSection) return;

    if (tag === 'h3') {
      commitNote();
      currentNote = {
        version: text,
        publishedAt: '',
        releaseUrl: '',
        sections: [],
      };
      return;
    }

    if (!currentNote) return;

    if (tag === 'p') {
      const publishedAt = releaseDateFromText(text);
      if (publishedAt) currentNote.publishedAt = publishedAt;

      const releaseLink = Array.from(element.querySelectorAll<HTMLAnchorElement>('a[href]'))
        .map((link) => cleanReleaseURL(link.getAttribute('href')))
        .find(Boolean);
      if (releaseLink) currentNote.releaseUrl = releaseLink;
      return;
    }

    if (tag === 'h4') {
      currentSection = { title: text, items: [] };
      currentNote.sections.push(currentSection);
      return;
    }

    if (tag === 'ul' && currentSection) {
      currentSection.items.push(
        ...Array.from(element.querySelectorAll('li'))
          .map((item) => cleanText(item.textContent))
          .filter(Boolean),
      );
    }
  });

  commitNote();
  return notes;
};
