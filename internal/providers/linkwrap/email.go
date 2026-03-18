package linkwrap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/net/html"
)

// WrapEmailLinks parses the email template data, rewrites <a href> URLs
// in the HTML body with tracking URLs, and returns the modified template data.
func WrapEmailLinks(data json.RawMessage, key []byte, trackingURL string, projectID, campaignID, userID uuid.UUID) (json.RawMessage, error) {
	var email map[string]any
	if err := json.Unmarshal(data, &email); err != nil {
		return data, fmt.Errorf("parse email data: %w", err)
	}

	htmlBody, ok := email["html"].(string)
	if !ok || htmlBody == "" {
		return data, nil
	}

	wrapped, err := rewriteHTMLLinks(htmlBody, key, trackingURL, projectID, campaignID, userID)
	if err != nil {
		return data, fmt.Errorf("rewrite HTML links: %w", err)
	}

	email["html"] = wrapped
	return json.Marshal(email)
}

// rewriteHTMLLinks parses HTML and rewrites href attributes on <a> tags.
func rewriteHTMLLinks(htmlBody string, key []byte, trackingURL string, projectID, campaignID, userID uuid.UUID) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlBody))
	if err != nil {
		return htmlBody, fmt.Errorf("parse HTML: %w", err)
	}

	var rewriteErr error
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for i, attr := range n.Attr {
				if attr.Key == "href" && shouldWrapURL(attr.Val) {
					token, err := Encrypt(key, LinkPayload{
						ProjectID:  projectID,
						CampaignID: campaignID,
						UserID:     userID,
						URL:        attr.Val,
					})
					if err != nil {
						rewriteErr = err
						return
					}
					n.Attr[i].Val = fmt.Sprintf("%s/c/%s", strings.TrimRight(trackingURL, "/"), token)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if rewriteErr != nil {
		return htmlBody, rewriteErr
	}

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return htmlBody, fmt.Errorf("render HTML: %w", err)
	}

	return buf.String(), nil
}
