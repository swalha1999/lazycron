# lazycron demo — records demo.gif for the README
# Run with: vhs demo.tape

Output demo.gif
Set Width 1200
Set Height 600
Set FontSize 16
Set Padding 10
Set TypingSpeed 60ms
Set Theme "Catppuccin Mocha"

# Launch
Type "./lazycron"
Enter
Sleep 2s

# 1. Show the 4-panel layout — start on jobs
Type "2"
Sleep 1.5s

# 2. Create a new blank job to show the form + human-readable schedules
Type "n"
Sleep 1s
Type "b"
Sleep 1s

# Fill in the form
Type "daily backup"
Tab
Sleep 500ms
Type "pg_dump mydb > /tmp/backup.sql"
Tab
Sleep 500ms

# Clear the default cron expression before typing human-readable schedule
Backspace 11
Type "every day at 3am"
Tab
Sleep 500ms

# Let the user see the filled form
Sleep 1.5s
Escape
Sleep 1s

# 3. Show template picker
Type "n"
Sleep 800ms
Type "t"
Sleep 1.5s

# Browse template categories
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Up
Sleep 400ms

# Exit templates
Escape
Sleep 500ms
Escape
Sleep 1s

# 4. Browse existing jobs
Down
Sleep 500ms
Down
Sleep 500ms
Up
Sleep 1s

# 5. Check history panel
Type "3"
Sleep 1.5s
Down
Sleep 400ms
Down
Sleep 400ms

# 6. Show help
Type "?"
Sleep 2.5s
Escape
Sleep 1s

# Quit
Type "q"
Sleep 500ms
