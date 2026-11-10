package markdown

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/fwojciec/jira4claude"
)

// codeLanguageDetail explains the degraded issue for a code block language
// with whitespace in it.
const codeLanguageDetail = "a code block language containing whitespace cannot be written in Markdown; only its first word is kept"

// Restoring what Markdown cannot carry (jir-h44x).
//
// Reattach compares three documents: the original, as Jira holds it; the
// reference, which is the original's unchanged Markdown written back (tokens
// re-attached); and the edited document (tokens re-attached). Where the
// edited document holds what the reference holds, the edit changed nothing
// there, so the original is put back. Where it does not, the attributes and
// block marks the edit left as the reference has them are taken from the
// original. What still differs from the original is reported (see census).
//
// Matching. The three are walked together from the document down. At each
// level, a child of the original is matched to the run of reference children
// it reads back as, and reference children to edited ones:
//
//   - Original to reference. In a container of blocks (the document, a
//     quote, a panel, a list item, an expand, a layout column, a table cell),
//     each child of the original is read and written back on its own; when
//     the runs it gives, joined, are the reference's children exactly, child
//     i stands for run i (one block, several, as a flattened layout, or none,
//     as an empty paragraph). Otherwise, and in lists, tables and rows, the
//     k-th child of a type stands for the k-th reference child of that type,
//     only when both hold the same number of that type.
//   - Reference to edited. Children that are equal (a whole subtree, localIds
//     aside, since the writer makes those up) pair the k-th with the k-th,
//     only when both hold the same number of that subtree. The children left
//     pair by type in the same way, only when both have the same number left
//     of that type.
//
// An original child whose run is paired equal, block for block and in order,
// comes back whole. One whose single reference block is paired by type only
// (the edit changed it) keeps the edited node, with its attributes and block
// marks merged: a value the edit left as the reference has it comes from the
// original, and a localId always does. Inside it the same matching goes on
// for the children of containers; inline content (a paragraph's text) is
// taken from the edit as it is. An original child that reads as nothing
// goes back next to the original child before (or after) it, when that one
// came back whole.
//
// Table geometry (colspan, rowspan, colwidth) is restored only while the
// table's shape, the number of cells in each row, is the same in all three.
//
// Not detected: an edit that removes one node and adds another of the same
// type in the same container, or reorders same-type siblings and changes
// them, gives the added node the attributes of the removed one.

// restorer holds what one restoration did.
type restorer struct {
	// whole holds the nodes of the original put back whole.
	whole map[*jira4claude.ADFNode]bool
	// reshaped counts, by container type, original containers some of whose
	// children read back in another shape, so they could not be matched.
	reshaped map[string]int
	// unplaced counts, by node type, original children that read as
	// nothing and had no neighbour to go back next to.
	unplaced map[string]int
	// alone memoises standaloneKeys.
	alone map[*jira4claude.ADFNode][]string
}

func newRestorer() *restorer {
	return &restorer{
		whole:    make(map[*jira4claude.ADFNode]bool),
		reshaped: make(map[string]int),
		unplaced: make(map[string]int),
		alone:    make(map[*jira4claude.ADFNode][]string),
	}
}

// Container kinds, for matching children.
const (
	containerNone   = iota // inline content, a leaf, or a protected node
	containerBlocks        // children are blocks, matched by their runs
	containerItems         // children are items, rows or cells, matched by type
)

func containerKind(nodeType string) int {
	switch nodeType {
	case "doc", "blockquote", "panel", "listItem", "expand", "nestedExpand", "layoutColumn", "tableCell", "tableHeader":
		return containerBlocks
	case "bulletList", "orderedList", "taskList", "decisionList", "table", "tableRow", "layoutSection":
		return containerItems
	}
	return containerNone
}

// node returns e, the edited node matched to o through r, with o put back
// as far as the edit allows. noGeometry is set inside a table whose shape
// changed.
func (rs *restorer) node(o, r, e *jira4claude.ADFNode, noGeometry bool) jira4claude.ADFNode {
	if o.Type != e.Type || r.Type != e.Type {
		return *e
	}
	geometric := isTablePart(e.Type)
	if (!noGeometry || !geometric) && normKey(e) == normKey(r) {
		rs.whole[o] = true
		return cloneNode(*o)
	}
	out := *e
	out.Attrs = mergeAttrs(o, r, e, noGeometry)
	if e.Type != "text" && marksKey(e.Marks) == marksKey(r.Marks) {
		out.Marks = cloneNode(jira4claude.ADFNode{Marks: o.Marks}).Marks
	}
	if containerKind(e.Type) == containerNone {
		return out
	}
	if e.Type == "table" && (tableShape(e) != tableShape(r) || tableShape(o) != tableShape(r)) {
		noGeometry = true
	}
	out.Content = rs.children(o, r, e, noGeometry)
	if out.Content == nil && e.Content != nil {
		out.Content = []jira4claude.ADFNode{}
	}
	return out
}

func isTablePart(nodeType string) bool {
	return nodeType == "tableRow" || nodeType == "tableCell" || nodeType == "tableHeader"
}

// span is the run of reference children, [a, b), that an original child
// reads back as; ok is false when it is not known.
type span struct {
	a, b int
	ok   bool
}

// children returns the children of e with the children of o put back as the
// matching rules allow.
func (rs *restorer) children(o, r, e *jira4claude.ADFNode, noGeometry bool) []jira4claude.ADFNode {
	spans := rs.spans(o, r)
	for _, s := range spans {
		if !s.ok {
			rs.reshaped[o.Type]++
			break
		}
	}

	rKeys := childKeys(r.Content)
	eKeys := childKeys(e.Content)
	exactR := pairBy(rKeys, eKeys)
	rTypes, eTypes := make([]string, len(r.Content)), make([]string, len(e.Content))
	for i := range r.Content {
		if exactR[i] < 0 {
			rTypes[i] = r.Content[i].Type
		}
	}
	exactE := make(map[int]bool)
	for _, j := range exactR {
		if j >= 0 {
			exactE[j] = true
		}
	}
	for j := range e.Content {
		if !exactE[j] {
			eTypes[j] = e.Content[j].Type
		}
	}
	byType := pairBy(rTypes, eTypes)

	type pair struct{ o, r int }
	whole := make(map[int]pair) // edited index -> original child put back whole
	merge := make(map[int]pair) // edited index -> original child merged into it
	skip := make(map[int]bool)
	before := make(map[int][]int)
	after := make(map[int][]int)
	for i, s := range spans {
		if !s.ok {
			continue
		}
		oc := &o.Content[i]
		switch n := s.b - s.a; {
		case n == 0:
			switch {
			case s.a > 0 && exactR[s.a-1] >= 0:
				after[exactR[s.a-1]] = append(after[exactR[s.a-1]], i)
			case s.a < len(r.Content) && exactR[s.a] >= 0:
				before[exactR[s.a]] = append(before[exactR[s.a]], i)
			default:
				rs.unplaced[oc.Type]++
			}
		case n == 1 && exactR[s.a] >= 0:
			whole[exactR[s.a]] = pair{i, s.a}
		case n == 1 && byType[s.a] >= 0 && oc.Type == r.Content[s.a].Type:
			merge[byType[s.a]] = pair{i, s.a}
		case n > 1 && consecutive(exactR[s.a:s.b]):
			whole[exactR[s.a]] = pair{i, s.a}
			for _, j := range exactR[s.a+1 : s.b] {
				skip[j] = true
			}
		}
	}

	putBack := func(i int) jira4claude.ADFNode {
		rs.whole[&o.Content[i]] = true
		return cloneNode(o.Content[i])
	}
	var out []jira4claude.ADFNode
	for j := range e.Content {
		for _, i := range before[j] {
			out = append(out, putBack(i))
		}
		switch p, isWhole := whole[j]; {
		case skip[j]:
		case isWhole && noGeometry && isTablePart(o.Content[p.o].Type):
			// Rows and cells of a reshaped table keep the edit's geometry.
			out = append(out, rs.node(&o.Content[p.o], &r.Content[p.r], &e.Content[j], noGeometry))
		case isWhole:
			out = append(out, putBack(p.o))
		default:
			if m, ok := merge[j]; ok {
				out = append(out, rs.node(&o.Content[m.o], &r.Content[m.r], &e.Content[j], noGeometry))
			} else {
				out = append(out, e.Content[j])
			}
		}
		for _, i := range after[j] {
			out = append(out, putBack(i))
		}
	}
	return out
}

// consecutive reports whether idx holds paired indexes that follow each
// other.
func consecutive(idx []int) bool {
	for k, j := range idx {
		if j < 0 || (k > 0 && j != idx[k-1]+1) {
			return false
		}
	}
	return true
}

// pairBy pairs the k-th element of a with the k-th of b among the elements
// with the same key, for each key both hold the same number of times. It
// returns, for each element of a, the index of its partner in b, or -1. An
// empty key never pairs.
func pairBy(a, b []string) []int {
	at := make(map[string][]int)
	for i, k := range a {
		if k != "" {
			at[k] = append(at[k], i)
		}
	}
	bt := make(map[string][]int)
	for j, k := range b {
		if k != "" {
			bt[k] = append(bt[k], j)
		}
	}
	out := make([]int, len(a))
	for i := range out {
		out[i] = -1
	}
	for k, is := range at {
		js := bt[k]
		if len(js) != len(is) {
			continue
		}
		for n, i := range is {
			out[i] = js[n]
		}
	}
	return out
}

// spans matches the children of o to runs of the children of r.
func (rs *restorer) spans(o, r *jira4claude.ADFNode) []span {
	out := make([]span, len(o.Content))
	if containerKind(o.Type) == containerBlocks {
		var joined []string
		for i := range o.Content {
			keys := rs.standaloneKeys(&o.Content[i])
			out[i] = span{a: len(joined), b: len(joined) + len(keys), ok: true}
			joined = append(joined, keys...)
		}
		if equalStrings(joined, childKeys(r.Content)) {
			return out
		}
	}
	oTypes := make([]string, len(o.Content))
	for i := range o.Content {
		oTypes[i] = o.Content[i].Type
	}
	rTypes := make([]string, len(r.Content))
	for i := range r.Content {
		rTypes[i] = r.Content[i].Type
	}
	for i, j := range pairBy(oTypes, rTypes) {
		out[i] = span{a: j, b: j + 1, ok: j >= 0}
	}
	return out
}

// standaloneKeys returns the keys (normKey) of the blocks that n, read on
// its own as a document and written back, gives.
func (rs *restorer) standaloneKeys(n *jira4claude.ADFNode) []string {
	if keys, ok := rs.alone[n]; ok {
		return keys
	}
	doc := &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{*n}}
	md, skipped := readDocument(doc)
	ref, _ := toADF(md, noLocalID)
	r := newReattacher(skipped, md, ref)
	keys := childKeys(r.content(ref.Type, ref.Content))
	if keys == nil {
		keys = []string{}
	}
	rs.alone[n] = keys
	return keys
}

func childKeys(nodes []jira4claude.ADFNode) []string {
	keys := make([]string, len(nodes))
	for i := range nodes {
		keys[i] = normKey(&nodes[i])
	}
	return keys
}

// normKey returns n as JSON with its localIds left out, so two documents the
// writer made from the same Markdown compare equal.
func normKey(n *jira4claude.ADFNode) string {
	data, err := json.Marshal(withoutLocalIDs(*n))
	if err != nil {
		return ""
	}
	return string(data)
}

func withoutLocalIDs(n jira4claude.ADFNode) jira4claude.ADFNode {
	if n.Attrs != nil {
		var attrs map[string]json.RawMessage
		if json.Unmarshal(n.Attrs, &attrs) == nil {
			if _, ok := attrs["localId"]; ok {
				delete(attrs, "localId")
				n.Attrs, _ = json.Marshal(attrs)
			}
		}
	}
	if n.Content != nil {
		content := make([]jira4claude.ADFNode, len(n.Content))
		for i := range n.Content {
			content[i] = withoutLocalIDs(n.Content[i])
		}
		n.Content = content
	}
	return n
}

func marksKey(marks []jira4claude.ADFMark) string {
	if len(marks) == 0 {
		return ""
	}
	data, _ := json.Marshal(marks)
	return string(data)
}

// tableShape returns the number of cells in each row of a table.
func tableShape(n *jira4claude.ADFNode) string {
	var b strings.Builder
	for i := range n.Content {
		b.WriteString(strconv.Itoa(len(n.Content[i].Content)))
		b.WriteByte(',')
	}
	return b.String()
}

// mergeAttrs returns the attributes of e, edited from r, with those of o put
// back: each key whose value e leaves as r has it takes o's value (or is
// left out when o has none), and a localId is always o's when o has one.
// Under noGeometry, a table cell's colspan, rowspan and colwidth stay e's.
// When the result is o's attributes, o's bytes are returned as they are.
func mergeAttrs(o, r, e *jira4claude.ADFNode, noGeometry bool) json.RawMessage {
	om, ok1 := attrMap(o.Attrs)
	rm, ok2 := attrMap(r.Attrs)
	em, ok3 := attrMap(e.Attrs)
	if !ok1 || !ok2 || !ok3 {
		return e.Attrs
	}
	keys := make(map[string]bool)
	for _, m := range []map[string]json.RawMessage{om, rm, em} {
		for k := range m {
			keys[k] = true
		}
	}
	res := make(map[string]json.RawMessage)
	for k := range keys {
		ov, oh := om[k]
		ev, eh := em[k]
		rv, rh := rm[k]
		switch {
		case k == "localId" && oh:
			res[k] = ov
		case k == "localId" || (noGeometry && isGeometry(k)):
			if eh {
				res[k] = ev
			}
		case eh == rh && (!eh || jsonEqual(ev, rv)):
			if oh {
				res[k] = ov
			}
		case eh:
			res[k] = ev
		}
	}
	switch {
	case attrsEqual(res, om):
		return o.Attrs
	case attrsEqual(res, em):
		return e.Attrs
	}
	data, err := json.Marshal(res)
	if err != nil {
		return e.Attrs
	}
	return data
}

func isGeometry(key string) bool {
	return key == "colspan" || key == "rowspan" || key == "colwidth"
}

// attrMap decodes attributes; nil decodes as none.
func attrMap(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	m := make(map[string]json.RawMessage)
	if raw == nil {
		return m, true
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, false
	}
	if m == nil {
		m = make(map[string]json.RawMessage)
	}
	return m, true
}

func attrsEqual(a, b map[string]json.RawMessage) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		w, ok := b[k]
		if !ok || !jsonEqual(v, w) {
			return false
		}
	}
	return true
}

func jsonEqual(a, b json.RawMessage) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return string(a) == string(b)
	}
	return reflect.DeepEqual(x, y)
}

// Census: what of the original the result no longer holds, in forms the
// writer never makes, counted by kind. The protected nodes are left out:
// they are restored verbatim from their tokens, or reported as lost.

// census counts the features of a document that Markdown cannot carry.
type census struct {
	nodes map[string]int            // node types the writer never makes
	marks map[string]map[string]int // mark type -> mark JSON -> count
	attrs map[string]map[string]int // node or mark type -> key=value -> count
	ids   map[string]int            // node type + localId -> count
}

// readerKnows reports whether the reader renders nodeType itself, rather
// than as a protected-node token.
func readerKnows(nodeType string) bool {
	switch nodeType {
	case "doc", "paragraph", "heading", "codeBlock", "bulletList", "orderedList", "listItem", "blockquote",
		"panel", "expand", "nestedExpand", "rule", "table", "tableRow", "tableCell", "tableHeader",
		"taskList", "taskItem", "blockTaskItem", "decisionList", "decisionItem", "layoutSection", "layoutColumn",
		"hardBreak", "text", "emoji", "inlineCard":
		return true
	}
	return false
}

// unwrittenNodes are node types the reader shows in another form and the
// writer never makes; their parts (decisionItem, layoutColumn) count with
// them.
func unwrittenNode(nodeType string) bool {
	switch nodeType {
	case "decisionList", "layoutSection", "nestedExpand", "blockTaskItem", "emoji", "inlineCard":
		return true
	}
	return false
}

// writtenMark reports whether the writer makes marks of this type.
func writtenMark(markType string) bool {
	switch markType {
	case "strong", "em", "strike", "code", "link", "underline", "subsup":
		return true
	}
	return false
}

func takeCensus(doc *jira4claude.ADFNode) census {
	c := census{
		nodes: make(map[string]int),
		marks: make(map[string]map[string]int),
		attrs: make(map[string]map[string]int),
		ids:   make(map[string]int),
	}
	var walk func(n *jira4claude.ADFNode, inside bool)
	walk = func(n *jira4claude.ADFNode, inside bool) {
		if !readerKnows(n.Type) {
			return
		}
		if unwrittenNode(n.Type) {
			c.nodes[n.Type]++
			inside = true
		}
		for _, m := range n.Marks {
			if !writtenMark(m.Type) {
				add(c.marks, m.Type, string(m.Attrs))
				continue
			}
			if m.Type == "link" {
				am, _ := attrMap(m.Attrs)
				for k, v := range am {
					if k != "href" {
						add(c.attrs, "link", k+"="+canonJSON(v))
					}
				}
			}
		}
		am, _ := attrMap(n.Attrs)
		for k, v := range am {
			switch {
			case k == "localId":
				if !inside {
					c.ids[n.Type+" "+canonJSON(v)]++
				}
			case !inside && hiddenAttr(n.Type, k, v):
				add(c.attrs, n.Type, k+"="+canonJSON(v))
			}
		}
		for i := range n.Content {
			walk(&n.Content[i], inside)
		}
	}
	walk(doc, false)
	return c
}

func add(m map[string]map[string]int, outer, inner string) {
	if m[outer] == nil {
		m[outer] = make(map[string]int)
	}
	m[outer][inner]++
}

func canonJSON(v json.RawMessage) string {
	var x any
	if json.Unmarshal(v, &x) != nil {
		return string(v)
	}
	data, _ := json.Marshal(x)
	return string(data)
}

// hiddenAttr reports whether the value v of attribute key on a node of type
// nodeType has no Markdown form, so a rewrite that does not restore it loses
// it. A value that means the same as no attribute (an ordered list starting
// at 1, a table cell spanning one column) is not hidden.
func hiddenAttr(nodeType, key string, v json.RawMessage) bool {
	var x any
	_ = json.Unmarshal(v, &x)
	s, isString := x.(string)
	f, isNumber := x.(float64)
	switch nodeType + "." + key {
	case "heading.level", "taskItem.state", "expand.title", "nestedExpand.title":
		return false
	case "codeBlock.language":
		return !isString || infoFirstWord(s) != s
	case "panel.panelType":
		return !isString || (s != "info" && s != "note" && s != "warning" && s != "error" && s != "success")
	case "orderedList.order":
		return !isNumber || f != float64(int64(f)) || f < 0 || f > 999999999
	case "hardBreak.text":
		return s != "\n"
	case "table.layout", "table.displayMode":
		return s != "default"
	case "table.isNumberColumnEnabled":
		return x != false
	case "tableCell.colspan", "tableCell.rowspan", "tableHeader.colspan", "tableHeader.rowspan":
		return !isNumber || f != 1
	}
	return true
}

// lostIssues reports, as Existing issues, what the original held in a form
// Markdown cannot carry and the result does not: one issue per node or mark
// type with the count, and one for all localIds.
func lostIssues(original, result *jira4claude.ADFNode) []jira4claude.ConversionIssue {
	before, after := takeCensus(original), takeCensus(result)
	var issues []jira4claude.ConversionIssue
	degraded := func(node, detail string) {
		issues = append(issues, jira4claude.ConversionIssue{
			Kind: jira4claude.ConversionDegraded, Node: node, Detail: detail, Existing: true,
		})
	}

	for _, t := range sortedKeys(before.nodes) {
		lost := before.nodes[t] - after.nodes[t]
		if lost <= 0 {
			continue
		}
		degraded(t, lostNodeDetail(t, lost))
	}
	for _, t := range sortedKeys(before.marks) {
		lost := deficit(before.marks[t], after.marks[t])
		if lost == 0 {
			continue
		}
		if t == "annotation" {
			issues = append(issues, jira4claude.ConversionIssue{
				Kind: jira4claude.ConversionDropped,
				Node: t,
				Detail: fmt.Sprintf("%s (inline comment marks) would come off the text they mark, which detaches the comments: "+
					"Markdown cannot carry them, and they are kept only on text the edit leaves as it was", plural(lost, "mark", "marks")),
				Existing: true,
			})
			continue
		}
		degraded(t, fmt.Sprintf("%s of type %s would be removed: Markdown cannot show them, "+
			"and they are kept only on text the edit leaves as it was", plural(lost, "mark", "marks"), t))
	}
	for _, t := range sortedKeys(before.attrs) {
		lost, keys := deficitKeys(before.attrs[t], after.attrs[t])
		if lost == 0 {
			continue
		}
		degraded(t, fmt.Sprintf("%s of %s would be lost (%s): Markdown cannot show them, "+
			"and they are kept only where the edit leaves the node in place", plural(lost, "attribute value", "attribute values"),
			t, strings.Join(keys, ", ")))
	}
	if lost, types := idDeficit(before.ids, after.ids); lost > 0 {
		degraded("localId", fmt.Sprintf("%s (of %s) would be replaced or left out: they are kept only "+
			"where the edit leaves the node in place", plural(lost, "localId", "localIds"), strings.Join(types, ", ")))
	}
	return issues
}

func lostNodeDetail(nodeType string, n int) string {
	switch nodeType {
	case "emoji":
		verb := "become"
		if n == 1 {
			verb = "becomes"
		}
		return fmt.Sprintf("%d emoji %s plain text: Markdown has no emoji, so each is written as its character or :shortName:", n, verb)
	case "inlineCard":
		what := fmt.Sprintf("%d smart links become plain links", n)
		if n == 1 {
			what = "1 smart link becomes a plain link"
		}
		return what + ": an inline card is put back where the text around it is unchanged, " +
			"or where its link comes back as only that card can (see the README)"
	case "decisionList":
		return plural(n, "decision list becomes a bullet list", "decision lists become bullet lists") +
			": Markdown has no decision list, and one is kept only while the edit leaves it unchanged"
	case "layoutSection":
		return plural(n, "layout is", "layouts are") + " flattened, the columns' content one after another: " +
			"Markdown has no columns, and a layout is kept only while the edit leaves its content unchanged"
	case "nestedExpand":
		return plural(n, "nested expand is", "nested expands are") + " flattened into the expand around it: " +
			"Markdown cannot nest them, and one is kept only while the edit leaves the expand around it unchanged"
	case "blockTaskItem":
		return plural(n, "task item holding paragraphs becomes", "task items holding paragraphs become") +
			" a plain task item, kept as it was only while the edit leaves it unchanged"
	}
	return plural(n, nodeType+" node changes form", nodeType+" nodes change form")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func deficit(before, after map[string]int) int {
	n := 0
	for k, b := range before {
		if d := b - after[k]; d > 0 {
			n += d
		}
	}
	return n
}

// deficitKeys returns the deficit of key=value counts and the keys lost.
func deficitKeys(before, after map[string]int) (int, []string) {
	n := 0
	keys := make(map[string]bool)
	for kv, b := range before {
		if d := b - after[kv]; d > 0 {
			n += d
			k, _, _ := strings.Cut(kv, "=")
			keys[k] = true
		}
	}
	return n, sortedKeys(keys)
}

// idDeficit returns the localIds lost and the node types they were on.
func idDeficit(before, after map[string]int) (int, []string) {
	n := 0
	types := make(map[string]bool)
	for k, b := range before {
		if d := b - after[k]; d > 0 {
			n += d
			t, _, _ := strings.Cut(k, " ")
			types[t] = true
		}
	}
	return n, sortedKeys(types)
}

// shapeIssues reports the containers a restoration could not match because
// the read changes their shape, and the children that read as nothing and
// found no place.
func (rs *restorer) shapeIssues() []jira4claude.ConversionIssue {
	issues := make([]jira4claude.ConversionIssue, 0, len(rs.reshaped)+len(rs.unplaced))
	for _, t := range sortedKeys(rs.reshaped) {
		issues = append(issues, jira4claude.ConversionIssue{
			Kind: jira4claude.ConversionDegraded,
			Node: t,
			Detail: plural(rs.reshaped[t], t+" is", t+" nodes are") + " written back in another shape: Markdown cannot hold " +
				"what they hold as it is (a table without a header row, a header cell outside the first row, several blocks " +
				"in a table cell), and they are kept as they were only while the edit leaves them unchanged",
			Existing: true,
		})
	}
	for _, t := range sortedKeys(rs.unplaced) {
		issues = append(issues, jira4claude.ConversionIssue{
			Kind:     jira4claude.ConversionDegraded,
			Node:     t,
			Detail:   plural(rs.unplaced[t], t, t+" nodes") + " with nothing to show (an empty paragraph, say) would be left out: the edit changed the blocks around them",
			Existing: true,
		})
	}
	return issues
}
