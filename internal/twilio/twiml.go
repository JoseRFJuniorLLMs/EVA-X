package twilio

import "fmt"

func GenerateTwiML(wsURL string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Connect>
        <Stream url="%s" />
    </Connect>
</Response>`, wsURL)
}
