# Basic playlist management demo
Output assets/demo.gif

Set Shell zsh
Set FontSize 16
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"
Set Padding 20
Set Framerate 30
Set TypingSpeed 50ms

# Start the app
Type "./rockbox-playlist /tmp/rockbox-demo/Playlists /tmp/rockbox-demo/Music"
Enter
Sleep 2s

# Navigate to rock playlist
Type "rock"
Sleep 500ms
Enter
Sleep 1s

# Type to search for songs
Type "stair"
Sleep 800ms

# Select a song with tab
Tab
Sleep 500ms

# Clear search and browse more
Backspace 5
Sleep 300ms
Type "karma"
Sleep 500ms
Tab
Sleep 500ms

# Clear and find another
Backspace 5
Type "wish"
Sleep 500ms
Tab
Sleep 1s

# Confirm selection
Enter
Sleep 1s

# Accept with 'y'
Type "y"
Sleep 2s
