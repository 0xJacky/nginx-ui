package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/html"
)

type Directive struct {
	Links []string `json:"links"`
}

func main() {
	// Fetch page content
	resp, err := http.Get("https://nginx.org/en/docs/dirindex.html")
	if err != nil {
		fmt.Println("Error fetching page:", err)
		return
	}
	defer resp.Body.Close()

	// Parse HTML
	doc, err := html.Parse(resp.Body)
	if err != nil {
		fmt.Println("Error parsing HTML:", err)
		return
	}

	// Change storage structure to map
	directives := make(map[string]Directive)

	// Find node with id="content"
	var content *html.Node
	var findContent func(*html.Node)
	findContent = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" {
			for _, attr := range n.Attr {
				if attr.Key == "id" && attr.Val == "content" {
					content = n
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findContent(c)
		}
	}
	findContent(doc)

	// Extract all a tags from content
	if content != nil {
		var extractLinks func(*html.Node)
		extractLinks = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "a" {
				var href string
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						href = attr.Val
						break
					}
				}
				if href != "" && n.FirstChild != nil {
					name := strings.TrimSpace(n.FirstChild.Data)
					if name != "" {
						fullLink := "https://nginx.org/en/docs/" + href
						directive, exists := directives[name]
						if !exists {
							directives[name] = Directive{
								Links: []string{fullLink},
							}
						} else {
							// Check if link already exists to avoid duplicates
							linkExists := false
							for _, existingLink := range directive.Links {
								if existingLink == fullLink {
									linkExists = true
									break
								}
							}
							if !linkExists {
								directive.Links = append(directive.Links, fullLink)
								directives[name] = directive
							}
						}
					}
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				extractLinks(c)
			}
		}
		extractLinks(content)
	}

	// Write results to JSON file
	jsonData, err := json.MarshalIndent(directives, "", "  ")
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}

	err = os.WriteFile("../../internal/nginx/nginx_directives.json", jsonData, 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
		return
	}

	fmt.Printf("Successfully parsed %d directives and saved to nginx_directives.json\n", len(directives))
}
