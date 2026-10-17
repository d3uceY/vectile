/**
 * Stamps the release tag into every file that carries the app version.
 *
 *   node scripts/stamp-version.mjs v1.2.3
 *
 * The binary itself reads its version from the build flag
 * `-ldflags "-X main.Version=..."`, so this only covers the files around it:
 * the installer, the bundle, and the packaged metadata. A file that stops
 * matching is an error, so a template update fails the release instead of
 * shipping a stale version.
 */
import { readFileSync, writeFileSync } from "node:fs";

const tag = (process.argv[2] ?? "").trim();
const match = /^v?(\d+)\.(\d+)\.(\d+)$/.exec(tag);
if (!match) {
  console.error(`stamp-version: expected a tag like v1.2.3, got "${tag}"`);
  process.exit(1);
}
const version = match.slice(1, 4).join(".");

const edits = [
  ["build/config.yml", /(^  version:\s*)"[^"]*"/m, `$1"${version}"`],
  ["build/windows/info.json", /("file_version":\s*)"[^"]*"/, `$1"${version}"`],
  ["build/windows/info.json", /("ProductVersion":\s*)"[^"]*"/, `$1"${version}"`],
  ["build/windows/wails.exe.manifest", /(name="com\.d3ucey\.vectile" version=")[^"]*"/, `$1${version}"`],
  ["build/windows/nsis/wails_tools.nsh", /(!define INFO_PRODUCTVERSION ")[^"]*"/, `$1${version}"`],
  ["build/linux/nfpm/nfpm.yaml", /(^version:\s*)"[^"]*"/m, `$1"${version}"`],
  ["build/darwin/Info.plist", /(<key>CFBundleVersion<\/key>\s*<string>)[^<]*(<\/string>)/, `$1${version}$2`],
  ["build/darwin/Info.plist", /(<key>CFBundleShortVersionString<\/key>\s*<string>)[^<]*(<\/string>)/, `$1${version}$2`],
];

const files = new Set();
for (const [file, find, replace] of edits) {
  const before = readFileSync(file, "utf8");
  // Test the pattern, not the result: stamping the version a file already has
  // is a no-op, so comparing before/after would read as "no match".
  if (!find.test(before)) {
    console.error(`stamp-version: no version field matched in ${file}`);
    process.exit(1);
  }
  writeFileSync(file, before.replace(find, replace));
  files.add(file);
}

console.log(`stamp-version: ${version} written to ${files.size} files`);
