package sheets

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type Service struct {
	ctx context.Context
}

func NewService(ctx context.Context) *Service {
	return &Service{ctx: ctx}
}

// CreateHealthSheet creates a new health tracking spreadsheet
func (s *Service) CreateHealthSheet(accessToken, title string) (string, error) {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
	})

	srv, err := sheets.NewService(s.ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return "", fmt.Errorf("unable to create sheets client: %v", err)
	}

	spreadsheet := &sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title: title,
		},
		Sheets: []*sheets.Sheet{
			{
				Properties: &sheets.SheetProperties{
					Title: "Dados de Saúde",
				},
			},
		},
	}

	result, err := srv.Spreadsheets.Create(spreadsheet).Do()
	if err != nil {
		return "", fmt.Errorf("unable to create spreadsheet: %v", err)
	}

	// Add headers
	headers := &sheets.ValueRange{
		Values: [][]interface{}{
			{"Data", "Hora", "Pressão Arterial", "Glicose", "Medicamento", "Observações"},
		},
	}
	_, err = srv.Spreadsheets.Values.Update(result.SpreadsheetId, "A1:F1", headers).
		ValueInputOption("RAW").Do()

	return result.SpreadsheetUrl, nil
}

// AppendHealthData adds a row to the health spreadsheet
func (s *Service) AppendHealthData(accessToken, spreadsheetID string, data map[string]string) error {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
	})

	srv, err := sheets.NewService(s.ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return fmt.Errorf("unable to create sheets client: %v", err)
	}

	values := &sheets.ValueRange{
		Values: [][]interface{}{
			{
				data["date"],
				data["time"],
				data["blood_pressure"],
				data["glucose"],
				data["medication"],
				data["notes"],
			},
		},
	}

	_, err = srv.Spreadsheets.Values.Append(spreadsheetID, "A:F", values).
		ValueInputOption("RAW").Do()
	if err != nil {
		return fmt.Errorf("unable to append data: %v", err)
	}

	return nil
}
