# Verifies the Black query set in sematic-test/queries.md.
#
# Rule (since the Black set was tightened): NO word of a Black query may exist
# in its target document. Because vectile's FTS5 table indexes both the content
# and the document title (which for these .txt files is the *filename*), a query
# word must be absent from the file body AND the filename-derived title tokens.
#
# Each row also carries a "headline" term (the salient word a keyword searcher
# would reach for) that must be absent from the ENTIRE corpus, so a plain FTS
# query for it returns nothing anywhere.
#
# Output: one line per row; FAIL marks any query word found in the target (body
# or title) or any headline term found anywhere in the corpus.
$ErrorActionPreference = 'Stop'
$dir = Join-Path $PSScriptRoot 'text documents'
$allFiles = Get-ChildItem $dir -Filter '*.txt' | Sort-Object Name
$corpus = ''
foreach ($f in $allFiles) { $corpus += (Get-Content $f.FullName -Raw).ToLower() + ' ' + $f.Name.ToLower() + ' ' }

# query -> target file -> headline term that must be absent from the whole corpus
$cands = @(
  @{ q = 'homemade marinara from ripe garden fruit';      t = '01-italian-sunday-sauce.txt'; kw = 'marinara' },
  @{ q = 'ripe yellow fruit quick breakfast cake';        t = '02-banana-bread.txt';          kw = 'cake' },
  @{ q = 'natural yeast raised from fermented grain culture'; t = '03-sourdough-starter.txt'; kw = 'yeast' },
  @{ q = 'cozy autumn chowder with root vegetables';      t = '04-fall-vegetable-soup.txt';   kw = 'chowder' },
  @{ q = 'HIIT sessions to raise your stamina';           t = '05-morning-intervals.txt';     kw = 'hiit' },
  @{ q = 'marathon fuel hydration plan';                  t = '06-long-run-prep.txt';         kw = 'marathon' },
  @{ q = 'mountain trail with sweeping scenery';          t = '07-weekend-hike.txt';          kw = 'mountain' },
  @{ q = 'airport hand luggage essentials';               t = '08-flight-packing.txt';        kw = 'airport' },
  @{ q = 'overnight camping kit that stays light';        t = '09-backpacking-gear.txt';      kw = 'camping' },
  @{ q = 'homegrown salsa from tiny patio planters';      t = '10-container-tomatoes.txt';    kw = 'salsa' },
  @{ q = 'vegan athletes diet strength building';         t = '11-plant-protein.txt';         kw = 'vegan' },
  @{ q = 'evening wind-down ritual for deep rest';        t = '12-sleep-hygiene.txt';         kw = 'ritual' },
  @{ q = 'office posture tips for long days on screen';   t = '13-ergonomic-desk.txt';        kw = 'posture' },
  @{ q = 'version control for personal projects';         t = '14-git-solo.txt';              kw = 'version' },
  @{ q = 'systems programming for terminal utilities';    t = '15-rust-cli.txt';              kw = 'terminal' },
  @{ q = 'dystopian novels envisioning future society';   t = '16-sci-fi-reading-list.txt';   kw = 'dystopian' },
  @{ q = 'cartoon marathon with family';                  t = '17-animated-movie-night.txt';  kw = 'cartoon' }
)

# Returns a set of word tokens (lowercased) from raw text, split like FTS5's
# unicode61 tokenizer: contiguous runs of letters/digits.
function Get-TokenSet([string]$text) {
  $set = @{}
  foreach ($m in [regex]::Matches($text.ToLower(), '[a-z0-9]+')) { $set[$m.Value] = $true }
  , $set
}

$fail = 0
foreach ($c in $cands) {
  $path = Join-Path $dir $c.t
  $bodyTokens  = Get-TokenSet (Get-Content $path -Raw)
  $titleTokens = Get-TokenSet $c.t                       # filename incl. extension is the FTS title
  $qtokens = [regex]::Matches($c.q.ToLower(), '[a-z0-9]+') | ForEach-Object { $_.Value }

  $bodyHits  = @($qtokens | Where-Object { $bodyTokens.ContainsKey($_) })
  $titleHits = @($qtokens | Where-Object { $titleTokens.ContainsKey($_) })

  $kwRe = '(?i)\b' + [regex]::Escape($c.kw) + '\b'
  $kwInCorpus = [regex]::IsMatch($corpus, $kwRe)

  if ($bodyHits.Count -or $titleHits.Count -or $kwInCorpus) { $fail++ }
  $status = if ($bodyHits.Count -or $titleHits.Count -or $kwInCorpus) { 'FAIL' } else { 'OK' }
  Write-Output ("{0}  {1,-10}  in-body: {2}  in-title: {3}  kw-in-corpus: {4}" -f `
    $status, $c.kw, $(if ($bodyHits.Count) { $bodyHits -join ',' } else { '-' }), `
    $(if ($titleHits.Count) { $titleHits -join ',' } else { '-' }), $kwInCorpus)
}
Write-Output ""
if ($fail -eq 0) {
  Write-Output "PASS: no Black query word exists in its target (body or title), and every headline term is absent from the whole corpus."
} else {
  Write-Output "FAIL: $fail row(s) broke the rule - see above."
}
exit $(if ($fail -eq 0) { 0 } else { 1 })
