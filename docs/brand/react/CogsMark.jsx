import React from 'react';

import iconUrl from '../icon.svg?url';
import logoAnimatedUrl from '../logo-animated.svg?url';
import spinnerUrl from '../spinner.svg?url';

function BrandObject({ src, size, className, label, square = true }) {
  const w = size;
  const h = square ? size : Math.round(size * 0.92);
  return (
    <object
      type="image/svg+xml"
      data={src}
      width={w}
      height={h}
      className={className}
      aria-label={label}
    />
  );
}

/** Loading gear — docs/brand/spinner.svg (SMIL, 1.4s). */
export function CogsSpinner({ size = 32, className = '', label }) {
  const aria = label || 'Loading';
  return (
    <span
      className={`cogs-spinner${className ? ` ${className}` : ''}`}
      role="status"
      aria-label={aria}
      style={{ width: size, height: size }}
    >
      <BrandObject src={spinnerUrl} size={size} label={aria} />
      {label ? <span className="cogs-mark__label">{label}</span> : null}
    </span>
  );
}

/** Animated wordmark — docs/brand/logo-animated.svg */
export function CogsLogo({ size = 96, className = '', label }) {
  return (
    <span
      className={`cogs-logo${className ? ` ${className}` : ''}`}
      role="img"
      aria-label={label || 'CoGS'}
      style={{ width: size, height: size }}
    >
      <BrandObject src={logoAnimatedUrl} size={size} label={label || 'CoGS'} />
    </span>
  );
}

/** Favicon-sized mark — docs/brand/icon.svg */
export function CogsIcon({ size = 32, className = '', label }) {
  return (
    <span
      className={`cogs-icon${className ? ` ${className}` : ''}`}
      role="img"
      aria-label={label || 'CoGS'}
      style={{ width: size, height: size }}
    >
      <BrandObject src={iconUrl} size={size} label={label || 'CoGS'} />
    </span>
  );
}

/**
 * Loading → spinner. Compact header → icon. Larger brand moments → animated logo.
 */
export function CogsMark({ size = 32, spin = false, className = '', label }) {
  if (spin) {
    return <CogsSpinner size={size} className={className} label={label} />;
  }
  if (size <= 36) {
    return <CogsIcon size={size} className={className} label={label} />;
  }
  return <CogsLogo size={size} className={className} label={label} />;
}
