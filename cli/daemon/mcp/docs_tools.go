package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"encr.dev/internal/conf"
	"encr.dev/internal/urlutil"

	"github.com/algolia/algoliasearch-client-go/v3/algolia/opt"
	"github.com/algolia/algoliasearch-client-go/v3/algolia/search"
	"github.com/mark3labs/mcp-go/mcp"
	"golang.org/x/net/html"
)

func (m *Manager) registerDocsTools() {
	// Add tool for searching Encore documentation using Algolia
	m.server.AddTool(mcp.NewTool("search_docs",
		mcp.WithDescription("Search the Encore documentation using Algolia's search engine. This tool helps find relevant documentation about Encore features, best practices, and examples."),
		mcp.WithString("query", mcp.Description("The search query to find relevant documentation. Can include keywords, feature names, or specific topics you're looking for.")),
		mcp.WithNumber("page", mcp.Description("Page number for pagination, starting from 0. Use this to navigate through large result sets.")),
		mcp.WithNumber("hits_per_page", mcp.Description("Number of results to return per page. Default is 10. Adjust this to control the size of the result set.")),
		mcp.WithArray("facet_filters",
			mcp.Items(map[string]any{
				"type":        "string",
				"description": "Optional array of facet filters to narrow down search results. These can include categories, tags, or other metadata to refine the search.",
			})),
	), m.searchDocs)

	// Add tool for fetching Encore documentation content
	m.server.AddTool(mcp.NewTool("get_docs",
		mcp.WithDescription("Retrieve the full content of specific documentation pages. This tool is useful for getting detailed information about specific topics after finding them with search_docs."),
		mcp.WithArray("paths",
			mcp.Items(map[string]any{
				"type":        "string",
				"description": "List of documentation paths to fetch (e.g. ['/docs/concepts', '/docs/services']). These paths should be valid documentation URLs without the domain.",
			})),
	), m.getDocs)
}

func (m *Manager) searchDocs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters from the request
	query, ok := request.Params.Arguments["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("invalid or missing query parameter")
	}

	// Default pagination settings
	page := 0
	if p, ok := request.Params.Arguments["page"].(float64); ok {
		page = int(p)
	}

	hitsPerPage := 10
	if hpp, ok := request.Params.Arguments["hits_per_page"].(float64); ok {
		hitsPerPage = int(hpp)
	}

	// Process facet filters if provided
	var facetFilters []string
	if filters, ok := request.Params.Arguments["facet_filters"].([]interface{}); ok {
		for _, filter := range filters {
			if filterStr, ok := filter.(string); ok && filterStr != "" {
				facetFilters = append(facetFilters, filterStr)
			}
		}
	}

	// Set context timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Perform the actual search with Algolia
	result, err := performAlgoliaSearch(ctx, query, page, hitsPerPage, facetFilters)
	if err != nil {
		return nil, fmt.Errorf("failed to search docs: %w", err)
	}

	// Marshal the response
	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search results: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// performAlgoliaSearch performs the actual search against Algolia
func performAlgoliaSearch(ctx context.Context, query string, page, hitsPerPage int, facetFilters []string) (map[string]interface{}, error) {
	// Initialize Algolia client with configurable app ID and API key
	// In a production environment, these should be loaded from configuration
	appID := os.Getenv("ENCORE_DOCS_SEARCH_APP_ID")
	if appID == "" {
		appID = "R7DAHI8GEL"
	}
	apiKey := os.Getenv("ENCORE_DOCS_SEARCH_API_KEY")
	if apiKey == "" {
		apiKey = "85bf0533142cccdbbc6b9deb92b19fdf"
	}

	client := search.NewClient(appID, apiKey)
	index := client.InitIndex("encore_docs")

	// Build search parameters
	params := []interface{}{
		opt.Page(page),
		opt.HitsPerPage(hitsPerPage),
	}

	// Add facet filters if any
	if len(facetFilters) > 0 {
		// For a simple AND of all filters - need to convert []string to variadic arguments
		if len(facetFilters) == 1 {
			params = append(params, opt.FacetFilter(facetFilters[0]))
		} else {
			// Convert []string to []interface{} for compatibility
			facetFilterInterfaces := make([]interface{}, len(facetFilters))
			for i, filter := range facetFilters {
				facetFilterInterfaces[i] = filter
			}
			params = append(params, opt.FacetFilterAnd(facetFilterInterfaces...))
		}
	}

	// Perform the search
	res, err := index.Search(query, params...)
	if err != nil {
		return nil, fmt.Errorf("algolia search failed: %w", err)
	}

	// Convert the Algolia response to our expected format
	result := map[string]interface{}{
		"hits":             res.Hits,
		"page":             res.Page,
		"nbHits":           res.NbHits,
		"nbPages":          res.NbPages,
		"hitsPerPage":      res.HitsPerPage,
		"processingTimeMS": res.ProcessingTimeMS,
		"query":            query,
		"params":           res.Params,
	}

	return result, nil
}

func (m *Manager) getDocs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract paths parameter from the request
	var docPaths []string
	if paths, ok := request.Params.Arguments["paths"].([]interface{}); ok {
		for _, path := range paths {
			if pathStr, ok := path.(string); ok && pathStr != "" {
				docPaths = append(docPaths, pathStr)
			}
		}
	}

	if len(docPaths) == 0 {
		return nil, fmt.Errorf("no valid documentation paths provided")
	}

	// Set context timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Fetch content for each path
	result := make(map[string]interface{})
	docs := make(map[string]interface{})

	for _, path := range docPaths {
		// Ensure path starts with a slash
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		url := urlutil.JoinURL(conf.DocsBaseURL(), path)
		content, err := fetchDocContent(ctx, url)
		if err != nil {
			docs[path] = map[string]interface{}{
				"error":   err.Error(),
				"success": false,
			}
		} else {
			docs[path] = map[string]interface{}{
				"content": content,
				"url":     url,
				"success": true,
			}
		}
	}

	result["docs"] = docs
	result["summary"] = map[string]interface{}{
		"total":        len(docPaths),
		"base_url":     conf.DocsBaseURL(),
		"requested_at": time.Now().UTC().Format(time.RFC3339),
	}

	// Marshal the response
	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal document results: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// fetchDocContent fetches content from a URL and returns only the text content from the <main> tag
func fetchDocContent(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add appropriate headers to mimic a browser request
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-OK status code: %d", resp.StatusCode)
	}

	// Parse the HTML document
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Find the main tag
	mainNode := findMainElement(doc)
	if mainNode == nil {
		return "", fmt.Errorf("no <main> tag found in the document")
	}

	// Extract text content from the main tag
	var textContent strings.Builder
	extractText(mainNode, &textContent)

	// Clean up the text content
	cleanedText := cleanText(textContent.String())

	return cleanedText, nil
}

// findMainElement finds the <main> element in the HTML document
func findMainElement(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && strings.ToLower(n.Data) == "main" {
		return n
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if result := findMainElement(c); result != nil {
			return result
		}
	}

	return nil
}

// extractText recursively extracts text nodes from an HTML node
func extractText(n *html.Node, sb *strings.Builder) {
	// Skip script, style, and non-visible elements
	if n.Type == html.ElementNode {
		nodeName := strings.ToLower(n.Data)
		if nodeName == "script" || nodeName == "style" || nodeName == "noscript" ||
			nodeName == "meta" || nodeName == "link" || nodeName == "iframe" {
			return
		}
	}

	// Process text nodes
	if n.Type == html.TextNode {
		text := strings.TrimSpace(n.Data)
		if text != "" {
			sb.WriteString(text)
			sb.WriteString(" ")
		}
	}

	// Recursively process all child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractText(c, sb)
	}

	// Add line breaks for certain block elements
	if n.Type == html.ElementNode {
		nodeName := strings.ToLower(n.Data)
		if nodeName == "p" || nodeName == "div" || nodeName == "h1" ||
			nodeName == "h2" || nodeName == "h3" || nodeName == "h4" ||
			nodeName == "h5" || nodeName == "h6" || nodeName == "li" ||
			nodeName == "br" || nodeName == "tr" {
			sb.WriteString("\n")
		}

		// Add extra line break for more significant sections
		if nodeName == "section" || nodeName == "article" ||
			nodeName == "header" || nodeName == "footer" {
			sb.WriteString("\n\n")
		}
	}
}

// cleanText removes excessive whitespace and normalizes line breaks
func cleanText(text string) string {
	// Replace multiple spaces with a single space
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	// Replace multiple newlines with a maximum of two
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")

	// Trim leading/trailing whitespace
	text = strings.TrimSpace(text)

	return text
}
