# VHS tape file for stegodon demo
# This creates a demo GIF showing registration and posting with Markdown links

Output demo.gif

Set Shell zsh

Set FontSize 14
Set Width 1200
Set Height 800

# Note: Stegodon must be running with fresh database
# Start with: rm database.db && ./stegodon

# Wait a moment
Sleep 1s

# Connect via SSH
Type "ssh localhost -p 23232"
Enter
Sleep 2s

# First-time registration - enter username
Type "stegodon"
Enter
Sleep 1s

# Enter display name
Type "Stegodon"
Enter
Sleep 1s

# Enter bio
Type "Testing stegodon's Markdown links feature"
Enter
Sleep 2s

# We're now in the TUI - should be at "New Note" view by default
# Type a note with a Markdown link
Type "Welcome to [Stegodon](https://github.com/deemkeen/stegodon)!"
Sleep 500ms
Enter
Enter
Type "A federated microblog accessible via SSH."
Sleep 500ms
Enter
Enter

# Save the note with Ctrl+S
Ctrl+S
Sleep 2s

# Navigate to Notes List with Tab
Tab

# Show the posted note with clickable link (in green)
Sleep 2s

# Navigate back to New Note with Shift+Tab
Shift+Tab
Sleep 1s

# Create another note
Type "Markdown links work everywhere:"
Sleep 500ms
Enter
Type "- TUI (green, clickable)"
Sleep 500ms
Enter
Type "- Web UI (blue links)"
Sleep 500ms
Enter
Type "- Federation (HTML)"
Sleep 2s

# Save second note
Ctrl+S
Sleep 2s

# Navigate to Notes List again
Tab
Sleep 2s

# Quit
Type "q"
Sleep 1s
