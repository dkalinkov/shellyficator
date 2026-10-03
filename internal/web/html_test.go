package web

import (
	"strings"

	"golang.org/x/net/html"
)

func renderedText(markup string) string {
	tokens := html.NewTokenizer(strings.NewReader(markup))
	var text strings.Builder
	for {
		switch tokens.Next() {
		case html.ErrorToken:
			return strings.Join(strings.Fields(text.String()), " ")
		case html.TextToken:
			text.Write(tokens.Text())
			text.WriteByte(' ')
		}
	}
}

func renderedAttribute(markup, element, id, attribute string) string {
	tokens := html.NewTokenizer(strings.NewReader(markup))
	for {
		switch tokens.Next() {
		case html.ErrorToken:
			return ""
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokens.Token()
			if token.Data != element {
				continue
			}
			var foundID, value string
			for _, attr := range token.Attr {
				if attr.Key == "id" {
					foundID = attr.Val
				}
				if attr.Key == attribute {
					value = attr.Val
				}
			}
			if foundID == id {
				return value
			}
		}
	}
}
