import clsx from 'clsx';
import {
  CogsMark as CogsMarkBase,
  CogsSpinner as CogsSpinnerBase,
  CogsLogo as CogsLogoBase,
  CogsIcon as CogsIconBase,
} from '@cogs/brand/CogsMark.jsx';

function wrap(Wrapped, className, props) {
  return (
    <Wrapped {...props} className={clsx('inline-flex flex-col items-center gap-2', className)} />
  );
}

/** Todo app wrapper — keeps tailwind layout helpers on the mark. */
export function CogsMark(props) {
  return wrap(CogsMarkBase, props.className, props);
}

export function CogsSpinner(props) {
  return wrap(CogsSpinnerBase, props.className, props);
}

export function CogsLogo(props) {
  return wrap(CogsLogoBase, props.className, props);
}

export function CogsIcon(props) {
  return wrap(CogsIconBase, props.className, props);
}
