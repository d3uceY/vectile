import {useEffect, useRef} from 'react';
import type {CSSProperties, ReactNode} from 'react';

/* Turned on at import time so the first paint already has the start state.
   Without it, elements would render visible and then jump to their offset. */
if (typeof document !== 'undefined') {
  document.documentElement.classList.add('vt-anim-ready');
}

export default function Reveal({
  children,
  delay = 0,
  className,
}: {
  children: ReactNode;
  delay?: number;
  className?: string;
}) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el || typeof IntersectionObserver === 'undefined') return;

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            entry.target.classList.add('is-in');
            observer.disconnect();
          }
        }
      },
      {threshold: 0.06, rootMargin: '0px 0px -6% 0px'},
    );

    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  return (
    <div
      ref={ref}
      className={['vt-reveal', className].filter(Boolean).join(' ')}
      style={{'--vt-reveal-delay': `${delay}ms`} as CSSProperties}>
      {children}
    </div>
  );
}
