// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package twilio

import (
	"fmt"

	"eva/internal/brainstem/config"

	"github.com/twilio/twilio-go"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
)

func MakeOutboundCall(cfg *config.Config, toPhone string, agendamentoID int64) error {
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: cfg.TwilioAccountSID,
		Password: cfg.TwilioAuthToken,
	})

	twimlURL := fmt.Sprintf("https://%s/calls/twiml?agendamento_id=%d", cfg.ServiceDomain, agendamentoID)

	params := &twilioApi.CreateCallParams{
		To:   &toPhone,
		From: &cfg.TwilioPhoneNumber,
		Url:  &twimlURL, // Twilio vai chamar esse endpoint ao conectar
	}

	_, err := client.Api.CreateCall(params)
	return err
}
