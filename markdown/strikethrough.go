package markdown

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// strikethroughParser wraps goldmark's GFM strikethrough parser so that only
// a run of exactly two tildes can open or close a strike. goldmark accepts one
// or two, which turns "5~10 and 20~30" into a strike; agents write single
// tildes for "approximately" and ranges, so a single tilde stays literal text.
type strikethroughParser struct {
	gfm parser.InlineParser
}

func newStrikethroughParser() parser.InlineParser {
	return &strikethroughParser{gfm: extension.NewStrikethroughParser()}
}

func (p *strikethroughParser) Trigger() []byte {
	return p.gfm.Trigger()
}

// Parse declines any tilde run whose length is not exactly two. A declined
// run is emitted as text by goldmark. Runs of two are handed to the GFM parser
// unchanged, so "~~text~~" converts exactly as before.
func (p *strikethroughParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	if block.PrecendingCharacter() == '~' {
		return nil
	}
	line, _ := block.PeekLine()
	run := 0
	for run < len(line) && line[run] == '~' {
		run++
	}
	if run != 2 {
		return nil
	}
	return p.gfm.Parse(parent, block, pc)
}
