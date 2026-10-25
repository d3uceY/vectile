import type {ReactNode} from 'react';
import {useState, useEffect} from 'react';
import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import Layout from '@theme/Layout';
import HomepageFeatures from '@site/src/components/HomepageFeatures';

import styles from './index.module.css';

function HomepageHeader() {
  const heroUrl = useBaseUrl('/img/hero.png');
  const [latestVersion, setLatestVersion] = useState<string | null>(null);

  useEffect(() => {
    fetch('https://api.github.com/repos/d3uceY/vectile/releases/latest', {
      headers: {Accept: 'application/vnd.github+json'},
    })
      .then((r) => r.json())
      .then((data) => {
        if (data?.tag_name) setLatestVersion(data.tag_name);
      })
      .catch(() => {});
  }, []);

  return (
    <header className={styles.hero}>
      <div className={styles.heroInner}>
        <div className={styles.heroText}>
          <h1 className={styles.title}>
            Every file you own.
            <br />
            One search box.
          </h1>

          <p className={styles.subtitle}>
            Your notes, documents, books, and code, indexed on your machine and
            searched twice: once for the words, once for the meaning.
          </p>

          <div className={styles.actions}>
            <Link className={styles.primary} to="/docs/installation">
              Download
            </Link>
            <Link className={styles.secondary} to="/docs/intro">
              Read the docs
            </Link>
          </div>

          <div className={styles.meta}>
            {latestVersion && (
              <span className={styles.version}>{latestVersion}</span>
            )}
            <span className={styles.metaItem}>Windows · macOS · Linux</span>
            <span className={styles.metaItem}>Runs locally</span>
          </div>
        </div>

        <figure className={styles.figure}>
          <img
            src={heroUrl}
            alt="The vectile search view: a query for kubernetes rollout, with ranked result cards showing highlighted snippets, collection chips, and file paths"
            className={styles.image}
          />
        </figure>
      </div>
    </header>
  );
}

export default function Home(): ReactNode {
  return (
    <Layout
      title="vectile — search the files already on your machine"
      description="vectile indexes your notes, documents, books, and code into one local database, then searches it twice: exact words and meaning. No account, no server, no upload.">
      <HomepageHeader />
      <main>
        <HomepageFeatures />
      </main>
    </Layout>
  );
}
