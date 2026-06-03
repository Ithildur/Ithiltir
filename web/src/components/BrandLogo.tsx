import React from 'react';
import { useSiteBrandStore } from '@stores/siteBrandStore';

interface Props {
  className?: string;
  alt?: string;
}

const BrandLogo: React.FC<Props> = ({ className = '', alt }) => {
  const brand = useSiteBrandStore((state) => state.brand);
  const resolvedAlt = alt ?? `${brand.topbar_text} logo`;

  return (
    <span className={`relative inline-flex size-full ${className}`}>
      <img
        src={brand.logo_url}
        alt={resolvedAlt}
        width="64"
        height="64"
        className="size-full object-contain"
      />
    </span>
  );
};

export default BrandLogo;
