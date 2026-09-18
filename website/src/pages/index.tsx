import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';
import {useBaseUrlUtils} from '@docusaurus/useBaseUrl';
import type {CSSProperties} from 'react';

import Reveal from '../components/Reveal';
import SearchDemo from '../components/SearchDemo';
import styles from './index.module.css';

function delay(ms: number): CSSProperties {
  return {'--d': `${ms}ms`} as CSSProperties;
}

const SOURCES = [
  {
    name: 'Obsidian vaults',
    body: 'Notes with frontmatter, tags, and wikilinks. Wikilink targets are kept, so a note is findable through the notes it points at.',
    meta: '.md',
  },
  {
    name: 'Project folders',
    body: 'Any folder of documents, each file parsed by its extension and split by its own structure.',
    meta: '.pdf .docx .html .csv .json .xlsx .pptx .ipynb .epub',
  },
  {
    name: 'Code repositories',
    body: 'Split with tree-sitter, so a function or a class arrives whole instead of as half of one. Commit history is indexed as its own source.',
    meta: 'tree-sitter · git log',
  },
  {
    name: 'Calibre libraries',
    body: 'Book metadata and text: title, author, tags, series, publisher, and the contents of EPUB and PDF files.',
    meta: 'epub · pdf',
  },
];

const PIPELINE = [
  {label: 'Your files', note: 'on disk, untouched'},
  {label: 'Parser and chunker', note: 'per format, per structure'},
  {label: 'Embedder', note: 'llama.cpp, in process'},
  {label: 'One SQLite file', note: 'vectors, text, cache'},
  {label: 'Rank fusion', note: 'two lists, one order'},
];

const DETAILS = [
  {
    title: 'One keystroke to the search box',
    body: 'Ctrl+K or Cmd+K works from every view, including Settings. There is no navigation step between being in the app and being in a query.',
  },
  {
    title: 'Delete one chunk, not the whole file',
    body: 'Browse lists chunks, and a selection bar above the list stays put while you scroll. Remove the four paragraphs of a stale document that were wrong, and keep the rest.',
  },
  {
    title: 'The second search costs nothing',
    body: 'Query vectors are cached per model inside the database, so repeating a search skips the model entirely. It empties itself when reindexing, pruning, or switching models could make it wrong.',
  },
  {
    title: 'Assistants can read your library, on your terms',
    body: 'A local MCP server exposes search to Claude or any client on the same machine. It binds to loopback, and the index and prune tools stay off until you turn them on.',
  },
  {
    title: 'It waits in the tray',
    body: 'Closing the window hides vectile instead of quitting. The tray menu carries the live status, per-collection index actions, and cancel.',
  },
  {
    title: 'Vexter',
    body: 'A pixel dinosaur lives in the sidebar footer and pokes up while a query runs, while a library indexes, and when a search comes up empty. Each moment has its own switch, and there is an off switch for all three.',
  },
];

const SCREENS = [
  {
    src: 'screenshots/library.png',
    alt: 'The Library view: collections with file counts, chunk counts, and last indexed dates',
    caption: 'Library, every collection with what is in it',
  },
  {
    src: 'screenshots/browse.png',
    alt: 'The Browse view: a paged chunk stream grouped by file with a reading pane',
    caption: 'Browse, chunks grouped by file with a reading pane',
  },
];

function Hero() {
  const {withBaseUrl} = useBaseUrlUtils();

  return (
    <header className={styles.hero}>
      <div className={styles.heroInner}>
        <div className={styles.heroCopy}>
          <p className={styles.eyebrow} style={delay(0)}>
            <span className={styles.pulse} aria-hidden="true" />
            local · open source · no account
          </p>

          <h1 className={styles.heroTitle} style={delay(110)}>
            Find it by <mark className={styles.mark}>meaning</mark>.
          </h1>

          <p className={styles.heroLede} style={delay(260)}>
            vectile reads your notes, documents, books, and code into one local database, then
            searches it two ways at once: the words you typed, and what you actually meant. No
            server, no account, and nothing leaving the machine.
          </p>

          <div className={styles.heroActions} style={delay(390)}>
            <Link className={styles.primary} to="/docs/download">
              Download vectile
            </Link>
            <Link className={styles.ghost} to="/docs/what-is-vectile">
              Read the docs
            </Link>
          </div>

          <p className={styles.heroMicro} style={delay(500)}>
            MIT licensed · Windows · macOS · Linux
          </p>
        </div>

        <div className={styles.heroDemo} style={delay(230)}>
          <SearchDemo />
          <p className={styles.demoNote}>
            A real query, ranked the way the app ranks it. Pick another one.
          </p>
        </div>
      </div>

      <div className={styles.scrollCue} aria-hidden="true">
        <span>scroll</span>
        <i />
      </div>
    </header>
  );
}

function Fusion() {
  return (
    <section className={styles.section} id="how">
      <div className={styles.sectionInner}>
        <Reveal>
          <p className={styles.sectionLabel}>01 / how it searches</p>
        </Reveal>
        <div className={styles.twoCol}>
          <Reveal>
            <h2 className={styles.h2}>Every query runs twice.</h2>
          </Reveal>
          <Reveal delay={80}>
            <div className={styles.prose}>
              <p>
                One search matches the exact words you typed against an FTS5 index. It is fast,
                literal, and the reason a specific error string turns up immediately.
              </p>
              <p>
                The other embeds your query and looks for passages pointing the same way. Two
                stages keep it quick: a binary-quantized index picks a candidate pool, then the
                exact vectors are fetched and reranked by distance.
              </p>
              <p>
                Two ranked lists is a problem, because a vector distance and a relevance score
                are not comparable numbers. Positions are. Reciprocal rank fusion blends the
                ranks, weighting each side, which is why a note that never uses your words can
                still come first.
              </p>
            </div>
          </Reveal>
        </div>

        <Reveal delay={120}>
          <div className={styles.formulaRow}>
            <pre className={styles.formula}>
              <span className={styles.formulaTag}>reciprocal rank fusion</span>
              <code>{`score(d) = 0.7 / (60 + rank_meaning(d))
              + 0.3 / (60 + rank_words(d))`}</code>
            </pre>
            <div className={styles.formulaNotes}>
              <p>
                Both weights and <code>k</code> are editable in Settings. Set a weight to zero
                and that path disappears, which is a quick way to see what pure keyword search
                would have returned.
              </p>
              <Link className={styles.inlineLink} to="/docs/search">
                How search works
              </Link>
            </div>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

function Sources() {
  return (
    <section className={styles.section} id="sources">
      <div className={styles.sectionInner}>
        <Reveal>
          <p className={styles.sectionLabel}>02 / what goes in</p>
        </Reveal>
        <Reveal>
          <h2 className={styles.h2}>Point it at what you already keep.</h2>
        </Reveal>
        <Reveal delay={60}>
          <p className={styles.sectionLede}>
            Four source types. You add paths in Settings, index, and forget about it until the
            next time you need something.
          </p>
        </Reveal>

        <ol className={styles.ledger}>
          {SOURCES.map((source, index) => (
            <Reveal key={source.name} delay={index * 60}>
              <li className={styles.ledgerRow}>
                <span className={styles.ledgerNum}>{String(index + 1).padStart(2, '0')}</span>
                <div className={styles.ledgerBody}>
                  <h3 className={styles.h3}>{source.name}</h3>
                  <p>{source.body}</p>
                </div>
                <span className={styles.ledgerMeta}>{source.meta}</span>
              </li>
            </Reveal>
          ))}
        </ol>

        <Reveal delay={120}>
          <p className={styles.footnote}>
            Files get hashed, so a second index run only touches what actually changed. Read the
            full list of formats on <Link to="/docs/sources">what you can index</Link>.
          </p>
        </Reveal>
      </div>
    </section>
  );
}

function Engine() {
  return (
    <section className={styles.band} id="engine">
      <div className={styles.sectionInner}>
        <Reveal>
          <p className={styles.bandLabel}>03 / how it runs</p>
        </Reveal>
        <Reveal>
          <h2 className={styles.bandH2}>One process. One file. No server to start.</h2>
        </Reveal>

        <Reveal delay={80}>
          <ol className={styles.pipeline}>
            {PIPELINE.map((node) => (
              <li key={node.label} className={styles.pipelineNode}>
                <span className={styles.pipelineLabel}>{node.label}</span>
                <span className={styles.pipelineNote}>{node.note}</span>
              </li>
            ))}
          </ol>
        </Reveal>

        <div className={styles.bandFacts}>
          {[
            {
              figure: '1 process',
              body: 'The embedder, the indexer, and search all live inside the app. Nothing to start, nothing to supervise, nothing to babysit.',
            },
            {
              figure: '1 file',
              body: 'Your entire index is a single SQLite database. Back it up by copying it, reset it by deleting it.',
            },
            {
              figure: '0 requests',
              body: 'Searching and indexing never touch the network. The one thing the app asks for on its own is a release check.',
            },
          ].map((fact, index) => (
            <Reveal key={fact.figure} delay={index * 70}>
              <div className={styles.bandFact}>
                <p className={styles.bandFigure}>{fact.figure}</p>
                <p className={styles.bandBody}>{fact.body}</p>
              </div>
            </Reveal>
          ))}
        </div>

        <Reveal>
          <p className={styles.bandFootnote}>
            Wails v3 · Go · SolidJS · SQLite with vectors and FTS5 ·{' '}
            <Link className={styles.bandLink} to="/docs/architecture">
              how it is built
            </Link>
          </p>
        </Reveal>
      </div>
    </section>
  );
}

function Screens() {
  const {withBaseUrl} = useBaseUrlUtils();

  return (
    <section className={styles.section} id="look">
      <div className={styles.sectionInner}>
        <Reveal>
          <p className={styles.sectionLabel}>04 / the app</p>
        </Reveal>
        <Reveal>
          <h2 className={styles.h2}>Five views, keyboard first.</h2>
        </Reveal>

        <Reveal delay={60}>
          <figure className={styles.shotWide}>
            <img
              src={withBaseUrl('screenshots/search.png')}
              alt="Searching a library for 'kubernetes rollout', with ranked result cards showing snippets, collections, and file paths"
              loading="lazy"
            />
            <figcaption>
              Search. Results carry the rank, the collection, the path, and the passage.
            </figcaption>
          </figure>
        </Reveal>

        <div className={styles.shotGrid}>
          {SCREENS.map((shot, index) => (
            <Reveal key={shot.src} delay={index * 70}>
              <figure className={styles.shot}>
                <img src={withBaseUrl(shot.src)} alt={shot.alt} loading="lazy" />
                <figcaption>{shot.caption}</figcaption>
              </figure>
            </Reveal>
          ))}
        </div>

        <Reveal delay={60}>
          <figure className={styles.shotWide}>
            <img
              src={withBaseUrl('screenshots/settings.png')}
              alt="The Settings view: model, chunking, search, cache, sources, indexing, Vexter, and Connect"
              loading="lazy"
            />
            <figcaption>
              Settings, a rail of eight sections, from the model to the local MCP server.
            </figcaption>
          </figure>
        </Reveal>
      </div>
    </section>
  );
}

function Details() {
  const {withBaseUrl} = useBaseUrlUtils();

  return (
    <section className={styles.section} id="details">
      <div className={styles.sectionInner}>
        <Reveal>
          <p className={styles.sectionLabel}>05 / the small things</p>
        </Reveal>
        <div className={styles.twoCol}>
          <Reveal>
            <h2 className={styles.h2}>Craft lives in the parts nobody screenshots.</h2>
            <figure className={styles.vexterStage}>
              <img
                src={withBaseUrl('img/vectile-mascot.webp')}
                alt="Vexter, the pixel dinosaur that lives in the sidebar"
                width={84}
                height={84}
                loading="lazy"
              />
              <figcaption>Vexter, on the job</figcaption>
            </figure>
          </Reveal>
          <Reveal delay={80}>
            <p className={styles.sectionLede}>
              A search box you can reach without thinking. Deletes that remove what you selected
              and nothing else. A cache that clears itself when it could be wrong. None of this
              shows up in a feature list, and all of it is the difference between a tool you use
              once and a tool you keep open.
            </p>
          </Reveal>
        </div>

        <dl className={styles.details}>
          {DETAILS.map((detail, index) => (
            <Reveal key={detail.title} delay={index * 50}>
              <div className={styles.detailRow}>
                <dt>{detail.title}</dt>
                <dd>{detail.body}</dd>
              </div>
            </Reveal>
          ))}
        </dl>
      </div>
    </section>
  );
}

function Cta() {
  return (
    <section className={styles.cta}>
      <div className={styles.sectionInner}>
        <Reveal>
          <p className={styles.sectionLabel}>06 / install</p>
        </Reveal>
        <Reveal>
          <h2 className={styles.ctaTitle}>
            Install it and point it at a folder.
          </h2>
        </Reveal>
        <Reveal delay={80}>
          <p className={styles.ctaBody}>
            Download an embedding model from the catalog inside the app, add one source, and run
            an index. That is the whole setup.
          </p>
        </Reveal>
        <Reveal delay={140}>
          <div className={styles.ctaActions}>
            <Link className={styles.primary} to="/docs/download">
              Choose your platform
            </Link>
            <Link className={styles.ghost} to="/docs/first-run">
              First run, step by step
            </Link>
          </div>
        </Reveal>
        <Reveal delay={200}>
          <ul className={styles.ctaPlatforms}>
            <li>
              <Link to="/docs/download#windows">Windows installer and portable exe</Link>
            </li>
            <li>
              <Link to="/docs/download#macos">macOS universal dmg</Link>
            </li>
            <li>
              <Link to="/docs/download#linux">Linux AppImage and deb</Link>
            </li>
          </ul>
        </Reveal>
      </div>
    </section>
  );
}

export default function Home() {
  return (
    <Layout
      title="Your private library, searchable on one machine"
      description="vectile indexes your notes, documents, books, and code into one local database and searches it by meaning and by exact words at the same time. Open source, offline, no account.">
      <main className={styles.page}>
        <Hero />
        <Fusion />
        <Sources />
        <Engine />
        <Screens />
        <Details />
        <Cta />
      </main>
    </Layout>
  );
}
