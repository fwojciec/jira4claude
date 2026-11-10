package markdown

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/fwojciec/jira4claude"
)

// Protected nodes (#149) are those whose content or behaviour plain text
// would lose: media, mediaSingle, mediaGroup, mediaInline, mention, and any
// node type the reader does not know. The reader writes each as a token,
// numbered in document order:
//
//	{{mediaSingle:1 shot.png}}   a block node, on a line of its own
//	{{mention:2 Jane Doe}}       an inline node, in the text
//
// The word before the colon is the node type and the text after the number
// is a label; both are for whoever reads the Markdown. Reattach trusts only
// the number. A token is plain text to the writer, so ToADF keeps it as text
// and Reattach swaps it for the original node.
//
// Literal text: a Markdown escape cannot keep text Jira holds from reading as
// a token, because the writer decodes every escape before Reattach sees the
// text. So the token syntax varies instead. Tokens open with "{{" plus a
// salt of k "!" characters, and the reader picks the smallest k for which the
// document, written out and read back with ToADF, holds exactly the tokens it
// wrote and nothing else that parses as one. Literal text is never changed;
// a document that already holds "{{mention:1 x}}" gets tokens such as
// "{{!mention:1 Jane Doe}}". Reattach derives k from the original document
// the same way, so both sides agree. Text in code (a code mark or a code
// block) is never read as a token.

const (
	// tokenSaltChar is repeated after "{{" to tell tokens apart from
	// literal text that looks like them.
	tokenSaltChar = "!"
	// maxTokenSalt bounds the search for a salt.
	maxTokenSalt = 64
	// maxLabelRunes bounds a token's label.
	maxLabelRunes = 80
	// tokenItemType is the type of the item emittingInline makes for an
	// inline token; its Text is the token's Markdown. It is not an ADF type.
	tokenItemType = "#token"
)

// protectedNode is one node the reader wrote as a token.
type protectedNode struct {
	node  jira4claude.ADFNode // as read, restored verbatim
	block bool                // written on a line of its own
	// parent is the container class (see containerClass) the reader found
	// a block node in.
	parent string
	// token is the token as text, which is what ToADF reads it back as.
	token string
	// kind is the word before the token's colon. Reattach checks it, so a
	// stale token whose number now names another kind of node is caught.
	kind string
}

// tokenRegistry collects the tokens of one read. It rides on the read
// side's skippedCollector, which every rendering function already receives.
type tokenRegistry struct {
	salt  string
	nodes []protectedNode
	// blocks holds the types of the blocks being rendered by adfNodeToGFM,
	// innermost last, so a block token knows its container.
	blocks []string
}

// enterBlock and leaveBlock track the block adfNodeToGFM is rendering.
func (s *skippedCollector) enterBlock(nodeType string) {
	if s != nil && s.tokens != nil {
		s.tokens.blocks = append(s.tokens.blocks, nodeType)
	}
}

func (s *skippedCollector) leaveBlock() {
	if s != nil && s.tokens != nil && len(s.tokens.blocks) > 0 {
		s.tokens.blocks = s.tokens.blocks[:len(s.tokens.blocks)-1]
	}
}

// blockContainer returns the container class of the block about to be
// rendered: the innermost block being rendered, or "doc". A layout is
// flattened into its parent on read, so it is skipped: its content is
// written back in the parent.
func (s *skippedCollector) blockContainer() string {
	if s == nil || s.tokens == nil {
		return "doc"
	}
	for i := len(s.tokens.blocks) - 1; i >= 0; i-- {
		switch t := s.tokens.blocks[i]; t {
		case "layoutSection", "layoutColumn":
		default:
			return containerClass(t)
		}
	}
	return "doc"
}

// protectBlock records node as a block protected node found in the given
// container class and returns its token's Markdown.
func (s *skippedCollector) protectBlock(node *jira4claude.ADFNode, container string) string {
	return s.protect(node, true, container)
}

// protectMisplaced writes a child its container (of type parent) does not
// expect, such as a mediaSingle directly in a bulletList, as a block token,
// so the node is kept. The token goes in an item or cell of its own, where
// Reattach restores the node, so the change of structure is reported as
// degraded.
func (s *skippedCollector) protectMisplaced(node *jira4claude.ADFNode, parent string) string {
	if s != nil {
		s.degrade(nodeName(node.Type), fmt.Sprintf("a %s directly inside a %s is kept, but in an item or cell of its own",
			nodeName(node.Type), parent))
	}
	return s.protectBlock(node, s.blockContainer())
}

// dropMisplaced reports a child its container (of type parent) does not
// expect and that has no Markdown form there, such as a paragraph directly in
// a taskList, as dropped.
func (s *skippedCollector) dropMisplaced(node *jira4claude.ADFNode, parent string) {
	if s == nil {
		return
	}
	s.record(jira4claude.ConversionIssue{
		Kind: jira4claude.ConversionDropped,
		Node: nodeName(node.Type),
		Detail: fmt.Sprintf("a %s directly inside a %s has no Markdown form and is left out",
			nodeName(node.Type), parent),
	})
}

// protectInline records node as an inline protected node and returns the
// item emittingInline emits for it.
func (s *skippedCollector) protectInline(node *jira4claude.ADFNode) jira4claude.ADFNode {
	return jira4claude.ADFNode{Type: tokenItemType, Text: s.protect(node, false, "")}
}

func (s *skippedCollector) protect(node *jira4claude.ADFNode, block bool, container string) string {
	kind := node.Type
	if !isTokenKind(kind) {
		kind = "node"
	}
	label := tokenLabel(node)
	if s == nil || s.tokens == nil {
		// No registry: the token cannot be re-attached, but still marks
		// the node's place.
		return tokenMarkdown("", kind, 0, label)
	}
	reg := s.tokens
	n := len(reg.nodes) + 1
	reg.nodes = append(reg.nodes, protectedNode{
		node:   *node,
		block:  block,
		parent: container,
		token:  tokenText(reg.salt, kind, n, label),
		kind:   kind,
	})
	return tokenMarkdown(reg.salt, kind, n, label)
}

// isTokenKind reports whether s can stand before the colon of a token.
func isTokenKind(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		letter := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		if !letter && (i == 0 || c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// tokenText returns a token as text.
func tokenText(salt, kind string, n int, label string) string {
	s := "{{" + salt + kind + ":" + strconv.Itoa(n)
	if label != "" {
		s += " " + label
	}
	return s + "}}"
}

// tokenMarkdown returns a token as Markdown: its label escaped so the writer
// reads the token back as the same text.
func tokenMarkdown(salt, kind string, n int, label string) string {
	if label != "" {
		label = escapeLabel(label)
	}
	return tokenText(salt, kind, n, label)
}

// tokenPattern matches the tokens of the given salt. Submatch 1 is the kind,
// 2 the number.
func tokenPattern(salt string) *regexp.Regexp {
	return regexp.MustCompile(`\{\{` + regexp.QuoteMeta(salt) + `([A-Za-z][A-Za-z0-9]*):([0-9]+)(?: [^{}\n]*)?\}\}`)
}

// tokenLabel returns the label of a protected node's token: a name a reader
// recognises the node by. It is never read back.
func tokenLabel(node *jira4claude.ADFNode) string {
	var label string
	switch node.Type {
	case "mention":
		label = strings.TrimPrefix(attrString(node, "text"), "@")
		if label == "" {
			label = attrString(node, "id")
		}
	case "mediaSingle", "mediaGroup":
		var names []string
		for i := range node.Content {
			if node.Content[i].Type == "media" {
				if name := mediaLabel(&node.Content[i]); name != "" {
					names = append(names, name)
				}
			}
		}
		label = strings.Join(names, ", ")
	case "media", "mediaInline":
		label = mediaLabel(node)
	default:
		label = attrString(node, "text")
		if label == "" {
			label = attrString(node, "extensionKey")
		}
	}
	return sanitizeLabel(label)
}

// mediaLabel returns the alt text of a media node, or its file name.
func mediaLabel(node *jira4claude.ADFNode) string {
	if alt := attrString(node, "alt"); alt != "" {
		return alt
	}
	return attrString(node, "__fileName")
}

// attrString returns the string attribute key of node, or "".
func attrString(node *jira4claude.ADFNode, key string) string {
	if node.Attrs == nil {
		return ""
	}
	var attrs map[string]any
	if err := json.Unmarshal(node.Attrs, &attrs); err != nil {
		return ""
	}
	s, _ := attrs[key].(string)
	return s
}

// sanitizeLabel keeps a label on one line, without braces (so it cannot end
// or open a token), with runs of blanks collapsed, and short.
func sanitizeLabel(s string) string {
	var b strings.Builder
	blank := false
	for _, r := range s {
		switch {
		case r == '{' || r == '}':
			continue
		case unicode.IsSpace(r) || unicode.IsControl(r):
			blank = true
			continue
		}
		if blank && b.Len() > 0 {
			b.WriteByte(' ')
		}
		blank = false
		b.WriteRune(r)
	}
	out := b.String()
	if utf8.RuneCountInString(out) > maxLabelRunes {
		out = strings.TrimRight(string([]rune(out)[:maxLabelRunes]), " ") + "…"
	}
	return out
}

// escapeLabel escapes a label for Markdown. On top of the escaping of link
// text, it keeps Linkify from turning part of the label into a link, which
// would change the text: an "@" (email), the ":" of "http:", "https:" and
// "ftp:", and the "." of "www.".
func escapeLabel(label string) string {
	s := escapeText(label, textContext{inLink: true})
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '@',
			c == ':' && (hasSuffixFold(s[:i], "http") || hasSuffixFold(s[:i], "https") || hasSuffixFold(s[:i], "ftp")),
			c == '.' && hasSuffixFold(s[:i], "www"):
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	return b.String()
}

func hasSuffixFold(s, suffix string) bool {
	return len(s) >= len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix)
}

// containerClass maps a node type to the class of container it stands for,
// so the reader's view of a container (it renders a list, not its items) and
// the writer's (a paragraph's parent) compare equal.
func containerClass(nodeType string) string {
	switch nodeType {
	case "bulletList", "orderedList", "listItem":
		return "listItem"
	case "table", "tableRow", "tableCell", "tableHeader":
		return "tableCell"
	case "expand", "nestedExpand":
		return "expand"
	case "taskList", "taskItem", "blockTaskItem":
		return "taskItem"
	case "decisionList", "decisionItem":
		// A decision list reads as a bullet list, and is written back as
		// one, so its items are list items to the writer.
		return "listItem"
	case "layoutSection", "layoutColumn":
		return "layoutColumn"
	}
	return nodeType
}

// blockAllowedIn reports whether the ADF schema allows a block node of type
// nodeType in a container of the given class.
func blockAllowedIn(nodeType, class string) bool {
	switch nodeType {
	case "mediaSingle", "extension":
		switch class {
		case "doc", "blockquote", "listItem", "expand", "panel", "tableCell", "layoutColumn", "bodiedExtension":
			return true
		}
	case "mediaGroup":
		switch class {
		case "doc", "blockquote", "expand", "panel", "tableCell", "layoutColumn", "bodiedExtension":
			return true
		}
	case "bodiedExtension":
		return class == "doc" || class == "layoutColumn"
	case "blockCard", "embedCard":
		switch class {
		case "doc", "expand", "tableCell", "layoutColumn", "bodiedExtension":
			return true
		case "panel":
			return nodeType == "blockCard"
		}
	}
	return false
}

// readDocument renders doc as Markdown with protected-node tokens, choosing
// the token salt, and returns the Markdown and the collector holding the
// issues and the tokens. It is the single source of token numbers: ToMarkdown
// and Reattach both call it.
func readDocument(doc *jira4claude.ADFNode) (string, *skippedCollector) {
	for k := 0; ; k++ {
		salt := strings.Repeat(tokenSaltChar, k)
		md, skipped := renderDocument(doc, salt)
		want := make([]string, len(skipped.tokens.nodes))
		for i, p := range skipped.tokens.nodes {
			want[i] = p.token
		}
		if len(want) == 0 && strings.Count(md, "{") < 2 {
			// Every token holds "{{" in its text. Each "{" of the text the
			// writer reads back is a "{" of the Markdown: the reader never
			// escapes it or writes it as a character reference, and an
			// escaped "&" keeps "&#123;" literal. The two may be apart in
			// the Markdown ("**{**{"), so the count is what is sound, not a
			// search for "{{".
			return md, skipped
		}
		adf, _ := toADF(md, func(int) string { return "" })
		found := scanTokens(adf, tokenPattern(salt))
		if equalStrings(found, want) {
			return md, skipped
		}
		if k < maxTokenSalt && hasExtra(found, want) {
			// Literal text reads as a token: try the next salt.
			continue
		}
		reportUnreadTokens(skipped, found)
		return md, skipped
	}
}

// renderDocument renders the blocks of doc with the given token salt.
func renderDocument(doc *jira4claude.ADFNode, salt string) (string, *skippedCollector) {
	skipped := newSkippedCollector()
	skipped.tokens = &tokenRegistry{salt: salt}
	blocks := siblingBlocks{skipped: skipped}
	var parts []string
	for i := range doc.Content {
		part := blocks.render(&doc.Content[i])
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "\n\n"), skipped
}

// reportUnreadTokens records as dropped every token the reader wrote that
// does not read back: writing the Markdown back would lose its node.
func reportUnreadTokens(skipped *skippedCollector, found []string) {
	left := make(map[string]int)
	for _, f := range found {
		left[f]++
	}
	for _, p := range skipped.tokens.nodes {
		if left[p.token] > 0 {
			left[p.token]--
			continue
		}
		skipped.record(jira4claude.ConversionIssue{
			Kind: jira4claude.ConversionDropped,
			Node: p.token,
			Detail: fmt.Sprintf("the token for this %s does not read back as written, "+
				"so writing this text back would lose the node", nodeName(p.node.Type)),
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// hasExtra reports whether found holds a token that want does not, counting
// repeats.
func hasExtra(found, want []string) bool {
	left := make(map[string]int)
	for _, w := range want {
		left[w]++
	}
	for _, f := range found {
		if left[f] == 0 {
			return true
		}
		left[f]--
	}
	return false
}

// isTokenText reports whether n is text a token can be read from: text
// outside code. Runs of such nodes are read as one text, so a token whose
// label picked up marks is still found.
func isTokenText(n *jira4claude.ADFNode) bool {
	return n.Type == "text" && !hasCodeMark(n.Marks)
}

// tokenRunEnd returns the end of the run of token text that starts at
// nodes[i].
func tokenRunEnd(nodes []jira4claude.ADFNode, i int) int {
	for i < len(nodes) && isTokenText(&nodes[i]) {
		i++
	}
	return i
}

// runText joins the text of a run and returns where each node starts in it.
func runText(run []jira4claude.ADFNode) (string, []int) {
	var b strings.Builder
	starts := make([]int, len(run))
	for i := range run {
		starts[i] = b.Len()
		b.WriteString(run[i].Text)
	}
	return b.String(), starts
}

// scanTokens returns the text of every token in doc, in document order, read
// the way Reattach reads them.
func scanTokens(doc *jira4claude.ADFNode, re *regexp.Regexp) []string {
	var found []string
	var walk func(nodes []jira4claude.ADFNode)
	walk = func(nodes []jira4claude.ADFNode) {
		for i := 0; i < len(nodes); {
			if isTokenText(&nodes[i]) {
				j := tokenRunEnd(nodes, i)
				s, _ := runText(nodes[i:j])
				found = append(found, re.FindAllString(s, -1)...)
				i = j
				continue
			}
			if nodes[i].Type != "codeBlock" {
				walk(nodes[i].Content)
			}
			i++
		}
	}
	walk(doc.Content)
	return found
}

// reattach implements Converter.Reattach.
func reattach(original, edited *jira4claude.ADFNode) (*jira4claude.ADFNode, []jira4claude.ConversionIssue) {
	if original == nil {
		original = &jira4claude.ADFNode{Type: "doc", Version: 1}
	}
	if edited == nil {
		edited = &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{}}
	}
	_, skipped := readDocument(original)
	r := &reattacher{
		re:    tokenPattern(skipped.tokens.salt),
		nodes: skipped.tokens.nodes,
		seen:  make([]bool, len(skipped.tokens.nodes)),
	}

	out := *edited
	out.Content = r.content(edited.Type, edited.Content)
	for i, p := range r.nodes {
		if !r.seen[i] {
			r.drop(p.token, fmt.Sprintf("the token is not in the new text, so the %s would be removed", nodeName(p.node.Type)))
		}
	}
	return &out, r.issues
}

// reattacher replaces tokens in an edited document by original nodes.
type reattacher struct {
	re     *regexp.Regexp
	nodes  []protectedNode
	seen   []bool // nodes whose token was met
	issues []jira4claude.ConversionIssue
}

// piece is a node of rewritten inline content; block marks a restored block
// node, which splits its paragraph.
type piece struct {
	node  jira4claude.ADFNode
	block bool
}

func (r *reattacher) drop(node, detail string) {
	r.issues = append(r.issues, jira4claude.ConversionIssue{
		Kind:   jira4claude.ConversionDropped,
		Node:   node,
		Detail: detail,
	})
}

// content rewrites the children of a node of type parent. The input is not
// modified.
func (r *reattacher) content(parent string, children []jira4claude.ADFNode) []jira4claude.ADFNode {
	var out []jira4claude.ADFNode
	for i := 0; i < len(children); {
		if isTokenText(&children[i]) {
			// Inline content outside a paragraph (a heading, a task item):
			// a block node fits only if its token is all the content, in
			// the kind of container the node was read from.
			j := tokenRunEnd(children, i)
			mode := blockNever
			if i == 0 && j == len(children) {
				mode = blockWhole
			}
			for _, p := range r.run(children[i:j], parent, mode) {
				out = append(out, p.node)
			}
			i = j
			continue
		}
		c := children[i]
		i++
		switch c.Type {
		case "paragraph":
			out = append(out, r.paragraph(c, parent)...)
		case "codeBlock":
			out = append(out, c)
		default:
			if c.Content != nil {
				c.Content = r.content(c.Type, c.Content)
			}
			out = append(out, c)
		}
	}
	if out == nil && children != nil {
		out = []jira4claude.ADFNode{}
	}
	return out
}

// paragraph rewrites a paragraph whose parent has type parent. A block node
// restored inside it splits it: the text before and after stays in
// paragraphs of their own, without the blanks and line breaks next to the
// block, and a part left empty is left out.
func (r *reattacher) paragraph(p jira4claude.ADFNode, parent string) []jira4claude.ADFNode {
	var pieces []piece
	for i := 0; i < len(p.Content); {
		if isTokenText(&p.Content[i]) {
			j := tokenRunEnd(p.Content, i)
			pieces = append(pieces, r.run(p.Content[i:j], parent, blockSplit)...)
			i = j
			continue
		}
		pieces = append(pieces, piece{node: p.Content[i]})
		i++
	}

	var out []jira4claude.ADFNode
	var cur []jira4claude.ADFNode
	split := false
	flush := func(trimStart, trimEnd bool) {
		cur = trimInline(cur, trimStart, trimEnd)
		if len(cur) > 0 {
			q := p
			q.Content = cur
			out = append(out, q)
		}
		cur = nil
	}
	for _, pc := range pieces {
		if pc.block {
			flush(split, true)
			split = true
			out = append(out, pc.node)
			continue
		}
		cur = append(cur, pc.node)
	}
	if !split {
		q := p
		if p.Content != nil {
			q.Content = cur
			if q.Content == nil {
				q.Content = []jira4claude.ADFNode{}
			}
		}
		return []jira4claude.ADFNode{q}
	}
	flush(true, false)
	return out
}

// trimInline removes blanks and hardBreaks from the start and/or end of
// inline content.
func trimInline(nodes []jira4claude.ADFNode, start, end bool) []jira4claude.ADFNode {
	for start && len(nodes) > 0 {
		n := nodes[0]
		if n.Type == "hardBreak" {
			nodes = nodes[1:]
			continue
		}
		if !isTokenText(&n) {
			break
		}
		n.Text = strings.TrimLeft(n.Text, " \t")
		if n.Text == "" {
			nodes = nodes[1:]
			continue
		}
		nodes = append([]jira4claude.ADFNode{n}, nodes[1:]...)
		break
	}
	for end && len(nodes) > 0 {
		last := len(nodes) - 1
		n := nodes[last]
		if n.Type == "hardBreak" {
			nodes = nodes[:last]
			continue
		}
		if !isTokenText(&n) {
			break
		}
		n.Text = strings.TrimRight(n.Text, " \t")
		if n.Text == "" {
			nodes = nodes[:last]
			continue
		}
		nodes = append(nodes[:last:last], n)
		break
	}
	return nodes
}

// blockMode says whether a block node can be restored in a run of text.
type blockMode int

const (
	// blockNever: inline content outside a paragraph.
	blockNever blockMode = iota
	// blockSplit: a paragraph, which the block node splits.
	blockSplit
	// blockWhole: all the inline content of a container (a task item, say).
	// The block node replaces it when the run is just its token and the
	// reader found the node in the same kind of container.
	blockWhole
)

// run rewrites a run of token text found in a container of type container,
// replacing each token that resolves by its original node.
func (r *reattacher) run(nodes []jira4claude.ADFNode, container string, mode blockMode) []piece {
	s, starts := runText(nodes)
	matches := r.re.FindAllStringSubmatchIndex(s, -1)
	if mode == blockWhole && (len(matches) != 1 || strings.Trim(s, " \t") != s[matches[0][0]:matches[0][1]]) {
		mode = blockNever
	}
	var out []piece
	// emit appends the parts of the run's nodes that lie in s[from:to],
	// each keeping its marks.
	emit := func(from, to int) {
		for k := range nodes {
			lo, hi := max(starts[k], from), min(starts[k]+len(nodes[k].Text), to)
			if lo >= hi {
				continue
			}
			part := nodes[k]
			part.Text = nodes[k].Text[lo-starts[k] : hi-starts[k]]
			out = append(out, piece{node: part})
		}
	}
	pos := 0
	for _, m := range matches {
		node, block, ok := r.resolve(s[m[0]:m[1]], s[m[2]:m[3]], s[m[4]:m[5]], container, mode)
		if !ok {
			continue
		}
		if block && mode == blockWhole {
			// The token is all the run holds, but for blanks.
			return []piece{{node: node, block: true}}
		}
		emit(pos, m[0])
		out = append(out, piece{node: node, block: block})
		pos = m[1]
	}
	emit(pos, len(s))
	return out
}

// resolve returns the original node for a token found in the edited text,
// or reports why it stays text.
func (r *reattacher) resolve(token, kind, number, container string, mode blockMode) (jira4claude.ADFNode, bool, bool) {
	n, err := strconv.Atoi(number)
	if err != nil || n < 1 || n > len(r.nodes) {
		r.drop(token, "the token names no protected node of the original document, so it is left as text")
		return jira4claude.ADFNode{}, false, false
	}
	p := r.nodes[n-1]
	if kind != p.kind {
		// The number names another kind of node than when the text was
		// read: the document changed in between, so the numbers may have
		// shifted. The label is not trusted, but the kind is.
		r.drop(token, fmt.Sprintf("node %d of the current document is a %s, not a %s; "+
			"the document may have changed since it was read, so the token is left as text",
			n, nodeName(p.node.Type), kind))
		return jira4claude.ADFNode{}, false, false
	}
	if r.seen[n-1] {
		r.drop(token, fmt.Sprintf("the token appears more than once; only the first is restored as the original %s, "+
			"and this one is left as text", nodeName(p.node.Type)))
		return jira4claude.ADFNode{}, false, false
	}
	r.seen[n-1] = true
	if p.block {
		class := containerClass(container)
		var placeable bool
		switch mode {
		case blockSplit:
			placeable = class == p.parent || blockAllowedIn(p.node.Type, class)
		case blockWhole:
			placeable = class == p.parent
		case blockNever:
		}
		if !placeable {
			where := container
			if mode == blockSplit {
				where = "paragraph in a " + container
			}
			r.drop(p.token, fmt.Sprintf("a %s cannot stand in a %s, so the token is left as text; "+
				"put it on a line of its own where the node is allowed", nodeName(p.node.Type), where))
			return jira4claude.ADFNode{}, false, false
		}
	}
	return cloneNode(p.node), p.block, true
}

// nodeName names a node type in an issue's detail.
func nodeName(nodeType string) string {
	if nodeType == "" {
		return "node without a type"
	}
	return nodeType
}

// cloneNode returns a deep copy of n that shares no memory with it. Raw
// attribute bytes are copied as they are.
func cloneNode(n jira4claude.ADFNode) jira4claude.ADFNode {
	c := n
	if n.Attrs != nil {
		c.Attrs = append([]byte{}, n.Attrs...)
	}
	if n.Marks != nil {
		c.Marks = make([]jira4claude.ADFMark, len(n.Marks))
		for i, m := range n.Marks {
			c.Marks[i] = m
			if m.Attrs != nil {
				c.Marks[i].Attrs = append([]byte{}, m.Attrs...)
			}
		}
	}
	if n.Content != nil {
		c.Content = make([]jira4claude.ADFNode, len(n.Content))
		for i := range n.Content {
			c.Content[i] = cloneNode(n.Content[i])
		}
	}
	return c
}
