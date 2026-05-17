export type OsLogoRule = {
  match: (haystack: string) => boolean;
  src: string;
  mode: 'mask' | 'image';
  paint?: string;
};

// Termius assets under /public/system-logos/termius are extracted from the official desktop app.
export const OS_LOGO_RULES: OsLogoRule[] = [
  {
    match: (s) => s.includes('ubuntu'),
    src: '/system-logos/termius/ubuntu.svg',
    mode: 'mask',
    paint: '#E95420',
  },
  {
    match: (s) => s.includes('debian'),
    src: '/system-logos/termius/debian.svg',
    mode: 'mask',
    paint: '#A81D33',
  },
  {
    match: (s) => s.includes('centos'),
    src: '/system-logos/termius/centos.svg',
    mode: 'mask',
    paint:
      'conic-gradient(from 45deg, #932279 0deg 90deg, #262577 90deg 180deg, #E28C00 180deg 270deg, #3E7C3A 270deg 360deg)',
  },
  {
    match: (s) => s.includes('rocky'),
    src: '/system-logos/termius/rocky.svg',
    mode: 'mask',
    paint: '#10B981',
  },
  {
    match: (s) => s.includes('alma'),
    src: '/system-logos/termius/alma.svg',
    mode: 'mask',
    paint:
      'conic-gradient(from 30deg, #86DA2F 0deg 72deg, #24C2FF 72deg 144deg, #0069DA 144deg 216deg, #FF4649 216deg 288deg, #FFCB12 288deg 360deg)',
  },
  {
    match: (s) => s.includes('red hat') || s.includes('redhat') || s.includes('rhel'),
    src: '/system-logos/termius/redhat.svg',
    mode: 'mask',
    paint: '#EE0000',
  },
  {
    match: (s) => s.includes('fedora'),
    src: '/system-logos/termius/fedora.svg',
    mode: 'mask',
    paint: '#51A2DA',
  },
  {
    match: (s) => s.includes('amazon linux') || s.includes('amzn'),
    src: '/system-logos/termius/amazon-linux.svg',
    mode: 'mask',
    paint: '#FF9900',
  },
  {
    match: (s) => s.includes('arch'),
    src: '/system-logos/termius/arch.svg',
    mode: 'mask',
    paint: '#1793D1',
  },
  {
    match: (s) => s.includes('alpine'),
    src: '/system-logos/termius/alpine.svg',
    mode: 'mask',
    paint: '#0D597F',
  },
  {
    match: (s) => s.includes('open') && s.includes('suse'),
    src: '/system-logos/termius/suse.svg',
    mode: 'mask',
    paint: '#73BA25',
  },
  {
    match: (s) => s.includes('gentoo'),
    src: '/system-logos/termius/gentoo.svg',
    mode: 'mask',
    paint: '#54487A',
  },
  {
    match: (s) => s.includes('mageia'),
    src: '/system-logos/termius/mageia.svg',
    mode: 'mask',
    paint: '#2397D4',
  },
  {
    match: (s) => s.includes('netbsd'),
    src: '/system-logos/termius/netbsd.svg',
    mode: 'mask',
    paint: '#FF6600',
  },
  {
    match: (s) => s.includes('openbsd'),
    src: '/system-logos/termius/openbsd.svg',
    mode: 'mask',
    paint: '#F2CA30',
  },
  {
    match: (s) => s.includes('routeros') || s.includes('mikrotik'),
    src: '/system-logos/termius/routeros.svg',
    mode: 'mask',
    paint: '#2F6DB5',
  },
  { match: (s) => s.includes('mint'), src: '/system-logos/mint.ico', mode: 'image' },
  { match: (s) => s.includes('kali'), src: '/system-logos/kali.svg', mode: 'image' },
];
