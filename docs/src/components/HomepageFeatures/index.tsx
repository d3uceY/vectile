import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import styles from './styles.module.css';

type Row = {
  name: string;
  href: string;
  description: string;
};

const ROWS: Row[] = [
  {
    name: 'Search',
    href: '/docs/features/search',
    description: 'One query box, filters, results as cards',
  },
  {
    name: 'Library',
    href: '/docs/features/library',
    description: 'Every collection with its file and chunk counts',
  },
  {
    name: 'Browse',
    href: '/docs/features/browse',
    description: 'The chunks of one collection, read in a side pane',
  },
  {
    name: 'Index',
    href: '/docs/features/indexing',
    description: 'Incremental and full runs, progress, prune',
  },
  {
    name: 'Models',
    href: '/docs/features/models',
    description: 'The one file vectile cannot supply',
  },
  {
    name: 'Sources',
    href: '/docs/features/sources',
    description: 'Vaults, folders, Calibre libraries, repositories',
  },
  {
    name: 'Scanned PDFs',
    href: '/docs/features/ocr',
    description: 'The optional Tesseract plugin',
  },
  {
    name: 'AI assistants',
    href: '/docs/features/mcp',
    description: 'A local MCP server on 127.0.0.1',
  },
  {
    name: 'Settings',
    href: '/docs/features/settings',
    description: 'Model, chunking, search, sources, indexing, Connect',
  },
];

export default function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.section}>
      <div className={styles.inner}>
        <div className={styles.panel}>
          <div className={styles.panelHead}>
            <span className={styles.panelTitle}>Pages</span>
            <Link className={styles.panelAction} to="/docs/intro">
              Get started
            </Link>
          </div>

          <ul className={styles.rows}>
            {ROWS.map((row, index) => (
              <li
                key={row.name}
                className={styles.row}
                style={{animationDelay: `${140 + index * 28}ms`}}>
                <Link className={styles.rowLink} to={row.href}>
                  <span className={styles.rowName}>{row.name}</span>
                  <span className={styles.rowDesc}>{row.description}</span>
                </Link>
              </li>
            ))}
          </ul>
        </div>

        <div className={styles.note}>
          <p className={styles.noteText}>
            The model, the index, and your settings live in one app data folder.
            Nothing about your files leaves the machine.
          </p>
          <a
            className={styles.noteLink}
            href="https://github.com/d3uceY/vectile"
            target="_blank"
            rel="noopener noreferrer">
            GitHub
          </a>
        </div>
      </div>
    </section>
  );
}
