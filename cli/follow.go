package cli

import (
	"fmt"
	"log"
	"strings"

	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/util"
	"github.com/deemkeen/stegodon/web"
)

// handleFollow toggles a follow on a user
func (h *Handler) handleFollow(args []string) error {
	if len(args) == 0 {
		err := fmt.Errorf("usage: follow <user@domain> or follow <profile-url>")
		h.output.Error(err)
		return err
	}

	input := strings.TrimSpace(args[0])
	if input == "" {
		err := fmt.Errorf("user cannot be empty")
		h.output.Error(err)
		return err
	}

	var username, domain string

	// Try parsing as URL first
	if parsedUsername, parsedDomain, ok := util.ParseActivityPubURL(input); ok {
		username = parsedUsername
		domain = parsedDomain
	} else {
		// Not a URL, try parsing as user@domain format
		// Remove leading @ if present
		input = strings.TrimPrefix(input, "@")

		parts := strings.Split(input, "@")
		if len(parts) == 1 {
			// Local user (just username, no domain)
			username = parts[0]
			domain = "" // Local
		} else if len(parts) == 2 {
			username = parts[0]
			domain = parts[1]
		} else {
			err := fmt.Errorf("invalid format. Use: user@domain, @user, or profile URL")
			h.output.Error(err)
			return err
		}

		if username == "" {
			err := fmt.Errorf("invalid format. Use: user@domain, @user, or profile URL")
			h.output.Error(err)
			return err
		}
	}

	// Determine if this is a local or remote user
	isLocal := domain == "" || strings.EqualFold(domain, h.conf.Conf.SslDomain)

	if isLocal {
		return h.handleLocalFollow(username)
	}
	return h.handleRemoteFollow(username, domain)
}

// handleLocalFollow toggles follow for a local user
func (h *Handler) handleLocalFollow(username string) error {
	// Look up local user
	dbErr, targetAccount := h.db.ReadAccByUsername(username)
	if dbErr != nil || targetAccount == nil {
		err := fmt.Errorf("local user not found: @%s", username)
		h.output.Error(err)
		return err
	}

	// Check if trying to follow yourself
	if targetAccount.Id == h.account.Id {
		err := fmt.Errorf("cannot follow yourself")
		h.output.Error(err)
		return err
	}

	// Check if already following
	isFollowing, err := h.db.IsFollowingLocal(h.account.Id, targetAccount.Id)
	if err != nil {
		err = fmt.Errorf("failed to check follow status: %v", err)
		h.output.Error(err)
		return err
	}

	if isFollowing {
		// Unfollow
		if err := h.db.DeleteLocalFollow(h.account.Id, targetAccount.Id); err != nil {
			err = fmt.Errorf("failed to unfollow: %v", err)
			h.output.Error(err)
			return err
		}

		// Output response
		if h.output.IsJSON() {
			h.output.JSON(FollowResponse{
				Username:  username,
				Following: false,
				Message:   fmt.Sprintf("Unfollowed @%s", username),
			})
		} else {
			h.output.Print("Unfollowed @%s\n", username)
		}
	} else {
		// Follow
		if err := h.db.CreateLocalFollow(h.account.Id, targetAccount.Id); err != nil {
			err = fmt.Errorf("failed to follow: %v", err)
			h.output.Error(err)
			return err
		}

		// Output response
		if h.output.IsJSON() {
			h.output.JSON(FollowResponse{
				Username:  username,
				Following: true,
				Message:   fmt.Sprintf("Now following @%s", username),
			})
		} else {
			h.output.Print("Now following @%s\n", username)
		}
	}

	return nil
}

// handleRemoteFollow toggles follow for a remote user
func (h *Handler) handleRemoteFollow(username, domain string) error {
	fullUsername := fmt.Sprintf("%s@%s", username, domain)

	// Check if ActivityPub is enabled
	if !h.conf.Conf.WithAp {
		err := fmt.Errorf("ActivityPub is not enabled on this server")
		h.output.Error(err)
		return err
	}

	// Resolve WebFinger to get actor URI
	actorURI, err := web.ResolveWebFinger(username, domain)
	if err != nil {
		err = fmt.Errorf("failed to resolve user %s: %v", fullUsername, err)
		h.output.Error(err)
		return err
	}

	// Check if we have this remote account cached
	dbErr, remoteAccount := h.db.ReadRemoteAccountByActorURI(actorURI)
	if dbErr != nil {
		log.Printf("CLI: Error looking up remote account: %v", dbErr)
	}

	// Check if already following
	if remoteAccount != nil {
		dbErr, existingFollow := h.db.ReadFollowByAccountIds(h.account.Id, remoteAccount.Id)
		if dbErr == nil && existingFollow != nil {
			// Already have a follow relationship - unfollow
			if existingFollow.Accepted {
				// Send Undo activity
				go func() {
					if err := activitypub.SendUndo(h.account, existingFollow, remoteAccount, h.conf); err != nil {
						log.Printf("CLI: Failed to send Undo (unfollow): %v", err)
					}
				}()

				// Delete the follow record
				if err := h.db.DeleteFollowByURI(existingFollow.URI); err != nil {
					log.Printf("CLI: Failed to delete follow record: %v", err)
				}

				// Output response
				if h.output.IsJSON() {
					h.output.JSON(FollowResponse{
						Username:  username,
						Domain:    domain,
						Following: false,
						Message:   fmt.Sprintf("Unfollowed @%s", fullUsername),
					})
				} else {
					h.output.Print("Unfollowed @%s\n", fullUsername)
				}
				return nil
			} else {
				// Follow is pending - cancel it
				go func() {
					if err := activitypub.SendUndo(h.account, existingFollow, remoteAccount, h.conf); err != nil {
						log.Printf("CLI: Failed to send Undo (cancel pending follow): %v", err)
					}
				}()

				// Delete the follow record
				if err := h.db.DeleteFollowByURI(existingFollow.URI); err != nil {
					log.Printf("CLI: Failed to delete follow record: %v", err)
				}

				// Output response
				if h.output.IsJSON() {
					h.output.JSON(FollowResponse{
						Username:  username,
						Domain:    domain,
						Following: false,
						Message:   fmt.Sprintf("Cancelled follow request to @%s", fullUsername),
					})
				} else {
					h.output.Print("Cancelled follow request to @%s\n", fullUsername)
				}
				return nil
			}
		}
	}

	// Not following yet - send follow request
	if err := activitypub.SendFollow(h.account, actorURI, h.conf); err != nil {
		// Check for specific error conditions
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "already following") {
			if h.output.IsJSON() {
				h.output.JSON(FollowResponse{
					Username:  username,
					Domain:    domain,
					Following: true,
					Message:   fmt.Sprintf("Already following @%s", fullUsername),
				})
			} else {
				h.output.Print("Already following @%s\n", fullUsername)
			}
			return nil
		}
		if strings.Contains(errMsg, "follow pending") {
			if h.output.IsJSON() {
				h.output.JSON(FollowResponse{
					Username:  username,
					Domain:    domain,
					Following: false,
					Pending:   true,
					Message:   fmt.Sprintf("Follow request pending for @%s", fullUsername),
				})
			} else {
				h.output.Print("Follow request pending for @%s\n", fullUsername)
			}
			return nil
		}
		if strings.Contains(errMsg, "self-follow") {
			err = fmt.Errorf("cannot follow yourself")
			h.output.Error(err)
			return err
		}
		err = fmt.Errorf("failed to follow: %v", err)
		h.output.Error(err)
		return err
	}

	// Output response - follow request sent (pending)
	if h.output.IsJSON() {
		h.output.JSON(FollowResponse{
			Username:  username,
			Domain:    domain,
			Following: false,
			Pending:   true,
			Message:   fmt.Sprintf("Sent follow request to @%s", fullUsername),
		})
	} else {
		h.output.Print("Sent follow request to @%s\n", fullUsername)
	}

	return nil
}
