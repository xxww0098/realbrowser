import React from 'react';

export interface RealBrowserIconProps extends React.SVGProps<SVGSVGElement> {
  size?: number | string;
  variant?: 'mark' | 'white-bg' | 'squircle';
}

/** Apple icon mask in 512 viewBox: quintic superellipse (n≈5), not CSS border-radius. */
const APPLE_ICON_SQUIRCLE =
  'M256,0 L170,0.22 L142.62,0.88 L122.85,1.98 L106.91,3.53 L93.42,5.52 L81.67,7.98 L71.28,10.91 L61.99,14.31 L53.63,18.22 L46.1,22.64 L39.29,27.6 L33.14,33.14 L27.6,39.29 L22.64,46.1 L18.22,53.63 L14.31,61.99 L10.91,71.28 L7.98,81.67 L5.52,93.42 L3.53,106.91 L1.98,122.85 L0.88,142.62 L0.22,170 L0,256 L0.22,342 L0.88,369.38 L1.98,389.15 L3.53,405.09 L5.52,418.58 L7.98,430.33 L10.91,440.72 L14.31,450.01 L18.22,458.37 L22.64,465.9 L27.6,472.71 L33.14,478.86 L39.29,484.4 L46.1,489.36 L53.63,493.78 L61.99,497.69 L71.28,501.09 L81.67,504.02 L93.42,506.48 L106.91,508.47 L122.85,510.02 L142.62,511.12 L170,511.78 L256,512 L342,511.78 L369.38,511.12 L389.15,510.02 L405.09,508.47 L418.58,506.48 L430.33,504.02 L440.72,501.09 L450.01,497.69 L458.37,493.78 L465.9,489.36 L472.71,484.4 L478.86,478.86 L484.4,472.71 L489.36,465.9 L493.78,458.37 L497.69,450.01 L501.09,440.72 L504.02,430.33 L506.48,418.58 L508.47,405.09 L510.02,389.15 L511.12,369.38 L511.78,342 L512,256 L511.78,170 L511.12,142.62 L510.02,122.85 L508.47,106.91 L506.48,93.42 L504.02,81.67 L501.09,71.28 L497.69,61.99 L493.78,53.63 L489.36,46.1 L484.4,39.29 L478.86,33.14 L472.71,27.6 L465.9,22.64 L458.37,18.22 L450.01,14.31 L440.72,10.91 L430.33,7.98 L418.58,5.52 L405.09,3.53 L389.15,1.98 L369.38,0.88 L342,0.22 Z';

function FacetedRMark({ uid }: { uid: string }) {
  return (
    <>
      <polygon points="262.57,48 435.54,153.09 433.35,262.57 321.68,328.25 439.92,407.07 358.91,450.86 229.73,376.42 229.73,271.33 286.65,240.67 347.96,258.19 347.96,203.45 256,146.53 164.04,203.45 164.04,411.45 80.84,464 72.08,459.62 72.08,153.09" fill="#0A5FFB" />
      <polygon points="72.08,153.09 164.04,203.45 164.04,411.45 80.84,464 72.08,459.62" fill={`url(#${uid}_stem)`} />
      <polygon points="72.08,153.09 262.57,48 256,146.53 164.04,203.45" fill={`url(#${uid}_roof_l)`} />
      <polygon points="262.57,48 435.54,153.09 347.96,203.45 256,146.53" fill={`url(#${uid}_roof_r)`} />
      <polygon points="435.54,153.09 433.35,262.57 347.96,258.19 347.96,203.45" fill={`url(#${uid}_bowl)`} />
      <polygon points="433.35,262.57 321.68,328.25 347.96,258.19" fill={`url(#${uid}_fold)`} />
      <polygon points="286.65,240.67 347.96,258.19 321.68,328.25 229.73,271.33" fill={`url(#${uid}_waist)`} />
      <polygon points="229.73,271.33 321.68,328.25 229.73,376.42" fill={`url(#${uid}_leg)`} />
      <polygon points="321.68,328.25 439.92,407.07 358.91,450.86 229.73,376.42" fill={`url(#${uid}_leg)`} />
    </>
  );
}

/**
 * RealBrowser faceted isometric 'R' mark.
 * `squircle` uses Apple's icon superellipse mask. macOS .icns is pre-masked the same way.
 */
export const RealBrowserIcon: React.FC<RealBrowserIconProps> = ({
  size = 24,
  variant = 'mark',
  className,
  style,
  ...props
}) => {
  const uid = React.useId().replace(/:/g, '');
  const isWhiteBg = variant === 'white-bg';
  const isSquircle = variant === 'squircle';

  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 512 512"
      width={size}
      height={size}
      fill="none"
      shapeRendering="geometricPrecision"
      className={className}
      style={style}
      {...props}
    >
      <defs>
        <linearGradient id={`${uid}_stem`} x1="72" y1="153" x2="164" y2="464" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#1B78FE" />
          <stop offset="45%" stopColor="#0A5FFB" />
          <stop offset="100%" stopColor="#0550F6" />
        </linearGradient>
        <linearGradient id={`${uid}_roof_l`} x1="72" y1="153" x2="262" y2="48" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#2A82FD" />
          <stop offset="100%" stopColor="#4B9AFD" />
        </linearGradient>
        <linearGradient id={`${uid}_roof_r`} x1="256" y1="48" x2="436" y2="153" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#6BB0FE" />
          <stop offset="55%" stopColor="#4DA0FD" />
          <stop offset="100%" stopColor="#2F86FC" />
        </linearGradient>
        <linearGradient id={`${uid}_bowl`} x1="348" y1="153" x2="436" y2="263" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#4CA0FE" />
          <stop offset="100%" stopColor="#2B84FD" />
        </linearGradient>
        <linearGradient id={`${uid}_fold`} x1="348" y1="258" x2="322" y2="328" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#1A72FC" />
          <stop offset="100%" stopColor="#0A48D6" />
        </linearGradient>
        <linearGradient id={`${uid}_waist`} x1="230" y1="241" x2="348" y2="328" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#1A78FE" />
          <stop offset="100%" stopColor="#0A5AF4" />
        </linearGradient>
        <linearGradient id={`${uid}_leg`} x1="230" y1="328" x2="440" y2="451" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#0E64FB" />
          <stop offset="100%" stopColor="#044AEF" />
        </linearGradient>
        {isSquircle && (
          <clipPath id={`${uid}_apple`} clipPathUnits="userSpaceOnUse">
            <path d={APPLE_ICON_SQUIRCLE} />
          </clipPath>
        )}
      </defs>

      {isWhiteBg && <rect width="512" height="512" fill="#FFFFFF" />}

      {isSquircle ? (
        <g clipPath={`url(#${uid}_apple)`}>
          <rect width="512" height="512" fill="#FFFFFF" />
          <g transform="translate(256 256) scale(0.78) translate(-256 -256)">
            <FacetedRMark uid={uid} />
          </g>
        </g>
      ) : (
        <g>
          <FacetedRMark uid={uid} />
        </g>
      )}
    </svg>
  );
};

export default RealBrowserIcon;
