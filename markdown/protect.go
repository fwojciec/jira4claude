package markdown

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/fwojciec/jira4claude"
)

// Protected nodes (#149) are those whose content or behaviour plain text
// would lose: media, mediaSingle, mediaGroup, mediaInline, mention, status (a
// lozenge's colour and style), date (a timestamp, of which the text shows
// only the day), placeholder (an editor hint with no text of its own), and
// any node type the reader does not know. The reader
// writes each as a token, numbered in document order:
//
//	{{mediaSingle:1 shot.png}}   a block node, on a line of its own
//	{{mention:2 Jane Doe}}       an inline node, in the text
//	{{date:3 2026-11-10}}        a date, labelled with its UTC day
//
// Emoji and inline cards are not protected: their content reads as text (the
// character or :shortName:) and as a link to the card's URL. Reattach gives
// them back with the paragraph they are in while it is unchanged (see
// restore.go), puts back an inline card whose link can only be that card
// (see planCards), and reports the rest.
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
	// cards holds the inline cards read as links, in document order. They
	// are not tokens: the read keeps their content in readable form and
	// reports nothing, and Reattach puts back the cards it can match.
	cards []jira4claude.ADFNode
}

// within marks the start of the rendering of n, and the function it returns
// the end: the issues of a node in quiet are left out (see Reattach).
func (s *skippedCollector) within(n *jira4claude.ADFNode) func() {
	if s == nil || !s.quiet[n] {
		return func() {}
	}
	s.quietDepth++
	return func() { s.quietDepth-- }
}

// readCard records an inline card read as a link.
func (s *skippedCollector) readCard(node *jira4claude.ADFNode) {
	if s != nil && s.tokens != nil {
		s.tokens.cards = append(s.tokens.cards, *node)
	}
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
	defer s.within(node)()
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
	defer s.within(node)()
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
	return regexp.MustCompile(`\{\{` + regexp.QuoteMeta(salt) + tokenBody)
}

// anySaltTokenPattern matches the tokens of every salt, for text that was not
// read from a document and so has no salt of its own.
func anySaltTokenPattern() *regexp.Regexp {
	return regexp.MustCompile(`\{\{` + regexp.QuoteMeta(tokenSaltChar) + `*` + tokenBody)
}

// tokenBody is a token after its "{{" and salt.
const tokenBody = `([A-Za-z][A-Za-z0-9]*):([0-9]+)(?: [^{}\n]*)?\}\}`

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
	case "status":
		label = attrString(node, "text")
	case "date":
		label = dateLabel(node)
	default:
		label = attrString(node, "text")
		if label == "" {
			label = attrString(node, "extensionKey")
		}
	}
	return sanitizeLabel(label)
}

// dateLabel returns the label of a date node: the UTC day of its timestamp,
// which ADF holds as milliseconds since the epoch, in a string. A timestamp
// that does not parse, or whose day has no four-digit year, is shown as it
// is; one that is not a string gives no label. The label is never read back:
// the node is restored as it was read either way.
func dateLabel(node *jira4claude.ADFNode) string {
	ts := attrString(node, "timestamp")
	ms, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return ts
	}
	t := time.UnixMilli(ms).UTC()
	if t.Year() < 0 || t.Year() > 9999 {
		return ts
	}
	return t.Format("2006-01-02")
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
// would change the text: the ":" of "http:", "https:" and "ftp:", and the
// "." of "www.". Linkify links no email address (noEmail), so an "@" is left
// as it is.
func escapeLabel(label string) string {
	s := escapeText(label, textContext{inLink: true})
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ':' && (hasSuffixFold(s[:i], "http") || hasSuffixFold(s[:i], "https") || hasSuffixFold(s[:i], "ftp")),
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
		md, skipped := renderDocument(doc, salt, nil)
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

// renderDocument renders the blocks of doc with the given token salt,
// leaving out the issues of the nodes in quiet.
func renderDocument(doc *jira4claude.ADFNode, salt string, quiet map[*jira4claude.ADFNode]bool) (string, *skippedCollector) {
	skipped := newSkippedCollector()
	skipped.tokens = &tokenRegistry{salt: salt}
	skipped.quiet = quiet
	defer skipped.within(doc)()
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
			Kind:  jira4claude.ConversionDropped,
			Node:  p.token,
			Token: p.token,
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
		return clearDocument(original)
	}
	md, skipped := readDocument(original)

	r := newReattacher(skipped, md, edited)
	out := *edited
	out.Content = r.content(edited.Type, edited.Content)
	for i, p := range r.nodes {
		if !r.seen[i] {
			r.lose(p.token, fmt.Sprintf("the token is not in the new text, so the %s would be removed", nodeName(p.node.Type)))
		}
	}

	// The reference: the original's unchanged Markdown, written back.
	ref, _ := toADF(md, noLocalID)
	refR := newReattacher(skipped, md, ref)
	refOut := *ref
	refOut.Content = refR.content(ref.Type, ref.Content)

	rs := newRestorer()
	result := rs.node(original, &refOut, &out, false)

	// Issues about the new text stand; a protected node reported lost is
	// lost only if the result does not hold it after all.
	present := protectedCounts(&result)
	kept := make(map[string]bool)
	for _, p := range r.nodes {
		key := nodeJSON(&p.node)
		if present[key] > 0 {
			present[key]--
			kept[p.token] = true
		}
	}
	var issues []jira4claude.ConversionIssue
	for _, i := range r.issues {
		if i.Existing && kept[i.Token] {
			continue
		}
		issues = append(issues, i)
	}

	// What of the original the result changes: forms the writer never
	// makes (the census), containers the read reshapes, and what the read
	// itself reports about the parts not put back whole.
	changed := lostIssues(original, &result)
	changed = append(changed, rs.shapeIssues()...)
	_, reread := renderDocument(original, skipped.tokens.salt, rs.whole)
	changed = append(changed, readIssues(reread.issues(), changed)...)
	return &result, append(issues, changed...)
}

// clearDocument implements Reattach with a nil edited document, a clear:
// every token is missing, and content the read leaves out is reported as
// lost. Emoji and inline cards go with the rest of the text and are not
// reported as changing.
func clearDocument(original *jira4claude.ADFNode) (*jira4claude.ADFNode, []jira4claude.ConversionIssue) {
	md, skipped := readDocument(original)
	r := newReattacher(skipped, md, nil)
	out := &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{}}
	for _, p := range r.nodes {
		r.lose(p.token, fmt.Sprintf("the token is not in the new text, so the %s would be removed", nodeName(p.node.Type)))
	}
	for _, d := range jira4claude.DroppedIssues(skipped.issues()) {
		if d.Token == "" {
			r.issues = append(r.issues, existingDropped(d))
		}
	}
	return out, r.issues
}

// newReattacher returns a reattacher of the tokens that a read of the
// original, skipped and md, wrote into edited.
func newReattacher(skipped *skippedCollector, md string, edited *jira4claude.ADFNode) *reattacher {
	return &reattacher{
		re:    tokenPattern(skipped.tokens.salt),
		nodes: skipped.tokens.nodes,
		seen:  make([]bool, len(skipped.tokens.nodes)),
		cards: planCards(skipped.tokens.cards, md, edited),
	}
}

// existingDropped turns a dropped issue of a read of the original into one
// about the write, which would remove that content.
func existingDropped(d jira4claude.ConversionIssue) jira4claude.ConversionIssue {
	return jira4claude.ConversionIssue{
		Kind:     jira4claude.ConversionDropped,
		Node:     d.Node,
		Detail:   "existing content with no Markdown form (" + d.Detail + "); writing the description would remove it",
		Existing: true,
	}
}

// readIssues turns what a read of the original reports about the parts of it
// not put back whole into issues about the write: content the read leaves
// out is lost (dropped), and content it shows in another form is written back
// in that form (degraded). Token issues are reported as missing tokens, a
// code block's language is counted by the census, and a node an issue in
// named already names is not named again as degraded.
func readIssues(read, named []jira4claude.ConversionIssue) []jira4claude.ConversionIssue {
	seen := make(map[string]bool)
	for _, i := range named {
		seen[i.Node] = true
	}
	var out []jira4claude.ConversionIssue
	for _, d := range read {
		switch {
		case d.Token != "", d.Detail == codeLanguageDetail:
		case d.Kind == jira4claude.ConversionDropped:
			out = append(out, existingDropped(d))
		case !seen[d.Node]:
			out = append(out, jira4claude.ConversionIssue{
				Kind:     jira4claude.ConversionDegraded,
				Node:     d.Node,
				Detail:   "existing content the edit touched is written back in another form: " + d.Detail,
				Existing: true,
			})
		}
	}
	return out
}

// protectedCounts counts the outermost protected nodes of doc by their JSON.
func protectedCounts(doc *jira4claude.ADFNode) map[string]int {
	counts := make(map[string]int)
	var walk func(n *jira4claude.ADFNode)
	walk = func(n *jira4claude.ADFNode) {
		if !readerKnows(n.Type) {
			counts[nodeJSON(n)]++
			return
		}
		for i := range n.Content {
			walk(&n.Content[i])
		}
	}
	walk(doc)
	return counts
}

// noLocalID is the localId source of the conversions Reattach makes to
// compare documents, where localIds play no part.
func noLocalID(int) string { return "" }

// planCards returns the inline cards of the original document, cards (as
// the read found them), that Reattach puts back, keyed by the link each reads
// back as (cardLinkKey). The Markdown of a card is a link whose text is its
// URL, so a card is put back only where that link can be nothing else: for
// each such link, the original's unchanged Markdown, md, read back, holds it
// exactly as many times as there are cards it stands for (no plain link reads
// the same, and no two cards merged into one link), those cards are the same
// node, and edited holds it the same number of times. Otherwise the cards of
// that link stay links, and Reattach reports them.
func planCards(cards []jira4claude.ADFNode, md string, edited *jira4claude.ADFNode) map[string]jira4claude.ADFNode {
	if len(cards) == 0 || edited == nil {
		return nil
	}
	type group struct {
		card jira4claude.ADFNode
		n    int
		same bool
	}
	groups := make(map[string]*group)
	for i := range cards {
		key := cardKey(&cards[i])
		if key == "" {
			continue
		}
		g := groups[key]
		if g == nil {
			groups[key] = &group{card: cards[i], n: 1, same: true}
			continue
		}
		g.n++
		g.same = g.same && nodeJSON(&g.card) == nodeJSON(&cards[i])
	}
	if len(groups) == 0 {
		return nil
	}
	ref, _ := toADF(md, noLocalID)
	read, written := countCardLinks(ref), countCardLinks(edited)
	plan := make(map[string]jira4claude.ADFNode)
	for key, g := range groups {
		if g.same && read[key] == g.n && written[key] == g.n {
			plan[key] = g.card
		}
	}
	return plan
}

// cardKey returns the key (cardLinkKey) of the link an inline card's
// Markdown reads back as on its own, or "" when that is not a single link
// whose text is the card's URL.
func cardKey(card *jira4claude.ADFNode) string {
	url := inlineCardURL(card)
	doc, _ := toADF(adfInlineCardToGFM(card, true, nil), noLocalID)
	if len(doc.Content) != 1 || doc.Content[0].Type != "paragraph" || len(doc.Content[0].Content) != 1 {
		return ""
	}
	link := doc.Content[0].Content[0]
	if link.Text != url {
		return ""
	}
	return cardLinkKey(&link)
}

// cardLinkKey returns a key that identifies n if n is text carrying a link
// and no other mark, or "".
func cardLinkKey(n *jira4claude.ADFNode) string {
	if n.Type != "text" || len(n.Marks) != 1 || n.Marks[0].Type != "link" {
		return ""
	}
	return nodeJSON(n)
}

// nodeJSON returns n as JSON, or "" when it cannot be encoded.
func nodeJSON(n *jira4claude.ADFNode) string {
	data, err := json.Marshal(n)
	if err != nil {
		return ""
	}
	return string(data)
}

// countCardLinks counts the text of doc outside code blocks by cardLinkKey.
func countCardLinks(doc *jira4claude.ADFNode) map[string]int {
	counts := make(map[string]int)
	var walk func(nodes []jira4claude.ADFNode)
	walk = func(nodes []jira4claude.ADFNode) {
		for i := range nodes {
			if key := cardLinkKey(&nodes[i]); key != "" {
				counts[key]++
			}
			if nodes[i].Type != "codeBlock" {
				walk(nodes[i].Content)
			}
		}
	}
	walk(doc.Content)
	return counts
}

// reattacher replaces tokens in an edited document by original nodes.
type reattacher struct {
	re     *regexp.Regexp
	nodes  []protectedNode
	seen   []bool // nodes whose token was met
	issues []jira4claude.ConversionIssue
	// cards maps the link a restorable inline card reads back as (see
	// planCards) to the card.
	cards map[string]jira4claude.ADFNode
}

// piece is a node of rewritten inline content; block marks a restored block
// node, which splits its paragraph.
type piece struct {
	node  jira4claude.ADFNode
	block bool
}

// drop reports a token of the new text that is left as text, while the node
// it names, if any, is restored or reported on its own.
func (r *reattacher) drop(node, detail string) {
	r.issues = append(r.issues, jira4claude.ConversionIssue{
		Kind:   jira4claude.ConversionDropped,
		Node:   node,
		Token:  node,
		Detail: detail,
	})
}

// lose reports a node of the original document that the result does not hold:
// the write removes existing content.
func (r *reattacher) lose(node, detail string) {
	r.issues = append(r.issues, jira4claude.ConversionIssue{
		Kind:     jira4claude.ConversionDropped,
		Node:     node,
		Token:    node,
		Detail:   detail,
		Existing: true,
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
		case "taskItem":
			out = append(out, r.taskItem(c))
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

// taskItem rewrites a task item. A taskItem holds inline content only, so a
// block node restored in it (resolve allows only what a blockTaskItem
// holds) makes it a blockTaskItem with the same attrs: the inline content
// around the block goes into paragraphs, without the blanks and line breaks
// next to it, and a part left empty is left out.
func (r *reattacher) taskItem(item jira4claude.ADFNode) jira4claude.ADFNode {
	var pieces []piece
	for i := 0; i < len(item.Content); {
		c := item.Content[i]
		if isTokenText(&c) {
			j := tokenRunEnd(item.Content, i)
			pieces = append(pieces, r.run(item.Content[i:j], item.Type, blockSplit)...)
			i = j
			continue
		}
		i++
		if c.Type == "paragraph" {
			for _, n := range r.paragraph(c, item.Type) {
				pieces = append(pieces, piece{node: n, block: true})
			}
			continue
		}
		if c.Content != nil {
			c.Content = r.content(c.Type, c.Content)
		}
		pieces = append(pieces, piece{node: c})
	}

	hasBlock := false
	for _, pc := range pieces {
		hasBlock = hasBlock || pc.block
	}
	if !hasBlock {
		if item.Content == nil {
			return item
		}
		nodes := make([]jira4claude.ADFNode, 0, len(pieces))
		for _, pc := range pieces {
			nodes = append(nodes, pc.node)
		}
		item.Content = nodes
		return item
	}

	var content, cur []jira4claude.ADFNode
	flush := func() {
		cur = trimInline(cur, true, true)
		if len(cur) > 0 {
			content = append(content, jira4claude.ADFNode{Type: "paragraph", Content: cur})
		}
		cur = nil
	}
	for _, pc := range pieces {
		if pc.block {
			flush()
			content = append(content, pc.node)
			continue
		}
		cur = append(cur, pc.node)
	}
	flush()
	item.Type = "blockTaskItem"
	item.Content = content
	return item
}

// knownBlockType reports whether nodeType is a block node type of the ADF
// schema.
func knownBlockType(nodeType string) bool {
	switch nodeType {
	case "paragraph", "heading", "blockquote", "codeBlock", "rule", "panel",
		"bulletList", "orderedList", "listItem", "taskList", "taskItem", "blockTaskItem",
		"decisionList", "decisionItem", "table", "tableRow", "tableCell", "tableHeader",
		"expand", "nestedExpand", "layoutSection", "layoutColumn",
		"mediaSingle", "mediaGroup", "media", "extension", "bodiedExtension",
		"blockCard", "embedCard", "multiBodiedExtension", "extensionFrame", "syncBlock", "bodiedSyncBlock":
		return true
	}
	return false
}

// blockAllowedInTaskItem reports whether a block node of type nodeType can
// stand in a task item, which must then be a blockTaskItem: the schema
// allows paragraphs and extensions there.
func blockAllowedInTaskItem(nodeType string) bool {
	return nodeType == "extension"
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
			if lo == starts[k] && hi == starts[k]+len(nodes[k].Text) {
				// A whole node, not a part a token split off.
				if card, ok := r.cards[cardLinkKey(&part)]; ok {
					part = cloneNode(card)
				}
			}
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
		switch {
		case class == "taskItem" && !knownBlockType(p.node.Type) && p.parent == "taskItem":
			// A node of a type the reader does not know, read from a task
			// item: whether it is block-level is unknown, so it goes back
			// into the task item's content, as it was read.
			return cloneNode(p.node), false, true
		case class == "taskItem":
			// Whatever the reader found it in: a task item holds what a
			// blockTaskItem allows, or nothing block-level at all.
			placeable = mode != blockNever && blockAllowedInTaskItem(p.node.Type)
		case mode == blockSplit:
			placeable = class == p.parent || blockAllowedIn(p.node.Type, class)
		case mode == blockWhole:
			placeable = class == p.parent
		}
		if !placeable {
			where := container
			switch {
			case class == "taskItem":
				where = "task item"
			case mode == blockSplit:
				where = "paragraph in a " + container
			}
			r.lose(p.token, fmt.Sprintf("a %s cannot stand in a %s, so the token is left as text; "+
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
