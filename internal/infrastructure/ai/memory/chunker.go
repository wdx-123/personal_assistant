package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	aidomain "personal_assistant/internal/domain/ai"
)

const (
	defaultChunkMaxChars     = 1200
	defaultChunkOverlapChars = 150
)

type chunkBlockKind string

const (
	blockHeading chunkBlockKind = "heading"
	blockProse   chunkBlockKind = "prose"
	blockCode    chunkBlockKind = "code"
	blockTable   chunkBlockKind = "table"
	blockList    chunkBlockKind = "list"
)

type chunkBlock struct {
	Kind chunkBlockKind
	Text string
}

type chunkListItem struct {
	Marker string
	Text   string
}

// ChunkerOptions 配置记忆文档切分策略。
type ChunkerOptions struct {
	MaxChars     int
	OverlapChars int
}

// ParagraphChunker 按 block / 段落 / 句界优先切分记忆文档，必要时回退到字符窗口。
type ParagraphChunker struct {
	maxChars     int
	overlapChars int
}

// NewParagraphChunker 创建 paragraph-aware chunker。
func NewParagraphChunker(opts ChunkerOptions) *ParagraphChunker {
	if opts.MaxChars <= 0 {
		opts.MaxChars = defaultChunkMaxChars
	}
	if opts.OverlapChars < 0 {
		opts.OverlapChars = 0
	}
	if opts.OverlapChars >= opts.MaxChars {
		opts.OverlapChars = opts.MaxChars / 4
	}
	return &ParagraphChunker{
		maxChars:     opts.MaxChars,
		overlapChars: opts.OverlapChars,
	}
}

// Chunk 把文档正文拆成稳定 chunks。
func (c *ParagraphChunker) Chunk(
	ctx context.Context,
	doc aidomain.MemoryDocumentForIndex,
) ([]aidomain.MemoryDocumentChunk, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	content := normalizeChunkDocumentText(doc.Content)
	if content == "" {
		return nil, nil
	}
	parts := c.splitText(content)
	chunks := make([]aidomain.MemoryDocumentChunk, 0, len(parts))
	for idx, part := range parts {
		hash := aidomain.BuildMemoryDocumentContentHash(part)
		chunkID := buildMemoryChunkID(doc.ID, idx, hash)
		pointID := buildMemoryChunkPointID(doc.ID, idx, hash)
		chunks = append(chunks, aidomain.MemoryDocumentChunk{
			ID:            chunkID,
			DocumentID:    doc.ID,
			ScopeKey:      doc.ScopeKey,
			ScopeType:     doc.ScopeType,
			Visibility:    doc.Visibility,
			UserID:        cloneUintPtr(doc.UserID),
			OrgID:         cloneUintPtr(doc.OrgID),
			MemoryType:    doc.MemoryType,
			Topic:         doc.Topic,
			SourceKind:    doc.SourceKind,
			SourceID:      doc.SourceID,
			ChunkIndex:    idx,
			ContentText:   part,
			ContentHash:   hash,
			TokenEstimate: estimateChunkTokens(part),
			QdrantPointID: pointID,
		})
	}
	return chunks, nil
}

func (c *ParagraphChunker) splitText(content string) []string {
	blocks := splitBlocks(content)
	chunks := make([]string, 0, len(blocks))
	current := ""
	for _, block := range blocks {
		blockChunks := c.splitBlock(block)
		if len(blockChunks) == 0 {
			continue
		}
		if len(blockChunks) > 1 {
			if strings.TrimSpace(current) != "" {
				chunks = append(chunks, current)
				current = ""
			}
			chunks = append(chunks, blockChunks...)
			continue
		}

		part := strings.TrimSpace(blockChunks[0])
		if part == "" {
			continue
		}
		if current == "" {
			current = part
			continue
		}

		candidate := joinChunkParts(current, part, "\n\n")
		if utf8.RuneCountInString(candidate) <= c.maxChars {
			current = candidate
			continue
		}

		chunks = append(chunks, current)
		current = c.startChunkWithOverlap(current, part, "\n\n")
	}
	if strings.TrimSpace(current) != "" {
		chunks = append(chunks, current)
	}
	return chunks
}

func (c *ParagraphChunker) splitBlock(block chunkBlock) []string {
	switch block.Kind {
	case blockHeading:
		return c.splitHeadingBlock(block.Text)
	case blockList:
		return c.splitListBlock(block.Text)
	case blockTable:
		return c.splitTableBlock(block.Text)
	case blockCode:
		return c.splitCodeBlock(block.Text)
	default:
		return c.splitProseBlock(block.Text)
	}
}

func (c *ParagraphChunker) splitHeadingBlock(text string) []string {
	text = normalizeInlineText(text)
	if text == "" {
		return nil
	}
	if utf8.RuneCountInString(text) <= c.maxChars {
		return []string{text}
	}
	return splitByRuneWindow(text, c.maxChars, 0)
}

func (c *ParagraphChunker) splitProseBlock(text string) []string {
	text = normalizeInlineText(text)
	if text == "" {
		return nil
	}
	if utf8.RuneCountInString(text) <= c.maxChars {
		return []string{text}
	}
	return c.buildChunkSequence(splitNarrativeUnits(text, c.maxChars), " ")
}

func (c *ParagraphChunker) splitListBlock(text string) []string {
	items := splitListItems(text)
	if len(items) == 0 {
		return c.splitProseBlock(text)
	}

	parts := make([]string, 0, len(items))
	for _, item := range items {
		body := normalizeInlineText(item.Text)
		if body == "" {
			continue
		}
		prefix := strings.TrimSpace(item.Marker)
		if prefix == "" {
			prefix = "-"
		}
		fullText := strings.TrimSpace(prefix + " " + body)
		if utf8.RuneCountInString(fullText) <= c.maxChars {
			parts = append(parts, fullText)
			continue
		}

		bodyBudget := c.maxChars - utf8.RuneCountInString(prefix) - 1
		if bodyBudget <= 0 {
			bodyBudget = c.maxChars
		}
		for _, unit := range splitNarrativeUnits(body, bodyBudget) {
			unit = strings.TrimSpace(unit)
			if unit == "" {
				continue
			}
			parts = append(parts, strings.TrimSpace(prefix+" "+unit))
		}
	}
	return c.buildChunkSequence(parts, "\n")
}

func (c *ParagraphChunker) splitTableBlock(text string) []string {
	text = normalizeChunkDocumentText(text)
	if text == "" {
		return nil
	}
	if utf8.RuneCountInString(text) <= c.maxChars {
		return []string{text}
	}

	lines := splitRawLines(text)
	if len(lines) < 2 || !isMarkdownTableDelimiterLine(lines[1]) {
		return splitByRuneWindow(text, c.maxChars, 0)
	}

	prefix := strings.TrimSpace(lines[0]) + "\n" + strings.TrimSpace(lines[1])
	bodyBudget := c.maxChars - utf8.RuneCountInString(prefix) - 1
	if bodyBudget <= 0 {
		return splitByRuneWindow(text, c.maxChars, 0)
	}

	rows := make([]string, 0, len(lines)-2)
	for _, line := range lines[2:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) <= bodyBudget {
			rows = append(rows, line)
			continue
		}
		rows = append(rows, splitByRuneWindow(line, bodyBudget, 0)...)
	}
	return packWrappedParts(rows, prefix, "", "\n", c.maxChars)
}

func (c *ParagraphChunker) splitCodeBlock(text string) []string {
	text = normalizeChunkDocumentText(text)
	if text == "" {
		return nil
	}
	if utf8.RuneCountInString(text) <= c.maxChars {
		return []string{text}
	}

	lines := splitRawLines(text)
	prefix := ""
	suffix := ""
	bodyLines := lines
	if len(lines) > 0 && isFenceLine(lines[0]) {
		prefix = strings.TrimSpace(lines[0])
		bodyLines = lines[1:]
	}
	if len(bodyLines) > 0 && isFenceLine(bodyLines[len(bodyLines)-1]) {
		suffix = strings.TrimSpace(bodyLines[len(bodyLines)-1])
		bodyLines = bodyLines[:len(bodyLines)-1]
	}

	bodyBudget := c.maxChars
	if prefix != "" {
		bodyBudget -= utf8.RuneCountInString(prefix) + 1
	}
	if suffix != "" {
		bodyBudget -= utf8.RuneCountInString(suffix) + 1
	}
	if bodyBudget <= 0 {
		return splitByRuneWindow(text, c.maxChars, 0)
	}

	groups := splitCodeGroups(bodyLines)
	parts := make([]string, 0, len(groups))
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if utf8.RuneCountInString(group) <= bodyBudget {
			parts = append(parts, group)
			continue
		}
		parts = append(parts, splitCodeGroupByLineWindow(group, bodyBudget)...)
	}
	return packWrappedParts(parts, prefix, suffix, "\n\n", c.maxChars)
}

func (c *ParagraphChunker) buildChunkSequence(parts []string, separator string) []string {
	if len(parts) == 0 {
		return nil
	}
	chunks := make([]string, 0, len(parts))
	current := ""
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if current == "" {
			current = part
			continue
		}
		candidate := joinChunkParts(current, part, separator)
		if utf8.RuneCountInString(candidate) <= c.maxChars {
			current = candidate
			continue
		}
		chunks = append(chunks, current)
		current = c.startChunkWithOverlap(current, part, separator)
	}
	if strings.TrimSpace(current) != "" {
		chunks = append(chunks, current)
	}
	return chunks
}

func (c *ParagraphChunker) startChunkWithOverlap(previous string, next string, separator string) string {
	next = strings.TrimSpace(next)
	if next == "" {
		return ""
	}
	overlap := c.chunkOverlap(previous)
	if overlap == "" {
		return next
	}
	candidate := joinChunkParts(overlap, next, separator)
	if utf8.RuneCountInString(candidate) <= c.maxChars {
		return candidate
	}
	return next
}

func (c *ParagraphChunker) chunkOverlap(value string) string {
	if c.overlapChars <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= c.overlapChars {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(string(runes[len(runes)-c.overlapChars:]))
}

func splitBlocks(content string) []chunkBlock {
	content = normalizeChunkDocumentText(content)
	if content == "" {
		return nil
	}

	lines := strings.Split(content, "\n")
	blocks := make([]chunkBlock, 0, len(lines))
	for index := 0; index < len(lines); {
		line := strings.TrimRight(lines[index], " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			index++
			continue
		}

		switch {
		case isFenceLine(trimmed):
			start := index
			index++
			for index < len(lines) {
				if isFenceLine(strings.TrimSpace(lines[index])) {
					index++
					break
				}
				index++
			}
			blocks = append(blocks, chunkBlock{Kind: blockCode, Text: strings.Join(lines[start:index], "\n")})
		case isHeadingLine(trimmed):
			blocks = append(blocks, chunkBlock{Kind: blockHeading, Text: trimmed})
			index++
		case isMarkdownTableStart(lines, index):
			start := index
			index += 2
			for index < len(lines) {
				next := strings.TrimSpace(lines[index])
				if next == "" || (!strings.Contains(next, "|") && !isMarkdownTableDelimiterLine(next)) {
					break
				}
				index++
			}
			blocks = append(blocks, chunkBlock{Kind: blockTable, Text: strings.Join(lines[start:index], "\n")})
		case isListItemLine(trimmed):
			start := index
			index++
			for index < len(lines) {
				next := strings.TrimSpace(lines[index])
				if next == "" || isFenceLine(next) || isHeadingLine(next) || isMarkdownTableStart(lines, index) {
					break
				}
				index++
			}
			blocks = append(blocks, chunkBlock{Kind: blockList, Text: strings.Join(lines[start:index], "\n")})
		default:
			start := index
			index++
			for index < len(lines) {
				next := strings.TrimSpace(lines[index])
				if next == "" || isFenceLine(next) || isHeadingLine(next) || isMarkdownTableStart(lines, index) || isListItemLine(next) {
					break
				}
				index++
			}
			blocks = append(blocks, chunkBlock{Kind: blockProse, Text: strings.Join(lines[start:index], "\n")})
		}
	}
	return blocks
}

// MergeChunkTextsWithOverlap 按字符 overlap 合并相邻 chunk，避免 recall 扩窗后重复注入。
func MergeChunkTextsWithOverlap(parts []string, overlapChars int) string {
	if len(parts) == 0 {
		return ""
	}
	merged := ""
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if merged == "" {
			merged = part
			continue
		}
		merged += trimLeadingChunkOverlap(merged, part, overlapChars)
	}
	return strings.TrimSpace(merged)
}

func trimLeadingChunkOverlap(previous string, current string, overlapChars int) string {
	if current == "" || overlapChars <= 0 {
		return current
	}
	prevRunes := []rune(previous)
	currentRunes := []rune(current)
	limit := overlapChars
	if len(prevRunes) < limit {
		limit = len(prevRunes)
	}
	if len(currentRunes) < limit {
		limit = len(currentRunes)
	}
	for size := limit; size > 0; size-- {
		if string(prevRunes[len(prevRunes)-size:]) == string(currentRunes[:size]) {
			return string(currentRunes[size:])
		}
	}
	return current
}

func splitNarrativeUnits(text string, maxChars int) []string {
	text = normalizeInlineText(text)
	if text == "" {
		return nil
	}
	if maxChars <= 0 || utf8.RuneCountInString(text) <= maxChars {
		return []string{text}
	}

	strongUnits := splitByBoundaries(text, isStrongSentenceBoundary)
	if len(strongUnits) == 0 {
		strongUnits = []string{text}
	}

	units := make([]string, 0, len(strongUnits))
	for _, unit := range strongUnits {
		unit = normalizeInlineText(unit)
		if unit == "" {
			continue
		}
		if utf8.RuneCountInString(unit) <= maxChars {
			units = append(units, unit)
			continue
		}

		softUnits := splitByBoundaries(unit, isSoftSentenceBoundary)
		if len(softUnits) == 0 {
			softUnits = []string{unit}
		}
		for _, softUnit := range softUnits {
			softUnit = normalizeInlineText(softUnit)
			if softUnit == "" {
				continue
			}
			if utf8.RuneCountInString(softUnit) <= maxChars {
				units = append(units, softUnit)
				continue
			}
			units = append(units, splitByRuneWindow(softUnit, maxChars, 0)...)
		}
	}
	return units
}

func splitByBoundaries(text string, isBoundary func(rune) bool) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	parts := make([]string, 0, 8)
	var builder strings.Builder
	for _, r := range text {
		builder.WriteRune(r)
		if isBoundary(r) {
			part := strings.TrimSpace(builder.String())
			if part != "" {
				parts = append(parts, part)
			}
			builder.Reset()
		}
	}
	tail := strings.TrimSpace(builder.String())
	if tail != "" {
		parts = append(parts, tail)
	}
	return parts
}

func splitListItems(text string) []chunkListItem {
	lines := splitRawLines(text)
	items := make([]chunkListItem, 0, len(lines))
	current := chunkListItem{}
	flush := func() {
		if strings.TrimSpace(current.Text) == "" {
			current = chunkListItem{}
			return
		}
		items = append(items, current)
		current = chunkListItem{}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		marker, body, ok := parseListMarker(trimmed)
		if ok {
			flush()
			current = chunkListItem{Marker: marker, Text: body}
			continue
		}
		if current.Text == "" {
			current = chunkListItem{Marker: "-", Text: trimmed}
			continue
		}
		current.Text = strings.TrimSpace(current.Text + " " + trimmed)
	}
	flush()
	return items
}

func parseListMarker(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if len(line) >= 2 {
		switch line[0] {
		case '-', '*', '+':
			if line[1] == ' ' || line[1] == '\t' {
				return string(line[0]), strings.TrimSpace(line[1:]), true
			}
		}
	}

	index := 0
	for index < len(line) && line[index] >= '0' && line[index] <= '9' {
		index++
	}
	if index > 0 && index+1 < len(line) && (line[index] == '.' || line[index] == ')') &&
		(line[index+1] == ' ' || line[index+1] == '\t') {
		return strings.TrimSpace(line[:index+1]), strings.TrimSpace(line[index+1:]), true
	}
	return "", "", false
}

func splitCodeGroups(lines []string) []string {
	groups := make([]string, 0, len(lines))
	current := make([]string, 0, len(lines))
	flush := func() {
		if len(current) == 0 {
			return
		}
		groups = append(groups, strings.Join(current, "\n"))
		current = current[:0]
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		current = append(current, strings.TrimRight(line, " \t"))
	}
	flush()
	return groups
}

func splitCodeGroupByLineWindow(group string, maxChars int) []string {
	lines := splitRawLines(group)
	if len(lines) == 0 {
		return nil
	}
	parts := make([]string, 0, len(lines))
	current := ""
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) > maxChars {
			if strings.TrimSpace(current) != "" {
				parts = append(parts, current)
				current = ""
			}
			parts = append(parts, splitByRuneWindow(line, maxChars, 0)...)
			continue
		}
		if current == "" {
			current = line
			continue
		}
		candidate := current + "\n" + line
		if utf8.RuneCountInString(candidate) <= maxChars {
			current = candidate
			continue
		}
		parts = append(parts, current)
		current = line
	}
	if strings.TrimSpace(current) != "" {
		parts = append(parts, current)
	}
	return parts
}

func packWrappedParts(parts []string, prefix string, suffix string, separator string, maxChars int) []string {
	if len(parts) == 0 {
		return nil
	}
	chunks := make([]string, 0, len(parts))
	current := ""
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		candidateBody := part
		if current != "" {
			candidateBody = joinChunkParts(current, part, separator)
		}
		candidate := wrapChunkBody(prefix, suffix, candidateBody)
		if utf8.RuneCountInString(candidate) <= maxChars {
			current = candidateBody
			continue
		}
		if strings.TrimSpace(current) != "" {
			chunks = append(chunks, wrapChunkBody(prefix, suffix, current))
		}
		current = part
	}
	if strings.TrimSpace(current) != "" {
		chunks = append(chunks, wrapChunkBody(prefix, suffix, current))
	}
	return chunks
}

func wrapChunkBody(prefix string, suffix string, body string) string {
	body = strings.TrimSpace(body)
	if prefix == "" && suffix == "" {
		return body
	}
	var builder strings.Builder
	if strings.TrimSpace(prefix) != "" {
		builder.WriteString(strings.TrimSpace(prefix))
	}
	if body != "" {
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(body)
	}
	if strings.TrimSpace(suffix) != "" {
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(strings.TrimSpace(suffix))
	}
	return strings.TrimSpace(builder.String())
}

func joinChunkParts(left string, right string, separator string) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	switch {
	case left == "":
		return right
	case right == "":
		return left
	default:
		return strings.TrimSpace(left + separator + right)
	}
}

func splitByRuneWindow(value string, maxChars int, overlap int) []string {
	runes := []rune(value)
	if len(runes) <= maxChars {
		return []string{strings.TrimSpace(value)}
	}
	step := maxChars - overlap
	if step <= 0 {
		step = maxChars
	}
	chunks := make([]string, 0, (len(runes)/step)+1)
	for start := 0; start < len(runes); start += step {
		end := start + maxChars
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(runes) {
			break
		}
	}
	return chunks
}

func splitRawLines(value string) []string {
	value = normalizeChunkDocumentText(value)
	if value == "" {
		return nil
	}
	raw := strings.Split(value, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		lines = append(lines, strings.TrimRight(line, " \t"))
	}
	return lines
}

func normalizeChunkDocumentText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.TrimSpace(value)
}

func normalizeInlineText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func isFenceLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "```")
}

func isHeadingLine(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "#")
}

func isListItemLine(line string) bool {
	_, _, ok := parseListMarker(line)
	return ok
}

func isMarkdownTableStart(lines []string, index int) bool {
	if index < 0 || index+1 >= len(lines) {
		return false
	}
	current := strings.TrimSpace(lines[index])
	next := strings.TrimSpace(lines[index+1])
	if current == "" || next == "" {
		return false
	}
	return strings.Contains(current, "|") && isMarkdownTableDelimiterLine(next)
}

func isMarkdownTableDelimiterLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	trimmed := strings.ReplaceAll(line, "|", "")
	trimmed = strings.ReplaceAll(trimmed, " ", "")
	trimmed = strings.ReplaceAll(trimmed, "\t", "")
	if trimmed == "" || !strings.Contains(trimmed, "-") {
		return false
	}
	for _, r := range trimmed {
		if r != '-' && r != ':' {
			return false
		}
	}
	return true
}

func isStrongSentenceBoundary(r rune) bool {
	switch r {
	case '。', '！', '？', '；', '.', '!', '?', ';':
		return true
	default:
		return false
	}
}

func isSoftSentenceBoundary(r rune) bool {
	switch r {
	case '，', ',', '：', ':':
		return true
	default:
		return false
	}
}

func estimateChunkTokens(value string) int {
	runes := utf8.RuneCountInString(value)
	if runes == 0 {
		return 0
	}
	return (runes + 3) / 4
}

func buildMemoryChunkID(documentID string, chunkIndex int, contentHash string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s", documentID, chunkIndex, contentHash)))
	return "mem_chunk_" + hex.EncodeToString(sum[:])[:32]
}

func buildMemoryChunkPointID(documentID string, chunkIndex int, contentHash string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s", documentID, chunkIndex, contentHash)))
	hexValue := hex.EncodeToString(sum[:])[:32]
	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		hexValue[0:8],
		hexValue[8:12],
		hexValue[12:16],
		hexValue[16:20],
		hexValue[20:32],
	)
}

func cloneUintPtr(value *uint) *uint {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
