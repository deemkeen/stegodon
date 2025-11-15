# Testing ActivityPub Federation Locally

This guide explains how to test stegodon's ActivityPub federation features with real Fediverse instances (Mastodon, Pleroma, etc.) using ngrok to expose your local server.

## Quick Start (15 minutes)

### Prerequisites

- stegodon binary (download or build with `go build`)
- ngrok installed (or use alternative tunneling service)
- A Mastodon or Pleroma account for testing

### Step 1: Start ngrok Tunnel

Open a terminal and start an ngrok tunnel to your HTTP port (9999):

```bash
ngrok http 9999
```

You'll see output like:

```
Forwarding   https://abc123.ngrok-free.app -> http://localhost:9999
```

**Important:** Copy the `https://` URL (e.g., `abc123.ngrok-free.app` without the protocol). This is your temporary domain.

**Note:** The free ngrok domain changes every time you restart ngrok. For persistent domains, consider ngrok's paid plan or alternatives like Cloudflare Tunnel.

### Step 2: Start stegodon with ActivityPub Enabled

In a **new terminal**, start stegodon with your ngrok domain:

```bash
STEGODON_WITH_AP=true STEGODON_SSLDOMAIN=abc123.ngrok-free.app ./stegodon
```

Replace `abc123.ngrok-free.app` with your actual ngrok domain from Step 1.

You should see:

```
Configuration:
...
Created default config file at ~/.config/stegodon/config.yaml
Using database at: ~/.config/stegodon/database.db
Using SSH host key at: ~/.config/stegodon/.ssh/stegodonhostkey
Starting SSH server on 127.0.0.1:23232
```

**Note:** On first run, stegodon creates `~/.config/stegodon/` with default config, database, and SSH keys.

### Step 3: Connect via SSH and Create an Account

In a **third terminal**, connect to your local stegodon instance:

```bash
ssh localhost -p 23232
```

On first connection:
1. You'll be prompted to choose a username (e.g., `alice`)
2. Navigate the TUI with **Tab** (forward) or **Shift+Tab** (backward):
   - "Write Note"
   - "List Notes"
   - "Follow User"
   - "Followers"
   - "Federated Timeline"

### Step 4: Create a Test Post

1. Press **Tab** to cycle to "Write Note" view
2. Type a test message: "Hello from stegodon!"
3. Press **Ctrl+S** to save

Your note is now posted and will federate to any followers.

### Step 5: Test Federation from Mastodon

Open your Mastodon account and search for:

```
@alice@abc123.ngrok-free.app
```

Replace `alice` with your chosen username and use your ngrok domain.

**Expected behavior:**
- Mastodon should find your account via WebFinger
- You should see your account profile (may be empty/basic)
- Click "Follow" to follow your stegodon account
- Your stegodon instance will automatically accept the follow

### Step 6: Verify Federation is Working

#### Check Logs

In your stegodon terminal, you should see:

```
Inbox: Received Follow from https://mastodon.social/users/yourname
Sent Accept activity to ...
```

#### Check Followers List

In your SSH session:
1. Press **Tab** to cycle to "Followers" view
2. You should see your Mastodon account listed

#### Test Incoming Posts

From your Mastodon account, follow yourself from stegodon:
1. In stegodon SSH session, press **Tab** to cycle to "Follow User" view
2. Enter: `yourname@mastodon.social`
3. Press Enter

Then post from Mastodon and check:
1. Press **Tab** to cycle to "Federated Timeline" view in stegodon
2. Your Mastodon post should appear

### Step 7: Test Outgoing Posts

1. Create another note in stegodon (cycle to "Write Note" view, write, **Ctrl+S**)
2. Check your Mastodon home timeline
3. Your stegodon post should appear in your feed

## Troubleshooting

### "User not found" when searching from Mastodon

**Possible causes:**
- Ngrok domain not set correctly (check `STEGODON_SSLDOMAIN` matches ngrok URL exactly)
- ActivityPub not enabled (`STEGODON_WITH_AP=true` required)
- Ngrok tunnel not running
- Ports mismatched

**Debug:**
```bash
# Test WebFinger endpoint directly
curl https://abc123.ngrok-free.app/.well-known/webfinger?resource=acct:alice@abc123.ngrok-free.app
```

Expected response:
```json
{
  "subject": "acct:alice@abc123.ngrok-free.app",
  "links": [
    {
      "rel": "self",
      "type": "application/activity+json",
      "href": "https://abc123.ngrok-free.app/users/alice"
    }
  ]
}
```

### Posts not federating

**Check delivery queue:**
```bash
# Database is in ~/.config/stegodon/ by default
sqlite3 ~/.config/stegodon/database.db "SELECT * FROM delivery_queue ORDER BY created_at DESC LIMIT 10;"
```

If queue is stuck:
- Check `next_retry_at` timestamp
- Check `attempts` count (maxes at 10)
- Delivery worker runs every 10 seconds

**Check logs for errors:**
Look for lines containing:
- "Delivery failed"
- "HTTP signature"
- "Sending to inbox"

### "SSL certificate problem" errors

This shouldn't happen with ngrok (they provide valid certs), but if you see cert errors:
- Verify you're using `https://` URLs everywhere
- Check ngrok is running and forwarding correctly

### Ngrok "Visit Site" button required

Some ngrok plans show an interstitial page. If you see this:
- Upgrade ngrok plan, OR
- Use an alternative like localtunnel: `lt --port 9999`

### Database locked errors

If you see `database is locked`:
- Only run one stegodon instance per database
- Check WAL mode is enabled: `sqlite3 ~/.config/stegodon/database.db "PRAGMA journal_mode;"`
- Should return `wal2` or `wal`

## Advanced Testing

### Testing Multiple Local Instances

To run multiple stegodon instances simultaneously, each needs its own data directory:

1. **Instance 1 (default location):**
   ```bash
   # Terminal 1: ngrok for instance 1
   ngrok http 9999

   # Terminal 2: Run instance 1
   STEGODON_WITH_AP=true STEGODON_SSLDOMAIN=abc123.ngrok-free.app ./stegodon
   ```

2. **Instance 2 (custom directory):**
   ```bash
   # Terminal 3: Create data directory for instance 2
   mkdir -p ~/stegodon-instance2/.config/stegodon

   # Terminal 4: ngrok for instance 2 (different port)
   ngrok http 9998

   # Terminal 5: Run instance 2 with custom config location
   # Use HOME to point to custom data directory
   HOME=~/stegodon-instance2 \
   STEGODON_WITH_AP=true \
   STEGODON_SSLDOMAIN=xyz789.ngrok-free.app \
   STEGODON_SSHPORT=23233 \
   STEGODON_HTTPPORT=9998 \
   ./stegodon
   ```

**Alternative:** Use local config files (backwards compatibility):
   ```bash
   # Create separate directories
   mkdir instance2 && cd instance2
   cp ../stegodon .

   # Create local config.yaml to override defaults
   cat > config.yaml <<EOF
   conf:
     host: 127.0.0.1
     sshPort: 23233
     httpPort: 9998
     sslDomain: xyz789.ngrok-free.app
     withAp: true
   EOF

   # Run instance 2 (will use local config.yaml and create local database.db)
   ./stegodon
   ```

Now you can test federation between your two instances:
- SSH to instance 1: `ssh localhost -p 23232`
- SSH to instance 2: `ssh localhost -p 23233`
- Follow across instances using `@user@xyz789.ngrok-free.app` format

### Inspecting ActivityPub Messages

Watch HTTP traffic in ngrok's web interface:
1. Open http://127.0.0.1:4040 (ngrok web UI)
2. See all incoming/outgoing HTTP requests
3. Inspect ActivityPub JSON payloads

### Database Inspection

Useful queries (adjust path if using custom data directory):

```bash
# View all remote accounts you've discovered
sqlite3 ~/.config/stegodon/database.db "SELECT username, domain, actor_uri FROM remote_accounts;"

# View all follows
sqlite3 ~/.config/stegodon/database.db "SELECT * FROM follows;"

# View recent activities
sqlite3 ~/.config/stegodon/database.db "SELECT activity_type, actor_uri, created_at FROM activities ORDER BY created_at DESC LIMIT 10;"

# Check delivery queue status
sqlite3 ~/.config/stegodon/database.db "SELECT inbox_uri, attempts, next_retry_at FROM delivery_queue;"
```

**For local config instances:** Replace `~/.config/stegodon/database.db` with `./database.db`

## Alternative Tunneling Services

### Cloudflare Tunnel (cloudflared)

Free and more reliable than ngrok:

```bash
# Install
brew install cloudflare/cloudflare/cloudflared

# Run
cloudflared tunnel --url http://localhost:9999
```

Use the provided `https://` URL as your STEGODON_SSLDOMAIN.

### localtunnel

```bash
# Install
npm install -g localtunnel

# Run
lt --port 9999

# Use provided URL (e.g., https://random-name-123.loca.lt)
STEGODON_WITH_AP=true STEGODON_SSLDOMAIN=random-name-123.loca.lt ./stegodon
```

### serveo (SSH-based)

```bash
ssh -R 80:localhost:9999 serveo.net

# Use provided URL
```

## Production Deployment Notes

ngrok testing is perfect for development, but for production:

1. **Get a real domain:** Register via Namecheap, Cloudflare, etc.
2. **Set up DNS:** Point A record to your server IP
3. **Install reverse proxy:** nginx or Caddy with Let's Encrypt
4. **Configure proxy:** Forward port 9999 with TLS
5. **Update config:**
   ```bash
   STEGODON_WITH_AP=true STEGODON_SSLDOMAIN=yourdomain.com ./stegodon
   ```

See deployment guides for your specific hosting platform.

## Testing Checklist

- [ ] ngrok tunnel running and shows HTTPS URL
- [ ] stegodon started with correct STEGODON_SSLDOMAIN
- [ ] WebFinger endpoint returns valid JSON
- [ ] Mastodon/Pleroma can find your account
- [ ] Following from Mastodon works
- [ ] Followers list shows Mastodon account
- [ ] Posts from stegodon appear in Mastodon timeline
- [ ] Following Mastodon account from stegodon works
- [ ] Mastodon posts appear in stegodon federated timeline

## Known Limitations

- **Temporary domains:** ngrok free domains change on restart (use paid plan for static domains)
- **Rate limits:** Free ngrok has connection limits (upgrade if testing heavily)
- **No media:** stegodon doesn't support images/videos yet
- **No likes UI:** Likes are received but not displayed in UI yet
- **Auto-accept:** All follows are auto-accepted (no approval mechanism)

## Getting Help

If federation isn't working:

1. Check stegodon logs for error messages
2. Test WebFinger endpoint with curl
3. Inspect ngrok web UI for failed requests
4. Check database tables (especially `delivery_queue` and `activities`)
5. Open an issue at https://github.com/deemkeen/stegodon/issues with:
   - Full error logs
   - Commands used to start stegodon and ngrok
   - Output of WebFinger curl test
   - Output of `delivery_queue` database query

## Next Steps

Once federation is working:
- Test with different Fediverse platforms (Pleroma, Pixelfed, etc.)
- Monitor delivery queue for failed deliveries
- Check HTTP signature verification in logs
- Experiment with different ActivityPub clients
- Consider implementing likes, boosts, or media attachments

Happy federating!
