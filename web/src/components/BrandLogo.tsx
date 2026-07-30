import React from 'react';
import { defaultSiteBrand, displayLogoURL } from '@lib/siteBrandModel';
import { useSiteBrandStore } from '@stores/siteBrandStore';

interface Props {
  className?: string;
  alt?: string;
}

interface BrandImageProps {
  src: string;
  alt: string;
  className?: string;
  width?: number;
  height?: number;
}

const BrandImageAttempt: React.FC<BrandImageProps> = (props) => {
  const [failed, setFailed] = React.useState(false);
  const src = failed ? defaultSiteBrand.logo_url : props.src;

  return (
    <img
      {...props}
      src={src}
      onError={() => {
        if (src !== defaultSiteBrand.logo_url) setFailed(true);
      }}
    />
  );
};

export const BrandImage: React.FC<BrandImageProps> = (props) => (
  <BrandImageAttempt key={props.src} {...props} />
);

const BrandLogo: React.FC<Props> = ({ className = '', alt }) => {
  const brand = useSiteBrandStore((state) => state.brand);
  const resolvedAlt = alt ?? `${brand.topbar_text} logo`;
  const brandLogoURL = displayLogoURL(brand.logo_url);

  return (
    <span className={`relative inline-flex size-full ${className}`}>
      <BrandImage
        src={brandLogoURL}
        alt={resolvedAlt}
        width={64}
        height={64}
        className="size-full object-contain"
      />
    </span>
  );
};

export default BrandLogo;
