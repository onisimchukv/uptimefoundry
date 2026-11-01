// SPDX-License-Identifier: MIT

package indexer

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
	"gopkg.in/yaml.v3"
)

var (
	headerRegex    = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	frontmatterSep = regexp.MustCompile(`^---\s*$`)
)

// DocumentChunk represents a segmented portion of an operational runbook or postmortem.
type DocumentChunk struct {
	DocumentID   string                 `json:"document_id"`
	SourceType   models.KnowledgeSourceType `json:"source_type"`
	DocumentTitle string                `json:"document_title"`
	SectionTitle string                 `json:"section_title"`
	PathOrURL    string                 `json:"path_or_url"`
	Anchor       string                 `json:"anchor"`
	Content      string                 `json:"content"`
	Metadata     map[string]any         `json:"metadata"`
}

// MarkdownChunker parses Markdown documents into semantic chunks.
type MarkdownChunker struct {
	MaxChunkSize int // Max characters per chunk (default: 1500)
	ChunkOverlap int // Overlap characters (default: 200)
}

// NewMarkdownChunker creates a new chunker.
func NewMarkdownChunker(maxChunkSize, chunkOverlap int) *MarkdownChunker {
	if maxChunkSize <= 0 {
		maxChunkSize = 1500
	}
	if chunkOverlap < 0 || chunkOverlap >= maxChunkSize {
		chunkOverlap = 200
	}
	return &MarkdownChunker{
		MaxChunkSize: maxChunkSize,
		ChunkOverlap: chunkOverlap,
	}
}

// Parse extracts frontmatter and segments markdown content into searchable chunks.
func (c *MarkdownChunker) Parse(pathOrURL string, sourceType models.KnowledgeSourceType, rawContent string) ([]*models.KnowledgeDocument, error) {
	scanner := bufio.NewScanner(strings.NewReader(rawContent))

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	metadata := make(map[string]any)
	lineIdx := 0

	// 1. Parse YAML Frontmatter if present
	if len(lines) > 0 && frontmatterSep.MatchString(lines[0]) {
		lineIdx++
		var fmLines []string
		for lineIdx < len(lines) {
			if frontmatterSep.MatchString(lines[lineIdx]) {
				lineIdx++
				break
			}
			fmLines = append(fmLines, lines[lineIdx])
			lineIdx++
		}
		if len(fmLines) > 0 {
			fmContent := strings.Join(fmLines, "\n")
			_ = yaml.Unmarshal([]byte(fmContent), &metadata)
		}
	}

	// 2. Discover root document title
	docTitle := ""
	if t, ok := metadata["title"].(string); ok && t != "" {
		docTitle = t
	}

	type section struct {
		title   string
		level   int
		content strings.Builder
	}

	var sections []section
	currentSec := section{title: "Overview", level: 1}

	for i := lineIdx; i < len(lines); i++ {
		line := lines[i]
		if match := headerRegex.FindStringSubmatch(line); match != nil {
			level := len(match[1])
			title := strings.TrimSpace(match[2])

			if docTitle == "" && level == 1 {
				docTitle = title
			}

			if currentSec.content.Len() > 0 {
				sections = append(sections, currentSec)
			}

			currentSec = section{
				title: title,
				level: level,
			}
			continue
		}

		currentSec.content.WriteString(line)
		currentSec.content.WriteString("\n")
	}

	if currentSec.content.Len() > 0 {
		sections = append(sections, currentSec)
	}

	if docTitle == "" {
		docTitle = pathOrURL
	}

	// 3. Segment sections into bounded KnowledgeDocuments
	var docs []*models.KnowledgeDocument
	now := time.Now().UTC()

	for _, sec := range sections {
		secText := strings.TrimSpace(sec.content.String())
		if len(secText) == 0 {
			continue
		}

		anchor := slugify(sec.title)
		fullTitle := fmt.Sprintf("%s > %s", docTitle, sec.title)
		chunkPath := fmt.Sprintf("%s#%s", pathOrURL, anchor)

		// Create merged metadata copy
		secMeta := make(map[string]any)
		for k, v := range metadata {
			secMeta[k] = v
		}
		secMeta["section"] = sec.title
		secMeta["anchor"] = anchor

		if len(secText) <= c.MaxChunkSize {
			fullContent := fmt.Sprintf("# %s\n## %s\n\n%s", docTitle, sec.title, secText)
			docs = append(docs, &models.KnowledgeDocument{
				SourceType: sourceType,
				Title:      fullTitle,
				PathOrURL:  chunkPath,
				Content:    fullContent,
				Metadata:   secMeta,
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		} else {
			// Sub-split with overlap
			subChunks := c.splitText(secText)
			for idx, sc := range subChunks {
				subTitle := fmt.Sprintf("%s (Part %d)", fullTitle, idx+1)
				fullContent := fmt.Sprintf("# %s\n## %s (Part %d)\n\n%s", docTitle, sec.title, idx+1, sc)
				docs = append(docs, &models.KnowledgeDocument{
					SourceType: sourceType,
					Title:      subTitle,
					PathOrURL:  chunkPath,
					Content:    fullContent,
					Metadata:   secMeta,
					CreatedAt:  now,
					UpdatedAt:  now,
				})
			}
		}
	}

	return docs, nil
}

func (c *MarkdownChunker) splitText(text string) []string {
	var chunks []string
	runes := []rune(text)
	total := len(runes)

	start := 0
	for start < total {
		end := start + c.MaxChunkSize
		if end > total {
			end = total
		} else {
			// Try to break at a newline or space within last 100 runes
			for b := end; b > start+c.MaxChunkSize-100; b-- {
				if runes[b-1] == '\n' || runes[b-1] == ' ' {
					end = b
					break
				}
			}
		}

		chunk := strings.TrimSpace(string(runes[start:end]))
		if len(chunk) > 0 {
			chunks = append(chunks, chunk)
		}

		if end == total {
			break
		}
		start = end - c.ChunkOverlap
		if start < 0 {
			start = 0
		}
	}

	return chunks
}

func slugify(s string) string {
	s = strings.ToLower(s)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	s = reg.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
