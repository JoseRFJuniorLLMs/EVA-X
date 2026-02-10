package calendar

import (
	"context"
	"fmt"
	"log"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type Service struct {
	ctx context.Context
}

func NewService(ctx context.Context) *Service {
	return &Service{ctx: ctx}
}

// CreateEventForUser creates an event using user's OAuth token
func (s *Service) CreateEventForUser(accessToken, summary, description, startTimeStr, endTimeStr string) (string, error) {
	// Create OAuth2 token source
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
	})

	srv, err := calendar.NewService(s.ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return "", fmt.Errorf("unable to create calendar client: %v", err)
	}

	// Parse times
	const layout = time.RFC3339
	start, err := time.Parse(layout, startTimeStr)
	if err != nil {
		return "", fmt.Errorf("invalid start time format: %v", err)
	}
	end, err := time.Parse(layout, endTimeStr)
	if err != nil {
		return "", fmt.Errorf("invalid end time format: %v", err)
	}

	event := &calendar.Event{
		Summary:     summary,
		Description: description,
		Start: &calendar.EventDateTime{
			DateTime: start.Format(time.RFC3339),
			TimeZone: "America/Sao_Paulo",
		},
		End: &calendar.EventDateTime{
			DateTime: end.Format(time.RFC3339),
			TimeZone: "America/Sao_Paulo",
		},
	}

	calendarId := "primary"
	event, err = srv.Events.Insert(calendarId, event).Do()
	if err != nil {
		return "", fmt.Errorf("unable to create event: %v", err)
	}

	log.Printf("✅ Event created: %s", event.HtmlLink)
	return event.HtmlLink, nil
}

// ListUpcomingEventsForUser lists events using user's OAuth token
func (s *Service) ListUpcomingEventsForUser(accessToken string) ([]*calendar.Event, error) {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
	})

	srv, err := calendar.NewService(s.ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("unable to create calendar client: %v", err)
	}

	t := time.Now().Format(time.RFC3339)
	events, err := srv.Events.List("primary").ShowDeleted(false).
		SingleEvents(true).TimeMin(t).MaxResults(10).OrderBy("startTime").Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve events: %v", err)
	}

	return events.Items, nil
}
