# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-03-02

### Added
- Podcast management feature for playlists named "podcasts"
  - Update all subscribed podcasts (fetch new episodes, keep latest 3, delete old)
  - Search and add new podcasts via iTunes API
  - Download episodes automatically
  - Automatic playlist regeneration sorted by date
  - Detailed progress output during updates and additions
  - Browse & add songs option for manual audio file additions

## [0.1.0] - 2026-03-02

### Added
- Initial release
- TUI playlist picker with filtering
- Song browser with type-to-search filtering
- Multi-select songs with tab key
- Selected songs panel showing picks in selection order
- Confirmation screen before adding songs
- User-friendly error messages with styled display
- Support for custom playlist and music directory paths via CLI arguments

### Fixed
- Space key now inputs into search instead of toggling selection
