package common

import "github.com/deemkeen/stegodon/util"

// ProcessPostContent applies the standard text processing pipeline for terminal display.
// isLocal determines whether Markdown link conversion is applied (local posts use Markdown, remote don't).
// linkifyURLs determines whether raw URLs are converted to clickable terminal links.
func ProcessPostContent(content string, isLocal bool, localDomain string, linkifyURLs bool) string {
	content = util.SanitizeRemoteContent(content)
	content = util.TruncateContent(content, MaxDisplayContentLength)
	content = util.UnescapeHTML(content)
	if isLocal {
		content = util.MarkdownLinksToTerminal(content)
	}
	if linkifyURLs {
		content = util.LinkifyRawURLsTerminal(content)
	}
	content = util.HighlightHashtagsTerminal(content)
	content = util.HighlightMentionsTerminal(content, localDomain)
	return content
}
