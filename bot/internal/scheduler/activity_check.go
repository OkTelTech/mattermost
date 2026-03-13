package scheduler

import (
	"context"
	"log"
	"math/rand/v2"
	"time"

	"oktel-bot/internal/i18n"
	"oktel-bot/internal/mattermost"
	"oktel-bot/internal/model"
	"oktel-bot/internal/store"
)

var vnTZ = time.FixedZone("UTC+7", 7*60*60)

// ActivityChecker periodically DMs users who are currently working
// to confirm they are still active. All state is stored in the attendance record.
type ActivityChecker struct {
	store    *store.AttendanceStore
	mm       *mattermost.Client
	botURL   string
	minSec   int
	maxSec   int
	timeout  time.Duration
	interval time.Duration
}

// NewActivityChecker creates a new ActivityChecker.
func NewActivityChecker(store *store.AttendanceStore, mm *mattermost.Client, botURL string, minSec, maxSec, timeoutSec, intervalSec int) *ActivityChecker {
	return &ActivityChecker{
		store:    store,
		mm:       mm,
		botURL:   botURL,
		minSec:   minSec,
		maxSec:   maxSec,
		timeout:  time.Duration(timeoutSec) * time.Second,
		interval: time.Duration(intervalSec) * time.Second,
	}
}

// Start runs the activity check loop. It blocks until ctx is cancelled.
func (ac *ActivityChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(ac.interval)
	defer ticker.Stop()

	log.Printf("activity checker started (interval=%s, min=%ds, max=%ds, timeout=%s)", ac.interval, ac.minSec, ac.maxSec, ac.timeout)

	ac.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("activity checker stopped")
			return
		case <-ticker.C:
			ac.tick(ctx)
		}
	}
}

func (ac *ActivityChecker) tick(ctx context.Context) {
	today := time.Now().In(vnTZ).Format(time.DateOnly)
	records, err := ac.store.GetAttendanceByDate(ctx, today)
	if err != nil {
		log.Printf("activity check: get attendance: %v", err)
		return
	}

	now := time.Now()
	for _, rec := range records {
		if rec.Status != model.AttendanceStatusWorking {
			continue
		}

		// Handle expired pending checks
		if rec.LastCheckStatus == "pending" && rec.LastCheckAt != nil {
			if now.Sub(*rec.LastCheckAt) >= ac.timeout {
				ac.expireCheck(ctx, rec)
			}
			// Still within timeout - wait
			continue
		}

		// Determine if it's time for a new check
		if rec.NextCheckAt == nil {
			next := now.Add(ac.randomInterval())
			rec.NextCheckAt = &next
			if err := ac.store.UpdateRecord(ctx, rec); err != nil {
				log.Printf("activity check: set initial next_check_at for %s: %v", rec.Username, err)
			}
			continue
		}

		if now.Before(*rec.NextCheckAt) {
			continue
		}

		ac.sendCheck(ctx, rec)
	}
}

func (ac *ActivityChecker) sendCheck(ctx context.Context, rec *model.AttendanceRecord) {
	user, err := ac.mm.GetUser(rec.UserID)
	if err != nil {
		log.Printf("activity check: get user %s: %v", rec.UserID, err)
		return
	}

	lctx := i18n.WithLocale(ctx, user.Locale)
	timeoutSec := int(ac.timeout.Seconds())
	prompt := i18n.T(lctx, "activity.check.prompt", map[string]any{"Timeout": timeoutSec})
	note := i18n.T(lctx, "activity.check.note")
	btnLabel := i18n.T(lctx, "activity.check.btn.confirm")

	post, err := ac.mm.SendDMPost(rec.UserID, &mattermost.Post{
		Message: prompt + "\n" + note,
		Props: mattermost.Props{
			Attachments: []mattermost.Attachment{{
				Actions: []mattermost.Action{{
					Name: btnLabel,
					Type: "button",
					Integration: mattermost.Integration{
						URL: ac.botURL + "/api/attendance/activity-confirm",
						Context: map[string]any{
							"user_id": rec.UserID,
						},
					},
				}},
			}},
		},
	})
	if err != nil {
		log.Printf("activity check: send DM to %s: %v", rec.Username, err)
		return
	}

	now := time.Now()
	rec.LastCheckAt = &now
	rec.LastCheckPostID = post.ID
	rec.LastCheckStatus = "pending"
	rec.NextCheckAt = nil
	if err := ac.store.UpdateRecord(ctx, rec); err != nil {
		log.Printf("activity check: update record for %s: %v", rec.Username, err)
	}
}

func (ac *ActivityChecker) expireCheck(ctx context.Context, rec *model.AttendanceRecord) {
	// Re-check current status
	today := time.Now().In(vnTZ).Format(time.DateOnly)
	fresh, err := ac.store.GetTodayRecord(ctx, rec.UserID, today)
	if err != nil {
		log.Printf("activity check: re-check %s: %v", rec.UserID, err)
		return
	}
	if fresh == nil || fresh.Status != model.AttendanceStatusWorking || fresh.LastCheckStatus != "pending" {
		return
	}

	now := time.Now()
	next := now.Add(ac.randomInterval())
	fresh.LastCheckStatus = "expired"
	fresh.NextCheckAt = &next
	if err := ac.store.UpdateRecord(ctx, fresh); err != nil {
		log.Printf("activity check: update expired for %s: %v", rec.UserID, err)
		return
	}

	// Post notification in attendance channel
	user, _ := ac.mm.GetUser(rec.UserID)
	locale := ""
	if user != nil {
		locale = user.Locale
	}
	lctx := i18n.WithLocale(ctx, locale)
	msg := i18n.T(lctx, "activity.check.expired", map[string]any{
		"Username": fresh.Username,
	})
	if _, err := ac.mm.CreatePost(&mattermost.Post{
		ChannelID: fresh.ChannelID,
		Message:   msg,
	}); err != nil {
		log.Printf("activity check: post expiry for %s: %v", fresh.Username, err)
	}

	// Update DM to show expired message
	ac.mm.UpdatePost(fresh.LastCheckPostID, &mattermost.Post{
		Message: i18n.T(lctx, "activity.check.dm.expired"),
		Props:   mattermost.Props{Attachments: []mattermost.Attachment{}},
	})
}

// HandleConfirm processes a user's confirm button click.
// Checks the DB record to determine if within timeout.
// Returns "confirmed", "expired", or "" (no pending check).
func (ac *ActivityChecker) HandleConfirm(ctx context.Context, userID string) string {
	today := time.Now().In(vnTZ).Format(time.DateOnly)
	rec, err := ac.store.GetTodayRecord(ctx, userID, today)
	if err != nil || rec == nil || rec.LastCheckStatus != "pending" {
		return ""
	}

	now := time.Now()
	next := now.Add(ac.randomInterval())

	if rec.LastCheckAt != nil && now.Sub(*rec.LastCheckAt) <= ac.timeout {
		rec.LastCheckStatus = "confirmed"
		rec.NextCheckAt = &next
		if err := ac.store.UpdateRecord(ctx, rec); err != nil {
			log.Printf("activity check: update confirmed for %s: %v", userID, err)
		}
		return "confirmed"
	}

	// Past timeout - expired
	rec.LastCheckStatus = "expired"
	rec.NextCheckAt = &next
	if err := ac.store.UpdateRecord(ctx, rec); err != nil {
		log.Printf("activity check: update expired for %s: %v", userID, err)
	}

	// Post notification
	user, _ := ac.mm.GetUser(userID)
	locale := ""
	if user != nil {
		locale = user.Locale
	}
	lctx := i18n.WithLocale(ctx, locale)
	msg := i18n.T(lctx, "activity.check.expired", map[string]any{
		"Username": rec.Username,
	})
	ac.mm.CreatePost(&mattermost.Post{
		ChannelID: rec.ChannelID,
		Message:   msg,
	})

	return "expired"
}

func (ac *ActivityChecker) randomInterval() time.Duration {
	n := ac.minSec + rand.IntN(ac.maxSec-ac.minSec+1)
	return time.Duration(n) * time.Second
}
