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

vectile is designed to be fully local and privacy-preserving: no server, no
cloud, and no network calls. Everything, including the embedding model, runs on
your machine. Security reports related to the handling of local data, the
indexing or parsing of documents, and the in-process model engine are in scope.

Out of scope:

- Issues with third-party libraries or tools used to build or run vectile
  (for example, Wails, Go, or MinGW), unless vectile itself is at fault
- Vulnerabilities in the content of documents you choose to index
