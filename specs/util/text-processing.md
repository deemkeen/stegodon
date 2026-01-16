# Text Processing

This document specifies hashtag/mention parsing, URL linkification, and text formatting utilities.

---

## Overview

Stegodon provides text processing for:
- Hashtag detection and highlighting
- Mention parsing (@user@domain)
- Markdown link conversion
- HTML tag stripping
- ANSI terminal formatting
- Character counting

---

## Pre-compiled Regex Patterns

```go
var ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;]*m|\x1b\]8;;[^\x1b]*\x1b\\`)
var hashtagRegex = regexp.MustCompile(`#([a-zA-Z][a-zA-Z0-9_]*)`)
var mentionRegex = regexp.MustCompile(`@([a-zA-Z0-9_]+)@([a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)
var markdownLinkRegex = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)
var urlRegex = regexp.MustCompile(`^https?://[^\s]+$`)
```

---

## Hashtag Processing

### ParseHashtags

Extracts hashtags from text as lowercase, deduplicated strings.

```go
func ParseHashtags(text string) []string {
    matches := hashtagRegex.FindAllStringSubmatch(text, -1)
    seen := make(map[string]bool)
    tags := make([]string, 0)

    for _, match := range matches {
        tag := strings.ToLower(match[1])
        if !seen[tag] {
            seen[tag] = true
            tags = append(tags, tag)
        }
    }
    return tags
}
```

### Hashtag Rules

| Rule | Example Valid | Example Invalid |
|------|---------------|-----------------|
| Must start with letter | `#hello`, `#Go123` | `#123`, `#_tag` |
| Can contain numbers | `#test2024` | - |
| Can contain underscores | `#my_tag` | - |
| Case insensitive | `#Tag` = `#tag` | - |

### HighlightHashtagsTerminal

```go
func HighlightHashtagsTerminal(text string) string {
    return hashtagRegex.ReplaceAllString(text,
        "\033[38;5;75m#$1\033[39m")
}
```

Uses ANSI 75 (`#5fafff`) color.

### HighlightHashtagsHTML

```go
func HighlightHashtagsHTML(text string) string {
    return hashtagRegex.ReplaceAllString(text,
        `<a href="/tags/$1" class="hashtag">#$1</a>`)
}
```

### HashtagsToActivityPubHTML

```go
func HashtagsToActivityPubHTML(text string, baseURL string) string {
    // Returns: <a href="baseURL/tags/tag" class="hashtag" rel="tag">#<span>tag</span></a>
}
```

ActivityPub-compliant format with `rel="tag"` attribute.

---

## Mention Processing

### Mention Structure

```go
type Mention struct {
    Username string
    Domain   string
}
```

### ParseMentions

Extracts `@username@domain` mentions, deduplicated.

```go
func ParseMentions(text string) []Mention {
    matches := mentionRegex.FindAllStringSubmatch(text, -1)
    seen := make(map[string]bool)
    mentions := make([]Mention, 0)

    for _, match := range matches {
        username := strings.ToLower(match[1])
        domain := strings.ToLower(match[2])
        key := username + "@" + domain
        if !seen[key] {
            seen[key] = true
            mentions = append(mentions, Mention{username, domain})
        }
    }
    return mentions
}
```

### Mention Rules

| Component | Pattern |
|-----------|---------|
| Username | `[a-zA-Z0-9_]+` |
| Domain | `[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}` |
| Format | `@username@domain.tld` |

### HighlightMentionsTerminal

```go
func HighlightMentionsTerminal(text string, localDomain string) string {
    // Local users: @username (links to /u/username)
    // Remote users: @username@domain (links to their profile)
}
```

Uses ANSI 48 (`#00ff87`) color with OSC 8 hyperlinks.

### HighlightMentionsHTML

```go
func HighlightMentionsHTML(text string, localDomain string) string {
    // Local: <a href="/u/username" class="mention">@username</a>
    // Remote: <a href="https://domain/@username" ...>@username@domain</a>
}
```

### MentionsToActivityPubHTML

```go
func MentionsToActivityPubHTML(text string, mentionURIs map[string]string) string {
    // Returns: <span class="h-card"><a href="actorURI" class="u-url mention">@<span>username</span></a></span>
}
```

Takes pre-resolved actor URIs for proper ActivityPub formatting.

---

## Markdown Link Processing

### MarkdownLinksToHTML

Converts `[text](url)` to HTML anchor tags.

```go
func MarkdownLinksToHTML(text string) string {
    return markdownLinkRegex.ReplaceAllStringFunc(text, func(match string) string {
        matches := markdownLinkRegex.FindStringSubmatch(match)
        linkText := html.EscapeString(matches[1])
        linkURL := html.EscapeString(matches[2])
        return fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`,
            linkURL, linkText)
    })
}
```

### MarkdownLinksToTerminal

Converts to OSC 8 clickable hyperlinks for terminal display.

```go
func MarkdownLinksToTerminal(text string) string {
    // Returns: COLOR + OSC8_START + URL + OSC8_END + TEXT + OSC8_CLOSE + RESET
}
```

### ExtractMarkdownLinks

Returns list of URLs from markdown links.

```go
func ExtractMarkdownLinks(text string) []string
```

### GetMarkdownLinkCount

Returns count of markdown links in text.

```go
func GetMarkdownLinkCount(text string) int
```

---

## URL Processing

### IsURL

Checks if string is valid HTTP/HTTPS URL.

```go
func IsURL(text string) bool {
    text = strings.TrimSpace(text)
    return urlRegex.MatchString(text)  // ^https?://[^\s]+$
}
```

### FormatClickableURL

Creates OSC 8 hyperlink for terminal display.

```go
func FormatClickableURL(url string, maxWidth int, prefix string) string {
    linkText := prefix + url
    truncatedLinkText := TruncateVisibleLength(linkText, maxWidth)
    return fmt.Sprintf("\033[38;2;0;255;135;4m\033]8;;%s\033\\%s\033]8;;\033\\\033[39;24m",
        url, truncatedLinkText)
}
```

---

## Character Counting

### CountVisibleChars

Counts visible characters, ignoring:
- ANSI escape sequences
- OSC 8 hyperlinks
- Markdown link syntax (URL portion)

```go
func CountVisibleChars(text string) int {
    // Strip ANSI escapes
    stripped := ansiEscapeRegex.ReplaceAllString(text, "")
    // Replace markdown links with just text
    result := markdownLinkRegex.ReplaceAllString(stripped, "$1")
    // Count Unicode runes
    return utf8.RuneCountInString(result)
}
```

Counts Unicode runes, not bytes:
- `"hello"` → 5
- `"café"` → 4
- `"[link](http://...)` → 4 (only "link" counted)

### ValidateNoteLength

```go
func ValidateNoteLength(text string) error {
    const maxDBLength = 1000
    if len(text) > maxDBLength {
        return fmt.Errorf("Note too long (max %d characters including links)", maxDBLength)
    }
    return nil
}
```

Validates full text including markdown syntax.

---

## Text Truncation

### TruncateVisibleLength

Truncates based on visible characters, preserving ANSI formatting.

```go
func TruncateVisibleLength(s string, maxLen int) string {
    // Strip ANSI to count visible chars
    visible := ansiEscapeRegex.ReplaceAllString(s, "")

    if len(visible) <= maxLen {
        return s
    }

    // Walk string, count visible chars, truncate at limit
    // Add "..." and reset sequence
    return s[:truncateAt] + "..." + "\x1b[0m"
}
```

---

## HTML Processing

### StripHTMLTags

Removes HTML tags and converts common entities.

```go
func StripHTMLTags(html string) string {
    text := htmlTagRegex.ReplaceAllString(html, "")

    // Convert entities
    text = strings.ReplaceAll(text, "&lt;", "<")
    text = strings.ReplaceAll(text, "&gt;", ">")
    text = strings.ReplaceAll(text, "&amp;", "&")
    text = strings.ReplaceAll(text, "&quot;", "\"")
    text = strings.ReplaceAll(text, "&#39;", "'")
    text = strings.ReplaceAll(text, "&nbsp;", " ")

    return strings.TrimSpace(text)
}
```

### NormalizeInput

Escapes HTML and normalizes newlines.

```go
func NormalizeInput(text string) string {
    normalized := strings.ReplaceAll(text, "\n", " ")
    normalized = html.EscapeString(normalized)
    return normalized
}
```

---

## ANSI Color Constants

```go
const (
    ansiHashtagColor = "75"        // #5fafff - blue
    ansiMentionColor = "48"        // #00ff87 - green
    ansiLinkRGB      = "0;255;135" // RGB for links
)
```

---

## Terminal Formatting

### OSC 8 Hyperlink Format

```
\033]8;;URL\033\\TEXT\033]8;;\033\\
```

| Component | Description |
|-----------|-------------|
| `\033]8;;URL\033\\` | Start hyperlink with URL |
| `TEXT` | Visible link text |
| `\033]8;;\033\\` | End hyperlink |

### Color + Hyperlink

```go
// COLOR_START + OSC8_START + TEXT + OSC8_END + COLOR_RESET
fmt.Sprintf("\033[38;5;48;4m\033]8;;%s\033\\%s\033]8;;\033\\\033[39;24m",
    profileURL, displayMention)
```

---

## Template Placeholder Processing

### ReplacePlaceholders

Replaces template placeholders in text with actual configuration values.

```go
func ReplacePlaceholders(text string, sshPort int) string {
    text = strings.ReplaceAll(text, "{{SSH_PORT}}", fmt.Sprintf("%d", sshPort))
    return text
}
```

### Supported Placeholders

| Placeholder | Replaced With | Example |
|-------------|---------------|---------|
| `{{SSH_PORT}}` | SSH port number | `23232` |

### Use Cases

Used primarily in info box content rendering:

```go
// Before rendering info box content
content := util.ReplacePlaceholders(box.Content, conf.Conf.SshPort)
htmlContent := convertMarkdownToHTML(content)
```

Example transformation:
```
Input:  "ssh -p {{SSH_PORT}} example.com"
Output: "ssh -p 23232 example.com"
```

---

## Source Files

- `util/util.go` - All text processing functions
- `ui/common/styles.go` - Color constant definitions
