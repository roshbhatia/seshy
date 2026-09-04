Require sy

Set Shell "zsh"
Set Theme "Catppuccin Macchiato"
Set FontSize 16
Set Width 1200
Set Height 720
Set Padding 24
Set Framerate 20
Set TypingSpeed 35ms

Sleep 1s
Type "sy list"
Enter
Sleep 2s
Type "sy status auth-hardening"
Enter
Sleep 3s
Type "sy archive auth-hardening"
Enter
Sleep 2s
Type "sy list --archived"
Enter
Sleep 3s
Type "sy unarchive auth-hardening"
Enter
Sleep 2s
Type "sy current --quiet || echo 'session metadata stays directory-bound'"
Enter
Sleep 3s
