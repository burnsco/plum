# intro

The big shift is this: **treat remote APIs as a last-mile confirmer, not your primary scanner.** Use local parsing, local indexes, batching, and caching first. OMDb and TVDB are useful, but if every scan depends on live lookups, you’ll bottleneck on network latency and rate limits. Tools like GuessIt are built specifically to extract title/season/episode/year straight from release names, and MediaInfo can pull embedded technical/tag metadata from files locally. ([GuessIt][1])

What I’d do for a Jellyfin-style pipeline:

1. **Fast local pre-parse before any API call**
   Parse filenames/folder names first:

   * movie: `Movie.Title.2023.1080p...`
   * show: `Show.Name.S02E05...`
   * anime/date-based specials/daily episodes
   * multi-episode files, season packs, specials

   Use a parser instead of ad-hoc regexes. GuessIt exists for exactly this, and there’s also a JS/WASM port now if your stack is Node/TS and you want speed in-process. ([GuessIt][1])

2. **Build a local title index**
   Yes, there are large datasets you can keep locally. The most practical one is **IMDb’s non-commercial datasets**, which include title basics and alternate titles, and IMDb explicitly says you can hold local copies for personal/non-commercial use subject to their license terms. Wikidata also publishes **weekly JSON dumps** that you can download and mine for aliases, language variants, franchise relationships, and external IDs. ([developer.imdb.com][2])

3. **Use local search first, remote enrichment second**
   Match parsed data against your local DB first:

   * normalized title
   * year proximity
   * type hint: movie vs series
   * season/episode presence
   * runtime range
   * country/language hints from filename or parent folder

   Then hit TMDB/TVDB only for the top 1–3 candidates to confirm artwork, overview, IDs, and episode maps. TMDB’s docs emphasize search/find flows for discovering records, so it fits well as a confirmer after your local narrowing step. ([The Movie Database (TMDB)][3])

4. **Cache aggressively**
   You want several layers:

   * parsed filename cache
   * file fingerprint cache keyed by path + size + mtime
   * metadata result cache keyed by normalized title/year/type
   * external-ID cache mapping your internal media item to IMDb/TMDB/TVDB IDs

   Once a file has been identified, rescans should be almost free unless name, size, or mtime changed.

5. **Fingerprint duplicates and hard cases**
   For files with ugly names like `video1.mkv`, use more than the filename:

   * MediaInfo fields
   * duration
   * resolution
   * audio languages
   * embedded title tags
   * subtitle names nearby
   * folder context

   OpenSubtitles supports searching by **moviehash** as well as by IMDb/TMDB IDs, which is useful as a fallback identification signal for otherwise terrible filenames. ([opensubtitles.stoplight.io][4])

6. **Precompute normalized aliases**
   Store many variants for every title:

   * stripped punctuation
   * no stopwords
   * romanized / accented / de-accented
   * `and` ↔ `&`
   * article shifting: `Office, The` ↔ `The Office`
   * country/year disambiguation
   * alternate titles from IMDb/Wikidata

   This alone will cut failed matches a lot, especially for foreign titles, anime, and remakes. IMDb alternate-title data plus Wikidata aliases are valuable here. ([developer.imdb.com][2])

7. **Separate movie and TV pipelines early**
   Do not run one generic matcher over everything. Detect likely type first:

   * if `SxxExx`, date-based episode, or season folders → TV path
   * if single title + year + quality tags → movie path

   TVMaze is also worth considering as a supplemental TV source; it has a free API and an **updates endpoint** specifically to avoid unnecessary repeated requests. ([TVmaze][5])

8. **Use confidence scoring**
   Score candidates with weighted signals:

   * exact normalized title match
   * year delta
   * episode format match
   * runtime similarity
   * source/type consistency
   * folder-name agreement
   * existing franchise/library context

   Then:

   * high confidence → auto-accept
   * medium → fetch remote confirmation
   * low → send to manual review queue

That’s the architecture side. In practice, the **best local datasets** to consider are:

* **IMDb non-commercial datasets** for titles and alternate titles, if your use fits the license. ([developer.imdb.com][2])
* **Wikidata dump** for aliases and cross-links to other IDs. ([Wikidata][6])
* Your own continuously built local cache/index from previous successful matches.
* Possibly **TVMaze** as a lightweight TV supplement, especially for updates and episode-focused lookups. ([TVmaze][5])

A few strong ideas beyond raw data sources:

* **Do event-driven scanning**, not full rescans. Use filesystem watchers and queue changed paths only.
* **Scan in stages**: filename parse first, then metadata probe, then remote enrichment only if needed.
* **Batch and parallelize carefully** with worker pools, but dedupe requests for the same normalized title.
* **Store canonical external IDs early**. Once one file in a series is matched, use that series ID to identify the rest much faster.
* **Exploit folder context**. If `/TV/The Office/Season 03/...`, do not search the universe again for every file.
* **Use release-group heuristics** for anime and scene names.
* **Prewarm popular titles** if you know what users usually add.

My blunt take:
**OMDb + TVDB alone is not the fastest foundation.** For speed, I’d build around:

* local filename parser
* local metadata/title index
* strong cache
* TMDB/TVDB/TVMaze only as confirmers
* optional hash fallback for ugly files

A very solid stack would be:

* **GuessIt / guessit-js** for filename parsing ([GuessIt][1])
* **MediaInfo** for local file metadata ([MediaArea][7])
* **IMDb local dataset** for title/AKA matching ([developer.imdb.com][2])
* **Wikidata dump** for alias enrichment and cross-IDs ([Wikidata][6])
* **TMDB + TVDB + TVMaze** as selective online confirmers ([The Movie Database (TMDB)][3])
* **OpenSubtitles hash search** as a fallback path ([opensubtitles.stoplight.io][4])

If you want, I can lay out a **full identification pipeline design** for your app in steps, including DB schema, matching score logic, cache tables, and a fast scan queue architecture in Go/TS.

[1]: https://guessit.readthedocs.io/en/0.9.4/?utm_source=chatgpt.com "GuessIt 0.9.4 documentation"
[2]: https://developer.imdb.com/non-commercial-datasets/?utm_source=chatgpt.com "IMDb Non-Commercial Datasets"
[3]: https://developer.themoviedb.org/docs/finding-data?utm_source=chatgpt.com "Finding Data"
[4]: https://opensubtitles.stoplight.io/docs/opensubtitles-api/a172317bd5ccc-search-for-subtitles?utm_source=chatgpt.com "Search for subtitles | Opensubtitles REST API Docs"
[5]: https://www.tvmaze.com/api?utm_source=chatgpt.com "TV API | TVmaze - Add TV information to your website or app."
[6]: https://www.wikidata.org/wiki/Wikidata%3ADatabase_download?utm_source=chatgpt.com "Wikidata:Database download"
[7]: https://mediaarea.net/MediaInfo?utm_source=chatgpt.com "MediaInfo"
