package revision

import (
	"fmt"
	"strings"
)

const minPrefix = 4

type BaseName struct {
	Name string
	Note string
}

func (c Comparison) BaseName() BaseName {
	tip := Short(c.BaseTip)
	prefix := strings.HasPrefix(c.BaseTip, strings.ToLower(c.BaseRef))
	byName := len(c.BaseRef) < minPrefix || !prefix
	switch {
	case c.MergeBase != c.BaseTip && byName:
		return BaseName{c.BaseRef, "merge base " + Short(c.MergeBase)}
	case c.MergeBase != c.BaseTip:
		return BaseName{tip, "merge base " + Short(c.MergeBase)}
	case byName:
		return BaseName{c.BaseRef, tip}
	}
	return BaseName{Name: tip}
}

func (c Comparison) BaseLabel() string {
	base := c.BaseName()
	if base.Note == "" {
		return base.Name
	}
	return fmt.Sprintf("%s (%s)", base.Name, base.Note)
}

func (c Comparison) HeadLabel() string {
	label := Short(c.Head)
	if c.Dirty {
		label += " + uncommitted changes"
	}
	return label
}
