// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package browser

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// PageResult resultado de navegação
type PageResult struct {
	URL        string            `json:"url"`
	StatusCode int               `json:"status_code"`
	Title      string            `json:"title"`
	Text       string            `json:"text"`
	Links      []Link            `json:"links,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// Link um link extraído da página
type Link struct {
	Text string `json:"text"`
	Href string `json:"href"`
}

// FormResult resultado de submissão de formulário
type FormResult struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Body       string `json:"body"`
}

// Service cliente HTTP para browser automation
type Service struct {
	client    *http.Client
	userAgent string
}

// NewService cria browser service
func NewService() *Service {
	return &Service{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		userAgent: "EVA-Mind/1.0 (Autonomous Agent)",
	}
}

// Navigate busca uma URL e extrai conteúdo
func (s *Service) Navigate(targetURL string) (*PageResult, error) {
	if !strings.HasPrefix(targetURL, "http") {
		targetURL = "https://" + targetURL
	}

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("URL inválida: %v", err)
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao acessar: %v", err)
	}
	defer resp.Body.Close()

	// Limitar leitura a 2MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("erro ao ler: %v", err)
	}

	html := string(body)

	result := &PageResult{
		URL:        resp.Request.URL.String(),
		StatusCode: resp.StatusCode,
		Title:      extractTitle(html),
		Text:       extractText(html),
		Links:      extractLinks(html, resp.Request.URL),
		Headers: map[string]string{
			"content-type": resp.Header.Get("Content-Type"),
		},
	}

	// Truncar texto
	if len(result.Text) > 10000 {
		result.Text = result.Text[:10000] + "... [truncado]"
	}

	// Limitar links
	if len(result.Links) > 50 {
		result.Links = result.Links[:50]
	}

	return result, nil
}

// FillForm submete um formulário via POST
func (s *Service) FillForm(targetURL string, fields map[string]string) (*FormResult, error) {
	if !strings.HasPrefix(targetURL, "http") {
		targetURL = "https://" + targetURL
	}

	form := url.Values{}
	for k, v := range fields {
		form.Set(k, v)
	}

	req, err := http.NewRequest("POST", targetURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("erro ao criar request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", s.userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao submeter: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))

	result := &FormResult{
		URL:        resp.Request.URL.String(),
		StatusCode: resp.StatusCode,
		Body:       truncateStr(extractText(string(body)), 5000),
	}

	return result, nil
}

// ExtractData extrai dados específicos de uma página usando seletor CSS simples
func (s *Service) ExtractData(targetURL, selector string) ([]string, error) {
	page, err := s.Navigate(targetURL)
	if err != nil {
		return nil, err
	}

	// Seletor simples: busca por tag, class ou id no texto extraído
	// Para seletores complexos, usar o sandbox com Python+BeautifulSoup
	var results []string

	switch {
	case selector == "links":
		for _, link := range page.Links {
			results = append(results, fmt.Sprintf("%s: %s", link.Text, link.Href))
		}
	case selector == "title":
		results = append(results, page.Title)
	case selector == "text":
		results = append(results, page.Text)
	default:
		// Buscar por padrão no texto
		lines := strings.Split(page.Text, "\n")
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), strings.ToLower(selector)) {
				results = append(results, strings.TrimSpace(line))
			}
		}
	}

	return results, nil
}

// ---- HTML parsing helpers (sem dependências externas) ----

var titleRe = regexp.MustCompile(`(?i)<title[^>]*>(.*?)</title>`)
var tagRe = regexp.MustCompile(`<[^>]+>`)
var scriptRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
var styleRe = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
var linkRe = regexp.MustCompile(`(?i)<a[^>]+href=["']([^"']+)["'][^>]*>(.*?)</a>`)
var spaceRe = regexp.MustCompile(`\s+`)

func extractTitle(html string) string {
	matches := titleRe.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func extractText(html string) string {
	// Remover scripts e styles
	text := scriptRe.ReplaceAllString(html, "")
	text = styleRe.ReplaceAllString(text, "")
	// Substituir tags por espaço
	text = tagRe.ReplaceAllString(text, " ")
	// Normalizar whitespace
	text = spaceRe.ReplaceAllString(text, " ")
	// Decode entidades comuns
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	return strings.TrimSpace(text)
}

func extractLinks(html string, baseURL *url.URL) []Link {
	matches := linkRe.FindAllStringSubmatch(html, -1)
	var links []Link
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		href := m[1]
		text := tagRe.ReplaceAllString(m[2], "")
		text = strings.TrimSpace(text)

		// Resolver URLs relativos
		if baseURL != nil && !strings.HasPrefix(href, "http") {
			if ref, err := url.Parse(href); err == nil {
				href = baseURL.ResolveReference(ref).String()
			}
		}

		if text != "" || href != "" {
			links = append(links, Link{Text: text, Href: href})
		}
	}
	return links
}

func truncateStr(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "... [truncado]"
	}
	return s
}
