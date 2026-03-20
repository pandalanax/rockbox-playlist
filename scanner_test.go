package main

import (
	"testing"
)

func TestParseMetadataFromPath(t *testing.T) {
	tests := []struct {
		name                       string
		path                       string
		wantArtist, wantAlbum, wantTitle string
	}{
		{
			name:       "full format with artist in filename",
			path:       "Music/Enter Shikari/Take to the Skies/0016 - Enter Shikari - Sorry You're Not a Winner.flac",
			wantArtist: "Enter Shikari",
			wantAlbum:  "Take to the Skies",
			wantTitle:  "Sorry You're Not a Winner",
		},
		{
			name:       "short format without artist in filename",
			path:       "Music/Gorillaz/Demon Days/01 - Feel Good Inc.mp3",
			wantArtist: "Gorillaz",
			wantAlbum:  "Demon Days",
			wantTitle:  "Feel Good Inc",
		},
		{
			name:       "no match falls back to filename as title",
			path:       "Music/Artist/Album/some track.flac",
			wantArtist: "Artist",
			wantAlbum:  "Album",
			wantTitle:  "some track",
		},
		{
			name:       "CD1 folder uses grandparent as album",
			path:       "Music/Artist/CD1/01 - Title.flac",
			wantArtist: "Artist",
			wantAlbum:  "Artist",
			wantTitle:  "Title",
		},
		{
			name:       "apostrophe in artist and title",
			path:       "Music/Guns N' Roses/Appetite for Destruction/01 - Guns N' Roses - Welcome to the Jungle.flac",
			wantArtist: "Guns N' Roses",
			wantAlbum:  "Appetite for Destruction",
			wantTitle:  "Welcome to the Jungle",
		},
		{
			name:       "accented characters",
			path:       "Music/Björk/Homogenic/03 - Björk - Jóga.flac",
			wantArtist: "Björk",
			wantAlbum:  "Homogenic",
			wantTitle:  "Jóga",
		},
		{
			name:       "parentheses in title",
			path:       "Music/Radiohead/OK Computer/01 - Radiohead - Airbag (Live Version).mp3",
			wantArtist: "Radiohead",
			wantAlbum:  "OK Computer",
			wantTitle:  "Airbag (Live Version)",
		},
		{
			name:       "ampersand in artist",
			path:       "Music/Simon & Garfunkel/Greatest Hits/01 - Simon & Garfunkel - The Sound of Silence.flac",
			wantArtist: "Simon & Garfunkel",
			wantAlbum:  "Greatest Hits",
			wantTitle:  "The Sound of Silence",
		},
		{
			name:       "feat. in title with short format",
			path:       "Music/Drake/Nothing Was the Same/05 - Hold On, We're Going Home (feat. Majid Jordan).m4a",
			wantArtist: "Drake",
			wantAlbum:  "Nothing Was the Same",
			wantTitle:  "Hold On, We're Going Home (feat. Majid Jordan)",
		},
		{
			name:       "japanese characters",
			path:       "Music/坂本龍一/千のナイフ/01 - 坂本龍一 - 千のナイフ.flac",
			wantArtist: "坂本龍一",
			wantAlbum:  "千のナイフ",
			wantTitle:  "千のナイフ",
		},
		{
			name:       "dots in album name",
			path:       "Music/Mr. Oizo/Lambs Anger/01 - Mr. Oizo - Positif.mp3",
			wantArtist: "Mr. Oizo",
			wantAlbum:  "Lambs Anger",
			wantTitle:  "Positif",
		},
		{
			name:       "multiple hyphens in title",
			path:       "Music/OutKast/Stankonia/03 - OutKast - B.O.B. (Bombs Over Baghdad).flac",
			wantArtist: "OutKast",
			wantAlbum:  "Stankonia",
			wantTitle:  "B.O.B. (Bombs Over Baghdad)",
		},
		{
			name:       "exclamation and question marks",
			path:       "Music/Panic! At The Disco/A Fever/01 - Panic! At The Disco - I Write Sins Not Tragedies.mp3",
			wantArtist: "Panic! At The Disco",
			wantAlbum:  "A Fever",
			wantTitle:  "I Write Sins Not Tragedies",
		},
		{
			name:       "CD2 folder",
			path:       "Music/The Beatles/White Album/CD2/01 - Birthday.flac",
			wantArtist: "White Album",
			wantAlbum:  "White Album",
			wantTitle:  "Birthday",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artist, album, title := ParseMetadataFromPath(tt.path)
			if artist != tt.wantArtist {
				t.Errorf("artist = %q, want %q", artist, tt.wantArtist)
			}
			if album != tt.wantAlbum {
				t.Errorf("album = %q, want %q", album, tt.wantAlbum)
			}
			if title != tt.wantTitle {
				t.Errorf("title = %q, want %q", title, tt.wantTitle)
			}
		})
	}
}

func TestSong_ConfirmDisplayName(t *testing.T) {
	tests := []struct {
		name string
		song Song
		want string
	}{
		{
			name: "normal",
			song: Song{Artist: "Gorillaz", Title: "Clint Eastwood"},
			want: "Gorillaz - Clint Eastwood",
		},
		{
			name: "empty artist",
			song: Song{Title: "Unknown Track"},
			want: "Unknown Artist - Unknown Track",
		},
		{
			name: "empty title uses filename",
			song: Song{Artist: "Gorillaz", Path: "/music/track.flac"},
			want: "Gorillaz - track",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.song.ConfirmDisplayName()
			if got != tt.want {
				t.Errorf("ConfirmDisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSong_RelativePath(t *testing.T) {
	s := Song{Path: "/mnt/device/Music/Artist/Album/song.flac"}
	got := s.RelativePath("/mnt/device/Music")
	want := "../Music/Artist/Album/song.flac"
	if got != want {
		t.Errorf("RelativePath() = %q, want %q", got, want)
	}
}

func TestScanMusicDirectory(t *testing.T) {
	dir := createMusicDir(t,
		"Artist1/Album1/01 - Song1.flac",
		"Artist1/Album1/02 - Song2.mp3",
		"Artist2/Album2/01 - Track.ogg",
		"Artist1/Album1/cover.jpg",
		".hidden/secret.flac",
	)

	songs, err := ScanMusicDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) != 3 {
		t.Fatalf("got %d songs, want 3", len(songs))
	}

	// Verify all songs have metadata
	for _, s := range songs {
		if s.Artist == "" {
			t.Errorf("song %q has empty artist", s.Path)
		}
		if s.Title == "" {
			t.Errorf("song %q has empty title", s.Path)
		}
	}
}

func TestScanMusicDirectory_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	songs, err := ScanMusicDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) != 0 {
		t.Fatalf("got %d songs, want 0", len(songs))
	}
}

func TestScanMusicDirectory_SpecialCharacters(t *testing.T) {
	dir := createMusicDir(t,
		"Guns N' Roses/Appetite/01 - Guns N' Roses - Welcome to the Jungle.flac",
		"Björk/Homogenic/03 - Björk - Jóga.flac",
		"坂本龍一/千のナイフ/01 - 坂本龍一 - 千のナイフ.flac",
		"Simon & Garfunkel/Hits/01 - Simon & Garfunkel - The Sound of Silence.mp3",
		"Panic! At The Disco/Fever/01 - I Write Sins.mp3",
	)

	songs, err := ScanMusicDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) != 5 {
		t.Fatalf("got %d songs, want 5", len(songs))
	}

	// Verify special characters are preserved in metadata
	for _, s := range songs {
		if s.Artist == "" || s.Title == "" {
			t.Errorf("song %q: artist=%q title=%q — should not be empty", s.Path, s.Artist, s.Title)
		}
	}
}

func TestScanMusicDirectory_EmojiInNames(t *testing.T) {
	dir := createMusicDir(t,
		"🎵 DJ Mix/💿 Album/01 - 🔥 Drop.mp3",
		"Artist/🌸 Cherry Blossoms/01 - Track.flac",
	)

	songs, err := ScanMusicDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) != 2 {
		t.Fatalf("got %d songs, want 2", len(songs))
	}

	for _, s := range songs {
		if s.Artist == "" || s.Title == "" {
			t.Errorf("song %q: artist=%q title=%q — should not be empty", s.Path, s.Artist, s.Title)
		}
		// Display name should not panic or be empty
		if s.DisplayName() == "" {
			t.Errorf("DisplayName() is empty for %q", s.Path)
		}
	}
}

func TestSong_ConfirmDisplayName_Emoji(t *testing.T) {
	s := Song{Artist: "🎤 Artist", Title: "Song 🎵"}
	got := s.ConfirmDisplayName()
	if got != "🎤 Artist - Song 🎵" {
		t.Errorf("ConfirmDisplayName() = %q, want %q", got, "🎤 Artist - Song 🎵")
	}
}

func TestScanMusicDirectory_SupportedExtensions(t *testing.T) {
	dir := createMusicDir(t,
		"Artist/Album/01 - Track.flac",
		"Artist/Album/02 - Track.mp3",
		"Artist/Album/03 - Track.m4a",
		"Artist/Album/04 - Track.ogg",
		"Artist/Album/05 - Track.wav",
		"Artist/Album/06 - Track.txt",
		"Artist/Album/07 - Track.pdf",
	)

	songs, err := ScanMusicDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) != 5 {
		t.Fatalf("got %d songs, want 5 (only audio files)", len(songs))
	}
}
