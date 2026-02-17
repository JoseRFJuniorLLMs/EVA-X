// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package email

import (
	"fmt"
	"time"
)

// MissedCallAlertTemplate gera HTML para alerta de chamada perdida
func MissedCallAlertTemplate(elderName, caregiverName string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; background-color: #f4f4f4; margin: 0; padding: 20px; }
        .container { max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .header { background-color: #FF0000; color: white; padding: 20px; text-align: center; }
        .header h1 { margin: 0; font-size: 24px; }
        .content { padding: 30px; }
        .alert-box { background-color: #FFF3CD; border-left: 4px solid #FF0000; padding: 15px; margin: 20px 0; }
        .footer { background-color: #f8f9fa; padding: 15px; text-align: center; font-size: 12px; color: #666; }
        .button { display: inline-block; background-color: #FF0000; color: white; padding: 12px 24px; text-decoration: none; border-radius: 4px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>⚠️ Chamada Não Atendida</h1>
        </div>
        <div class="content">
            <p>Olá <strong>%s</strong>,</p>
            
            <div class="alert-box">
                <strong>ALERTA:</strong> <strong>%s</strong> não atendeu a chamada programada da EVA.
            </div>
            
            <p><strong>Data/Hora:</strong> %s</p>
            
            <p>Por favor, verifique se está tudo bem com o idoso. Este alerta foi enviado porque a notificação push não foi entregue.</p>
            
            <p><strong>Ações recomendadas:</strong></p>
            <ul>
                <li>Ligar para o idoso para verificar se está tudo bem</li>
                <li>Verificar se o dispositivo móvel está funcionando</li>
                <li>Verificar se as notificações estão habilitadas no app</li>
            </ul>
        </div>
        <div class="footer">
            <p>Este é um email automático do sistema EVA - Assistente Virtual para Idosos</p>
            <p>Não responda a este email</p>
        </div>
    </div>
</body>
</html>
    `, caregiverName, elderName, time.Now().Format("02/01/2006 15:04"))
}

// EmergencyAlertTemplate gera HTML para alerta de emergência
func EmergencyAlertTemplate(elderName, caregiverName, reason string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; background-color: #f4f4f4; margin: 0; padding: 20px; }
        .container { max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .header { background-color: #DC3545; color: white; padding: 20px; text-align: center; }
        .header h1 { margin: 0; font-size: 24px; }
        .content { padding: 30px; }
        .critical-box { background-color: #F8D7DA; border-left: 4px solid #DC3545; padding: 15px; margin: 20px 0; }
        .footer { background-color: #f8f9fa; padding: 15px; text-align: center; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🚨 ALERTA CRÍTICO</h1>
        </div>
        <div class="content">
            <p>Olá <strong>%s</strong>,</p>
            
            <div class="critical-box">
                <strong>EMERGÊNCIA DETECTADA:</strong> %s
            </div>
            
            <p><strong>Idoso:</strong> %s</p>
            <p><strong>Data/Hora:</strong> %s</p>
            
            <p><strong>⚠️ AÇÃO IMEDIATA NECESSÁRIA</strong></p>
            <p>Por favor, entre em contato com o idoso imediatamente ou acione serviços de emergência se necessário.</p>
        </div>
        <div class="footer">
            <p>Este é um email automático do sistema EVA - Assistente Virtual para Idosos</p>
            <p>Não responda a este email</p>
        </div>
    </div>
</body>
</html>
    `, caregiverName, reason, elderName, time.Now().Format("02/01/2006 15:04"))
}
