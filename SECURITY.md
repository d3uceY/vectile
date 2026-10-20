# Security Policy

## Supported Versions

vectile is a desktop application built from source and released as binaries. We
currently provide security support for the latest release. If you are running an
older version, please upgrade to the latest release before reporting an issue.

| Version      | Supported          |
| ------------ | ------------------ |
| latest       | :white_check_mark: |
| older        | :x:                |

## Reporting a Vulnerability

If you believe you have found a security vulnerability in vectile, please **do
not** open a public issue. Report it privately through GitHub's private
vulnerability reporting form:

https://github.com/d3uceY/vectile/security/advisories/new

The same form is available from the repository's **Security** tab via **Report
a vulnerability**. Reports arrive as a draft security advisory that only the
maintainers can see, so we can assess, discuss, and prepare a fix before any
public disclosure. You can request a CVE from the advisory once it is published.

Please include as much of the following information as possible:

- The version of vectile and the operating system you are running
- A description of the vulnerability and the potential impact
- Steps to reproduce the issue (if known)
- Any relevant logs, error messages, or proof-of-concept

You can expect an acknowledgement of your report within a few business days. We
will keep you informed of our progress toward a fix and a disclosure timeline.

## Scope

vectile is designed to be local and privacy-preserving. Indexing, parsing,
chunking, embedding, search, and OCR run in-process on your machine. No code
path sends document text, chunks, vectors, or library metadata anywhere. No
server component, no account, no cloud sync, no remote embedding service.

It is not a zero-network application. These are the only requests it makes:

| When | What | Host |
| --- | --- | --- |
| Every launch | Newest release tag, compared with the running version to offer an update. Sends no identifiers. No setting disables it for now. | `api.github.com` |
| You press download in Settings → Model → Get a model | The selected `.gguf`, streamed into the models folder. | `huggingface.co`, then a Hugging Face CDN (`*.cdn.hf.co`) for the bytes |
| You install the Tesseract OCR plugin in Settings | The platform archive and the release's `SHA256SUMS`, which the archive is verified against before unpacking. | `github.com` release assets, then `release-assets.githubusercontent.com` |

Blocking any host above breaks nothing else: the update check skips, and a
blocked download shows an error. Links (repository, release page, model catalog)
open in your default browser, never in-app. The optional MCP server is off by
default and binds to `127.0.0.1` only.

No analytics, telemetry, crash reporting, or background uploads.

Security reports related to the handling of local data, the indexing or parsing
of documents, the download and verification of models and OCR bundles, the MCP
listener, and the in-process model engine are in scope.

Out of scope:

- Issues with third-party libraries or tools used to build or run vectile
  (for example, Wails, Go, or MinGW), unless vectile itself is at fault
- Vulnerabilities in the content of documents you choose to index
