import {useMemo, useRef, useState} from 'react';
import type {CSSProperties} from 'react';
import styles from './SearchDemo.module.css';

/* The numbers below are the app's own defaults, so the demo ranks the same way
   the real thing does: reciprocal rank fusion over two result lists. */
const K = 60;
const VECTOR_WEIGHT = 0.7;
const FTS_WEIGHT = 0.3;
const SHOWN = 3;

type Hit = {
  title: string;
  path: string;
  collection: string;
  snippet: string;
  /** 1-based rank in the meaning-based list. Absent means it missed that list. */
  meaning?: number;
  /** 1-based rank in the exact-word list. Absent means it missed that list. */
  words?: number;
};

type Example = {
  chip: string;
  query: string;
  hits: Hit[];
};

const EXAMPLES: Example[] = [
  {
    chip: 'how do we ship changes safely',
    query: 'how do we ship changes safely',
    hits: [
      {
        title: 'Blue-green deploys',
        path: 'ops/blue-green-deploys.md',
        collection: 'notes',
        meaning: 1,
        words: 4,
        snippet:
          'Traffic moves to the new colour only after the smoke suite goes green. If anything fails, the router flips back in under a second.',
      },
      {
        title: 'Canary checklist',
        path: 'work/docs/canary-checklist.md',
        collection: 'work',
        meaning: 2,
        words: 6,
        snippet:
          'Roll out to one percent, watch the error rate for twenty minutes, then step to ten. Stop the rollout on any p99 regression.',
      },
      {
        title: 'Rollback runbook',
        path: 'ops/rollback-runbook.md',
        collection: 'notes',
        meaning: 4,
        words: 1,
        snippet:
          'Revert the release, drain the pool, and re-point the load balancer. Owned by whoever is on call.',
      },
    ],
  },
  {
    chip: 'why did the sourdough go flat',
    query: 'why did the sourdough go flat',
    hits: [
      {
        title: 'Starter log, week 14',
        path: 'baking/starter-log.md',
        collection: 'notes',
        meaning: 1,
        words: 3,
        snippet:
          'Fed at 8am with a 1:5:5 ratio and it still peaked before noon. Anything past twelve hours is out of sugar by the time it hits the dough.',
      },
      {
        title: 'Wild yeast population dynamics',
        path: 'papers/wild-yeast.md',
        collection: 'papers',
        meaning: 2,
        snippet:
          'A culture that has gone acidic favours Lactobacillus over the yeast that actually produces gas, which shows up as a rise that stalls halfway.',
      },
      {
        title: 'Oven notes',
        path: 'baking/oven-notes.md',
        collection: 'notes',
        meaning: 5,
        words: 2,
        snippet:
          'This oven runs twenty degrees hot at the top. Proof in the cool room and the dough holds its shape through the bake.',
      },
    ],
  },
  {
    chip: 'where is the retry backoff set',
    query: 'where is the retry backoff set',
    hits: [
      {
        title: 'retry.go',
        path: 'backend/core/net/retry.go',
        collection: 'vectile',
        meaning: 2,
        words: 1,
        snippet:
          'func backoff(attempt int) time.Duration { base := 250 * time.Millisecond; return min(base<<attempt, maxBackoff) }',
      },
      {
        title: 'client.go',
        path: 'backend/core/net/client.go',
        collection: 'vectile',
        meaning: 4,
        words: 3,
        snippet:
          'The client reads its ceiling from the caller and never retries a request the server marked as terminal.',
      },
      {
        title: 'Rate limits and retries',
        path: 'docs/ops/rate-limits.md',
        collection: 'work',
        meaning: 7,
        words: 2,
        snippet:
          'Every outbound call gets a jittered wait so a fleet of clients does not wake up at the same instant.',
      },
    ],
  },
];

type Scored = Hit & {
  percent: number;
  /** Share of the fused score that came from the meaning list, 0 to 1. */
  meaningSplit: number;
};

function fuse(hits: Hit[]): Scored[] {
  const scored = hits.map((hit) => {
    const fromMeaning = hit.meaning ? VECTOR_WEIGHT / (K + hit.meaning) : 0;
    const fromWords = hit.words ? FTS_WEIGHT / (K + hit.words) : 0;
    const total = fromMeaning + fromWords;
    return {...hit, total, meaningSplit: total > 0 ? fromMeaning / total : 0};
  });

  const best = Math.max(...scored.map((hit) => hit.total), 0.000001);

  return scored
    .sort((a, b) => b.total - a.total)
    .slice(0, SHOWN)
    .map((hit) => ({
      title: hit.title,
      path: hit.path,
      collection: hit.collection,
      snippet: hit.snippet,
      meaning: hit.meaning,
      words: hit.words,
      percent: Math.round((hit.total / best) * 100),
      meaningSplit: hit.meaningSplit,
    }));
}

export default function SearchDemo() {
  const [active, setActive] = useState(0);
  const chips = useRef<(HTMLButtonElement | null)[]>([]);
  const example = EXAMPLES[active];
  const results = useMemo(() => fuse(example.hits), [example]);

  function step(from: number, delta: number) {
    const next = (from + delta + EXAMPLES.length) % EXAMPLES.length;
    setActive(next);
    chips.current[next]?.focus();
  }

  return (
    <div className={styles.card}>
      <div className={styles.cardHead}>
        <span className={styles.windowDots} aria-hidden="true">
          <i />
          <i />
          <i />
        </span>
        <span className={styles.cardLabel}>Search</span>
      </div>

      <div className={styles.queryRow}>
        <span className={styles.queryIcon} aria-hidden="true">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9">
            <circle cx="10.5" cy="10.5" r="6.25" />
            <path d="M15.4 15.4 20 20" strokeLinecap="round" />
          </svg>
        </span>
        <span className={styles.query} key={example.query}>
          {example.query}
        </span>
        <span className={styles.caret} aria-hidden="true" />
      </div>

      <div className={styles.chips} role="group" aria-label="Example searches">
        {EXAMPLES.map((item, index) => (
          <button
            key={item.chip}
            ref={(el) => {
              chips.current[index] = el;
            }}
            type="button"
            className={styles.chip}
            aria-pressed={index === active}
            onClick={() => setActive(index)}
            onKeyDown={(event) => {
              if (event.key === 'ArrowRight') {
                event.preventDefault();
                step(index, 1);
              } else if (event.key === 'ArrowLeft') {
                event.preventDefault();
                step(index, -1);
              }
            }}>
            {item.chip}
          </button>
        ))}
      </div>

      <ul className={styles.results} aria-live="polite" aria-atomic="false">
        {results.map((hit, index) => (
          <li
            key={hit.path}
            className={styles.result}
            style={{'--step': `${index * 55}ms`} as CSSProperties}>
            <div className={styles.resultHead}>
              <span className={styles.rank}>#{index + 1}</span>
              <span className={styles.title}>{hit.title}</span>
              <span className={styles.percent}>{hit.percent}%</span>
            </div>
            <div
              className={styles.bar}
              style={
                {
                  '--split': `${Math.round(hit.meaningSplit * 100)}%`,
                } as CSSProperties
              }
              aria-hidden="true"
            />
            <p className={styles.snippet}>{hit.snippet}</p>
            <div className={styles.meta}>
              <span className={styles.path}>{hit.path}</span>
              <span className={styles.dot} aria-hidden="true" />
              <span className={styles.collection}>{hit.collection}</span>
              <span className={styles.ranks}>
                {[
                  hit.meaning ? `meaning #${hit.meaning}` : null,
                  hit.words ? `words #${hit.words}` : null,
                ]
                  .filter(Boolean)
                  .join(' · ')}
              </span>
            </div>
          </li>
        ))}
      </ul>

      <div className={styles.legend}>
        <span>
          <i className={styles.swatchMeaning} aria-hidden="true" />
          meaning, weight 0.7
        </span>
        <span>
          <i className={styles.swatchWords} aria-hidden="true" />
          exact words, weight 0.3
        </span>
        <span className={styles.legendK}>rrf k 60</span>
      </div>
    </div>
  );
}
