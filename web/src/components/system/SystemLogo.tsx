import React from 'react';
import { FaApple, FaLinux, FaWindows } from 'react-icons/fa';
import type { NodeDeployPlatform } from '@app-types/api';
import { OS_LOGO_RULES } from '@config/osIcons';

type RuntimeSystem = {
  os?: string | null;
  platform?: string | null;
  kernelVersion?: string | null;
};

type LogoProps = {
  size?: number;
  className?: string;
};

const joinClass = (...parts: Array<string | undefined>) => parts.filter(Boolean).join(' ');

const ImageLogo: React.FC<LogoProps & { src: string }> = ({ src, size = 16, className = '' }) => (
  <span
    aria-hidden="true"
    className={joinClass('block shrink-0', className)}
    style={{ width: size, height: size }}
  >
    <img
      alt=""
      src={src}
      width={size}
      height={size}
      className="block size-full object-contain"
      style={{ display: 'block' }}
    />
  </span>
);

const MaskLogo: React.FC<LogoProps & { src: string; paint: string }> = ({
  src,
  paint,
  size = 16,
  className = '',
}) => (
  <span
    aria-hidden="true"
    className={joinClass('block shrink-0', className)}
    style={{
      width: size,
      height: size,
      display: 'block',
      background: paint,
      WebkitMaskImage: `url(${src})`,
      maskImage: `url(${src})`,
      WebkitMaskPosition: 'center',
      maskPosition: 'center',
      WebkitMaskRepeat: 'no-repeat',
      maskRepeat: 'no-repeat',
      WebkitMaskSize: 'contain',
      maskSize: 'contain',
    }}
  />
);

const LinuxLogo: React.FC<LogoProps> = ({ size = 16, className = '' }) => (
  <MaskLogo
    src="/system-logos/termius/linux.svg"
    paint="#FCC624"
    size={size}
    className={className}
  />
);

const WindowsLogo: React.FC<LogoProps> = ({ size = 16, className = '' }) => (
  <MaskLogo
    src="/system-logos/termius/windows.svg"
    paint="#0078D4"
    size={size}
    className={className}
  />
);

const MacOSLogo: React.FC<LogoProps> = ({ size = 16, className = '' }) => (
  <MaskLogo
    src="/system-logos/termius/macos.svg"
    paint="currentColor"
    size={size}
    className={joinClass('text-black dark:text-white', className)}
  />
);

const ProxmoxLogo: React.FC<LogoProps> = ({ size = 16, className = '' }) => (
  <ImageLogo src="/system-logos/proxmox.svg" size={size} className={className} />
);

export const PlatformLogo: React.FC<LogoProps & { platform: NodeDeployPlatform }> = ({
  platform,
  size = 16,
  className = '',
}) => {
  switch (platform) {
    case 'windows':
      return (
        <FaWindows
          aria-hidden="true"
          size={size}
          className={className}
          style={{ display: 'block' }}
        />
      );
    case 'macos':
      return (
        <FaApple
          aria-hidden="true"
          size={size}
          className={className}
          style={{ display: 'block' }}
        />
      );
    case 'linux':
    default:
      return (
        <FaLinux
          aria-hidden="true"
          size={size}
          className={className}
          style={{ display: 'block' }}
        />
      );
  }
};

export const SystemLogo: React.FC<LogoProps & { system: RuntimeSystem }> = ({
  system,
  size = 16,
  className = '',
}) => {
  const haystack = `${system.os ?? ''} ${system.platform ?? ''} ${system.kernelVersion ?? ''}`
    .trim()
    .toLowerCase();

  if (haystack.includes('windows')) return <WindowsLogo size={size} className={className} />;
  if (haystack.includes('proxmox') || haystack.includes(' pve') || haystack.includes('-pve')) {
    return <ProxmoxLogo size={size} className={className} />;
  }
  if (haystack.includes('mac') || haystack.includes('os x') || haystack.includes('darwin')) {
    return <MacOSLogo size={size} className={className} />;
  }

  const rule = OS_LOGO_RULES.find((item) => item.match(haystack));
  if (!rule) return <LinuxLogo size={size} className={className} />;

  if (rule.mode === 'mask' && rule.paint) {
    return <MaskLogo src={rule.src} paint={rule.paint} size={size} className={className} />;
  }

  return <ImageLogo src={rule.src} size={size} className={className} />;
};
