package metadata

import "testing"

func TestParseFilename_IgnoresResolutionLikeSeasonEpisodePattern(t *testing.T) {
	info := ParseFilename("Anime Show 1440x1080.mkv")
	if info.Season != 0 || info.Episode != 0 {
		t.Fatalf("unexpected season/episode parse: %+v", info)
	}
}

func TestParseFilename_AnimeFlatRelease(t *testing.T) {
	info := ParseFilename("[SubsPlease] Frieren - 12 [1080p].mkv")
	if info.Title != "frieren" {
		t.Fatalf("title = %q", info.Title)
	}
	if info.Season != 0 || info.Episode != 0 || info.AbsoluteEpisode != 12 {
		t.Fatalf("unexpected episode info: %+v", info)
	}
}

func TestParseFilename_AnimeSpecial(t *testing.T) {
	info := ParseFilename("[Group] Show OVA 02.mkv")
	if !info.IsSpecial {
		t.Fatal("expected special episode")
	}
	if info.Season != 0 || info.Episode != 2 {
		t.Fatalf("unexpected special mapping: %+v", info)
	}
}

func TestParseFilename_MultiEpisodeRange(t *testing.T) {
	info := ParseFilename("Show.S01E01-E02.mkv")
	if info.Season != 1 || info.Episode != 1 || info.EpisodeEnd != 2 {
		t.Fatalf("unexpected multi-episode parse: %+v", info)
	}
}

func TestParseMovie_CollectionDiscLayout(t *testing.T) {
	info := ParseMovie("Collection/Movie (2010)/Disc 1/movie.mkv", "movie.mkv")
	if info.Title != "Movie" {
		t.Fatalf("title = %q", info.Title)
	}
	if info.Year != 2010 {
		t.Fatalf("year = %d", info.Year)
	}
	if len(info.Collection) != 1 || info.Collection[0] != "Collection" {
		t.Fatalf("collection = %#v", info.Collection)
	}
}

func TestParseMovie_NoisyReleaseFilenameUsesFolderTitle(t *testing.T) {
	info := ParseMovie("Die My Love (2025)/Die My Love 2025 BluRay 1080p DD 5 1 x264-BHDStudio.mp4", "Die My Love 2025 BluRay 1080p DD 5 1 x264-BHDStudio.mp4")
	if info.Title != "Die My Love" {
		t.Fatalf("title = %q", info.Title)
	}
	if info.Year != 2025 {
		t.Fatalf("year = %d", info.Year)
	}
}

func TestParseMovie_RemovesReleasePrefixNoise(t *testing.T) {
	info := ParseMovie("[MrManager] Riding Bean (1989) BDRemux (Dual Audio, Special Features).mkv", "[MrManager] Riding Bean (1989) BDRemux (Dual Audio, Special Features).mkv")
	if info.Title != "Riding Bean" {
		t.Fatalf("title = %q", info.Title)
	}
	if info.Year != 1989 {
		t.Fatalf("year = %d", info.Year)
	}
}

func TestParseMovie_NoisyFolderNameUsesCleanTitleAndYear(t *testing.T) {
	info := ParseMovie(
		"I.Heart.Huckabees.2004.1080p.AMZN.WEBRip.DDP5.1.x264-monkee/I.Heart.Huckabees.2004.1080p.AMZN.WEBRip.DDP5.1.x264-monkee.mkv",
		"I.Heart.Huckabees.2004.1080p.AMZN.WEBRip.DDP5.1.x264-monkee.mkv",
	)
	if info.Title != "I Heart Huckabees" {
		t.Fatalf("title = %q", info.Title)
	}
	if info.Year != 2004 {
		t.Fatalf("year = %d", info.Year)
	}
}

func TestParseMovie_FolderTitleKeepsInitialisms(t *testing.T) {
	info := ParseMovie(
		"L.A. Confidential (1997)/L A Confidential 1997 1080p BluRay DDP5 1 x265 10bit GalaxyRG265.mkv",
		"L A Confidential 1997 1080p BluRay DDP5 1 x265 10bit GalaxyRG265.mkv",
	)
	if info.Title != "L.A. Confidential" {
		t.Fatalf("title = %q", info.Title)
	}
	if info.Year != 1997 {
		t.Fatalf("year = %d", info.Year)
	}
}

func TestParseMovie_FolderTitleKeepsHonorificWord(t *testing.T) {
	info := ParseMovie(
		"Mr. Brooks (2007)/Mr. Brooks (2007) (1080p BluRay x265 HEVC 10bit AAC 5.1 afm72).mkv",
		"Mr. Brooks (2007) (1080p BluRay x265 HEVC 10bit AAC 5.1 afm72).mkv",
	)
	if info.Title != "Mr. Brooks" {
		t.Fatalf("title = %q", info.Title)
	}
	if info.Year != 2007 {
		t.Fatalf("year = %d", info.Year)
	}
}

func TestParseMovie_FolderTitleStripsReleasePrefixBeforeYear(t *testing.T) {
	info := ParseMovie(
		"[YTS] Mr. Brooks (2007)/Mr. Brooks (2007) (1080p BluRay x265 HEVC 10bit AAC 5.1 afm72).mkv",
		"Mr. Brooks (2007) (1080p BluRay x265 HEVC 10bit AAC 5.1 afm72).mkv",
	)
	if info.Title != "Mr. Brooks" {
		t.Fatalf("title = %q", info.Title)
	}
	if info.Year != 2007 {
		t.Fatalf("year = %d", info.Year)
	}
}

func TestParseFilename_StructuredTVNormalizesSeriesName(t *testing.T) {
	info := ParseFilename("Dragon Ball (1986) - S01E01 - Secret of the Dragon Balls [SDTV][AAC 2.0][x265].mkv")
	if info.Title != "dragon ball" {
		t.Fatalf("title = %q", info.Title)
	}
	if info.Season != 1 || info.Episode != 1 {
		t.Fatalf("unexpected episode info: %+v", info)
	}
}

func TestParsePathForMusic_DiscLayout(t *testing.T) {
	info := ParsePathForMusic("Artist/Album/Disc 2/01 - Track.flac", "01 - Track.flac")
	if info.Artist != "Artist" || info.Album != "Album" || info.DiscNumber != 2 {
		t.Fatalf("unexpected music path info: %+v", info)
	}
}
