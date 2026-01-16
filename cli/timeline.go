package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/deemkeen/stegodon/util"
)

const defaultTimelineLimit = 20

// handleTimeline shows the home timeline
func (h *Handler) handleTimeline(args []string) error {
	limit := defaultTimelineLimit

	// Parse -n flag
	for i := 0; i < len(args); i++ {
		if args[i] == "-n" && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				err = fmt.Errorf("invalid value for -n: %s", args[i+1])
				h.output.Error(err)
				return err
			}
			if n < 1 {
				err = fmt.Errorf("-n must be at least 1")
				h.output.Error(err)
				return err
			}
			limit = n
			i++ // Skip the next argument (the number)
		}
	}

	// Read timeline posts
	err, posts := h.db.ReadHomeTimelinePosts(h.account.Id, limit)
	if err != nil {
		h.output.Error(err)
		return err
	}

	if posts == nil || len(*posts) == 0 {
		if h.output.IsJSON() {
			h.output.JSON(TimelineResponse{
				Posts: []TimelinePost{},
				Count: 0,
			})
		} else {
			h.output.Println("No posts in timeline.")
		}
		return nil
	}

	// Output response
	if h.output.IsJSON() {
		timelinePosts := make([]TimelinePost, 0, len(*posts))
		for _, post := range *posts {
			// Parse author and domain
			author := post.Author
			domain := ""
			if strings.Contains(author, "@") && strings.Count(author, "@") >= 2 {
				// Remote user: @user@domain -> user, domain
				parts := strings.SplitN(strings.TrimPrefix(author, "@"), "@", 2)
				if len(parts) == 2 {
					author = parts[0]
					domain = parts[1]
				}
			} else {
				// Local user: @user -> user
				author = strings.TrimPrefix(author, "@")
			}

			// Strip HTML tags from content for CLI output
			content := util.StripHTMLTags(post.Content)

			timelinePosts = append(timelinePosts, TimelinePost{
				ID:         post.ID.String(),
				Author:     author,
				Domain:     domain,
				Message:    content,
				CreatedAt:  post.Time,
				ReplyCount: post.ReplyCount,
				LikeCount:  post.LikeCount,
				BoostCount: post.BoostCount,
			})
		}

		h.output.JSON(TimelineResponse{
			Posts: timelinePosts,
			Count: len(timelinePosts),
		})
	} else {
		// Text output
		for _, post := range *posts {
			// Strip HTML tags from content for CLI output
			content := util.StripHTMLTags(post.Content)

			h.output.Print("%s (%s)\n", post.Author, FormatTimeAgo(post.Time))
			h.output.Print("%s\n\n", content)
		}
	}

	return nil
}
