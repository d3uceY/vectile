import useBaseUrl from '@docusaurus/useBaseUrl';
import type {ReactNode} from 'react';

/**
 * A documentation screenshot: framed image plus an optional caption.
 *
 * Wrap two of these in `<div className="doc-shots">` for a side by side row.
 */
export default function Shot({
  src,
  alt,
  caption,
}: {
  src: string;
  alt: string;
  caption?: ReactNode;
}) {
  return (
    <figure className="doc-shot">
      <img src={useBaseUrl(src)} alt={alt} loading="lazy" decoding="async" />
      {caption ? <figcaption>{caption}</figcaption> : null}
    </figure>
  );
}
